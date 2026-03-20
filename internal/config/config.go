package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server     ServerConfig      `yaml:"server"`
	AI         AIConfig          `yaml:"ai"`
	Executor   ExecutorConfig    `yaml:"executor"`
	FileOps    FileOpsConfig     `yaml:"fileops"`
	Prompts    PromptsConfig     `yaml:"prompts"`
	WebSearch  WebSearchConfig   `yaml:"web_search"`
	SkillsPath string            `yaml:"skills_path"`
	Reboot     RebootConfig      `yaml:"reboot"`
	// Env holds environment variables that are injected into the process
	// environment at server startup. Each key-value pair is set via os.Setenv.
	Env        map[string]string `yaml:"env,omitempty"`
	mu         sync.RWMutex      `yaml:"-"`
	configPath string            `yaml:"-"`
}

// WebSearchConfig holds configuration for the web search tool.
// Priority: DuckDuckGo is used first (no API key required for the free Instant
// Answer endpoint, but an API key can be set for future paid tiers). If
// DuckDuckGo's api_key is empty, Perplexity is used as the fallback via its
// Search API (requires perplexity_api_key).
type WebSearchConfig struct {
	// DuckDuckGoAPIKey is optional. Leave empty to use the free Instant Answer API.
	DuckDuckGoAPIKey string `yaml:"duckduckgo_api_key"`
	// PerplexityAPIKey enables the Perplexity Search API as a fallback.
	PerplexityAPIKey string `yaml:"perplexity_api_key"`
}

// RebootConfig holds settings for how the bot restarts itself.
type RebootConfig struct {
	// Plan selects which reboot strategy to use.
	// Built-in plans:
	//   "screen" (default) – kills the current process; the wrapper script
	//                        re-launches the server inside the same screen session.
	Plan string `yaml:"plan"`

	// Screen plan options
	Screen ScreenRebootConfig `yaml:"screen"`
}

// ScreenRebootConfig contains parameters for the "screen" reboot plan.
type ScreenRebootConfig struct {
	// SessionName is the screen session that hosts the server (default: "pibot").
	SessionName string `yaml:"session_name"`
	// WorkDir is the directory to cd into before starting the server.
	WorkDir string `yaml:"work_dir"`
	// StartCommand is the command used to launch the server.
	StartCommand string `yaml:"start_command"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// AIConfig holds AI provider settings
type AIConfig struct {
	DefaultProvider string          `yaml:"default_provider"`
	OpenAI          OpenAIConfig    `yaml:"openai"`
	Anthropic       AnthropicConfig `yaml:"anthropic"`
	Google          GoogleConfig    `yaml:"google"`
	Ollama          OllamaConfig    `yaml:"ollama"`
	Qwen            QwenConfig      `yaml:"qwen"`
}

// OpenAIConfig holds OpenAI-specific settings
type OpenAIConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// AnthropicConfig holds Anthropic-specific settings
type AnthropicConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// GoogleConfig holds Google Gemini-specific settings
type GoogleConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// OllamaConfig holds Ollama-specific settings
type OllamaConfig struct {
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

// QwenConfig holds Qwen-specific settings
type QwenConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

// ExecutorConfig holds command execution settings
type ExecutorConfig struct {
	SafeCommands      []string `yaml:"safe_commands"`
	ModerateCommands  []string `yaml:"moderate_commands"`
	DangerousCommands []string `yaml:"dangerous_commands"`
	BlockedCommands   []string `yaml:"blocked_commands"`
}

// FileOpsConfig holds file operation settings
type FileOpsConfig struct {
	BaseDirectory string   `yaml:"base_directory"`
	AllowedPaths  []string `yaml:"allowed_paths"`
}

// PromptsConfig holds prompt customization settings
type PromptsConfig struct {
	// SystemPrompt is a custom system prompt (overrides default if set)
	SystemPrompt string `yaml:"system_prompt"`
	// BotName is the name PiBot uses to identify itself
	BotName string `yaml:"bot_name"`
	// EnableTools enables/disables tool calling (default: true)
	EnableTools bool `yaml:"enable_tools"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		AI: AIConfig{
			DefaultProvider: "openai",
			OpenAI: OpenAIConfig{
				Model: "gpt-4o",
			},
			Anthropic: AnthropicConfig{
				Model: "claude-sonnet-4-20250514",
			},
			Google: GoogleConfig{
				Model: "gemini-1.5-pro",
			},
			Ollama: OllamaConfig{
				BaseURL: "http://localhost:11434",
				Model:   "llama3",
			},
			Qwen: QwenConfig{
				BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
				Model:   "qwen-plus",
			},
		},
		Executor: ExecutorConfig{
			SafeCommands:      []string{"ls", "pwd", "cat", "echo", "whoami", "date", "uptime", "df", "free", "ps", "head", "tail", "wc", "grep", "find", "which", "env", "hostname"},
			ModerateCommands:  []string{"mkdir", "cp", "mv", "touch", "chmod", "chown", "tar", "gzip", "gunzip", "zip", "unzip", "wget", "curl"},
			DangerousCommands: []string{"rm", "sudo", "apt", "yum", "dnf", "pacman", "pip", "npm", "go", "make", "systemctl", "service", "reboot", "shutdown", "kill", "pkill"},
			BlockedCommands:   []string{"dd", "mkfs", "fdisk", "parted", "format", ":(){:|:&};:"},
		},
		FileOps: FileOpsConfig{
			BaseDirectory: filepath.Join(homeDir, "pibot-workspace"),
			AllowedPaths:  []string{},
		},
		Prompts: PromptsConfig{
			BotName:     "PiBot",
			EnableTools: true,
		},
		SkillsPath: "~/.pibot_skills",
		Reboot: RebootConfig{
			Plan: "screen",
			Screen: ScreenRebootConfig{
				SessionName:  "pibot",
				WorkDir:      "/home/orangepi/workspace/pibot",
				StartCommand: "go run cmd/server/main.go",
			},
		},
	}
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.configPath = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config file
			if err := cfg.Save(); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the current configuration to the YAML file
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(c.configPath, data, 0600)
}

