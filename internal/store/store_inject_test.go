package store

import (
	"context"
	"fmt"
	"screws-box/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failTrigger installs a BEFORE-{op} trigger on the given table that aborts
// the operation with RAISE(FAIL). Subsequent INSERT/UPDATE/DELETE matching
// the trigger fail with a SQLite constraint error, hitting the corresponding
// `if err != nil` branch in the production code.
func failTrigger(t *testing.T, s *Store, op, table string) {
	t.Helper()
	stmt := fmt.Sprintf( //nolint:gosec // G201: op/table are test-only literals, not user input
		"CREATE TRIGGER fail_%s_%s BEFORE %s ON %s BEGIN SELECT RAISE(FAIL, 'injected'); END;",
		op, table, op, table,
	)
	_, err := s.conn.Exec(stmt)
	require.NoError(t, err, "install trigger")
}

// seedOneItem creates a single item with one tag in container (1,1) and returns its ID.
// Used as setup so functions reach their inner branches before the injected fault fires.
func seedOneItem(t *testing.T, s *Store, tag string) int64 {
	t.Helper()
	cID, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	resp, err := s.CreateItem(context.Background(), cID, "x", nil, []string{tag})
	require.NoError(t, err)
	return resp.ID
}

// --- CreateItem deep branches ---------------------------------------------

func TestCreateItemInsertItemFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "INSERT", "item")
	cID, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	_, err = s.CreateItem(context.Background(), cID, "x", nil, nil)
	assert.Error(t, err)
}

func TestCreateItemInsertTagFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "INSERT", "tag")
	cID, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	_, err = s.CreateItem(context.Background(), cID, "x", nil, []string{"newtag"})
	assert.Error(t, err)
}

func TestCreateItemInsertItemTagFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "INSERT", "item_tag")
	cID, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	_, err = s.CreateItem(context.Background(), cID, "x", nil, []string{"newtag"})
	assert.Error(t, err)
}

// --- AddTagToItem deep branches -------------------------------------------

func TestAddTagToItemInsertTagFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "existing")
	failTrigger(t, s, "INSERT", "tag")
	_, err := s.AddTagToItem(context.Background(), itemID, "brand_new")
	assert.Error(t, err)
}

func TestAddTagToItemInsertItemTagFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "a")
	failTrigger(t, s, "INSERT", "item_tag")
	_, err := s.AddTagToItem(context.Background(), itemID, "b")
	assert.Error(t, err)
}

func TestAddTagToItemUpdateItemFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "a")
	failTrigger(t, s, "UPDATE", "item")
	_, err := s.AddTagToItem(context.Background(), itemID, "b")
	assert.Error(t, err)
}

// --- UpdateItem / DeleteItem deep branches --------------------------------

func TestUpdateItemExecFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "t")
	cID, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	failTrigger(t, s, "UPDATE", "item")
	_, err = s.UpdateItem(context.Background(), itemID, "y", nil, cID)
	assert.Error(t, err)
}

func TestDeleteItemExecFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "t")
	failTrigger(t, s, "DELETE", "item")
	err := s.DeleteItem(context.Background(), itemID)
	assert.Error(t, err)
}

// --- RemoveTagFromItem deep branches --------------------------------------

func TestRemoveTagFromItemDeleteFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "tagx")
	failTrigger(t, s, "DELETE", "item_tag")
	err := s.RemoveTagFromItem(context.Background(), itemID, "tagx")
	assert.Error(t, err)
}

// Item exists, tag exists, but no association → "tag not associated with item" branch.
func TestRemoveTagFromItemNotAssociated(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "have")
	// Insert a second tag without associating it to the item.
	_, err := s.conn.Exec("INSERT INTO tag (name, created_at, updated_at) VALUES ('orphan', datetime('now'), datetime('now'))")
	require.NoError(t, err)
	err = s.RemoveTagFromItem(context.Background(), itemID, "orphan")
	assert.Error(t, err)
}

func TestRemoveTagFromItemUpdateItemFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "tagx")
	failTrigger(t, s, "UPDATE", "item")
	err := s.RemoveTagFromItem(context.Background(), itemID, "tagx")
	assert.Error(t, err)
}

// --- RenameTag / DeleteUnusedTag deep branches ----------------------------

func TestRenameTagUpdateRowsAffected_NoOp(t *testing.T) {
	// Rename a missing tag with no constraint conflict — exec succeeds, rows affected = 0.
	// Already covered by TestRenameTagMissingID, but exercise the path explicitly.
	s := openTestStore(t)
	err := s.RenameTag(context.Background(), 12345, "fresh")
	assert.Error(t, err)
}

// --- ResizeShelf deep branches --------------------------------------------

func TestResizeShelfDeleteContainersFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "DELETE", "container")
	// Shrink so out-of-bounds containers exist and need to be deleted.
	_, err := s.ResizeShelf(context.Background(), 3, 3)
	assert.Error(t, err)
}

func TestResizeShelfInsertContainersFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "INSERT", "container")
	// Grow so new containers must be inserted.
	_, err := s.ResizeShelf(context.Background(), 8, 12)
	assert.Error(t, err)
}

func TestResizeShelfUpdateShelfFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "UPDATE", "shelf")
	_, err := s.ResizeShelf(context.Background(), 6, 11)
	assert.Error(t, err)
}

// --- ImportAllData deep branches ------------------------------------------

// Item insert during import must fail when item INSERT trigger fires.
func TestImportAllDataInsertItemFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "INSERT", "item")
	data := &model.ExportData{
		Version: 1,
		Shelf: model.ExportShelf{
			Name: "x", Rows: 5, Cols: 5,
			Containers: []model.ExportContainer{
				{Col: 1, Row: 1, Items: []model.ExportItem{{Name: "x"}}},
			},
		},
	}
	err := s.ImportAllData(context.Background(), data)
	assert.Error(t, err)
}

