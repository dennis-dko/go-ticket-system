package service

import (
	"context"
	"time"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
	"github.com/dennis-dko/go-time-recording/internal/support/apperror"
)

// PlaceSource answers the installation's choice of public holidays: a place,
// model.NoHolidays, or empty where nobody has chosen. The settings answer it; a
// service built without one keeps none, which is what an installation that never
// chose anything keeps too.
type PlaceSource func(ctx context.Context) (string, error)

// holidaysOf returns the public holidays somebody keeps between two days, keyed
// by their day: their own place's, or the installation's where they follow it.
//
// One place for it, because the days a balance counts and the days an absence is
// recorded on have to agree about which days are holidays.
func holidaysOf(
	ctx context.Context,
	installation PlaceSource,
	user *model.User,
	from, to time.Time,
) (map[time.Time]model.Holiday, error) {
	var instance string

	if installation != nil {
		place, err := installation(ctx)
		if err != nil {
			return nil, err
		}

		instance = place
	}

	return model.HolidaysBetween(user.HolidayPlaceOf(instance), from, to), nil
}

// refuseHolidayChoice refuses a choice of public holidays the rules do not know,
// naming the half of it that has to change.
func refuseHolidayChoice(choice string) error {
	switch model.HolidayChoiceFaultOf(choice) {
	case model.UnknownHolidayCountry:
		return apperror.InvalidFields("country")
	case model.UnknownHolidayRegion:
		return apperror.InvalidFields("region")
	default:
		return nil
	}
}
