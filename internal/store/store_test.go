package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"screws-box/internal/model"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s := &Store{}
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, s.Open(tmpFile), "store.Open(%q)", tmpFile)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStoreOpenCreatesFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	s := &Store{}
	require.NoError(t, s.Open(tmpFile))
	defer s.Close()

	_, err := os.Stat(tmpFile)
	assert.False(t, os.IsNotExist(err), "database file %q was not created", tmpFile)
}

func TestPragmasSet(t *testing.T) {
	s := openTestStore(t)

	var journalMode string
	require.NoError(t, s.conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode))
	assert.Equal(t, "wal", journalMode)

	var fk int
	require.NoError(t, s.conn.QueryRow("PRAGMA foreign_keys").Scan(&fk))
	assert.Equal(t, 1, fk)

	var bt int
	require.NoError(t, s.conn.QueryRow("PRAGMA busy_timeout").Scan(&bt))
	assert.Equal(t, 5000, bt)
}

func TestSchemaTablesExist(t *testing.T) {
	s := openTestStore(t)

	tables := []string{"shelf", "container", "item", "tag", "item_tag"}
	for _, table := range tables {
		var name string
		err := s.conn.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		assert.NoError(t, err, "table %q not found", table)
	}
}

func TestDefaultShelfSeeded(t *testing.T) {
	s := openTestStore(t)

	var name string
	var rows, cols int
	err := s.conn.QueryRow("SELECT name, rows, cols FROM shelf").Scan(&name, &rows, &cols)
	require.NoError(t, err, "query shelf")
	assert.Equal(t, "My Organizer", name)
	assert.Equal(t, 5, rows)
	assert.Equal(t, 10, cols)

	var containerCount int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM container").Scan(&containerCount))
	assert.Equal(t, 50, containerCount)
}

func TestSeedIdempotent(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")

	s1 := &Store{}
	require.NoError(t, s1.Open(tmpFile), "first Open")
	s1.Close()

	s2 := &Store{}
	require.NoError(t, s2.Open(tmpFile), "second Open")
	defer s2.Close()

	var shelfCount int
	require.NoError(t, s2.conn.QueryRow("SELECT COUNT(*) FROM shelf").Scan(&shelfCount))
	assert.Equal(t, 1, shelfCount)

	var containerCount int
	require.NoError(t, s2.conn.QueryRow("SELECT COUNT(*) FROM container").Scan(&containerCount))
	assert.Equal(t, 50, containerCount)
}

func TestGetGridData(t *testing.T) {
	s := openTestStore(t)

	data, err := s.GetGridData()
	require.NoError(t, err)

	assert.Equal(t, "My Organizer", data.ShelfName)
	assert.Equal(t, 5, data.Rows)
	assert.Equal(t, 10, data.Cols)
	require.Len(t, data.Grid, 5)
	require.Len(t, data.Grid[0].Cells, 10)

	assert.Equal(t, "A", data.Grid[0].Letter)
	assert.Equal(t, "E", data.Grid[4].Letter)

	assert.Equal(t, "1A", data.Grid[0].Cells[0].Coord)
	assert.Equal(t, "10A", data.Grid[0].Cells[9].Coord)
	assert.Equal(t, "3B", data.Grid[1].Cells[2].Coord)

	for ri, row := range data.Grid {
		for ci, cell := range row.Cells {
			assert.True(t, cell.IsEmpty, "Grid[%d].Cells[%d] not empty", ri, ci)
			assert.Equal(t, 0, cell.Count, "Grid[%d].Cells[%d].Count", ri, ci)
		}
	}

	assert.Equal(t, "cell-light", data.Grid[0].Cells[0].CSSClass)
	assert.Equal(t, "cell-dark", data.Grid[0].Cells[1].CSSClass)
}

func TestGetGridDataItemCounts(t *testing.T) {
	s := openTestStore(t)

	var containerID int64
	err := s.conn.QueryRow("SELECT id FROM container WHERE col = 3 AND row = 2").Scan(&containerID)
	require.NoError(t, err, "find container")

	for i := range 3 {
		_, err := s.conn.Exec("INSERT INTO item (container_id, name) VALUES (?, ?)",
			containerID, fmt.Sprintf("Item %d", i+1))
		require.NoError(t, err, "insert item %d", i+1)
	}

	data, err := s.GetGridData()
	require.NoError(t, err)

	cell := data.Grid[1].Cells[2]
	assert.Equal(t, 3, cell.Count)
	assert.False(t, cell.IsEmpty)
}

func TestGetGridDataContainerIDs(t *testing.T) {
	s := openTestStore(t)

	data, err := s.GetGridData()
	require.NoError(t, err)

	seen := make(map[int64]bool)
	for ri, row := range data.Grid {
		for ci, cell := range row.Cells {
			assert.Positive(t, cell.ContainerID, "Grid[%d].Cells[%d].ContainerID", ri, ci)
			seen[cell.ContainerID] = true
		}
	}

	assert.Len(t, seen, 50)

	cell3B := data.Grid[1].Cells[2]
	var dbID int64
	err = s.conn.QueryRow("SELECT id FROM container WHERE col = 3 AND row = 2").Scan(&dbID)
	require.NoError(t, err, "query container")
	assert.Equal(t, dbID, cell3B.ContainerID)
}

