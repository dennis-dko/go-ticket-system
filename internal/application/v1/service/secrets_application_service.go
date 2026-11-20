package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/domain/repository"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// SecretsService answers one question at start-up: is the configured key the key
// this installation's secrets were written with.
//
// Bringing what an installation already had under that key is a separate job and
// lives with the data, in sqldb.SealStoredSecrets - it has to see the column
// rather than the value, and every read here has already decrypted.
type SecretsService struct {
	settings repository.SettingsRepository
	secrets  *security.Sealer
}

// NewSecretsService is given the sealer built from the configured key, which is
// the key its one question is about.
func NewSecretsService(
	settings repository.SettingsRepository,
	secrets *security.Sealer,
) *SecretsService {
	return &SecretsService{settings: settings, secrets: secrets}
}

// theKeyIsRight is what the check value is sealed from. What it says is not
// compared when the check is read back: the cipher authenticates what it opens,
// so a check that opens was written with this key, and one written by an earlier
// version goes on saying what that version wrote.
const theKeyIsRight = "timepulse"

// Verify reports whether the configured key is the one this installation's data
// was written with, and records it the first time.
//
// The failure this prevents is the quiet one. A key that is merely *a* valid key
// decrypts nothing that was written with another, and everything that reads a
// secret fails one at a time: a second factor that says the code is wrong, a
// directory that will not bind. Each looks like a different problem, none of them
// looks like the key, and the person diagnosing it is doing so days after
// whatever changed.
//
// So the answer is taken once, at start-up, before anything serves.
func (s *SecretsService) Verify(ctx context.Context) error {
	stored, err := s.settings.Get(ctx, model.SettingSecretKeyCheck)
	if err != nil {
		return err
	}

	if stored == "" {
		// Nothing to check against yet. Recorded only when there is a key, so an
		// installation that never configures one never grows a value it would
		// then be measured against.
		if !s.secrets.Enabled() {
			return nil
		}

		sealed, sealErr := s.secrets.Seal(theKeyIsRight)
		if sealErr != nil {
			return sealErr
		}

		return s.settings.Set(ctx, model.SettingSecretKeyCheck, sealed)
	}

	return CheckSecretKey(s.secrets, stored)
}

var (
	// ErrSecretKeyMissing says an installation's secrets were sealed and no
	// SECRET_KEY is configured to open them.
	ErrSecretKeyMissing = errors.New("this installation's secrets were encrypted and SECRET_KEY " +
		"is not set: put the key back, or clear the second factors and the " +
		"directory password and let them be entered again")

	// ErrSecretKeyDiffers says SECRET_KEY is a key, and not the one an
	// installation's secrets were sealed with.
	ErrSecretKeyDiffers = errors.New("SECRET_KEY is not the key this installation's secrets were written with")
)

// CheckSecretKey asks whether secrets opens stored, the key check an
// installation kept - the question Verify asks at every start. The installer
// asks it of the key check inside a backup before taking the backup, because
// the start that follows a restore would ask it anyway, and by then the only
// answer left is to end the process.
func CheckSecretKey(secrets *security.Sealer, stored string) error {
	// Sealed, or it proves nothing: Open hands a value without the marker back as
	// it is, and a check that never was sealed would pass whatever the key.
	if !security.IsSealed(stored) {
		return errors.New("this installation's key check is not sealed, so SECRET_KEY " +
			"cannot be checked against it")
	}

	if _, err := secrets.Open(stored); err != nil {
		if errors.Is(err, security.ErrNoKey) {
			return ErrSecretKeyMissing
		}

		return fmt.Errorf("%w: %w", ErrSecretKeyDiffers, err)
	}

	return nil
}
