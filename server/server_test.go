package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTempFile creates a temp file with the given content and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test*")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// writeDecisionRecords marshals records to a temp JSON file and returns its path.
func writeDecisionRecords(t *testing.T, records interface{}) string {
	t.Helper()
	data, err := json.Marshal(records)
	require.NoError(t, err)
	return writeTempFile(t, string(data))
}

// newTestServer creates a Server using a temp directory for the decisions file.
func newTestServer(t *testing.T, pairs []Pair) (*Server, string) {
	t.Helper()
	decisionsPath := filepath.Join(t.TempDir(), "decisions.json")
	return New(pairs, decisionsPath), decisionsPath
}

// --- ParsePairFile ---

func TestParsePairFile_Valid(t *testing.T) {
	f := writeTempFile(t, "/a/b.jpg  /c/d.jpg\n/e/f.mp4  /g/h.mp4\n")

	pairs, err := ParsePairFile(f)
	require.NoError(t, err)
	require.Len(t, pairs, 2)

	assert.Equal(t, 0, pairs[0].Index)
	assert.Equal(t, "/a/b.jpg", pairs[0].Left)
	assert.Equal(t, "/c/d.jpg", pairs[0].Right)

	assert.Equal(t, 1, pairs[1].Index)
	assert.Equal(t, "/e/f.mp4", pairs[1].Left)
	assert.Equal(t, "/g/h.mp4", pairs[1].Right)
}

func TestParsePairFile_SkipsBlankLines(t *testing.T) {
	f := writeTempFile(t, "\n/a.jpg  /b.jpg\n\n/c.jpg  /d.jpg\n\n")

	pairs, err := ParsePairFile(f)
	require.NoError(t, err)
	assert.Len(t, pairs, 2)
}

func TestParsePairFile_TabSeparated(t *testing.T) {
	f := writeTempFile(t, "/a.jpg\t/b.jpg\n")

	pairs, err := ParsePairFile(f)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	assert.Equal(t, "/a.jpg", pairs[0].Left)
	assert.Equal(t, "/b.jpg", pairs[0].Right)
}

func TestParsePairFile_MalformedLine(t *testing.T) {
	f := writeTempFile(t, "/only-one-path\n")

	_, err := ParsePairFile(f)
	assert.ErrorContains(t, err, "line 1")
}

func TestParsePairFile_MissingFile(t *testing.T) {
	_, err := ParsePairFile("/nonexistent/pairs.txt")
	assert.Error(t, err)
}

func TestParsePairFile_EmptyFile(t *testing.T) {
	f := writeTempFile(t, "")

	pairs, err := ParsePairFile(f)
	require.NoError(t, err)
	assert.Empty(t, pairs)
}

// --- LoadDecisions ---

func TestLoadDecisions_AppliesMatchingDecisions(t *testing.T) {
	pairs := []Pair{
		{Index: 0, Left: "/a.jpg", Right: "/b.jpg"},
		{Index: 1, Left: "/c.jpg", Right: "/d.jpg"},
	}

	type rec struct {
		Left     string `json:"left"`
		Right    string `json:"right"`
		Decision string `json:"decision"`
	}
	f := writeDecisionRecords(t, []rec{
		{Left: "/a.jpg", Right: "/b.jpg", Decision: "delete_right"},
	})

	require.NoError(t, LoadDecisions(pairs, f))
	assert.Equal(t, "delete_right", pairs[0].Decision)
	assert.Equal(t, "", pairs[1].Decision, "unmatched pair must be untouched")
}