// Update modifies configuration values and saves
func (c *Config) Update(updates func(*Config)) error {
	c.mu.Lock()
	updates(c)
	c.mu.Unlock()
	return c.Save()
}

// GetAI returns AI configuration (thread-safe)
func (c *Config) GetAI() AIConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AI
}

// GetServer returns server configuration (thread-safe)
func (c *Config) GetServer() ServerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Server
}

// GetExecutor returns executor configuration (thread-safe)
func (c *Config) GetExecutor() ExecutorConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Executor
}

// GetFileOps returns file operations configuration (thread-safe)
func (c *Config) GetFileOps() FileOpsConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.FileOps
}

// GetPrompts returns prompts configuration (thread-safe)
func (c *Config) GetPrompts() PromptsConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Prompts
}

// GetWebSearch returns the web search configuration (thread-safe).
func (c *Config) GetWebSearch() WebSearchConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WebSearch
}

// GetReboot returns the reboot configuration (thread-safe).
func (c *Config) GetReboot() RebootConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Reboot
}

// GetSkillsPath returns the path to the external skills directory (thread-safe).
// Defaults to ~/.pibot_skills if not configured.
func (c *Config) GetSkillsPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.SkillsPath == "" {
		return "~/.pibot_skills"
	}
	return c.SkillsPath
}

// GetEnv returns the environment variable map (thread-safe).
func (c *Config) GetEnv() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]string, len(c.Env))
	for k, v := range c.Env {
		result[k] = v
	}
	return result
}

// InjectEnv sets each key-value pair from the Env config section into the
// current process environment using os.Setenv. Existing variables with the
// same name are overwritten. Returns the first error encountered, if any.
func (c *Config) InjectEnv() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, v := range c.Env {
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("setting env %q: %w", k, err)
		}
	}
	return nil
}

// PublicConfig returns a config safe to expose (without API keys)
type PublicConfig struct {
	Server          ServerConfig `json:"server"`
	DefaultProvider string       `json:"default_provider"`
	OpenAIModel     string       `json:"openai_model"`
	AnthropicModel  string       `json:"anthropic_model"`
	GoogleModel     string       `json:"google_model"`
	OllamaBaseURL   string       `json:"ollama_base_url"`
	OllamaModel     string       `json:"ollama_model"`
	QwenBaseURL     string       `json:"qwen_base_url"`
	QwenModel       string       `json:"qwen_model"`
	BaseDirectory   string       `json:"base_directory"`
}

// Public returns a public-safe version of the config
func (c *Config) Public() PublicConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return PublicConfig{
		Server:          c.Server,
		DefaultProvider: c.AI.DefaultProvider,
		OpenAIModel:     c.AI.OpenAI.Model,
		AnthropicModel:  c.AI.Anthropic.Model,
		GoogleModel:     c.AI.Google.Model,
		OllamaBaseURL:   c.AI.Ollama.BaseURL,
		OllamaModel:     c.AI.Ollama.Model,
		QwenBaseURL:     c.AI.Qwen.BaseURL,
		QwenModel:       c.AI.Qwen.Model,
		BaseDirectory:   c.FileOps.BaseDirectory,
	}
}
