package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"screws-box/internal/model"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite" // register SQLite driver
)

// ErrTagInUse is returned when attempting to delete a tag that still has item associations.
var ErrTagInUse = errors.New("tag is in use")

// hashPassword hashes a password using bcrypt with default cost.
func hashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		// bcrypt only fails if password > 72 bytes; this is validated upstream.
		panic(fmt.Sprintf("bcrypt hash: %v", err))
	}
	return string(hash)
}

// checkPassword verifies a password against a stored hash.
// Supports bcrypt hashes and legacy sha256:salt:hash format (read-only, for migration).
func checkPassword(stored, password string) bool {
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	}
	// Legacy sha256 hashes — constant-time comparison not needed because
	// bcrypt.CompareHashAndPassword already handles timing for bcrypt.
	// For legacy hashes, reject outright to force password reset.
	slog.Warn("legacy password hash detected, user must reset password")
	return false
}

// schemaDDL contains all CREATE TABLE and CREATE INDEX statements.
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
	`CREATE INDEX IF NOT EXISTS idx_container_shelf_id ON container(shelf_id)`,
	`CREATE INDEX IF NOT EXISTS idx_item_container_id ON item(container_id)`,
	`CREATE INDEX IF NOT EXISTS idx_item_tag_item_id ON item_tag(item_id)`,
	`CREATE INDEX IF NOT EXISTS idx_item_tag_tag_id ON item_tag(tag_id)`,
	`CREATE TABLE IF NOT EXISTS oidc_user (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sub TEXT NOT NULL,
		issuer TEXT NOT NULL,
		email TEXT NOT NULL DEFAULT '',
		display_name TEXT NOT NULL DEFAULT '',
		avatar_url TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
		UNIQUE(sub, issuer)
	)`,
}

// migrations runs ALTER TABLE statements for columns added after the initial schema.
// Each is idempotent: errors from "duplicate column" are silently ignored.
var migrations = []string{
	`ALTER TABLE shelf ADD COLUMN auth_enabled INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE shelf ADD COLUMN auth_user TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE shelf ADD COLUMN auth_pass TEXT NOT NULL DEFAULT ''`,
	// OIDC config columns on shelf table
	`ALTER TABLE shelf ADD COLUMN oidc_enabled INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE shelf ADD COLUMN oidc_issuer TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE shelf ADD COLUMN oidc_client_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE shelf ADD COLUMN oidc_client_secret TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE shelf ADD COLUMN oidc_display_name TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE shelf ADD COLUMN encryption_key TEXT NOT NULL DEFAULT ''`,
}

// deferRollback is used with defer to rollback a transaction.
// After a successful Commit, Rollback returns sql.ErrTxDone which is expected.
// Any other error indicates a real problem and gets logged.
func deferRollback(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
		slog.Error("tx rollback failed", "err", err)
	}
}

// Store wraps the SQLite database connection.
type Store struct {
	conn *sql.DB
}

// DisableAuth clears all authentication settings on the shelf.
func (s *Store) DisableAuth() error {
	_, err := s.conn.Exec("UPDATE shelf SET auth_enabled = 0, auth_user = '', auth_pass = '' WHERE id = (SELECT id FROM shelf LIMIT 1)")
	return err
}

// GetContainerIDByPosition returns the container ID at the given grid position.
func (s *Store) GetContainerIDByPosition(col, row int) (int64, error) {
	var id int64
	err := s.conn.QueryRow("SELECT id FROM container WHERE col = ? AND row = ?", col, row).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get container by position (%d,%d): %w", col, row, err)
	}
	return id, nil
}

// GetShelfName returns the name of the first shelf.
func (s *Store) GetShelfName() (string, error) {
	var name string
	err := s.conn.QueryRow("SELECT name FROM shelf LIMIT 1").Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get shelf name: %w", err)
	}
	return name, nil
}

// GetRawAuthRow returns raw auth fields for testing purposes.
func (s *Store) GetRawAuthRow() (enabled int, user, passHash string, err error) {
	err = s.conn.QueryRow("SELECT auth_enabled, auth_user, auth_pass FROM shelf LIMIT 1").Scan(&enabled, &user, &passHash)
	return
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.conn.PingContext(ctx)
}

// Open opens the SQLite database at dbPath, configures pragmas,
// creates the schema, and seeds the default shelf if needed.
func (s *Store) Open(dbPath string) error {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	s.conn = db

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
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *Store) createSchema() error {
	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin schema tx: %w", err)
	}
	defer deferRollback(tx)

	for _, ddl := range schemaDDL {
		if _, err := tx.Exec(ddl); err != nil {
			return fmt.Errorf("schema exec: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema: %w", err)
	}

	// Run migrations (idempotent ALTER TABLE statements).
	for _, m := range migrations {
		_, _ = s.conn.Exec(m) // ignore "duplicate column" errors
	}
	return nil
}

