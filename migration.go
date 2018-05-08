package migration

import "database/sql"

// Migration is a migration interface. A migration can apply itself, rollback
// itself and has a unique name.
// isDry flag identify to not migrate just show
type Migration interface {
	Apply(tx *sql.Tx, isDry bool) error
	Rollback(tx *sql.Tx, isDry bool) error

	Name() string
}

// Struct is a simple implementation of the Migration interface.
type Struct struct {
	NameString   string
	ApplyFunc    func(tx *sql.Tx, isDry bool) error
	RollbackFunc func(tx *sql.Tx, isDry bool) error
}

// Apply implements Migration for Struct.
func (s Struct) Apply(tx *sql.Tx, isDry bool) error {
	return s.ApplyFunc(tx, isDry)
}

// Rollback implements Migration for Struct.
func (s Struct) Rollback(tx *sql.Tx, isDry bool) error {
	return s.RollbackFunc(tx, isDry)
}

// Name implements Migration for Struct.
func (s Struct) Name() string {
	return s.NameString
}

var _ Migration = Struct{}

// FindByName finds a migration by name.
func FindByName(migrations []Migration, name string) Migration {
	for _, m := range migrations {
		if m.Name() == name {
			return m
		}
	}
	return nil
}
