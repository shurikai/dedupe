package photo

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestNewState tests the NewState function
func TestNewState(t *testing.T) {
	state := NewState(100)

	if state.total != 100 {
		t.Errorf("Expected total to be 100, got %d", state.total)
	}
	if state.processed != 0 {
		t.Errorf("Expected processed to be 0, got %d", state.processed)
	}
	if state.duplicates != 0 {
		t.Errorf("Expected duplicates to be 0, got %d", state.duplicates)
	}
	if state.errorCount != 0 {
		t.Errorf("Expected errorCount to be 0, got %d", state.errorCount)
	}
	if state.message != "Initializing..." {
		t.Errorf("Expected message to be 'Initializing...', got %s", state.message)
	}
}

// TestStateGetters tests the getter methods of the State struct
func TestStateGetters(t *testing.T) {
	state := &State{
		total:      100,
		processed:  50,
		duplicates: 10,
		errorCount: 5,
		message:    "Processing...",
	}

	if state.GetTotalCount() != 100 {
		t.Errorf("Expected GetTotalCount to return 100, got %d", state.GetTotalCount())
	}
	if state.GetProcessedCount() != 50 {
		t.Errorf("Expected GetProcessedCount to return 50, got %d", state.GetProcessedCount())
	}
	if state.GetDuplicateCount() != 10 {
		t.Errorf("Expected GetDuplicateCount to return 10, got %d", state.GetDuplicateCount())
	}
	if state.GetErrorCount() != 5 {
		t.Errorf("Expected GetErrorCount to return 5, got %d", state.GetErrorCount())
	}
}

// TestStateIncrementMethods tests the increment methods of the State struct
func TestStateIncrementMethods(t *testing.T) {
	state := NewState(100)

	state.IncrementProcessed()
	if state.GetProcessedCount() != 1 {
		t.Errorf("Expected processed count to be 1, got %d", state.GetProcessedCount())
	}

	state.IncrementDuplicates()
	if state.GetDuplicateCount() != 1 {
		t.Errorf("Expected duplicate count to be 1, got %d", state.GetDuplicateCount())
	}

	state.IncrementError()
	if state.GetErrorCount() != 1 {
		t.Errorf("Expected error count to be 1, got %d", state.GetErrorCount())
	}
}

// TestUpdateMessage tests the UpdateMessage method
func TestUpdateMessage(t *testing.T) {
	state := NewState(100)

	state.UpdateMessage("New message")

	// We need to access the message field directly for testing
	state.mu.RLock()
	defer state.mu.RUnlock()
	if state.message != "New message" {
		t.Errorf("Expected message to be 'New message', got %s", state.message)
	}
}

// TestStatus tests the Status method
func TestStatus(t *testing.T) {
	original := &State{
		total:      100,
		processed:  50,
		duplicates: 10,
		errorCount: 5,
		message:    "Processing...",
	}

	copy := original.Status()

	if copy.total != 100 || copy.processed != 50 || copy.duplicates != 10 ||
		copy.errorCount != 5 || copy.message != "Processing..." {
		t.Errorf("Status did not return a correct copy of the state")
	}

	// Verify that modifying the copy doesn't affect the original
	copy.processed = 60
	if original.processed != 50 {
		t.Errorf("Modifying the copy affected the original state")
	}
}

// TestCountFiles tests the CountFiles function
func TestCountFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-count-files")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some test files
	files := []string{"file1.txt", "file2.jpg", ".hidden.txt", "file3.png"}
	for _, file := range files {
		path := filepath.Join(tempDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create a subdirectory with files
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFiles := []string{"subfile1.txt", "subfile2.jpg"}
	for _, file := range subFiles {
		path := filepath.Join(subDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file in subdirectory: %v", err)
		}
	}

	// Test counting files
	count, err := CountFiles(tempDir)
	if err != nil {
		t.Fatalf("CountFiles returned an error: %v", err)
	}

	// We expect 5 files (3 in root dir excluding the hidden file, 2 in subdir)
	expectedCount := 5
	if count != expectedCount {
		t.Errorf("Expected count to be %d, got %d", expectedCount, count)
	}

	// Test error handling: non-existent directory
	nonExistentDir := filepath.Join(tempDir, "nonexistent")
	_, err = CountFiles(nonExistentDir)
	if err == nil {
		t.Errorf("Expected error for non-existent directory, got nil")
	}
}

// MockMessenger implements the Messenger interface for testing
type MockMessenger struct {
	Messages []interface{}
	mu       sync.Mutex
}

func NewMockMessenger() *MockMessenger {
	return &MockMessenger{
		Messages: make([]interface{}, 0),
	}
}

func (m *MockMessenger) Send(msg interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, msg)
}

