package test

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// endsInSOnPurpose are the directory names that end in an s and may, each with
// its reason. Not a list of exceptions to grow: a new plural is renamed instead.
var endsInSOnPurpose = map[string]string{
	"configs":   "GoFr reads ./configs from the working directory, and every installation keeps its configuration there",
	"workflows": "GitHub looks for workflows in .github/workflows and nowhere else",
	"res":       "the Android build reads an app's resources from src/main/res, where every Android project keeps them",
	"kerberos":  "a name, not a plural",
}

// Every directory is named in the singular - locale, migration, asset, image -
// as the maintainer decided on 2026-10-29, when the five plural ones were
// renamed.
//
// Read from the names themselves: one ending in an s, but not in ss - harness,
// progress - is taken for a plural unless endsInSOnPurpose says why it is not,
// so a new directory called helpers or fixtures is reported here rather than
// found standing beside the renamed ones. Irregular plurals get past it; a rule
// that needed a dictionary to apply would not be applied.
func TestEveryDirectoryIsNamedInTheSingular(t *testing.T) {
	root := ".."
	looked := 0

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() || path == root {
			return nil
		}

		name := entry.Name()

		// Tools' directories and what builds leave behind are not the project's
		// structure. .github is the one hidden directory the project writes.
		if (strings.HasPrefix(name, ".") && name != ".github") || name == "bin" || name == "dist" {
			return filepath.SkipDir
		}

		looked++

		if _, allowed := endsInSOnPurpose[name]; allowed {
			// What is inside configs is GoFr's to name as well - its TLS cache
			// is configs/certs.
			if name == "configs" {
				return filepath.SkipDir
			}

			return nil
		}

		if strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "ss") {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s is named in the plural; this tree names its directories in the singular",
				filepath.ToSlash(rel))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}

	if looked < 40 {
		t.Fatalf("only %d directories were looked at; the walk has stopped reading the tree", looked)
	}
}

// filesEndingInSOnPurpose are the files whose names end in an s and may, by their
// path from the root, each with its reason: another program reads them by that
// name. Not a list to grow either.
var filesEndingInSOnPurpose = map[string]string{
	".gitattributes":                                "git reads .gitattributes and no other name",
	".vscode/extensions.json":                       "VS Code reads its workspace's settings under these names",
	".vscode/settings.json":                         "VS Code reads its workspace's settings under these names",
	".vscode/tasks.json":                            "VS Code reads its workspace's settings under these names",
	"internal/interface/mobile/settings.gradle.kts": "Gradle reads settings.gradle.kts and no other name",
	"internal/interface/web/asset/api-docs.html":    "named after /api-docs, the address the API's documentation is published at",
	"internal/interface/web/asset/api-docs.js":      "named after /api-docs, the address the API's documentation is published at",
}

// singularWords are words a file's name may end in that end in an s without
// being plurals.
var singularWords = []string{"kerberos", "hsts", "focus", "basis"}

// Every file is named in the singular too - form.go, migration.go, code.go,
// doc/feature.md - as the maintainer decided on 2026-11-03, a week after the
// directories, when the hundred and five plural ones were renamed.
//
// The same reading of the name as for a directory, of every file git keeps -
// what a build or a test leaves behind is not the project's to name - after
// taking off what Go reads from the end of a name: an extension, _test, and
// _windows, which is a platform rather than a plural. A name that reads as a
// verb goes to a noun rather than to a singular nobody would say: what a
// sign-out leaves behind is signoutleftover_test.go.
func TestEveryFileIsNamedInTheSingular(t *testing.T) {
	// safe.directory, because in the Linux container the suite runs in here the
	// mounted repository belongs to another user and git refuses it otherwise.
	out, err := exec.Command("git", "-c", "safe.directory=*", "-C", "..", "ls-files").Output()
	if err != nil {
		t.Fatalf("asking git which files it keeps: %v", err)
	}

	files := strings.Fields(string(out))
	if len(files) < 500 {
		t.Fatalf("git listed %d files; the reading has stopped finding them", len(files))
	}

	for _, path := range files {
		name := strings.ToLower(stemOf(filepath.Base(path)))

		if !strings.HasSuffix(name, "s") || strings.HasSuffix(name, "ss") {
			continue
		}

		if _, allowed := filesEndingInSOnPurpose[path]; allowed {
			continue
		}

		if slices.ContainsFunc(singularWords, func(word string) bool { return strings.HasSuffix(name, word) }) {
			continue
		}

		// A last part of two letters is a code rather than a word - holiday_us.go
		// holds the public holidays of the United States - and no plural is two
		// letters long.
		if parts := strings.Split(name, "_"); len(parts) > 1 && len(parts[len(parts)-1]) == 2 {
			continue
		}

		t.Errorf("%s is named in the plural; this tree names its files in the singular", path)
	}
}

// stemOf is a file's name without its extensions and without _test or _windows,
// keeping the dot a hidden file's name starts with.
func stemOf(name string) string {
	stem := name
	if dot := strings.Index(name[1:], "."); dot >= 0 {
		stem = name[:dot+1]
	}

	return strings.TrimSuffix(strings.TrimSuffix(stem, "_windows"), "_test")
}
