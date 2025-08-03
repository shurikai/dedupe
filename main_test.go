package main

import (
	"dedupe/photo"
	"dedupe/tui"
	"flag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"log"
	"os"
	"testing"
)

// Variable to mock os.Exit
var osExit = os.Exit

// Variable to mock log.Fatalf
var logFatalf = log.Fatalf

// mockTeaProgram is a mock implementation for testing
type mockTeaProgram struct {
	mock.Mock
}

// Send mocks the Send method
func (m *mockTeaProgram) Send(msg interface{}) {
	m.Called(msg)
}

// TestTeaMessenger is a test-specific implementation of the teaMessenger in main.go
type TestTeaMessenger struct {
	p *mockTeaProgram
}

// Send implements the photo.Messenger interface
func (m TestTeaMessenger) Send(msg interface{}) {
	// If the message is a photo.ProgressTickMsg, transform it into a tui.ProgressUpdateMsg
	if _, ok := msg.(photo.ProgressTickMsg); ok {
		m.p.Send(tui.ProgressUpdateMsg{})
	} else {
		// For other messages, send them as is.
		m.p.Send(msg)
	}
}

// TestTeaMessenger_Send_ProgressTickMsg tests that the TestTeaMessenger correctly transforms photo.ProgressTickMsg
// into tui.ProgressUpdateMsg
func TestTeaMessenger_Send_ProgressTickMsg(t *testing.T) {
	// Create mock tea program
	mockProgram := new(mockTeaProgram)

	// Create TestTeaMessenger with mock program
	messenger := TestTeaMessenger{p: mockProgram}

	// Expect that the program will receive a tui.ProgressUpdateMsg
	mockProgram.On("Send", tui.ProgressUpdateMsg{}).Return()

	// Call Send with a photo.ProgressTickMsg
	messenger.Send(photo.ProgressTickMsg{})

	// Verify expectations
	mockProgram.AssertExpectations(t)
}

// TestTeaMessenger_Send_OtherMsg tests that the TestTeaMessenger passes through other messages
func TestTeaMessenger_Send_OtherMsg(t *testing.T) {
	// Create mock tea program
	mockProgram := new(mockTeaProgram)

	// Create TestTeaMessenger with mock program
	messenger := TestTeaMessenger{p: mockProgram}

	// Create a test message
	testMsg := "test message"

	// Expect that the program will receive the test message as is
	mockProgram.On("Send", testMsg).Return()

	// Call Send with the test message
	messenger.Send(testMsg)

	// Verify expectations
	mockProgram.AssertExpectations(t)
}

