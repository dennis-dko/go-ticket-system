package service_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

// on is a day of 2026 by month and day, for the holidays that are not in July.
func on(month time.Month, d int) time.Time {
	return time.Date(2026, month, d, 0, 0, 0, 0, time.UTC)
}

// The week of Whit Monday is four days of leave in a place that keeps it, and
// five where the installation keeps no holidays at all - because nobody chose
// any, or because somebody chose none.
func TestLeaveLeavesOutThePublicHolidays(t *testing.T) {
	for place, want := range map[string]int{"DE-BY": 4, "AT": 4, "": 5, model.NoHolidays: 5} {
		f := newPlannedFixture(t)
		f.place = place

		recorded, err := f.away.Record(context.Background(), f.userID,
			on(time.May, 25), on(time.May, 29), model.AbsenceVacation)
		if err != nil {
			t.Fatalf("record with the installation in %q: %v", place, err)
		}

		if len(recorded) != want {
			t.Errorf("Whit week with the installation in %q was recorded as %v, want %d days",
				place, daysOf(recorded), want)
		}
	}
}

// The days of a week name the holidays among them.
func TestTheDaysNameTheirHolidays(t *testing.T) {
	f := newPlannedFixture(t)
	f.place = "DE-BY"

	days, err := f.overtime.Days(context.Background(), f.userID, on(time.May, 25), on(time.May, 31))
	if err != nil {
		t.Fatalf("days: %v", err)
	}

	for _, d := range days {
		want := ""
		if d.Date.Equal(on(time.May, 25)) {
			want = "whitMonday"
		}

		if d.Holiday != want {
			t.Errorf("%s reads holiday %q, want %q", d.Date.Format(time.DateOnly), d.Holiday, want)
		}
	}
}

// A person's own choice wins over the installation's: their own place's
// holidays, or none at all.
func TestAPersonsOwnPlaceWinsOverTheInstallations(t *testing.T) {
	f := newPlannedFixture(t)
	f.place = "DE-BY"
	ctx := context.Background()

	holidayOn := func(day time.Time) string {
		t.Helper()

		days, err := f.overtime.Days(ctx, f.userID, day, day)
		if err != nil {
			t.Fatalf("days: %v", err)
		}

		return days[0].Holiday
	}

	// Corpus Christi is kept in Bavaria and not in Berlin.
	if got := holidayOn(on(time.June, 4)); got != "corpusChristi" {
		t.Errorf("following the installation, Corpus Christi reads %q", got)
	}

	if err := f.userRepo.SetPreference(ctx, f.userID, repository.PreferenceHolidayPlace, "DE-BE"); err != nil {
		t.Fatalf("choose Berlin: %v", err)
	}

	if got := holidayOn(on(time.June, 4)); got != "" {
		t.Errorf("somebody in Berlin has Corpus Christi as %q, want no holiday", got)
	}

	if err := f.userRepo.SetPreference(ctx, f.userID, repository.PreferenceHolidayPlace, model.NoHolidays); err != nil {
		t.Fatalf("choose none: %v", err)
	}

	if got := holidayOn(on(time.May, 25)); got != "" {
		t.Errorf("somebody keeping no holidays has Whit Monday as %q", got)
	}
}

// A holiday a country moves off a weekend is kept on the weekday it moves to,
// under its own name and said to stand in for it: Christmas 2027 is a Saturday,
// and England keeps it on the Monday and Boxing Day on the Tuesday.
func TestADayKeptInPlaceOfAHolidaySaysSo(t *testing.T) {
	f := newPlannedFixture(t)
	f.place = "GB-ENG"

	christmas := time.Date(2027, time.December, 24, 0, 0, 0, 0, time.UTC)

	days, err := f.overtime.Days(context.Background(), f.userID, christmas, christmas.AddDate(0, 0, 4))
	if err != nil {
		t.Fatalf("days: %v", err)
	}

	var read []string

	for _, d := range days {
		mark := d.Holiday
		if d.HolidaySubstitute {
			mark += "*"
		}

		read = append(read, d.Date.Format("02")+" "+mark)
	}

	if got, want := strings.Join(read, ", "), "24 , 25 christmasDay, 26 stStephensDay, 27 christmasDay*, 28 stStephensDay*"; got != want {
		t.Errorf("Christmas 2027 in England reads %q, want %q", got, want)
	}
}

// A choice the rules do not know is refused, for a person and for the
// installation alike, naming the half to change: the country where it is
// unknown, the region where the country is known and has no such region or
// cannot be kept without one.
func TestAPlaceTheRulesDoNotKnowIsRefusedByTheHalfToChange(t *testing.T) {
	f := newFixture(t)
	settings := service.NewSettingsService(newStubSettings(), f.roleRepo, f.userRepo, "Test")
	ctx := context.Background()

	for choice, field := range map[string]string{"XX": "country", "XX-BY": "country", "DE": "region", "DE-XX": "region", "GB": "region"} {
		for who, err := range map[string]error{
			"a person":         f.sessions.SetHolidayPlace(ctx, f.userID, choice),
			"the installation": settings.SaveHolidayPlace(ctx, choice),
		} {
			if detail, ok := apperror.Detail(err); !ok || !slices.Equal(detail.Fields, []string{field}) {
				t.Errorf("%s choosing %q answered %v, want %s refused", who, choice, err, field)
			}
		}
	}

	for _, choice := range []string{"", model.NoHolidays, "AT", "DE-BY", "FR-57"} {
		if err := f.sessions.SetHolidayPlace(ctx, f.userID, choice); err != nil {
			t.Errorf("a person choosing %q answered %v", choice, err)
		}

		if err := settings.SaveHolidayPlace(ctx, choice); err != nil {
			t.Errorf("the installation choosing %q answered %v", choice, err)
		}

		if got, err := settings.HolidayPlace(ctx); err != nil || got != choice {
			t.Errorf("the installation chose %q and keeps %q (%v)", choice, got, err)
		}
	}
}
