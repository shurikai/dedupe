package tui

import "github.com/charmbracelet/lipgloss"

const (
	rgb_heading        = "#7FB4CA"
	rgb_label          = "#98BB6C"
	rgb_number         = "#E6C384"
	rgb_gradient_left  = "#D27E99"
	rgb_gradient_right = "#957FB8"
	rgb_sakura_pink    = "#D27E99"
	rgb_sumi_ink2      = "#1A1A22"

	//--- lipgloss.Color() constants
	colorHeadingFg = lipgloss.Color(rgb_heading)
	colorLabel     = lipgloss.Color(rgb_label)
	colorNumber    = lipgloss.Color(rgb_number)
	colorStatusBg  = lipgloss.Color(rgb_sumi_ink2)
	colorStatusFg  = lipgloss.Color(rgb_sakura_pink)
)

var (
	controlsStyle = lipgloss.NewStyle().
			Background(colorStatusBg).
			Foreground(colorStatusFg).
			Align(lipgloss.Right)
	progressStyle = lipgloss.NewStyle().
			Align(lipgloss.Center).
			Padding(0, padding)
	headingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorHeadingFg).
			Align(lipgloss.Center)
	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorLabel).
			Align(lipgloss.Right).
			Padding(0, padding)
	numberStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorNumber).
			Align(lipgloss.Right).
			Padding(0, padding)
)
