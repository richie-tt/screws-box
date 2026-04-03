package main

import (
	"fmt"
	"os"
	"path/filepath"
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
