package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetCleanFlags restores package-level flag variables to their zero values.
func resetCleanFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		cleanExecute = false
		cleanDryRun = false
		cleanMoveTo = ""
		cleanYes = false
	})
}

// writeDecisionsJSON writes a decisions JSON file to a temp path and returns the path.
func writeDecisionsJSON(t *testing.T, entries []decisionEntry) string {
	t.Helper()
	data, err := json.Marshal(entries)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "decisions.json")
	require.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

// --- readDecisionsFile ---

func TestReadDecisionsFile_Valid(t *testing.T) {
	entries := []decisionEntry{
		{Left: "/a.jpg", Right: "/b.jpg", Decision: "delete_right"},
		{Left: "/c.jpg", Right: "/d.jpg", Decision: "keep_both"},
	}
	path := writeDecisionsJSON(t, entries)

	got, err := readDecisionsFile(path)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "delete_right", got[0].Decision)
	assert.Equal(t, "keep_both", got[1].Decision)
}

func TestReadDecisionsFile_Missing(t *testing.T) {
	_, err := readDecisionsFile("/no/such/file.json")
	assert.Error(t, err)
}

func TestReadDecisionsFile_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	_, err := readDecisionsFile(path)
	assert.ErrorContains(t, err, "parsing JSON")
}

func TestReadDecisionsFile_EmptyArray(t *testing.T) {
	path := writeDecisionsJSON(t, []decisionEntry{})

	got, err := readDecisionsFile(path)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// --- runClean: dry-run (default) ---

func TestRunClean_DryRunByDefault_NoFilesDeleted(t *testing.T) {
	resetCleanFlags(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "photo.jpg")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0644))

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: "/unrelated.jpg", Right: target, Decision: "delete_right"},
	})

	err := runClean(nil, []string{path})
	require.NoError(t, err)

	_, statErr := os.Stat(target)
	assert.NoError(t, statErr, "dry-run must not delete files")
}

func TestRunClean_ExplicitDryRunFlag_NoFilesDeleted(t *testing.T) {
	resetCleanFlags(t)
	cleanExecute = true // would normally execute
	cleanDryRun = true  // but dry-run overrides it
	cleanYes = true

	dir := t.TempDir()
	target := filepath.Join(dir, "photo.jpg")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0644))

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: "/other.jpg", Right: target, Decision: "delete_right"},
	})

	require.NoError(t, runClean(nil, []string{path}))

	_, err := os.Stat(target)
	assert.NoError(t, err, "--dry-run must override --execute")
}

// --- runClean: execute ---

func TestRunClean_Execute_DeletesRight(t *testing.T) {
	resetCleanFlags(t)
	cleanExecute = true
	cleanYes = true

	dir := t.TempDir()
	left := filepath.Join(dir, "left.jpg")
	right := filepath.Join(dir, "right.jpg")
	require.NoError(t, os.WriteFile(left, []byte("left"), 0644))
	require.NoError(t, os.WriteFile(right, []byte("right"), 0644))

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: left, Right: right, Decision: "delete_right"},
	})

	require.NoError(t, runClean(nil, []string{path}))

	_, err := os.Stat(left)
	assert.NoError(t, err, "left (canonical) must survive")
	_, err = os.Stat(right)
	assert.True(t, os.IsNotExist(err), "right (marked delete) must be removed")
}

func TestRunClean_Execute_DeletesLeft(t *testing.T) {
	resetCleanFlags(t)
	cleanExecute = true
	cleanYes = true

	dir := t.TempDir()
	left := filepath.Join(dir, "left.jpg")
	right := filepath.Join(dir, "right.jpg")
	require.NoError(t, os.WriteFile(left, []byte("left"), 0644))
	require.NoError(t, os.WriteFile(right, []byte("right"), 0644))

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: left, Right: right, Decision: "delete_left"},
	})

	require.NoError(t, runClean(nil, []string{path}))

	_, err := os.Stat(right)
	assert.NoError(t, err, "right (canonical) must survive")
	_, err = os.Stat(left)
	assert.True(t, os.IsNotExist(err), "left (marked delete) must be removed")
}

func TestRunClean_Execute_KeepBoth_NoFilesDeleted(t *testing.T) {
	resetCleanFlags(t)
	cleanExecute = true
	cleanYes = true

	dir := t.TempDir()
	left := filepath.Join(dir, "left.jpg")
	right := filepath.Join(dir, "right.jpg")
	require.NoError(t, os.WriteFile(left, []byte("left"), 0644))
	require.NoError(t, os.WriteFile(right, []byte("right"), 0644))

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: left, Right: right, Decision: "keep_both"},
	})

	require.NoError(t, runClean(nil, []string{path}))

	_, err := os.Stat(left)
	assert.NoError(t, err)
	_, err = os.Stat(right)
	assert.NoError(t, err)
}

