package sqldb_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// sealedEverywhere keeps every kind of secret a SECRET_KEY seals, sealed with
// key, the way an installation that had that key keeps them - with its key check
// and a browser subscribed to its push key - and answers the account that holds
// the second factor.
func sealedEverywhere(t *testing.T, db *sql.DB, key *security.Sealer) uint {
	t.Helper()

	ctx := context.Background()

	seal := func(plain string) string {
		sealed, err := key.Seal(plain)
		if err != nil {
			t.Fatalf("seal %q: %v", plain, err)
		}

		return sealed
	}

	role, err := sqldb.NewRoleRepository(db, sqldb.DialectSQLite).GetByName(ctx, model.RoleUser)
	if err != nil {
		t.Fatalf("read seeded role: %v", err)
	}

	user, err := sqldb.NewUserRepository(db, sqldb.DialectSQLite).Save(ctx, &model.User{
		Name: "Enrolled", Email: "enrolled@example.com", RoleID: role.ID,
		TOTPSecret: "JBSWY3DPEHPK3PXP", TOTPEnabled: true,
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// Into the column, because a repository reads what it saved back, and one
	// without the key cannot open a sealed seed.
	if _, err := db.Exec("UPDATE users SET totp_secret = ? WHERE id = ?", seal("JBSWY3DPEHPK3PXP"), user.ID); err != nil {
		t.Fatalf("seal the second factor: %v", err)
	}

	encode := func(value any) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}

		return string(raw)
	}

	settings := sqldb.NewSettingsRepository(db, sqldb.DialectSQLite)

	for name, value := range map[string]string{
		model.SettingLDAPSettings: encode(model.LDAPConfig{
			Host: "ldap.example.com", BindPassword: seal("the-bind-password"),
		}),
		model.SettingOpenID: encode(model.OpenIDConfig{
			Issuer: "https://id.example.com", ClientSecret: seal("the-client-secret"),
		}),
		model.SettingWebPushKey:     seal("the-private-push-key"),
		model.SettingSecretKeyCheck: seal("timepulse"),
	} {
		if err := settings.Set(ctx, name, value); err != nil {
			t.Fatalf("store %s: %v", name, err)
		}
	}

	if _, err := db.Exec("INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, origin, created_at) "+
		"VALUES (?, ?, ?, ?, ?, ?)", user.ID, "https://push.example.com/one", "the-p256dh", "the-auth",
		"https://time.example.com", time.Now()); err != nil {
		t.Fatalf("subscribe a browser: %v", err)
	}

	return user.ID
}

// storedSettingSecrets reads the settings that hold a secret as they are stored:
// the two JSON settings whole, and the push key.
func storedSettingSecrets(t *testing.T, db *sql.DB) (model.LDAPConfig, model.OpenIDConfig, string) {
	t.Helper()

	ctx := context.Background()
	settings := sqldb.NewSettingsRepository(db, sqldb.DialectSQLite)

	var (
		ldap   model.LDAPConfig
		openID model.OpenIDConfig
	)

	for name, into := range map[string]any{model.SettingLDAPSettings: &ldap, model.SettingOpenID: &openID} {
		raw, err := settings.Get(ctx, name)
		if err != nil || json.Unmarshal([]byte(raw), into) != nil {
			t.Fatalf("read %s: %q (%v)", name, raw, err)
		}
	}

	pushKey, err := settings.Get(ctx, model.SettingWebPushKey)
	if err != nil {
		t.Fatalf("read the push key: %v", err)
	}

	return ldap, openID, pushKey
}

// keyCheck is the key check as it is kept, empty where there is none.
func keyCheck(t *testing.T, db *sql.DB) string {
	t.Helper()

	check, err := sqldb.NewSettingsRepository(db, sqldb.DialectSQLite).Get(context.Background(), model.SettingSecretKeyCheck)
	if err != nil {
		t.Fatalf("read the key check: %v", err)
	}

	return check
}

