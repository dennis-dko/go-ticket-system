package model_test

import (
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/domain/model"
)

// A schedule names the minutes it says, in the zone of the moment it is asked
// about, and no others.
//
// It is read the way the framework's scheduler reads one, so a backup scheduled
// with the same expression as a directory run starts at the same minute: a day of
// the month and a day of the week are alternatives when both are given, and a star
// in one of them leaves only the other.
func TestAScheduleNamesTheMinutesItSays(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("no zone database here: %v", err)
	}

	// Thursday the 19th of November 2026.
	at := func(day, hour, minute int) time.Time {
		return time.Date(2026, 11, day, hour, minute, 0, 0, berlin)
	}

	for _, c := range []struct {
		schedule string
		at       time.Time
		names    bool
	}{
		{"0 3 * * *", at(19, 3, 0), true},
		{"0 3 * * *", at(19, 3, 1), false},
		{"0 3 * * *", at(19, 4, 0), false},
		{"0 3 * * *", time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC).In(berlin), false},
		{"*/15 * * * *", at(19, 10, 45), true},
		{"*/15 * * * *", at(19, 10, 50), false},
		{"5/20 * * * *", at(19, 10, 45), true},
		{"5/20 * * * *", at(19, 10, 5), true},
		{"5/20 * * * *", at(19, 10, 15), false},
		{"0 9-17/4 * * *", at(19, 13, 0), true},
		{"0 9-17/4 * * *", at(19, 15, 0), false},
		{"30 2 * * 0,4", at(19, 2, 30), true},
		{"30 2 * * 1-3", at(19, 2, 30), false},
		{"0 0 1 * *", at(19, 0, 0), false},
		{"0 0 19 * *", at(19, 0, 0), true},
		{"0 0 1 * 4", at(19, 0, 0), true},
		{"0 0 19 * 1", at(19, 0, 0), true},
		{"0 0 1 * 1", at(19, 0, 0), false},
		{"0 0 * 12 *", at(19, 0, 0), false},
		{"", at(19, 3, 0), false},
		{"0 25 * * *", at(19, 3, 0), false},
	} {
		if got := model.ScheduleMatches(c.schedule, c.at); got != c.names {
			t.Errorf("%q at %s: %v, want %v", c.schedule, c.at.Format("Mon 2 Jan 15:04 MST"), got, c.names)
		}
	}
}
