package cmd

import (
	"dedupe/photo"
	"dedupe/tui"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSender captures messages sent through teaMessenger without needing a real terminal.
type mockSender struct {
	msgs []tea.Msg
}

func (m *mockSender) Send(msg tea.Msg) {
	m.msgs = append(m.msgs, msg)
}

func TestTeaMessenger_ProgressTickMsg_TransformsToProgressUpdateMsg(t *testing.T) {
	s := &mockSender{}
	m := teaMessenger{p: s}

	m.Send(photo.ProgressTickMsg{})

	require.Len(t, s.msgs, 1)
	assert.IsType(t, tui.ProgressUpdateMsg{}, s.msgs[0])
}

func TestTeaMessenger_OtherMsg_PassesThrough(t *testing.T) {
	s := &mockSender{}
	m := teaMessenger{p: s}

	type customMsg struct{ val int }
	msg := customMsg{val: 42}
	m.Send(msg)

	require.Len(t, s.msgs, 1)
	assert.Equal(t, msg, s.msgs[0])
}

func TestTeaMessenger_ImplementsPhotoMessenger(t *testing.T) {
	var _ photo.Messenger = teaMessenger{p: &mockSender{}}
}

func TestRunOrganize_InvalidSourceDir(t *testing.T) {
	err := runOrganize(nil, []string{"/no/such/path", t.TempDir()})
	assert.ErrorContains(t, err, "source directory does not exist")
}

func TestRunOrganize_CreatesDestDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "new", "nested", "dest")

	// dst does not exist yet; runOrganize should create it before reaching the TUI.
	// With an empty source dir and no terminal, tea.Program.Run will fail, but
	// the dest dir creation happens before that — so we just check the dir exists.
	_ = runOrganize(nil, []string{src, dst})

	_, err := os.Stat(dst)
	assert.NoError(t, err, "destination directory should have been created")
}
