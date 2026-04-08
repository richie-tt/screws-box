package store

import (
	"context"
	"screws-box/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedTestData creates a shelf with 2 containers and items with tags for testing export.
func seedTestData(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()

	// Default shelf is already created by Open (5 rows, 10 cols, "My Organizer").
	// Update name for test clarity.
	_, err := s.conn.ExecContext(ctx, "UPDATE shelf SET name = 'Test Shelf' WHERE id = 1")
	require.NoError(t, err)

	// Get container IDs at positions (1,1) and (2,1) — these exist from seedDefaultShelf.
	var c1ID, c2ID int64
	err = s.conn.QueryRowContext(ctx, "SELECT id FROM container WHERE col = 1 AND row = 1").Scan(&c1ID)
	require.NoError(t, err, "container at (1,1) should exist")
	err = s.conn.QueryRowContext(ctx, "SELECT id FROM container WHERE col = 2 AND row = 1").Scan(&c2ID)
	require.NoError(t, err, "container at (2,1) should exist")

	// Create items via CreateItem (realistic seeding).
	desc1 := "M3 bolts 10mm"
	_, err = s.CreateItem(ctx, c1ID, "M3x10", &desc1, []string{"bolt", "metric"})
	require.NoError(t, err)

	_, err = s.CreateItem(ctx, c1ID, "M4x12", nil, []string{"bolt"})
	require.NoError(t, err)

	desc2 := "Flat washers M3"
	_, err = s.CreateItem(ctx, c2ID, "Washer M3", &desc2, []string{"washer", "metric"})
	require.NoError(t, err)
}

func TestExportAllData(t *testing.T) {
	s := openTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()

	data, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	// Top-level fields
	assert.Equal(t, 1, data.Version)
	assert.NotEmpty(t, data.ExportedAt)

	// Shelf
	assert.Equal(t, "Test Shelf", data.Shelf.Name)
	assert.Equal(t, 5, data.Shelf.Rows)
	assert.Equal(t, 10, data.Shelf.Cols)

	// Containers — we have 50 (5x10), but only 2 have items. All are exported.
	assert.Len(t, data.Shelf.Containers, 50)

	// Find containers with items.
	var withItems []model.ExportContainer
	for _, c := range data.Shelf.Containers {
		if len(c.Items) > 0 {
			withItems = append(withItems, c)
		}
	}
	assert.Len(t, withItems, 2)

	// Container at (1,1) = label "1A"
	c1 := findExportContainer(data.Shelf.Containers, 1, 1)
	require.NotNil(t, c1, "container at (1,1) not found")
	assert.Equal(t, "1A", c1.Label)
	assert.Len(t, c1.Items, 2)
	// Items sorted by name
	assert.Equal(t, "M3x10", c1.Items[0].Name)
	assert.Equal(t, "M4x12", c1.Items[1].Name)
	assert.NotNil(t, c1.Items[0].Description)
	assert.Equal(t, "M3 bolts 10mm", *c1.Items[0].Description)
	assert.Nil(t, c1.Items[1].Description)
	assert.Equal(t, []string{"bolt", "metric"}, c1.Items[0].Tags)
	assert.Equal(t, []string{"bolt"}, c1.Items[1].Tags)

	// Container at (2,1) = label "2A"
	c2 := findExportContainer(data.Shelf.Containers, 2, 1)
	require.NotNil(t, c2, "container at (2,1) not found")
	assert.Equal(t, "2A", c2.Label)
	assert.Len(t, c2.Items, 1)
	assert.Equal(t, "Washer M3", c2.Items[0].Name)
	assert.Equal(t, []string{"metric", "washer"}, c2.Items[0].Tags)

	// Empty containers should have empty Items slice, not nil
	for _, c := range data.Shelf.Containers {
		assert.NotNil(t, c.Items, "container %s Items should not be nil", c.Label)
	}
}

func TestExportAllDataEmpty(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	data, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, data.Version)
	assert.NotEmpty(t, data.ExportedAt)
	assert.Equal(t, "My Organizer", data.Shelf.Name)
	assert.Equal(t, 5, data.Shelf.Rows)
	assert.Equal(t, 10, data.Shelf.Cols)
	// All 50 containers exist but have no items.
	assert.Len(t, data.Shelf.Containers, 50)
	for _, c := range data.Shelf.Containers {
		assert.Empty(t, c.Items)
		assert.NotNil(t, c.Items)
	}
}

