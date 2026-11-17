package migration_test

import (
	"testing"

	"github.com/dennis-dko/timepulse/internal/domain/model"
)

// beforeTheNewMarker is the last version an installation can have run while a
// sealed value carried the marker from before the project was called TimePulse.
const beforeTheNewMarker = int64(20261110010000)

// Every value sealed under the old marker is moved to the new one wherever a
// sealed value is kept - a second factor, the key check and the push key as they
// are, the directory's and the provider's passwords inside their settings - with
// the body after the marker untouched, so the key still opens it. A second factor
// kept in the clear, and settings that seal nothing, are left as they were.
func TestASecretSealedUnderTheOldMarkerIsMovedToTheNewOne(t *testing.T) {
	t.Parallel()

	db := freshDB(t)
	migrate(t, db, 0, beforeTheNewMarker)

	// A body as sealing writes it: standard base64 of a nonce and a ciphertext.
	const body = "dcm7/pSH7xLzgw2E9mika1zcWL1AmyI1zQ9QlZvq3lMKUWGbXLkPqzoQRCg="

	people := map[string]string{
		"sealed@example.com": "gtr.v1:" + body,
		"plain@example.com":  "JBSWY3DPEHPK3PXP",
		"none@example.com":   "",
	}

	for email, secret := range people {
		if _, err := db.Exec(`INSERT INTO users (name, email, role_id, password_hash, totp_secret)
			SELECT ?, ?, id, '', ? FROM roles WHERE name = ?`, email, email, secret, model.RoleUser); err != nil {
			t.Fatalf("making %s: %v", email, err)
		}
	}

	settings := map[string]string{
		model.SettingSecretKeyCheck: "gtr.v1:" + body,
		model.SettingWebPushKey:     "gtr.v1:" + body,
		model.SettingLDAPSettings:   `{"url":"ldap://dir:389","bindPassword":"gtr.v1:` + body + `"}`,
		model.SettingOpenID:         `{"issuer":"https://id.example.com","clientSecret":"gtr.v1:` + body + `"}`,
		model.SettingHolidayPlace:   "DE-BY",
	}

	for key, value := range settings {
		if _, err := db.Exec(`INSERT INTO settings (key_name, value, updated_at) VALUES (?, ?, '2026-11-10')`,
			key, value); err != nil {
			t.Fatalf("storing %s: %v", key, err)
		}
	}

	migrate(t, db, beforeTheNewMarker, latest(t))

	wantPeople := map[string]string{
		"sealed@example.com": "timepulse.v1:" + body,
		"plain@example.com":  "JBSWY3DPEHPK3PXP",
		"none@example.com":   "",
	}

	for email, want := range wantPeople {
		var got string
		if err := db.QueryRow(`SELECT totp_secret FROM users WHERE email = ?`, email).Scan(&got); err != nil {
			t.Fatalf("reading %s back: %v", email, err)
		}

		if got != want {
			t.Errorf("%s keeps %q, want %q", email, got, want)
		}
	}

	wantSettings := map[string]string{
		model.SettingSecretKeyCheck: "timepulse.v1:" + body,
		model.SettingWebPushKey:     "timepulse.v1:" + body,
		model.SettingLDAPSettings:   `{"url":"ldap://dir:389","bindPassword":"timepulse.v1:` + body + `"}`,
		model.SettingOpenID:         `{"issuer":"https://id.example.com","clientSecret":"timepulse.v1:` + body + `"}`,
		model.SettingHolidayPlace:   "DE-BY",
	}

	for key, want := range wantSettings {
		var got string
		if err := db.QueryRow(`SELECT value FROM settings WHERE key_name = ?`, key).Scan(&got); err != nil {
			t.Fatalf("reading %s back: %v", key, err)
		}

		if got != want {
			t.Errorf("%s keeps %q, want %q", key, got, want)
		}
	}
}
