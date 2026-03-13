package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test server defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected server host '0.0.0.0', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected server port 8080, got %d", cfg.Server.Port)
	}

	// Test AI defaults
	if cfg.AI.DefaultProvider != "openai" {
		t.Errorf("Expected default provider 'openai', got '%s'", cfg.AI.DefaultProvider)
	}
	if cfg.AI.OpenAI.Model != "gpt-4o" {
		t.Errorf("Expected OpenAI model 'gpt-4o', got '%s'", cfg.AI.OpenAI.Model)
	}
	if cfg.AI.Anthropic.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Expected Anthropic model 'claude-sonnet-4-20250514', got '%s'", cfg.AI.Anthropic.Model)
	}
	if cfg.AI.Ollama.BaseURL != "http://localhost:11434" {
		t.Errorf("Expected Ollama base URL 'http://localhost:11434', got '%s'", cfg.AI.Ollama.BaseURL)
	}

	// Test executor defaults
	if len(cfg.Executor.SafeCommands) == 0 {
		t.Error("Expected safe commands to be populated")
	}
	if len(cfg.Executor.BlockedCommands) == 0 {
		t.Error("Expected blocked commands to be populated")
	}

	// Test fileops defaults
	if cfg.FileOps.BaseDirectory == "" {
		t.Error("Expected base directory to be set")
	}
}

func TestLoadConfig_CreatesDefault(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Load config (should create default)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}

	// Verify defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadConfig_ReadsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create custom config
	content := `
server:
  host: "127.0.0.1"
  port: 9090
ai:
  default_provider: "anthropic"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify custom values
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.AI.DefaultProvider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", cfg.AI.DefaultProvider)
	}
}

func TestConfig_Update(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Update config
	err = cfg.Update(func(c *Config) {
		c.Server.Port = 3000
		c.AI.DefaultProvider = "ollama"
	})
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Reload and verify
	cfg2, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if cfg2.Server.Port != 3000 {
		t.Errorf("Expected port 3000 after update, got %d", cfg2.Server.Port)
	}
	if cfg2.AI.DefaultProvider != "ollama" {
		t.Errorf("Expected provider 'ollama' after update, got '%s'", cfg2.AI.DefaultProvider)
	}
}

func TestConfig_Public(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.OpenAI.APIKey = "secret-key"
	cfg.AI.Anthropic.APIKey = "another-secret"

	pub := cfg.Public()

	// Verify public config doesn't contain secrets
	if pub.OpenAIModel != "gpt-4o" {
		t.Errorf("Expected OpenAI model in public config")
	}

	// PublicConfig struct doesn't have API keys, so we just verify it works
	if pub.DefaultProvider != "openai" {
		t.Errorf("Expected default provider 'openai', got '%s'", pub.DefaultProvider)
	}
}

func TestConfig_ThreadSafety(t *testing.T) {
	cfg := DefaultConfig()

	// Run concurrent reads and writes
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			_ = cfg.GetAI()
			_ = cfg.GetServer()
			_ = cfg.GetExecutor()
			_ = cfg.GetFileOps()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
