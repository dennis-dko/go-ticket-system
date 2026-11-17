package service_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/dennis-dko/timepulse/internal/application/v1/service"
	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// aSealer is a sealer with a key of its own.
func aSealer(t *testing.T) *security.Sealer {
	t.Helper()

	raw := make([]byte, security.KeyBytes)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("no randomness: %v", err)
	}

	sealer, err := security.NewSealer(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}

	return sealer
}

// checked keeps stored as the key check and asks whether sealer's key is the one
// it was written with.
func checked(t *testing.T, sealer *security.Sealer, stored string) error {
	t.Helper()

	settings := newStubSettings()
	if err := settings.Set(context.Background(), model.SettingSecretKeyCheck, stored); err != nil {
		t.Fatal(err)
	}

	return service.NewSecretsService(settings, sealer).Verify(context.Background())
}

// A key check that opens with the configured key passes, whatever it says. An
// installation keeps the check its first start wrote, and a version since may
// seal another word: the cipher authenticates what it opens, so opening is the
// proof, and comparing the words would refuse a key that is right.
func TestAKeyCheckThatOpensWithTheKeyPasses(t *testing.T) {
	sealer := aSealer(t)

	stored, err := sealer.Seal("what an earlier version wrote")
	if err != nil {
		t.Fatal(err)
	}

	if err := checked(t, sealer, stored); err != nil {
		t.Errorf("the key the check was written with is refused: %v", err)
	}
}

// A key check another key wrote is refused, before anything serves.
func TestAKeyCheckAnotherKeyWroteIsRefused(t *testing.T) {
	stored, err := aSealer(t).Seal("timepulse")
	if err != nil {
		t.Fatal(err)
	}

	if err := checked(t, aSealer(t), stored); err == nil || !strings.Contains(err.Error(), "SECRET_KEY") {
		t.Errorf("a check another key wrote was answered %v, want SECRET_KEY refused", err)
	}
}

// A key check that is not sealed proves nothing about the key and is refused:
// Open hands such a value back as it is, so without this any key would pass.
func TestAKeyCheckThatIsNotSealedIsRefused(t *testing.T) {
	if err := checked(t, aSealer(t), "timepulse"); err == nil {
		t.Error("a check that was never sealed passed, so any key would have")
	}
}
