package rest

import (
	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/go-time-recording/internal/application/v1/service"
	"github.com/dennis-dko/go-time-recording/internal/support/apperror"
)

// WithTokens lets the owner of a personal token sign in to the web interface
// with it; see TokenSignIn.
func (h *AuthHandler) WithTokens(tokens *service.APITokenService) *AuthHandler {
	h.tokens = tokens

	return h
}

// TokenSignIn handles POST /api/v1/auth/token: a session for the owner of the
// personal token the request carries. It is how the Android app signs in to the
// web interface it shows, connected by scanning the code a new token is shown
// with, so that the phone is not where a password and a second factor are typed.
//
// It grants nothing the token does not. The token already carries the whole of
// its owner's role through the API, past any second factor, because its owner
// passed one to make it; a session is the same rights in the form the web
// interface reads them in, with the same lifetime and the same refusal during
// maintenance as every other session, since it is opened and completed as a
// passkey's or a provider's is.
//
// Only the token decides who. A session cookie on the same request is not asked,
// so a stray cookie cannot turn somebody else's token into its own session. A
// token that does not resolve is refused as a wrong password is, with nothing
// to tell the two apart, and one whose owner is still on the initial password
// says so, as the token does everywhere else.
func (h *AuthHandler) TokenSignIn(c *gofr.Context) (any, error) {
	secret := presentedToken(requestOf(c))
	if secret == "" || h.tokens == nil {
		return nil, unauthorizedError{}
	}

	principal, err := h.tokens.Resolve(c, secret)
	if err != nil {
		switch apperror.KindOf(err) {
		case apperror.KindInternal, apperror.KindConflict:
			return nil, toHTTPError(err)
		}

		return nil, unauthorizedError{}
	}

	result, err := h.sessions.OpenSession(c, principal.User)
	if err != nil {
		return nil, toHTTPError(err)
	}

	return completeSignIn(c, h.sessions, h.maintenance, result, h.timezone.resolve(c))
}
