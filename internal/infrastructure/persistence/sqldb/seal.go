package sqldb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// SealStoredSecrets encrypts what was written before a key was configured.
//
// Turning encryption on is something an installation does after it has been
// running, not before. There are already TOTP secrets in the users table and a
// bind password in the settings, written in the clear, and a key that applied
// only to what is written next would leave every one of them exactly as exposed
// as before while the configuration says otherwise. A second factor is enrolled
// once and then read for years; without this, "encryption is on" would be true of
// almost nothing.
//
// It reads the columns directly rather than going through the repositories,
// because that is the only way to see which values are already sealed: every read
// path decrypts, so a repository would hand back plaintext either way and this
// would rewrite the whole table on every start.
//
// Returns how many values it moved, which is zero on every start after the first.
func SealStoredSecrets(
	ctx context.Context,
	db DB,
	dialect string,
	secrets *security.Sealer,
) (int, error) {
	if !secrets.Enabled() {
		return 0, nil
	}

	return rewriteStoredSecrets(ctx, db, dialect, func(stored string) (string, error) {
		if security.IsSealed(stored) {
			return stored, nil
		}

		return secrets.Seal(stored)
	})
}

// rewriteStoredSecrets hands change every value a SECRET_KEY seals - the second
// factors, the directory's bind password, a sign-in provider's client secret and
// the key that signs a push to a browser - as it is stored, empty ones aside, and
// stores what change answers wherever that differs.
//
// Bringing what was written in the clear under a key is one change; re-sealing a
// restored backup's secrets under another key and leaving them behind are two
// more. One list of where the secrets are serves all three, so a secret added to
// it is sealed, re-sealed and left behind alike - the push key was missing from
// the first of them for as long as each kept a list of its own.
//
// Returns how many values it changed.
func rewriteStoredSecrets(
	ctx context.Context,
	db DB,
	dialect string,
	change func(stored string) (string, error),
) (int, error) {
	changed, err := rewriteTOTPSecrets(ctx, db, dialect, change)
	if err != nil {
		return changed, err
	}

	for _, rewrite := range []func() (int, error){
		func() (int, error) {
			return rewriteSettingSecret(ctx, db, dialect, model.SettingLDAPSettings,
				func(config *model.LDAPConfig) *string { return &config.BindPassword }, change)
		},
		func() (int, error) {
			return rewriteSettingSecret(ctx, db, dialect, model.SettingOpenID,
				func(config *model.OpenIDConfig) *string { return &config.ClientSecret }, change)
		},
		func() (int, error) {
			return rewritePlainSetting(ctx, db, dialect, model.SettingWebPushKey, change)
		},
	} {
		n, err := rewrite()
		changed += n

		if err != nil {
			return changed, err
		}
	}

	return changed, nil
}

// rewriteTOTPSecrets changes the seeds of the second factors.
func rewriteTOTPSecrets(
	ctx context.Context,
	db DB,
	dialect string,
	change func(stored string) (string, error),
) (int, error) {
	rows, err := db.QueryContext(ctx,
		Rebind(dialect, "SELECT id, totp_secret FROM users WHERE totp_secret <> ''"))
	if err != nil {
		return 0, fmt.Errorf("reading the stored second factors: %w", err)
	}

	// Collected before anything is written: an update while the rows are open is
	// a lock the same connection is waiting on, on SQLite.
	type stored struct {
		id     uint
		secret string
	}

	var all []stored

	for rows.Next() {
		var row stored
		if err := rows.Scan(&row.id, &row.secret); err != nil {
			_ = rows.Close()

			return 0, fmt.Errorf("reading the stored second factors: %w", err)
		}

		all = append(all, row)
	}

	err = rows.Err()
	_ = rows.Close()

	if err != nil {
		return 0, fmt.Errorf("reading the stored second factors: %w", err)
	}

	changed := 0

	for _, row := range all {
		value, err := change(row.secret)
		if err != nil {
			return changed, fmt.Errorf("a stored second factor: %w", err)
		}

		if value == row.secret {
			continue
		}

		if _, err := db.ExecContext(ctx,
			Rebind(dialect, "UPDATE users SET totp_secret = ? WHERE id = ?"),
			value, row.id); err != nil {
			return changed, fmt.Errorf("rewriting a stored second factor: %w", err)
		}

		changed++
	}

	return changed, nil
}

// rewriteSettingSecret changes the credential inside a setting stored as JSON -
// the directory's bind password, a sign-in provider's client secret.
//
// Only that field. The rest of the configuration is a setting rather than a
// credential, and a stored setting is worth being able to read when something is
// wrong with it.
func rewriteSettingSecret[C any](
	ctx context.Context,
	db DB,
	dialect string,
	key string,
	secretOf func(*C) *string,
	change func(stored string) (string, error),
) (int, error) {
	var raw string

	err := db.QueryRowContext(ctx,
		Rebind(dialect, "SELECT value FROM settings WHERE key_name = ?"), key).Scan(&raw)
	if err != nil || raw == "" {
		// Nothing configured is the ordinary case, and sql.ErrNoRows says exactly
		// that. Nothing else here can fail in a way worth stopping a start-up over.
		return 0, nil
	}

	var config C
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		// A corrupt entry is the settings screen's problem, not this one's.
		return 0, nil
	}

	secret := secretOf(&config)
	if *secret == "" {
		return 0, nil
	}

	value, err := change(*secret)
	if err != nil {
		return 0, fmt.Errorf("the credential in %s: %w", key, err)
	}

	if value == *secret {
		return 0, nil
	}

	*secret = value

	encoded, err := json.Marshal(config)
	if err != nil {
		return 0, err
	}

	if _, err := db.ExecContext(ctx,
		Rebind(dialect, "UPDATE settings SET value = ? WHERE key_name = ?"), string(encoded), key); err != nil {
		return 0, fmt.Errorf("rewriting the credential in %s: %w", key, err)
	}

	return 1, nil
}

// rewritePlainSetting changes a setting whose whole value is the secret - the key
// that signs a push to a browser.
func rewritePlainSetting(
	ctx context.Context,
	db DB,
	dialect string,
	key string,
	change func(stored string) (string, error),
) (int, error) {
	var stored string

	err := db.QueryRowContext(ctx,
		Rebind(dialect, "SELECT value FROM settings WHERE key_name = ?"), key).Scan(&stored)
	if err != nil || stored == "" {
		return 0, nil
	}

	value, err := change(stored)
	if err != nil {
		return 0, fmt.Errorf("the secret in %s: %w", key, err)
	}

	if value == stored {
		return 0, nil
	}

	if _, err := db.ExecContext(ctx,
		Rebind(dialect, "UPDATE settings SET value = ? WHERE key_name = ?"), value, key); err != nil {
		return 0, fmt.Errorf("rewriting the secret in %s: %w", key, err)
	}

	return 1, nil
}
