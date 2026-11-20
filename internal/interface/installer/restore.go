package installer

import (
	"encoding/base64"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	appservice "github.com/dennis-dko/timepulse/internal/application/v1/service"
	appconfig "github.com/dennis-dko/timepulse/internal/infrastructure/config"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/apperror"
	"github.com/dennis-dko/timepulse/internal/support/backup"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// pendingRestore is a backup the installer has opened once, to be sure it can
// be, and hands to the application to restore after the migrations - with what
// to do with secrets this process's key cannot open: the key they came under,
// or leave them behind.
type pendingRestore struct {
	file           []byte
	passphrase     string
	secretKey      string
	withoutSecrets bool
}

// restoreAsked is the backup an answer asks to restore, once the questions a
// restore raises have been answered here, while this screen can still say so:
// asked by the application, each would be a start that fails.
func (s *server) restoreAsked(r *http.Request, given answer, ds appconfig.Datasource) (*pendingRestore, error) {
	if !given.Restore {
		return nil, nil
	}

	if given.Demo {
		return nil, apperror.Invalidf("a new database is filled with demo data or from a backup, not both").
			WithCode("demoOrRestore")
	}

	s.mu.Lock()
	restore := s.restore
	s.mu.Unlock()

	if restore == nil {
		return nil, apperror.Invalidf("no backup has been uploaded to restore").WithCode("noBackupUploaded")
	}

	holds, err := appconfig.HoldsAccounts(r.Context(), ds)
	if err != nil {
		return nil, probeFailure(err)
	}

	if holds {
		return nil, apperror.Invalidf("the database already holds accounts; a backup is only restored into " +
			"an empty one").WithCode("restoreNeedsEmptyDatabase")
	}

	return restore, nil
}

// backupUploadTime is how long an upload of a backup may take once its token
// has been checked: the largest file over a slow line, and not for ever.
const backupUploadTime = 15 * time.Minute

// backup takes the backup a new installation is to start from, and opens it
// once, so that a wrong passphrase or a file that is no backup is said on this
// screen rather than at the start that would restore it.
//
// The file is the body as it is. The passphrase comes in a header, base64 of its
// UTF-8: a browser sends only Latin-1 in a header, and a passphrase may hold any
// character at all.
func (s *server) backup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))

		return
	}

	if !s.holdsTheToken(r) {
		writeError(w, http.StatusUnauthorized,
			apperror.Invalidf("the setup token is wrong - it is printed in this "+
				"process's log").WithCode("wrongSetupToken"))

		return
	}

	// The read deadline every other request keeps is thirty seconds, which a
	// backup over a slow line does not fit in.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(backupUploadTime))

	file, err := io.ReadAll(http.MaxBytesReader(w, r.Body, backup.MaxFileSize))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge,
			apperror.Invalidf("the backup could not be read whole: %v", err).
				WithCode("backupTooLarge", strconv.Itoa(backup.MaxFileSize>>20)))

		return
	}

	passphrase, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Backup-Passphrase"))
	if err != nil {
		writeError(w, http.StatusBadRequest, backupRefusal(backup.ErrWrongPassphrase))

		return
	}

	schema := s.cfg.Schema
	if schema == 0 {
		schema = math.MaxInt64
	}

	// Read whole, not only its first line: everything the restore would refuse
	// is refused here, while there is still a screen to say it on.
	var contents sqldb.BackupContents

	err = backup.Open(file, string(passphrase), func(content io.Reader) error {
		contents, err = sqldb.InspectBackup(content, schema)

		return err
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, backupRefusal(err))

		return
	}

	pending := &pendingRestore{file: file, passphrase: string(passphrase)}

	// The key the backup's installation had comes as the passphrase does, as
	// base64 of what was typed; one that does not decode opens nothing.
	secretKey, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Backup-Secret-Key"))
	if err != nil {
		secretKey = []byte("not a key")
	}

	if contents.KeyCheck != "" {
		err := s.secretsOf(contents.KeyCheck, strings.TrimSpace(string(secretKey)),
			r.Header.Get("X-Backup-Without-Secrets") == "true", pending)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)

			return
		}
	}

	s.mu.Lock()
	s.restore = pending
	s.mu.Unlock()

	header := contents.Header

	secrets := ""

	switch {
	case pending.secretKey != "":
		secrets = "; its secrets are sealed again under this installation's key"
	case pending.withoutSecrets:
		secrets = "; its secrets are left behind"
	}

	s.cfg.Logf("a backup %s is ready to restore%s", header.Describe(), secrets)

	writeJSON(w, http.StatusOK, map[string]any{
		"application": header.Application, "dialect": header.Dialect, "createdAt": header.CreatedAt,
	})
}

// backupRefusal is why a backup was not taken, coded for the page to say.
func backupRefusal(err error) error {
	switch {
	case errors.Is(err, backup.ErrWrongPassphrase):
		return apperror.Invalidf("the passphrase does not open this backup").WithCode("backupWrongPassphrase")
	case errors.Is(err, backup.ErrNewerFormat), errors.Is(err, sqldb.ErrNewerSchema):
		return apperror.Invalidf("a newer version of TimePulse wrote this backup").WithCode("backupFromNewerVersion")
	case errors.Is(err, sqldb.ErrCutShort):
		return apperror.Invalidf("the backup ends before its last line: it was cut short on its way here, " +
			"so download it again").WithCode("backupCutShort")
	case errors.Is(err, backup.ErrTooLarge):
		return apperror.Invalidf("the backup is larger than a backup may be").
			WithCode("backupTooLarge", strconv.Itoa(backup.MaxFileSize>>20))
	default:
		return apperror.Invalidf("this is not a TimePulse backup").WithCode("notABackup")
	}
}

// secretsOf decides what the restore does with a backup's secrets, sealed where
// keyCheck was kept: nothing, where this process's key opens them; seal them
// again under it with the key they came under, where that is given and opens
// them; leave them behind, where that is asked; and otherwise ask for one of the
// two, saying why this process's own key does not do.
func (s *server) secretsOf(keyCheck, secretKey string, without bool, pending *pendingRestore) error {
	own := appservice.CheckSecretKey(s.cfg.Secrets, keyCheck)
	if own == nil {
		return nil
	}

	switch {
	case secretKey != "":
		old, err := security.NewSealer(secretKey)
		if err == nil {
			err = appservice.CheckSecretKey(old, keyCheck)
		}

		if err != nil {
			return apperror.Invalidf("the SECRET_KEY given does not open this backup's secrets").
				WithCode("backupSecretKeyWrong")
		}

		pending.secretKey = secretKey
	case without:
		pending.withoutSecrets = true
	default:
		return keyRefusal(own)
	}

	return nil
}

// keyRefusal is why a backup whose secrets this process's SECRET_KEY cannot open
// was not taken without more, coded for the page to ask for the key the backup's
// installation had, or whether to restore it without its secrets.
func keyRefusal(err error) error {
	if errors.Is(err, appservice.ErrSecretKeyMissing) {
		return apperror.Invalidf("this backup was made where SECRET_KEY was set, and its second factors, " +
			"directory password and other secrets are sealed with that key: give the SECRET_KEY of the " +
			"installation it was made on, or restore it without its secrets").WithCode("backupNeedsSecretKey")
	}

	return apperror.Invalidf("this backup's secrets were sealed with another SECRET_KEY than the one set " +
		"here: give the SECRET_KEY of the installation it was made on, or restore it without its secrets").
		WithCode("backupOtherSecretKey")
}