// TestIsVideoFile tests the isVideoFile function
func TestIsVideoFile(t *testing.T) {
	tests := []struct {
		extension string
		expected  bool
	}{
		{".mp4", true},
		{".MP4", true}, // Test case insensitivity
		{".avi", true},
		{".mov", true},
		{".mkv", true},
		{".wmv", true},
		{".jpg", false},
		{".jpeg", false},
		{".png", false},
		{".txt", false},
		{"", false},
	}

	for _, test := range tests {
		result := isVideoFile(test.extension)
		if result != test.expected {
			t.Errorf("isVideoFile(%s) = %v, expected %v", test.extension, result, test.expected)
		}
	}
}

// TestResolveNamingConflict tests the resolveNamingConflict function
func TestResolveNamingConflict(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-resolve-naming")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test resolving naming conflict
	resolvedPath := resolveNamingConflict(testFile)
	expectedPath := filepath.Join(tempDir, "test_1.txt")

	if resolvedPath != expectedPath {
		t.Errorf("Expected resolved path to be %s, got %s", expectedPath, resolvedPath)
	}

	// Create the first resolved file and test again
	if err := os.WriteFile(expectedPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create resolved test file: %v", err)
	}

	resolvedPath = resolveNamingConflict(testFile)
	expectedPath = filepath.Join(tempDir, "test_2.txt")

	if resolvedPath != expectedPath {
		t.Errorf("Expected second resolved path to be %s, got %s", expectedPath, resolvedPath)
	}
}

// TestCalculateChecksum tests the calculateChecksum function
func TestCalculateChecksum(t *testing.T) {
	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "test-checksum-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write test data to the file
	testData := "Hello, world!"
	if _, err := tempFile.WriteString(testData); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	// Reset file pointer to beginning
	if _, err := tempFile.Seek(0, 0); err != nil {
		t.Fatalf("Failed to reset file pointer: %v", err)
	}

	// Calculate checksum
	checksum, err := calculateChecksum(tempFile)
	if err != nil {
		t.Fatalf("calculateChecksum returned an error: %v", err)
	}

	// Expected MD5 checksum for "Hello, world!"
	expectedChecksum := "6cd3556deb0da54bca060b4c39479839"

	if checksum != expectedChecksum {
		t.Errorf("Expected checksum to be %s, got %s", expectedChecksum, checksum)
	}
}

// TestCopyFile tests the copyFile function
func TestCopyFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-copy-file")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a source file
	srcPath := filepath.Join(tempDir, "source.txt")
	srcContent := "Test content for copy file"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Define destination path
	destPath := filepath.Join(tempDir, "destination.txt")

	// Test copying the file
	if err := copyFile(srcPath, destPath); err != nil {
		t.Fatalf("copyFile returned an error: %v", err)
	}

	// Verify the destination file exists and has the correct content
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(destContent) != srcContent {
		t.Errorf("Destination file content doesn't match source. Expected %q, got %q", srcContent, string(destContent))
	}

	// Verify the permissions were copied correctly
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("Failed to get source file info: %v", err)
	}

	destInfo, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to get destination file info: %v", err)
	}

	if srcInfo.Mode() != destInfo.Mode() {
		t.Errorf("Destination file mode doesn't match source. Expected %v, got %v", srcInfo.Mode(), destInfo.Mode())
	}

	// Test error handling: non-existent source file
	nonExistentSrc := filepath.Join(tempDir, "nonexistent.txt")
	err = copyFile(nonExistentSrc, destPath)
	if err == nil {
		t.Errorf("Expected error for non-existent source file, got nil")
	}

	// Test error handling: invalid destination path
	invalidDestDir := filepath.Join(tempDir, "nonexistent-dir")
	invalidDestPath := filepath.Join(invalidDestDir, "invalid.txt")
	err = copyFile(srcPath, invalidDestPath)
	if err == nil {
		t.Errorf("Expected error for invalid destination path, got nil")
	}
}

