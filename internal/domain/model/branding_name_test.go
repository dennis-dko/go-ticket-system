package model_test

import (
	"testing"

	"github.com/dennis-dko/timepulse/internal/domain/model"
)

// TimePulse is the name of every installation, and what an installation writes
// is added to it after a bar rather than put in its place.
func TestAnAdditionFollowsTimePulseAndNeverReplacesIt(t *testing.T) {
	t.Parallel()

	for addition, want := range map[string]string{
		"":            "TimePulse",
		"   ":         "TimePulse",
		"htp.net":     "TimePulse | htp.net",
		"  htp.net  ": "TimePulse | htp.net",

		// The name written again adds nothing. It was the obvious answer to "name
		// this installation" while the name could still be replaced, so it is
		// stored on installations that never meant to add anything.
		"TimePulse":  "TimePulse",
		"timepulse ": "TimePulse",
	} {
		if got := model.Named(addition); got != want {
			t.Errorf("%q is named %q, want %q", addition, got, want)
		}
	}
}
