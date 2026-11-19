package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	appservice "github.com/dennis-dko/timepulse/internal/application/v1/service"
	appconfig "github.com/dennis-dko/timepulse/internal/infrastructure/config"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// recoverTimeout bounds the whole command, so a database that answers the ping
// and then nothing else does not hold a terminal for ever.
const recoverTimeout = time.Minute

// recoverSystemUser is the recover command: it gives the built-in administrator
// a new password and prints it, for an installation nobody can sign into any
// more. It answers with the exit code.
//
// It works on the database beside a running server rather than as one, with
// the connection and the key the server reads - configs/datasource.json, then
// the environment and configs/.env - so it has to run where the server runs,
// from the directory it starts in. In the image that is what docker compose exec
// does by itself.
func recoverSystemUser() int {
	ds, configured := appconfig.LoadDatasource(appconfig.DatasourceFile)
	if !configured {
		ds, configured = appconfig.DatasourceFromEnvironment()
	}

	if !configured {
		_, _ = fmt.Fprintln(os.Stderr, "no database is configured here, so there is no account to recover.\n"+
			"  Run this where the application runs, from the directory it starts in, so that\n"+
			"  "+appconfig.DatasourceFile+" or DB_DIALECT is found.")

		return 1
	}

	secrets, err := security.NewSealer(appconfig.SecretKeyFromEnvironment())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "SECRET_KEY cannot be used: %v\n", err)

		return 1
	}

	// The wait the server gives SQLite, so a write the server is in the middle of
	// is waited out rather than refused.
	makeSQLiteWait()

	ctx, cancel := context.WithTimeout(context.Background(), recoverTimeout)
	defer cancel()

	db, err := appconfig.OpenDatasource(ctx, ds)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n  dialect %q, name %q%s\n", err, ds.Dialect, ds.Name, hostSuffix(ds))

		return 1
	}

	defer func() { _ = db.Close() }()

	recovery := appservice.NewRecoveryService(
		sqldb.NewUserRepository(db, ds.Dialect).WithSecrets(secrets),
		sqldb.NewPasskeyRepository(db, ds.Dialect),
		sqldb.NewSessionRepository(db, ds.Dialect),
		sqldb.NewAdminEventRepository(db, ds.Dialect),
	)

	password, err := recovery.RecoverSystemUser(ctx)
	if err != nil && !errors.Is(err, appservice.ErrRecoveryNotLogged) {
		_, _ = fmt.Fprintf(os.Stderr, "the built-in administrator was not recovered: %v\n", err)

		return 1
	}

	fmt.Printf("The built-in administrator %s has a new password:\n\n    %s\n\n"+
		"It is shown this once. Sign in with it, and you will be asked to choose your own.\n"+
		"Its second factor is removed and every session it had is ended.\n",
		appservice.SystemUserEmail, password)

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "\nwarning: %v\n", err)
	}

	return 0
}