func TestRunClean_Execute_MoveTo(t *testing.T) {
	resetCleanFlags(t)
	cleanExecute = true
	cleanYes = true

	dir := t.TempDir()
	staging := filepath.Join(t.TempDir(), "staging")
	cleanMoveTo = staging

	target := filepath.Join(dir, "photo.jpg")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0644))

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: "/other.jpg", Right: target, Decision: "delete_right"},
	})

	require.NoError(t, runClean(nil, []string{path}))

	_, err := os.Stat(target)
	assert.True(t, os.IsNotExist(err), "original must be moved away")

	_, err = os.Stat(filepath.Join(staging, "photo.jpg"))
	assert.NoError(t, err, "file must appear in staging directory")
}

func TestRunClean_Execute_UnsureEntries_NotActedOn(t *testing.T) {
	resetCleanFlags(t)
	cleanExecute = true
	cleanYes = true

	dir := t.TempDir()
	left := filepath.Join(dir, "left.jpg")
	right := filepath.Join(dir, "right.jpg")
	require.NoError(t, os.WriteFile(left, []byte("left"), 0644))
	require.NoError(t, os.WriteFile(right, []byte("right"), 0644))

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: left, Right: right, Decision: "unsure"},
	})

	require.NoError(t, runClean(nil, []string{path}))

	_, err := os.Stat(left)
	assert.NoError(t, err, "unsure left must not be touched")
	_, err = os.Stat(right)
	assert.NoError(t, err, "unsure right must not be touched")
}

func TestRunClean_Execute_MixedDecisions(t *testing.T) {
	resetCleanFlags(t)
	cleanExecute = true
	cleanYes = true

	dir := t.TempDir()
	files := map[string]string{
		"a_left": filepath.Join(dir, "a_left.jpg"),
		"a_right": filepath.Join(dir, "a_right.jpg"),
		"b_left": filepath.Join(dir, "b_left.jpg"),
		"b_right": filepath.Join(dir, "b_right.jpg"),
		"c_left": filepath.Join(dir, "c_left.jpg"),
		"c_right": filepath.Join(dir, "c_right.jpg"),
	}
	for _, p := range files {
		require.NoError(t, os.WriteFile(p, []byte("data"), 0644))
	}

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: files["a_left"], Right: files["a_right"], Decision: "delete_right"},
		{Left: files["b_left"], Right: files["b_right"], Decision: "keep_both"},
		{Left: files["c_left"], Right: files["c_right"], Decision: "unsure"},
	})

	require.NoError(t, runClean(nil, []string{path}))

	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}
	assert.True(t, exists(files["a_left"]), "a_left (canonical) must survive")
	assert.False(t, exists(files["a_right"]), "a_right (marked) must be deleted")
	assert.True(t, exists(files["b_left"]))
	assert.True(t, exists(files["b_right"]))
	assert.True(t, exists(files["c_left"]))
	assert.True(t, exists(files["c_right"]))
}

func TestRunClean_NothingToDo(t *testing.T) {
	resetCleanFlags(t)
	cleanExecute = true
	cleanYes = true

	path := writeDecisionsJSON(t, []decisionEntry{
		{Left: "/a.jpg", Right: "/b.jpg", Decision: "keep_both"},
		{Left: "/c.jpg", Right: "/d.jpg", Decision: "unsure"},
		{Left: "/e.jpg", Right: "/f.jpg", Decision: ""},
	})

	err := runClean(nil, []string{path})
	assert.NoError(t, err)
}

// --- renameOrCopy / copyFileForMove ---

func TestRenameOrCopy_SameDevice(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.jpg")
	dst := filepath.Join(dir, "dst.jpg")
	require.NoError(t, os.WriteFile(src, []byte("content"), 0644))

	require.NoError(t, renameOrCopy(src, dst))

	_, err := os.Stat(src)
	assert.True(t, os.IsNotExist(err), "source must be gone after move")
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

func TestCopyFileForMove_CopiesContent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	require.NoError(t, os.WriteFile(src, []byte("hello"), 0644))

	require.NoError(t, copyFileForMove(src, dst))

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))

	_, err = os.Stat(src)
	assert.NoError(t, err, "copyFileForMove must not remove source")
}
