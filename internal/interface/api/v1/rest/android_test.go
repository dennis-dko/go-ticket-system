package rest

import (
	"strings"
	"testing"
)

// The answer names the release its app belongs to, which the app compares its
// own version with at every start to offer itself anew. Named only beside a
// file to download: an installation that runs no release has no app of its own
// to offer, and a version it could not offer would be one the app tried for.
func TestTheAndroidAppNamesTheReleaseItBelongsTo(t *testing.T) {
	t.Parallel()

	if answer := androidAnswer("v0.4.23"); answer.Version != "v0.4.23" ||
		!strings.HasSuffix(answer.Download, "/v0.4.23/timepulse-mobile_v0.4.23.apk") {
		t.Errorf("the app of v0.4.23 is named %q and downloaded from %q", answer.Version, answer.Download)
	}

	for _, version := range []string{"v0.4.23-3-g30c92cb", "dev", ""} {
		if answer := androidAnswer(version); answer.Version != "" || answer.Download != "" {
			t.Errorf("a build that is %q names the app %q and offers %q, want neither", version, answer.Version, answer.Download)
		}
	}
}
