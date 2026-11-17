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
// found to have no binary for its platform. The name has been changed twice: to
// go-time-recording-web, after which the versions from before it were left
// asking for a name releases no longer carry, which was decided rather than
// overlooked; and to timepulse-web, whose predecessors are still answered - see
// TestTheReleaseStillAnswersTheNameFromBeforeTheRename.
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

// The release attaches every binary a second time under the name the versions
// from before the rename to TimePulse update themselves by.
//
// Those versions ask for go-time-recording-web_<version>_<os>_<arch> and
// nothing else, so a release without the copy is one every such installation is
// offered and cannot take, and it stays on its version until somebody replaces
// it by hand. The bridge is meant to come down once nothing that old is left to
// update - on purpose, with this case, rather than because a line in the
// workflow went missing.
func TestTheReleaseStillAnswersTheNameFromBeforeTheRename(t *testing.T) {
	workflow := releaseWorkflow(t)
	stem, _ := assetStem(t)

	copied := regexp.MustCompile(`(?m)^\s+for binary in dist/` + regexp.QuoteMeta(stem) + `_\*; do\n` +
		`\s+cp "\$\{binary\}" "dist/go-time-recording-web_\$\{binary#dist/` + regexp.QuoteMeta(stem) + `_\}"\n`)
	if !copied.MatchString(workflow) {
		t.Errorf("the release no longer copies each %s_... to go-time-recording-web_..., which every version from before the rename asks for", stem)
	}

	if !regexp.MustCompile(`(?m)^\s+dist/go-time-recording-web_\*\s*$`).MatchString(workflow) {
		t.Error("the release publishes nothing called go-time-recording-web_..., which every version from before the rename asks for")
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
