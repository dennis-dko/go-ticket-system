package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// formerName matches what the project was called before 2026-11-17 however it
// is written - the three words joined by a hyphen, an underscore, a space or
// nothing, in any case - and the three-letter abbreviation its cookies, token
// prefixes, variables and sealed secrets carried, as a word of its own: not
// after a letter or a digit, nor after the + or / that would make it the inside
// of a hash. Both are spelled so that this file does not match them.
var formerName = regexp.MustCompile(`(?i)go[-_ ]?time[-_ ]?recording|(^|[^a-z0-9+/])g[t]r`)

// oldNameKeptOnPurpose are the files that may still use the former name, each
// with its reason. A row whose file has stopped using it fails as well, so the
// list cannot go on excusing what is no longer there.
var oldNameKeptOnPurpose = map[string]string{
	"doc/audit-ledger.md": "it quotes what a file said on the day it was read, and a record is not rewritten",
	"internal/infrastructure/persistence/migration/migration.go": "it converts, once, the marker that secrets sealed before " +
		"the rename carry in a database from then",
	"internal/infrastructure/persistence/migration/seal_marker_test.go": "it seals secrets the old way to prove that conversion",
}

// The project was renamed to TimePulse on 2026-11-17, and the sweep that did it
// looked for the spellings it thought of. One got past it: the Android app's
// download keeper marked every page it prepared with the former name in camel
// case, which nothing reported, because nothing reads that mark but the script
// that sets it. So every spelling is looked for here at once, in every text
// file git keeps.
func TestNothingStillCarriesTheFormerName(t *testing.T) {
	// safe.directory, because in the Linux container the suite runs in here the
	// mounted repository belongs to another user and git refuses it otherwise.
	out, err := exec.Command("git", "-c", "safe.directory=*", "-C", "..", "ls-files", "--eol").Output()
	if err != nil {
		t.Fatalf("asking git which files it keeps: %v", err)
	}

	listed := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(listed) < 500 {
		t.Fatalf("git listed %d files; the reading has stopped finding them", len(listed))
	}

	usesIt := map[string]bool{}

	for _, line := range listed {
		meta, path, _ := strings.Cut(line, "\t")
		if strings.HasPrefix(meta, "i/-text") {
			continue
		}

		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("reading %s: %v", path, err)

			continue
		}

		for n, text := range strings.Split(string(content), "\n") {
			if !formerName.MatchString(text) {
				continue
			}

			usesIt[path] = true

			if _, kept := oldNameKeptOnPurpose[path]; !kept {
				t.Errorf("%s:%d still uses the project's former name: %s", path, n+1, strings.TrimSpace(text))
			}
		}
	}

	for path, reason := range oldNameKeptOnPurpose {
		if !usesIt[path] {
			t.Errorf("%s no longer uses the former name, so its row (%q) excuses nothing; remove it", path, reason)
		}
	}
}