func TestGetGridDataCustomDimensions(t *testing.T) {
	s := openTestStore(t)

	_, err := s.conn.Exec("UPDATE shelf SET rows = 2, cols = 3")
	require.NoError(t, err, "update shelf")
	_, err = s.conn.Exec("DELETE FROM container")
	require.NoError(t, err, "delete containers")

	var shelfID int64
	require.NoError(t, s.conn.QueryRow("SELECT id FROM shelf LIMIT 1").Scan(&shelfID))
	for col := 1; col <= 3; col++ {
		for row := 1; row <= 2; row++ {
			_, err := s.conn.Exec(
				"INSERT INTO container (shelf_id, col, row) VALUES (?, ?, ?)",
				shelfID, col, row,
			)
			require.NoError(t, err, "insert container (%d,%d)", col, row)
		}
	}

	data, err := s.GetGridData()
	require.NoError(t, err)

	assert.Equal(t, 2, data.Rows)
	assert.Equal(t, 3, data.Cols)
	require.Len(t, data.Grid, 2)
	require.Len(t, data.Grid[0].Cells, 3)
}

func TestCascadeDeleteContainerRemovesItems(t *testing.T) {
	s := openTestStore(t)

	var containerID int64
	require.NoError(t, s.conn.QueryRow("SELECT id FROM container LIMIT 1").Scan(&containerID))

	_, err := s.conn.Exec(
		"INSERT INTO item (container_id, name) VALUES (?, ?)",
		containerID, "Test Screw",
	)
	require.NoError(t, err, "insert item")

	var itemCount int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM item WHERE container_id = ?", containerID).Scan(&itemCount))
	require.Equal(t, 1, itemCount, "expected 1 item before delete")

	_, err = s.conn.Exec("DELETE FROM container WHERE id = ?", containerID)
	require.NoError(t, err, "delete container")

	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM item WHERE container_id = ?", containerID).Scan(&itemCount))
	assert.Equal(t, 0, itemCount)
}

func TestCascadeDeleteItemRemovesItemTags(t *testing.T) {
	s := openTestStore(t)

	var containerID int64
	require.NoError(t, s.conn.QueryRow("SELECT id FROM container LIMIT 1").Scan(&containerID))

	res, err := s.conn.Exec("INSERT INTO tag (name) VALUES (?)", "m6")
	require.NoError(t, err, "insert tag")
	tagID, _ := res.LastInsertId()

	res, err = s.conn.Exec(
		"INSERT INTO item (container_id, name) VALUES (?, ?)",
		containerID, "M6 Bolt",
	)
	require.NoError(t, err, "insert item")
	itemID, _ := res.LastInsertId()

	_, err = s.conn.Exec("INSERT INTO item_tag (item_id, tag_id) VALUES (?, ?)", itemID, tagID)
	require.NoError(t, err, "insert item_tag")

	var linkCount int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM item_tag WHERE item_id = ?", itemID).Scan(&linkCount))
	require.Equal(t, 1, linkCount, "expected 1 item_tag before delete")

	_, err = s.conn.Exec("DELETE FROM item WHERE id = ?", itemID)
	require.NoError(t, err, "delete item")

	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM item_tag WHERE item_id = ?", itemID).Scan(&linkCount))
	assert.Equal(t, 0, linkCount)
}

// --- Test helpers for item CRUD ---

func getTestContainerID(t *testing.T, s *Store) int64 {
	t.Helper()
	var id int64
	err := s.conn.QueryRow("SELECT id FROM container LIMIT 1").Scan(&id)
	require.NoError(t, err, "get test container")
	return id
}

func getSecondContainerID(t *testing.T, s *Store, firstID int64) int64 {
	t.Helper()
	var id int64
	err := s.conn.QueryRow("SELECT id FROM container WHERE id != ? LIMIT 1", firstID).Scan(&id)
	require.NoError(t, err, "get second container")
	return id
}

func getContainerIDByPos(t *testing.T, s *Store, col, row int) int64 {
	t.Helper()
	var id int64
	err := s.conn.QueryRow("SELECT id FROM container WHERE col = ? AND row = ?", col, row).Scan(&id)
	require.NoError(t, err, "get container at (%d,%d)", col, row)
	return id
}

func insertItemAt(t *testing.T, s *Store, col, row int, name string) {
	t.Helper()
	containerID := getContainerIDByPos(t, s, col, row)
	ctx := context.Background()
	_, err := s.CreateItem(ctx, containerID, name, nil, []string{"test"})
	require.NoError(t, err, "create item %q in container (%d,%d)", name, col, row)
}

// --- Item CRUD tests ---

func TestCreateItemWithTags(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	desc := "DIN 933"
	item, err := s.CreateItem(ctx, containerID, "M6 bolt", &desc, []string{"m6", "bolt"})
	require.NoError(t, err)

	assert.Positive(t, item.ID)
	assert.Equal(t, "M6 bolt", item.Name)
	require.NotNil(t, item.Description)
	assert.Equal(t, "DIN 933", *item.Description)
	assert.Len(t, item.Tags, 2)
	if len(item.Tags) >= 2 {
		assert.Equal(t, "bolt", item.Tags[0])
		assert.Equal(t, "m6", item.Tags[1])
	}
	assert.Regexp(t, `^\d+[A-Z]$`, item.ContainerLabel)
	assert.NotEmpty(t, item.CreatedAt)
	assert.NotEmpty(t, item.UpdatedAt)
}

func TestCreateItemContainerNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.CreateItem(ctx, 99999, "Test", nil, nil)
	require.Error(t, err)
	assert.Equal(t, "container not found", err.Error())
}