func (s *Store) seedDefaultShelf() error {
	var count int
	if err := s.conn.QueryRow("SELECT COUNT(*) FROM shelf").Scan(&count); err != nil {
		return fmt.Errorf("check shelf count: %w", err)
	}
	if count > 0 {
		return nil
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer deferRollback(tx)

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

// GetGridData loads the first shelf and builds a GridData view model.
func (s *Store) GetGridData() (*model.GridData, error) {
	var shelfID int64
	var shelf model.Shelf
	var authEnabled int
	var authUser, authPass string
	err := s.conn.QueryRow("SELECT id, name, rows, cols, auth_enabled, auth_user, auth_pass FROM shelf LIMIT 1").
		Scan(&shelfID, &shelf.Name, &shelf.Rows, &shelf.Cols, &authEnabled, &authUser, &authPass)
	if err != nil {
		return nil, fmt.Errorf("query shelf: %w", err)
	}

	rows, err := s.conn.Query(`
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

	colNums := make([]int, shelf.Cols)
	for i := range colNums {
		colNums[i] = i + 1
	}

	grid := make([]model.Row, shelf.Rows)
	for r := 1; r <= shelf.Rows; r++ {
		row := model.Row{
			Letter: string(rune('A' + r - 1)),
			Cells:  make([]model.Cell, shelf.Cols),
		}
		for c := 1; c <= shelf.Cols; c++ {
			info := cells[pos{row: r, col: c}]
			cssClass := "cell-light"
			if (c+r)%2 != 0 {
				cssClass = "cell-dark"
			}
			row.Cells[c-1] = model.Cell{
				Coord:       model.LabelFor(c, r),
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

	return &model.GridData{
		ShelfName:       shelf.Name,
		Rows:            shelf.Rows,
		Cols:            shelf.Cols,
		ColNumbers:      colNums,
		Grid:            grid,
		AuthEnabled:     authEnabled != 0,
		AuthUser:        authUser,
		AuthHasPassword: authPass != "",
	}, nil
}

// CreateItem inserts an item with name, description, and tags in a single transaction.
func (s *Store) CreateItem(ctx context.Context, containerID int64, name string, description *string, tags []string) (*model.ItemResponse, error) {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer deferRollback(tx)

	var cID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM container WHERE id = ?", containerID).Scan(&cID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("container not found")
	}
	if err != nil {
		return nil, fmt.Errorf("check container: %w", err)
	}

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

	for _, tagName := range tags {
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
func (s *Store) GetItem(ctx context.Context, id int64) (*model.ItemResponse, error) {
	var item model.ItemResponse
	var col, row int
	var createdAt, updatedAt time.Time

	err := s.conn.QueryRowContext(ctx,
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

	item.ContainerLabel = model.LabelFor(col, row)
	item.CreatedAt = model.FormatTime(createdAt)
	item.UpdatedAt = model.FormatTime(updatedAt)

	tagRows, err := s.conn.QueryContext(ctx,
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
// Does NOT touch tags. Returns nil, nil if the item does not exist.
func (s *Store) UpdateItem(ctx context.Context, id int64, name string, description *string, containerID int64) (*model.ItemResponse, error) {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer deferRollback(tx)

	var itemID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM item WHERE id = ?", id).Scan(&itemID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("check item: %w", err)
	}

	var cID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM container WHERE id = ?", containerID).Scan(&cID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("container not found")
	}
	if err != nil {
		return nil, fmt.Errorf("check container: %w", err)
	}

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
func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	var itemID int64
	err := s.conn.QueryRowContext(ctx, "SELECT id FROM item WHERE id = ?", id).Scan(&itemID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("item not found")
	}
	if err != nil {
		return fmt.Errorf("check item: %w", err)
	}

	if _, err := s.conn.ExecContext(ctx, "DELETE FROM item WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

// AddTagToItem creates the tag if not exists and links it to the item.
func (s *Store) AddTagToItem(ctx context.Context, itemID int64, tagName string) (*model.ItemResponse, error) {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer deferRollback(tx)

	var iID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM item WHERE id = ?", itemID).Scan(&iID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item not found")
	}
	if err != nil {
		return nil, fmt.Errorf("check item: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT OR IGNORE INTO tag (name, created_at, updated_at) VALUES (?, datetime('now'), datetime('now'))",
		tagName); err != nil {
		return nil, fmt.Errorf("insert tag: %w", err)
	}

	var tagID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM tag WHERE name = ?", tagName).Scan(&tagID); err != nil {
		return nil, fmt.Errorf("get tag id: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT OR IGNORE INTO item_tag (item_id, tag_id) VALUES (?, ?)",
		itemID, tagID); err != nil {
		return nil, fmt.Errorf("insert item_tag: %w", err)
	}

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
// The tag itself remains in the tag table (orphaned tags kept).
func (s *Store) RemoveTagFromItem(ctx context.Context, itemID int64, tagName string) error {
	var tagID int64
	err := s.conn.QueryRowContext(ctx, "SELECT id FROM tag WHERE name = ?", tagName).Scan(&tagID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("tag not found")
	}
	if err != nil {
		return fmt.Errorf("get tag: %w", err)
	}

	res, err := s.conn.ExecContext(ctx, "DELETE FROM item_tag WHERE item_id = ? AND tag_id = ?", itemID, tagID)
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

	if _, err := s.conn.ExecContext(ctx,
		"UPDATE item SET updated_at = datetime('now') WHERE id = ?", itemID); err != nil {
		return fmt.Errorf("update item timestamp: %w", err)
	}

	return nil
}

// ListItemsByContainer returns a container with all its items and tags.
// Returns nil, nil if the container does not exist.
func (s *Store) ListItemsByContainer(ctx context.Context, containerID int64) (*model.ContainerWithItems, error) {
	var c model.ContainerWithItems
	var createdAt, updatedAt time.Time

	err := s.conn.QueryRowContext(ctx,
		"SELECT id, shelf_id, col, row, created_at, updated_at FROM container WHERE id = ?",
		containerID).Scan(&c.ID, &c.ShelfID, &c.Col, &c.Row, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get container: %w", err)
	}

	c.Label = model.LabelFor(c.Col, c.Row)
	c.CreatedAt = model.FormatTime(createdAt)
	c.UpdatedAt = model.FormatTime(updatedAt)

	rows, err := s.conn.QueryContext(ctx,
		"SELECT id FROM item WHERE container_id = ? ORDER BY name", containerID)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	items := []model.ItemResponse{}
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
func (s *Store) ListAllItems(ctx context.Context) ([]model.ItemResponse, error) {
	rows, err := s.conn.QueryContext(ctx, "SELECT id FROM item ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	items := []model.ItemResponse{}
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

// SearchItems finds items matching query by partial name or exact tag match.
func (s *Store) SearchItems(ctx context.Context, query string) ([]model.ItemResponse, error) {
	if query == "" {
		return []model.ItemResponse{}, nil
	}

	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	likePattern := "%" + escaped + "%"

	rows, err := s.conn.QueryContext(ctx, `
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

	var items []model.ItemResponse
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
		items = []model.ItemResponse{}
	}
	return items, nil
}

// SearchItemsByTags returns items matching ALL given tags, optionally filtered by a name query.
// When query is empty, returns all items that have all specified tags.
// When query is non-empty, also filters by item name (LIKE match), but does NOT search tags by name.
func (s *Store) SearchItemsByTags(ctx context.Context, query string, tags []string) ([]model.ItemResponse, error) {
	if len(tags) == 0 {
		return s.SearchItems(ctx, query)
	}

	// Build query: items that have ALL specified tags.
	// Use COUNT to ensure the item has every requested tag.
	args := make([]any, 0, len(tags)+1)
	sql := `
		SELECT DISTINCT i.id, c.col, c.row
		FROM item i
		JOIN container c ON c.id = i.container_id
		JOIN item_tag it ON it.item_id = i.id
		JOIN tag t ON t.id = it.tag_id
		WHERE t.name IN (`

	for idx, tag := range tags {
		if idx > 0 {
			sql += ", "
		}
		sql += "?"
		args = append(args, tag)
	}
	sql += fmt.Sprintf(`) GROUP BY i.id, c.col, c.row HAVING COUNT(DISTINCT t.name) = %d`, len(tags)) //nolint:gosec // G202: len(tags) is an integer, not user input

	if query != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
		sql += " AND LOWER(i.name) LIKE ? ESCAPE '\\'"
		args = append(args, "%"+escaped+"%")
	}

	sql += " ORDER BY c.col ASC, c.row ASC"

	rows, err := s.conn.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search items by tags: %w", err)
	}
	defer rows.Close()

	var items []model.ItemResponse
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
		items = []model.ItemResponse{}
	}
	return items, nil
}

