package main

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

// schemaDDL contains all CREATE TABLE and CREATE INDEX statements.
// Executed idempotently on every startup via CREATE ... IF NOT EXISTS.
// Per D-12: single Go function, run on every startup.
var schemaDDL = []string{
	`CREATE TABLE IF NOT EXISTS shelf (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL DEFAULT 'My Organizer',
		rows INTEGER NOT NULL,
		cols INTEGER NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS container (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		shelf_id INTEGER NOT NULL REFERENCES shelf(id) ON DELETE CASCADE,
		col INTEGER NOT NULL,
		row INTEGER NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
		UNIQUE(shelf_id, col, row)
	)`,
	`CREATE TABLE IF NOT EXISTS item (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		container_id INTEGER NOT NULL REFERENCES container(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS tag (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS item_tag (
		item_id INTEGER NOT NULL REFERENCES item(id) ON DELETE CASCADE,
		tag_id INTEGER NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
		PRIMARY KEY (item_id, tag_id)
	)`,
	// Foreign key indexes for CASCADE performance
	`CREATE INDEX IF NOT EXISTS idx_container_shelf_id ON container(shelf_id)`,
	`CREATE INDEX IF NOT EXISTS idx_item_container_id ON item(container_id)`,
	`CREATE INDEX IF NOT EXISTS idx_item_tag_item_id ON item_tag(item_id)`,
	`CREATE INDEX IF NOT EXISTS idx_item_tag_tag_id ON item_tag(tag_id)`,
}

// Store wraps the SQLite database connection.
type Store struct {
	db *sql.DB
}

// Open opens the SQLite database at dbPath, configures pragmas via DSN,
// creates the schema, and seeds the default shelf if needed.
// Per D-01: dbPath comes from DB_PATH env var (handled by caller).
func (s *Store) Open(dbPath string) error {
	// DSN with pragmas set per-connection (not post-open Exec).
	// This ensures every connection from the pool gets the pragmas.
	// Per STATE.md locked decision: WAL + foreign_keys + busy_timeout in Store.Open().
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	s.db = db

	if err := s.createSchema(); err != nil {
		db.Close()
		return fmt.Errorf("create schema: %w", err)
	}

	if err := s.seedDefaultShelf(); err != nil {
		db.Close()
		return fmt.Errorf("seed default shelf: %w", err)
	}

	slog.Info("database opened", "path", dbPath)
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// createSchema runs all DDL statements in a single transaction.
// Idempotent: uses CREATE ... IF NOT EXISTS.
func (s *Store) createSchema() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema tx: %w", err)
	}
	defer tx.Rollback()

	for _, ddl := range schemaDDL {
		if _, err := tx.Exec(ddl); err != nil {
			return fmt.Errorf("schema exec: %w", err)
		}
	}
	return tx.Commit()
}

// seedDefaultShelf creates the default shelf with 5 rows and 10 columns
// (50 containers) if no shelf exists yet.
// Per D-02: default shelf 5x10 on first run.
// Per D-03: auto-generate all 50 container records.
// Per D-11: shelf name defaults to "My Organizer".
func (s *Store) seedDefaultShelf() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM shelf").Scan(&count); err != nil {
		return fmt.Errorf("check shelf count: %w", err)
	}
	if count > 0 {
		return nil // already seeded
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer tx.Rollback()

	const (
		defaultName = "My Organizer"
		defaultRows = 5
		defaultCols = 10
	)

	res, err := tx.Exec(
		"INSERT INTO shelf (name, rows, cols) VALUES (?, ?, ?)",
		defaultName, defaultRows, defaultCols,
	)
	if err != nil {
		return fmt.Errorf("insert shelf: %w", err)
	}
	shelfID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get shelf id: %w", err)
	}

	for col := 1; col <= defaultCols; col++ {
		for row := 1; row <= defaultRows; row++ {
			if _, err := tx.Exec(
				"INSERT INTO container (shelf_id, col, row) VALUES (?, ?, ?)",
				shelfID, col, row,
			); err != nil {
				return fmt.Errorf("insert container (%d,%d): %w", col, row, err)
			}
		}
	}

	slog.Info("seeded default shelf", "name", defaultName, "rows", defaultRows, "cols", defaultCols, "containers", defaultRows*defaultCols)
	return tx.Commit()
}

// pos is an unexported key for the item-count lookup map.
type pos struct{ row, col int }

// GetGridData loads the first shelf and builds a GridData view model
// with container item counts for template rendering.
func (s *Store) GetGridData() (*GridData, error) {
	// Load the first shelf.
	var shelfID int64
	var shelf Shelf
	err := s.db.QueryRow("SELECT id, name, rows, cols FROM shelf LIMIT 1").
		Scan(&shelfID, &shelf.Name, &shelf.Rows, &shelf.Cols)
	if err != nil {
		return nil, fmt.Errorf("query shelf: %w", err)
	}

	// Query containers with item counts.
	rows, err := s.db.Query(`
		SELECT c.col, c.row, COUNT(i.id) AS item_count
		FROM container c
		LEFT JOIN item i ON c.id = i.container_id
		WHERE c.shelf_id = ?
		GROUP BY c.id, c.col, c.row
		ORDER BY c.row, c.col`, shelfID)
	if err != nil {
		return nil, fmt.Errorf("query containers: %w", err)
	}
	defer rows.Close()

	counts := make(map[pos]int)
	for rows.Next() {
		var col, row, count int
		if err := rows.Scan(&col, &row, &count); err != nil {
			return nil, fmt.Errorf("scan container row: %w", err)
		}
		counts[pos{row: row, col: col}] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate containers: %w", err)
	}

	// Build ColNumbers slice.
	colNums := make([]int, shelf.Cols)
	for i := range colNums {
		colNums[i] = i + 1
	}

	// Build nested Grid structure.
	grid := make([]Row, shelf.Rows)
	for r := 1; r <= shelf.Rows; r++ {
		row := Row{
			Letter: string(rune('A' + r - 1)),
			Cells:  make([]Cell, shelf.Cols),
		}
		for c := 1; c <= shelf.Cols; c++ {
			count := counts[pos{row: r, col: c}]
			cssClass := "cell-light"
			if (c+r)%2 != 0 {
				cssClass = "cell-dark"
			}
			row.Cells[c-1] = Cell{
				Coord:    labelFor(c, r),
				Col:      c,
				Row:      r,
				Count:    count,
				IsEmpty:  count == 0,
				CSSClass: cssClass,
			}
		}
		grid[r-1] = row
	}

	return &GridData{
		ShelfName:  shelf.Name,
		Rows:       shelf.Rows,
		Cols:       shelf.Cols,
		ColNumbers: colNums,
		Grid:       grid,
	}, nil
}