// TestMoveFile tests the moveFile function
func TestMoveFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-move-file")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a source file
	srcPath := filepath.Join(tempDir, "source.txt")
	srcContent := "Test content for move file"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Define destination path
	destPath := filepath.Join(tempDir, "destination.txt")

	// Test moving the file
	if err := moveFile(srcPath, destPath); err != nil {
		t.Fatalf("moveFile returned an error: %v", err)
	}

	// Verify the source file no longer exists
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Errorf("Source file still exists after move")
	}

	// Verify the destination file exists and has the correct content
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(destContent) != srcContent {
		t.Errorf("Destination file content doesn't match source. Expected %q, got %q", srcContent, string(destContent))
	}

	// Test error handling: non-existent source file
	nonExistentSrc := filepath.Join(tempDir, "nonexistent.txt")
	err = moveFile(nonExistentSrc, destPath)
	if err == nil {
		t.Errorf("Expected error for non-existent source file, got nil")
	}

	// Test error handling: moving to a non-existent directory
	nonExistentDir := filepath.Join(tempDir, "nonexistent-dir")
	nonExistentDestPath := filepath.Join(nonExistentDir, "file.txt")

	// Create a new source file
	newSrcPath := filepath.Join(tempDir, "new-source.txt")
	if err := os.WriteFile(newSrcPath, []byte("New source content"), 0644); err != nil {
		t.Fatalf("Failed to create new source file: %v", err)
	}

	// Try to move it to a non-existent directory
	err = moveFile(newSrcPath, nonExistentDestPath)
	if err == nil {
		t.Errorf("Expected error when moving to non-existent directory, got nil")
	}

	// Verify the source file still exists (move failed)
	if _, err := os.Stat(newSrcPath); os.IsNotExist(err) {
		t.Errorf("Source file was removed even though move failed")
	}
}

