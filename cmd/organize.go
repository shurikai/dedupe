package cmd

import (
	"dedupe/photo"
	"dedupe/tui"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	moveFiles bool
	logFile   string
	benchmark bool
)

var organizeCmd = &cobra.Command{
	Use:   "organize <source-dir> <dest-dir>",
	Short: "Organize photos into a year/month/day directory structure",
	Args:  cobra.ExactArgs(2),
	RunE:  runOrganize,
}

func init() {
	rootCmd.AddCommand(organizeCmd)
	organizeCmd.Flags().BoolVar(&moveFiles, "move", false, "Move files instead of copying them")
	organizeCmd.Flags().StringVar(&logFile, "log", "duplicates.log", "Log file location")
	organizeCmd.Flags().BoolVar(&benchmark, "benchmark", false, "Capture timing data and append a summary to the log file")
}

// teaSender is satisfied by *tea.Program and allows the messenger to be tested
// without a real terminal.
type teaSender interface {
	Send(tea.Msg)
}

// teaMessenger adapts a teaSender to photo.Messenger, keeping the photo
// package decoupled from the TUI implementation.
type teaMessenger struct {
	p teaSender
}

func (m teaMessenger) Send(msg interface{}) {
	if _, ok := msg.(photo.ProgressTickMsg); ok {
		m.p.Send(tui.ProgressUpdateMsg{})
	} else {
		m.p.Send(msg)
	}
}

func runOrganize(_ *cobra.Command, args []string) error {
	sourceDir := args[0]
	destDir := args[1]

	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", sourceDir)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("cannot create destination directory: %s", destDir)
	}

	state := photo.NewState(0)
	p := tea.NewProgram(tui.New(state))
	messenger := teaMessenger{p: p}

	go func() {
		options := photo.Options{MoveFiles: moveFiles, Benchmark: benchmark}
		if err := photo.ProcessFiles(sourceDir, destDir, logFile, state, messenger, options); err != nil {
			fmt.Fprintf(os.Stderr, "error processing files: %s\n", err)
			p.Quit()
		}
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
