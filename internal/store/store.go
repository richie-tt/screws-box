package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"screws-box/internal/model"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite" // register SQLite driver
)

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
}

// migrations runs ALTER TABLE statements for columns added after the initial schema.
// Each is idempotent: errors from "duplicate column" are silently ignored.
var migrations = []string{
	`ALTER TABLE shelf ADD COLUMN auth_enabled INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE shelf ADD COLUMN auth_user TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE shelf ADD COLUMN auth_pass TEXT NOT NULL DEFAULT ''`,
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
func (s *Store) ListTags(ctx context.Context, prefix string) ([]model.TagResponse, error) {
	var rows *sql.Rows
	var err error

	if prefix == "" {
		rows, err = s.conn.QueryContext(ctx, "SELECT id, name, created_at, updated_at FROM tag ORDER BY name")
	} else {
		rows, err = s.conn.QueryContext(ctx, "SELECT id, name, created_at, updated_at FROM tag WHERE name LIKE ? ORDER BY name", prefix+"%")
	}
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	tags := []model.TagResponse{}
	for rows.Next() {
		var t model.TagResponse
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.ID, &t.Name, &createdAt, &updatedAt); err != nil {
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
