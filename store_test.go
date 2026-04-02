package main

import (
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
