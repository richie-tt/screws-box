package store

import (
	"context"
	"errors"
	"screws-box/internal/model"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errBoom is used as the canonical injected DB error in mock tests.
var errBoom = errors.New("boom")

// newMockStore returns a Store backed by sqlmock and the mock controller for
// setting expectations. Default matcher is regex, which we use throughout.
func newMockStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &Store{conn: db}, mock
}

// reAny matches any SQL — convenient when we don't need to assert exact text.
const reAny = `.*`

// --- ResizeShelf deep branches --------------------------------------------

func TestMockResizeShelfShelfScanError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, rows, cols FROM shelf`).WillReturnError(errBoom)
	mock.ExpectRollback()

	_, err := s.ResizeShelf(context.Background(), 5, 5)
	assert.Error(t, err)
}

func TestMockResizeShelfQueryContainersError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rows", "cols"}).AddRow(1, 5, 5))
	mock.ExpectQuery(`SELECT c.id, c.col, c.row FROM container c`).WillReturnError(errBoom)
	mock.ExpectRollback()

	_, err := s.ResizeShelf(context.Background(), 3, 3)
	assert.Error(t, err)
}

func TestMockResizeShelfScanContainerError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rows", "cols"}).AddRow(1, 5, 5))
	// Return a row whose id column is non-int → forces Scan to fail.
	rows := sqlmock.NewRows([]string{"id", "col", "row"}).AddRow("not-an-int", 5, 5)
	mock.ExpectQuery(`SELECT c.id, c.col, c.row FROM container c`).WillReturnRows(rows)
	mock.ExpectRollback()

	_, err := s.ResizeShelf(context.Background(), 3, 3)
	assert.Error(t, err)
}

func TestMockResizeShelfIterateContainersError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rows", "cols"}).AddRow(1, 5, 5))
	rows := sqlmock.NewRows([]string{"id", "col", "row"}).
		AddRow(int64(10), 5, 5).
		RowError(0, errBoom)
	mock.ExpectQuery(`SELECT c.id, c.col, c.row FROM container c`).WillReturnRows(rows)
	mock.ExpectRollback()

	_, err := s.ResizeShelf(context.Background(), 3, 3)
	assert.Error(t, err)
}

func TestMockResizeShelfQueryItemsError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rows", "cols"}).AddRow(1, 5, 5))
	mock.ExpectQuery(`SELECT c.id, c.col, c.row FROM container c`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}).AddRow(int64(10), 5, 5))
	mock.ExpectQuery(`SELECT name FROM item WHERE container_id`).WillReturnError(errBoom)
	mock.ExpectRollback()

	_, err := s.ResizeShelf(context.Background(), 3, 3)
	assert.Error(t, err)
}

func TestMockResizeShelfScanItemNameError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rows", "cols"}).AddRow(1, 5, 5))
	mock.ExpectQuery(`SELECT c.id, c.col, c.row FROM container c`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}).AddRow(int64(10), 5, 5))
	// Return a name with closer-error to force Scan failure.
	itemRows := sqlmock.NewRows([]string{"name"}).AddRow(nil)
	mock.ExpectQuery(`SELECT name FROM item WHERE container_id`).WillReturnRows(itemRows)
	mock.ExpectRollback()

	_, err := s.ResizeShelf(context.Background(), 3, 3)
	assert.Error(t, err)
}

func TestMockResizeShelfIterateItemsError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rows", "cols"}).AddRow(1, 5, 5))
	mock.ExpectQuery(`SELECT c.id, c.col, c.row FROM container c`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}).AddRow(int64(10), 5, 5))
	itemRows := sqlmock.NewRows([]string{"name"}).
		AddRow("apple").
		RowError(0, errBoom)
	mock.ExpectQuery(`SELECT name FROM item WHERE container_id`).WillReturnRows(itemRows)
	mock.ExpectRollback()

	_, err := s.ResizeShelf(context.Background(), 3, 3)
	assert.Error(t, err)
}

func TestMockResizeShelfCommitError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rows", "cols"}).AddRow(1, 5, 5))
	// No out-of-bounds containers → empty result.
	mock.ExpectQuery(`SELECT c.id, c.col, c.row FROM container c`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}))
	mock.ExpectExec(`DELETE FROM container WHERE shelf_id`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`UPDATE shelf SET rows`).WillReturnResult(sqlmock.NewResult(0, 1))
	// 5x5 = 25 inserts before commit.
	for range 25 {
		mock.ExpectExec(`INSERT OR IGNORE INTO container`).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit().WillReturnError(errBoom)

	_, err := s.ResizeShelf(context.Background(), 5, 5)
	assert.Error(t, err)
}

// --- CreateItem deep branches --------------------------------------------

func TestMockCreateItemLastInsertIdError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM container WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT INTO item`).
		WillReturnResult(sqlmock.NewErrorResult(errBoom))
	mock.ExpectRollback()

	_, err := s.CreateItem(context.Background(), 1, "x", nil, nil)
	assert.Error(t, err)
}

