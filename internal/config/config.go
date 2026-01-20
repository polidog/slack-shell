package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/polidog/slack-tui/internal/keymap"
	"github.com/polidog/slack-tui/internal/notification"
	"gopkg.in/yaml.v3"
)

type Config struct {
	// OAuth settings
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`

	// Direct token (optional, for backwards compatibility)
	SlackToken string `yaml:"slack_token"`
	AppToken   string `yaml:"app_token"`

	// OAuth settings
	RedirectPort int `yaml:"redirect_port"`

	// Keybindings
	Keybindings *keymap.KeyBindings `yaml:"keybindings"`

	// Notifications
	Notifications *notification.Config `yaml:"notifications"`
}

type Credentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	UserID       string `json:"user_id"`
	TeamID       string `json:"team_id"`
	TeamName     string `json:"team_name"`
}

func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".slack-tui"), nil
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

	// Try config file
	configDir, err := GetConfigDir()
	if err == nil {
		configPath := filepath.Join(configDir, "config.yaml")
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
				// Merge keybindings
				if fileCfg.Keybindings != nil {
					cfg.Keybindings = fileCfg.Keybindings
				}
				// Merge notifications
				if fileCfg.Notifications != nil {
					cfg.Notifications = fileCfg.Notifications
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

func LoadCredentials() (*Credentials, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	credPath := filepath.Join(configDir, "credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
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
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	credPath := filepath.Join(configDir, "credentials.json")
	return os.Remove(credPath)
}

func (c *Config) HasOAuthConfig() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

func (c *Config) HasDirectToken() bool {
	return c.SlackToken != ""
}
