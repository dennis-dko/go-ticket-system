package rest

import (
	"cmp"

	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/timepulse/internal/infrastructure/selfupdate"
	"github.com/dennis-dko/timepulse/internal/support/qrcode"
)

// AndroidResponse is where the Android app that belongs to this installation is
// downloaded.
type AndroidResponse struct {
	// Release is the page of the release the app comes from.
	Release string `json:"release"`

	// Download is the app itself, where this installation runs a release.
	Download string `json:"download,omitempty"`

	// Version is the release the app at Download is, beside it and only where it
	// is: what the app compares its own with, at every start, to offer itself
	// anew when this installation has moved on.
	Version string `json:"version,omitempty"`

	// QR is a code of Download, or of Release where there is no file to name.
	QR *string `json:"qr,omitempty"`
}

// Android handles GET /api/v1/me/android: where the Android app that belongs to
// this installation's version is downloaded, with a code a phone's camera opens,
// so that the card offering the app leads a phone straight to the file. Drawn
// here because the page has no QR encoder of its own, which is also why the
// address is worked out here rather than beside the code in the page.
func (h *UpdateHandler) Android(c *gofr.Context) (any, error) {
	if _, err := h.authz.Principal(c); err != nil {
		return nil, err
	}

	answer := androidAnswer(h.version)

	// Not essential: without a code, the card has its link alone.
	if code, err := qrcode.SVGDataURI(cmp.Or(answer.Download, answer.Release)); err == nil {
		answer.QR = &code
	} else {
		c.Logger.Errorf("could not render the Android app's QR code: %v", err)
	}

	return answer, nil
}

// androidAnswer is where the app that belongs to version is downloaded, and
// which release that app is.
func androidAnswer(version string) AndroidResponse {
	page, download := selfupdate.AndroidApp(version)
	answer := AndroidResponse{Release: page, Download: download}

	if download != "" {
		answer.Version = version
	}

	return answer
}
