package tui

import (
	"dedupe/photo"
	"fmt"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	padding  = 2  // Padding around the progress bar
	maxWidth = 80 // Maximum width of the progress bar
)

var (
	labelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#8839ef"))
	numberStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#e6e9ef"))
	headingStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	itemStyle       = lipgloss.NewStyle().MarginRight(2)
	percentageStyle = lipgloss.NewStyle().Align(lipgloss.Right).Width(10).Foreground(lipgloss.Color("4"))
)

// ProgressProvider defines an interface to fetch progress states.
type ProgressProvider interface {
	GetTotalCount() int
	GetProcessedCount() int
	GetDuplicateCount() int
	GetErrorCount() int // Returns a copy of the current state
}

// DupModel represents the TUI model for this program.
type DupModel struct {
	StatusProvider ProgressProvider
	Progress       progress.Model
	Quitting       bool
}

// New creates and returns a new DupModel with default settings.
func New(provider ProgressProvider) DupModel {
	return DupModel{
		StatusProvider: provider,
		Progress:       progress.New(progress.WithScaledGradient("#ea76cb", "#8839ef")),
		Quitting:       false,
	}
}

// Init is called when the program starts. It kicks off the ticking updates.
// We no longer need a timer, so we return nil.
func (m DupModel) Init() tea.Cmd {
	return nil
}

// Update processes messages and handles updates.
func (m DupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg: // Handle keypress events
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.Quitting = true
			return m, tea.Quit
		}

	// This is our new message case. It's triggered by the photo organizer
	// each time a file is processed.
	case photo.ProgressTickMsg:
		cmd := m.Progress.SetPercent(float64(m.StatusProvider.GetProcessedCount()) / float64(m.StatusProvider.GetTotalCount()))
		return m, cmd

	case tea.WindowSizeMsg:
		m.Progress.Width = msg.Width - padding*2 - 4
		if m.Progress.Width > maxWidth {
			m.Progress.Width = maxWidth
		}
		return m, nil

	case progress.FrameMsg:
		progressModel, cmd := m.Progress.Update(msg)
		m.Progress = progressModel.(progress.Model)
		return m, cmd

	}
	return m, nil
}

// View renders the TUI output.
func (m DupModel) View() string {
	if m.Quitting {
		return "Bye!\n"
	}

	// Fetch the current state
	if m.StatusProvider.GetTotalCount() == 0 {
		return "Initializing and counting files..."
	}

	state := m.StatusProvider
	progressRatio := float64(state.GetProcessedCount()) / float64(state.GetTotalCount())

	progressBar := m.Progress.ViewAs(progressRatio)

	stats := []string{
		fmt.Sprintf("%s%s", itemStyle.Render("Processed:"), percentageStyle.Render(fmt.Sprintf("%d / %d", state.GetProcessedCount(), state.GetTotalCount()))),
		fmt.Sprintf("%s%s", itemStyle.Render("Errors:"), percentageStyle.Render(fmt.Sprintf("%d", state.GetErrorCount()))),
		fmt.Sprintf("%s%s", itemStyle.Render("Duplicates:"), percentageStyle.Render(fmt.Sprintf("%d", state.GetDuplicateCount()))),
	}

	statsOutput := lipgloss.JoinVertical(lipgloss.Left, stats...)

	bottomOutput := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(maxWidth).
		Render(fmt.Sprintf("%s", progressBar))

	view := lipgloss.JoinVertical(lipgloss.Center, headingStyle.Render("Photo Organizer"), statsOutput, "\n\n", bottomOutput)

	if state.GetProcessedCount() >= state.GetTotalCount() {
		return view + "\n\nProcessing complete! Shutting down...\n"
	}

	return view + "\n\nPress 'q' to quit."
}
