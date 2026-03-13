package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pibot/pibot/internal/ai"
	"github.com/pibot/pibot/internal/config"
	"github.com/pibot/pibot/internal/executor"
	"github.com/pibot/pibot/internal/fileops"
)

func newTestServer(t *testing.T) (*Server, string) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Load config (creates file)
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg.FileOps.BaseDirectory = tmpDir

	aiMgr := ai.NewManager(cfg)
	// Register a mock/test provider or use Ollama (doesn't need API key)
	aiMgr.RegisterProvider(ai.NewOllamaProvider(cfg))

	exec := executor.NewExecutor(cfg)
	fops := fileops.NewFileOps(cfg)
	fops.EnsureBaseDirectory()

	server := NewServer(cfg, aiMgr, exec, fops)
	return server, tmpDir
}

func TestAPI_GetConfig(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["default_provider"] != "openai" {
		t.Errorf("Expected default_provider 'openai', got %v", result["default_provider"])
	}
}

func TestAPI_UpdateConfig(t *testing.T) {
	server, _ := newTestServer(t)

	payload := map[string]string{
		"default_provider": "anthropic",
		"openai_model":     "gpt-4-turbo",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(respBody, &result)

	if result["status"] != "updated" {
		t.Errorf("Expected status 'updated', got %v", result["status"])
	}
}

func TestAPI_ListProviders(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/providers", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	providers, ok := result["providers"].([]interface{})
	if !ok {
		t.Fatal("Expected providers array")
	}
	if len(providers) == 0 {
		t.Error("Expected at least one provider")
	}
}

func TestAPI_ExecSafeCommand(t *testing.T) {
	server, _ := newTestServer(t)

	payload := map[string]string{"command": "echo hello"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/exec", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["pending"].(bool) {
		t.Error("Safe command should not be pending")
	}
	if result["level"] != "safe" {
		t.Errorf("Expected level 'safe', got %v", result["level"])
	}
	if !strings.Contains(result["output"].(string), "hello") {
		t.Errorf("Expected output to contain 'hello', got %v", result["output"])
	}
}

func TestAPI_ExecDangerousCommand_Pending(t *testing.T) {
	server, _ := newTestServer(t)

	payload := map[string]string{"command": "rm -rf /tmp/test"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/exec", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if !result["pending"].(bool) {
		t.Error("Dangerous command should be pending")
	}
	if result["pending_id"] == "" {
		t.Error("Expected pending_id to be set")
	}
	if result["level"] != "dangerous" {
		t.Errorf("Expected level 'dangerous', got %v", result["level"])
	}
}

func TestAPI_ExecBlockedCommand_Rejected(t *testing.T) {
	server, _ := newTestServer(t)

	payload := map[string]string{"command": "dd if=/dev/zero of=/dev/sda"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/exec", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(respBody, &result)

	if !strings.Contains(result["error"], "blocked") {
		t.Errorf("Expected error about blocked command, got %v", result["error"])
	}
}

func TestAPI_ListPendingCommands(t *testing.T) {
	server, _ := newTestServer(t)

	// First create a pending command
	payload := map[string]string{"command": "sudo reboot"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/exec", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	// Now list pending
	req = httptest.NewRequest("GET", "/api/exec/pending", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	pending, ok := result["pending"].([]interface{})
	if !ok {
		t.Fatal("Expected pending array")
	}
	if len(pending) == 0 {
		t.Error("Expected at least one pending command")
	}
}

func TestAPI_ListFiles(t *testing.T) {
	server, tmpDir := newTestServer(t)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := server.fileOps.Write(testFile, "test content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/files", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	files, ok := result["files"].([]interface{})
	if !ok {
		t.Fatal("Expected files array")
	}
	// The tmpDir may contain config.yaml and test.txt, so at least 1 file
	if len(files) < 1 {
		t.Errorf("Expected at least 1 file, got %d", len(files))
	}

	// Check that test.txt is in the list
	found := false
	for _, f := range files {
		file := f.(map[string]interface{})
		if file["name"] == "test.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected test.txt in file list")
	}
}

func TestAPI_WriteFile(t *testing.T) {
	server, _ := newTestServer(t)

	payload := map[string]string{"content": "Hello, World!"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/files/newfile.txt", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(respBody, &result)

	if result["status"] != "written" {
		t.Errorf("Expected status 'written', got %v", result["status"])
	}
}

func TestAPI_ReadFile(t *testing.T) {
	server, tmpDir := newTestServer(t)

	// Create a test file
	testContent := "Test file content"
	testFile := filepath.Join(tmpDir, "readable.txt")
	server.fileOps.Write(testFile, testContent)

	req := httptest.NewRequest("GET", "/api/files/readable.txt", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(respBody, &result)

	if result["content"] != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, result["content"])
	}
}

func TestAPI_DeleteFile(t *testing.T) {
	server, tmpDir := newTestServer(t)

	// Create a test file
	testFile := filepath.Join(tmpDir, "deleteme.txt")
	server.fileOps.Write(testFile, "delete me")

	req := httptest.NewRequest("DELETE", "/api/files/deleteme.txt", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify file is deleted
	exists, _ := server.fileOps.Exists(testFile)
	if exists {
		t.Error("File should have been deleted")
	}
}

func TestAPI_StaticFiles(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "PiBot") {
		t.Error("Expected HTML to contain 'PiBot'")
	}
}

func TestAPI_StaticCSS(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/css/style.css", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), ":root") {
		t.Error("Expected CSS content")
	}
}

func TestAPI_StaticJS(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/js/app.js", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "PiBot") {
		t.Error("Expected JavaScript content")
	}
}

func TestAPI_InvalidJSON(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}
}

func TestAPI_NestedFilePath(t *testing.T) {
	server, _ := newTestServer(t)

	// Write to nested path
	payload := map[string]string{"content": "nested content"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/files/subdir/nested/file.txt", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Read it back
	req = httptest.NewRequest("GET", "/api/files/subdir/nested/file.txt", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	resp = w.Result()
	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(respBody, &result)

	if result["content"] != "nested content" {
		t.Errorf("Expected 'nested content', got '%s'", result["content"])
	}
}
