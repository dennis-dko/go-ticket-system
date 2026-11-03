package migration_test

import (
	"testing"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// beforeCountries is the last version an installation can have run while a place
// for public holidays was a German state's code alone.
const beforeCountries = int64(20261022090000)

// germanStates is every place there was before there were countries.
var germanStates = []string{
	"BW", "BY", "BE", "BB", "HB", "HH", "HE", "MV",
	"NI", "NW", "RP", "SL", "SN", "ST", "SH", "TH",
}

// Every German state stored before there were countries is written with its
// country, the installation's and each person's, as a place today's rules know -
// and nothing else is touched: empty still follows the installation, and none
// still keeps none.
//
// Berlin is the installation's on purpose. BE is Berlin written the old way and
// Belgium the new one, so a value left as it was would not be refused, it would
// quietly move the installation to another country.
func TestAStateStoredBeforeCountriesIsPlacedInGermany(t *testing.T) {
	t.Parallel()

	db := freshDB(t)
	migrate(t, db, 0, beforeCountries)

	if _, err := db.Exec(`INSERT INTO settings (key_name, value, updated_at) VALUES (?, ?, '2026-10-22')`,
		model.SettingHolidayPlace, "BE"); err != nil {
		t.Fatalf("storing the installation's state: %v", err)
	}

	want := map[string]string{"follows@example.com": "", "none@example.com": model.NoHolidays}
	stored := map[string]string{"follows@example.com": "", "none@example.com": model.NoHolidays}

	for _, state := range germanStates {
		stored[state+"@example.com"] = state
		want[state+"@example.com"] = "DE-" + state
	}

	for email, region := range stored {
		if _, err := db.Exec(`INSERT INTO users (name, email, role_id, password_hash, holiday_region)
			SELECT ?, ?, id, '', ? FROM roles WHERE name = ?`, email, email, region, model.RoleUser); err != nil {
			t.Fatalf("making %s: %v", email, err)
		}
	}

	migrate(t, db, beforeCountries, latest(t))

	var installation string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key_name = ?`,
		model.SettingHolidayPlace).Scan(&installation); err != nil || installation != "DE-BE" {
		t.Errorf("the installation keeps %q (%v), want Berlin with its country, DE-BE", installation, err)
	}

	for email, place := range want {
		var got string
		if err := db.QueryRow(`SELECT holiday_region FROM users WHERE email = ?`, email).Scan(&got); err != nil {
			t.Fatalf("reading %s back: %v", email, err)
		}

		if got != place {
			t.Errorf("%s keeps %q, want %q", email, got, place)
		}

		if !model.ValidHolidayChoice(got) {
			t.Errorf("%s keeps %q, which today's rules refuse", email, got)
		}
	}
}
