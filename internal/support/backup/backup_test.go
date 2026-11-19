package backup

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

const passphrase = "a passphrase long enough"

func seal(t *testing.T, content string) []byte {
	t.Helper()

	file, err := Seal(passphrase, func(w io.Writer) error {
		_, err := io.WriteString(w, content)

		return err
	})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	return file
}

func open(file []byte, passphrase string) (string, error) {
	var content string

	err := Open(file, passphrase, func(r io.Reader) error {
		read, err := io.ReadAll(r)
		content = string(read)

		return err
	})

	return content, err
}

// What is sealed opens again with the same passphrase, and nothing of it can be
// read in the file itself.
func TestABackupOpensWithItsPassphrase(t *testing.T) {
	const content = `{"format":"timepulse-backup"}` + "\n" + `[1,"Wera","wera@example.com"]`

	file := seal(t, content)

	if strings.Contains(string(file), "wera@example.com") {
		t.Fatal("the sealed file carries what it holds in the clear")
	}

	got, err := open(file, passphrase)
	if err != nil || got != content {
		t.Fatalf("opened as %q, %v; want %q", got, err, content)
	}
}

// Another passphrase opens nothing, and neither does a file with any byte of it
// changed: the header is authenticated with the rest, so a changed salt or cost
// is refused as surely as a changed row.
func TestABackupIsRefusedWholeWhenItCannotBeTrusted(t *testing.T) {
	file := seal(t, "the rows")

	if _, err := open(file, "another passphrase entirely"); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("another passphrase: %v, want ErrWrongPassphrase", err)
	}

	for _, at := range []int{len(magic) + 2, headerLength - 1, headerLength + 3, len(file) - 1} {
		changed := append([]byte(nil), file...)
		changed[at] ^= 0x01

		if _, err := open(changed, passphrase); !errors.Is(err, ErrWrongPassphrase) {
			t.Errorf("a bit changed at byte %d: %v, want ErrWrongPassphrase", at, err)
		}
	}

	if _, err := open(file[:len(file)-1], passphrase); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("a file cut short: %v, want ErrWrongPassphrase", err)
	}
}

// A file that is not a backup says so, rather than which passphrase it would
// have wanted; and one from a later format says that instead.
func TestAFileThatIsNotABackupSaysSo(t *testing.T) {
	for name, file := range map[string][]byte{
		"empty":       nil,
		"a workbook":  []byte("PK\x03\x04 and the rest of a zip"),
		"only header": []byte(magic),
	} {
		if _, err := open(file, passphrase); !errors.Is(err, ErrNotABackup) {
			t.Errorf("%s: %v, want ErrNotABackup", name, err)
		}
	}

	newer := seal(t, "rows")
	newer[len(magic)] = formatVersion + 1

	if _, err := open(newer, passphrase); !errors.Is(err, ErrNewerFormat) {
		t.Errorf("a later format: %v, want ErrNewerFormat", err)
	}
}

// The cost of deriving the key is in the header, so a file cannot be trusted to
// name a sane one: asking for 2^30 would hold a processor and a gigabyte of
// memory before anything had been authenticated. Refused at once instead.
func TestAFileCannotAskForAnyCostItLikes(t *testing.T) {
	file := seal(t, "rows")
	file[len(magic)+1] = 30

	started := time.Now()

	if _, err := open(file, passphrase); !errors.Is(err, ErrNotABackup) {
		t.Errorf("a cost of 2^30: %v, want ErrNotABackup", err)
	}

	if took := time.Since(started); took > time.Second {
		t.Errorf("refusing the cost took %v, so it was paid first", took)
	}
}

// A passphrase too short to protect a file that leaves the server seals none.
func TestAShortPassphraseSealsNothing(t *testing.T) {
	_, err := Seal("short", func(io.Writer) error { return nil })
	if !errors.Is(err, ErrWeakPassphrase) {
		t.Errorf("a five-character passphrase: %v, want ErrWeakPassphrase", err)
	}
}

// What a file decompresses to is bounded, and running past the bound is an
// error rather than an end: an end would read as a file that simply stopped.
// Asked of the reader the bound is kept by, because a real file four gigabytes
// past it is not something to make in a test.
func TestReadingPastTheBoundIsAnError(t *testing.T) {
	within := &bounded{from: strings.NewReader("abc"), left: 3}
	if got, err := io.ReadAll(within); err != nil || string(got) != "abc" {
		t.Errorf("exactly the bound: %q, %v", got, err)
	}

	past := &bounded{from: strings.NewReader("abcd"), left: 3}
	if _, err := io.ReadAll(past); !errors.Is(err, ErrTooLarge) {
		t.Errorf("one byte past the bound: %v, want ErrTooLarge", err)
	}
}

// The names a backup is given are the only ones read from its directory, so a
// name that climbs out of it is never one of them.
func TestOnlyABackupsOwnNameIsValid(t *testing.T) {
	made := Name(time.Date(2026, 11, 19, 21, 30, 5, 0, time.FixedZone("CET", 3600)))
	if made != "timepulse-20261119-203005.tpbak" {
		t.Errorf("a backup made at 21:30:05 CET is named %q, want it named in UTC", made)
	}

	if !ValidName(made) {
		t.Errorf("%q, a name Name gives, is not valid", made)
	}

	for _, name := range []string{
		"", "../timepulse-20261119-203005.tpbak", "timepulse-20261119-203005.tpbak/../x",
		"timepulse-20261119-203005.tpbak.exe", `..\timepulse-20261119-203005.tpbak`,
		"timepulse-2026111-203005.tpbak", "datasource.json",
	} {
		if ValidName(name) {
			t.Errorf("%q is valid", name)
		}
	}
}