// Tag insert during import must fail when tag INSERT trigger fires.
func TestImportAllDataInsertTagFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "INSERT", "tag")
	data := &model.ExportData{
		Version: 1,
		Shelf: model.ExportShelf{
			Name: "x", Rows: 5, Cols: 5,
			Containers: []model.ExportContainer{
				{Col: 1, Row: 1, Items: []model.ExportItem{{Name: "x", Tags: []string{"t1"}}}},
			},
		},
	}
	err := s.ImportAllData(context.Background(), data)
	assert.Error(t, err)
}

// Item-tag link insert during import must fail.
func TestImportAllDataLinkItemTagFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "INSERT", "item_tag")
	data := &model.ExportData{
		Version: 1,
		Shelf: model.ExportShelf{
			Name: "x", Rows: 5, Cols: 5,
			Containers: []model.ExportContainer{
				{Col: 1, Row: 1, Items: []model.ExportItem{{Name: "x", Tags: []string{"t1"}}}},
			},
		},
	}
	err := s.ImportAllData(context.Background(), data)
	assert.Error(t, err)
}

// Update shelf during import must fail.
func TestImportAllDataUpdateShelfFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "UPDATE", "shelf")
	data := &model.ExportData{
		Version: 1,
		Shelf:   model.ExportShelf{Name: "x", Rows: 5, Cols: 5},
	}
	err := s.ImportAllData(context.Background(), data)
	assert.Error(t, err)
}

// Delete during import (the FK-order cleanup at the start) must fail.
func TestImportAllDataInitialDeleteFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "DELETE", "container")
	data := &model.ExportData{
		Version: 1,
		Shelf:   model.ExportShelf{Name: "x", Rows: 5, Cols: 5},
	}
	err := s.ImportAllData(context.Background(), data)
	assert.Error(t, err)
}

// --- MergeTags / SaveOIDCConfig deep branches -----------------------------

// MergeTags second-Exec branch: install DELETE FROM item_tag failure.
func TestMergeTagsDeleteAssociationsFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "src")
	// Add a second tag so we have a target.
	_, err := s.AddTagToItem(context.Background(), itemID, "tgt")
	require.NoError(t, err)
	tags, err := s.ListTags(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, tags, 2)
	failTrigger(t, s, "DELETE", "item_tag")
	err = s.MergeTags(context.Background(), tags[0].ID, tags[1].ID)
	assert.Error(t, err)
}

// MergeTags third-Exec branch: install DELETE FROM tag failure.
func TestMergeTagsDeleteTagFails(t *testing.T) {
	s := openTestStore(t)
	itemID := seedOneItem(t, s, "src")
	_, err := s.AddTagToItem(context.Background(), itemID, "tgt")
	require.NoError(t, err)
	tags, err := s.ListTags(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, tags, 2)
	failTrigger(t, s, "DELETE", "tag")
	err = s.MergeTags(context.Background(), tags[0].ID, tags[1].ID)
	assert.Error(t, err)
}

func TestSaveOIDCConfigUpdateFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "UPDATE", "shelf")
	err := s.SaveOIDCConfig(context.Background(), &model.OIDCConfig{
		IssuerURL: "https://x", ClientID: "c", ClientSecret: "s", DisplayName: "n", Enabled: true,
	})
	assert.Error(t, err)
}

func TestSaveOIDCConfigUpdateNoSecretFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "UPDATE", "shelf")
	err := s.SaveOIDCConfig(context.Background(), &model.OIDCConfig{
		IssuerURL: "https://x", ClientID: "c", DisplayName: "n", Enabled: false,
	})
	assert.Error(t, err)
}

// --- UpdateAuthSettings / UpdateShelfName / DisableAuth deep branches -----

func TestUpdateShelfNameExecFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "UPDATE", "shelf")
	err := s.UpdateShelfName(context.Background(), "new")
	assert.Error(t, err)
}

func TestUpdateAuthSettingsExecFailsWithPassword(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "UPDATE", "shelf")
	err := s.UpdateAuthSettings(context.Background(), &model.AuthSettings{
		Username: "u", Password: "p", Enabled: true,
	})
	assert.Error(t, err)
}

func TestUpdateAuthSettingsExecFailsNoPassword(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "UPDATE", "shelf")
	err := s.UpdateAuthSettings(context.Background(), &model.AuthSettings{
		Username: "u", Enabled: false,
	})
	assert.Error(t, err)
}

func TestDisableAuthExecFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "UPDATE", "shelf")
	err := s.DisableAuth()
	assert.Error(t, err)
}

// --- UpsertOIDCUser deep branch -------------------------------------------

func TestUpsertOIDCUserInsertFails(t *testing.T) {
	s := openTestStore(t)
	failTrigger(t, s, "INSERT", "oidc_user")
	_, err := s.UpsertOIDCUser(context.Background(), &model.OIDCUser{
		Sub: "s", Issuer: "i",
	})
	assert.Error(t, err)
}

// --- GetOrCreateEncryptionKey deep branch ---------------------------------

func TestGetOrCreateEncryptionKeyStoreFails(t *testing.T) {
	s := openTestStore(t)
	// Default seeded shelf has empty encryption_key, so the function will
	// try to UPDATE it. Trigger blocks UPDATE on shelf.
	failTrigger(t, s, "UPDATE", "shelf")
	_, err := s.GetOrCreateEncryptionKey(context.Background())
	assert.Error(t, err)
}
