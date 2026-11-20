package sqldb

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// BackupTables are the tables a backup carries, each after every table it points
// at, which is the order a restore inserts them in and the reverse of the order
// it empties them.
//
// test/backup_test.go holds this list against the migration chain in both
// directions: a table the chain creates is either here or in LeftOutOfBackups
// with its reason, and no table here points at one that comes after it.
var BackupTables = []string{
	"settings",
	"roles",
	"role_permissions",
	"users",
	"projects",
	"timesheets",
	"timesheet_revisions",
	"running_timers",
	"work_schedules",
	"entry_templates",
	"absences",
	"vacation_entitlements",
	"calendar_feeds",
	"openid_identities",
	"work_days",
	"api_tokens",
	"passkeys",
	"push_subscriptions",
	"managed_accounts",
	"admin_events",
	"directory_runs",
}

// LeftOutOfBackups are the tables the chain creates that a backup does not
// carry, each with the reason.
var LeftOutOfBackups = map[string]string{
	"sessions": "a session is a sign-in, not a record: in a file meant to leave the server it would be " +
		"a credential, and after a restore everybody signs in again anyway",
}

// backupFormat names the content of a backup, so a file of other JSON lines is
// refused rather than half read.
const (
	backupFormat  = "timepulse-backup"
	backupVersion = 1
)

