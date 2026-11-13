package rest

import (
	"net/http"

	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

// Authorizer resolves the caller behind a request and checks their rights.
//
// Authorization is enforced per handler rather than by a route table, because
// several rules depend on the resource: whether a ticket may be read is decided
// by the queue it is in, which is not known until the ticket has been read.
type Authorizer struct {
	// open disables enforcement. It exists for running the instance without
	// authentication at all, where there is no caller to check.
	open bool
}

// NewAuthorizer creates an authorizer. When enforce is false every request is
// allowed and the caller is reported as anonymous.
func NewAuthorizer(enforce bool) *Authorizer {
	return &Authorizer{open: !enforce}
}

// Enabled reports whether authorization is being enforced.
func (a *Authorizer) Enabled() bool { return !a.open }

// forbiddenError renders a 403. GoFr has no error type for it, and reusing a
// 400 or 404 would hide the reason from the client.
//
// It carries the same code and values every other refusal does, for the same
// reason: "missing permission: settings:manage" is a sentence for a log, and the
// person who read it had chosen German. It is also the one refusal the interface
// has to act on rather than only show - a 403 where the screen still offers the
// button means the rights changed underneath somebody.
type forbiddenError struct {
	msg        string
	code       string
	codeValues []any
}

func (e forbiddenError) Error() string { return e.msg }

// WithCode names the reason, mirroring apperror.Error.WithCode so a refusal is
// annotated the same way whichever layer raised it - and so the one test that
// checks every code has a German sentence can find these too.
func (e forbiddenError) WithCode(code string, values ...any) forbiddenError {
	e.code, e.codeValues = code, values

	return e
}

// StatusCode is what GoFr's responder reads to pick the HTTP status.
func (forbiddenError) StatusCode() int { return http.StatusForbidden }

// Response is GoFr's ResponseMarshaller, which merges this into the error body.
func (e forbiddenError) Response() map[string]any {
	return reason{code: e.code, values: e.codeValues}.response()
}

// Principal returns the authenticated caller.
//
// With enforcement off there is no caller, so a synthetic principal holding
// every permission of the staff is returned; that keeps handlers free of special
// cases. Not the customer's: without anybody signed in there is no contact the
// portal could show the tickets of.
func (a *Authorizer) Principal(c *gofr.Context) (*service.Principal, error) {
	if a.open {
		return &service.Principal{
			User:        &model.User{Name: "anonymous"},
			Permissions: model.AdminPermissions(),
		}, nil
	}

	principal, ok := principalFromContext(c.Request.Context())
	if !ok || principal.User == nil {
		// Not "not signed in" when what was presented could not be checked: that
		// answer sends the screen to the sign-in form over a session that may be
		// perfectly good, and tells a script its token is invalid.
		if err, unchecked := c.Request.Context().Value(uncheckedCallerKey{}).(error); unchecked {
			return nil, toHTTPError(err)
		}

		return nil, unauthorizedError{}
	}

	return principal, nil
}

// unauthorizedError renders a 401, which tells the UI to show the sign-in
// form. A 403 would instead mean "signed in, but not allowed".
type unauthorizedError struct{}

func (unauthorizedError) Error() string { return "not authenticated" }

// StatusCode is what GoFr's responder reads to pick the HTTP status.
func (unauthorizedError) StatusCode() int { return http.StatusUnauthorized }

// Response names it, like every other refusal. This one had no code for as long
// as it existed, on the reasoning that a 401 is unambiguous - and it is, to
// something reading the status. What reads the body is a person, and they were
// reading "not authenticated" in English on a German screen.
func (unauthorizedError) Response() map[string]any {
	return reason{code: apperror.CodeUnauthenticated}.response()
}

// RequireInstallationAdmin returns the caller only if they may configure the
// installation: the built-in administrator, or somebody holding
// model.PermSettingsManage.
//
// Everything behind here - the database connection, the directory bind, the
// mailboxes, the process log - describes the installation rather than the work
// done in it, and a role that can be edited into holding the right is a way to
// grant yourself the installation: roles:write is the installation as well,
// because whoever holds it can tick settings:manage on a role and assign it.
// Nothing here prevents that, and pretending otherwise would be worse than
// saying it - so it is said here, where the next reader meets it.
//
// What makes it worth paying: an account that administers the accounts but
// cannot reach the database screen is half an administrator, and the way people
// work around that is to sign in as the built-in account, which is the one
// account whose actions are hardest to attribute to a person.
func (a *Authorizer) RequireInstallationAdmin(c *gofr.Context) (*service.Principal, error) {
	principal, err := a.Principal(c)
	if err != nil {
		return nil, err
	}

	if a.open || principal.User.IsSystem || principal.Can(model.PermSettingsManage) {
		return principal, nil
	}

	return nil, missingPermission(model.PermSettingsManage)
}

// missingPermission is the one refusal shape, so every 403 the interface meets
// carries the same code and can be acted on rather than only shown.
func missingPermission(permission string) forbiddenError {
	return forbiddenError{msg: "missing permission: " + permission}.
		WithCode("missingPermission", permission)
}

// Require returns the caller only if they hold the permission.
func (a *Authorizer) Require(c *gofr.Context, permission string) (*service.Principal, error) {
	principal, err := a.permittedPrincipal(c)
	if err != nil {
		return nil, err
	}

	if a.open || principal.Can(permission) {
		return principal, nil
	}

	return nil, missingPermission(permission)
}

// permittedPrincipal resolves the caller and refuses anyone still on their
// initial password.
//
// The interface tells users that changes are blocked until they choose a
// password; without this the server did not actually enforce it. The password
// change itself, and reading /me, go through Principal and stay reachable.
func (a *Authorizer) permittedPrincipal(c *gofr.Context) (*service.Principal, error) {
	principal, err := a.Principal(c)
	if err != nil {
		return nil, err
	}

	if !a.open {
		if err := mustChangePassword(principal); err != nil {
			return nil, toHTTPError(err)
		}
	}

	return principal, nil
}

// RequireSelfOr allows the action when the caller is acting on their own
// account, or otherwise holds the wider permission.
func (a *Authorizer) RequireSelfOr(
	c *gofr.Context,
	targetUserID uint,
	permission string,
) (*service.Principal, error) {
	principal, err := a.permittedPrincipal(c)
	if err != nil {
		return nil, err
	}

	if a.open || principal.User.ID == targetUserID || principal.Can(permission) {
		return principal, nil
	}

	return nil, missingPermission(permission)
}

// mustChangePassword blocks everything except the password change itself
// while an initial password is still in place.
func mustChangePassword(principal *service.Principal) error {
	if principal.User != nil && principal.User.MustChangePassword {
		return apperror.Conflictf("the initial password must be changed before using the application").
			WithCode("initialPasswordPending")
	}

	return nil
}
