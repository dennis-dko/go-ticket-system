package main

import (
	"path/filepath"
	"testing"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
)

// A start with no database is refused because unavailable says so, and it has
// to say so of the container exactly as GoFr leaves it: one built with no
// database settings holds none, and one built with them holds what it opened.
// Asked of GoFr itself rather than of a value made here, because the shape of
// "none" is GoFr's to choose - a typed nil pointer up to v1.62, which a plain
// comparison with nil does not see.
func TestAStartWithoutADatabaseIsToldApartFromOneWithIt(t *testing.T) {
	bare := container.NewContainer(config.NewMockConfig(map[string]string{}))
	if !unavailable(bare.SQL) {
		t.Errorf("a container built with no database settings is taken to hold one: %T", bare.SQL)
	}

	opened := container.NewContainer(config.NewMockConfig(map[string]string{
		"DB_DIALECT": "sqlite",
		"DB_NAME":    filepath.Join(t.TempDir(), "probe"),
	}))
	t.Cleanup(func() { _ = opened.Close() })

	if unavailable(opened.SQL) {
		t.Error("a container that opened its SQLite database is taken to hold none")
	}
}
