package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/application/v1/command"
	"github.com/dennis-dko/timepulse/internal/application/v1/service"
	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/domain/repository"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/memory"
)

// demoFixture is a new installation as the installer leaves it: migrated, with
// the built-in administrator and the shipped roles, and nothing else.
type demoFixture struct {
	demo       *service.DemoService
	users      *memory.UserRepository
	projects   *memory.ProjectRepository
	timesheets *memory.TimesheetRepository
	absences   *memory.AbsenceRepository
	templates  *memory.TemplateRepository
	projectApp *service.ProjectApplicationService
	admin      uint
}

func newDemoFixture(t *testing.T, today time.Time) *demoFixture {
	t.Helper()

	users := memory.NewUserRepository()
	roles := memory.NewRoleRepository(users)
	projects := memory.NewProjectRepository()
	timesheets := memory.NewTimesheetRepository()
	schedules := memory.NewWorkScheduleRepository()
	absences := memory.NewAbsenceRepository()
	templates := memory.NewTemplateRepository()

	if _, err := service.NewAuthService(users, roles).EnsureSystemUser(context.Background()); err != nil {
		t.Fatalf("seed the built-in administrator: %v", err)
	}

	admin, err := users.GetByEmail(context.Background(), service.SystemUserEmail)
	if err != nil {
		t.Fatalf("read the built-in administrator: %v", err)
	}

	places := service.PlaceSource(func(context.Context) (string, error) { return "", nil })
	clock := service.Today(func(context.Context, *model.User) time.Time { return today })
	projectApp := service.NewProjectApplicationService(projects, timesheets, memory.NewTimerRepository())

	return &demoFixture{
		demo: service.NewDemoService(users, projects,
			service.NewUserApplicationService(users, roles, timesheets, &memoryPurger{users: users, timesheets: timesheets}),
			projectApp,
			service.NewTimesheetApplicationService(timesheets, users, projects, maxDailyHours),
			service.NewWorkScheduleService(schedules, users),
			service.NewVacationService(memory.NewVacationRepository(), absences, users, schedules, places, clock),
			service.NewAbsenceService(absences, users, schedules, places),
			service.NewTemplateService(templates, projects),
			clock),
		users: users, projects: projects, timesheets: timesheets, absences: absences, templates: templates,
		projectApp: projectApp, admin: admin.ID,
	}
}

// The demo fills a new installation so that every screen has something to show:
// four projects, eight weeks of working days in the built-in administrator's
// account, a week of leave and a day off sick, and two templates - written in
// the language the installer was read in.
//
// And the account is given the role that records time, because as shipped it
// administers and records nothing, and the demo's hours would sit in an account
// that cannot open them.
func TestTheDemoFillsANewInstallation(t *testing.T) {
	// A Thursday, so the current week has days before it and days after it.
	today := time.Date(2026, 11, 19, 14, 30, 0, 0, time.UTC)
	f := newDemoFixture(t, today)
	ctx := context.Background()

	summary, err := f.demo.Seed(ctx, "de")
	if err != nil {
		t.Fatalf("seeding the demo: %v", err)
	}

	if summary.Skipped != "" || summary.Projects != 4 || summary.Templates != 2 || summary.Absences != 6 {
		t.Fatalf("the demo added %+v, want four projects, six days away and two templates", summary)
	}

	admin, err := f.users.GetByID(ctx, f.admin)
	if err != nil {
		t.Fatal(err)
	}

	if admin.RoleName != model.RoleUserAdmin {
		t.Errorf("the built-in administrator has the role %q, want %q so it can open its own hours",
			admin.RoleName, model.RoleUserAdmin)
	}

	owned, err := f.projects.GetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}

	for _, p := range owned {
		names[p.Name] = true

		if p.OwnerID == nil || *p.OwnerID != f.admin {
			t.Errorf("the project %q is not the built-in administrator's", p.Name)
		}
	}

	if !names["Website-Relaunch"] || !names["Kundensupport"] {
		t.Errorf("the demo's projects are %v, not written in German", names)
	}

	entries, err := f.timesheets.GetByFilter(ctx, repository.TimesheetFilter{UserID: f.admin})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != summary.Entries || len(entries) < 100 {
		t.Fatalf("%d entries are stored and %d were reported; eight weeks hold well over a hundred",
			len(entries), summary.Entries)
	}

	away, err := f.absences.Between(ctx, f.admin, today.AddDate(0, 0, -70), today)
	if err != nil {
		t.Fatal(err)
	}

	off := map[time.Time]bool{}
	for _, a := range away {
		off[a.Day] = true
	}

	perDay := map[time.Time]float64{}
	calendarToday := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

	for _, e := range entries {
		perDay[e.Date] += e.DurationHours

		switch {
		case e.Date.Weekday() == time.Saturday || e.Date.Weekday() == time.Sunday:
			t.Errorf("a booking falls on %s, a %s", e.Date.Format(time.DateOnly), e.Date.Weekday())
		case off[e.Date]:
			t.Errorf("a booking falls on %s, a day away", e.Date.Format(time.DateOnly))
		case !e.Date.Before(calendarToday):
			t.Errorf("a booking falls on %s, which is not yet over", e.Date.Format(time.DateOnly))
		}
	}

	for date, hours := range perDay {
		if hours < 7.5 || hours > 8.5 {
			t.Errorf("%s holds %.2f hours, outside an ordinary working day", date.Format(time.DateOnly), hours)
		}
	}

	kept, err := f.templates.ForUser(ctx, f.admin)
	if err != nil {
		t.Fatal(err)
	}

	if len(kept) != 2 {
		t.Errorf("%d templates were kept, want two", len(kept))
	}
}

// A database that holds projects is left exactly as it is.
//
// The installer can be pointed at a database that is already in use, and demo
// hours mixed into somebody's real ones could never be told apart again.
func TestTheDemoLeavesADatabaseInUseAlone(t *testing.T) {
	f := newDemoFixture(t, time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC))
	ctx := context.Background()

	if _, err := f.projectApp.CreateProject(ctx, command.CreateProjectCommand{
		Name: "Somebody's real work", StartDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), OwnerID: &f.admin,
	}); err != nil {
		t.Fatal(err)
	}

	summary, err := f.demo.Seed(ctx, "en")
	if err != nil {
		t.Fatalf("seeding the demo: %v", err)
	}

	if summary.Skipped == "" || summary.Projects+summary.Entries+summary.Absences+summary.Templates != 0 {
		t.Errorf("the demo went into a database in use: %+v", summary)
	}

	if all, _ := f.projects.GetAll(ctx); len(all) != 1 {
		t.Errorf("the database holds %d projects after the demo was refused, want the one it had", len(all))
	}

	if admin, _ := f.users.GetByID(ctx, f.admin); admin.RoleName != model.RoleAdmin {
		t.Errorf("the built-in administrator's role became %q although nothing was added", admin.RoleName)
	}
}
