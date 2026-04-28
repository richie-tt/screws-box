package store

import (
	"context"
	"path/filepath"
	"screws-box/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closedStore returns a Store whose underlying connection is closed.
// Every subsequent DB call hits its first error branch ("sql: database is closed").
func closedStore(t *testing.T) *Store {
	t.Helper()
	s := openTestStore(t)
	require.NoError(t, s.conn.Close())
	return s
}

// canceledCtx returns a context that is already canceled.
func canceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// --- Open / Ping / Close ---------------------------------------------------

func TestOpenBadPath(t *testing.T) {
	s := &Store{}
	// A path under a non-existent parent directory makes sql.Open succeed but
	// db.Ping fail (file cannot be created), exercising the ping branch.
	bogus := filepath.Join(t.TempDir(), "no", "such", "dir", "x.db")
	err := s.Open(bogus)
	assert.Error(t, err)
}

func TestPingClosedDB(t *testing.T) {
	s := closedStore(t)
	assert.Error(t, s.Ping(context.Background()))
}

func TestCloseNilConnIsNoop(t *testing.T) {
	s := &Store{}
	assert.NoError(t, s.Close())
}

// --- Simple read methods on closed DB -------------------------------------

func TestDisableAuthClosedDB(t *testing.T) {
	s := closedStore(t)
	assert.Error(t, s.DisableAuth())
}

func TestGetContainerIDByPositionClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetContainerIDByPosition(1, 1)
	assert.Error(t, err)
}

func TestGetShelfNameClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetShelfName()
	assert.Error(t, err)
}

func TestGetRawAuthRowClosedDB(t *testing.T) {
	s := closedStore(t)
	_, _, _, err := s.GetRawAuthRow()
	assert.Error(t, err)
}

func TestGetGridDataClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetGridData()
	assert.Error(t, err)
}

// --- Item CRUD on closed DB ------------------------------------------------

func TestCreateItemClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.CreateItem(context.Background(), 1, "x", nil, nil)
	assert.Error(t, err)
}

func TestGetItemClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetItem(context.Background(), 1)
	assert.Error(t, err)
}

func TestUpdateItemClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.UpdateItem(context.Background(), 1, "x", nil, 1)
	assert.Error(t, err)
}

func TestDeleteItemClosedDB(t *testing.T) {
	s := closedStore(t)
	assert.Error(t, s.DeleteItem(context.Background(), 1))
}

func TestAddTagToItemClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.AddTagToItem(context.Background(), 1, "tag")
	assert.Error(t, err)
}

func TestRemoveTagFromItemClosedDB(t *testing.T) {
	s := closedStore(t)
	assert.Error(t, s.RemoveTagFromItem(context.Background(), 1, "tag"))
}

func TestListItemsByContainerClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.ListItemsByContainer(context.Background(), 1)
	assert.Error(t, err)
}

func TestListAllItemsClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.ListAllItems(context.Background())
	assert.Error(t, err)
}

// --- Search on closed DB ---------------------------------------------------

func TestSearchItemsClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.SearchItems(context.Background(), "q")
	assert.Error(t, err)
}

func TestSearchItemsByTagsClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.SearchItemsByTags(context.Background(), "q", []string{"a"})
	assert.Error(t, err)
}

func TestSearchItemsBatchClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.SearchItemsBatch(context.Background(), "q", nil)
	assert.Error(t, err)
}

// --- Resize / shelf name on closed DB --------------------------------------

func TestResizeShelfClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.ResizeShelf(context.Background(), 5, 5)
	assert.Error(t, err)
}

func TestUpdateShelfNameClosedDB(t *testing.T) {
	s := closedStore(t)
	assert.Error(t, s.UpdateShelfName(context.Background(), "new"))
}

// --- Auth on closed DB -----------------------------------------------------

func TestGetAuthSettingsClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetAuthSettings(context.Background())
	assert.Error(t, err)
}

func TestUpdateAuthSettingsClosedDBWithPassword(t *testing.T) {
	s := closedStore(t)
	err := s.UpdateAuthSettings(context.Background(), &model.AuthSettings{
		Username: "u", Password: "p", Enabled: true,
	})
	assert.Error(t, err)
}

func TestUpdateAuthSettingsClosedDBNoPassword(t *testing.T) {
	s := closedStore(t)
	err := s.UpdateAuthSettings(context.Background(), &model.AuthSettings{
		Username: "u", Enabled: false,
	})
	assert.Error(t, err)
}

func TestValidateCredentialsClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.ValidateCredentials(context.Background(), "u", "p")
	assert.Error(t, err)
}

