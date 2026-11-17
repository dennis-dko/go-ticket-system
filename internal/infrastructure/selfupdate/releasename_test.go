package selfupdate

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The release builds this version's binary from ./cmd and publishes it under the
// name this version downloads its update by.
//
// A running version cannot be told a new name, so a release that published the
// web binary under any other one would be offered on every installation and then
// found to have no binary for its platform. The name has changed twice - to a
// -web binary, and to TimePulse's - and each time the versions from before were
// left asking for a name releases no longer carry, which was decided rather than
// overlooked. This is what keeps it from happening by accident.
func TestTheReleasePublishesTheNameThisVersionUpdatesBy(t *testing.T) {
	workflow := releaseWorkflow(t)
	stem, asked := assetStem(t)

	if !regexp.MustCompile(`(?m)^\s+compile ` + regexp.QuoteMeta(stem) + ` \./cmd "\$@"\s*$`).MatchString(workflow) {
		t.Errorf("the release builds no %s from ./cmd, and this version asks for %s", stem, asked)
	}

	if !regexp.MustCompile(`(?m)^\s+dist/` + regexp.QuoteMeta(stem) + `_\*\s*$`).MatchString(workflow) {
		t.Errorf("the release publishes nothing called %s_..., and this version asks for %s", stem, asked)
	}
}

// releaseWorkflow is the release workflow as it is written.
func releaseWorkflow(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile("../../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("reading the release workflow: %v", err)
	}

	return string(raw)
}

// assetStem is the part of the name this version asks for that comes before the
// version: what the release has to build and publish.
func assetStem(t *testing.T) (stem, asked string) {
	t.Helper()

	asked = assetName("v1.2.3")

	stem, _, found := strings.Cut(asked, "_v1.2.3_")
	if !found {
		t.Fatalf("this version asks for %s, which is not <name>_<version>_<os>_<arch>", asked)
	}

	return stem, asked
}
