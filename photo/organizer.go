package photo

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/cajax/yami"
	"github.com/rwcarlsen/goexif/exif"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// State tracks the progress of the photo processing operation.
type State struct {
	mu         sync.RWMutex
	processed  int
	total      int
	message    string
	duplicates int
	errorCount int
}

// NewState initializes and returns a new State.
func NewState(total int) *State {
	return &State{
		total:      total,
		processed:  0,
		message:    "Initializing...",
		duplicates: 0,
		errorCount: 0,
	}
}

func (s *State) GetTotalCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.total
}

func (s *State) GetProcessedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.processed
}

func (s *State) GetDuplicateCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.duplicates
}

func (s *State) GetErrorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errorCount
}

// IncrementProcessed safely increments the count of processed files.
func (s *State) IncrementProcessed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed++
}

// IncrementDuplicates safely increments the count of duplicate files found.
func (s *State) IncrementDuplicates() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.duplicates++
}

// IncrementError safely increments the count of files that failed to process.
func (s *State) IncrementError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorCount++
}

// UpdateMessage updates the current message.
func (s *State) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}

// Status returns a snapshot of the current state as a copy.
func (s *State) Status() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s // Copy the state for immutability
}

// CountFiles calculates the total number of files in the given directory.
func CountFiles(srcDir string) (int, error) {
	total := 0

	// Walk through the directory tree
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") { // Only count files
			total++
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("error counting files: %w", err)
	}

	return total, nil
}

// Messenger defines an interface for sending messages, allowing the photo
// package to remain decoupled from the TUI implementation. *tea.Program
// satisfies this interface.
type Messenger interface {
	Send(interface{})
}

// ProgressTickMsg is an empty message sent to the TUI to signal that it
// should re-render its state.
type ProgressTickMsg struct{}

// ProcessFiles organizes files into a year/month/day directory tree, counting progress and updates state.
func ProcessFiles(srcDir, destDir, logFilePath string, state *State, messenger Messenger) error {
	// Constants for special directories
	duplicatesDir := filepath.Join(destDir, "duplicates")
	noDataDir := filepath.Join(destDir, "nodata")

	// Ensure required directories exist
	if err := os.MkdirAll(duplicatesDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create duplicates directory: %w", err)
	}
	if err := os.MkdirAll(noDataDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create no-data directory: %w", err)
	}

	// Open the log file for duplicates
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	// Shared resources
	duplicates := make(map[string][]string) // checksum -> file paths
	var mapLock sync.Mutex                  // Protects access to `duplicates`

	// Channel for distributing files to workers
	filePathChan := make(chan string)

	// WaitGroup to synchronize workers
	var wg sync.WaitGroup

	// Worker pool
	numWorkers := runtime.NumCPU() // Use the number of CPU cores for the worker pool
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range filePathChan {
				if err := processFile(path, destDir, duplicatesDir, noDataDir, duplicates, &mapLock, logFile, state); err != nil {
					state.IncrementError()
				}
				// A file has been "processed" (attempted), so increment the counter
				// to ensure the progress bar completes.
				state.IncrementProcessed()
				// Notify the TUI that an update is available.
				messenger.Send(ProgressTickMsg{})
			}
		}()
	}

	// Walk the source directory and send file paths to the channel
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") { // Ignore directories and hidden files
			filePathChan <- path // Send the file to workers
		}
		return nil
	})

	// Close the channel after walking the directory
	close(filePathChan)

	// Wait for all workers to finish
	wg.Wait()

	// The calling function (`main`) is responsible for quitting the TUI program.
	return err
}

