package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/polidog/slack-shell/internal/keymap"
	"github.com/polidog/slack-shell/internal/notification"
	"gopkg.in/yaml.v3"
)

type Config struct {
	// OAuth settings
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`

	// Direct token (optional, for backwards compatibility)
	SlackToken string `yaml:"slack_token"`
	AppToken   string `yaml:"app_token"`

	// Debug mode
	Debug bool `yaml:"debug"`

	// OAuth settings
	RedirectPort int `yaml:"redirect_port"`

	// Keybindings
	Keybindings *keymap.KeyBindings `yaml:"keybindings"`

	// Notifications
	Notifications *notification.Config `yaml:"notifications"`

	// Prompt customization
	Prompt *PromptConfig `yaml:"prompt"`

	// Startup customization
	Startup *StartupConfig `yaml:"startup"`
}

// PromptConfig defines prompt customization settings
type PromptConfig struct {
	// Format is the prompt template string
	// Available variables:
	//   {workspace} - workspace name
	//   {location}  - #channel, @user, or empty for root
	//   {channel}   - channel name only (without #)
	//   {user}      - user name only (without @)
	// Default: "{workspace} {location}> "
	Format string `yaml:"format"`
}

// StartupConfig defines startup customization settings
type StartupConfig struct {
	// Message is a single line welcome message
	// Available variables:
	//   {workspace} - workspace name
	// Default: "Welcome to Slack Shell - {workspace}"
	Message string `yaml:"message"`

	// Banner is a multi-line banner displayed at startup (overrides Message if set)
	// Available variables:
	//   {workspace} - workspace name
	Banner string `yaml:"banner"`

	// InitCommands are commands to execute automatically at startup
	// Example: ["cd #general", "ls"]
	InitCommands []string `yaml:"init_commands"`
}

type Credentials struct {
	AccessToken  string `json:"access_token"`
	BotToken     string `json:"bot_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	BotScope     string `json:"bot_scope,omitempty"`
	UserID       string `json:"user_id"`
	TeamID       string `json:"team_id"`
	TeamName     string `json:"team_name"`
}

// GetConfigDir returns the configuration directory path.
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config/slack-shell/
func GetConfigDir() (string, error) {
	// Check XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "slack-shell"), nil
	}

	// Default to ~/.config/slack-shell/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "slack-shell"), nil
}

// GetLegacyConfigDir returns the legacy configuration directory path (~/.slack-shell/)
func GetLegacyConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".slack-shell"), nil
}

