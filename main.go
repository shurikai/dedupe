package main

import (
	"dedupe/photo"
	"dedupe/tui"
	"flag"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"log"
	"os"
)

// teaMessenger is an adapter that allows a tea.Program to be used as a photo.Messenger.
// This keeps the photo package decoupled from the tea package.
type teaMessenger struct {
	p *tea.Program
}

func (m teaMessenger) Send(msg interface{}) {
	// If the message is a photo.ProgressTickMsg, transform it into a tui.ProgressUpdateMsg
	// This decouples the tui package from directly knowing about photo package's internal messages.
	if _, ok := msg.(photo.ProgressTickMsg); ok {
		m.p.Send(tui.ProgressUpdateMsg{})
	} else {
		// For other messages, send them as is.
		m.p.Send(msg)
	}
}

func main() {
	// Define command-line flags
	moveFiles := flag.Bool("move", false, "Move files instead of copying them.")
	logFile := flag.String("log", "duplicates.log", "Specify the log file location and name.")

	flag.Usage = func() {
		fmt.Println("Usage: dedupe [options] <source-dir> <dest-dir>")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Ensure the required positional arguments are present
	args := flag.Args()
	if len(args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	sourceDir := args[0]
	destDir := args[1]

	// Validate directories
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		log.Fatalf("Source directory does not exist: %s", sourceDir)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		log.Fatalf("Cannot create destination directory: %s", destDir)
	}

	// Calculate total files in the source directory
	totalFiles, err := photo.CountFiles(sourceDir)
	if err != nil {
		log.Fatalf("Error counting files: %s", err)
	}

	// Initialize progress state
	state := photo.NewState(totalFiles)

	// Start the TUI
	p := tea.NewProgram(tui.New(state))

	// Create the messenger adapter.
	messenger := teaMessenger{p: p}

	// Process files asynchronously
	go func() {
		options := photo.Options{
			MoveFiles: *moveFiles,
		}
		if err := photo.ProcessFiles(sourceDir, destDir, *logFile, state, messenger, options); err != nil {
			log.Fatalf("Error processing files: %s", err)
		}
		//p.Quit() // Tell the TUI to exit gracefully once processing is complete.
	}()

	// Start the TUI program
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error starting TUI: %s", err)
	}
}