func TestMockCreateItemGetTagIdError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM container WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT INTO item`).
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec(`INSERT OR IGNORE INTO tag`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id FROM tag WHERE name`).WillReturnError(errBoom)
	mock.ExpectRollback()

	_, err := s.CreateItem(context.Background(), 1, "x", nil, []string{"t"})
	assert.Error(t, err)
}

func TestMockCreateItemCommitError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM container WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT INTO item`).WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectCommit().WillReturnError(errBoom)

	_, err := s.CreateItem(context.Background(), 1, "x", nil, nil)
	assert.Error(t, err)
}

// --- AddTagToItem deep branches -------------------------------------------

func TestMockAddTagToItemGetTagIdError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM item WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT OR IGNORE INTO tag`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id FROM tag WHERE name`).WillReturnError(errBoom)
	mock.ExpectRollback()

	_, err := s.AddTagToItem(context.Background(), 1, "t")
	assert.Error(t, err)
}

func TestMockAddTagToItemCommitError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM item WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT OR IGNORE INTO tag`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id FROM tag WHERE name`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
	mock.ExpectExec(`INSERT OR IGNORE INTO item_tag`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE item SET updated_at`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errBoom)

	_, err := s.AddTagToItem(context.Background(), 1, "t")
	assert.Error(t, err)
}

// --- UpdateItem deep branches ---------------------------------------------

func TestMockUpdateItemContainerCheckScanError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM item WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT id FROM container WHERE id`).WillReturnError(errBoom)
	mock.ExpectRollback()

	_, err := s.UpdateItem(context.Background(), 1, "x", nil, 99)
	assert.Error(t, err)
}

func TestMockUpdateItemCommitError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM item WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT id FROM container WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
	mock.ExpectExec(`UPDATE item SET name`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errBoom)

	_, err := s.UpdateItem(context.Background(), 1, "x", nil, 2)
	assert.Error(t, err)
}

func TestMockUpdateItemContainerNotFound(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM item WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	// Empty result → triggers sql.ErrNoRows in container check.
	mock.ExpectQuery(`SELECT id FROM container WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	_, err := s.UpdateItem(context.Background(), 1, "x", nil, 99)
	assert.Error(t, err)
}

// --- DeleteItem / RemoveTagFromItem deep branches -------------------------

func TestMockDeleteItemCheckError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id FROM item WHERE id`).WillReturnError(errBoom)

	err := s.DeleteItem(context.Background(), 1)
	assert.Error(t, err)
}

func TestMockRemoveTagFromItemRowsAffectedError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id FROM tag WHERE name`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
	mock.ExpectExec(`DELETE FROM item_tag`).
		WillReturnResult(sqlmock.NewErrorResult(errBoom))

	err := s.RemoveTagFromItem(context.Background(), 1, "t")
	assert.Error(t, err)
}

// --- GetItem deep branches ------------------------------------------------

func TestMockGetItemQueryTagsError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at"}
	mock.ExpectQuery(`SELECT i.id, i.container_id, i.name`).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), int64(2), "n", nil, 1, 1, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"))
	mock.ExpectQuery(`SELECT t.name FROM tag t JOIN item_tag`).WillReturnError(errBoom)

	_, err := s.GetItem(context.Background(), 1)
	assert.Error(t, err)
}