// findConfigFile looks for config.yaml in XDG config dir, then legacy dir
func findConfigFile() string {
	// Try new location first (~/.config/slack-shell/ or $XDG_CONFIG_HOME/slack-shell/)
	if configDir, err := GetConfigDir(); err == nil {
		configPath := filepath.Join(configDir, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Fall back to legacy location (~/.slack-shell/)
	if legacyDir, err := GetLegacyConfigDir(); err == nil {
		configPath := filepath.Join(legacyDir, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	return ""
}

func Load() (*Config, error) {
	cfg := &Config{
		RedirectPort: 8080, // Default port
	}

	// Try environment variables first
	if token := os.Getenv("SLACK_TOKEN"); token != "" {
		cfg.SlackToken = token
	}
	if appToken := os.Getenv("SLACK_APP_TOKEN"); appToken != "" {
		cfg.AppToken = appToken
	}
	if clientID := os.Getenv("SLACK_CLIENT_ID"); clientID != "" {
		cfg.ClientID = clientID
	}
	if clientSecret := os.Getenv("SLACK_CLIENT_SECRET"); clientSecret != "" {
		cfg.ClientSecret = clientSecret
	}
	if debug := os.Getenv("SLACK_DEBUG"); debug == "1" || debug == "true" {
		cfg.Debug = true
	}

	// Try config file (new location first, then legacy)
	if configPath := findConfigFile(); configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			var fileCfg Config
			if err := yaml.Unmarshal(data, &fileCfg); err == nil {
				// File values override only if env vars are empty
				if cfg.SlackToken == "" && fileCfg.SlackToken != "" {
					cfg.SlackToken = fileCfg.SlackToken
				}
				if cfg.AppToken == "" && fileCfg.AppToken != "" {
					cfg.AppToken = fileCfg.AppToken
				}
				if cfg.ClientID == "" && fileCfg.ClientID != "" {
					cfg.ClientID = fileCfg.ClientID
				}
				if cfg.ClientSecret == "" && fileCfg.ClientSecret != "" {
					cfg.ClientSecret = fileCfg.ClientSecret
				}
				if fileCfg.RedirectPort != 0 {
					cfg.RedirectPort = fileCfg.RedirectPort
				}
				// Merge debug (env var takes precedence)
				if !cfg.Debug && fileCfg.Debug {
					cfg.Debug = fileCfg.Debug
				}
				// Merge keybindings
				if fileCfg.Keybindings != nil {
					cfg.Keybindings = fileCfg.Keybindings
				}
				// Merge notifications
				if fileCfg.Notifications != nil {
					cfg.Notifications = fileCfg.Notifications
				}
				// Merge prompt config
				if fileCfg.Prompt != nil {
					cfg.Prompt = fileCfg.Prompt
				}
				// Merge startup config
				if fileCfg.Startup != nil {
					cfg.Startup = fileCfg.Startup
				}
			}
		}
	}

	return cfg, nil
}

// LoadFromPath loads configuration from a specific file path
func LoadFromPath(path string) (*Config, error) {
	cfg := &Config{
		RedirectPort: 8080,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// GetKeymap returns a Keymap with user customizations merged with defaults
func (c *Config) GetKeymap() *keymap.Keymap {
	bindings := keymap.DefaultKeyBindings()
	if c.Keybindings != nil {
		bindings.Merge(c.Keybindings)
	}
	return keymap.New(bindings)
}

// GetNotificationConfig returns notification config with defaults merged
func (c *Config) GetNotificationConfig() *notification.Config {
	cfg := notification.DefaultConfig()
	if c.Notifications != nil {
		cfg.Merge(c.Notifications)
	}
	return cfg
}

// GetPromptConfig returns prompt config with defaults
func (c *Config) GetPromptConfig() *PromptConfig {
	if c.Prompt != nil && c.Prompt.Format != "" {
		return c.Prompt
	}
	return DefaultPromptConfig()
}

// DefaultPromptConfig returns the default prompt configuration
func DefaultPromptConfig() *PromptConfig {
	return &PromptConfig{
		Format: "{workspace} {location}> ",
	}
}

// GetStartupConfig returns startup config with defaults
func (c *Config) GetStartupConfig() *StartupConfig {
	if c.Startup != nil {
		return c.Startup
	}
	return DefaultStartupConfig()
}

// DefaultStartupConfig returns the default startup configuration
func DefaultStartupConfig() *StartupConfig {
	return &StartupConfig{
		Message:      "Welcome to Slack Shell - {workspace}",
		Banner:       "",
		InitCommands: nil,
	}
}

func LoadCredentials() (*Credentials, error) {
	// Try new location first
	if configDir, err := GetConfigDir(); err == nil {
		credPath := filepath.Join(configDir, "credentials.json")
		if data, err := os.ReadFile(credPath); err == nil {
			var creds Credentials
			if err := json.Unmarshal(data, &creds); err != nil {
				return nil, err
			}
			return &creds, nil
		}
	}

	// Fall back to legacy location
	if legacyDir, err := GetLegacyConfigDir(); err == nil {
		credPath := filepath.Join(legacyDir, "credentials.json")
		if data, err := os.ReadFile(credPath); err == nil {
			var creds Credentials
			if err := json.Unmarshal(data, &creds); err != nil {
				return nil, err
			}
			return &creds, nil
		}
	}

	return nil, fmt.Errorf("credentials not found")
}

func SaveCredentials(creds *Credentials) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	credPath := filepath.Join(configDir, "credentials.json")
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(credPath, data, 0600)
}

func DeleteCredentials() error {
	var lastErr error

	// Delete from new location
	if configDir, err := GetConfigDir(); err == nil {
		credPath := filepath.Join(configDir, "credentials.json")
		if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}

	// Also delete from legacy location
	if legacyDir, err := GetLegacyConfigDir(); err == nil {
		credPath := filepath.Join(legacyDir, "credentials.json")
		if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}

	return lastErr
}

func (c *Config) HasOAuthConfig() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

func (c *Config) HasDirectToken() bool {
	return c.SlackToken != ""
}

// SampleConfigYAML returns a sample configuration file with comments
func SampleConfigYAML() string {
	return `# Slack Shell Configuration
# Place this file at ~/.config/slack-shell/config.yaml
# (or $XDG_CONFIG_HOME/slack-shell/config.yaml)

# ============================================================
# Authentication
# ============================================================
# Option 1: OAuth (recommended)
# Get these from your Slack App settings at https://api.slack.com/apps
# client_id: "your-client-id"
# client_secret: "your-client-secret"
# redirect_port: 8080

# Option 2: Direct token (legacy)
# slack_token: "xoxp-your-token"
# app_token: "xapp-your-app-token"

# ============================================================
# Prompt Customization
# ============================================================
prompt:
  # Available variables:
  #   {workspace} - workspace name
  #   {location}  - #channel, @user, or empty for root
  #   {channel}   - channel name only (without #)
  #   {user}      - user name only (without @)
  format: "{workspace} {location}> "

# ============================================================
# Startup Customization
# ============================================================
startup:
  # Single line welcome message
  # Available variables: {workspace}
  message: "Welcome to Slack Shell - {workspace}"

  # Multi-line banner (overrides message if set)
  # banner: |
  #   ╔═══════════════════════════════╗
  #   ║  Welcome to {workspace}       ║
  #   ╚═══════════════════════════════╝

  # Commands to execute automatically at startup
  # init_commands:
  #   - "cd #general"
  #   - "cat -n 10"

# ============================================================
# Keybindings (Vim-like defaults)
# ============================================================
keybindings:
  # Navigation
  up: ["k", "up"]
  down: ["j", "down"]
  top: ["g", "home"]
  bottom: ["G", "end"]
  page_up: ["ctrl+b", "pgup"]
  page_down: ["ctrl+f", "pgdown"]
  half_up: ["ctrl+u"]
  half_down: ["ctrl+d"]

  # Panel navigation
  next_panel: ["tab", "l"]
  prev_panel: ["shift+tab", "h"]

  # Actions
  select: ["enter", "o"]
  back: ["esc", "q"]
  input_mode: ["i", "a"]
  reply: ["r"]
  quit: ["q"]
  force_quit: ["ctrl+c"]
  open_thread: ["enter", "t"]
  close_thread: ["esc", "q"]

  # Input mode
  submit: ["enter"]
  cancel: ["esc"]

  # Search
  search: ["/"]
  next_match: ["n"]
  prev_match: ["N"]

  # Misc
  refresh: ["ctrl+r", "R"]
  help: ["?"]

# ============================================================
# Notifications
# ============================================================
notifications:
  enabled: true
  dnd: false

  # Mute specific channels
  # mute_channels:
  #   - "#random"
  #   - "#announcements"

  # Terminal bell
  bell:
    enabled: true
    mentions_only: false

  # Desktop notifications (requires notify-send on Linux)
  desktop:
    enabled: true
    mentions_only: false

  # Terminal title (shows unread count)
  title:
    enabled: true
    format: "Slack Shell (%d)"
    base_title: "Slack Shell"

  # Visual notifications (in-app)
  visual:
    enabled: true
    max_items: 5
    dismiss_after: 10
`
}

// InitConfig creates a sample config file at the specified path
// If path is empty, uses the default location (~/.slack-shell/config.yaml)
func InitConfig(path string, force bool) (string, error) {
	var configPath string

	if path == "" {
		// Use default location
		configDir, err := GetConfigDir()
		if err != nil {
			return "", err
		}

		// Create config directory if it doesn't exist
		if err := os.MkdirAll(configDir, 0700); err != nil {
			return "", err
		}

		configPath = filepath.Join(configDir, "config.yaml")
	} else {
		// Use specified path
		configPath = path

		// Expand ~ to home directory
		if len(configPath) > 0 && configPath[0] == '~' {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configPath = filepath.Join(homeDir, configPath[1:])
		}

		// Create parent directory if it doesn't exist
		parentDir := filepath.Dir(configPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return "", err
		}
	}

	// Check if file already exists
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return "", fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Write sample config
	if err := os.WriteFile(configPath, []byte(SampleConfigYAML()), 0600); err != nil {
		return "", err
	}

	return configPath, nil
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}
