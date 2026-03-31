package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB with migration support.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at path and configures pragmas.
func Open(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Single writer; WAL keeps readers non-blocking.
	sqldb.SetMaxOpenConns(1)

	if _, err := sqldb.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("setting journal_mode: %w", err)
	}
	if _, err := sqldb.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("setting foreign_keys: %w", err)
	}

	return &DB{sqldb}, nil
}

// Migrate runs any unapplied *.up.sql files from the provided FS in order.
func (db *DB) Migrate(migrations fs.FS) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT     PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}

	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("querying applied migrations: %w", err)
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		version := strings.TrimSuffix(name, ".up.sql")
		if applied[version] {
			continue
		}

		content, err := fs.ReadFile(migrations, name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", name, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("applying migration %s: %w", name, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("recording migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", name, err)
		}
	}

	return nil
}
