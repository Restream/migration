# migration

A Go migration library.

# Warning

This is a work in progress. As of now this library lacks:

* Proper documentation
* Tests
* Support for anything other than PostgreSQL

# Example:

```go
var migrations = []migration.Migration{
	migration.Struct{
		NameString: "20161114105737_init",
		ApplyFunc: func(tx *sql.Tx, isDry bool) error {
			var err error
			var q string
			q = `CREATE TABLE IF NOT EXISTS users (` +
				`id BIGSERIAL PRIMARY KEY` +
				`, login TEXT NOT NULL UNIQUE` +
				`, properties JSONB NOT NULL` +
				`, created_at TIMESTAMP NOT NULL` +
				`, updated_at TIMESTAMP NOT NULL` +
				`)`
			fmt.Printf("-- Apply %s --\n", name)
			if !isDry {
				_, err = tx.Exec(q)
			} else {
				fmt.Printf("%s\n\n",q)
			}
			if err != nil {
				return err
			}

			return nil
		},
		RollbackFunc: func(tx *sql.Tx, isDry bool) error {
			var err error
			var q string
			q = `DROP TABLE rose.channels`
			fmt.Printf("-- Rollback %s --\n", name)
			if !isDry {
				_, err = tx.Exec(q)
			} else {
				fmt.Printf("%s\n\n",q)
			}
			if err != nil {
				return err
			}

			return nil
		},
	},
}

func main() {
	db, err := sql.Open("postgres", "...")
	// ...
	sch := migration.NewSchema(db, migration.DefaultSchemaName, migration.DefaultMigrationTableName)
	err = sch.Init()
	if err != nil {
		log.Fatalf("can't init the db: %v", err)
	}

	migs, err := sch.FindUnapplied(migrations)
	if err != nil {
		log.Fatalf("can't find migrations: %v", err)
	}

	n, err := sch.Apply(migs)
	if err != nil {
		log.Fatalf("can't migrate the db: %v", err)
	}

	log.Printf("applied %d migrations", n)
}
```
