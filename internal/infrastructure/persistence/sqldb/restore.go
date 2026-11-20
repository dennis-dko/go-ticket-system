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

// readHeader reads a backup's first line, and refuses one that is not a backup
// this release reads or one from a schema newer than schema, the newest
// migration there is to restore it into.
func readHeader(decoder *json.Decoder, schema int64) (BackupHeader, error) {
	var header BackupHeader
	if err := decoder.Decode(&header); err != nil {
		return header, ErrNotABackupHeader
	}

	if header.Format != backupFormat || header.Version != backupVersion {
		return header, ErrNotABackupHeader
	}

	if header.Schema > schema {
		return header, ErrNewerSchema
	}

	return header, nil
}

// BackupContents is what a whole read of a backup found: its header, how much it
// holds, and the key check of the installation it was made on - empty where that
// installation configured no SECRET_KEY, so nothing in it is sealed.
type BackupContents struct {
	Header   BackupHeader
	Tables   int
	Rows     int64
	KeyCheck string
}

// InspectBackup reads a whole backup and writes nothing, so that what a restore
// would refuse - a file cut short, a table no backup carries, a last line that
// does not count the rows before it, a schema newer than schema - is refused
// before a database is chosen for it. The installer asks it of an upload,
// because the restore itself runs after the installer has handed over, where a
// refusal can only end the process.
func InspectBackup(r io.Reader, schema int64) (BackupContents, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()

	var contents BackupContents

	header, err := readHeader(decoder, schema)
	contents.Header = header

	if err != nil {
		return contents, err
	}

	err = walkBackup(decoder, func(section backupSection) (func(row []any) error, error) {
		contents.Tables++

		key, value := slices.Index(section.Columns, "key_name"), slices.Index(section.Columns, "value")

		return func(row []any) error {
			contents.Rows++

			if section.Table == "settings" && key >= 0 && value >= 0 && max(key, value) < len(row) {
				if name, _ := row[key].(string); name == model.SettingSecretKeyCheck {
					contents.KeyCheck, _ = row[value].(string)
				}
			}

			return nil
		}, nil
	})

	return contents, err
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
//
// finish, where it is given, runs in the same transaction once every row is
// written, so what it changes lands with the restore or not at all: a backup made
// under another SECRET_KEY has its secrets sealed again or left behind there.
func Restore(
	ctx context.Context,
	db DB,
	dialect string,
	schema int64,
	r io.Reader,
	finish func(context.Context, DB) error,
) (RestoreSummary, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()

	var summary RestoreSummary

	header, err := readHeader(decoder, schema)
	summary.Header = header

	if err != nil {
		return summary, err
	}

	if HoldsAccounts(ctx, db) {
		return summary, ErrNotEmpty
	}

	err = base{db: db, dialect: dialect}.withTx(ctx, func(tx base) error {
		for _, table := range slices.Backward(BackupTables) {
			if _, err := tx.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
				return fmt.Errorf("empty %s: %w", table, err)
			}
		}

		err := walkBackup(decoder, func(section backupSection) (func(row []any) error, error) {
			summary.Tables++

			write, err := tableWriter(ctx, tx, section)
			if err != nil {
				return nil, err
			}

			return func(row []any) error {
				written, err := write(row)
				if written {
					summary.Rows++
				}

				return err
			}, nil
		})
		if err != nil {
			return err
		}

		if finish != nil {
			if err := finish(ctx, tx.db); err != nil {
				return err
			}
		}

		return resetSequences(ctx, tx)
	})

	return summary, err
}

// walkBackup reads a backup after its header, table by table, to its last line:
// open is told each table as it begins and answers with what to do with each of
// its rows. What a restore and an inspection both stand on is checked here, once
// for both - that every table is one a backup carries, that the file does not
// stop before its last line, and that the last line counts every row read, the
// ones a restore leaves out included.
func walkBackup(decoder *json.Decoder, open func(backupSection) (func(row []any) error, error)) error {
	var read int64

	line, err := nextLine(decoder)

	for err == nil {
		var end backupEnd
		if json.Unmarshal(line, &end) == nil && end.End {
			if end.Rows != read {
				return fmt.Errorf("the backup says it holds %d rows and %d were read", end.Rows, read)
			}

			return nil
		}

		var section backupSection
		if json.Unmarshal(line, &section) != nil || section.Table == "" {
			return fmt.Errorf("the backup has a line where a table should begin: %.80s", line)
		}

		if !slices.Contains(BackupTables, section.Table) {
			return fmt.Errorf("%q is not a table a backup carries", section.Table)
		}

		each, opened := open(section)
		if opened != nil {
			return fmt.Errorf("%s: %w", section.Table, opened)
		}

		// The line after a table's rows is read to learn that they have ended, so
		// it is the next one this loop looks at - and the end of the input, which
		// is never where a backup ends, leaves err set and the loop with it.
		for line, err = nextLine(decoder); err == nil && line[0] == '['; line, err = nextLine(decoder) {
			row, bad := rowOf(line)
			if bad == nil {
				read++
				bad = each(row)
			}

			if bad != nil {
				return fmt.Errorf("%s: %w", section.Table, bad)
			}
		}
	}

	return err
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

// tableWriter prepares the rows of one table of a backup to be written into the
// same table here, and answers for each row whether it was written: a setting a
// restore leaves as the new installation has it is not, and still counts towards
// the number the backup's last line names.
func tableWriter(ctx context.Context, tx base, section backupSection) (func(row []any) (bool, error), error) {
	target, err := columnKinds(ctx, tx, section.Table)
	if err != nil {
		return nil, err
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

	return func(row []any) (bool, error) {
		if section.Table == "settings" && keyColumn >= 0 && keyColumn < len(row) {
			if key, _ := row[keyColumn].(string); slices.Contains(skippedOnRestore, key) {
				return false, nil
			}
		}

		args := make([]any, len(names))

		for j, position := range positions {
			if position >= len(row) {
				return false, fmt.Errorf("a row of %d values where %d columns were named", len(row), len(section.Columns))
			}

			value, err := decodeValue(kinds[j], row[position])
			if err != nil {
				return false, fmt.Errorf("column %s: %w", names[j], err)
			}

			args[j] = value
		}

		if _, err := tx.db.ExecContext(ctx, insert, args...); err != nil {
			return false, err
		}

		return true, nil
	}, nil
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
