package service

import (
	"context"
	"strings"
	"time"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
	"github.com/dennis-dko/go-time-recording/internal/domain/repository"
	"github.com/dennis-dko/go-time-recording/internal/support/apperror"
	"github.com/dennis-dko/go-time-recording/internal/support/security"
)

// APITokenPrefix marks the value as a token of this application, so a leaked
// string is recognisable in a log or a repository scan.
const APITokenPrefix = "gtr_"

// maxTokensPerUser bounds how many tokens one account can accumulate.
const maxTokensPerUser = 20

// APITokenService issues and resolves personal API tokens.
type APITokenService struct {
	tokens   repository.APITokenRepository
	sessions repository.SessionRepository
	users    repository.UserRepository
	auth     *AuthService
}

// NewAPITokenService needs the auth service because a token resolves to its
// account's principal through the same code a session's does, and the sessions
// because taking a token back ends the ones it was exchanged for.
func NewAPITokenService(
	tokens repository.APITokenRepository,
	sessions repository.SessionRepository,
	users repository.UserRepository,
	auth *AuthService,
) *APITokenService {
	return &APITokenService{tokens: tokens, sessions: sessions, users: users, auth: auth}
}

// IssuedToken is returned once, at creation. The secret is never retrievable
// again, which is why it is a separate type from the stored token.
type IssuedToken struct {
	Token *model.APIToken

	// Secret is the value the caller must keep; only its hash is stored.
	Secret string
}

// Create issues a token for the user.
//
// expiresInDays of 0 means the token does not expire on its own; it can still
// be revoked at any time.
func (s *APITokenService) Create(
	ctx context.Context,
	userID uint,
	name string,
	expiresInDays int,
) (*IssuedToken, error) {
	name = strings.TrimSpace(name)
	if name == "" || model.TooLong(name, model.MaxTokenLabelLength) {
		return nil, apperror.InvalidFields("name")
	}

	if expiresInDays < 0 || expiresInDays > 3650 {
		return nil, apperror.InvalidFields("expiresInDays")
	}

	existing, err := s.tokens.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(existing) >= maxTokensPerUser {
		return nil, apperror.Conflictf("at most %d tokens per user; revoke one first", maxTokensPerUser).
			WithCode("tooManyTokens", maxTokensPerUser)
	}

	secret, err := security.NewSessionToken()
	if err != nil {
		return nil, apperror.Internal(err)
	}

	secret = APITokenPrefix + secret

	token := &model.APIToken{
		UserID:    userID,
		Name:      name,
		TokenHash: security.HashToken(secret),
		Prefix:    secret[:len(APITokenPrefix)+6],
		CreatedAt: time.Now(),
	}

	if expiresInDays > 0 {
		expiry := time.Now().AddDate(0, 0, expiresInDays)
		token.ExpiresAt = &expiry
	}

	saved, err := s.tokens.Save(ctx, token)
	if err != nil {
		return nil, err
	}

	return &IssuedToken{Token: saved, Secret: secret}, nil
}

// List returns the user's tokens, without any secret material.
func (s *APITokenService) List(ctx context.Context, userID uint) ([]*model.APIToken, error) {
	return s.tokens.ListForUser(ctx, userID)
}

// Revoke deletes one of the user's own tokens, and ends the sessions it was
// exchanged for.
//
// The sessions first: a failure between the two then leaves a token to take back
// again, where the other order would leave a phone signed in by a token that is
// gone. Both are scoped to the user, so nobody ends another person's sessions by
// naming a token of theirs.
func (s *APITokenService) Revoke(ctx context.Context, id, userID uint) error {
	if err := s.sessions.DeleteOpenedBy(ctx, id, userID); err != nil {
		return err
	}

	return s.tokens.Delete(ctx, id, userID)
}

// Resolve turns a presented token into its owner's principal.
//
// The permissions come from the owner's current role, evaluated now: a role
// change or a revoked permission takes effect on the very next request, and a
// token can never outrank the person it belongs to.
func (s *APITokenService) Resolve(ctx context.Context, secret string) (*Principal, error) {
	_, principal, err := s.ResolveToken(ctx, secret)

	return principal, err
}

// ResolveToken is Resolve for a caller that has to know which token it was:
// exchanging one for a session, which ends with the token.
func (s *APITokenService) ResolveToken(ctx context.Context, secret string) (*model.APIToken, *Principal, error) {
	invalid := apperror.Invalidf("invalid token").WithCode("invalidToken")

	if !strings.HasPrefix(secret, APITokenPrefix) {
		return nil, nil, invalid
	}

	token, err := s.tokens.GetByHash(ctx, security.HashToken(secret))
	if err != nil {
		return nil, nil, missingOr(err, invalid)
	}

	if token.Expired(time.Now()) {
		return nil, nil, invalid
	}

	user, err := s.users.GetByID(ctx, token.UserID)
	if err != nil {
		return nil, nil, missingOr(err, invalid)
	}

	// An account still on its initial password must not be usable through a
	// token either, or the password rule could simply be bypassed.
	if user.MustChangePassword {
		return nil, nil, apperror.Conflictf("the account must change its initial password first").
			WithCode("mustChangePasswordFirst")
	}

	// At most once a maxTouchInterval, for the reason a session's last use is:
	// a script is what a token is for, and writing on every one of its requests
	// made each a write and its commit - measured on SQLite, 3.3 to 6.3 ms a
	// request against 0.5 without it. Best effort, like that one: failing to
	// record a use must not fail the request.
	if token.LastUsedAt == nil || time.Since(*token.LastUsedAt) >= maxTouchInterval {
		_ = s.tokens.TouchLastUsed(ctx, token.ID)
	}

	principal, err := s.auth.principalFor(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return token, principal, nil
}
