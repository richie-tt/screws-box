package store

import (
	"context"
	"fmt"
	"screws-box/internal/model"
	"time"
)

// ExportAllData builds a nested JSON-friendly tree of all shelf data.
// No database IDs are included — containers are keyed by position.
func (s *Store) ExportAllData(ctx context.Context) (*model.ExportData, error) {
	// 1. Query shelf.
	var shelfID int64
	var shelfName string
	var rows, cols int
	err := s.conn.QueryRowContext(ctx, "SELECT id, name, rows, cols FROM shelf LIMIT 1").
		Scan(&shelfID, &shelfName, &rows, &cols)
	if err != nil {
		return nil, fmt.Errorf("export: query shelf: %w", err)
	}

	// 2. Query all containers ordered by row, col.
	contRows, err := s.conn.QueryContext(ctx,
		"SELECT id, col, row FROM container WHERE shelf_id = ? ORDER BY row, col", shelfID)
	if err != nil {
		return nil, fmt.Errorf("export: query containers: %w", err)
	}
	defer contRows.Close()

	type containerInfo struct {
		id  int64
		col int
		row int
	}
	var containers []containerInfo
	containerIndex := make(map[int64]int) // container DB ID -> index in slice

	for contRows.Next() {
		var ci containerInfo
		if err := contRows.Scan(&ci.id, &ci.col, &ci.row); err != nil {
			return nil, fmt.Errorf("export: scan container: %w", err)
		}
		containerIndex[ci.id] = len(containers)
		containers = append(containers, ci)
	}
	if err := contRows.Err(); err != nil {
		return nil, fmt.Errorf("export: containers iteration: %w", err)
	}

	// 3. Batch query all items.
	itemRows, err := s.conn.QueryContext(ctx,
		"SELECT id, container_id, name, description FROM item ORDER BY container_id, name")
	if err != nil {
		return nil, fmt.Errorf("export: query items: %w", err)
	}
	defer itemRows.Close()

	type itemInfo struct {
		id          int64
		containerID int64
		name        string
		description *string
	}
	var items []itemInfo
	itemIndex := make(map[int64]int) // item DB ID -> index in slice

	for itemRows.Next() {
		var ii itemInfo
		if err := itemRows.Scan(&ii.id, &ii.containerID, &ii.name, &ii.description); err != nil {
			return nil, fmt.Errorf("export: scan item: %w", err)
		}
		itemIndex[ii.id] = len(items)
		items = append(items, ii)
	}
	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("export: items iteration: %w", err)
	}

	// 4. Batch query all item-tag relationships.
	tagRows, err := s.conn.QueryContext(ctx,
		"SELECT it.item_id, t.name FROM item_tag it JOIN tag t ON t.id = it.tag_id ORDER BY it.item_id, t.name")
	if err != nil {
		return nil, fmt.Errorf("export: query tags: %w", err)
	}
	defer tagRows.Close()

	// Build tag map: item ID -> []string
	tagMap := make(map[int64][]string)
	for tagRows.Next() {
		var itemID int64
		var tagName string
		if err := tagRows.Scan(&itemID, &tagName); err != nil {
			return nil, fmt.Errorf("export: scan tag: %w", err)
		}
		tagMap[itemID] = append(tagMap[itemID], tagName)
	}
	if err := tagRows.Err(); err != nil {
		return nil, fmt.Errorf("export: tags iteration: %w", err)
	}

	// 5. Group items by container ID.
	itemsByContainer := make(map[int64][]model.ExportItem)
	for _, ii := range items {
		tags := tagMap[ii.id]
		if tags == nil {
			tags = []string{}
		}
		itemsByContainer[ii.containerID] = append(itemsByContainer[ii.containerID], model.ExportItem{
			Name:        ii.name,
			Description: ii.description,
			Tags:        tags,
		})
	}

	// 6. Build export containers.
	exportContainers := make([]model.ExportContainer, 0, len(containers))
	for _, ci := range containers {
		items := itemsByContainer[ci.id]
		if items == nil {
			items = []model.ExportItem{}
		}
		exportContainers = append(exportContainers, model.ExportContainer{
			Col:   ci.col,
			Row:   ci.row,
			Label: model.LabelFor(ci.col, ci.row),
			Items: items,
		})
	}

	return &model.ExportData{
		Version:    1,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Shelf: model.ExportShelf{
			Name:       shelfName,
			Rows:       rows,
			Cols:       cols,
			Containers: exportContainers,
		},
	}, nil
}

// ImportAllData replaces all data in a single transaction.
// On any error, the transaction is rolled back and existing data is preserved.
func (s *Store) ImportAllData(ctx context.Context, data *model.ExportData) error {
	if data.Version != 1 {
		return fmt.Errorf("unsupported version %d", data.Version)
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("import: begin tx: %w", err)
	}
	defer deferRollback(tx)

	// Get shelf ID.
	var shelfID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM shelf LIMIT 1").Scan(&shelfID)
	if err != nil {
		return fmt.Errorf("import: get shelf: %w", err)
	}

	// Delete in FK order.
	for _, stmt := range []string{
		"DELETE FROM item_tag",
		"DELETE FROM item",
		"DELETE FROM tag",
		fmt.Sprintf("DELETE FROM container WHERE shelf_id = %d", shelfID),
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("import: %s: %w", stmt, err)
		}
	}

	// Update shelf.
	if _, err := tx.ExecContext(ctx,
		"UPDATE shelf SET name = ?, rows = ?, cols = ? WHERE id = ?",
		data.Shelf.Name, data.Shelf.Rows, data.Shelf.Cols, shelfID); err != nil {
		return fmt.Errorf("import: update shelf: %w", err)
	}

	// Insert containers and their items.
	for _, ec := range data.Shelf.Containers {
		res, err := tx.ExecContext(ctx,
			"INSERT INTO container (shelf_id, col, row) VALUES (?, ?, ?)",
			shelfID, ec.Col, ec.Row)
		if err != nil {
			return fmt.Errorf("import: insert container (%d,%d): %w", ec.Col, ec.Row, err)
		}
		containerID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("import: get container id: %w", err)
		}

		for _, ei := range ec.Items {
			itemRes, err := tx.ExecContext(ctx,
				"INSERT INTO item (container_id, name, description) VALUES (?, ?, ?)",
				containerID, ei.Name, ei.Description)
			if err != nil {
				return fmt.Errorf("import: insert item %q: %w", ei.Name, err)
			}
			itemID, err := itemRes.LastInsertId()
			if err != nil {
				return fmt.Errorf("import: get item id: %w", err)
			}

			for _, tagName := range ei.Tags {
				if _, err := tx.ExecContext(ctx,
					"INSERT OR IGNORE INTO tag (name) VALUES (?)", tagName); err != nil {
					return fmt.Errorf("import: insert tag %q: %w", tagName, err)
				}

				var tagID int64
				if err := tx.QueryRowContext(ctx,
					"SELECT id FROM tag WHERE name = ?", tagName).Scan(&tagID); err != nil {
					return fmt.Errorf("import: get tag id %q: %w", tagName, err)
				}

				if _, err := tx.ExecContext(ctx,
					"INSERT INTO item_tag (item_id, tag_id) VALUES (?, ?)",
					itemID, tagID); err != nil {
					return fmt.Errorf("import: link item-tag: %w", err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("import: commit: %w", err)
	}
	return nil
}

// Ensure *Store satisfies the export/import interface at compile time.
var _ interface {
	ExportAllData(ctx context.Context) (*model.ExportData, error)
	ImportAllData(ctx context.Context, data *model.ExportData) error
} = (*Store)(nil)