// --- Tags on closed DB -----------------------------------------------------

func TestListTagsClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.ListTags(context.Background(), "")
	require.Error(t, err)
	_, err = s.ListTags(context.Background(), "p")
	require.Error(t, err)
}

func TestRenameTagClosedDB(t *testing.T) {
	s := closedStore(t)
	assert.Error(t, s.RenameTag(context.Background(), 1, "new"))
}

func TestMergeTagsClosedDB(t *testing.T) {
	s := closedStore(t)
	assert.Error(t, s.MergeTags(context.Background(), 1, 2))
}

func TestDeleteUnusedTagClosedDB(t *testing.T) {
	s := closedStore(t)
	assert.Error(t, s.DeleteUnusedTag(context.Background(), 1))
}

// --- OIDC on closed DB -----------------------------------------------------

func TestGetOIDCConfigClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetOIDCConfig(context.Background())
	assert.Error(t, err)
}

func TestGetOIDCConfigMaskedClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetOIDCConfigMasked(context.Background())
	assert.Error(t, err)
}

func TestSaveOIDCConfigClosedDBWithSecret(t *testing.T) {
	s := closedStore(t)
	err := s.SaveOIDCConfig(context.Background(), &model.OIDCConfig{
		IssuerURL: "https://x", ClientID: "c", ClientSecret: "s", DisplayName: "n", Enabled: true,
	})
	assert.Error(t, err)
}

func TestSaveOIDCConfigClosedDBNoSecret(t *testing.T) {
	s := closedStore(t)
	err := s.SaveOIDCConfig(context.Background(), &model.OIDCConfig{
		IssuerURL: "https://x", ClientID: "c", DisplayName: "n", Enabled: false,
	})
	assert.Error(t, err)
}

func TestUpsertOIDCUserClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.UpsertOIDCUser(context.Background(), &model.OIDCUser{
		Sub: "s", Issuer: "i",
	})
	assert.Error(t, err)
}

func TestGetOIDCUserBySubClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetOIDCUserBySub(context.Background(), "s", "i")
	assert.Error(t, err)
}

// --- Duplicates / encryption key / export / import on closed DB ----------

func TestFindDuplicatesClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.FindDuplicates(context.Background())
	assert.Error(t, err)
}

func TestGetOrCreateEncryptionKeyClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.GetOrCreateEncryptionKey(context.Background())
	assert.Error(t, err)
}

func TestExportAllDataClosedDB(t *testing.T) {
	s := closedStore(t)
	_, err := s.ExportAllData(context.Background())
	assert.Error(t, err)
}

func TestImportAllDataClosedDB(t *testing.T) {
	s := closedStore(t)
	err := s.ImportAllData(context.Background(), &model.ExportData{Version: 1})
	assert.Error(t, err)
}

// --- Canceled-context tests -----------------------------------------------
// These hit the early ctx-check in QueryRowContext / QueryContext / BeginTx.

func TestCanceledContextErrors(t *testing.T) {
	s := openTestStore(t)
	ctx := canceledCtx()

	cases := []struct {
		name string
		call func() error
	}{
		{"GetItem", func() error { _, err := s.GetItem(ctx, 1); return err }},
		{"ListAllItems", func() error { _, err := s.ListAllItems(ctx); return err }},
		{"SearchItems", func() error { _, err := s.SearchItems(ctx, "x"); return err }},
		{"SearchItemsByTags", func() error {
			_, err := s.SearchItemsByTags(ctx, "x", []string{"t"})
			return err
		}},
		{"SearchItemsBatch", func() error { _, err := s.SearchItemsBatch(ctx, "x", nil); return err }},
		{"ListItemsByContainer", func() error { _, err := s.ListItemsByContainer(ctx, 1); return err }},
		{"ListTagsAll", func() error { _, err := s.ListTags(ctx, ""); return err }},
		{"ListTagsPrefix", func() error { _, err := s.ListTags(ctx, "p"); return err }},
		{"GetAuthSettings", func() error { _, err := s.GetAuthSettings(ctx); return err }},
		{"ValidateCredentials", func() error { _, err := s.ValidateCredentials(ctx, "u", "p"); return err }},
		{"GetOIDCConfig", func() error { _, err := s.GetOIDCConfig(ctx); return err }},
		{"GetOIDCUserBySub", func() error { _, err := s.GetOIDCUserBySub(ctx, "s", "i"); return err }},
		{"FindDuplicates", func() error { _, err := s.FindDuplicates(ctx); return err }},
		{"GetOrCreateEncryptionKey", func() error { _, err := s.GetOrCreateEncryptionKey(ctx); return err }},
		{"ExportAllData", func() error { _, err := s.ExportAllData(ctx); return err }},
		{"ImportAllData", func() error { return s.ImportAllData(ctx, &model.ExportData{Version: 1}) }},
		{"ResizeShelf", func() error { _, err := s.ResizeShelf(ctx, 6, 11); return err }},
		{"CreateItem", func() error { _, err := s.CreateItem(ctx, 1, "x", nil, nil); return err }},
		{"UpdateItem", func() error { _, err := s.UpdateItem(ctx, 1, "x", nil, 1); return err }},
		{"DeleteItem", func() error { return s.DeleteItem(ctx, 1) }},
		{"AddTagToItem", func() error { _, err := s.AddTagToItem(ctx, 1, "t"); return err }},
		{"RemoveTagFromItem", func() error { return s.RemoveTagFromItem(ctx, 1, "t") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, tc.call())
		})
	}
}

