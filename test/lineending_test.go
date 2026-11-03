package test

import (
	"os/exec"
	"strings"
	"testing"
)

// Every text file in the tree has its line endings named in .gitattributes.
//
// A file that falls through to `* text=auto` takes its endings from core.eol,
// which a clone leaves unset and so native - CRLF on Windows - whatever
// core.autocrlf says: measured, gradle.properties came back w/crlf when it was
// deleted and checked out again in a clone set to core.autocrlf=false. The gap
// has always been a suffix nobody pinned - *.conf missing slapd.conf.template,
// then *.svg, then *.json.in, then the Android app's XML - so this asks git
// which files fall through rather than trusting the list to be complete.
func TestEveryTextFileHasItsLineEndingsPinned(t *testing.T) {
	// safe.directory, because in the Linux container the suite runs in here the
	// mounted repository belongs to another user and git refuses it otherwise.
	out, err := exec.Command("git", "-c", "safe.directory=*", "-C", "..", "ls-files", "--eol").Output()
	if err != nil {
		t.Fatalf("asking git how it treats each file's endings: %v", err)
	}

	listed := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(listed) < 500 {
		t.Fatalf("git listed %d files; the reading has stopped finding them", len(listed))
	}

	for _, line := range listed {
		meta, path, _ := strings.Cut(line, "\t")
		fields := strings.Fields(meta)

		// A file git took for binary has no endings to have.
		if len(fields) == 3 && fields[2] == "attr/text=auto" && fields[0] != "i/-text" && fields[0] != "i/none" {
			t.Errorf("%s has no line endings of its own in .gitattributes, so a checkout on Windows writes it with CRLF", path)
		}
	}
}
