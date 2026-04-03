package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// helper: open store in temp dir, return store + cleanup
func openTestStore(t *testing.T) *Store {
	t.Helper()
	store := &Store{}
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	if err := store.Open(tmpFile); err != nil {
		t.Fatalf("store.Open(%q): %v", tmpFile, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestStoreOpenCreatesFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	store := &Store{}
	if err := store.Open(tmpFile); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Errorf("database file %q was not created", tmpFile)
	}
}

func TestPragmasSet(t *testing.T) {
	store := openTestStore(t)

	var journalMode string
	if err := store.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	var fk int
	if err := store.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}

	var bt int
	if err := store.db.QueryRow("PRAGMA busy_timeout").Scan(&bt); err != nil {
		t.Fatal(err)
	}
	if bt != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", bt)
	}
}

func TestSchemaTablesExist(t *testing.T) {
	store := openTestStore(t)

	tables := []string{"shelf", "container", "item", "tag", "item_tag"}
	for _, table := range tables {
		var name string
		err := store.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestDefaultShelfSeeded(t *testing.T) {
	store := openTestStore(t)

	var name string
	var rows, cols int
	err := store.db.QueryRow("SELECT name, rows, cols FROM shelf").Scan(&name, &rows, &cols)
	if err != nil {
		t.Fatalf("query shelf: %v", err)
	}
	if name != "My Organizer" {
		t.Errorf("shelf name = %q, want %q", name, "My Organizer")
	}
	if rows != 5 {
		t.Errorf("shelf rows = %d, want 5", rows)
	}
	if cols != 10 {
		t.Errorf("shelf cols = %d, want 10", cols)
	}

	var containerCount int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM container").Scan(&containerCount); err != nil {
		t.Fatal(err)
	}
	if containerCount != 50 {
		t.Errorf("container count = %d, want 50", containerCount)
	}
}

func TestSeedIdempotent(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")

	// Open once
	store1 := &Store{}
	if err := store1.Open(tmpFile); err != nil {
		t.Fatalf("first Open: %v", err)
	}
	store1.Close()

	// Open again on same file
	store2 := &Store{}
	if err := store2.Open(tmpFile); err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer store2.Close()

	var shelfCount int
	if err := store2.db.QueryRow("SELECT COUNT(*) FROM shelf").Scan(&shelfCount); err != nil {
		t.Fatal(err)
	}
	if shelfCount != 1 {
		t.Errorf("shelf count after two Opens = %d, want 1", shelfCount)
	}

	var containerCount int
	if err := store2.db.QueryRow("SELECT COUNT(*) FROM container").Scan(&containerCount); err != nil {
		t.Fatal(err)
	}
	if containerCount != 50 {
		t.Errorf("container count after two Opens = %d, want 50", containerCount)
	}
}

func TestGetGridData(t *testing.T) {
	store := openTestStore(t)

	data, err := store.GetGridData()
	if err != nil {
		t.Fatalf("GetGridData() error: %v", err)
	}

	if data.ShelfName != "My Organizer" {
		t.Errorf("ShelfName = %q, want %q", data.ShelfName, "My Organizer")
	}
	if data.Rows != 5 {
		t.Errorf("Rows = %d, want 5", data.Rows)
	}
	if data.Cols != 10 {
		t.Errorf("Cols = %d, want 10", data.Cols)
	}
	if len(data.Grid) != 5 {
		t.Fatalf("len(Grid) = %d, want 5", len(data.Grid))
	}
	if len(data.Grid[0].Cells) != 10 {
		t.Fatalf("len(Grid[0].Cells) = %d, want 10", len(data.Grid[0].Cells))
	}

	// Row letters
	if data.Grid[0].Letter != "A" {
		t.Errorf("Grid[0].Letter = %q, want %q", data.Grid[0].Letter, "A")
	}
	if data.Grid[4].Letter != "E" {
		t.Errorf("Grid[4].Letter = %q, want %q", data.Grid[4].Letter, "E")
	}

	// Coordinate labels
	if data.Grid[0].Cells[0].Coord != "1A" {
		t.Errorf("Grid[0].Cells[0].Coord = %q, want %q", data.Grid[0].Cells[0].Coord, "1A")
	}
	if data.Grid[0].Cells[9].Coord != "10A" {
		t.Errorf("Grid[0].Cells[9].Coord = %q, want %q", data.Grid[0].Cells[9].Coord, "10A")
	}
	if data.Grid[1].Cells[2].Coord != "3B" {
		t.Errorf("Grid[1].Cells[2].Coord = %q, want %q", data.Grid[1].Cells[2].Coord, "3B")
	}

	// All cells empty (no items seeded)
	for ri, row := range data.Grid {
		for ci, cell := range row.Cells {
			if !cell.IsEmpty {
				t.Errorf("Grid[%d].Cells[%d].IsEmpty = false, want true", ri, ci)
			}
			if cell.Count != 0 {
				t.Errorf("Grid[%d].Cells[%d].Count = %d, want 0", ri, ci, cell.Count)
			}
		}
	}

	// Chessboard CSS class alternation
	if data.Grid[0].Cells[0].CSSClass != "cell-light" {
		t.Errorf("Grid[0].Cells[0].CSSClass = %q, want %q", data.Grid[0].Cells[0].CSSClass, "cell-light")
	}
	if data.Grid[0].Cells[1].CSSClass != "cell-dark" {
		t.Errorf("Grid[0].Cells[1].CSSClass = %q, want %q", data.Grid[0].Cells[1].CSSClass, "cell-dark")
	}
}

func TestGetGridDataItemCounts(t *testing.T) {
	store := openTestStore(t)

	// Find container at col=3, row=2 (coord "3B")
	var containerID int64
	err := store.db.QueryRow("SELECT id FROM container WHERE col = 3 AND row = 2").Scan(&containerID)
	if err != nil {
		t.Fatalf("find container: %v", err)
	}

	// Insert 3 items
	for i := 0; i < 3; i++ {
		_, err := store.db.Exec("INSERT INTO item (container_id, name) VALUES (?, ?)",
			containerID, fmt.Sprintf("Item %d", i+1))
		if err != nil {
			t.Fatalf("insert item %d: %v", i+1, err)
		}
	}

	data, err := store.GetGridData()
	if err != nil {
		t.Fatalf("GetGridData() error: %v", err)
	}

	// Grid[1] = row B, Cells[2] = col 3 -> coord "3B"
	cell := data.Grid[1].Cells[2]
	if cell.Count != 3 {
		t.Errorf("cell 3B Count = %d, want 3", cell.Count)
	}
	if cell.IsEmpty {
		t.Errorf("cell 3B IsEmpty = true, want false")
	}
}

func TestGetGridDataContainerIDs(t *testing.T) {
	store := openTestStore(t)

	data, err := store.GetGridData()
	if err != nil {
		t.Fatalf("GetGridData() error: %v", err)
	}

	// Every cell must have a non-zero ContainerID.
	seen := make(map[int64]bool)
	for ri, row := range data.Grid {
		for ci, cell := range row.Cells {
			if cell.ContainerID <= 0 {
				t.Errorf("Grid[%d].Cells[%d].ContainerID = %d, want > 0", ri, ci, cell.ContainerID)
			}
			seen[cell.ContainerID] = true
		}
	}

	// All 50 ContainerIDs must be unique.
	if len(seen) != 50 {
		t.Errorf("unique ContainerIDs = %d, want 50", len(seen))
	}

	// Verify ContainerID matches the actual DB record for a specific cell.
	cell3B := data.Grid[1].Cells[2] // row B (index 1), col 3 (index 2)
	var dbID int64
	err = store.db.QueryRow("SELECT id FROM container WHERE col = 3 AND row = 2").Scan(&dbID)
	if err != nil {
		t.Fatalf("query container: %v", err)
	}
	if cell3B.ContainerID != dbID {
		t.Errorf("cell 3B ContainerID = %d, want DB id %d", cell3B.ContainerID, dbID)
	}
}

func TestGetGridDataCustomDimensions(t *testing.T) {
	store := openTestStore(t)

	// Update shelf to 2 rows, 3 cols
	if _, err := store.db.Exec("UPDATE shelf SET rows = 2, cols = 3"); err != nil {
		t.Fatalf("update shelf: %v", err)
	}
	// Delete old containers
	if _, err := store.db.Exec("DELETE FROM container"); err != nil {
		t.Fatalf("delete containers: %v", err)
	}
	// Insert 6 new containers (3 cols x 2 rows)
	var shelfID int64
	store.db.QueryRow("SELECT id FROM shelf LIMIT 1").Scan(&shelfID)
	for col := 1; col <= 3; col++ {
		for row := 1; row <= 2; row++ {
			if _, err := store.db.Exec(
				"INSERT INTO container (shelf_id, col, row) VALUES (?, ?, ?)",
				shelfID, col, row,
			); err != nil {
				t.Fatalf("insert container (%d,%d): %v", col, row, err)
			}
		}
	}

	data, err := store.GetGridData()
	if err != nil {
		t.Fatalf("GetGridData() error: %v", err)
	}

	if data.Rows != 2 {
		t.Errorf("Rows = %d, want 2", data.Rows)
	}
	if data.Cols != 3 {
		t.Errorf("Cols = %d, want 3", data.Cols)
	}
	if len(data.Grid) != 2 {
		t.Fatalf("len(Grid) = %d, want 2", len(data.Grid))
	}
	if len(data.Grid[0].Cells) != 3 {
		t.Fatalf("len(Grid[0].Cells) = %d, want 3", len(data.Grid[0].Cells))
	}
}

func TestCascadeDeleteContainerRemovesItems(t *testing.T) {
	store := openTestStore(t)

	// Get first container
	var containerID int64
	if err := store.db.QueryRow("SELECT id FROM container LIMIT 1").Scan(&containerID); err != nil {
		t.Fatal(err)
	}

	// Insert an item in that container
	_, err := store.db.Exec(
		"INSERT INTO item (container_id, name) VALUES (?, ?)",
		containerID, "Test Screw",
	)
	if err != nil {
		t.Fatalf("insert item: %v", err)
	}

	// Verify item exists
	var itemCount int
	store.db.QueryRow("SELECT COUNT(*) FROM item WHERE container_id = ?", containerID).Scan(&itemCount)
	if itemCount != 1 {
		t.Fatalf("expected 1 item before delete, got %d", itemCount)
	}

	// Delete the container -- CASCADE should remove the item
	if _, err := store.db.Exec("DELETE FROM container WHERE id = ?", containerID); err != nil {
		t.Fatalf("delete container: %v", err)
	}

	// Verify item was cascaded
	store.db.QueryRow("SELECT COUNT(*) FROM item WHERE container_id = ?", containerID).Scan(&itemCount)
	if itemCount != 0 {
		t.Errorf("expected 0 items after cascade delete, got %d", itemCount)
	}
}

func TestCascadeDeleteItemRemovesItemTags(t *testing.T) {
	store := openTestStore(t)

	// Get first container
	var containerID int64
	store.db.QueryRow("SELECT id FROM container LIMIT 1").Scan(&containerID)

	// Insert a tag
	res, err := store.db.Exec("INSERT INTO tag (name) VALUES (?)", "m6")
	if err != nil {
		t.Fatalf("insert tag: %v", err)
	}
	tagID, _ := res.LastInsertId()

	// Insert an item
	res, err = store.db.Exec(
		"INSERT INTO item (container_id, name) VALUES (?, ?)",
		containerID, "M6 Bolt",
	)
	if err != nil {
		t.Fatalf("insert item: %v", err)
	}
	itemID, _ := res.LastInsertId()

	// Create item_tag association
	if _, err := store.db.Exec("INSERT INTO item_tag (item_id, tag_id) VALUES (?, ?)", itemID, tagID); err != nil {
		t.Fatalf("insert item_tag: %v", err)
	}

	// Verify item_tag exists
	var linkCount int
	store.db.QueryRow("SELECT COUNT(*) FROM item_tag WHERE item_id = ?", itemID).Scan(&linkCount)
	if linkCount != 1 {
		t.Fatalf("expected 1 item_tag before delete, got %d", linkCount)
	}

	// Delete the item -- CASCADE should remove item_tag
	if _, err := store.db.Exec("DELETE FROM item WHERE id = ?", itemID); err != nil {
		t.Fatalf("delete item: %v", err)
	}

	// Verify item_tag was cascaded
	store.db.QueryRow("SELECT COUNT(*) FROM item_tag WHERE item_id = ?", itemID).Scan(&linkCount)
	if linkCount != 0 {
		t.Errorf("expected 0 item_tag rows after cascade delete, got %d", linkCount)
	}
}

// --- Test helpers for item CRUD ---

func getTestContainerID(t *testing.T, store *Store) int64 {
	t.Helper()
	var id int64
	err := store.db.QueryRow("SELECT id FROM container LIMIT 1").Scan(&id)
	if err != nil {
		t.Fatalf("get test container: %v", err)
	}
	return id
}

func getSecondContainerID(t *testing.T, store *Store, firstID int64) int64 {
	t.Helper()
	var id int64
	err := store.db.QueryRow("SELECT id FROM container WHERE id != ? LIMIT 1", firstID).Scan(&id)
	if err != nil {
		t.Fatalf("get second container: %v", err)
	}
	return id
}

// --- Item CRUD tests ---

func TestCreateItemWithTags(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	desc := "DIN 933"
	item, err := store.CreateItem(ctx, containerID, "M6 bolt", &desc, []string{"m6", "bolt"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	if item.ID <= 0 {
		t.Errorf("ID = %d, want > 0", item.ID)
	}
	if item.Name != "M6 bolt" {
		t.Errorf("Name = %q, want %q", item.Name, "M6 bolt")
	}
	if item.Description == nil || *item.Description != "DIN 933" {
		t.Errorf("Description = %v, want ptr to %q", item.Description, "DIN 933")
	}
	if len(item.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(item.Tags))
	}
	// Tags are sorted alphabetically by GetItem
	if len(item.Tags) >= 2 {
		if item.Tags[0] != "bolt" || item.Tags[1] != "m6" {
			t.Errorf("Tags = %v, want [bolt m6]", item.Tags)
		}
	}
	labelPattern := regexp.MustCompile(`^\d+[A-Z]$`)
	if !labelPattern.MatchString(item.ContainerLabel) {
		t.Errorf("ContainerLabel = %q, doesn't match pattern \\d+[A-Z]", item.ContainerLabel)
	}
	if item.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}
	if item.UpdatedAt == "" {
		t.Error("UpdatedAt is empty")
	}
}

func TestCreateItemContainerNotFound(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	_, err := store.CreateItem(ctx, 99999, "Test", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "container not found" {
		t.Errorf("error = %q, want %q", got, "container not found")
	}
}

func TestCreateItemDuplicateTagsDeduped(t *testing.T) {
	// Test the dedup function directly.
	result := dedup([]string{"m6", "m6", "bolt"})
	if len(result) != 2 {
		t.Errorf("dedup len = %d, want 2", len(result))
	}

	// Test CreateItem with already-deduped tags works.
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	dedupedTags := dedup([]string{"m6", "m6", "bolt"})
	item, err := store.CreateItem(ctx, containerID, "Dedup test", nil, dedupedTags)
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if len(item.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(item.Tags))
	}
}

func TestGetItem(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	desc := "test description"
	created, err := store.CreateItem(ctx, containerID, "Get test", &desc, []string{"alpha", "beta"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	item, err := store.GetItem(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if item == nil {
		t.Fatal("GetItem returned nil")
	}
	if item.ID != created.ID {
		t.Errorf("ID = %d, want %d", item.ID, created.ID)
	}
	if item.Name != "Get test" {
		t.Errorf("Name = %q, want %q", item.Name, "Get test")
	}
	if item.Description == nil || *item.Description != "test description" {
		t.Errorf("Description mismatch")
	}
	if len(item.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(item.Tags))
	}
	if item.ContainerLabel == "" {
		t.Error("ContainerLabel is empty")
	}
}

func TestGetItemNotFound(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	item, err := store.GetItem(ctx, 99999)
	if err != nil {
		t.Fatalf("GetItem error: %v", err)
	}
	if item != nil {
		t.Errorf("expected nil, got %+v", item)
	}
}

func TestUpdateItem(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	oldDesc := "old desc"
	created, err := store.CreateItem(ctx, containerID, "old", &oldDesc, []string{"tag1"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	newDesc := "new desc"
	updated, err := store.UpdateItem(ctx, created.ID, "new", &newDesc, containerID)
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if updated == nil {
		t.Fatal("UpdateItem returned nil")
	}
	if updated.Name != "new" {
		t.Errorf("Name = %q, want %q", updated.Name, "new")
	}
	if updated.Description == nil || *updated.Description != "new desc" {
		t.Errorf("Description mismatch")
	}
	// Tags unchanged per D-18.
	if len(updated.Tags) != 1 || updated.Tags[0] != "tag1" {
		t.Errorf("Tags = %v, want [tag1]", updated.Tags)
	}
}

func TestUpdateItemMoveContainer(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	containerA := getTestContainerID(t, store)
	containerB := getSecondContainerID(t, store, containerA)

	created, err := store.CreateItem(ctx, containerA, "Movable", nil, nil)
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	labelA := created.ContainerLabel

	moved, err := store.UpdateItem(ctx, created.ID, "Movable", nil, containerB)
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if moved.ContainerLabel == labelA {
		t.Errorf("ContainerLabel unchanged after move: %q", moved.ContainerLabel)
	}
}

func TestUpdateItemNotFound(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	item, err := store.UpdateItem(ctx, 99999, "test", nil, containerID)
	if err != nil {
		t.Fatalf("UpdateItem error: %v", err)
	}
	if item != nil {
		t.Errorf("expected nil, got %+v", item)
	}
}

func TestDeleteItem(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	created, err := store.CreateItem(ctx, containerID, "To delete", nil, []string{"temp"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	if err := store.DeleteItem(ctx, created.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	item, err := store.GetItem(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetItem after delete: %v", err)
	}
	if item != nil {
		t.Errorf("expected nil after delete, got %+v", item)
	}
}

func TestDeleteItemContainerPersists(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	created, err := store.CreateItem(ctx, containerID, "To delete", nil, nil)
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	if err := store.DeleteItem(ctx, created.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM container WHERE id = ?", containerID).Scan(&count)
	if err != nil {
		t.Fatalf("query container: %v", err)
	}
	if count != 1 {
		t.Errorf("container count = %d, want 1", count)
	}
}

func TestDeleteItemNotFound(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	err := store.DeleteItem(ctx, 99999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "item not found" {
		t.Errorf("error = %q, want %q", got, "item not found")
	}
}

func TestMultipleItemsPerContainer(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	_, err := store.CreateItem(ctx, containerID, "Item A", nil, nil)
	if err != nil {
		t.Fatalf("CreateItem A: %v", err)
	}
	_, err = store.CreateItem(ctx, containerID, "Item B", nil, nil)
	if err != nil {
		t.Fatalf("CreateItem B: %v", err)
	}

	result, err := store.ListItemsByContainer(ctx, containerID)
	if err != nil {
		t.Fatalf("ListItemsByContainer: %v", err)
	}
	if result == nil {
		t.Fatal("ListItemsByContainer returned nil")
	}
	if len(result.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(result.Items))
	}
}

func TestAddTagToItem(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	created, err := store.CreateItem(ctx, containerID, "Tag test", nil, []string{"initial"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	updated, err := store.AddTagToItem(ctx, created.ID, "added")
	if err != nil {
		t.Fatalf("AddTagToItem: %v", err)
	}
	if len(updated.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(updated.Tags))
	}
}

func TestRemoveTagFromItem(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	created, err := store.CreateItem(ctx, containerID, "Remove tag test", nil, []string{"keep", "remove"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	err = store.RemoveTagFromItem(ctx, created.ID, "remove")
	if err != nil {
		t.Fatalf("RemoveTagFromItem: %v", err)
	}

	item, err := store.GetItem(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if len(item.Tags) != 1 {
		t.Errorf("len(Tags) = %d, want 1", len(item.Tags))
	}
	if item.Tags[0] != "keep" {
		t.Errorf("Tags[0] = %q, want %q", item.Tags[0], "keep")
	}

	// Per D-15: tag still exists in tag table.
	var tagCount int
	err = store.db.QueryRow("SELECT COUNT(*) FROM tag WHERE name = ?", "remove").Scan(&tagCount)
	if err != nil {
		t.Fatalf("query tag: %v", err)
	}
	if tagCount != 1 {
		t.Errorf("tag 'remove' count = %d, want 1 (orphaned tags kept)", tagCount)
	}
}

func TestListItemsByContainer(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	_, err := store.CreateItem(ctx, containerID, "List A", nil, []string{"x"})
	if err != nil {
		t.Fatalf("CreateItem A: %v", err)
	}
	_, err = store.CreateItem(ctx, containerID, "List B", nil, []string{"y"})
	if err != nil {
		t.Fatalf("CreateItem B: %v", err)
	}

	result, err := store.ListItemsByContainer(ctx, containerID)
	if err != nil {
		t.Fatalf("ListItemsByContainer: %v", err)
	}
	if result == nil {
		t.Fatal("returned nil")
	}

	labelPattern := regexp.MustCompile(`^\d+[A-Z]$`)
	if !labelPattern.MatchString(result.Label) {
		t.Errorf("Label = %q, doesn't match pattern", result.Label)
	}
	if len(result.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(result.Items))
	}
}

func TestListItemsByContainerNotFound(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	result, err := store.ListItemsByContainer(ctx, 99999)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestListAllItems(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	containerA := getTestContainerID(t, store)
	containerB := getSecondContainerID(t, store, containerA)

	_, err := store.CreateItem(ctx, containerA, "All A", nil, nil)
	if err != nil {
		t.Fatalf("CreateItem A: %v", err)
	}
	_, err = store.CreateItem(ctx, containerB, "All B", nil, nil)
	if err != nil {
		t.Fatalf("CreateItem B: %v", err)
	}

	items, err := store.ListAllItems(ctx)
	if err != nil {
		t.Fatalf("ListAllItems: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}

func TestListTags(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	_, err := store.CreateItem(ctx, containerID, "Tags test", nil, []string{"alpha", "beta"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	tags, err := store.ListTags(ctx, "")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) < 2 {
		t.Errorf("len(tags) = %d, want >= 2", len(tags))
	}
	// Verify sorted alphabetically.
	if len(tags) >= 2 {
		if tags[0].Name > tags[1].Name {
			t.Errorf("tags not sorted: %q > %q", tags[0].Name, tags[1].Name)
		}
	}
}

func TestListTagsWithPrefix(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	_, err := store.CreateItem(ctx, containerID, "Prefix test", nil, []string{"m6", "m8", "bolt"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	tags, err := store.ListTags(ctx, "m")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("len(tags) = %d, want 2", len(tags))
	}
	for _, tag := range tags {
		if tag.Name != "m6" && tag.Name != "m8" {
			t.Errorf("unexpected tag %q with prefix 'm'", tag.Name)
		}
	}
}

func TestTagsInJunctionTable(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, store)

	item, err := store.CreateItem(ctx, containerID, "Junction test", nil, []string{"tag1", "tag2"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM item_tag WHERE item_id = ?", item.ID).Scan(&count)
	if err != nil {
		t.Fatalf("query item_tag: %v", err)
	}
	if count != 2 {
		t.Errorf("item_tag count = %d, want 2", count)
	}
}
