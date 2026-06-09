package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

//go:embed assets/index.html
var indexHTML []byte

// Pair holds two file paths to compare and the recorded decision.
type Pair struct {
	Index    int    `json:"index"`
	Left     string `json:"left"`
	Right    string `json:"right"`
	Decision string `json:"decision"`
}

type decisionRequest struct {
	Index    int    `json:"index"`
	Decision string `json:"decision"`
}

// Server is an HTTP server that serves the review UI and JSON API.
type Server struct {
	mu            sync.Mutex
	pairs         []Pair
	decisionsPath string
	validPaths    map[string]bool // read-only after New
	mux           *http.ServeMux
}

// New creates a Server loaded with the given pairs. Decisions are persisted to decisionsPath on every change.
func New(pairs []Pair, decisionsPath string) *Server {
	validPaths := make(map[string]bool, len(pairs)*2)
	for _, p := range pairs {
		validPaths[p.Left] = true
		validPaths[p.Right] = true
	}

	s := &Server{
		pairs:         pairs,
		decisionsPath: decisionsPath,
		validPaths:    validPaths,
		mux:           http.NewServeMux(),
	}

	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/pairs", s.handlePairs)
	s.mux.HandleFunc("/api/decision", s.handleDecision)
	s.mux.HandleFunc("/api/file", s.handleFile)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func (s *Server) handlePairs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	snapshot := make([]Pair, len(s.pairs))
	copy(snapshot, s.pairs)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req decisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Index < 0 || req.Index >= len(s.pairs) {
		http.Error(w, "index out of range", http.StatusBadRequest)
		return
	}

	if !isValidDecision(req.Decision) {
		http.Error(w, "invalid decision value", http.StatusBadRequest)
		return
	}

	s.pairs[req.Index].Decision = req.Decision

	if err := s.writeDecisionsLocked(); err != nil {
		http.Error(w, "failed to save decision", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path parameter required", http.StatusBadRequest)
		return
	}

	// validPaths is read-only after New, no lock needed
	if !s.validPaths[path] {
		http.Error(w, "file not in pair list", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, path)
}

// writeDecisionsLocked persists the current decisions to disk. Caller must hold s.mu.
func (s *Server) writeDecisionsLocked() error {
	type record struct {
		Left     string `json:"left"`
		Right    string `json:"right"`
		Decision string `json:"decision"`
	}

	records := make([]record, len(s.pairs))
	for i, p := range s.pairs {
		records[i] = record{Left: p.Left, Right: p.Right, Decision: p.Decision}
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.decisionsPath, data, 0644)
}

// ParsePairFile reads a whitespace-delimited pair file and returns the parsed pairs.
// Each non-empty line must contain at least two fields (left path, right path).
func ParsePairFile(path string) ([]Pair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pair file: %w", err)
	}

	var pairs []Pair
	for lineNum, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("line %d: expected two paths, got %q", lineNum+1, line)
		}
		pairs = append(pairs, Pair{
			Index: len(pairs),
			Left:  fields[0],
			Right: fields[1],
		})
	}
	return pairs, nil
}

// LoadDecisions reads a decisions file and applies the recorded decisions to pairs in-place.
// Returns os.ErrNotExist (unwrapped via errors.Is) if the file does not exist.
func LoadDecisions(pairs []Pair, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	type record struct {
		Left     string `json:"left"`
		Right    string `json:"right"`
		Decision string `json:"decision"`
	}

	var records []record
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("parsing decisions file: %w", err)
	}

	byKey := make(map[string]string, len(records))
	for _, r := range records {
		byKey[r.Left+"|"+r.Right] = r.Decision
	}

	for i := range pairs {
		if d, ok := byKey[pairs[i].Left+"|"+pairs[i].Right]; ok {
			pairs[i].Decision = d
		}
	}

	return nil
}

func isValidDecision(d string) bool {
	switch d {
	case "delete_left", "delete_right", "keep_both", "unsure", "":
		return true
	}
	return false
}
