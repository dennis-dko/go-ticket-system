package rest

import (
	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/support/webpush"
)

// PushHandler serves Web Push: the installation's switch, and the browsers
// somebody subscribes to have their reminders pushed to.
type PushHandler struct {
	push  *service.PushService
	authz *Authorizer
}

// NewPushHandler creates the handler.
func NewPushHandler(push *service.PushService, authz *Authorizer) *PushHandler {
	return &PushHandler{push: push, authz: authz}
}

// PushSwitch is whether the installation pushes reminders.
type PushSwitch struct {
	Enabled bool `json:"enabled"`
}

// PushState is what a page needs to subscribe the browser it is open in: whether
// the installation pushes, the key to subscribe with, and the addresses of the
// caller's own subscribed browsers, to tell whether its own is among them.
type PushState struct {
	Enabled   bool     `json:"enabled"`
	PublicKey string   `json:"publicKey"`
	Endpoints []string `json:"endpoints"`
}

// PushKeys are the two keys of a browser's push channel.
type PushKeys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}

// PushSubscriptionRequest is a browser's subscription as the browser writes it
// out itself, so a page hands it over unchanged.
type PushSubscriptionRequest struct {
	Endpoint string   `json:"endpoint"`
	Keys     PushKeys `json:"keys"`
}

// PushEndpointRequest names one subscribed browser by its push channel's address.
type PushEndpointRequest struct {
	Endpoint string `json:"endpoint"`
}

// Switch handles GET /api/v1/settings/push.
func (h *PushHandler) Switch(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	enabled, err := h.push.Enabled(c)
	if err != nil {
		return nil, toHTTPError(err)
	}

	return PushSwitch{Enabled: enabled}, nil
}

// SaveSwitch handles PUT /api/v1/settings/push.
//
// The installation's to decide, and shipped off: every push leaves through the
// push service of the browser's maker, which is a decision about whom this
// installation talks to rather than about anybody's reminders.
func (h *PushHandler) SaveSwitch(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	var req PushSwitch
	if err := bind(c, &req); err != nil {
		return nil, toHTTPError(err)
	}

	if err := h.push.SetEnabled(c, req.Enabled); err != nil {
		return nil, toHTTPError(err)
	}

	return req, nil
}

// State handles GET /api/v1/me/push.
func (h *PushHandler) State(c *gofr.Context) (any, error) {
	principal, err := h.authz.Require(c, model.PermTimesheetReadOwn)
	if err != nil {
		return nil, err
	}

	key, err := h.push.PublicKey(c)
	if err != nil {
		return nil, toHTTPError(err)
	}

	endpoints, err := h.push.Endpoints(c, principal.User.ID)
	if err != nil {
		return nil, toHTTPError(err)
	}

	return PushState{Enabled: key != "", PublicKey: key, Endpoints: endpoints}, nil
}

// Subscribe handles POST /api/v1/me/push: the browser the page is open in, to
// have the caller's reminders pushed to. The address it is signed with is the one
// the browser reached the installation at, as a token's code holds it.
func (h *PushHandler) Subscribe(c *gofr.Context) (any, error) {
	principal, err := h.authz.Require(c, model.PermTimesheetReadOwn)
	if err != nil {
		return nil, err
	}

	var req PushSubscriptionRequest
	if err := bind(c, &req); err != nil {
		return nil, toHTTPError(err)
	}

	origin := ""
	if request := requestOf(c); request != nil {
		origin = addressAsSeen(request)
	}

	sub := webpush.Subscription{Endpoint: req.Endpoint, P256dh: req.Keys.P256dh, Auth: req.Keys.Auth, Origin: origin}
	if err := h.push.Subscribe(c, principal.User.ID, sub); err != nil {
		return nil, toHTTPError(err)
	}

	return h.State(c)
}

// Unsubscribe handles DELETE /api/v1/me/push: one of the caller's browsers, no
// longer pushed to. Signed in is all it asks, because no right withdrawn since
// may keep somebody from taking a browser of theirs back.
func (h *PushHandler) Unsubscribe(c *gofr.Context) (any, error) {
	principal, err := h.authz.Principal(c)
	if err != nil {
		return nil, err
	}

	var req PushEndpointRequest
	if err := bind(c, &req); err != nil {
		return nil, toHTTPError(err)
	}

	if err := h.push.Unsubscribe(c, principal.User.ID, req.Endpoint); err != nil {
		return nil, toHTTPError(err)
	}

	return PushEndpointRequest{Endpoint: req.Endpoint}, nil
}
