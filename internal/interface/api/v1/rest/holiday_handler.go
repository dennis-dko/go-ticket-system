package rest

import (
	"strings"

	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/go-time-recording/internal/application/v1/service"
	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// HolidayHandler answers whose public holidays apply: the installation's, set by
// whoever administers it, and each person's own, set by them.
//
// The two halves of one question, which is why they share a handler: the
// timezone works the same way, an installation-wide answer and a personal one
// that overrides it.
//
// Every answer says the choice twice. Place is how it is written now that there
// are countries - an ISO 3166-2 code, DE-BY - and the region fields are how it
// was written before, a German state's code alone, kept for any client written
// then: a field somebody reads is part of the contract, and dropping it would
// leave such a client reading no holidays where there are some.
type HolidayHandler struct {
	sessions *service.SessionService
	settings *service.SettingsService
	authz    *Authorizer
}

// NewHolidayHandler creates the handler.
func NewHolidayHandler(
	sessions *service.SessionService,
	settings *service.SettingsService,
	authz *Authorizer,
) *HolidayHandler {
	return &HolidayHandler{sessions: sessions, settings: settings, authz: authz}
}

// HolidayChoiceRequest names where public holidays are kept.
type HolidayChoiceRequest struct {
	// Place is a country by its ISO 3166-1 code, with its region after a dash
	// where it has one - AT, DE-BY - or "none", or empty.
	Place *string `json:"place"`

	// Region is the same choice written as it was before there were countries,
	// read only where Place is absent: a German state's code alone, "none", or
	// empty.
	Region string `json:"region"`
}

// choice is the place asked for, whichever way it was written.
func (r HolidayChoiceRequest) choice() string {
	if r.Place != nil {
		return strings.TrimSpace(*r.Place)
	}

	return model.LegacyHolidayPlace(strings.TrimSpace(r.Region))
}

// HolidayChoiceResponse is the choice stored, both ways it is written.
type HolidayChoiceResponse struct {
	Place  string `json:"place"`
	Region string `json:"region"`
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

	// Region and Regions are the choice and the German states as they were
	// written before there were countries, when the installation said none by
	// being empty.
	Region  string   `json:"region"`
	Regions []string `json:"regions"`
}

// MyHolidaysResponse is whose holidays somebody keeps, and what they could choose.
type MyHolidaysResponse struct {
	// Place is their own choice: empty follows the installation, "none" keeps
	// none.
	Place string `json:"place"`

	// InstancePlace is the installation's choice, for saying what following it
	// means.
	InstancePlace string `json:"instancePlace"`

	// EffectivePlace is what applies, empty for none.
	EffectivePlace string `json:"effectivePlace"`

	Countries []HolidayCountryResponse `json:"countries"`

	// The same, written as before there were countries.
	Region          string   `json:"region"`
	InstanceRegion  string   `json:"instanceRegion"`
	EffectiveRegion string   `json:"effectiveRegion"`
	Regions         []string `json:"regions"`
}

// holidayCountries is every country a choice can name, and the German states on
// their own, which were every choice there was before.
func holidayCountries() (countries []HolidayCountryResponse, germanStates []string) {
	for _, country := range model.HolidayCountries() {
		regions := country.Regions
		if regions == nil {
			regions = []string{}
		}

		countries = append(countries, HolidayCountryResponse{
			Code: country.Code, Regions: regions, RegionRequired: country.RegionRequired,
		})

		if country.Code == "DE" {
			germanStates = regions
		}
	}

	return countries, germanStates
}

// installationRegion writes the installation's choice as it was written before
// there were countries, when the installation said none by being empty.
func installationRegion(choice string) string {
	if choice == model.NoHolidays {
		return ""
	}

	return model.LegacyHolidayRegion(choice)
}

// Mine handles GET /api/v1/me/holidays.
func (h *HolidayHandler) Mine(c *gofr.Context) (any, error) {
	principal, err := h.authz.Principal(c)
	if err != nil {
		return nil, err
	}

	instance, err := h.settings.HolidayPlace(c)
	if err != nil {
		return nil, toHTTPError(err)
	}

	effective := principal.User.HolidayPlaceOf(instance)
	countries, germanStates := holidayCountries()

	return MyHolidaysResponse{
		Place:           principal.User.HolidayPlace,
		InstancePlace:   instance,
		EffectivePlace:  effective,
		Countries:       countries,
		Region:          model.LegacyHolidayRegion(principal.User.HolidayPlace),
		InstanceRegion:  installationRegion(instance),
		EffectiveRegion: model.LegacyHolidayRegion(effective),
		Regions:         germanStates,
	}, nil
}

// SetMine handles PUT /api/v1/me/holidays.
func (h *HolidayHandler) SetMine(c *gofr.Context) (any, error) {
	principal, err := h.authz.Principal(c)
	if err != nil {
		return nil, err
	}

	var req HolidayChoiceRequest
	if err := bind(c, &req); err != nil {
		return nil, toHTTPError(err)
	}

	choice := req.choice()

	if err := h.sessions.SetHolidayPlace(c, principal.User.ID, choice); err != nil {
		return nil, toHTTPError(err)
	}

	return HolidayChoiceResponse{Place: choice, Region: model.LegacyHolidayRegion(choice)}, nil
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

	countries, germanStates := holidayCountries()

	return InstallationHolidaysResponse{
		Place:     choice,
		Countries: countries,
		Region:    installationRegion(choice),
		Regions:   germanStates,
	}, nil
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

	choice := req.choice()

	if err := h.settings.SaveHolidayPlace(c, choice); err != nil {
		return nil, toHTTPError(err)
	}

	return HolidayChoiceResponse{Place: choice, Region: installationRegion(choice)}, nil
}
