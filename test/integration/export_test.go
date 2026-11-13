//go:build integration

package integration

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Everything held about an account is handed over with it, a part per table, on
// every database - and none of the secrets: not the personal token the account
// was just shown once, and no column that holds a hash, a seed or a key. A part
// with nothing in it is there as an empty list.
func TestAnAccountIsHandedOverWholeAndWithoutItsSecrets(t *testing.T) {
	t.Parallel()

	_, _, worker := startWithWorker(t)

	var token struct {
		Secret *string `json:"secret"`
	}

	worker.must(worker.api(http.MethodPost, "/me/tokens", map[string]any{"name": "ci", "expiresInDays": 30}),
		http.StatusCreated, http.StatusOK).Data(t, &token)

	if token.Secret == nil {
		t.Fatal("the token was not shown once")
	}

	exported := worker.api(http.MethodGet, "/me/export", nil)
	if exported.Status != http.StatusOK {
		t.Fatalf("the export answered %d: %s", exported.Status, exported.Body)
	}

	archive, err := zip.NewReader(bytes.NewReader(exported.Body), int64(len(exported.Body)))
	if err != nil {
		t.Fatalf("the export is not a ZIP: %v", err)
	}

	parts := map[string][]map[string]any{}
	whole := ""

	for _, file := range archive.File {
		opened, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}

		content, _ := io.ReadAll(opened)
		_ = opened.Close()
		whole += string(content)

		if file.Name == "about.json" {
			continue
		}

		var rows []map[string]any
		if err := json.Unmarshal(content, &rows); err != nil {
			t.Fatalf("%s is not a list of rows: %v", file.Name, err)
		}

		parts[strings.TrimSuffix(file.Name, ".json")] = rows
	}

	for _, part := range []string{"account", "api-tokens", "sessions"} {
		if len(parts[part]) == 0 {
			t.Errorf("%s holds nothing, though the account has some", part)
		}
	}

	for _, part := range []string{"owned-tickets", "tickets-responsible-for", "contact", "watched-tickets",
		"notifications", "queues-told-about", "passkeys", "push-subscriptions", "openid-links",
		"written-articles"} {
		if rows, there := parts[part]; !there || len(rows) != 0 {
			t.Errorf("%s reads %v, want an empty list", part, rows)
		}
	}

	for _, secret := range []string{*token.Secret,
		"password_hash", "totp_secret", "token_hash", "key_hash", "public_key", "credential_id"} {
		if strings.Contains(whole, secret) {
			t.Errorf("the export holds %q", secret)
		}
	}
}
