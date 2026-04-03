package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
		SELECT c.id, c.col, c.row, COUNT(i.id) AS item_count
		FROM container c
		LEFT JOIN item i ON c.id = i.container_id
		WHERE c.shelf_id = ?
		GROUP BY c.id, c.col, c.row
		ORDER BY c.row, c.col`, shelfID)
	if err != nil {
		return nil, fmt.Errorf("query containers: %w", err)
	}
	defer rows.Close()

	type cellInfo struct {
		containerID int64
		count       int
	}
	cells := make(map[pos]cellInfo)
	for rows.Next() {
		var containerID int64
		var col, row, count int
		if err := rows.Scan(&containerID, &col, &row, &count); err != nil {
			return nil, fmt.Errorf("scan container row: %w", err)
		}
		cells[pos{row: row, col: col}] = cellInfo{containerID: containerID, count: count}
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
			info := cells[pos{row: r, col: c}]
			cssClass := "cell-light"
			if (c+r)%2 != 0 {
				cssClass = "cell-dark"
			}
			row.Cells[c-1] = Cell{
				Coord:       labelFor(c, r),
				Col:         c,
				Row:         r,
				Count:       info.count,
				IsEmpty:     info.count == 0,
				CSSClass:    cssClass,
				ContainerID: info.containerID,
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

// formatTime formats a time.Time as RFC3339 string for API responses.
func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// CreateItem inserts an item with name, description, and tags in a single transaction.
// Tags are auto-created if they don't exist (per D-14). Caller should deduplicate
// and lowercase-normalize tags before calling.
func (s *Store) CreateItem(ctx context.Context, containerID int64, name string, description *string, tags []string) (*ItemResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check container exists (per D-11).
	var cID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM container WHERE id = ?", containerID).Scan(&cID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("container not found")
	}
	if err != nil {
		return nil, fmt.Errorf("check container: %w", err)
	}

	// Insert item.
	res, err := tx.ExecContext(ctx,
		"INSERT INTO item (container_id, name, description, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))",
		containerID, name, description)
	if err != nil {
		return nil, fmt.Errorf("insert item: %w", err)
	}
	itemID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get item id: %w", err)
	}

	// Insert tags and associations.
	for _, tagName := range tags {
		// Auto-create tag if not exists (per D-14).
		if _, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO tag (name, created_at, updated_at) VALUES (?, datetime('now'), datetime('now'))",
			tagName); err != nil {
			return nil, fmt.Errorf("insert tag %q: %w", tagName, err)
		}

		var tagID int64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM tag WHERE name = ?", tagName).Scan(&tagID); err != nil {
			return nil, fmt.Errorf("get tag id %q: %w", tagName, err)
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO item_tag (item_id, tag_id) VALUES (?, ?)",
			itemID, tagID); err != nil {
			return nil, fmt.Errorf("insert item_tag: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.GetItem(ctx, itemID)
}

// GetItem returns an item with tags and computed container label.
// Returns nil, nil if the item does not exist.
func (s *Store) GetItem(ctx context.Context, id int64) (*ItemResponse, error) {
	var item ItemResponse
	var col, row int
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT i.id, i.container_id, i.name, i.description, c.col, c.row, i.created_at, i.updated_at
		 FROM item i JOIN container c ON c.id = i.container_id
		 WHERE i.id = ?`, id).
		Scan(&item.ID, &item.ContainerID, &item.Name, &item.Description, &col, &row, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}

	item.ContainerLabel = labelFor(col, row)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)

	// Fetch tags.
	tagRows, err := s.db.QueryContext(ctx,
		"SELECT t.name FROM tag t JOIN item_tag it ON it.tag_id = t.id WHERE it.item_id = ? ORDER BY t.name", id)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer tagRows.Close()

	tags := []string{}
	for tagRows.Next() {
		var tagName string
		if err := tagRows.Scan(&tagName); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tagName)
	}
	if err := tagRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	item.Tags = tags

	return &item, nil
}

