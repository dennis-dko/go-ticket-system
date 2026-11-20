package sqldb

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dennis-dko/timepulse/internal/domain/model"
)

// skippedOnRestore are settings a restore leaves as the new installation has
// them, because they describe the process that wrote them rather than the
// installation: every moment a restore writes is written by the restoring
// process, in its zone, so the zone the backup's server recorded would be wrong
// about all of them.
var skippedOnRestore = []string{model.SettingStorageZone}

var (
	// ErrNotEmpty says the database already holds an installation, which a
	// restore never writes over.
	ErrNotEmpty = errors.New("the database already holds accounts; a backup is only restored into an empty one")

	// ErrNewerSchema says the backup comes from a schema this release does not
	// have yet.
	ErrNewerSchema = errors.New("the backup comes from a newer version of TimePulse; restore it with that version or a later one")

	// ErrCutShort says the backup ends before its end.
	ErrCutShort = errors.New("the backup ends before its last line; it was cut short")
)

// ErrNotABackupHeader says the first line of what was opened is not a backup's.
var ErrNotABackupHeader = errors.New("the file opened, and what it holds is not a TimePulse backup")

// ReadBackupHeader reads a backup's first line and nothing more, for saying what
// a backup is before anything is restored from it.
func ReadBackupHeader(r io.Reader) (BackupHeader, error) {
	var header BackupHeader
	if err := json.NewDecoder(r).Decode(&header); err != nil {
		return header, ErrNotABackupHeader
	}

	if header.Format != backupFormat || header.Version != backupVersion {
		return header, ErrNotABackupHeader
	}

	return header, nil
}

// HoldsAccounts reports whether the database has an account in it - the
// question a restore asks first. A database whose tables do not exist yet holds
// none.
func HoldsAccounts(ctx context.Context, db DB) bool {
	var count int64

	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false
	}

	return count > 0
}

// RestoreSummary is what a restore put back.
type RestoreSummary struct {
	Header BackupHeader
	Tables int
	Rows   int64
}

// Restore puts a backup into a database the migrations have just made, in one
// transaction: everything lands, or nothing does and the database is as the
// migrations left it.
//
// It refuses a database that already holds accounts, and a backup from a schema
// newer than schema, the newest migration the database has. A backup from an
// older one restores: the columns both know are written, a column only the
// database has takes its default - the chain adds no column without one - and a
// column only the backup has is dropped. Every column is written as the
// database's own column expects it, whatever dialect the backup came from.
//
// The rows the migrations seeded - the built-in roles and their permissions -
// are removed first, because the backup carries them as they were.
func Restore(ctx context.Context, db DB, dialect string, schema int64, r io.Reader) (RestoreSummary, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()

	var summary RestoreSummary

	if err := decoder.Decode(&summary.Header); err != nil {
		return summary, fmt.Errorf("read the backup's header: %w", err)
	}

	if summary.Header.Format != backupFormat || summary.Header.Version != backupVersion {
		return summary, fmt.Errorf("the backup is of format %q version %d, not %s version %d",
			summary.Header.Format, summary.Header.Version, backupFormat, backupVersion)
	}

	if summary.Header.Schema > schema {
		return summary, ErrNewerSchema
	}

	if HoldsAccounts(ctx, db) {
		return summary, ErrNotEmpty
	}

	err := base{db: db, dialect: dialect}.withTx(ctx, func(tx base) error {
		for _, table := range slices.Backward(BackupTables) {
			if _, err := tx.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
				return fmt.Errorf("empty %s: %w", table, err)
			}
		}

		// The line after a table's rows is read to learn that they have ended, so
		// it comes back from restoreTable and is the next one looked at here.
		var line json.RawMessage

		// Every row read, the ones left out included, is what the last line counts.
		var read int64

		for {
			if line == nil {
				var err error
				if line, err = nextLine(decoder); err != nil {
					return err
				}
			}

			var end backupEnd
			if json.Unmarshal(line, &end) == nil && end.End {
				if end.Rows != read {
					return fmt.Errorf("the backup says it holds %d rows and %d were read", end.Rows, read)
				}

				return resetSequences(ctx, tx)
			}

			var section backupSection
			if err := json.Unmarshal(line, &section); err != nil || section.Table == "" {
				return fmt.Errorf("the backup has a line where a table should begin: %.80s", line)
			}

			rows, next, err := restoreTable(ctx, tx, section, decoder)
			if err != nil {
				return fmt.Errorf("restore %s: %w", section.Table, err)
			}

			summary.Tables++
			summary.Rows += rows.restored
			read += rows.read
			line = next
		}
	})

	return summary, err
}

// nextLine reads the backup's next line as it stands. The end of the input is
// never where a backup ends - its last line says so - so reaching it is a file
// cut short.
func nextLine(decoder *json.Decoder) (json.RawMessage, error) {
	var line json.RawMessage
	if err := decoder.Decode(&line); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrCutShort
		}

		return nil, fmt.Errorf("read the backup: %w", err)
	}

	return line, nil
}

