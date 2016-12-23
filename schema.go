package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

// DefaultMigrationTableName is the default migrations table name.
const DefaultMigrationTableName = "schema_migrations"

// DefaultSchemaName is the default schema name.
const DefaultSchemaName = "public"

// Schema is the single database's schema representation.
type Schema struct {
	db           *sql.DB
	schemaName   string
	migTableName string
}

// NewSchema returns a new Schema.
func NewSchema(db *sql.DB, schemaName, migTableName string) *Schema {
	return &Schema{
		db:           db,
		schemaName:   schemaName,
		migTableName: migTableName,
	}
}

// ErrorPair is a pair of errors.
type ErrorPair struct {
	Err1, Err2 error
}

// Error implements the error interface for ErrorPair.
func (err ErrorPair) Error() string {
	return fmt.Sprintf("err1: %q, err2: %q", err.Err1, err.Err2)
}

// Apply applies all migrations in a single transaction. It returns the number
// of applied migrations and error if any.
func (sch *Schema) Apply(migrations []Migration) (n int, err error) {
	tx, err := sch.db.Begin()
	if err != nil {
		return 0, err
	}

	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			rbErr := tx.Rollback()
			if rbErr != nil {
				err = ErrorPair{
					Err1: err,
					Err2: rbErr,
				}
			}
		}
	}()

	now := time.Now()
	q := `INSERT INTO "` + sch.schemaName + `"` + `."` + sch.migTableName + `" (name, applied_at) ` +
		`VALUES ($1, $2)`
	for _, m := range migrations {
		err = m.Apply(tx)
		if err != nil {
			return 0, err
		}

		_, err = tx.Exec(q, m.Name(), now)
		if err != nil {
			return 0, err
		}

		n++
	}

	return n, nil
}

// Rollback rolls back all migrations in a single transaction. It returns the
// number of rolled back migrations and error if any.
func (sch *Schema) Rollback(migrations []Migration) (n int, err error) {
	tx, err := sch.db.Begin()
	if err != nil {
		return 0, err
	}

	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			rbErr := tx.Rollback()
			if rbErr != nil {
				err = ErrorPair{
					Err1: err,
					Err2: rbErr,
				}
			}
		}
	}()

	q := `DELETE FROM "` + sch.schemaName + `"` + `."` + sch.migTableName + `" ` +
		`WHERE name = $1`
	for _, m := range migrations {
		err = m.Rollback(tx)
		if err != nil {
			return 0, err
		}

		_, err = tx.Exec(q, m.Name())
		if err != nil {
			return 0, err
		}

		n++
	}

	return n, nil
}

// Init creates a migrations table in the database.
func (sch *Schema) Init() error {
	var err error
	var q string
	q = `CREATE SCHEMA IF NOT EXISTS "` + sch.schemaName + `"`
	_, err = sch.db.Exec(q)
	if err != nil {
		return err
	}

	q = `CREATE TABLE IF NOT EXISTS "` + sch.schemaName + `"` + `."` + sch.migTableName + `" ` +
		`(name TEXT UNIQUE, applied_at TIMESTAMP)`
	_, err = sch.db.Exec(q)
	if err != nil {
		return err
	}
	return nil
}

// ErrNameNotUnique is returned whenever a non-unique migration name is found.
type ErrNameNotUnique struct {
	Name string
}

// Error implements the error interface for ErrNameNotUnique.
func (err ErrNameNotUnique) Error() string {
	return fmt.Sprintf("migration name not unique: %q", err.Name)
}

var _ error = ErrNameNotUnique{}

// ErrMigrationNotFound is returned by FindOne when migration is not found by
// name.
var ErrMigrationNotFound = errors.New("migration not found")

// FindOne finds a migration by name
func (sch *Schema) FindOne(migrations []Migration, name string) (res []Migration, err error) {
	for _, m := range migrations {
		if m.Name() == name {
			return []Migration{m}, nil
		}
	}
	return nil, ErrMigrationNotFound
}

// FindUnapplied finds unapplied migrations.
func (sch *Schema) FindUnapplied(migrations []Migration) (res []Migration, err error) {
	if len(migrations) == 0 {
		return nil, nil
	}

	var names []string
	migByName := map[string]Migration{}
	for _, m := range migrations {
		names = append(names, m.Name())
		if migByName[m.Name()] != nil {
			return nil, ErrNameNotUnique{Name: m.Name()}
		}
		migByName[m.Name()] = m
	}

	q := `SELECT name FROM "` + sch.schemaName + `"` + `."` + sch.migTableName + `"` +
		`ORDER BY name`

	rows, err := sch.db.Query(q)
	if err != nil {
		return nil, err
	}

	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			if err != nil {
				err = ErrorPair{Err1: err, Err2: closeErr}
			} else {
				err = closeErr
			}
		}
	}()

	var resNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		resNames = append(resNames, name)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, name := range resNames {
		if _, ok := migByName[name]; ok {
			delete(migByName, name)
		}
	}

	for _, m := range migByName {
		res = append(res, m)
	}

	sort.Sort(migrationsByName(res))

	return res, nil
}

type migrationsByName []Migration

func (ms migrationsByName) Len() int           { return len(ms) }
func (ms migrationsByName) Less(i, j int) bool { return ms[i].Name() < ms[j].Name() }
func (ms migrationsByName) Swap(i, j int)      { ms[i], ms[j] = ms[j], ms[i] }

type migrationsByNameDesc []Migration

func (ms migrationsByNameDesc) Len() int           { return len(ms) }
func (ms migrationsByNameDesc) Less(i, j int) bool { return ms[j].Name() < ms[i].Name() }
func (ms migrationsByNameDesc) Swap(i, j int)      { ms[i], ms[j] = ms[j], ms[i] }

// FindUnrolled finds migrations that were not rolled back.
func (sch *Schema) FindUnrolled(migrations []Migration) (res []Migration, err error) {
	if len(migrations) == 0 {
		return nil, nil
	}

	var names []string
	migByName := map[string]Migration{}
	for _, m := range migrations {
		names = append(names, m.Name())
		if migByName[m.Name()] != nil {
			return nil, ErrNameNotUnique{Name: m.Name()}
		}
		migByName[m.Name()] = m
	}

	q := `SELECT name FROM "` + sch.schemaName + `"` + `."` + sch.migTableName + `"` +
		`ORDER BY name DESC`

	rows, err := sch.db.Query(q)
	if err != nil {
		return nil, err
	}

	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			if err != nil {
				err = ErrorPair{Err1: err, Err2: closeErr}
			} else {
				err = closeErr
			}
		}
	}()

	var resNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		resNames = append(resNames, name)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, name := range resNames {
		if _, ok := migByName[name]; !ok {
			delete(migByName, name)
		}
	}

	for _, m := range migByName {
		res = append(res, m)
	}

	sort.Sort(migrationsByNameDesc(res))

	return res, nil
}