func TestCreateItemDuplicateTagsDeduped(t *testing.T) {
	result := model.Dedup([]string{"m6", "m6", "bolt"})
	assert.Len(t, result, 2)

	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	dedupedTags := model.Dedup([]string{"m6", "m6", "bolt"})
	item, err := s.CreateItem(ctx, containerID, "Dedup test", nil, dedupedTags)
	require.NoError(t, err)
	assert.Len(t, item.Tags, 2)
}

func TestGetItem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	desc := "test description"
	created, err := s.CreateItem(ctx, containerID, "Get test", &desc, []string{"alpha", "beta"})
	require.NoError(t, err)

	item, err := s.GetItem(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, created.ID, item.ID)
	assert.Equal(t, "Get test", item.Name)
	require.NotNil(t, item.Description)
	assert.Equal(t, "test description", *item.Description)
	assert.Len(t, item.Tags, 2)
	assert.NotEmpty(t, item.ContainerLabel)
}

func TestGetItemNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	item, err := s.GetItem(ctx, 99999)
	require.NoError(t, err)
	assert.Nil(t, item)
}

func TestUpdateItem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	oldDesc := "old desc"
	created, err := s.CreateItem(ctx, containerID, "old", &oldDesc, []string{"tag1"})
	require.NoError(t, err)

	newDesc := "new desc"
	updated, err := s.UpdateItem(ctx, created.ID, "new", &newDesc, containerID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "new", updated.Name)
	require.NotNil(t, updated.Description)
	assert.Equal(t, "new desc", *updated.Description)
	assert.Equal(t, []string{"tag1"}, updated.Tags)
}

func TestUpdateItemMoveContainer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	containerA := getTestContainerID(t, s)
	containerB := getSecondContainerID(t, s, containerA)

	created, err := s.CreateItem(ctx, containerA, "Movable", nil, nil)
	require.NoError(t, err)
	labelA := created.ContainerLabel

	moved, err := s.UpdateItem(ctx, created.ID, "Movable", nil, containerB)
	require.NoError(t, err)
	assert.NotEqual(t, labelA, moved.ContainerLabel)
}

func TestUpdateItemNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	item, err := s.UpdateItem(ctx, 99999, "test", nil, containerID)
	require.NoError(t, err)
	assert.Nil(t, item)
}

func TestDeleteItem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	created, err := s.CreateItem(ctx, containerID, "To delete", nil, []string{"temp"})
	require.NoError(t, err)

	require.NoError(t, s.DeleteItem(ctx, created.ID))

	item, err := s.GetItem(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, item)
}

func TestDeleteItemContainerPersists(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	created, err := s.CreateItem(ctx, containerID, "To delete", nil, nil)
	require.NoError(t, err)

	require.NoError(t, s.DeleteItem(ctx, created.ID))

	var count int
	err = s.conn.QueryRow("SELECT COUNT(*) FROM container WHERE id = ?", containerID).Scan(&count)
	require.NoError(t, err, "query container")
	assert.Equal(t, 1, count)
}

func TestDeleteItemNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.DeleteItem(ctx, 99999)
	require.Error(t, err)
	assert.Equal(t, "item not found", err.Error())
}

func TestMultipleItemsPerContainer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, containerID, "Item A", nil, nil)
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, containerID, "Item B", nil, nil)
	require.NoError(t, err)

	result, err := s.ListItemsByContainer(ctx, containerID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Items, 2)
}

func TestAddTagToItem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	created, err := s.CreateItem(ctx, containerID, "Tag test", nil, []string{"initial"})
	require.NoError(t, err)

	updated, err := s.AddTagToItem(ctx, created.ID, "added")
	require.NoError(t, err)
	assert.Len(t, updated.Tags, 2)
}

func TestRemoveTagFromItem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	created, err := s.CreateItem(ctx, containerID, "Remove tag test", nil, []string{"keep", "remove"})
	require.NoError(t, err)

	require.NoError(t, s.RemoveTagFromItem(ctx, created.ID, "remove"))

	item, err := s.GetItem(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"keep"}, item.Tags)

	var tagCount int
	err = s.conn.QueryRow("SELECT COUNT(*) FROM tag WHERE name = ?", "remove").Scan(&tagCount)
	require.NoError(t, err, "query tag")
	assert.Equal(t, 1, tagCount, "orphaned tags should be kept")
}

func TestListItemsByContainer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, containerID, "List A", nil, []string{"x"})
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, containerID, "List B", nil, []string{"y"})
	require.NoError(t, err)

	result, err := s.ListItemsByContainer(ctx, containerID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Regexp(t, `^\d+[A-Z]$`, result.Label)
	assert.Len(t, result.Items, 2)
}

func TestListItemsByContainerNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	result, err := s.ListItemsByContainer(ctx, 99999)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestListAllItems(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	containerA := getTestContainerID(t, s)
	containerB := getSecondContainerID(t, s, containerA)

	_, err := s.CreateItem(ctx, containerA, "All A", nil, nil)
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, containerB, "All B", nil, nil)
	require.NoError(t, err)

	items, err := s.ListAllItems(ctx)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestListTags(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, containerID, "Tags test", nil, []string{"alpha", "beta"})
	require.NoError(t, err)

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tags), 2)
	if len(tags) >= 2 {
		assert.LessOrEqual(t, tags[0].Name, tags[1].Name, "tags not sorted")
	}
}

