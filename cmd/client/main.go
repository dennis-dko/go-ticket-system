package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	// The time zone database, compiled in as the web binary's is. Windows has
	// none for Go to find, so without it every zone but UTC is unknown there, and
	// the stopwatch and the day were told in UTC instead of in the owner's zone.
	_ "time/tzdata"

	"github.com/dennis-dko/go-time-recording/internal/interface/cli"
)

// version is the release this client was built for, set the way the web
// binary's is - -ldflags "-X main.version=v1.2.3" - and from the same tag, so
// the two answer --version alike when they belong together.
var version = "dev"

// commit and day are the commit the client was built from and the day of it,
// stamped in beside the version: -X main.commit=5e201fc9 -X main.day=2026-10-29.
var commit, day string

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println(version)
		os.Exit(0)
	}

	// Ctrl-C cancels the request it is waiting on.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	status := cli.Run(ctx, cli.Build{Version: version, Commit: commit, Day: day}, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)

	stop()
	os.Exit(status)
}