// SearchItemsBatch finds items matching a text query and/or tag filters in a single SQL round-trip.
// When tags is empty, matches name OR exact tag OR description (case-insensitive).
// When tags is non-empty, returns items having ALL specified tags (AND logic);
// text query filters on name+description only (tags excluded from text search per D-09).
// Results are capped at 50; TotalCount reflects the untruncated count.
func (s *Store) SearchItemsBatch(ctx context.Context, query string, tags []string) (*model.SearchResponse, error) {
	if query == "" && len(tags) == 0 {
		return &model.SearchResponse{Results: []model.SearchResult{}, TotalCount: 0}, nil
	}

	escaper := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	lowerQuery := strings.ToLower(query)
	likePattern := "%" + escaper.Replace(lowerQuery) + "%"
	tagsActive := len(tags) > 0

	var sqlStr string
	var args []any

	if !tagsActive {
		// Path 1: No tag filters — match name OR exact tag OR description
		sqlStr = `
			SELECT i.id, i.container_id, i.name, i.description,
			       c.col, c.row, i.created_at, i.updated_at,
			       GROUP_CONCAT(t.name, '|') AS tag_list
			FROM item i
			JOIN container c ON c.id = i.container_id
			LEFT JOIN item_tag it ON it.item_id = i.id
			LEFT JOIN tag t ON t.id = it.tag_id
			WHERE LOWER(i.name) LIKE ? ESCAPE '\'
			   OR t.name = ?
			   OR LOWER(COALESCE(i.description, '')) LIKE ? ESCAPE '\'
			GROUP BY i.id, i.container_id, i.name, i.description,
			         c.col, c.row, i.created_at, i.updated_at
			ORDER BY c.col ASC, c.row ASC
			LIMIT 51`
		args = []any{likePattern, lowerQuery, likePattern}
	} else {
		// Path 2: With tag filters — AND logic on tags
		placeholders := make([]string, len(tags))
		args = make([]any, 0, len(tags)+2)
		for i, tag := range tags {
			placeholders[i] = "?"
			args = append(args, strings.ToLower(tag))
		}

		sqlStr = fmt.Sprintf(`
			SELECT i.id, i.container_id, i.name, i.description,
			       c.col, c.row, i.created_at, i.updated_at,
			       (SELECT GROUP_CONCAT(t2.name, '|') FROM item_tag it2 JOIN tag t2 ON t2.id = it2.tag_id WHERE it2.item_id = i.id) AS tag_list
			FROM item i
			JOIN container c ON c.id = i.container_id
			JOIN item_tag it ON it.item_id = i.id
			JOIN tag t ON t.id = it.tag_id
			WHERE t.name IN (%s)
			GROUP BY i.id, i.container_id, i.name, i.description,
			         c.col, c.row, i.created_at, i.updated_at
			HAVING COUNT(DISTINCT t.name) = %d`, strings.Join(placeholders, ", "), len(tags)) //nolint:gosec // G202: len(tags) is an integer, not user input

		if query != "" {
			sqlStr += ` AND (LOWER(i.name) LIKE ? ESCAPE '\' OR LOWER(COALESCE(i.description, '')) LIKE ? ESCAPE '\')`
			args = append(args, likePattern, likePattern)
		}

		sqlStr += `
			ORDER BY c.col ASC, c.row ASC
			LIMIT 51`
	}

	rows, err := s.conn.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("search items batch: %w", err)
	}
	defer rows.Close()

	var results []model.SearchResult
	for rows.Next() {
		var (
			id                   int64
			containerID          int64
			name                 string
			description          sql.NullString
			col, row             int
			createdAt, updatedAt time.Time
			tagList              sql.NullString
		)
		if err := rows.Scan(&id, &containerID, &name, &description, &col, &row, &createdAt, &updatedAt, &tagList); err != nil {
			return nil, fmt.Errorf("scan search batch result: %w", err)
		}

		var itemTags []string
		if tagList.Valid && tagList.String != "" {
			itemTags = strings.Split(tagList.String, "|")
			// Deduplicate tags (GROUP_CONCAT may duplicate when joining)
			itemTags = model.Dedup(itemTags)
			sortStrings(itemTags)
		} else {
			itemTags = []string{}
		}

		var desc *string
		if description.Valid {
			desc = &description.String
		}

		matchedOn := computeMatchedOn(name, desc, itemTags, lowerQuery, tagsActive)

		result := model.SearchResult{
			ItemResponse: model.ItemResponse{
				ID:             id,
				ContainerID:    containerID,
				ContainerLabel: model.LabelFor(col, row),
				Name:           name,
				Description:    desc,
				Tags:           itemTags,
				CreatedAt:      model.FormatTime(createdAt),
				UpdatedAt:      model.FormatTime(updatedAt),
			},
			MatchedOn: matchedOn,
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search batch results: %w", err)
	}

	totalCount := len(results)
	if len(results) == 51 {
		// More than 50 results exist — run count query
		count, countErr := s.searchBatchCount(ctx, query, tags, tagsActive, likePattern, lowerQuery)
		if countErr != nil {
			return nil, countErr
		}
		totalCount = count
		results = results[:50]
	}

	if results == nil {
		results = []model.SearchResult{}
	}

	return &model.SearchResponse{Results: results, TotalCount: totalCount}, nil
}

