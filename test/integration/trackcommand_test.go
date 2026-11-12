//go:build integration

package integration

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dennis-dko/go-ticket-system/test/harness"
)

// The terminal client was once the web binary's track command, and is gone. That
// command line is answered with where to book instead rather than passed over:
// passed over, the web binary starts and serves an installer instead of booking
// anything, which is what a script still running the old instructions would do.
func TestTheWebBinaryAnswersTheTrackCommandRatherThanStarting(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, harness.Binary(), "track", "status")
	cmd.Dir = t.TempDir()

	out, err := cmd.CombinedOutput()

	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 2 || !strings.Contains(string(out), "no terminal client") {
		t.Errorf("track on the web binary answered %v %q, want 2 and that there is no terminal client", err, out)
	}
}