// processFile handles the processing of a single file, including duplicate detection and organizing into a directory structure.
func processFile(
	path string,
	destDir string,
	duplicatesDir string,
	noDataDir string,
	duplicates map[string][]string,
	mapLock *sync.Mutex,
	logFile *os.File,
	state *State,
) error {
	// Open the file to calculate checksum and extract metadata
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	// Extract the file extension
	extension := strings.ToLower(filepath.Ext(path))

	// Extract the creation date based on the file type
	var date time.Time
	if isVideoFile(extension) {
		// Handle video files
		date, err = getVideoCreationDate(path)
		if err != nil {
			date = time.Time{} // No valid date found
		}
	} else {
		date = getPhotoCreationDate(file, date)
	}

	// Reset the file pointer for reading the checksum
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset file pointer for %s: %w", path, err)
	}

	// Calculate the file checksum for duplicate detection
	checksum, err := calculateChecksum(file)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum for %s: %w", path, err)
	}

	// Lock the shared duplicates map for duplicate operations
	mapLock.Lock()
	defer mapLock.Unlock()

	if paths, exists := duplicates[checksum]; exists {
		// Duplicate file logic
		duplicatePath := filepath.Join(duplicatesDir, filepath.Base(path))
		if _, err := os.Stat(duplicatePath); err == nil {
			duplicatePath = resolveNamingConflict(duplicatePath)
		}
		if err := copyFile(path, duplicatePath); err != nil {
			return fmt.Errorf("failed to copy duplicate file %s: %w", path, err)
		}
		duplicates[checksum] = append(paths, path)
		state.IncrementDuplicates()
		_, _ = logFile.WriteString(fmt.Sprintf("Duplicate detected: %s (duplicate of: %s)\n", path, paths[0]))
		return nil
	}

	// Determine the destination path
	var destPath string
	if date.IsZero() {
		// No valid date: copy to the no-data directory
		destPath = filepath.Join(noDataDir, filepath.Base(path))
		if _, err := os.Stat(destPath); err == nil {
			destPath = resolveNamingConflict(destPath)
		}
	} else {
		// Valid date: organize into YYYY/MM/DD directory structure
		destFolder := filepath.Join(destDir, date.Format("2006"), date.Format("01"), date.Format("02"))
		if err := os.MkdirAll(destFolder, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", destFolder, err)
		}
		destFileName := fmt.Sprintf("%s_%s%s", strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), checksum[:8], filepath.Ext(path))
		destPath = filepath.Join(destFolder, destFileName)
		if _, err := os.Stat(destPath); err == nil {
			destPath = resolveNamingConflict(destPath)
		}
	}

	// Copy the file to its destination
	if err := copyFile(path, destPath); err != nil {
		return fmt.Errorf("failed to copy file %s: %w", path, err)
	}

	// Update duplicates map
	duplicates[checksum] = []string{destPath}
	return nil
}

func getPhotoCreationDate(file *os.File, date time.Time) time.Time {
	// Handle photo files
	x, err := exif.Decode(file)
	if err == nil {
		date, err = x.DateTime()
		if err != nil {
			date = time.Time{} // No valid EXIF date found
		}
	} else {
		date = time.Time{} // EXIF decode failed
	}
	return date
}

// copyFile copies a file from the source path to the destination path.
func copyFile(src, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Perform the file copy
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Ensure the destination file has the same permissions as the source
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}
	if err := os.Chmod(dest, sourceInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set file permissions on destination: %w", err)
	}

	return nil
}

// calculateChecksum computes an MD5 checksum for a file.
func calculateChecksum(file *os.File) (string, error) {
	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// resolveNamingConflict generates a unique filename to resolve collisions.
func resolveNamingConflict(path string) string {
	dir, file := filepath.Split(path)
	ext := filepath.Ext(file)
	base := strings.TrimSuffix(file, ext)

	// Add numeric suffix until the name is unique
	for i := 1; ; i++ {
		newPath := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath // Found a unique name
		}
	}
}

// moveFile moves a file from source to destination, replacing copying and removing logic for simplicity.
func moveFile(src, dest string) error {
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %w", src, dest, err)
	}
	return nil
}

// Helper function to determine if a file is a video
func isVideoFile(extension string) bool {
	videoExtensions := []string{".mp4", ".avi", ".mov", ".mkv", ".wmv"} // Add more extensions as needed
	for _, ext := range videoExtensions {
		if strings.EqualFold(extension, ext) {
			return true
		}
	}
	return false
}

// Helper function to retrieve creation date for video files
func getVideoCreationDate(filePath string) (time.Time, error) {
	if !isVideoFile(filepath.Ext(filePath)) {
		return time.Time{}, fmt.Errorf("not a valid video file: %s", filePath)
	}

	info, err := yami.GetMediaInfo(filePath, 10*time.Second, "--Language=raw")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to retrieve video metadata: %w", err)
	}

	videoTrack := info.GetFirstVideoTrack()
	if videoTrack == nil {
		return time.Time{}, fmt.Errorf("no video track found in file: %s", filePath)
	}

	if videoTrack.TaggedDate == "" {
		return time.Time{}, fmt.Errorf("no tagged date found in video file: %s", filePath)
	}

	// The video creation date can be found in the TaggedDate field, which is formatted slightly differently
	// than the DateTime format. This includes the timezone, so we need to parse it accordingly.
	creationDate, err := time.Parse("2006-01-02 15:04:05 UTC", videoTrack.TaggedDate)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse video creation date: %w", err)
	}

	return creationDate, nil
}
