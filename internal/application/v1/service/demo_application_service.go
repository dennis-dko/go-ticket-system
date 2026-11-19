package service

import (
	"context"
	"fmt"
	"time"

	"github.com/dennis-dko/timepulse/internal/application/v1/command"
	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/domain/repository"
	"github.com/dennis-dko/timepulse/internal/support/i18n"
)

// DemoService fills a new installation with demo data, where whoever answered
// the installer asked for it, so that every screen has something to show from
// the first sign-in.
//
// Everything goes through the services the interface uses, so the demo obeys
// every rule somebody's own bookings do - a day's cap, a project that has to be
// active, leave that takes a working day off - and it cannot make a state the
// application could not have made itself.
//
// Into the built-in administrator's account, because that is the account the
// browser which answered the installer is signed into: the demo is on screen the
// moment the installation is. As shipped that account administers and records no
// time, so it is given the role that does both, which is what an installation
// gives somebody who has both jobs. No second account is made for it, because an
// account with a password somebody could read would outlive the demo.
type DemoService struct {
	users      repository.UserRepository
	projects   repository.ProjectRepository
	accounts   *UserApplicationService
	projectApp *ProjectApplicationService
	entries    *TimesheetApplicationService
	weeks      *WorkScheduleService
	leave      *VacationService
	absences   *AbsenceService
	templates  *TemplateService
	today      Today
}

// NewDemoService is handed the services the interface books through, so the demo
// is held to what they hold anybody to.
func NewDemoService(
	users repository.UserRepository,
	projects repository.ProjectRepository,
	accounts *UserApplicationService,
	projectApp *ProjectApplicationService,
	entries *TimesheetApplicationService,
	weeks *WorkScheduleService,
	leave *VacationService,
	absences *AbsenceService,
	templates *TemplateService,
	today Today,
) *DemoService {
	return &DemoService{
		users: users, projects: projects, accounts: accounts, projectApp: projectApp,
		entries: entries, weeks: weeks, leave: leave, absences: absences, templates: templates,
		today: today,
	}
}

// DemoSummary is what a seed added, for the log.
type DemoSummary struct {
	Projects, Entries, Absences, Templates int

	// Skipped says why nothing was added, and is empty when the demo went in.
	Skipped string
}

// demoWeeks is how far back the bookings reach: long enough for the week, the
// month and the overtime to have something to show, short enough to read.
const demoWeeks = 8

// demoProject is one of the demo's projects: what it is called and is about,
// what an hour of it is charged at and how many hours it is planned with - zero
// for neither - and the work booked on it. Each text is in the dictionaries
// under demo.<key>, with its English here.
type demoProject struct {
	key, name, description string
	rate, budget           float64
	work                   []string
}

// demoProjects are the demo's four, in the order demoDay refers to them by.
var demoProjects = []demoProject{
	{key: "website", name: "Website relaunch", description: "A new website with a shop", rate: 95, budget: 400,
		work: []string{"Home page concept", "Build components", "Client check-in #meeting", "Accessibility review",
			"Move the content over"}},
	{key: "mobile", name: "Mobile app", description: "An app for the field team", rate: 110, budget: 300,
		work: []string{"Sign-in flow", "Offline mode", "Android bug hunt", "Sprint planning #meeting",
			"Prepare the release"}},
	{key: "support", name: "Customer support", description: "Requests and tickets", rate: 70,
		work: []string{"Work through tickets", "Call a customer back", "Extend the knowledge base"}},
	{key: "internal", name: "Internal tooling", description: "Our own tools and routines",
		work: []string{"Look after the build pipeline", "Code review", "Documentation"}},
}

const (
	demoWebsite = iota
	demoMobile
	demoSupport
	demoInternal
)

// demoStandup is the day's first booking, every day, on the internal project.
const demoStandup = "Stand-up #meeting"

// demoBooking is one entry of a demo day.
type demoBooking struct {
	project     int
	hours       float64
	description string
}

