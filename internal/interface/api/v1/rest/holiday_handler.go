package rest

import (
	"strings"

	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
)

// HolidayHandler answers which public holidays the installation keeps: the days
// its service hours leave out, set by whoever administers it.
type HolidayHandler struct {
	settings *service.SettingsService
	authz    *Authorizer
}

// NewHolidayHandler creates the handler.
func NewHolidayHandler(settings *service.SettingsService, authz *Authorizer) *HolidayHandler {
	return &HolidayHandler{settings: settings, authz: authz}
}

// HolidayChoiceRequest names where public holidays are kept: a country by its
// ISO 3166-1 code, with its region after a dash where it has one - AT, DE-BY -
// or "none", or empty.
type HolidayChoiceRequest struct {
	Place string `json:"place"`
}

// HolidayChoiceResponse is the choice stored.
type HolidayChoiceResponse struct {
	Place string `json:"place"`
}

// HolidayCountryResponse is a country whose public holidays can be kept: its
// ISO 3166-1 code, the regions that keep holidays of their own by the rest of
// their ISO 3166-2 code, and whether one has to be named.
type HolidayCountryResponse struct {
	Code           string   `json:"code"`
	Regions        []string `json:"regions"`
	RegionRequired bool     `json:"regionRequired"`
}

// InstallationHolidaysResponse is the installation's choice, and what it could be.
type InstallationHolidaysResponse struct {
	// Place is the choice: a place, "none", or empty while nobody has chosen.
	Place     string                   `json:"place"`
	Countries []HolidayCountryResponse `json:"countries"`
}

// holidayCountries is every country a choice can name.
func holidayCountries() []HolidayCountryResponse {
	known := model.HolidayCountries()
	countries := make([]HolidayCountryResponse, 0, len(known))

	for _, country := range known {
		regions := country.Regions
		if regions == nil {
			regions = []string{}
		}

		countries = append(countries, HolidayCountryResponse{
			Code: country.Code, Regions: regions, RegionRequired: country.RegionRequired,
		})
	}

	return countries
}

// Installation handles GET /api/v1/settings/holidays.
func (h *HolidayHandler) Installation(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	choice, err := h.settings.HolidayPlace(c)
	if err != nil {
		return nil, toHTTPError(err)
	}

	return InstallationHolidaysResponse{Place: choice, Countries: holidayCountries()}, nil
}

// SetInstallation handles PUT /api/v1/settings/holidays.
func (h *HolidayHandler) SetInstallation(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	var req HolidayChoiceRequest
	if err := bind(c, &req); err != nil {
		return nil, toHTTPError(err)
	}

	choice := strings.TrimSpace(req.Place)

	if err := h.settings.SaveHolidayPlace(c, choice); err != nil {
		return nil, toHTTPError(err)
	}

	return HolidayChoiceResponse{Place: choice}, nil
}
