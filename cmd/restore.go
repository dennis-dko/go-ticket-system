package main

import (
	"io"
	"maps"
	"slices"

	"gofr.dev/pkg/gofr"

	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/migration"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/backup"
)

// newestSchema is the newest migration this binary carries, which the installer
// holds an uploaded backup's schema to. The chain is the same for every dialect;
// only what each step runs differs.
func newestSchema() int64 {
	return slices.Max(slices.Collect(maps.Keys(migration.All(sqldb.DialectSQLite))))
}

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

		ctx.Logger.Infof("restored the backup %s: %d rows in %d tables",
			summary.Header.Describe(), summary.Rows, summary.Tables)

		return nil
	})
}
