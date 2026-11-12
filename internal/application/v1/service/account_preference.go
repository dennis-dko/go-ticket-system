package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

// SetLanguage stores the user's interface language.
func (s *SessionService) SetLanguage(ctx context.Context, userID uint, language string) error {
	if !model.IsSupportedLanguage(language) {
		return apperror.InvalidFields("language")
	}

	return s.users.SetPreference(ctx, userID, repository.PreferenceLanguage, language)
}

// SetTheme stores the appearance this person reads in, or clears it.
//
// An empty value is the normal case rather than an omission: it means "follow
// the time of day", which is what somebody who has never chosen gets and what
// they go back to by choosing automatic.
func (s *SessionService) SetTheme(ctx context.Context, userID uint, theme string) error {
	if !model.IsSupportedTheme(theme) {
		return apperror.InvalidFields("theme")
	}

	return s.users.SetPreference(ctx, userID, repository.PreferenceTheme, theme)
}

// SetTourSeen records whether this person has been shown the guided tour.
//
// Settable both ways: someone who wants to see it again should be able to ask,
// rather than being told they have already had their chance.
func (s *SessionService) SetTourSeen(ctx context.Context, userID uint, seen bool) error {
	// One column, not the whole row: this is written the moment somebody signs in,
	// while they are already doing something else. Writing the row back would take
	// whatever that something else changed with it.
	return s.users.SetPreference(ctx, userID, repository.PreferenceTourSeen,
		strconv.FormatBool(seen))
}

// SetTimezone stores the user's own zone, or clears it so they follow the
// instance setting again.
//
// An empty name is the normal case and is deliberately allowed: most people
// should move with the instance rather than be pinned to whatever zone they
// happened to be in when the account was made.
func (s *SessionService) SetTimezone(ctx context.Context, userID uint, timezone string) error {
	timezone = strings.TrimSpace(timezone)

	if timezone != "" && !model.IsSupportedTimezone(timezone) {
		return apperror.InvalidFields("timezone")
	}

	err := s.users.SetPreference(ctx, userID, repository.PreferenceTimezone, timezone)

	return err
}

// SetHolidayPlace stores where this person keeps public holidays: a place,
// empty to follow the installation, or model.NoHolidays for none.
func (s *SessionService) SetHolidayPlace(ctx context.Context, userID uint, place string) error {
	place = strings.TrimSpace(place)

	if err := refuseHolidayChoice(place); err != nil {
		return err
	}

	return s.users.SetPreference(ctx, userID, repository.PreferenceHolidayPlace, place)
}

// SetReminders keeps both of somebody's reminder choices together: whether the
// last working day is mentioned when nothing is booked on it, and after how many
// hours a running stopwatch is - zero for never, and at most a day, past which the
// stopwatch already says it holds more than one entry can.
func (s *SessionService) SetReminders(ctx context.Context, userID uint, unbookedDay bool, timerAlertHours int) error {
	if timerAlertHours < 0 || timerAlertHours > model.MaxTimerAlertHours {
		return apperror.InvalidFields("timerAlertHours")
	}

	return s.users.SetReminders(ctx, userID, !unbookedDay, &timerAlertHours)
}
