package selfupdate

import "fmt"

// Releases is where this project's releases are shown, each carrying the
// Android app built from the same commit.
const Releases = "https://github.com/dennis-dko/go-time-recording/releases"

// AndroidApp is where the Android app that belongs to version is downloaded:
// the page of its release, and the APK that release carries. The app belongs to
// the release it was built with, so a release's version is sent to its own; any
// other - "dev", or a source tree some commits past its last tag - has no
// release of its own and is sent to the newest one's page with no file named,
// because the newest one's file carries a version this one cannot know.
func AndroidApp(version string) (page, download string) {
	if !isRelease(version) {
		return Releases + "/latest", ""
	}

	return Releases + "/tag/" + version, Releases + "/download/" + version + "/" + androidAssetName(version)
}

// androidAssetName is what the Android app of version is called in its release.
func androidAssetName(version string) string {
	return "go-time-recording-mobile_" + version + ".apk"
}

// isRelease says whether version is the tag a release is named by and nothing
// more: parseVersion also reads what a build from a source tree says about how
// far it has moved on from its last tag, which names no release.
func isRelease(version string) bool {
	numbers, ok := parseVersion(version)

	return ok && version == fmt.Sprintf("v%d.%d.%d", numbers[0], numbers[1], numbers[2])
}
