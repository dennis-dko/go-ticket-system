package selfupdate

import (
	"os"
	"regexp"
	"testing"
)

// The Android app is downloaded from the release this installation runs, because
// the app belongs to the release it was built with: its page, and the APK itself.
// A build that is no release's has no release of its own, and is sent to the
// newest one's page with no file named.
func TestTheAndroidAppIsTheOneOfThisRelease(t *testing.T) {
	for version, want := range map[string][2]string{
		"v0.4.6": {Releases + "/tag/v0.4.6", Releases + "/download/v0.4.6/go-time-recording-mobile_v0.4.6.apk"},
		"v1.0.0": {Releases + "/tag/v1.0.0", Releases + "/download/v1.0.0/go-time-recording-mobile_v1.0.0.apk"},

		// What a build from a source tree says - how far past its last tag, a
		// changed tree, no tag at all - and what a build says without a version.
		"v0.4.6-3-g30c92cb":       {Releases + "/latest", ""},
		"v0.4.6-3-g30c92cb-dirty": {Releases + "/latest", ""},
		"v0.4.6-dirty":            {Releases + "/latest", ""},
		"30c92cb":                 {Releases + "/latest", ""},
		"0.0.0-dev":               {Releases + "/latest", ""},
		"0.4.6":                   {Releases + "/latest", ""},
		"dev":                     {Releases + "/latest", ""},
		"":                        {Releases + "/latest", ""},
	} {
		page, download := AndroidApp(version)

		if page != want[0] || download != want[1] {
			t.Errorf("the app of %q is at %q, downloaded from %q; want %q and %q", version, page, download, want[0], want[1])
		}
	}
}

// The release publishes the app under the name this version sends a phone to.
// Named anywhere else, the code on the card would download nothing at all.
func TestTheReleasePublishesTheAndroidAppUnderTheNameItIsAskedFor(t *testing.T) {
	raw, err := os.ReadFile("../../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("reading the release workflow: %v", err)
	}

	asked := androidAssetName("${VERSION}")

	if !regexp.MustCompile(`(?m)^\s+cp "\$\{apk\}" "dist/` + regexp.QuoteMeta(asked) + `"\s*$`).Match(raw) {
		t.Errorf("the release copies no APK to dist/%s, the name this version asks for", asked)
	}

	if !regexp.MustCompile(`(?m)^\s+dist/go-time-recording-mobile_\*\s*$`).Match(raw) {
		t.Error("the release publishes nothing called go-time-recording-mobile_...")
	}
}