// BackupHeader opens a backup: what made it, and from which schema.
type BackupHeader struct {
	Format      string    `json:"format"`
	Version     int       `json:"version"`
	Application string    `json:"application"`
	Dialect     string    `json:"dialect"`
	Schema      int64     `json:"schema"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Describe says what a backup is in one line, for a log: the release it was
// made by, the moment and the dialect. The release and the dialect are read from
// the file, so a line break in either is taken out rather than left to start a
// log line of its own, written by whoever made the file.
func (h BackupHeader) Describe() string {
	flat := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " ")
	}

	return fmt.Sprintf("made by %s on %s from %s",
		flat(h.Application), h.CreatedAt.UTC().Format(time.RFC3339), flat(h.Dialect))
}

// backupSection opens a table's rows: its columns, and what kind of value each
// holds where it was read.
type backupSection struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
	Kinds   []string `json:"kinds"`
}

// backupEnd closes a backup. A file that stops before it was cut short, and a
// restore refuses it rather than keep what it read so far.
type backupEnd struct {
	End  bool  `json:"end"`
	Rows int64 `json:"rows"`
}

// The kinds a column's values are written as: the one thing the three dialects
// agree on, since SQLite keeps a boolean as an integer, MySQL hands text back as
// bytes, and only PostgreSQL has a type for either.
const (
	kindInt   = "int"
	kindFloat = "float"
	kindBool  = "bool"
	kindText  = "text"
	kindBytes = "bytes"
	kindTime  = "time"
	kindOther = "other"
)

// SchemaVersion is the newest migration applied to the database.
func SchemaVersion(ctx context.Context, db DB) (int64, error) {
	var version sql64

	if err := db.QueryRowContext(ctx, "SELECT MAX(version) FROM gofr_migrations").Scan(&version); err != nil {
		return 0, fmt.Errorf("read the schema version: %w", err)
	}

	return int64(version), nil
}

// sql64 scans a number that may be NULL as zero.
type sql64 int64

func (s *sql64) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*s = 0
	case int64:
		*s = sql64(v)
	case []byte:
		n, err := strconv.ParseInt(string(v), 10, 64)
		*s = sql64(n)

		return err
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		*s = sql64(n)

		return err
	default:
		return fmt.Errorf("a schema version of type %T", src)
	}

	return nil
}

// Dump writes every table of BackupTables to w as lines of JSON: the header, and
// then for each table a line naming its columns followed by a line per row. It
// answers with the number of rows written.
//
// Every table is read in one transaction, so the backup is one moment of the
// database: read table by table while people book, it could hold an entry whose
// project was made after the projects were read - which PostgreSQL would refuse
// at restore time. Repeatable read where the dialect offers it; a SQLite
// transaction reads one moment of its own.
func Dump(ctx context.Context, db DB, header BackupHeader, w io.Writer) (int64, error) {
	if beginner, ok := db.(txBeginner); ok {
		options := &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
		if header.Dialect == DialectSQLite {
			options = nil
		}

		tx, err := beginner.BeginTx(ctx, options)
		if err != nil {
			return 0, fmt.Errorf("begin reading the backup: %w", err)
		}

		// Read only: nothing to keep, and a rollback is how it ends.
		defer func() { _ = tx.Rollback() }()

		db = tx
	}

	encoder := json.NewEncoder(w)

	header.Format, header.Version = backupFormat, backupVersion
	if err := encoder.Encode(header); err != nil {
		return 0, err
	}

	var total int64

	for _, table := range BackupTables {
		written, err := dumpTable(ctx, db, table, encoder)
		if err != nil {
			return total, fmt.Errorf("back up %s: %w", table, err)
		}

		total += written
	}

	return total, encoder.Encode(backupEnd{End: true, Rows: total})
}

// dumpTable writes one table: its section line and its rows.
func dumpTable(ctx context.Context, db DB, table string, encoder *json.Encoder) (int64, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return 0, err
	}

	defer func() { _ = rows.Close() }()

	types, err := rows.ColumnTypes()
	if err != nil {
		return 0, err
	}

	section := backupSection{Table: table}
	for _, column := range types {
		section.Columns = append(section.Columns, strings.ToLower(column.Name()))
		section.Kinds = append(section.Kinds, kindOf(column.DatabaseTypeName()))
	}

	if err := encoder.Encode(section); err != nil {
		return 0, err
	}

	values := make([]any, len(types))
	pointers := make([]any, len(types))

	for i := range values {
		pointers[i] = &values[i]
	}

	var written int64

	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return written, err
		}

		line := make([]any, len(values))
		for i, value := range values {
			if line[i], err = encodeValue(section.Kinds[i], value); err != nil {
				return written, fmt.Errorf("column %s: %w", section.Columns[i], err)
			}
		}

		if err := encoder.Encode(line); err != nil {
			return written, err
		}

		written++
	}

	return written, rows.Err()
}

// kindOf is the kind a column's values are written as, from the type its
// database names it with.
func kindOf(databaseType string) string {
	t := strings.ToUpper(databaseType)

	switch {
	case strings.Contains(t, "BOOL"):
		return kindBool
	case strings.Contains(t, "BLOB"), strings.Contains(t, "BYTEA"), strings.Contains(t, "BINARY"):
		return kindBytes
	case strings.Contains(t, "TIME"), strings.Contains(t, "DATE"):
		return kindTime
	case strings.Contains(t, "CHAR"), strings.Contains(t, "TEXT"), strings.Contains(t, "CLOB"):
		return kindText
	case strings.Contains(t, "INT"):
		return kindInt
	case strings.Contains(t, "REAL"), strings.Contains(t, "FLOA"), strings.Contains(t, "DOUB"),
		strings.Contains(t, "NUMERIC"), strings.Contains(t, "DECIMAL"):
		return kindFloat
	default:
		return kindOther
	}
}

// storedTimeLayouts are the ways a moment comes back from SQLite as text.
var storedTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02",
}

// encodeValue turns what a driver scanned into what a backup line carries.
func encodeValue(kind string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	if raw, isBytes := value.([]byte); isBytes && kind != kindBytes {
		value = string(raw)
	}

	switch kind {
	case kindBytes:
		switch v := value.(type) {
		case []byte:
			return base64.StdEncoding.EncodeToString(v), nil
		case string:
			return base64.StdEncoding.EncodeToString([]byte(v)), nil
		}
	case kindTime:
		switch v := value.(type) {
		case time.Time:
			return v.UTC().Format(time.RFC3339Nano), nil
		case string:
			for _, layout := range storedTimeLayouts {
				if parsed, err := time.Parse(layout, v); err == nil {
					return parsed.UTC().Format(time.RFC3339Nano), nil
				}
			}

			return nil, fmt.Errorf("%q is not a moment", v)
		}
	case kindBool:
		switch v := value.(type) {
		case bool:
			return v, nil
		case int64:
			return v != 0, nil
		case string:
			return strconv.ParseBool(v)
		}
	case kindInt:
		switch v := value.(type) {
		case int64:
			return v, nil
		case bool:
			if v {
				return int64(1), nil
			}

			return int64(0), nil
		case string:
			return strconv.ParseInt(v, 10, 64)
		}
	case kindFloat:
		switch v := value.(type) {
		case float64:
			return v, nil
		case int64:
			return float64(v), nil
		case string:
			return strconv.ParseFloat(v, 64)
		}
	}

	if t, isTime := value.(time.Time); isTime {
		return t.UTC().Format(time.RFC3339Nano), nil
	}

	return value, nil
}