// A backup made under another key keeps every secret it carries when it is
// restored with that key: each is opened with the key it came under and sealed
// with this installation's - or kept in the clear where this one has none - and
// the key check of the installation it came from goes, so that the start records
// this installation's rather than refusing it.
func TestARestoredBackupsSecretsAreSealedAgainUnderThisKey(t *testing.T) {
	none, err := security.NewSealer("")
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	for _, target := range []struct {
		name string
		to   *security.Sealer
	}{
		{"under this installation's key", aSealer(t)},
		{"into an installation without one", none},
	} {
		t.Run(target.name, func(t *testing.T) {
			db := newTestDB(t)
			old := aSealer(t)
			id := sealedEverywhere(t, db, old)

			changed, err := sqldb.RekeyStoredSecrets(context.Background(), db, sqldb.DialectSQLite, old, target.to)
			if err != nil || changed != 4 {
				t.Fatalf("re-sealed %d secrets (%v), want the second factor, both credentials and the push key",
					changed, err)
			}

			ldap, openID, pushKey := storedSettingSecrets(t, db)

			for what, stored := range map[string][2]string{
				"the second factor": {storedSecret(t, db, id), "JBSWY3DPEHPK3PXP"},
				"the bind password": {ldap.BindPassword, "the-bind-password"},
				"the client secret": {openID.ClientSecret, "the-client-secret"},
				"the push key":      {pushKey, "the-private-push-key"},
			} {
				if opened, err := target.to.Open(stored[0]); err != nil || opened != stored[1] {
					t.Errorf("%s opens to %q (%v) with this installation's key", what, opened, err)
				}

				if target.to.Enabled() {
					if !security.IsSealed(stored[0]) {
						t.Errorf("%s is stored in the clear in an installation with a key", what)
					} else if _, err := old.Open(stored[0]); err == nil {
						t.Errorf("%s still opens with the key it came under", what)
					}
				}

				if !target.to.Enabled() && stored[0] != stored[1] {
					t.Errorf("%s is stored as %q in an installation without a key, want it in the clear",
						what, stored[0])
				}
			}

			if ldap.Host != "ldap.example.com" || openID.Issuer != "https://id.example.com" {
				t.Errorf("the settings around the credentials changed: %+v %+v", ldap, openID)
			}

			if check := keyCheck(t, db); check != "" {
				t.Errorf("the key check of the installation the backup came from is still kept: %q", check)
			}
		})
	}
}

// A backup whose key is lost comes back without its secrets: the second factor
// is switched off, the bind password and the client secret are empty while the
// rest of their settings stay, the push key goes with the browsers subscribed to
// it, and so does the key check.
func TestARestoredBackupsSecretsCanBeLeftBehind(t *testing.T) {
	db := newTestDB(t)
	id := sealedEverywhere(t, db, aSealer(t))

	left, err := sqldb.LeaveStoredSecretsBehind(context.Background(), db, sqldb.DialectSQLite)
	if err != nil || left != 4 {
		t.Fatalf("left %d secrets behind (%v), want the second factor, both credentials and the push key", left, err)
	}

	var (
		secret  string
		enabled bool
	)

	if err := db.QueryRow("SELECT totp_secret, totp_enabled FROM users WHERE id = ?", id).
		Scan(&secret, &enabled); err != nil {
		t.Fatalf("read the account: %v", err)
	}

	if secret != "" || enabled {
		t.Errorf("the account keeps a second factor: seed %q, switched on %v", secret, enabled)
	}

	ldap, openID, pushKey := storedSettingSecrets(t, db)

	if ldap.BindPassword != "" || openID.ClientSecret != "" || pushKey != "" {
		t.Errorf("secrets were kept: bind password %q, client secret %q, push key %q",
			ldap.BindPassword, openID.ClientSecret, pushKey)
	}

	if ldap.Host != "ldap.example.com" || openID.Issuer != "https://id.example.com" {
		t.Errorf("the settings around the credentials went with them: %+v %+v", ldap, openID)
	}

	var subscriptions int

	if err := db.QueryRow("SELECT COUNT(*) FROM push_subscriptions").Scan(&subscriptions); err != nil ||
		subscriptions != 0 {
		t.Errorf("%d browsers stay subscribed to a push key that is gone (%v)", subscriptions, err)
	}

	if check := keyCheck(t, db); check != "" {
		t.Errorf("the key check of the installation the backup came from is still kept: %q", check)
	}
}