// TestGetPhotoCreationDate tests the getPhotoCreationDate function
func TestGetPhotoCreationDate(t *testing.T) {
	// This is a simplified test since we can't easily create EXIF data
	// We'll test the case where no EXIF data is found

	// Create a temporary file without EXIF data
	tempFile, err := os.CreateTemp("", "test-photo-*.jpg")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write some non-EXIF data
	if _, err := tempFile.WriteString("This is not a valid JPEG file with EXIF data"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	// Reset file pointer to beginning
	if _, err := tempFile.Seek(0, 0); err != nil {
		t.Fatalf("Failed to reset file pointer: %v", err)
	}

	// Test getPhotoCreationDate with a file that has no EXIF data
	var emptyTime time.Time
	date := getPhotoCreationDate(tempFile, emptyTime)

	// Since our test file has no valid EXIF data, we expect an empty time
	if !date.IsZero() {
		t.Errorf("Expected zero time for file without EXIF data, got %v", date)
	}
}

// TestGetVideoCreationDate tests the getVideoCreationDate function
// Note: This test is limited since it depends on the external yami package
func TestGetVideoCreationDate(t *testing.T) {
	// Since we can't easily create a valid video file for testing,
	// and the getVideoCreationDate function depends on an external package (yami),
	// we'll skip this test in normal test runs.
	//t.Skip("Skipping test that requires valid video files and external dependencies")

	// The following code is kept for reference but is skipped during testing
	// Create a temporary file that's not a valid video
	tempFile, err := os.CreateTemp("", "test-video-*.mp4")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write some non-video data
	if _, err := tempFile.WriteString("This is not a valid video file"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	// Close the file so getVideoCreationDate can open it
	tempFile.Close()

	// Test getVideoCreationDate with an invalid video file
	// This should return an error since the file isn't a valid video
	date, err := getVideoCreationDate(tempFile.Name())

	// We expect an error and a zero time
	if err == nil {
		t.Errorf("Expected an error for invalid video file, got nil")
	}

	if !date.IsZero() {
		t.Errorf("Expected zero time for invalid video file, got %v", date)
	}
}

// TestProcessFile tests the processFile function
func TestProcessFile(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "test-process-file")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create subdirectories
	destDir := filepath.Join(tempDir, "dest")
	duplicatesDir := filepath.Join(destDir, "duplicates")
	noDataDir := filepath.Join(destDir, "nodata")

	for _, dir := range []string{destDir, duplicatesDir, noDataDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := "Test content for process file"
	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a log file
	logFilePath := filepath.Join(tempDir, "test.log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	// Initialize state and duplicates map
	state := NewState(1)
	duplicates := make(map[string][]string)
	var mapLock sync.Mutex

	// Process the file
	err = processFile(testFilePath, destDir, duplicatesDir, noDataDir, duplicates, &mapLock, logFile, state)
	if err != nil {
		t.Fatalf("processFile returned an error: %v", err)
	}

	// Verify the file was processed correctly
	// Since our test file has no date metadata, it should be copied to the nodata directory
	expectedPath := filepath.Join(noDataDir, "test.txt")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file to be copied to %s, but it doesn't exist", expectedPath)
	}

	// Verify the content was copied correctly
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read processed file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Processed file content doesn't match original. Expected %q, got %q", testContent, string(content))
	}

	// Test duplicate detection
	// First, calculate the checksum of our test file
	file, err := os.Open(testFilePath)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	checksum, err := calculateChecksum(file)
	file.Close()
	if err != nil {
		t.Fatalf("Failed to calculate checksum: %v", err)
	}

	// Create a duplicate file
	duplicateFilePath := filepath.Join(tempDir, "duplicate.txt")
	if err := os.WriteFile(duplicateFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create duplicate file: %v", err)
	}

	// Add the original file's path to the duplicates map
	duplicates[checksum] = []string{expectedPath}

	// Process the duplicate file
	err = processFile(duplicateFilePath, destDir, duplicatesDir, noDataDir, duplicates, &mapLock, logFile, state)
	if err != nil {
		t.Fatalf("processFile returned an error for duplicate: %v", err)
	}

	// Verify the duplicate was detected and copied to the duplicates directory
	expectedDuplicatePath := filepath.Join(duplicatesDir, "duplicate.txt")
	if _, err := os.Stat(expectedDuplicatePath); os.IsNotExist(err) {
		t.Errorf("Expected duplicate to be copied to %s, but it doesn't exist", expectedDuplicatePath)
	}

	// Verify the duplicate count was incremented
	if state.GetDuplicateCount() != 1 {
		t.Errorf("Expected duplicate count to be 1, got %d", state.GetDuplicateCount())
	}
}

// TestProcessFiles tests the ProcessFiles function
func TestProcessFiles(t *testing.T) {
	// This test is more complex and might be flaky due to concurrency and file system interactions
	// We'll focus on testing the basic functionality and error handling

	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "test-process-files")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source and destination directories
	srcDir := filepath.Join(tempDir, "src")
	destDir := filepath.Join(tempDir, "dest")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create destination directory: %v", err)
	}

	// Create test files in the source directory
	fileContents := []struct {
		name    string
		content string
	}{
		{"file1.txt", "Content of file 1"},
		{"file2.txt", "Content of file 2"},
		{"file3.txt", "Content of file 3"},
		{"duplicate.txt", "Content of file 1"}, // Duplicate of file1.txt
	}

	for _, fc := range fileContents {
		filePath := filepath.Join(srcDir, fc.name)
		if err := os.WriteFile(filePath, []byte(fc.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", fc.name, err)
		}
	}

	// Create a log file
	logFilePath := filepath.Join(tempDir, "test.log")

	// Initialize state
	totalFiles, err := CountFiles(srcDir)
	if err != nil {
		t.Fatalf("Failed to count files: %v", err)
	}
	state := NewState(totalFiles)

	// Create a mock messenger
	messenger := NewMockMessenger()

	// Process the files
	err = ProcessFiles(srcDir, destDir, logFilePath, state, messenger)
	if err != nil {
		t.Fatalf("ProcessFiles returned an error: %v", err)
	}

	// Verify all files were processed
	if state.GetProcessedCount() != totalFiles {
		t.Errorf("Expected %d processed files, got %d", totalFiles, state.GetProcessedCount())
	}

	// Verify the messenger received messages
	if len(messenger.Messages) == 0 {
		t.Errorf("Expected messenger to receive messages, but none were sent")
	}

	// Verify the required directories were created
	requiredDirs := []string{
		filepath.Join(destDir, "duplicates"),
		filepath.Join(destDir, "nodata"),
	}

	for _, dir := range requiredDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s to exist, but it doesn't", dir)
		}
	}

	// Verify that files were processed and copied somewhere in the destination directory
	// We don't check specific paths because the exact organization depends on file metadata
	err = filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// Found at least one processed file
			return io.EOF // Use EOF as a signal to stop walking
		}
		return nil
	})

	if err != io.EOF {
		t.Errorf("Expected to find at least one processed file in destination directory")
	}
}