func TestListTagsWithPrefix(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, containerID, "Prefix test", nil, []string{"m6", "m8", "bolt"})
	require.NoError(t, err)

	tags, err := s.ListTags(ctx, "m")
	require.NoError(t, err)
	assert.Len(t, tags, 2)
	for _, tag := range tags {
		assert.Contains(t, []string{"m6", "m8"}, tag.Name)
	}
}

// --- Search tests ---

func TestSearchItemsByName(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getContainerIDByPos(t, s, 1, 1)

	_, err := s.CreateItem(ctx, containerID, "sprezynowa", nil, []string{"washer"})
	require.NoError(t, err)

	results, err := s.SearchItems(ctx, "sprez")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "sprezynowa", results[0].Name)
	assert.NotEmpty(t, results[0].ContainerLabel)
}

func TestSearchItemsByTag(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getContainerIDByPos(t, s, 1, 1)

	_, err := s.CreateItem(ctx, containerID, "Bolt DIN 933", nil, []string{"m6"})
	require.NoError(t, err)

	results, err := s.SearchItems(ctx, "m6")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Bolt DIN 933", results[0].Name)
}

func TestSearchTagExactMatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getContainerIDByPos(t, s, 1, 1)

	_, err := s.CreateItem(ctx, containerID, "Small bolt", nil, []string{"m6"})
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, containerID, "Large bolt", nil, []string{"m60"})
	require.NoError(t, err)

	results, err := s.SearchItems(ctx, "m6")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Small bolt", results[0].Name)
}

func TestSearchItemsCaseInsensitive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getContainerIDByPos(t, s, 1, 1)

	_, err := s.CreateItem(ctx, containerID, "PODKLADKA", nil, []string{"washer"})
	require.NoError(t, err)

	results, err := s.SearchItems(ctx, "podkladka")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "PODKLADKA", results[0].Name)
}

func TestSearchItemsPartialName(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getContainerIDByPos(t, s, 1, 1)

	_, err := s.CreateItem(ctx, containerID, "sprezynowa", nil, []string{"washer"})
	require.NoError(t, err)

	results, err := s.SearchItems(ctx, "sprez")
	require.NoError(t, err)
	require.Len(t, results, 1, "prefix match")

	results, err = s.SearchItems(ctx, "zynowa")
	require.NoError(t, err)
	require.Len(t, results, 1, "suffix match")
}

func TestSearchItemsDedup(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getContainerIDByPos(t, s, 1, 1)

	_, err := s.CreateItem(ctx, containerID, "m6 bolt", nil, []string{"m6"})
	require.NoError(t, err)

	results, err := s.SearchItems(ctx, "m6")
	require.NoError(t, err)
	assert.Len(t, results, 1, "dedup failed")
}

func TestSearchItemsSorted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c3A := getContainerIDByPos(t, s, 3, 1)
	_, err := s.CreateItem(ctx, c3A, "sorttest alpha", nil, []string{"sorttest"})
	require.NoError(t, err)

	c1B := getContainerIDByPos(t, s, 1, 2)
	_, err = s.CreateItem(ctx, c1B, "sorttest beta", nil, []string{"sorttest"})
	require.NoError(t, err)

	c2A := getContainerIDByPos(t, s, 2, 1)
	_, err = s.CreateItem(ctx, c2A, "sorttest gamma", nil, []string{"sorttest"})
	require.NoError(t, err)

	results, err := s.SearchItems(ctx, "sorttest")
	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.Equal(t, "1B", results[0].ContainerLabel)
	assert.Equal(t, "2A", results[1].ContainerLabel)
	assert.Equal(t, "3A", results[2].ContainerLabel)
}

func TestSearchItemsEmpty(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	results, err := s.SearchItems(ctx, "")
	require.NoError(t, err)
	require.NotNil(t, results, "results is nil, want empty slice")
	assert.Empty(t, results)
}

func TestSearchItemsWildcardEscape(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getContainerIDByPos(t, s, 1, 1)

	_, err := s.CreateItem(ctx, containerID, "100% bolt", nil, []string{"special"})
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, containerID, "200 nut", nil, []string{"other"})
	require.NoError(t, err)

	results, err := s.SearchItems(ctx, "100%")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "100% bolt", results[0].Name)
}

func TestTagsInJunctionTable(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	item, err := s.CreateItem(ctx, containerID, "Junction test", nil, []string{"tag1", "tag2"})
	require.NoError(t, err)

	var count int
	err = s.conn.QueryRow("SELECT COUNT(*) FROM item_tag WHERE item_id = ?", item.ID).Scan(&count)
	require.NoError(t, err, "query item_tag")
	assert.Equal(t, 2, count)
}

// --- ResizeShelf tests ---

func TestResizeShelf_BlockedWhenItemsExist(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	insertItemAt(t, s, 10, 5, "M6 bolt")

	result, err := s.ResizeShelf(ctx, 3, 3)
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.NotEmpty(t, result.AffectedContainers)

	found := false
	for _, ac := range result.AffectedContainers {
		if ac.Label == "10E" {
			found = true
			assert.Equal(t, 1, ac.ItemCount)
			assert.Equal(t, []string{"M6 bolt"}, ac.Items)
		}
	}
	assert.True(t, found, "AffectedContainers missing label 10E, got: %+v", result.AffectedContainers)

	var rows, cols int
	require.NoError(t, s.conn.QueryRow("SELECT rows, cols FROM shelf LIMIT 1").Scan(&rows, &cols))
	assert.Equal(t, 5, rows)
	assert.Equal(t, 10, cols)
}