func TestImportAllData(t *testing.T) {
	s := openTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()

	// Prepare import data that replaces everything.
	desc := "Hex nuts M5"
	importData := &model.ExportData{
		Version:    1,
		ExportedAt: "2026-01-01T00:00:00Z",
		Shelf: model.ExportShelf{
			Name: "Imported Shelf",
			Rows: 3,
			Cols: 4,
			Containers: []model.ExportContainer{
				{
					Col: 1, Row: 1, Label: "1A",
					Items: []model.ExportItem{
						{Name: "Nut M5", Description: &desc, Tags: []string{"nut", "metric"}},
					},
				},
				{
					Col: 2, Row: 1, Label: "2A",
					Items: []model.ExportItem{},
				},
			},
		},
	}

	err := s.ImportAllData(ctx, importData)
	require.NoError(t, err)

	// Verify old data is gone and new data is present.
	exported, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	assert.Equal(t, "Imported Shelf", exported.Shelf.Name)
	assert.Equal(t, 3, exported.Shelf.Rows)
	assert.Equal(t, 4, exported.Shelf.Cols)

	// After import, only the containers from the import data exist.
	assert.Len(t, exported.Shelf.Containers, 2)

	c1 := findExportContainer(exported.Shelf.Containers, 1, 1)
	require.NotNil(t, c1)
	assert.Len(t, c1.Items, 1)
	assert.Equal(t, "Nut M5", c1.Items[0].Name)
	assert.Equal(t, []string{"metric", "nut"}, c1.Items[0].Tags)

	// Old items should be gone.
	items, err := s.ListAllItems(ctx)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestImportAllDataRollback(t *testing.T) {
	s := openTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()

	// Export current data for comparison.
	before, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	// Import with invalid container position (col > shelf cols).
	badData := &model.ExportData{
		Version:    1,
		ExportedAt: "2026-01-01T00:00:00Z",
		Shelf: model.ExportShelf{
			Name: "Bad Shelf",
			Rows: 2,
			Cols: 2,
			Containers: []model.ExportContainer{
				{
					Col: 1, Row: 1, Label: "1A",
					Items: []model.ExportItem{
						// Item with empty name to trigger constraint error
						{Name: "", Tags: []string{}},
					},
				},
			},
		},
	}

	// Cause failure by using a duplicate container position (UNIQUE constraint).
	_ = badData // not used directly; we test with duplicate positions below.
	badData2 := &model.ExportData{
		Version:    1,
		ExportedAt: "2026-01-01T00:00:00Z",
		Shelf: model.ExportShelf{
			Name: "Dup Shelf",
			Rows: 2,
			Cols: 2,
			Containers: []model.ExportContainer{
				{Col: 1, Row: 1, Label: "1A", Items: []model.ExportItem{}},
				{Col: 1, Row: 1, Label: "1A", Items: []model.ExportItem{}}, // duplicate!
			},
		},
	}

	err = s.ImportAllData(ctx, badData2)
	require.Error(t, err, "duplicate container positions should cause error")

	// Verify existing data is unchanged.
	after, err := s.ExportAllData(ctx)
	require.NoError(t, err)
	assert.Equal(t, before.Shelf.Name, after.Shelf.Name)
	assert.Equal(t, before.Shelf.Rows, after.Shelf.Rows)
	assert.Equal(t, before.Shelf.Cols, after.Shelf.Cols)
	assert.Len(t, after.Shelf.Containers, len(before.Shelf.Containers))
}

func TestImportInvalidVersion(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	data := &model.ExportData{
		Version:    99,
		ExportedAt: "2026-01-01T00:00:00Z",
		Shelf: model.ExportShelf{
			Name: "Whatever",
			Rows: 2,
			Cols: 2,
		},
	}

	err := s.ImportAllData(ctx, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version")
}

func TestRoundTrip(t *testing.T) {
	s := openTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()

	// Export.
	exported1, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	// Import the exported data.
	err = s.ImportAllData(ctx, exported1)
	require.NoError(t, err)

	// Export again.
	exported2, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	// Compare (ignore ExportedAt timestamps).
	exported1.ExportedAt = ""
	exported2.ExportedAt = ""
	assert.Equal(t, exported1.Shelf, exported2.Shelf)
}

func TestImportTagDeduplication(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Import data where multiple items share the same tag.
	data := &model.ExportData{
		Version:    1,
		ExportedAt: "2026-01-01T00:00:00Z",
		Shelf: model.ExportShelf{
			Name: "Tag Test",
			Rows: 2,
			Cols: 2,
			Containers: []model.ExportContainer{
				{
					Col: 1, Row: 1, Label: "1A",
					Items: []model.ExportItem{
						{Name: "Item1", Tags: []string{"shared", "unique1"}},
						{Name: "Item2", Tags: []string{"shared", "unique2"}},
					},
				},
			},
		},
	}

	err := s.ImportAllData(ctx, data)
	require.NoError(t, err, "import with shared tags should not cause UNIQUE constraint error")

	// Verify both items have the shared tag.
	exported, err := s.ExportAllData(ctx)
	require.NoError(t, err)

	c := findExportContainer(exported.Shelf.Containers, 1, 1)
	require.NotNil(t, c)
	assert.Len(t, c.Items, 2)
	// Both should have "shared" tag.
	for _, item := range c.Items {
		assert.Contains(t, item.Tags, "shared")
	}
}

// findExportContainer finds a container by col and row in a slice.
func findExportContainer(containers []model.ExportContainer, col, row int) *model.ExportContainer { //nolint:unparam // row kept explicit for test readability
	for i := range containers {
		if containers[i].Col == col && containers[i].Row == row {
			return &containers[i]
		}
	}
	return nil
}
