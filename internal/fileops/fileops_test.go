package fileops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pibot/pibot/internal/config"
)

func newTestFileOps(t *testing.T) (*FileOps, string) {
	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	// Override base directory to temp dir
	cfg.FileOps.BaseDirectory = tmpDir

	return NewFileOps(cfg), tmpDir
}

func TestFileOps_EnsureBaseDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "pibot-workspace")

	cfg := config.DefaultConfig()
	cfg.FileOps.BaseDirectory = baseDir

	fops := NewFileOps(cfg)

	err := fops.EnsureBaseDirectory()
	if err != nil {
		t.Fatalf("Failed to ensure base directory: %v", err)
	}

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		t.Error("Base directory was not created")
	}
}

func TestFileOps_WriteAndRead(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	testContent := "Hello, PiBot!"
	testPath := filepath.Join(tmpDir, "test.txt")

	// Write file
	err := fops.Write(testPath, testContent)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Read file
	content, err := fops.Read(testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if content != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, content)
	}
}

func TestFileOps_WriteCreatesParentDirs(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	testPath := filepath.Join(tmpDir, "subdir", "nested", "test.txt")

	err := fops.Write(testPath, "nested content")
	if err != nil {
		t.Fatalf("Failed to write nested file: %v", err)
	}

	content, err := fops.Read(testPath)
	if err != nil {
		t.Fatalf("Failed to read nested file: %v", err)
	}

	if content != "nested content" {
		t.Errorf("Expected 'nested content', got '%s'", content)
	}
}

func TestFileOps_List(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	// Create some files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)

	// List directory
	files, err := fops.List(tmpDir)
	if err != nil {
		t.Fatalf("Failed to list directory: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("Expected 3 items, got %d", len(files))
	}

	// Verify file info
	names := make(map[string]bool)
	for _, f := range files {
		names[f.Name] = true
		if f.Name == "subdir" && !f.IsDir {
			t.Error("Expected subdir to be a directory")
		}
		if f.Name == "file1.txt" && f.IsDir {
			t.Error("Expected file1.txt to be a file")
		}
	}

	if !names["file1.txt"] || !names["file2.txt"] || !names["subdir"] {
		t.Error("Missing expected files in listing")
	}
}

func TestFileOps_ListEmptyDir(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	files, err := fops.List(tmpDir)
	if err != nil {
		t.Fatalf("Failed to list empty directory: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 items in empty dir, got %d", len(files))
	}
}

func TestFileOps_ListDefaultPath(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	// Create a file
	os.WriteFile(filepath.Join(tmpDir, "default.txt"), []byte("test"), 0644)

	// List with empty path (should use base directory)
	files, err := fops.List("")
	if err != nil {
		t.Fatalf("Failed to list with empty path: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 item, got %d", len(files))
	}
}

func TestFileOps_Delete(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	testPath := filepath.Join(tmpDir, "to-delete.txt")
	os.WriteFile(testPath, []byte("delete me"), 0644)

	err := fops.Delete(testPath)
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Error("File should have been deleted")
	}
}

func TestFileOps_DeleteEmptyDir(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	dirPath := filepath.Join(tmpDir, "empty-dir")
	os.MkdirAll(dirPath, 0755)

	err := fops.Delete(dirPath)
	if err != nil {
		t.Fatalf("Failed to delete empty directory: %v", err)
	}

	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("Directory should have been deleted")
	}
}

func TestFileOps_DeleteNonEmptyDir_Fails(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	dirPath := filepath.Join(tmpDir, "non-empty-dir")
	os.MkdirAll(dirPath, 0755)
	os.WriteFile(filepath.Join(dirPath, "file.txt"), []byte("content"), 0644)

	err := fops.Delete(dirPath)
	if err == nil {
		t.Error("Expected error when deleting non-empty directory")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Errorf("Expected 'not empty' error, got: %v", err)
	}
}

func TestFileOps_DeleteRecursive(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	dirPath := filepath.Join(tmpDir, "recursive-dir")
	os.MkdirAll(filepath.Join(dirPath, "nested"), 0755)
	os.WriteFile(filepath.Join(dirPath, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(dirPath, "nested", "file2.txt"), []byte("content"), 0644)

	err := fops.DeleteRecursive(dirPath)
	if err != nil {
		t.Fatalf("Failed to delete recursively: %v", err)
	}

	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("Directory should have been deleted recursively")
	}
}

func TestFileOps_DeleteRecursive_CannotDeleteBase(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	err := fops.DeleteRecursive(tmpDir)
	if err == nil {
		t.Error("Expected error when trying to delete base directory")
	}
	if !strings.Contains(err.Error(), "base directory") {
		t.Errorf("Expected 'base directory' error, got: %v", err)
	}
}

func TestFileOps_CreateDirectory(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	dirPath := filepath.Join(tmpDir, "new-dir", "nested")

	err := fops.CreateDirectory(dirPath)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected path to be a directory")
	}
}

func TestFileOps_Exists(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	existingFile := filepath.Join(tmpDir, "existing.txt")
	os.WriteFile(existingFile, []byte("exists"), 0644)

	// Test existing file
	exists, err := fops.Exists(existingFile)
	if err != nil {
		t.Fatalf("Error checking existence: %v", err)
	}
	if !exists {
		t.Error("Expected file to exist")
	}

	// Test non-existing file
	exists, err = fops.Exists(filepath.Join(tmpDir, "nonexistent.txt"))
	if err != nil {
		t.Fatalf("Error checking existence: %v", err)
	}
	if exists {
		t.Error("Expected file to not exist")
	}
}

func TestFileOps_PathValidation_OutsideBase(t *testing.T) {
	fops, _ := newTestFileOps(t)

	// Try to access file outside base directory
	_, err := fops.Read("/etc/passwd")
	if err == nil {
		t.Error("Expected error when accessing file outside base directory")
	}
	if !strings.Contains(err.Error(), "outside allowed") {
		t.Errorf("Expected 'outside allowed' error, got: %v", err)
	}
}

func TestFileOps_PathValidation_DirectoryTraversal(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	// Try directory traversal
	traversalPath := filepath.Join(tmpDir, "..", "..", "etc", "passwd")
	_, err := fops.Read(traversalPath)
	if err == nil {
		t.Error("Expected error for directory traversal")
	}
}

func TestFileOps_ReadDirectory_Fails(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	_, err := fops.Read(tmpDir)
	if err == nil {
		t.Error("Expected error when reading a directory as file")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("Expected 'directory' error, got: %v", err)
	}
}

func TestFileOps_GetBaseDirectory(t *testing.T) {
	fops, tmpDir := newTestFileOps(t)

	baseDir := fops.GetBaseDirectory()
	if baseDir != tmpDir {
		t.Errorf("Expected base directory '%s', got '%s'", tmpDir, baseDir)
	}
}

func TestExpandHome(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/test", filepath.Join(homeDir, "test")},
		{"~", homeDir},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~user/path", "~user/path"}, // Only ~/... is expanded
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := expandHome(tt.input)
			if result != tt.expected {
				t.Errorf("expandHome(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