// UpdateItem changes name, description, and container_id of an item.
// Per D-18: does NOT touch tags.
// Returns nil, nil if the item does not exist.
func (s *Store) UpdateItem(ctx context.Context, id int64, name string, description *string, containerID int64) (*ItemResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check item exists.
	var itemID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM item WHERE id = ?", id).Scan(&itemID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("check item: %w", err)
	}

	// Check target container exists.
	var cID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM container WHERE id = ?", containerID).Scan(&cID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("container not found")
	}
	if err != nil {
		return nil, fmt.Errorf("check container: %w", err)
	}

	// Update item.
	if _, err := tx.ExecContext(ctx,
		"UPDATE item SET name = ?, description = ?, container_id = ?, updated_at = datetime('now') WHERE id = ?",
		name, description, containerID, id); err != nil {
		return nil, fmt.Errorf("update item: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.GetItem(ctx, id)
}

// DeleteItem removes an item by ID. CASCADE handles item_tag cleanup.
// Returns error if the item does not exist.
func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	var itemID int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM item WHERE id = ?", id).Scan(&itemID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("item not found")
	}
	if err != nil {
		return fmt.Errorf("check item: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, "DELETE FROM item WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

// AddTagToItem creates the tag if not exists and links it to the item.
// Returns the updated item.
func (s *Store) AddTagToItem(ctx context.Context, itemID int64, tagName string) (*ItemResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check item exists.
	var iID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM item WHERE id = ?", itemID).Scan(&iID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item not found")
	}
	if err != nil {
		return nil, fmt.Errorf("check item: %w", err)
	}

	// Auto-create tag if not exists (per D-14).
	if _, err := tx.ExecContext(ctx,
		"INSERT OR IGNORE INTO tag (name, created_at, updated_at) VALUES (?, datetime('now'), datetime('now'))",
		tagName); err != nil {
		return nil, fmt.Errorf("insert tag: %w", err)
	}

	var tagID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM tag WHERE name = ?", tagName).Scan(&tagID); err != nil {
		return nil, fmt.Errorf("get tag id: %w", err)
	}

	// Link tag to item (IGNORE handles already-linked case).
	if _, err := tx.ExecContext(ctx,
		"INSERT OR IGNORE INTO item_tag (item_id, tag_id) VALUES (?, ?)",
		itemID, tagID); err != nil {
		return nil, fmt.Errorf("insert item_tag: %w", err)
	}

	// Update item.updated_at.
	if _, err := tx.ExecContext(ctx,
		"UPDATE item SET updated_at = datetime('now') WHERE id = ?", itemID); err != nil {
		return nil, fmt.Errorf("update item timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.GetItem(ctx, itemID)
}

// RemoveTagFromItem removes the tag association from an item.
// Per D-15: the tag itself remains in the tag table (orphaned tags kept).
func (s *Store) RemoveTagFromItem(ctx context.Context, itemID int64, tagName string) error {
	// Get tag ID.
	var tagID int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM tag WHERE name = ?", tagName).Scan(&tagID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("tag not found")
	}
	if err != nil {
		return fmt.Errorf("get tag: %w", err)
	}

	// Delete junction row.
	res, err := s.db.ExecContext(ctx, "DELETE FROM item_tag WHERE item_id = ? AND tag_id = ?", itemID, tagID)
	if err != nil {
		return fmt.Errorf("delete item_tag: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("tag not associated with item")
	}

	// Update item.updated_at.
	if _, err := s.db.ExecContext(ctx,
		"UPDATE item SET updated_at = datetime('now') WHERE id = ?", itemID); err != nil {
		return fmt.Errorf("update item timestamp: %w", err)
	}

	return nil
}

// ListItemsByContainer returns a container with all its items and tags.
// Returns nil, nil if the container does not exist.
func (s *Store) ListItemsByContainer(ctx context.Context, containerID int64) (*ContainerWithItems, error) {
	var c ContainerWithItems
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		"SELECT id, shelf_id, col, row, created_at, updated_at FROM container WHERE id = ?",
		containerID).Scan(&c.ID, &c.ShelfID, &c.Col, &c.Row, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get container: %w", err)
	}

	c.Label = labelFor(c.Col, c.Row)
	c.CreatedAt = formatTime(createdAt)
	c.UpdatedAt = formatTime(updatedAt)

	// Query item IDs in this container.
	rows, err := s.db.QueryContext(ctx,
		"SELECT id FROM item WHERE container_id = ? ORDER BY name", containerID)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	items := []ItemResponse{}
	for rows.Next() {
		var itemID int64
		if err := rows.Scan(&itemID); err != nil {
			return nil, fmt.Errorf("scan item id: %w", err)
		}
		item, err := s.GetItem(ctx, itemID)
		if err != nil {
			return nil, fmt.Errorf("get item %d: %w", itemID, err)
		}
		if item != nil {
			items = append(items, *item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}
	c.Items = items

	return &c, nil
}

// ListAllItems returns all items across all containers with tags.
func (s *Store) ListAllItems(ctx context.Context) ([]ItemResponse, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM item ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	items := []ItemResponse{}
	for rows.Next() {
		var itemID int64
		if err := rows.Scan(&itemID); err != nil {
			return nil, fmt.Errorf("scan item id: %w", err)
		}
		item, err := s.GetItem(ctx, itemID)
		if err != nil {
			return nil, fmt.Errorf("get item %d: %w", itemID, err)
		}
		if item != nil {
			items = append(items, *item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}

	return items, nil
}

// SearchItems finds items matching query by partial name (case-insensitive LIKE)
// or exact tag match. Results are deduplicated and sorted by container position
// (col ASC, row ASC). The query should already be lowercased and trimmed by the caller.
func (s *Store) SearchItems(ctx context.Context, query string) ([]ItemResponse, error) {
	if query == "" {
		return []ItemResponse{}, nil
	}

	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	likePattern := "%" + escaped + "%"

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT i.id, c.col, c.row
		FROM item i
		JOIN container c ON c.id = i.container_id
		WHERE LOWER(i.name) LIKE ? ESCAPE '\'

		UNION

		SELECT DISTINCT i.id, c.col, c.row
		FROM item i
		JOIN container c ON c.id = i.container_id
		JOIN item_tag it ON it.item_id = i.id
		JOIN tag t ON t.id = it.tag_id
		WHERE t.name = ?

		ORDER BY col ASC, row ASC
	`, likePattern, query)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	defer rows.Close()

	var items []ItemResponse
	for rows.Next() {
		var id int64
		var col, row int
		if err := rows.Scan(&id, &col, &row); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		item, err := s.GetItem(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get item %d: %w", id, err)
		}
		if item != nil {
			items = append(items, *item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	if items == nil {
		items = []ItemResponse{}
	}
	return items, nil
}

// ListTags returns all tags, optionally filtered by name prefix.
func (s *Store) ListTags(ctx context.Context, prefix string) ([]TagResponse, error) {
	var rows *sql.Rows
	var err error

	if prefix == "" {
		rows, err = s.db.QueryContext(ctx, "SELECT id, name, created_at, updated_at FROM tag ORDER BY name")
	} else {
		rows, err = s.db.QueryContext(ctx, "SELECT id, name, created_at, updated_at FROM tag WHERE name LIKE ? ORDER BY name", prefix+"%")
	}
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	tags := []TagResponse{}
	for rows.Next() {
		var t TagResponse
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.ID, &t.Name, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		t.CreatedAt = formatTime(createdAt)
		t.UpdatedAt = formatTime(updatedAt)
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}