// --- Constraint / not-found error branches --------------------------------

// Renaming a tag to an already-used name violates the UNIQUE constraint on tag.name,
// hitting the Exec error branch in RenameTag.
func TestRenameTagDuplicateName(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	c1, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, c1, "i1", nil, []string{"alpha", "beta"})
	require.NoError(t, err)

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	require.Len(t, tags, 2)

	// Try to rename "alpha" -> "beta", which already exists.
	err = s.RenameTag(ctx, tags[0].ID, tags[1].Name)
	assert.Error(t, err, "renaming to a duplicate name should fail")
}

// Deleting a tag that is still in use must return ErrTagInUse.
func TestDeleteUnusedTagInUse(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	c1, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, c1, "i1", nil, []string{"hot"})
	require.NoError(t, err)

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	require.Len(t, tags, 1)

	err = s.DeleteUnusedTag(ctx, tags[0].ID)
	assert.ErrorIs(t, err, ErrTagInUse)
}

// MergeTags with a non-existent target ID hits the FK violation branch.
func TestMergeTagsTargetNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	c1, err := s.GetContainerIDByPosition(1, 1)
	require.NoError(t, err)
	_, err = s.CreateItem(ctx, c1, "i1", nil, []string{"src"})
	require.NoError(t, err)

	tags, err := s.ListTags(ctx, "")
	require.NoError(t, err)
	require.Len(t, tags, 1)
	srcID := tags[0].ID

	err = s.MergeTags(ctx, srcID, 99999)
	assert.Error(t, err)
}

// RenameTag for a nonexistent ID hits the affected-rows == 0 branch.
func TestRenameTagMissingID(t *testing.T) {
	s := openTestStore(t)
	err := s.RenameTag(context.Background(), 99999, "newname")
	assert.Error(t, err)
}

// DeleteUnusedTag for a nonexistent ID hits the not-found branch.
func TestDeleteUnusedTagNotFound(t *testing.T) {
	s := openTestStore(t)
	err := s.DeleteUnusedTag(context.Background(), 99999)
	assert.Error(t, err)
}

// AddTagToItem on a missing item hits the not-found branch.
func TestAddTagToItemMissingItem(t *testing.T) {
	s := openTestStore(t)
	_, err := s.AddTagToItem(context.Background(), 99999, "t")
	assert.Error(t, err)
}

// RemoveTagFromItem on a missing item or tag hits not-found branches.
func TestRemoveTagFromItemNotFound(t *testing.T) {
	s := openTestStore(t)
	err := s.RemoveTagFromItem(context.Background(), 99999, "t")
	assert.Error(t, err)
}

// ImportAllData with version 0 / 2 / 99 hits the unsupported-version branch.
func TestImportAllDataUnsupportedVersionVariants(t *testing.T) {
	s := openTestStore(t)
	for _, v := range []int{0, 2, 99} {
		err := s.ImportAllData(context.Background(), &model.ExportData{Version: v})
		assert.Error(t, err, "version %d should be unsupported", v)
	}
}

// ImportAllData with a duplicate container coord triggers the unique constraint
// inside the insert-container loop, exercising that error branch.
func TestImportAllDataDuplicateContainerFails(t *testing.T) {
	s := openTestStore(t)
	data := &model.ExportData{
		Version: 1,
		Shelf: model.ExportShelf{
			Name: "x", Rows: 5, Cols: 5,
			Containers: []model.ExportContainer{
				{Col: 1, Row: 1, Items: nil},
				{Col: 1, Row: 1, Items: nil}, // duplicate (col,row)
			},
		},
	}
	err := s.ImportAllData(context.Background(), data)
	assert.Error(t, err)
}
