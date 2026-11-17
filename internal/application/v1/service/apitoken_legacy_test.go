package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/application/v1/service"
	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// A token issued before the project was called TimePulse begins gtr_, and one
// that never expires - a script's, or the Android app's, which signs in with
// nothing else - is held with that value for as long as it lives. So it still
// signs in as its owner, as a token of today's shape does.
func TestATokenIssuedBeforeTheRenameStillSignsIn(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	tokens := &stubTokens{byID: map[uint]*model.APIToken{}}
	svc := service.NewAPITokenService(tokens, newStubSessions(), f.userRepo, f.auth)

	// Past the initial password, which a token is refused on.
	user, err := f.userRepo.GetByID(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}

	user.MustChangePassword = false

	if _, err := f.userRepo.Update(ctx, user); err != nil {
		t.Fatal(err)
	}

	const secret = "gtr_dG9rZW4gaXNzdWVkIGJlZm9yZSB0aGUgcmVuYW1l"

	if _, err := tokens.Save(ctx, &model.APIToken{
		UserID: f.userID, Name: "the app", TokenHash: security.HashToken(secret),
		Prefix: secret[:10], CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	principal, err := svc.Resolve(ctx, secret)
	if err != nil {
		t.Fatalf("a token from before the rename was refused: %v", err)
	}

	if principal.User.ID != f.userID {
		t.Errorf("the token signed in as account %d, want its owner %d", principal.User.ID, f.userID)
	}
}
