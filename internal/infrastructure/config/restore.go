package config

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
)

// HoldsAccounts reports whether the database a connection reaches already
// holds an account - the question asked before a backup is restored into it,
// while the installer is still on screen to say so, rather than after the
// application has started on it.
//
// A database with no tables yet holds none. Opened and closed here, for the
// reason ProbeDatasource gives: this is a question, not a switch.
func HoldsAccounts(ctx context.Context, ds Datasource) (bool, error) {
	driver, dsn, err := driverDSN(ds)
	if err != nil {
		return false, err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return false, fmt.Errorf("cannot open the connection: %w", err)
	}

	defer func() { _ = db.Close() }()

	askCtx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	return sqldb.HoldsAccounts(askCtx, db), nil
}