// tableRows is what a table's rows came to: every row read, and the ones written.
// They differ by the settings a restore leaves out, which still count towards
// the number the backup's last line names.
type tableRows struct {
	read, restored int64
}

// restoreTable inserts the rows that follow a section line, up to the first line
// that is not a row, which it hands back unread.
func restoreTable(ctx context.Context, tx base, section backupSection, decoder *json.Decoder) (tableRows, json.RawMessage, error) {
	var rows tableRows

	if !slices.Contains(BackupTables, section.Table) {
		return rows, nil, fmt.Errorf("%q is not a table a backup carries", section.Table)
	}

	target, err := columnKinds(ctx, tx, section.Table)
	if err != nil {
		return rows, nil, err
	}

	// The columns both the backup and the database know, by their position in
	// the backup's rows. Only names the database itself answered with reach the
	// statement below; nothing the file says is written into SQL.
	var (
		names     []string
		positions []int
		kinds     []string
	)

	for i, column := range section.Columns {
		kind, known := target[column]
		if !known {
			continue
		}

		if kind == kindOther && i < len(section.Kinds) {
			kind = section.Kinds[i]
		}

		names = append(names, column)
		positions = append(positions, i)
		kinds = append(kinds, kind)
	}

	insert := tx.rebind(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", section.Table,
		strings.Join(names, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ")))

	keyColumn := slices.Index(section.Columns, "key_name")

	for {
		line, err := nextLine(decoder)
		if err != nil {
			return rows, nil, err
		}

		// A row is an array; the line that is not one opens what follows.
		if line[0] != '[' {
			return rows, line, nil
		}

		row, err := rowOf(line)
		if err != nil {
			return rows, nil, err
		}

		rows.read++

		if section.Table == "settings" && keyColumn >= 0 && keyColumn < len(row) {
			if key, _ := row[keyColumn].(string); slices.Contains(skippedOnRestore, key) {
				continue
			}
		}

		args := make([]any, len(names))
		for j, position := range positions {
			if position >= len(row) {
				return rows, nil, fmt.Errorf("a row of %d values where %d columns were named", len(row), len(section.Columns))
			}

			if args[j], err = decodeValue(kinds[j], row[position]); err != nil {
				return rows, nil, fmt.Errorf("column %s: %w", names[j], err)
			}
		}

		if _, err := tx.db.ExecContext(ctx, insert, args...); err != nil {
			return rows, nil, err
		}

		rows.restored++
	}
}

// rowOf reads a row's values with its numbers as written: an id or a count
// read as a float would come back rounded past 2^53.
func rowOf(line json.RawMessage) ([]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()

	var row []any
	if err := decoder.Decode(&row); err != nil {
		return nil, fmt.Errorf("read a row: %w", err)
	}

	return row, nil
}

// columnKinds asks the database which columns a table has, and the kind each
// one's values are written as.
func columnKinds(ctx context.Context, tx base, table string) (map[string]string, error) {
	rows, err := tx.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s WHERE 1 = 0", table))
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	kinds := make(map[string]string, len(types))
	for _, column := range types {
		kinds[strings.ToLower(column.Name())] = kindOf(column.DatabaseTypeName())
	}

	return kinds, rows.Err()
}

// resetSequences moves PostgreSQL's counters past the ids a restore wrote, which
// it inserts as they were. MySQL and SQLite move theirs on an insert that names
// an id; PostgreSQL's would hand out 1 next and collide with the first account.
func resetSequences(ctx context.Context, tx base) error {
	if tx.dialect != DialectPostgres {
		return nil
	}

	for _, table := range BackupTables {
		kinds, err := columnKinds(ctx, tx, table)
		if err != nil {
			return err
		}

		if _, hasID := kinds["id"]; !hasID {
			continue
		}

		if _, err := tx.db.ExecContext(ctx, fmt.Sprintf(
			"SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false)",
			table, table)); err != nil {
			return fmt.Errorf("move the counter of %s past what was restored: %w", table, err)
		}
	}

	return nil
}

// decodeValue turns what a backup line carries into what the database's column
// expects.
func decodeValue(kind string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	switch kind {
	case kindBytes:
		if s, isString := value.(string); isString {
			return base64.StdEncoding.DecodeString(s)
		}
	case kindTime:
		if s, isString := value.(string); isString {
			return time.Parse(time.RFC3339Nano, s)
		}
	case kindBool:
		switch v := value.(type) {
		case bool:
			return v, nil
		case json.Number:
			n, err := v.Int64()

			return n != 0, err
		}
	case kindInt:
		switch v := value.(type) {
		case json.Number:
			return v.Int64()
		case bool:
			if v {
				return int64(1), nil
			}

			return int64(0), nil
		}
	case kindFloat:
		if v, isNumber := value.(json.Number); isNumber {
			return v.Float64()
		}
	case kindText:
		switch v := value.(type) {
		case string:
			return v, nil
		case json.Number:
			return v.String(), nil
		case bool:
			return strconv.FormatBool(v), nil
		}
	}

	if v, isNumber := value.(json.Number); isNumber {
		if n, err := v.Int64(); err == nil {
			return n, nil
		}

		return v.Float64()
	}

	return value, nil
}