func TestMockGetItemScanTagError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at"}
	mock.ExpectQuery(`SELECT i.id, i.container_id, i.name`).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), int64(2), "n", nil, 1, 1, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"))
	tagRows := sqlmock.NewRows([]string{"name"}).AddRow(nil)
	mock.ExpectQuery(`SELECT t.name FROM tag t JOIN item_tag`).WillReturnRows(tagRows)

	_, err := s.GetItem(context.Background(), 1)
	assert.Error(t, err)
}

func TestMockGetItemIterateTagsError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at"}
	mock.ExpectQuery(`SELECT i.id, i.container_id, i.name`).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), int64(2), "n", nil, 1, 1, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"))
	tagRows := sqlmock.NewRows([]string{"name"}).AddRow("a").RowError(0, errBoom)
	mock.ExpectQuery(`SELECT t.name FROM tag t JOIN item_tag`).WillReturnRows(tagRows)

	_, err := s.GetItem(context.Background(), 1)
	assert.Error(t, err)
}

// --- ListAllItems / ListItemsByContainer / SearchItems deep branches ------

func TestMockListAllItemsScanError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at", "tags"}
	rows := sqlmock.NewRows(cols).AddRow(nil, int64(1), "n", nil, 1, 1, "x", "x", "")
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.ListAllItems(context.Background())
	assert.Error(t, err)
}

func TestMockListAllItemsIterateError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at", "tags"}
	rows := sqlmock.NewRows(cols).
		AddRow(int64(1), int64(2), "n", nil, 1, 1, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", "").
		RowError(0, errBoom)
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.ListAllItems(context.Background())
	assert.Error(t, err)
}

func TestMockListItemsByContainerScanError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT col, row FROM container WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"col", "row"}).AddRow(1, 1))
	cols := []string{"id", "container_id", "name", "description", "created_at", "updated_at", "tags"}
	rows := sqlmock.NewRows(cols).AddRow(nil, int64(1), "n", nil, "x", "x", "")
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.ListItemsByContainer(context.Background(), 1)
	assert.Error(t, err)
}

