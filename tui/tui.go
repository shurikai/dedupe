package tui

import (
	"dedupe/photo"
	"fmt"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

const (
	padding = 2 // Padding around the progress bar
)

// ProgressProvider defines an interface to fetch progress states.
type ProgressProvider interface {
	GetTotalCount() int
	GetProcessedCount() int
	GetDuplicateCount() int
	GetErrorCount() int
	GetUniqueFileCount() int
	GetNoDataCount() int // Returns a copy of the current state
	GetMessage() string
	UpdateMessage(string)
}

type HeaderProps struct {
	Heading string
	Width   int
}

// DupModel represents the TUI model for this program.
type DupModel struct {
	StatusProvider ProgressProvider
	Progress       progress.Model
	Quitting       bool
	width          int
}

// New creates and returns a new DupModel with default settings.
func New(provider ProgressProvider) DupModel {
	return DupModel{
		StatusProvider: provider,
		Progress:       progress.New(progress.WithScaledGradient(rgb_gradient_left, rgb_gradient_right)),
		Quitting:       false,
		width:          40,
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
		m.width = msg.Width
		m.Progress.Width = msg.Width - padding*2 // - 4
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

	doc := &strings.Builder{}

	// Fetch the current state
	if m.StatusProvider.GetTotalCount() == 0 {
		return "Initializing and counting files..."
	}

	state := m.StatusProvider

	progressRatio := float64(state.GetProcessedCount()) / float64(state.GetTotalCount())
	m.Progress.ShowPercentage = false
	progressBar := m.Progress.ViewAs(progressRatio)

	RenderHeader(doc, HeaderProps{
		Heading: "Photo Organizer",
		Width:   m.width,
	})
	RenderStatsList(doc, state, m.width)
	doc.WriteString("\n\n")

	doc.WriteString(progressStyle.Render(fmt.Sprintf("%s", progressBar)))
	doc.WriteString("\n\n")

	doc.WriteString(controlsStyle.Width(m.width).PaddingRight(padding).Render("Press ctrl+c or 'q' to quit."))
	doc.WriteString("\n")

	return doc.String()
}

func RenderStatsList(doc *strings.Builder, progress ProgressProvider, width int) {
	cWidth := width / 2
	label := labelStyle.Width(cWidth)
	count := fmt.Sprintf("%d", progress.GetTotalCount())
	number := numberStyle.Width(lipgloss.Width(count) + padding*2)

	rowLabels := []string{
		label.Render("Total Files:"),
		label.Render("Processed:"),
		label.Render("Unique Files:"),
		label.Render("No Date:"),
		label.Render("Duplicates:"),
		label.Render("Errors:"),
	}

	stats := []string{
		number.Render(count),
		number.Render(fmt.Sprintf("%d", progress.GetProcessedCount())),
		number.Render(fmt.Sprintf("%d", progress.GetUniqueFileCount())),
		number.Render(fmt.Sprintf("%d", progress.GetNoDataCount())),
		number.Render(fmt.Sprintf("%d", progress.GetDuplicateCount())),
		number.Render(fmt.Sprintf("%d", progress.GetErrorCount())),
	}

	doc.WriteString(
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, rowLabels...),
			lipgloss.JoinVertical(lipgloss.Right, stats...),
		),
	)
}

func RenderHeader(doc *strings.Builder, props HeaderProps) {
	headerStyle := headingStyle.Width(props.Width)
	doc.WriteString("\n\n")
	doc.WriteString(headerStyle.Render(props.Heading))
	doc.WriteString("\n\n")
}