// searchBatchCount runs a COUNT(*) query matching the same filters as SearchItemsBatch.
func (s *Store) searchBatchCount(ctx context.Context, query string, tags []string, tagsActive bool, likePattern, lowerQuery string) (int, error) {
	var sqlStr string
	var args []any

	if !tagsActive {
		sqlStr = `
			SELECT COUNT(*) FROM (
				SELECT i.id
				FROM item i
				JOIN container c ON c.id = i.container_id
				LEFT JOIN item_tag it ON it.item_id = i.id
				LEFT JOIN tag t ON t.id = it.tag_id
				WHERE LOWER(i.name) LIKE ? ESCAPE '\'
				   OR t.name = ?
				   OR LOWER(COALESCE(i.description, '')) LIKE ? ESCAPE '\'
				GROUP BY i.id
			)`
		args = []any{likePattern, lowerQuery, likePattern}
	} else {
		placeholders := make([]string, len(tags))
		args = make([]any, 0, len(tags)+2)
		for i, tag := range tags {
			placeholders[i] = "?"
			args = append(args, strings.ToLower(tag))
		}

		inner := fmt.Sprintf(`
			SELECT i.id
			FROM item i
			JOIN item_tag it ON it.item_id = i.id
			JOIN tag t ON t.id = it.tag_id
			WHERE t.name IN (%s)
			GROUP BY i.id
			HAVING COUNT(DISTINCT t.name) = %d`, strings.Join(placeholders, ", "), len(tags)) //nolint:gosec // G202: len(tags) is integer

		if query != "" {
			inner += ` AND (LOWER(i.name) LIKE ? ESCAPE '\' OR LOWER(COALESCE(i.description, '')) LIKE ? ESCAPE '\')`
			args = append(args, likePattern, likePattern)
		}

		sqlStr = fmt.Sprintf(`SELECT COUNT(*) FROM (%s)`, inner)
	}

	var count int
	if err := s.conn.QueryRowContext(ctx, sqlStr, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count search batch: %w", err)
	}
	return count, nil
}