func TestMockListItemsByContainerContainerScanError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT col, row FROM container WHERE id`).WillReturnError(errBoom)

	_, err := s.ListItemsByContainer(context.Background(), 1)
	assert.Error(t, err)
}

func TestMockSearchItemsScanError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at", "tags"}
	rows := sqlmock.NewRows(cols).AddRow(nil, int64(1), "n", nil, 1, 1, "x", "x", "")
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.SearchItems(context.Background(), "q")
	assert.Error(t, err)
}

func TestMockSearchItemsIterateError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at", "tags"}
	rows := sqlmock.NewRows(cols).
		AddRow(int64(1), int64(2), "n", nil, 1, 1, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", "").
		RowError(0, errBoom)
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.SearchItems(context.Background(), "q")
	assert.Error(t, err)
}

// --- ListTags deep branches -----------------------------------------------

func TestMockListTagsScanError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "name", "created_at", "updated_at", "item_count"}
	rows := sqlmock.NewRows(cols).AddRow(nil, "t", "x", "x", 0)
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.ListTags(context.Background(), "")
	assert.Error(t, err)
}

func TestMockListTagsIterateError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "name", "created_at", "updated_at", "item_count"}
	rows := sqlmock.NewRows(cols).
		AddRow(int64(1), "t", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 0).
		RowError(0, errBoom)
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.ListTags(context.Background(), "")
	assert.Error(t, err)
}

// --- RenameTag / DeleteUnusedTag rows-affected branches -------------------

func TestMockRenameTagRowsAffectedError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectExec(`UPDATE tag SET name`).
		WillReturnResult(sqlmock.NewErrorResult(errBoom))

	err := s.RenameTag(context.Background(), 1, "new")
	assert.Error(t, err)
}

func TestMockDeleteUnusedTagRowsAffectedError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectExec(`DELETE FROM tag`).
		WillReturnResult(sqlmock.NewErrorResult(errBoom))

	err := s.DeleteUnusedTag(context.Background(), 1)
	assert.Error(t, err)
}

// --- FindDuplicates deep branches -----------------------------------------

func TestMockFindDuplicatesScanError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"original_name", "norm_name", "container_id", "col", "row", "tag_fingerprint"}
	// container_id is nil → forces Scan error on int64.
	rows := sqlmock.NewRows(cols).AddRow("n", "n", nil, 1, 1, "")
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.FindDuplicates(context.Background())
	assert.Error(t, err)
}

func TestMockFindDuplicatesIterateError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"original_name", "norm_name", "container_id", "col", "row", "tag_fingerprint"}
	rows := sqlmock.NewRows(cols).
		AddRow("n", "n", int64(1), 1, 1, "").
		RowError(0, errBoom)
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.FindDuplicates(context.Background())
	assert.Error(t, err)
}

// --- GetOrCreateEncryptionKey deep branches -------------------------------

func TestMockGetOrCreateEncryptionKeyHexDecodeError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT encryption_key FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"encryption_key"}).AddRow("not-hex-Z"))

	_, err := s.GetOrCreateEncryptionKey(context.Background())
	assert.Error(t, err)
}

// --- ExportAllData deep branches ------------------------------------------

func TestMockExportShelfQueryError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).WillReturnError(errBoom)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportContainersQueryError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	mock.ExpectQuery(`SELECT id, col, row FROM container`).WillReturnError(errBoom)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportContainersScanError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	rows := sqlmock.NewRows([]string{"id", "col", "row"}).AddRow(nil, 1, 1)
	mock.ExpectQuery(`SELECT id, col, row FROM container`).WillReturnRows(rows)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportContainersIterateError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	rows := sqlmock.NewRows([]string{"id", "col", "row"}).
		AddRow(int64(1), 1, 1).
		RowError(0, errBoom)
	mock.ExpectQuery(`SELECT id, col, row FROM container`).WillReturnRows(rows)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportItemsQueryError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	mock.ExpectQuery(`SELECT id, col, row FROM container`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}))
	mock.ExpectQuery(`SELECT id, container_id, name, description FROM item`).WillReturnError(errBoom)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportItemsScanError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	mock.ExpectQuery(`SELECT id, col, row FROM container`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}))
	rows := sqlmock.NewRows([]string{"id", "container_id", "name", "description"}).AddRow(nil, int64(1), "n", nil)
	mock.ExpectQuery(`SELECT id, container_id, name, description FROM item`).WillReturnRows(rows)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportItemsIterateError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	mock.ExpectQuery(`SELECT id, col, row FROM container`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}))
	rows := sqlmock.NewRows([]string{"id", "container_id", "name", "description"}).
		AddRow(int64(1), int64(2), "n", nil).
		RowError(0, errBoom)
	mock.ExpectQuery(`SELECT id, container_id, name, description FROM item`).WillReturnRows(rows)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportTagsQueryError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	mock.ExpectQuery(`SELECT id, col, row FROM container`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}))
	mock.ExpectQuery(`SELECT id, container_id, name, description FROM item`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "container_id", "name", "description"}))
	mock.ExpectQuery(`SELECT it.item_id, t.name`).WillReturnError(errBoom)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportTagsScanError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	mock.ExpectQuery(`SELECT id, col, row FROM container`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}))
	mock.ExpectQuery(`SELECT id, container_id, name, description FROM item`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "container_id", "name", "description"}))
	rows := sqlmock.NewRows([]string{"item_id", "name"}).AddRow(nil, "t")
	mock.ExpectQuery(`SELECT it.item_id, t.name`).WillReturnRows(rows)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestMockExportTagsIterateError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectQuery(`SELECT id, name, rows, cols FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rows", "cols"}).AddRow(1, "x", 5, 5))
	mock.ExpectQuery(`SELECT id, col, row FROM container`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "col", "row"}))
	mock.ExpectQuery(`SELECT id, container_id, name, description FROM item`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "container_id", "name", "description"}))
	rows := sqlmock.NewRows([]string{"item_id", "name"}).
		AddRow(int64(1), "t").
		RowError(0, errBoom)
	mock.ExpectQuery(`SELECT it.item_id, t.name`).WillReturnRows(rows)

	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

