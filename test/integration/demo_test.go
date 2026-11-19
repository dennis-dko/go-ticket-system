//go:build integration

package integration

import (
	"net/http"
	"testing"
)

// Ticking the demo box on the installer fills the new database: signed in, the
// built-in administrator finds its own projects and weeks of bookings, written
// in the language the installer was read in.
//
// Asked of the binary as somebody meets it - the installer answered over HTTP,
// the application starting in the same process, the database migrated - because
// the demo is put in during that start, after the built-in administrator is
// made, and only a real start has those in that order.
func TestTheInstallerCanFillTheNewDatabaseWithDemoData(t *testing.T) {
	t.Parallel()

	c := openInstaller(t)

	answer := map[string]any{"dialect": "sqlite", "name": "chosen", "demo": true, "language": "de"}
	if status, body := c.post("/install/save", c.token, answer); status != http.StatusOK {
		t.Fatalf("saving a working connection with demo data: got %d, %v", status, body)
	}

	waitForTheApplication(t, c)

	admin := (&app{App: c.app, t: t}).signInAsAdmin("a-much-better-password")

	var projects listOf[projectResponse]

	admin.must(admin.api(http.MethodGet, "/projects", nil), http.StatusOK).Data(t, &projects)

	names := map[string]bool{}
	for _, p := range projects.Items {
		names[p.Name] = true
	}

	if len(projects.Items) != 4 || !names["Website-Relaunch"] {
		t.Fatalf("the administrator finds the projects %v, want the demo's four in German\n%s",
			names, c.app.Log())
	}

	var entries listOf[timesheetResponse]

	admin.must(admin.api(http.MethodGet, "/timesheets", nil), http.StatusOK).Data(t, &entries)

	if entries.TotalCount < 100 {
		t.Errorf("the administrator finds %d entries, where eight weeks of demo hold well over a hundred",
			entries.TotalCount)
	}
}

// Without the box ticked the new database is as empty as it ever was.
func TestTheInstallerFillsNothingUnasked(t *testing.T) {
	t.Parallel()

	c := openInstaller(t)

	if status, body := c.post("/install/save", c.token, sqliteDatasource()); status != http.StatusOK {
		t.Fatalf("saving a working connection: got %d, %v", status, body)
	}

	waitForTheApplication(t, c)

	admin := (&app{App: c.app, t: t}).signInAsAdmin("a-much-better-password")

	var projects listOf[projectResponse]

	// The built-in administrator cannot list projects at all as shipped, which
	// is itself the answer: nothing was put in its name, and nothing changed
	// what it may do.
	if got := admin.api(http.MethodGet, "/projects", nil); got.Status == http.StatusOK {
		got.Data(t, &projects)

		if len(projects.Items) != 0 {
			t.Errorf("an installation that asked for nothing holds %d projects", len(projects.Items))
		}
	}
}
