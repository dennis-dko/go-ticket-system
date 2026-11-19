package backup

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/scrypt"
)

const (
	// Extension is what every backup file is named with.
	Extension = ".tpbak"

	// MinPassphraseLength is the shortest passphrase a backup is sealed with.
	// The file is meant to leave the server - a download, a copy on a laptop - so
	// the passphrase is all that stands between it and whoever holds it next.
	MinPassphraseLength = 12

	// MaxFileSize is the largest file read back, which is also what the installer
	// accepts as an upload.
	MaxFileSize = 256 << 20

	// MaxPayloadSize bounds what a file decompresses to. Measured rather than
	// guessed: twenty thousand entries make 2.4 MB of lines and 186 KB compressed,
	// 13:1, so a real backup at MaxFileSize comes to about 3.3 GiB and stays below
	// this. Deflate itself reaches about a thousand to one, and a file that does
	// was made to.
	MaxPayloadSize = 4 << 30
)

// The header: the magic, the format, the scrypt cost and the salt.
const (
	formatVersion = 1
	scryptLogN    = 15
	scryptR       = 8
	scryptP       = 1
	keyLength     = 32
	saltLength    = 16
	headerLength  = len(magic) + 2 + saltLength

	// minLogN and maxLogN bound the cost a file may ask for. The cost is in the
	// header, and scrypt's memory is 128·r·N bytes: a header asking for 2^20 would
	// take a gigabyte, and 2^30 a terabyte, before the first byte had been
	// authenticated. 2^17 is 128 MiB, room for a later version to ask more than
	// the 32 MiB this one does.
	minLogN = 10
	maxLogN = 17
)

// magic opens every backup file.
const magic = "TPBACKUP"

var (
	// ErrNotABackup says the file is not one this package writes.
	ErrNotABackup = errors.New("this is not a TimePulse backup")

	// ErrWrongPassphrase says the passphrase does not open the file - or the file
	// was changed, which an authenticated cipher cannot tell apart.
	ErrWrongPassphrase = errors.New("the passphrase does not open this backup, or the file has been changed")

	// ErrNewerFormat says a later version wrote the file.
	ErrNewerFormat = errors.New("this backup was written by a newer version of TimePulse")

	// ErrTooLarge says the file, or what it decompresses to, is past its bound.
	ErrTooLarge = errors.New("the backup is larger than a backup may be")

	// ErrWeakPassphrase says the passphrase is too short to seal a file with.
	ErrWeakPassphrase = fmt.Errorf("the passphrase is shorter than %d characters", MinPassphraseLength)
)

// Seal compresses what write produces and encrypts it with the passphrase.
//
// The whole file is made in memory: a backup compresses to megabytes, and an
// authenticated cipher has to see the whole of it before anything may be
// trusted - so reading it back holds it whole anyway.
func Seal(passphrase string, write func(io.Writer) error) ([]byte, error) {
	if utf8.RuneCountInString(passphrase) < MinPassphraseLength {
		return nil, ErrWeakPassphrase
	}

	var plain bytes.Buffer

	compressed := gzip.NewWriter(&plain)

	if err := write(compressed); err != nil {
		return nil, err
	}

	if err := compressed.Close(); err != nil {
		return nil, fmt.Errorf("compress the backup: %w", err)
	}

	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("make the backup's salt: %w", err)
	}

	header := make([]byte, 0, headerLength)
	header = append(header, magic...)
	header = append(header, formatVersion, scryptLogN)
	header = append(header, salt...)

	sealer, err := cipherFor(passphrase, salt, scryptLogN)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, sealer.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("make the backup's nonce: %w", err)
	}

	file := append(header, nonce...)

	return sealer.Seal(file, nonce, plain.Bytes(), header), nil
}

// Open decrypts a backup and hands read what it holds, decompressed and bounded:
// reading past MaxPayloadSize fails with ErrTooLarge rather than ending as if
// the file did.
func Open(file []byte, passphrase string, read func(io.Reader) error) error {
	if len(file) > MaxFileSize {
		return ErrTooLarge
	}

	if len(file) < headerLength || string(file[:len(magic)]) != magic {
		return ErrNotABackup
	}

	switch version := file[len(magic)]; {
	case version > formatVersion:
		return ErrNewerFormat
	case version != formatVersion:
		return ErrNotABackup
	}

	logN := file[len(magic)+1]
	if logN < minLogN || logN > maxLogN {
		return ErrNotABackup
	}

	header, salt := file[:headerLength], file[len(magic)+2:headerLength]

	opener, err := cipherFor(passphrase, salt, logN)
	if err != nil {
		return err
	}

	if len(file) < headerLength+opener.NonceSize()+opener.Overhead() {
		return ErrNotABackup
	}

	nonce := file[headerLength : headerLength+opener.NonceSize()]

	plain, err := opener.Open(nil, nonce, file[headerLength+opener.NonceSize():], header)
	if err != nil {
		return ErrWrongPassphrase
	}

	decompressed, err := gzip.NewReader(bytes.NewReader(plain))
	if err != nil {
		return ErrNotABackup
	}

	defer func() { _ = decompressed.Close() }()

	return read(&bounded{from: decompressed, left: MaxPayloadSize})
}

// cipherFor derives the file's key from the passphrase and its salt.
func cipherFor(passphrase string, salt []byte, logN byte) (cipher.AEAD, error) {
	key, err := scrypt.Key([]byte(passphrase), salt, 1<<logN, scryptR, scryptP, keyLength)
	if err != nil {
		return nil, fmt.Errorf("derive the backup's key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("make the backup's cipher: %w", err)
	}

	return cipher.NewGCM(block)
}

// bounded reads on until left runs out, and then refuses rather than ends - an
// end there would read as a file that stopped, not one that went on.
type bounded struct {
	from io.Reader
	left int64
}

func (b *bounded) Read(p []byte) (int, error) {
	if b.left <= 0 {
		var probe [1]byte
		if n, _ := b.from.Read(probe[:]); n > 0 {
			return 0, ErrTooLarge
		}

		return 0, io.EOF
	}

	if int64(len(p)) > b.left {
		p = p[:b.left]
	}

	n, err := b.from.Read(p)
	b.left -= int64(n)

	return n, err
}

// Name is the file name of a backup made at the moment given, in UTC so that
// the names sort as the moments do.
func Name(at time.Time) string {
	return "timepulse-" + at.UTC().Format("20060102-150405") + Extension
}

// namePattern is every name Name gives, and nothing else.
var namePattern = regexp.MustCompile(`^timepulse-\d{8}-\d{6}\.tpbak$`)

// ValidName reports whether a name is one Name gives. Nothing else is ever read
// from or deleted in the backup directory, so a name that came with a request
// cannot reach anything outside it.
func ValidName(name string) bool {
	return namePattern.MatchString(name)
}
