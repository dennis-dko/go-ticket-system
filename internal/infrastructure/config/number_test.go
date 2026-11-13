package config_test

import (
	"math"
	"strings"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/config"
)

// A figure in the configuration that is not a finite number falls back to the
// default, as one that cannot be read at all does.
//
// ParseFloat reads "NaN" and "Inf", and NaN compares false against every bound -
// so a reader that checks "below zero or above one" lets it through. TRACER_RATIO
// already catches it, for the reason written there: the value is sent to the
// settings screen, JSON cannot encode it, and the screen then renders nothing.
// The reader beside it did not, and what it feeds is worse than a blank screen:
// the deletion limit is the guard on how much of the directory one
// synchronisation may remove, and NaN compares false against it too.
func TestAFigureThatIsNotAFiniteNumberFallsBack(t *testing.T) {
	defaults := config.Load(mapConfig{})

	for _, raw := range []string{"NaN", "nan", "Inf", "+Inf", "-Inf", "infinity"} {
		got := config.Load(mapConfig{"LDAP_SYNC_MAX_DELETE_RATIO": raw}).LDAPSyncMaxDeleteRatio

		if math.IsNaN(got) || math.IsInf(got, 0) || got != defaults.LDAPSyncMaxDeleteRatio {
			t.Errorf("LDAP_SYNC_MAX_DELETE_RATIO=%q was read as %v, want the default %v",
				raw, got, defaults.LDAPSyncMaxDeleteRatio)
		}
	}
}

// A value that cannot be used is said, as well as replaced.
//
// The fallback is right - an installation has to start whatever its environment
// holds - and the silence was not. TLS_ENABLED=yes is not a value ParseBool
// reads, so TLS was off, and the start then told somebody who had switched it
// on that "TLS_ENABLED is false". Every setting read here fell back the same
// way, without a word.
func TestAValueThatCannotBeUsedIsNamed(t *testing.T) {
	cfg := config.Load(mapConfig{
		"TLS_ENABLED":       "yes",
		"SESSION_LIFETIME":  "12hours",
		"RATE_LIMIT":        "-3",
		"TLS_REDIRECT_PORT": "8080",
		"SESSION_IDLE":      "0",
	})

	named := strings.Join(cfg.Unusable, "\n")

	for _, key := range []string{"TLS_ENABLED", "SESSION_LIFETIME", "RATE_LIMIT"} {
		if !strings.Contains(named, key) {
			t.Errorf("%s could not be used and nothing says so: %q", key, cfg.Unusable)
		}
	}

	// A value that was used as given is not a problem, and neither is 0 where 0
	// means something: an idle timeout of 0 is no idle timeout.
	for _, key := range []string{"TLS_REDIRECT_PORT", "SESSION_IDLE"} {
		if strings.Contains(named, key) {
			t.Errorf("%s was used as given and is still reported: %q", key, cfg.Unusable)
		}
	}

	if quiet := config.Load(mapConfig{}).Unusable; len(quiet) != 0 {
		t.Errorf("an installation that sets nothing is told %q", quiet)
	}
}

// HSTS_MAX_AGE=0 switches the header off.
//
// The middleware has always read zero that way, and the reader turned it into a
// year before the middleware saw it. So an installation that set 0 because it
// makes its own certificate told every browser that trusted it to accept nothing
// less for a year, and a certificate that lapsed or was replaced would have left
// those browsers no way past the warning - the lock-out the setting was there to
// prevent - while the start said only that 0 "cannot be used".
func TestZeroSwitchesHSTSOffRatherThanFallingBack(t *testing.T) {
	cfg := config.Load(mapConfig{"HSTS_MAX_AGE": "0"})

	if cfg.HSTSMaxAge != 0 {
		t.Errorf("HSTS_MAX_AGE=0 is read as %s rather than as off", cfg.HSTSMaxAge)
	}

	if len(cfg.Unusable) != 0 {
		t.Errorf("HSTS_MAX_AGE=0 is reported as a value that cannot be used: %q", cfg.Unusable)
	}

	// Below zero is still a mistake, and still says so.
	cfg = config.Load(mapConfig{"HSTS_MAX_AGE": "-1h"})

	if cfg.HSTSMaxAge <= 0 || !strings.Contains(strings.Join(cfg.Unusable, "\n"), "HSTS_MAX_AGE") {
		t.Errorf("HSTS_MAX_AGE=-1h is read as %s, and the start is told %q", cfg.HSTSMaxAge, cfg.Unusable)
	}
}
