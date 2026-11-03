package cli

import (
	"context"
	"net/http"
	"time"
)

// account is the token's owner as /me answers: who they are, where they keep
// their days, and what they set for themselves - which the screen follows, and
// offers to change.
//
// Read with the day, so that a choice made in the browser reaches an open
// screen at its next read, the way a stopwatch stopped there does.
type account struct {
	id   uint
	name string

	// zone is where they keep their days: their own, or the installation's where
	// they chose none, worked out by the installation so that a day here is the
	// day the browser would show them.
	zone *time.Location

	// language and theme are their choices, empty for none.
	language, theme string

	// timezone is the zone they chose, empty for following the installation's,
	// which installationZone names.
	timezone, installationZone string

	// unbookedDay and alertHours are the reminders: whether the last working day
	// is mentioned when nothing is booked on it, and after how many hours a
	// running stopwatch is, zero for never.
	unbookedDay bool
	alertHours  int

	// targetHours and maxHours are their working hours, zero for the
	// installation's, and countsFrom the day from which every working day counts,
	// empty for none.
	targetHours, maxHours float64
	countsFrom            string

	permissions []string
}

// account reads the token's owner.
func (s *session) account(ctx context.Context) (account, error) {
	var me struct {
		User struct {
			ID                 uint    `json:"id"`
			Name               string  `json:"name"`
			Language           string  `json:"language"`
			Theme              string  `json:"theme"`
			Timezone           string  `json:"timezone"`
			EffectiveTimezone  string  `json:"effectiveTimezone"`
			InstanceTimezone   string  `json:"instanceTimezone"`
			DailyTargetHours   float64 `json:"dailyTargetHours"`
			MaxDailyHours      float64 `json:"maxDailyHours"`
			EveryDayCountsFrom *string `json:"everyDayCountsFrom"`
		} `json:"user"`
		Permissions []string `json:"permissions"`
		Reminders   struct {
			UnbookedDay     bool `json:"unbookedDay"`
			TimerAlertHours int  `json:"timerAlertHours"`
		} `json:"reminders"`
	}

	if err := s.api.call(ctx, http.MethodGet, "/me", nil, &me); err != nil {
		return account{}, err
	}

	u := me.User

	zone, err := time.LoadLocation(u.EffectiveTimezone)
	if err != nil || u.EffectiveTimezone == "" {
		zone = time.UTC
	}

	owner := account{
		id: u.ID, name: u.Name, zone: zone,
		language: u.Language, theme: u.Theme,
		timezone: u.Timezone, installationZone: u.InstanceTimezone,
		unbookedDay: me.Reminders.UnbookedDay, alertHours: me.Reminders.TimerAlertHours,
		targetHours: u.DailyTargetHours, maxHours: u.MaxDailyHours,
		permissions: me.Permissions,
	}

	if u.EveryDayCountsFrom != nil {
		owner.countsFrom = *u.EveryDayCountsFrom
	}

	return owner, nil
}