// computeMatchedOn determines which fields matched the search query.
func computeMatchedOn(name string, description *string, tags []string, query string, tagsActive bool) []string {
	if query == "" {
		return nil
	}
	var matched []string
	if strings.Contains(strings.ToLower(name), query) {
		matched = append(matched, "name")
	}
	if description != nil && strings.Contains(strings.ToLower(*description), query) {
		matched = append(matched, "description")
	}
	if !tagsActive {
		for _, tag := range tags {
			if tag == query {
				matched = append(matched, "tag")
				break
			}
		}
	}
	return matched
}

// sortStrings sorts a string slice in place.
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

// ResizeShelf atomically resizes the shelf grid.
func (s *Store) ResizeShelf(ctx context.Context, newRows, newCols int) (*model.ResizeResult, error) {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer deferRollback(tx)

	var shelfID int64
	var curRows, curCols int
	err = tx.QueryRowContext(ctx, "SELECT id, rows, cols FROM shelf LIMIT 1").Scan(&shelfID, &curRows, &curCols)
	if err != nil {
		return nil, fmt.Errorf("get shelf: %w", err)
	}

	outRows, err := tx.QueryContext(ctx,
		"SELECT c.id, c.col, c.row FROM container c WHERE c.shelf_id = ? AND (c.row > ? OR c.col > ?)",
		shelfID, newRows, newCols)
	if err != nil {
		return nil, fmt.Errorf("query out-of-bounds containers: %w", err)
	}
	defer outRows.Close()

	type outContainer struct {
		id  int64
		col int
		row int
	}
	var outsideContainers []outContainer
	for outRows.Next() {
		var oc outContainer
		if err := outRows.Scan(&oc.id, &oc.col, &oc.row); err != nil {
			return nil, fmt.Errorf("scan container: %w", err)
		}
		outsideContainers = append(outsideContainers, oc)
	}
	if err := outRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate containers: %w", err)
	}

	var affected []model.AffectedContainer
	for _, oc := range outsideContainers {
		itemRows, err := tx.QueryContext(ctx, "SELECT name FROM item WHERE container_id = ?", oc.id)
		if err != nil {
			return nil, fmt.Errorf("query items for container %d: %w", oc.id, err)
		}
		var items []string
		for itemRows.Next() {
			var name string
			if err := itemRows.Scan(&name); err != nil {
				itemRows.Close()
				return nil, fmt.Errorf("scan item name: %w", err)
			}
			items = append(items, name)
		}
		itemRows.Close()
		if err := itemRows.Err(); err != nil {
			return nil, fmt.Errorf("iterate items: %w", err)
		}

		if len(items) > 0 {
			affected = append(affected, model.AffectedContainer{
				Label:     model.LabelFor(oc.col, oc.row),
				ItemCount: len(items),
				Items:     items,
			})
		}
	}

	if len(affected) > 0 {
		return &model.ResizeResult{
			Rows:               curRows,
			Cols:               curCols,
			Blocked:            true,
			Message:            "Cannot resize: containers with items would be removed",
			AffectedContainers: affected,
		}, nil
	}

	delResult, err := tx.ExecContext(ctx,
		"DELETE FROM container WHERE shelf_id = ? AND (row > ? OR col > ?)",
		shelfID, newRows, newCols)
	if err != nil {
		return nil, fmt.Errorf("delete containers: %w", err)
	}
	removed, _ := delResult.RowsAffected()

	if _, err := tx.ExecContext(ctx,
		"UPDATE shelf SET rows = ?, cols = ?, updated_at = datetime('now') WHERE id = ?",
		newRows, newCols, shelfID); err != nil {
		return nil, fmt.Errorf("update shelf: %w", err)
	}

	var added int64
	for col := 1; col <= newCols; col++ {
		for row := 1; row <= newRows; row++ {
			res, err := tx.ExecContext(ctx,
				"INSERT OR IGNORE INTO container (shelf_id, col, row) VALUES (?, ?, ?)",
				shelfID, col, row)
			if err != nil {
				return nil, fmt.Errorf("insert container (%d,%d): %w", col, row, err)
			}
			ra, _ := res.RowsAffected()
			added += ra
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &model.ResizeResult{
		Rows:              newRows,
		Cols:              newCols,
		ContainersAdded:   int(added),
		ContainersRemoved: int(removed),
	}, nil
}

// UpdateShelfName updates the name of the first shelf.
func (s *Store) UpdateShelfName(ctx context.Context, name string) error {
	_, err := s.conn.ExecContext(ctx,
		"UPDATE shelf SET name = ?, updated_at = datetime('now') WHERE id = (SELECT id FROM shelf LIMIT 1)", name)
	return err
}

// GetAuthSettings returns the authentication settings for the shelf.
// The password hash is never returned to callers.
func (s *Store) GetAuthSettings(ctx context.Context) (*model.AuthSettings, error) {
	var enabled int
	var user, pass string
	err := s.conn.QueryRowContext(ctx, "SELECT auth_enabled, auth_user, auth_pass FROM shelf LIMIT 1").
		Scan(&enabled, &user, &pass)
	if err != nil {
		return nil, fmt.Errorf("get auth settings: %w", err)
	}
	return &model.AuthSettings{
		Enabled:     enabled != 0,
		Username:    user,
		HasPassword: pass != "",
	}, nil
}

// UpdateAuthSettings saves authentication settings.
// If Password is empty, the existing password is kept.
func (s *Store) UpdateAuthSettings(ctx context.Context, settings *model.AuthSettings) error {
	enabled := 0
	if settings.Enabled {
		enabled = 1
	}
	if settings.Password != "" {
		hashed := hashPassword(settings.Password)
		_, err := s.conn.ExecContext(ctx,
			"UPDATE shelf SET auth_enabled = ?, auth_user = ?, auth_pass = ?, updated_at = datetime('now') WHERE id = (SELECT id FROM shelf LIMIT 1)",
			enabled, settings.Username, hashed)
		return err
	}
	// No password change — keep existing
	_, err := s.conn.ExecContext(ctx,
		"UPDATE shelf SET auth_enabled = ?, auth_user = ?, updated_at = datetime('now') WHERE id = (SELECT id FROM shelf LIMIT 1)",
		enabled, settings.Username)
	return err
}

// ValidateCredentials checks username and password against stored auth settings.
func (s *Store) ValidateCredentials(ctx context.Context, username, password string) (bool, error) {
	var enabled int
	var storedUser, storedPass string
	err := s.conn.QueryRowContext(ctx, "SELECT auth_enabled, auth_user, auth_pass FROM shelf LIMIT 1").
		Scan(&enabled, &storedUser, &storedPass)
	if err != nil {
		return false, fmt.Errorf("get auth settings: %w", err)
	}
	if enabled == 0 {
		return true, nil
	}
	if username != storedUser {
		return false, nil
	}
	return checkPassword(storedPass, password), nil
}

// ListTags returns all tags, optionally filtered by name prefix.
// Each tag includes an ItemCount reflecting the number of associated items.
func (s *Store) ListTags(ctx context.Context, prefix string) ([]model.TagResponse, error) {
	var rows *sql.Rows
	var err error

	const baseQuery = `SELECT t.id, t.name, t.created_at, t.updated_at, COUNT(it.item_id) AS item_count
		FROM tag t
		LEFT JOIN item_tag it ON it.tag_id = t.id`

	if prefix == "" {
		rows, err = s.conn.QueryContext(ctx, baseQuery+" GROUP BY t.id ORDER BY t.name")
	} else {
		rows, err = s.conn.QueryContext(ctx, baseQuery+" WHERE t.name LIKE ? GROUP BY t.id ORDER BY t.name", prefix+"%")
	}
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	tags := []model.TagResponse{}
	for rows.Next() {
		var t model.TagResponse
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.ID, &t.Name, &createdAt, &updatedAt, &t.ItemCount); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		t.CreatedAt = model.FormatTime(createdAt)
		t.UpdatedAt = model.FormatTime(updatedAt)
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}

// RenameTag updates the name of a tag by ID.
// Returns an error if the tag is not found or if the new name conflicts with an existing tag.
func (s *Store) RenameTag(ctx context.Context, tagID int64, newName string) error {
	res, err := s.conn.ExecContext(ctx,
		"UPDATE tag SET name = ?, updated_at = datetime('now') WHERE id = ?",
		newName, tagID)
	if err != nil {
		return fmt.Errorf("rename tag: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rename tag rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}

// MergeTags moves all item associations from sourceID to targetID (deduplicating),
// then deletes the source tag. Runs in a transaction.
func (s *Store) MergeTags(ctx context.Context, sourceID, targetID int64) error {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("merge tags begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	// Move associations from source to target, ignoring duplicates
	if _, err := tx.ExecContext(ctx,
		"INSERT OR IGNORE INTO item_tag (item_id, tag_id) SELECT item_id, ? FROM item_tag WHERE tag_id = ?",
		targetID, sourceID); err != nil {
		return fmt.Errorf("merge tags copy associations: %w", err)
	}

	// Remove old associations
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM item_tag WHERE tag_id = ?", sourceID); err != nil {
		return fmt.Errorf("merge tags delete old associations: %w", err)
	}

	// Delete source tag
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM tag WHERE id = ?", sourceID); err != nil {
		return fmt.Errorf("merge tags delete source: %w", err)
	}

	return tx.Commit()
}

// DeleteUnusedTag deletes a tag only if it has no item associations.
// Returns ErrTagInUse if the tag still has items.
func (s *Store) DeleteUnusedTag(ctx context.Context, tagID int64) error {
	res, err := s.conn.ExecContext(ctx,
		`DELETE FROM tag WHERE id = ? AND NOT EXISTS (
			SELECT 1 FROM item_tag WHERE tag_id = ?
		)`, tagID, tagID)
	if err != nil {
		return fmt.Errorf("delete unused tag: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete unused tag rows affected: %w", err)
	}
	if n == 0 {
		return ErrTagInUse
	}
	return nil
}

// GetOIDCConfig returns the OIDC configuration including the client secret.
// For API responses, use GetOIDCConfigMasked instead (per T-14-01).
func (s *Store) GetOIDCConfig(ctx context.Context) (*model.OIDCConfig, error) {
	var enabled int
	var issuer, clientID, clientSecret, displayName string
	err := s.conn.QueryRowContext(ctx,
		"SELECT oidc_enabled, oidc_issuer, oidc_client_id, oidc_client_secret, oidc_display_name FROM shelf LIMIT 1").
		Scan(&enabled, &issuer, &clientID, &clientSecret, &displayName)
	if err != nil {
		return nil, fmt.Errorf("get oidc config: %w", err)
	}
	status := "not_set"
	if clientSecret != "" {
		status = "configured"
	}
	return &model.OIDCConfig{
		Enabled:      enabled != 0,
		IssuerURL:    issuer,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		DisplayName:  displayName,
		SecretStatus: status,
	}, nil
}

// GetOIDCConfigMasked returns the OIDC configuration with the client secret stripped.
// Safe for API responses per D-23 / T-14-01.
func (s *Store) GetOIDCConfigMasked(ctx context.Context) (*model.OIDCConfig, error) {
	cfg, err := s.GetOIDCConfig(ctx)
	if err != nil {
		return nil, err
	}
	cfg.ClientSecret = ""
	return cfg, nil
}

// SaveOIDCConfig updates the OIDC configuration on the shelf.
// If ClientSecret is empty, the existing secret is preserved.
func (s *Store) SaveOIDCConfig(ctx context.Context, cfg *model.OIDCConfig) error {
	enabled := 0
	if cfg.Enabled {
		enabled = 1
	}
	if cfg.ClientSecret != "" {
		_, err := s.conn.ExecContext(ctx,
			"UPDATE shelf SET oidc_enabled = ?, oidc_issuer = ?, oidc_client_id = ?, oidc_client_secret = ?, oidc_display_name = ?, updated_at = datetime('now') WHERE id = (SELECT id FROM shelf LIMIT 1)",
			enabled, cfg.IssuerURL, cfg.ClientID, cfg.ClientSecret, cfg.DisplayName)
		return err
	}
	// No secret change — keep existing
	_, err := s.conn.ExecContext(ctx,
		"UPDATE shelf SET oidc_enabled = ?, oidc_issuer = ?, oidc_client_id = ?, oidc_display_name = ?, updated_at = datetime('now') WHERE id = (SELECT id FROM shelf LIMIT 1)",
		enabled, cfg.IssuerURL, cfg.ClientID, cfg.DisplayName)
	return err
}

// UpsertOIDCUser creates or updates an OIDC user by sub+issuer.
// Returns the upserted row.
func (s *Store) UpsertOIDCUser(ctx context.Context, user *model.OIDCUser) (*model.OIDCUser, error) {
	_, err := s.conn.ExecContext(ctx,
		`INSERT INTO oidc_user (sub, issuer, email, display_name, avatar_url, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		 ON CONFLICT(sub, issuer) DO UPDATE SET
			email = excluded.email,
			display_name = excluded.display_name,
			avatar_url = excluded.avatar_url,
			updated_at = datetime('now')`,
		user.Sub, user.Issuer, user.Email, user.DisplayName, user.AvatarURL)
	if err != nil {
		return nil, fmt.Errorf("upsert oidc user: %w", err)
	}
	return s.GetOIDCUserBySub(ctx, user.Sub, user.Issuer)
}

// GetOIDCUserBySub returns an OIDC user by subject ID and issuer.
// Returns nil, nil if not found.
func (s *Store) GetOIDCUserBySub(ctx context.Context, sub, issuer string) (*model.OIDCUser, error) {
	var u model.OIDCUser
	err := s.conn.QueryRowContext(ctx,
		"SELECT id, sub, issuer, email, display_name, avatar_url, created_at, updated_at FROM oidc_user WHERE sub = ? AND issuer = ?",
		sub, issuer).
		Scan(&u.ID, &u.Sub, &u.Issuer, &u.Email, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get oidc user: %w", err)
	}
	return &u, nil
}

// FindDuplicates returns groups of items that share the same normalized name
// and identical tag set across multiple containers.
func (s *Store) FindDuplicates(ctx context.Context) ([]model.DuplicateGroup, error) {
	const query = `
WITH item_fp AS (
    SELECT
        i.id,
        i.name AS original_name,
        LOWER(TRIM(i.name)) AS norm_name,
        i.container_id,
        (SELECT GROUP_CONCAT(t.name, '|')
         FROM item_tag it
         JOIN tag t ON t.id = it.tag_id
         WHERE it.item_id = i.id
         ORDER BY t.name
        ) AS tag_fingerprint
    FROM item i
),
dup_groups AS (
    SELECT
        norm_name,
        COALESCE(tag_fingerprint, '') AS tfp
    FROM item_fp
    GROUP BY norm_name, COALESCE(tag_fingerprint, '')
    HAVING COUNT(DISTINCT container_id) >= 2
)
SELECT
    fp.original_name,
    fp.norm_name,
    fp.container_id,
    c.col,
    c.row,
    COALESCE(fp.tag_fingerprint, '') AS tag_fingerprint
FROM item_fp fp
JOIN dup_groups dg ON fp.norm_name = dg.norm_name AND COALESCE(fp.tag_fingerprint, '') = dg.tfp
JOIN container c ON c.id = fp.container_id
ORDER BY fp.norm_name, c.col, c.row
`
	rows, err := s.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find duplicates: %w", err)
	}
	defer rows.Close()

	type groupKey struct {
		normName       string
		tagFingerprint string
	}
	groupMap := make(map[groupKey]*model.DuplicateGroup)
	var groupOrder []groupKey

	for rows.Next() {
		var originalName, normName, tagFingerprint string
		var containerID int64
		var col, row int

		if err := rows.Scan(&originalName, &normName, &containerID, &col, &row, &tagFingerprint); err != nil {
			return nil, fmt.Errorf("scan duplicate row: %w", err)
		}

		key := groupKey{normName: normName, tagFingerprint: tagFingerprint}
		g, ok := groupMap[key]
		if !ok {
			var tags []string
			if tagFingerprint != "" {
				tags = strings.Split(tagFingerprint, "|")
			}
			g = &model.DuplicateGroup{
				Name: originalName,
				Tags: tags,
			}
			groupMap[key] = g
			groupOrder = append(groupOrder, key)
		}

		g.Containers = append(g.Containers, model.DuplicateLocation{
			ContainerID: containerID,
			Label:       model.LabelFor(col, row),
		})
		g.Count = len(g.Containers)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate duplicates: %w", err)
	}

	groups := make([]model.DuplicateGroup, 0, len(groupOrder))
	for _, key := range groupOrder {
		groups = append(groups, *groupMap[key])
	}
	return groups, nil
}

// GetOrCreateEncryptionKey returns the 32-byte AES-256 encryption key,
// generating and persisting one if it doesn't exist yet (per D-16).
func (s *Store) GetOrCreateEncryptionKey(ctx context.Context) ([]byte, error) {
	var keyHex string
	err := s.conn.QueryRowContext(ctx, "SELECT encryption_key FROM shelf LIMIT 1").Scan(&keyHex)
	if err != nil {
		return nil, fmt.Errorf("get encryption key: %w", err)
	}
	if keyHex != "" {
		return hex.DecodeString(keyHex)
	}
	// Generate new 32-byte key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate encryption key: %w", err)
	}
	keyHex = hex.EncodeToString(key)
	_, err = s.conn.ExecContext(ctx,
		"UPDATE shelf SET encryption_key = ? WHERE id = (SELECT id FROM shelf LIMIT 1)", keyHex)
	if err != nil {
		return nil, fmt.Errorf("store encryption key: %w", err)
	}
	return key, nil
}
