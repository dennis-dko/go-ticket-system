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

// An installation that configured SECRET_KEY while the project was called
// Go-Time-Recording recorded its key check then, once, under the marker and with
// the content of that time. Neither is written again, so the same key has to go
// on passing the check - or the application refuses to start on an installation
// whose key has not changed.
func TestAKeyCheckRecordedBeforeTheRenameStillPasses(t *testing.T) {
	raw := make([]byte, security.KeyBytes)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("no randomness: %v", err)
	}

	sealer, err := security.NewSealer(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}

	sealed, err := sealer.Seal("go-time-recording")
	if err != nil {
		t.Fatal(err)
	}

	// The body is what it always was; only the marker in front of it changed.
	recorded := "gtr.v1:" + sealed[strings.Index(sealed, ":")+1:]

	settings := newStubSettings()
	if err := settings.Set(context.Background(), model.SettingSecretKeyCheck, recorded); err != nil {
		t.Fatal(err)
	}

	if err := service.NewSecretsService(settings, sealer).Verify(context.Background()); err != nil {
		t.Errorf("the key the installation was set up with is refused: %v", err)
	}
}
