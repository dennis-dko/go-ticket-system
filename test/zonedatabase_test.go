package test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// Every program this repository ships carries the time zone database inside it.
//
// Go looks for the database on the system first, and Windows has none. After that
// it looks in its own installation, which a release built with -trimpath cannot
// find, and in the copy a program links with time/tzdata. Without that copy, every
// zone but UTC is unknown on Windows. The web binary has carried it since it first
// read a zone. The terminal client this repository shipped beside it until
// 2026-11 did not, so on the machine it was installed on it read its owner's
// Europe/Berlin as UTC: the stopwatch said it had started an hour early, and the
// booking form offered the day in UTC. A program added later is listed here.
//
// Measured rather than assumed: a program built with -trimpath and without the
// import answers "unknown time zone Europe/Berlin" on Windows, and the same
// program with it loads the zone.
func TestEveryShippedProgramCarriesTheZoneDatabase(t *testing.T) {
	for _, program := range []string{"../cmd"} {
		out, err := exec.Command("go", "list", "-deps", program).Output()
		if err != nil {
			var said []byte
			if exited, ok := err.(*exec.ExitError); ok {
				said = exited.Stderr
			}

			t.Fatalf("listing what %s is built from: %v\n%s", program, err, said)
		}

		deps := strings.Fields(string(out))
		if len(deps) < 50 {
			t.Fatalf("%s is built from %d packages; the listing has stopped reading it", program, len(deps))
		}

		if !slices.Contains(deps, "time/tzdata") {
			t.Errorf("%s does not link time/tzdata, so on Windows it knows no zone but UTC", program)
		}
	}
}