// --- ImportAllData deep branches ------------------------------------------

func TestMockImportContainerLastInsertIdError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec(reAny).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(reAny).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(reAny).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(reAny).WillReturnResult(sqlmock.NewResult(0, 0)) // delete container
	mock.ExpectExec(`UPDATE shelf`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO container`).WillReturnResult(sqlmock.NewErrorResult(errBoom))
	mock.ExpectRollback()

	err := s.ImportAllData(context.Background(), &model.ExportData{
		Version: 1,
		Shelf: model.ExportShelf{
			Name: "x", Rows: 5, Cols: 5,
			Containers: []model.ExportContainer{{Col: 1, Row: 1}},
		},
	})
	assert.Error(t, err)
}

func TestMockImportItemLastInsertIdError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	for range 4 {
		mock.ExpectExec(reAny).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectExec(`UPDATE shelf`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO container`).WillReturnResult(sqlmock.NewResult(int64(10), 1))
	mock.ExpectExec(`INSERT INTO item`).WillReturnResult(sqlmock.NewErrorResult(errBoom))
	mock.ExpectRollback()

	err := s.ImportAllData(context.Background(), &model.ExportData{
		Version: 1,
		Shelf: model.ExportShelf{
			Name: "x", Rows: 5, Cols: 5,
			Containers: []model.ExportContainer{{Col: 1, Row: 1, Items: []model.ExportItem{{Name: "x"}}}},
		},
	})
	assert.Error(t, err)
}

func TestMockImportTagSelectError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	for range 4 {
		mock.ExpectExec(reAny).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectExec(`UPDATE shelf`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO container`).WillReturnResult(sqlmock.NewResult(int64(10), 1))
	mock.ExpectExec(`INSERT INTO item`).WillReturnResult(sqlmock.NewResult(int64(20), 1))
	mock.ExpectExec(`INSERT OR IGNORE INTO tag`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT id FROM tag WHERE name`).WillReturnError(errBoom)
	mock.ExpectRollback()

	err := s.ImportAllData(context.Background(), &model.ExportData{
		Version: 1,
		Shelf: model.ExportShelf{
			Name: "x", Rows: 5, Cols: 5,
			Containers: []model.ExportContainer{{Col: 1, Row: 1, Items: []model.ExportItem{{Name: "x", Tags: []string{"t1"}}}}},
		},
	})
	assert.Error(t, err)
}

func TestMockImportCommitError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM shelf`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	for range 4 {
		mock.ExpectExec(reAny).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectExec(`UPDATE shelf`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errBoom)

	err := s.ImportAllData(context.Background(), &model.ExportData{
		Version: 1,
		Shelf:   model.ExportShelf{Name: "x", Rows: 5, Cols: 5},
	})
	assert.Error(t, err)
}

// --- SearchItemsBatch deeper branches -------------------------------------

func TestMockSearchItemsBatchIterateError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at", "tag_list"}
	rows := sqlmock.NewRows(cols).
		AddRow(int64(1), int64(1), "n", nil, 1, 1, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", nil).
		RowError(0, errBoom)
	mock.ExpectQuery(reAny).WillReturnRows(rows)

	_, err := s.SearchItemsBatch(context.Background(), "q", nil)
	assert.Error(t, err)
}

func TestMockSearchItemsBatchCountError(t *testing.T) {
	s, mock := newMockStore(t)
	cols := []string{"id", "container_id", "name", "description", "col", "row", "created_at", "updated_at", "tag_list"}
	rs := sqlmock.NewRows(cols)
	for i := range 51 {
		rs = rs.AddRow(int64(i+1), int64(1), "n", nil, 1, 1, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", nil)
	}
	mock.ExpectQuery(reAny).WillReturnRows(rs)
	// Count query fails.
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).WillReturnError(errBoom)

	_, err := s.SearchItemsBatch(context.Background(), "q", nil)
	assert.Error(t, err)
}
