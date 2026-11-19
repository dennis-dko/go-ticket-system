package backup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ErrNoSuchBackup says the directory holds no backup of that name - which is
// also the answer for a name Name would never give, so a name made up to reach
// another file reads as one that is simply not there.
var ErrNoSuchBackup = errors.New("there is no backup of that name")

// Directory is the folder backups are kept in on the server. It holds nothing
// this package reads but files named as Name names them; anything else put
// there is passed over.
type Directory struct {
	path string
}

// NewDirectory is the folder at path. It is made with the first backup kept in
// it, readable by the server's own user and nobody else.
func NewDirectory(path string) Directory {
	return Directory{path: path}
}

// Path is where the backups are kept, for saying so.
func (d Directory) Path() string {
	return d.path
}

// Stored is a backup in the directory.
type Stored struct {
	Name string
	Size int64

	// Made is when it was made, read from its name rather than from the file,
	// which a copy or a restore of the folder would change.
	Made time.Time
}

// Keep writes a backup under its name.
//
// Staged and forced to the disk before it takes its name, so a power cut leaves
// either the backup or nothing - never a file of the right name that is cut
// short. A name that is taken is refused rather than written over.
func (d Directory) Keep(name string, file []byte) error {
	if !ValidName(name) {
		return fmt.Errorf("%q is not a name a backup is given", name)
	}

	if err := os.MkdirAll(d.path, 0o700); err != nil {
		return fmt.Errorf("make the backup folder %s: %w", d.path, err)
	}

	target := filepath.Join(d.path, name)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("a backup named %s is there already", name)
	}

	staged, err := os.CreateTemp(d.path, ".staging-*")
	if err != nil {
		return fmt.Errorf("stage the backup in %s: %w", d.path, err)
	}

	defer func() { _ = os.Remove(staged.Name()) }()

	if _, err := staged.Write(file); err != nil {
		_ = staged.Close()

		return fmt.Errorf("write the backup: %w", err)
	}

	if err := staged.Sync(); err != nil {
		_ = staged.Close()

		return fmt.Errorf("write the backup to the disk: %w", err)
	}

	if err := staged.Close(); err != nil {
		return fmt.Errorf("close the backup: %w", err)
	}

	return os.Rename(staged.Name(), target)
}

// List names the backups in the directory, the newest first. A folder that is
// not there yet holds none.
func (d Directory) List() ([]Stored, error) {
	entries, err := os.ReadDir(d.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read the backup folder %s: %w", d.path, err)
	}

	var stored []Stored

	for _, entry := range entries {
		if entry.IsDir() || !ValidName(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		made, err := time.Parse("20060102-150405",
			strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "timepulse-"), Extension))
		if err != nil {
			continue
		}

		stored = append(stored, Stored{Name: entry.Name(), Size: info.Size(), Made: made})
	}

	slices.SortFunc(stored, func(a, b Stored) int { return b.Made.Compare(a.Made) })

	return stored, nil
}

// Read is the backup of that name, whole.
func (d Directory) Read(name string) ([]byte, error) {
	if !ValidName(name) {
		return nil, ErrNoSuchBackup
	}

	path := filepath.Join(d.path, name)

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoSuchBackup
	}

	if err != nil {
		return nil, err
	}

	if info.Size() > MaxFileSize {
		return nil, ErrTooLarge
	}

	return os.ReadFile(path)
}

// Remove deletes the backup of that name.
func (d Directory) Remove(name string) error {
	if !ValidName(name) {
		return ErrNoSuchBackup
	}

	err := os.Remove(filepath.Join(d.path, name))
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNoSuchBackup
	}

	return err
}
