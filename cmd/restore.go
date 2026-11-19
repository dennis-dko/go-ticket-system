package main

import (
	"io"
	"time"

	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/backup"
)

// restoreBackup puts the backup the installer was given into the database the
// migrations have just made, opened with the passphrase given with it and read
// as it decompresses, so no copy of the whole installation is held in memory.
func restoreBackup(ctx *gofr.Context, db sqldb.DB, dialect string, file []byte, passphrase string) error {
	schema, err := sqldb.SchemaVersion(ctx, db)
	if err != nil {
		return err
	}

	return backup.Open(file, passphrase, func(content io.Reader) error {
		summary, err := sqldb.Restore(ctx, db, dialect, schema, content)
		if err != nil {
			return err
		}

		ctx.Logger.Infof("restored the backup made by %s on %s from %s: %d rows in %d tables",
			summary.Header.Application, summary.Header.CreatedAt.Format(time.RFC3339), summary.Header.Dialect,
			summary.Rows, summary.Tables)

		return nil
	})
}
