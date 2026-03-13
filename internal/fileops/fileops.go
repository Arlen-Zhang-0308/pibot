package fileops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pibot/pibot/internal/config"
)

// FileInfo represents information about a file or directory
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	Mode    string `json:"mode"`
}

// FileOps handles file operations with path restrictions
type FileOps struct {
	config *config.Config
}

// NewFileOps creates a new file operations handler
func NewFileOps(cfg *config.Config) *FileOps {
	return &FileOps{
		config: cfg,
	}
}

// expandHome expands ~ to the user's home directory
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	return path
}

// validatePath ensures the path is within allowed directories
func (f *FileOps) validatePath(path string) (string, error) {
	foCfg := f.config.GetFileOps()
	
	// Expand home directory
	path = expandHome(path)
	
	// Make path absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Clean the path to prevent directory traversal
	absPath = filepath.Clean(absPath)

	// Check if path is within base directory
	baseDir, err := filepath.Abs(expandHome(foCfg.BaseDirectory))
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %w", err)
	}

	if strings.HasPrefix(absPath, baseDir) {
		return absPath, nil
	}

	// Check additional allowed paths
	for _, allowed := range foCfg.AllowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, allowedAbs) {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("path %q is outside allowed directories", path)
}

// EnsureBaseDirectory creates the base directory if it doesn't exist
func (f *FileOps) EnsureBaseDirectory() error {
	foCfg := f.config.GetFileOps()
	return os.MkdirAll(expandHome(foCfg.BaseDirectory), 0755)
}

// List returns the contents of a directory
func (f *FileOps) List(path string) ([]FileInfo, error) {
	if path == "" {
		path = expandHome(f.config.GetFileOps().BaseDirectory)
	}

	absPath, err := f.validatePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	result := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		result = append(result, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(absPath, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			Mode:    info.Mode().String(),
		})
	}

	return result, nil
}

// Read returns the contents of a file
func (f *FileOps) Read(path string) (string, error) {
	absPath, err := f.validatePath(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return "", errors.New("path is a directory, not a file")
	}

	// Limit file size to prevent memory issues
	const maxSize = 10 * 1024 * 1024 // 10MB
	if info.Size() > maxSize {
		return "", fmt.Errorf("file too large (%d bytes, max %d)", info.Size(), maxSize)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// Write creates or overwrites a file with the given content
func (f *FileOps) Write(path, content string) error {
	absPath, err := f.validatePath(path)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Delete removes a file or empty directory
func (f *FileOps) Delete(path string) error {
	absPath, err := f.validatePath(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		// Check if directory is empty
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return fmt.Errorf("failed to read directory: %w", err)
		}
		if len(entries) > 0 {
			return errors.New("directory is not empty; use DeleteRecursive for non-empty directories")
		}
	}

	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

// DeleteRecursive removes a file or directory and all its contents
func (f *FileOps) DeleteRecursive(path string) error {
	absPath, err := f.validatePath(path)
	if err != nil {
		return err
	}

	// Safety check: don't delete the base directory itself
	foCfg := f.config.GetFileOps()
	baseDir, _ := filepath.Abs(expandHome(foCfg.BaseDirectory))
	if absPath == baseDir {
		return errors.New("cannot delete base directory")
	}

	if err := os.RemoveAll(absPath); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

// CreateDirectory creates a directory
func (f *FileOps) CreateDirectory(path string) error {
	absPath, err := f.validatePath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// Exists checks if a path exists
func (f *FileOps) Exists(path string) (bool, error) {
	absPath, err := f.validatePath(path)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(absPath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// GetBaseDirectory returns the configured base directory
func (f *FileOps) GetBaseDirectory() string {
	return expandHome(f.config.GetFileOps().BaseDirectory)
}
