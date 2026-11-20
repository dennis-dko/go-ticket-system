package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"

	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/migration"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/backup"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// newestSchema is the newest migration this binary carries, which the installer
// holds an uploaded backup's schema to. The chain is the same for every dialect;
// only what each step runs differs.
func newestSchema() int64 {
	return slices.Max(slices.Collect(maps.Keys(migration.All(sqldb.DialectSQLite))))
}

// restoreRequest is the backup the installer was given and what came with it:
// the passphrase it opens with, and what to do with secrets sealed under a
// SECRET_KEY this installation does not have - the key they came under, or leave
// them behind. Named fields rather than parameters, because the passphrase and
// the key are both strings and swapping them would compile.
type restoreRequest struct {
	file           []byte
	passphrase     string
	secretKey      string
	withoutSecrets bool
}

// restoreBackup puts the backup the installer was given into the database the
// migrations have just made, opened with the passphrase given with it and read
// as it decompresses, so no copy of the whole installation is held in memory.
// secrets is this installation's sealer, which a backup's secrets are sealed
// again under where the key they came under was given.
func restoreBackup(
	ctx *gofr.Context,
	db sqldb.DB,
	dialect string,
	request restoreRequest,
	secrets *security.Sealer,
) error {
	schema, err := sqldb.SchemaVersion(ctx, db)
	if err != nil {
		return err
	}

	finish, err := restoredSecrets(ctx, dialect, request, secrets)
	if err != nil {
		return err
	}

	return backup.Open(request.file, request.passphrase, func(content io.Reader) error {
		summary, err := sqldb.Restore(ctx, db, dialect, schema, content, finish)
		if err != nil {
			return err
		}

		ctx.Logger.Infof("restored the backup %s: %d rows in %d tables",
			summary.Header.Describe(), summary.Rows, summary.Tables)

		return nil
	})
}

// restoredSecrets is the restore's last step for secrets this installation's key
// cannot open: sealed again under it with the key they came under, or left
// behind. Nil where the installer was given neither, because they open as they
// are - or there are none.
func restoredSecrets(
	ctx *gofr.Context,
	dialect string,
	request restoreRequest,
	secrets *security.Sealer,
) (func(context.Context, sqldb.DB) error, error) {
	switch {
	case request.secretKey != "":
		from, err := security.NewSealer(request.secretKey)
		if err != nil {
			return nil, fmt.Errorf("the SECRET_KEY given for the backup cannot be used: %w", err)
		}

		return func(c context.Context, db sqldb.DB) error {
			sealed, err := sqldb.RekeyStoredSecrets(c, db, dialect, from, secrets)
			if err == nil {
				ctx.Logger.Infof("sealed %d of the backup's secrets again under this installation's key", sealed)
			}

			return err
		}, nil
	case request.withoutSecrets:
		return func(c context.Context, db sqldb.DB) error {
			left, err := sqldb.LeaveStoredSecretsBehind(c, db, dialect)
			if err == nil {
				ctx.Logger.Warnf("left %d of the backup's secrets behind: second factors are set up again, "+
					"and the directory password and a sign-in provider's client secret entered again", left)
			}

			return err
		}, nil
	}

	return nil, nil
}