func TestResizeShelf_AffectedContainerDetails(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	insertItemAt(t, s, 5, 5, "M6 bolt")
	insertItemAt(t, s, 5, 5, "M6 nut")

	result, err := s.ResizeShelf(ctx, 4, 4)
	require.NoError(t, err)
	require.True(t, result.Blocked)

	found := false
	for _, ac := range result.AffectedContainers {
		if ac.Label == "5E" {
			found = true
			assert.Equal(t, 2, ac.ItemCount)
			assert.Contains(t, ac.Items, "M6 bolt")
			assert.Contains(t, ac.Items, "M6 nut")
		}
	}
	assert.True(t, found, "AffectedContainers missing label 5E, got: %+v", result.AffectedContainers)
}

func TestResizeShelf_ExpandCreatesContainers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	result, err := s.ResizeShelf(ctx, 7, 12)
	require.NoError(t, err)
	assert.False(t, result.Blocked)
	assert.Equal(t, 7, result.Rows)
	assert.Equal(t, 12, result.Cols)

	for _, tc := range []struct{ col, row int }{{11, 1}, {1, 6}, {12, 7}} {
		var count int
		err := s.conn.QueryRow("SELECT COUNT(*) FROM container WHERE col = ? AND row = ?", tc.col, tc.row).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "container at (%d,%d)", tc.col, tc.row)
	}

	var total int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM container").Scan(&total))
	assert.Equal(t, 84, total)

	var rows, cols int
	require.NoError(t, s.conn.QueryRow("SELECT rows, cols FROM shelf LIMIT 1").Scan(&rows, &cols))
	assert.Equal(t, 7, rows)
	assert.Equal(t, 12, cols)
}

func TestResizeShelf_ShrinkDeletesEmptyContainers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	result, err := s.ResizeShelf(ctx, 3, 5)
	require.NoError(t, err)
	assert.False(t, result.Blocked)
	assert.Equal(t, 3, result.Rows)
	assert.Equal(t, 5, result.Cols)

	var outOfBounds int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM container WHERE row > 3 OR col > 5").Scan(&outOfBounds))
	assert.Equal(t, 0, outOfBounds)

	var total int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM container").Scan(&total))
	assert.Equal(t, 15, total)
}

func TestResizeShelf_SameSize(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	result, err := s.ResizeShelf(ctx, 5, 10)
	require.NoError(t, err)
	assert.False(t, result.Blocked)
	assert.Equal(t, 5, result.Rows)
	assert.Equal(t, 10, result.Cols)

	var total int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM container").Scan(&total))
	assert.Equal(t, 50, total)
}

func TestResizeShelf_ShrinkWithMixedOccupancy(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	insertItemAt(t, s, 1, 1, "Small bolt")

	result, err := s.ResizeShelf(ctx, 3, 3)
	require.NoError(t, err)
	assert.False(t, result.Blocked, "item at (1,1) is within 3x3 bounds")
	assert.Equal(t, 3, result.Rows)
	assert.Equal(t, 3, result.Cols)

	var outOfBounds int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM container WHERE row > 3 OR col > 3").Scan(&outOfBounds))
	assert.Equal(t, 0, outOfBounds)

	var itemCount int
	require.NoError(t, s.conn.QueryRow("SELECT COUNT(*) FROM item").Scan(&itemCount))
	assert.Equal(t, 1, itemCount)
}

// --- UpdateShelfName ---

func TestUpdateShelfName(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.UpdateShelfName(ctx, "Test Shelf"))

	var name string
	err := s.conn.QueryRow("SELECT name FROM shelf LIMIT 1").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "Test Shelf", name)
}

// --- DB accessor ---

// --- Close nil ---

func TestCloseNilDB(t *testing.T) {
	s := &Store{}
	assert.NoError(t, s.Close())
}

// --- AddTagToItem edge cases ---

func TestAddTagToItemNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.AddTagToItem(ctx, 99999, "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "item not found")
}

func TestAddTagDuplicate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	created, err := s.CreateItem(ctx, containerID, "dup tag", nil, []string{"m6"})
	require.NoError(t, err)

	// Adding same tag again should not error (INSERT OR IGNORE)
	item, err := s.AddTagToItem(ctx, created.ID, "m6")
	require.NoError(t, err)
	assert.Len(t, item.Tags, 1)
}

// --- RemoveTagFromItem edge cases ---

func TestRemoveTagNotAssociated(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	created, err := s.CreateItem(ctx, containerID, "test", nil, []string{"m6"})
	require.NoError(t, err)

	// Create a tag that is NOT associated with the item
	_, err = s.conn.Exec("INSERT OR IGNORE INTO tag (name) VALUES (?)", "orphan")
	require.NoError(t, err)

	err = s.RemoveTagFromItem(ctx, created.ID, "orphan")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag not associated with item")
}

func TestRemoveTagNonexistent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.RemoveTagFromItem(ctx, 1, "nonexistent_tag_xyz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag not found")
}

// --- SearchItemsBatch tests ---

func TestSearchBatchNoTagsNameMatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, cid, "M6 Bolt", nil, []string{"hardware"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "bolt", nil)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "M6 Bolt", resp.Results[0].Name)
	assert.Contains(t, resp.Results[0].MatchedOn, "name")
}

func TestSearchBatchNoTagsTagMatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, cid, "Hex Screw", nil, []string{"m6", "din912"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "m6", nil)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "Hex Screw", resp.Results[0].Name)
	assert.Contains(t, resp.Results[0].MatchedOn, "tag")
}

func TestSearchBatchNoTagsDescriptionMatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	desc := "stainless steel hex bolt"
	_, err := s.CreateItem(ctx, cid, "Bolt A2", &desc, []string{"hardware"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "stainless", nil)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "Bolt A2", resp.Results[0].Name)
	assert.Contains(t, resp.Results[0].MatchedOn, "description")
}

func TestSearchBatchMultiTagAND(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)
	cid2 := getSecondContainerID(t, s, cid)

	_, err := s.CreateItem(ctx, cid, "Item Both", nil, []string{"m6", "din912"})
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, cid2, "Item One", nil, []string{"m6", "hex"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "", []string{"m6", "din912"})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "Item Both", resp.Results[0].Name)
}

func TestSearchBatchTagsActiveNoTagTextMatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, cid, "Screw M6", nil, []string{"m6", "bolt"})
	require.NoError(t, err)

	// With tags=["m6"] active and query "bolt", item should NOT be returned
	// because "bolt" is a tag name, not in the item's name or description,
	// and tag text matching is disabled when tag filters are active (D-09).
	resp, err := s.SearchItemsBatch(ctx, "bolt", []string{"m6"})
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
}

func TestSearchBatchTagsActiveNameMatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, cid, "Screw M6", nil, []string{"m6"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "screw", []string{"m6"})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "Screw M6", resp.Results[0].Name)
	assert.Contains(t, resp.Results[0].MatchedOn, "name")
}

func TestSearchBatchTagsActiveDescMatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	desc := "stainless steel"
	_, err := s.CreateItem(ctx, cid, "Bolt X", &desc, []string{"m6"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "stainless", []string{"m6"})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Contains(t, resp.Results[0].MatchedOn, "description")
}

func TestSearchBatchTagsOnlyNoText(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)
	cid2 := getSecondContainerID(t, s, cid)

	_, err := s.CreateItem(ctx, cid, "A", nil, []string{"m6", "din912"})
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, cid2, "B", nil, []string{"m6", "din912"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "", []string{"m6", "din912"})
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, 2, resp.TotalCount)
}

func TestSearchBatchGroupConcat(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, cid, "Multi Tag", nil, []string{"m6", "din912", "hex"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "multi", nil)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Len(t, resp.Results[0].Tags, 3)
	assert.Contains(t, resp.Results[0].Tags, "m6")
	assert.Contains(t, resp.Results[0].Tags, "din912")
	assert.Contains(t, resp.Results[0].Tags, "hex")
}

func TestSearchBatchLimit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	for i := range 55 {
		_, err := s.CreateItem(ctx, cid, fmt.Sprintf("Bolt %03d", i), nil, []string{"bulk"})
		require.NoError(t, err)
	}

	resp, err := s.SearchItemsBatch(ctx, "bolt", nil)
	require.NoError(t, err)
	assert.Len(t, resp.Results, 50)
	assert.Equal(t, 55, resp.TotalCount)
}

func TestSearchBatchCaseInsensitive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	desc := "Hex Bolt spec"
	_, err := s.CreateItem(ctx, cid, "m6 bolt", &desc, []string{"hardware"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "BOLT", nil)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Contains(t, resp.Results[0].MatchedOn, "name")
	assert.Contains(t, resp.Results[0].MatchedOn, "description")
}

func TestSearchBatchNoResults(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	resp, err := s.SearchItemsBatch(ctx, "nonexistent", nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
	assert.Equal(t, 0, resp.TotalCount)
}

func TestSearchBatchEmptyQuery(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	resp, err := s.SearchItemsBatch(ctx, "", nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
	assert.Equal(t, 0, resp.TotalCount)
}

func TestSearchBatchNullDescription(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cid := getTestContainerID(t, s)

	_, err := s.CreateItem(ctx, cid, "Bolt NoDesc", nil, []string{"m6"})
	require.NoError(t, err)

	resp, err := s.SearchItemsBatch(ctx, "bolt", nil)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Contains(t, resp.Results[0].MatchedOn, "name")
	assert.NotContains(t, resp.Results[0].MatchedOn, "description")
	assert.Nil(t, resp.Results[0].Description)
}

func TestOIDCConfigSaveAndGet(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	cfg := &model.OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "my-client-id",
		ClientSecret: "super-secret",
		DisplayName:  "Authelia",
	}
	err := s.SaveOIDCConfig(ctx, cfg)
	require.NoError(t, err)

	got, err := s.GetOIDCConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.Enabled)
	assert.Equal(t, "https://auth.example.com", got.IssuerURL)
	assert.Equal(t, "my-client-id", got.ClientID)
	assert.Equal(t, "super-secret", got.ClientSecret)
	assert.Equal(t, "Authelia", got.DisplayName)
	assert.Equal(t, "configured", got.SecretStatus)
}

func TestOIDCConfigGetDefault(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	got, err := s.GetOIDCConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.Enabled)
	assert.Empty(t, got.IssuerURL)
	assert.Empty(t, got.ClientID)
	assert.Empty(t, got.ClientSecret)
	assert.Empty(t, got.DisplayName)
	assert.Equal(t, "not_set", got.SecretStatus)
}

func TestOIDCConfigMaskedHidesSecret(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	cfg := &model.OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "my-client-id",
		ClientSecret: "super-secret",
		DisplayName:  "Authelia",
	}
	require.NoError(t, s.SaveOIDCConfig(ctx, cfg))

	got, err := s.GetOIDCConfigMasked(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got.ClientSecret, "masked config should not return client secret")
	assert.Equal(t, "configured", got.SecretStatus)
}

