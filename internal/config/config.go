package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server     ServerConfig   `yaml:"server"`
	AI         AIConfig       `yaml:"ai"`
	Executor   ExecutorConfig `yaml:"executor"`
	FileOps    FileOpsConfig  `yaml:"fileops"`
	Prompts    PromptsConfig  `yaml:"prompts"`
	SkillsPath string         `yaml:"skills_path"`
	mu         sync.RWMutex   `yaml:"-"`
	configPath string         `yaml:"-"`
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
