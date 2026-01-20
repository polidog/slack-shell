package notification

// Config holds all notification configuration
type Config struct {
	Enabled bool `yaml:"enabled"`

	Bell    BellConfig    `yaml:"bell"`
	Desktop DesktopConfig `yaml:"desktop"`
	Title   TitleConfig   `yaml:"title"`
	Visual  VisualConfig  `yaml:"visual"`

	MuteChannels []string `yaml:"mute_channels"`
	DND          bool     `yaml:"dnd"`
}

// BellConfig configures terminal bell notifications
type BellConfig struct {
	Enabled      bool `yaml:"enabled"`
	MentionsOnly bool `yaml:"mentions_only"`
}

// DesktopConfig configures desktop notifications
type DesktopConfig struct {
	Enabled      bool `yaml:"enabled"`
	MentionsOnly bool `yaml:"mentions_only"`
}

// TitleConfig configures terminal title notifications
type TitleConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Format    string `yaml:"format"`
	BaseTitle string `yaml:"base_title"`
}

// VisualConfig configures visual notifications
type VisualConfig struct {
	Enabled      bool `yaml:"enabled"`
	MaxItems     int  `yaml:"max_items"`
	DismissAfter int  `yaml:"dismiss_after"`
}

// DefaultConfig returns the default notification configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Bell: BellConfig{
			Enabled:      true,
			MentionsOnly: false,
		},
		Desktop: DesktopConfig{
			Enabled:      true,
			MentionsOnly: false,
		},
		Title: TitleConfig{
			Enabled:   true,
			Format:    "Slack Shell (%d)",
			BaseTitle: "Slack Shell",
		},
		Visual: VisualConfig{
			Enabled:      true,
			MaxItems:     5,
			DismissAfter: 10,
		},
		MuteChannels: []string{},
		DND:          false,
	}
}

// Merge merges user config with defaults
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	c.Enabled = other.Enabled
	c.DND = other.DND

	if other.MuteChannels != nil {
		c.MuteChannels = other.MuteChannels
	}

	// Bell config
	c.Bell.Enabled = other.Bell.Enabled
	c.Bell.MentionsOnly = other.Bell.MentionsOnly

	// Desktop config
	c.Desktop.Enabled = other.Desktop.Enabled
	c.Desktop.MentionsOnly = other.Desktop.MentionsOnly

	// Title config
	c.Title.Enabled = other.Title.Enabled
	if other.Title.Format != "" {
		c.Title.Format = other.Title.Format
	}
	if other.Title.BaseTitle != "" {
		c.Title.BaseTitle = other.Title.BaseTitle
	}

	// Visual config
	c.Visual.Enabled = other.Visual.Enabled
	if other.Visual.MaxItems > 0 {
		c.Visual.MaxItems = other.Visual.MaxItems
	}
	if other.Visual.DismissAfter >= 0 {
		c.Visual.DismissAfter = other.Visual.DismissAfter
	}
}