func TestLoadDecisions_MissingFile_ReturnsErrNotExist(t *testing.T) {
	err := LoadDecisions([]Pair{}, "/nonexistent/decisions.json")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadDecisions_InvalidJSON(t *testing.T) {
	f := writeTempFile(t, "not json")
	err := LoadDecisions([]Pair{}, f)
	assert.ErrorContains(t, err, "parsing decisions file")
}

func TestLoadDecisions_EmptyFile(t *testing.T) {
	f := writeDecisionRecords(t, []interface{}{})
	pairs := []Pair{{Index: 0, Left: "/a.jpg", Right: "/b.jpg"}}

	require.NoError(t, LoadDecisions(pairs, f))
	assert.Equal(t, "", pairs[0].Decision)
}

// --- GET /api/pairs ---

func TestHandlePairs_ReturnsAllPairs(t *testing.T) {
	pairs := []Pair{
		{Index: 0, Left: "/a.jpg", Right: "/b.jpg", Decision: "keep_both"},
		{Index: 1, Left: "/c.jpg", Right: "/d.jpg", Decision: ""},
	}
	srv, _ := newTestServer(t, pairs)

	req := httptest.NewRequest(http.MethodGet, "/api/pairs", nil)
	w := httptest.NewRecorder()
	srv.handlePairs(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []Pair
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Len(t, got, 2)
	assert.Equal(t, "keep_both", got[0].Decision)
	assert.Equal(t, "", got[1].Decision)
}

func TestHandlePairs_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/pairs", nil)
	w := httptest.NewRecorder()
	srv.handlePairs(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// --- POST /api/decision ---

func TestHandleDecision_UpdatesPairAndPersists(t *testing.T) {
	pairs := []Pair{{Index: 0, Left: "/a.jpg", Right: "/b.jpg"}}
	srv, decisionsPath := newTestServer(t, pairs)

	body, _ := json.Marshal(decisionRequest{Index: 0, Decision: "delete_right"})
	req := httptest.NewRequest(http.MethodPost, "/api/decision", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleDecision(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "delete_right", srv.pairs[0].Decision)

	data, err := os.ReadFile(decisionsPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "delete_right")
}

func TestHandleDecision_AllValidDecisions(t *testing.T) {
	valid := []string{"delete_left", "delete_right", "keep_both", "unsure", ""}
	for _, d := range valid {
		srv, _ := newTestServer(t, []Pair{{Index: 0, Left: "/a.jpg", Right: "/b.jpg"}})
		body, _ := json.Marshal(decisionRequest{Index: 0, Decision: d})
		req := httptest.NewRequest(http.MethodPost, "/api/decision", bytes.NewReader(body))
		w := httptest.NewRecorder()
		srv.handleDecision(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code, "decision %q should be accepted", d)
	}
}

func TestHandleDecision_InvalidDecision(t *testing.T) {
	srv, _ := newTestServer(t, []Pair{{Index: 0, Left: "/a.jpg", Right: "/b.jpg"}})
	body, _ := json.Marshal(decisionRequest{Index: 0, Decision: "do_something_weird"})
	req := httptest.NewRequest(http.MethodPost, "/api/decision", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleDecision(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleDecision_IndexOutOfRange(t *testing.T) {
	srv, _ := newTestServer(t, []Pair{{Index: 0, Left: "/a.jpg", Right: "/b.jpg"}})
	body, _ := json.Marshal(decisionRequest{Index: 99, Decision: "keep_both"})
	req := httptest.NewRequest(http.MethodPost, "/api/decision", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleDecision(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleDecision_NegativeIndex(t *testing.T) {
	srv, _ := newTestServer(t, []Pair{{Index: 0, Left: "/a.jpg", Right: "/b.jpg"}})
	body, _ := json.Marshal(decisionRequest{Index: -1, Decision: "keep_both"})
	req := httptest.NewRequest(http.MethodPost, "/api/decision", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleDecision(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleDecision_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/decision", nil)
	w := httptest.NewRecorder()
	srv.handleDecision(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleDecision_InvalidBody(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/decision", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.handleDecision(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- GET /api/file ---

func TestHandleFile_ValidPath_ServesContent(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "photo.jpg")
	require.NoError(t, os.WriteFile(tmpFile, []byte("image-data"), 0644))

	srv, _ := newTestServer(t, []Pair{{Index: 0, Left: tmpFile, Right: "/other.jpg"}})

	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+url.QueryEscape(tmpFile), nil)
	w := httptest.NewRecorder()
	srv.handleFile(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "image-data")
}

func TestHandleFile_PathNotInPairList_Forbidden(t *testing.T) {
	srv, _ := newTestServer(t, []Pair{{Index: 0, Left: "/a.jpg", Right: "/b.jpg"}})

	req := httptest.NewRequest(http.MethodGet, "/api/file?path=/etc/passwd", nil)
	w := httptest.NewRecorder()
	srv.handleFile(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandleFile_MissingPathParam(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/file", nil)
	w := httptest.NewRecorder()
	srv.handleFile(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleFile_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/file?path=/a.jpg", nil)
	w := httptest.NewRecorder()
	srv.handleFile(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// --- isValidDecision ---

func TestIsValidDecision(t *testing.T) {
	valid := []string{"delete_left", "delete_right", "keep_both", "unsure", ""}
	for _, d := range valid {
		assert.True(t, isValidDecision(d), "expected %q to be valid", d)
	}
	invalid := []string{"DELETE_RIGHT", "remove", "yes", "no", "maybe"}
	for _, d := range invalid {
		assert.False(t, isValidDecision(d), "expected %q to be invalid", d)
	}
}
