//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/selfupdate"
	"github.com/dennis-dko/go-ticket-system/internal/support/qrcode"
)

// Where the Android app that belongs to this installation is downloaded, and the
// same as a code for a phone's camera, for somebody signed in only. The suite's
// instance is built without a version and runs no release, so it is given the
// newest release's page and no file - which is what a build from source gets.
func TestTheAndroidAppComesAsACode(t *testing.T) {
	t.Parallel()

	a, _, worker := startWithWorker(t)

	var answer struct {
		Release  string `json:"release"`
		Download string `json:"download"`
		QR       string `json:"qr"`
	}

	worker.must(worker.api(http.MethodGet, "/me/android", nil), http.StatusOK).Data(t, &answer)

	if answer.Release != selfupdate.Releases+"/latest" || answer.Download != "" {
		t.Errorf("a build from source is sent to %q and %q, want the newest release's page and no file",
			answer.Release, answer.Download)
	}

	if want, err := qrcode.SVGDataURI(answer.Release); err != nil || answer.QR != want {
		t.Errorf("the code is not the code of the release's page (%d characters, want %d)", len(answer.QR), len(want))
	}

	if anonymous := a.newClient().do(http.MethodGet, "/api/v1/me/android", nil); anonymous.Status != http.StatusUnauthorized {
		t.Errorf("asked by nobody, the Android app answered %d, want 401", anonymous.Status)
	}
}
