package rest

import (
	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

// MeHandler serves the endpoints about the signed-in user: who they are, what
// they may do, and their password.
type MeHandler struct {
	auth     *service.AuthService
	sessions *service.SessionService
	authz    *Authorizer
	timezone InstanceTimezoneFunc
}

// NewMeHandler creates the handler.
func NewMeHandler(
	auth *service.AuthService,
	sessions *service.SessionService,
	authz *Authorizer,
	timezone InstanceTimezoneFunc,
) *MeHandler {
	return &MeHandler{auth: auth, sessions: sessions, authz: authz, timezone: timezone}
}

// Me handles GET /api/v1/me. The UI uses it to decide what to render, so it
// deliberately stays reachable while a password change is still pending.
func (h *MeHandler) Me(c *gofr.Context) (any, error) {
	principal, err := h.authz.Principal(c)
	if err != nil {
		return nil, err
	}

	permissions := principal.Permissions
	if permissions == nil {
		permissions = []string{}
	}

	return MeResponse{
		User:        newUserResponseFromModel(principal.User, h.timezone.resolve(c)),
		Permissions: permissions,
		AuthEnabled: h.authz.Enabled(),

		// The baseline the interface compares every later response's
		// X-Permissions-Revision against, so it can say that the rights changed
		// rather than only meeting refusals on controls it is still showing.
		PermissionsRevision: PermissionRevision(principal),
	}, nil
}

// ChangePassword handles PUT /api/v1/me/password.
func (h *MeHandler) ChangePassword(c *gofr.Context) (any, error) {
	principal, err := h.authz.Principal(c)
	if err != nil {
		return nil, err
	}

	if !h.authz.Enabled() {
		return nil, toHTTPError(apperror.Conflictf(
			"this instance runs without authentication, so there is no password to change").
			WithCode("noAuthNoPassword"))
	}

	var req ChangePasswordRequest
	if err := bind(c, &req); err != nil {
		return nil, toHTTPError(err)
	}

	err = h.auth.ChangePassword(c, principal.User.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		return nil, toHTTPError(err)
	}

	// Every other session is ended, so one opened with the old password on another
	// device stops working immediately - and this one is not, because it is the
	// device that just proved it knew the old password. Ending it as well signed
	// somebody out of the setup wizard between two of its steps.
	var token string
	if cookie, err := requestOf(c).Cookie(SessionCookieName); err == nil {
		token = cookie.Value
	}

	if err := h.sessions.LogoutOthers(c, principal.User.ID, token); err != nil {
		return nil, toHTTPError(err)
	}

	return map[string]string{"status": "password changed"}, nil
}