func TestOIDCConfigSavePreservesSecret(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	cfg := &model.OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "my-client-id",
		ClientSecret: "original-secret",
		DisplayName:  "Authelia",
	}
	require.NoError(t, s.SaveOIDCConfig(ctx, cfg))

	cfg2 := &model.OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "my-client-id",
		ClientSecret: "",
		DisplayName:  "Updated Authelia",
	}
	require.NoError(t, s.SaveOIDCConfig(ctx, cfg2))

	got, err := s.GetOIDCConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "original-secret", got.ClientSecret, "empty secret should preserve existing")
	assert.Equal(t, "Updated Authelia", got.DisplayName)
}

func TestUpsertOIDCUser(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	user := &model.OIDCUser{
		Sub:         "user123",
		Issuer:      "https://auth.example.com",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		AvatarURL:   "https://example.com/avatar.png",
	}
	got, err := s.UpsertOIDCUser(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "user123", got.Sub)
	assert.Equal(t, "https://auth.example.com", got.Issuer)
	assert.Equal(t, "alice@example.com", got.Email)
	assert.Equal(t, "Alice", got.DisplayName)
	assert.NotEmpty(t, got.CreatedAt)

	fetched, err := s.GetOIDCUserBySub(ctx, "user123", "https://auth.example.com")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, "Alice", fetched.DisplayName)
}

func TestUpsertOIDCUserUpdates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	user := &model.OIDCUser{
		Sub:         "user123",
		Issuer:      "https://auth.example.com",
		Email:       "alice@example.com",
		DisplayName: "Alice",
	}
	_, err := s.UpsertOIDCUser(ctx, user)
	require.NoError(t, err)

	user2 := &model.OIDCUser{
		Sub:         "user123",
		Issuer:      "https://auth.example.com",
		Email:       "alice-new@example.com",
		DisplayName: "Alice Updated",
	}
	got, err := s.UpsertOIDCUser(ctx, user2)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Alice Updated", got.DisplayName)
	assert.Equal(t, "alice-new@example.com", got.Email)
}

func TestGetOIDCUserBySub_NotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	got, err := s.GetOIDCUserBySub(ctx, "nonexistent", "https://auth.example.com")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestGetOrCreateEncryptionKey(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	key1, err := s.GetOrCreateEncryptionKey(ctx)
	require.NoError(t, err)
	require.NotNil(t, key1)

	key2, err := s.GetOrCreateEncryptionKey(ctx)
	require.NoError(t, err)
	assert.Equal(t, key1, key2, "second call should return same key")
}

func TestGetOrCreateEncryptionKey_Length(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	key, err := s.GetOrCreateEncryptionKey(ctx)
	require.NoError(t, err)
	assert.Len(t, key, 32, "encryption key should be 32 bytes (256 bits)")
}

// --- Tag management tests ---

// helper: insert tag directly, return tag ID
func insertTestTag(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	res, err := s.conn.Exec("INSERT INTO tag (name) VALUES (?)", name)
	require.NoError(t, err, "insert tag %q", name)
	id, _ := res.LastInsertId()
	return id
}

// helper: insert item directly, return item ID
func insertTestItem(t *testing.T, s *Store, containerID int64, name string) int64 {
	t.Helper()
	res, err := s.conn.Exec("INSERT INTO item (container_id, name) VALUES (?, ?)", containerID, name)
	require.NoError(t, err, "insert item %q", name)
	id, _ := res.LastInsertId()
	return id
}

// helper: link item to tag
func linkItemTag(t *testing.T, s *Store, itemID, tagID int64) {
	t.Helper()
	_, err := s.conn.Exec("INSERT INTO item_tag (item_id, tag_id) VALUES (?, ?)", itemID, tagID)
	require.NoError(t, err, "link item %d to tag %d", itemID, tagID)
}

func TestFindDuplicates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := getTestContainerID(t, s)
	c2 := getSecondContainerID(t, s, c1)

	// Two items with same name (different case), same tags
	tagID := insertTestTag(t, s, "metric")
	tagID2 := insertTestTag(t, s, "hex")
	item1 := insertTestItem(t, s, c1, "M4 Bolt")
	linkItemTag(t, s, item1, tagID)
	linkItemTag(t, s, item1, tagID2)
	item2 := insertTestItem(t, s, c2, "m4 bolt")
	linkItemTag(t, s, item2, tagID)
	linkItemTag(t, s, item2, tagID2)

	groups, err := s.FindDuplicates(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, 2, groups[0].Count)
	assert.Len(t, groups[0].Containers, 2)
	assert.Len(t, groups[0].Tags, 2)
}