// Seed fills the database with demo data written in language, around the built-
// in administrator's today, and says what it added.
//
// Only into a database that holds no projects: one that does is being worked
// in, and demo hours mixed into somebody's real ones could never be told apart
// again.
func (s *DemoService) Seed(ctx context.Context, language string) (DemoSummary, error) {
	existing, err := s.projects.GetAll(ctx)
	if err != nil {
		return DemoSummary{}, fmt.Errorf("asking whether the database is empty: %w", err)
	}

	if len(existing) > 0 {
		return DemoSummary{Skipped: "the database already holds projects"}, nil
	}

	admin, err := s.users.GetByEmail(ctx, SystemUserEmail)
	if err != nil {
		return DemoSummary{}, fmt.Errorf("finding the built-in administrator: %w", err)
	}

	role := model.RoleUserAdmin
	if _, err := s.accounts.UpdateUser(ctx, command.UpdateUserCommand{ID: admin.ID, Role: &role}); err != nil {
		return DemoSummary{}, fmt.Errorf("giving the built-in administrator the role that records time: %w", err)
	}

	words := i18n.For(language)
	now := s.today(ctx, admin)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monday := day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	start := monday.AddDate(0, 0, -7*demoWeeks)

	if _, err := s.weeks.Plan(ctx, admin.ID, start, model.WeekHours{8, 8, 8, 8, 8, 0, 0}); err != nil {
		return DemoSummary{}, fmt.Errorf("planning the week: %w", err)
	}

	if err := s.leave.Entitle(ctx, admin.ID, day.Year(), 30); err != nil {
		return DemoSummary{}, fmt.Errorf("granting the year's leave: %w", err)
	}

	var summary DemoSummary

	ids := make([]uint, len(demoProjects))

	for i, p := range demoProjects {
		created, err := s.projectApp.CreateProject(ctx, demoProjectCommand(p, words, admin.ID, start, day))
		if err != nil {
			return summary, fmt.Errorf("creating the project %q: %w", p.name, err)
		}

		ids[i] = created.Result.ID
		summary.Projects++
	}

	away := map[time.Time]bool{}

	for _, absence := range []struct {
		from, to time.Time
		kind     string
	}{
		{from: monday.AddDate(0, 0, -21), to: monday.AddDate(0, 0, -17), kind: model.AbsenceVacation},
		{from: monday.AddDate(0, 0, -40), to: monday.AddDate(0, 0, -40), kind: model.AbsenceSick},
	} {
		recorded, err := s.absences.Record(ctx, admin.ID, absence.from, absence.to, absence.kind)
		if err != nil {
			return summary, fmt.Errorf("recording the %s: %w", absence.kind, err)
		}

		for _, a := range recorded {
			away[a.Day] = true
			summary.Absences++
		}
	}

	booked := 0

	for d := start; d.Before(day); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || away[d] {
			continue
		}

		for _, b := range demoDay(booked, int(d.Sub(start).Hours()/24/7)) {
			description := demoText(words, b.project, b.description)

			if _, err := s.entries.CreateTimesheet(ctx, command.CreateTimesheetCommand{
				UserID: admin.ID, ProjectID: ids[b.project], Date: d,
				DurationHours: b.hours, Description: &description,
			}); err != nil {
				return summary, fmt.Errorf("booking %s on %s: %w", description, d.Format(time.DateOnly), err)
			}

			summary.Entries++
		}

		booked++
	}

	for _, template := range []demoBooking{
		{project: demoInternal, hours: 0.25, description: demoStandup},
		{project: demoSupport, hours: 2, description: demoProjects[demoSupport].work[0]},
	} {
		description := demoText(words, template.project, template.description)
		hours := template.hours

		if _, err := s.templates.Keep(ctx, model.EntryTemplate{
			UserID: admin.ID, ProjectID: &ids[template.project], DurationHours: &hours, Description: &description,
		}); err != nil {
			return summary, fmt.Errorf("keeping the template %q: %w", description, err)
		}

		summary.Templates++
	}

	return summary, nil
}

// demoProjectCommand is one of the demo's projects as the form would create it,
// begun four weeks before the first booking so that every booking falls inside it.
func demoProjectCommand(p demoProject, words i18n.Translator, owner uint, start, today time.Time) command.CreateProjectCommand {
	description := words.Text("demo."+p.key+".description", p.description)

	cmd := command.CreateProjectCommand{
		Name:        words.Text("demo."+p.key+".name", p.name),
		Description: &description,
		StartDate:   start.AddDate(0, 0, -28),
		Status:      model.ProjectStatusActive,
		OwnerID:     &owner,
		Today:       today,
		Billable:    p.rate > 0,
	}

	if p.rate > 0 {
		rate := p.rate
		cmd.HourlyRate = &rate
	}

	if p.budget > 0 {
		budget := p.budget
		cmd.BudgetHours = &budget
	}

	return cmd
}

// demoDay is the n-th booked day of the demo, in week w of it: a stand-up, a
// block on the week's main project, one on support and the rest of the day
// elsewhere, adding up to between seven and a half and eight and a half hours.
//
// Worked out from n rather than drawn at random, so two installations seeded on
// the same day hold the same demo, and a test can say what it is. The bounds are
// chosen so that the rest is never less than three quarters of an hour.
func demoDay(n, w int) []demoBooking {
	target := []float64{8, 7.5, 8.5, 8, 7.75, 8.25, 8}[n%7]
	main, other := demoWebsite, demoMobile

	if w%2 == 1 {
		main, other = demoMobile, demoWebsite
	}

	mainHours := []float64{3.5, 4, 4.5}[n%3]
	supportHours := []float64{1, 1.5, 2}[(n/3)%3]
	rest := target - 0.25 - mainHours - supportHours

	last := other
	if n%2 == 1 {
		last = demoInternal
	}

	pick := func(project, offset int) string {
		work := demoProjects[project].work

		return work[(n+offset)%len(work)]
	}

	return []demoBooking{
		{project: demoInternal, hours: 0.25, description: demoStandup},
		{project: main, hours: mainHours, description: pick(main, 0)},
		{project: demoSupport, hours: supportHours, description: pick(demoSupport, 0)},
		{project: last, hours: rest, description: pick(last, 1)},
	}
}

// demoText is one of the demo's English texts in the language being written: the
// stand-up under demo.standup, a project's work under demo.<project>.<n>.
func demoText(words i18n.Translator, project int, english string) string {
	if english == demoStandup {
		return words.Text("demo.standup", english)
	}

	p := demoProjects[project]

	for i, work := range p.work {
		if work == english {
			return words.Text(fmt.Sprintf("demo.%s.%d", p.key, i+1), english)
		}
	}

	return english
}