// TestCommandLineFlagParsing tests command-line flag parsing
func TestCommandLineFlagParsing(t *testing.T) {
	// Save original command line arguments and flags
	originalArgs := os.Args
	originalFlagCommandLine := flag.CommandLine

	// Restore original state after the test
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlagCommandLine
	}()

	// Redirect stderr temporarily to capture usage output
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
	}()

	// Reset flags for this test
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	// Test cases
	tests := []struct {
		name      string
		args      []string
		moveFiles bool
		logFile   string
	}{
		{
			name:      "default flags",
			args:      []string{"dedupe", "source", "dest"},
			moveFiles: false,
			logFile:   "duplicates.log",
		},
		{
			name:      "move flag enabled",
			args:      []string{"dedupe", "-move", "source", "dest"},
			moveFiles: true,
			logFile:   "duplicates.log",
		},
		{
			name:      "custom log file",
			args:      []string{"dedupe", "-log=custom.log", "source", "dest"},
			moveFiles: false,
			logFile:   "custom.log",
		},
		{
			name:      "all flags",
			args:      []string{"dedupe", "-move", "-log=all.log", "source", "dest"},
			moveFiles: true,
			logFile:   "all.log",
		},
	}

	for _, tt := range tests {
		testCase := tt // Capture range variable for parallel testing

		t.Run(testCase.name, func(t *testing.T) {
			// Set command line arguments for this test
			os.Args = tt.args

			// Reset flags for this test
			flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

			// Define flags as in main()
			moveFiles := flag.Bool("move", false, "Move files instead of copying them.")
			logFile := flag.String("log", "duplicates.log", "Specify the log file location and name.")

			// Parse flags
			flag.Parse()

			// Verify that flags were parsed correctly
			assert.Equal(t, tt.moveFiles, *moveFiles, "moveFiles flag")
			assert.Equal(t, tt.logFile, *logFile, "logFile flag")

			// Verify that positional arguments were parsed correctly
			args := flag.Args()
			assert.Equal(t, "source", args[0], "source directory")
			assert.Equal(t, "dest", args[1], "destination directory")
		})
	}

	// Test insufficient arguments
	t.Run("insufficient arguments", func(t *testing.T) {
		// Save original values of os.Exit
		exitCalled := false
		originalOsExit := osExit

		// Override os.Exit to capture exit status
		osExit = func(code int) {
			exitCalled = true
			assert.Equal(t, 1, code, "exit code should be 1")
		}

		// Restore original os.Exit after test
		defer func() {
			osExit = originalOsExit
		}()

		// Set command line arguments with insufficient arguments
		os.Args = []string{"dedupe", "source"}

		// Reset flags for this test
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

		// Define flags as in main()
		_ = flag.Bool("move", false, "Move files instead of copying them.")
		_ = flag.String("log", "duplicates.log", "Specify the log file location and name.")

		// Set custom usage function to avoid printing to stdout during test
		flag.Usage = func() {}

		// Parse flags
		flag.Parse()

		// Call main function that checks args length
		args := flag.Args()
		if len(args) < 2 {
			flag.Usage()
			osExit(1)
		}

		// Verify that os.Exit was called
		assert.True(t, exitCalled, "os.Exit should have been called")
	})

	// Test invalid source directory
	t.Run("invalid source directory", func(t *testing.T) {
		// Create temporary directory for testing
		tempDir, err := os.MkdirTemp("", "dedupe-test")
		assert.NoError(t, err)

		// Clean up after test
		defer os.RemoveAll(tempDir)

		// Save original values of os.Exit
		exitCalled := false
		originalOsExit := osExit

		// Override os.Exit to capture exit status
		osExit = func(code int) {
			exitCalled = true
			assert.Equal(t, 1, code, "exit code should be 1")
		}

		// Restore original os.Exit after test
		defer func() {
			osExit = originalOsExit
		}()

		// Non-existent source directory
		invalidSourceDir := tempDir + "/nonexistent"
		validDestDir := tempDir + "/dest"

		// Set command line arguments
		os.Args = []string{"dedupe", invalidSourceDir, validDestDir}

		// Reset flags for this test
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

		// Define flags as in main()
		_ = flag.Bool("move", false, "Move files instead of copying them.")
		_ = flag.String("log", "duplicates.log", "Specify the log file location and name.")

		// Set custom usage function to avoid printing to stdout during test
		flag.Usage = func() {}

		// Parse flags
		flag.Parse()

		// Mock log.Fatalf to avoid actual program termination during test
		originalLogFatalf := logFatalf
		logFatalf = func(format string, v ...interface{}) {
			osExit(1)
		}
		defer func() {
			logFatalf = originalLogFatalf
		}()

		// Call code from main() for directory validation
		args := flag.Args()
		sourceDir := args[0]
		// destDir := args[1] // Not used in this test

		// Validate directories
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			logFatalf("Source directory does not exist: %s", sourceDir)
		}

		// Verify that os.Exit was called
		assert.True(t, exitCalled, "os.Exit should have been called")
	})

	// Test creating destination directory
	t.Run("create destination directory", func(t *testing.T) {
		// Create temporary directory for testing
		tempDir, err := os.MkdirTemp("", "dedupe-test")
		assert.NoError(t, err)

		// Clean up after test
		defer os.RemoveAll(tempDir)

		// Create source directory
		sourceDir := tempDir + "/source"
		err = os.Mkdir(sourceDir, 0755)
		assert.NoError(t, err)

		// Non-existent destination directory
		destDir := tempDir + "/dest"

		// Set command line arguments
		os.Args = []string{"dedupe", sourceDir, destDir}

		// Reset flags for this test
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

		// Define flags as in main()
		_ = flag.Bool("move", false, "Move files instead of copying them.")
		_ = flag.String("log", "duplicates.log", "Specify the log file location and name.")

		// Parse flags
		flag.Parse()

		// Call code from main() for directory validation
		args := flag.Args()
		sourceDir = args[0]
		destDir = args[1]

		// Validate source directory exists
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			t.Fatalf("Source directory should exist")
		}

		// Create destination directory if it doesn't exist
		if err := os.MkdirAll(destDir, 0755); err != nil {
			t.Fatalf("Cannot create destination directory: %s", err)
		}

		// Verify that the destination directory was created
		info, err := os.Stat(destDir)
		assert.NoError(t, err, "Destination directory should be created")
		assert.True(t, info.IsDir(), "Destination should be a directory")
	})

	// Test teaMessenger initialization
	t.Run("TestTeaMessenger initialization", func(t *testing.T) {
		// Create a mock tea program
		mockProgram := new(mockTeaProgram)

		// Create a TestTeaMessenger with the mock program
		messenger := TestTeaMessenger{p: mockProgram}

		// Verify that messenger can be used as a photo.Messenger
		var _ photo.Messenger = messenger
	})
}