func TestFindDuplicatesDifferentTags(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := getTestContainerID(t, s)
	c2 := getSecondContainerID(t, s, c1)

	tag1 := insertTestTag(t, s, "metric")
	tag2 := insertTestTag(t, s, "imperial")
	item1 := insertTestItem(t, s, c1, "M4 Bolt")
	linkItemTag(t, s, item1, tag1)
	item2 := insertTestItem(t, s, c2, "M4 Bolt")
	linkItemTag(t, s, item2, tag2)

	groups, err := s.FindDuplicates(ctx)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestFindDuplicatesTagless(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := getTestContainerID(t, s)
	c2 := getSecondContainerID(t, s, c1)

	insertTestItem(t, s, c1, "Plain washer")
	insertTestItem(t, s, c2, "plain washer")

	groups, err := s.FindDuplicates(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, 2, groups[0].Count)
	assert.Empty(t, groups[0].Tags)
}

func TestFindDuplicatesNone(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := getTestContainerID(t, s)

	insertTestItem(t, s, c1, "Bolt A")
	insertTestItem(t, s, c1, "Bolt B")
	insertTestItem(t, s, c1, "Bolt C")

	groups, err := s.FindDuplicates(ctx)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestFindDuplicatesSorted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := getTestContainerID(t, s)
	c2 := getSecondContainerID(t, s, c1)

	// Washer duplicates
	insertTestItem(t, s, c1, "Washer")
	insertTestItem(t, s, c2, "washer")

	// Bolt duplicates
	insertTestItem(t, s, c1, "Bolt")
	insertTestItem(t, s, c2, "bolt")

	groups, err := s.FindDuplicates(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	// Alphabetical: bolt before washer
	assert.Equal(t, "bolt", strings.ToLower(groups[0].Name))
	assert.Equal(t, 2, groups[0].Count)
	assert.Equal(t, "washer", strings.ToLower(groups[1].Name))
	assert.Equal(t, 2, groups[1].Count)
}

func TestFindDuplicatesWhitespace(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := getTestContainerID(t, s)
	c2 := getSecondContainerID(t, s, c1)

	insertTestItem(t, s, c1, "M4 bolt ")
	insertTestItem(t, s, c2, "M4 bolt")

	groups, err := s.FindDuplicates(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, 2, groups[0].Count)
}

func TestFindDuplicatesSameContainer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c1 := getTestContainerID(t, s)

	// Two items with same name in the SAME container should NOT be reported
	insertTestItem(t, s, c1, "M4 bolt")
	insertTestItem(t, s, c1, "m4 bolt")

	groups, err := s.FindDuplicates(ctx)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestListTagsWithCount(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	tagA := insertTestTag(t, s, "bolt")
	tagB := insertTestTag(t, s, "nut")
	_ = insertTestTag(t, s, "washer") // no items

	item1 := insertTestItem(t, s, containerID, "M6 Bolt")
	item2 := insertTestItem(t, s, containerID, "M8 Bolt")
	item3 := insertTestItem(t, s, containerID, "M6 Nut")

	linkItemTag(t, s, item1, tagA)
	linkItemTag(t, s, item2, tagA)
	linkItemTag(t, s, item3, tagB)

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	require.Len(t, tags, 3)

	// Tags are sorted alphabetically: bolt, nut, washer
	assert.Equal(t, "bolt", tags[0].Name)
	assert.Equal(t, 2, tags[0].ItemCount)

	assert.Equal(t, "nut", tags[1].Name)
	assert.Equal(t, 1, tags[1].ItemCount)

	assert.Equal(t, "washer", tags[2].Name)
	assert.Equal(t, 0, tags[2].ItemCount)
}

func TestRenameTag(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	tagID := insertTestTag(t, s, "bolt")
	itemID := insertTestItem(t, s, containerID, "M6 Bolt")
	linkItemTag(t, s, itemID, tagID)

	err := s.RenameTag(ctx, tagID, "screw")
	require.NoError(t, err)

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "screw", tags[0].Name)
	assert.Equal(t, 1, tags[0].ItemCount)
}

func TestRenameTagNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.RenameTag(ctx, 99999, "anything")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMergeTagsBasic(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	tagA := insertTestTag(t, s, "bolt")
	tagB := insertTestTag(t, s, "screw")
	item1 := insertTestItem(t, s, containerID, "M6 Bolt")
	item2 := insertTestItem(t, s, containerID, "M8 Bolt")
	item3 := insertTestItem(t, s, containerID, "M6 Screw")

	linkItemTag(t, s, item1, tagA)
	linkItemTag(t, s, item2, tagA)
	linkItemTag(t, s, item3, tagB)

	err := s.MergeTags(ctx, tagA, tagB)
	require.NoError(t, err)

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "screw", tags[0].Name)
	assert.Equal(t, 3, tags[0].ItemCount)
}

func TestMergeTagsDedup(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	tagA := insertTestTag(t, s, "bolt")
	tagB := insertTestTag(t, s, "screw")
	item1 := insertTestItem(t, s, containerID, "M6 Bolt")

	// item1 has both tags
	linkItemTag(t, s, item1, tagA)
	linkItemTag(t, s, item1, tagB)

	err := s.MergeTags(ctx, tagA, tagB)
	require.NoError(t, err)

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "screw", tags[0].Name)
	assert.Equal(t, 1, tags[0].ItemCount) // no duplicate
}

func TestDeleteUnusedTag(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_ = insertTestTag(t, s, "empty-tag")

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	require.Len(t, tags, 1)

	err = s.DeleteUnusedTag(ctx, tags[0].ID)
	require.NoError(t, err)

	tags, err = s.ListTags(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestDeleteUsedTagFails(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	containerID := getTestContainerID(t, s)

	tagID := insertTestTag(t, s, "bolt")
	itemID := insertTestItem(t, s, containerID, "M6 Bolt")
	linkItemTag(t, s, itemID, tagID)

	err := s.DeleteUnusedTag(ctx, tagID)
	require.ErrorIs(t, err, ErrTagInUse)

	// Tag should still exist
	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	assert.Len(t, tags, 1)
}
