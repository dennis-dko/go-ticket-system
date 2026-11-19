package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A backup kept in the directory is listed, newest first, read back whole and
// removed; and nothing in the folder that is not a backup is listed, read or
// removed, whatever name a request asks for.
func TestTheDirectoryKeepsBackupsAndNothingElse(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "backups")
	dir := NewDirectory(folder)

	if stored, err := dir.List(); err != nil || len(stored) != 0 {
		t.Fatalf("a folder that is not there yet lists %v, %v", stored, err)
	}

	older := Name(time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC))
	newer := Name(time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC))

	for _, name := range []string{older, newer} {
		if err := dir.Keep(name, []byte("sealed "+name)); err != nil {
			t.Fatalf("keep %s: %v", name, err)
		}
	}

	if err := dir.Keep(newer, []byte("again")); err == nil {
		t.Error("a backup was written over one of the same name")
	}

	// Somebody else's file in the same folder.
	if err := os.WriteFile(filepath.Join(folder, "notes.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}

	stored, err := dir.List()
	if err != nil || len(stored) != 2 || stored[0].Name != newer || stored[1].Name != older {
		t.Fatalf("listed %+v, %v; want the two backups, the newer first", stored, err)
	}

	if !stored[0].Made.Equal(time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC)) || stored[0].Size == 0 {
		t.Errorf("the newer backup is listed as %+v", stored[0])
	}

	if read, err := dir.Read(older); err != nil || string(read) != "sealed "+older {
		t.Errorf("read %q, %v", read, err)
	}

	for _, name := range []string{"notes.txt", "../backups/notes.txt", filepath.Join("..", older)} {
		if _, err := dir.Read(name); !errors.Is(err, ErrNoSuchBackup) {
			t.Errorf("reading %q: %v, want ErrNoSuchBackup", name, err)
		}

		if err := dir.Remove(name); !errors.Is(err, ErrNoSuchBackup) {
			t.Errorf("removing %q: %v, want ErrNoSuchBackup", name, err)
		}
	}

	if err := dir.Remove(older); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := dir.Read(older); !errors.Is(err, ErrNoSuchBackup) {
		t.Errorf("a removed backup reads as %v", err)
	}

	if _, err := os.Stat(filepath.Join(folder, "notes.txt")); err != nil {
		t.Errorf("the file that is not a backup was touched: %v", err)
	}
}
