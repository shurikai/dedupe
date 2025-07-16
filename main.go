package main

import (
	"dedupe/photo"
	"dedupe/tui"
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
	m.p.Send(msg)
}

func main() {
	logFilePath := "duplicates.log"

	if len(os.Args) < 3 {
		fmt.Println("Usage: photo-organizer <source-dir> <dest-dir>")
		os.Exit(1)
	}

	sourceDir := os.Args[1]
	destDir := os.Args[2]

	// Validate directories
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		log.Fatalf("Source directory does not exist: %s", sourceDir)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		log.Fatalf("Cannot create destination directory: %s", err)
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
		// Pass the program 'p' as the Messenger. When processing is done,
		// this goroutine will exit.
		if err := photo.ProcessFiles(sourceDir, destDir, logFilePath, state, messenger); err != nil {
			log.Fatalf("Error processing files: %s", err)
		}
		p.Quit() // Tell the TUI to exit gracefully once processing is complete.
	}()

	// Start the TUI program
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error starting TUI: %s", err)
	}
}
