package service

import (
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

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
