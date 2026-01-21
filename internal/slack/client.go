package slack

import (
	"strings"

	"github.com/slack-go/slack"
)

type Client struct {
	api      *slack.Client
	botAPI   *slack.Client
	token    string
	botToken string
	userID   string
	userName string
	teamID   string
	teamName string
}

func NewClient(token string) (*Client, error) {
	return NewClientWithBotToken(token, "")
}

func NewClientWithBotToken(token, botToken string) (*Client, error) {
	api := slack.New(token)

	// Test authentication and get user info
	authTest, err := api.AuthTest()
	if err != nil {
		return nil, err
	}

	client := &Client{
		api:      api,
		token:    token,
		botToken: botToken,
		userID:   authTest.UserID,
		userName: authTest.User,
		teamID:   authTest.TeamID,
		teamName: authTest.Team,
	}

	// Create bot API client if bot token is provided
	if botToken != "" {
		client.botAPI = slack.New(botToken)
	}

	return client, nil
}

func (c *Client) GetUserID() string {
	return c.userID
}

func (c *Client) GetUserName() string {
	return c.userName
}

func (c *Client) API() *slack.Client {
	return c.api
}

// TeamInfo represents basic team information
type TeamInfo struct {
	ID   string
	Name string
}

func (c *Client) GetTeamInfo() (*TeamInfo, error) {
	// Return cached team info from AuthTest
	return &TeamInfo{
		ID:   c.teamID,
		Name: c.teamName,
	}, nil
}

func (c *Client) GetTeamName() string {
	return c.teamName
}

func (c *Client) GetTeamID() string {
	return c.teamID
}

// GetTokenType returns the type of token being used
func (c *Client) GetTokenType() string {
	if strings.HasPrefix(c.token, "xoxp-") {
		return "User Token (xoxp-)"
	}
	if strings.HasPrefix(c.token, "xoxb-") {
		return "Bot Token (xoxb-)"
	}
	if strings.HasPrefix(c.token, "xoxs-") {
		return "Legacy Token (xoxs-)"
	}
	return "Unknown"
}

// GetTokenPrefix returns the first part of the token for display (masked)
func (c *Client) GetTokenPrefix() string {
	if len(c.token) > 15 {
		return c.token[:15] + "..."
	}
	return c.token
}

// HasBotToken returns true if a bot token is configured
func (c *Client) HasBotToken() bool {
	return c.botAPI != nil
}

// BotAPI returns the bot API client (may be nil if no bot token)
func (c *Client) BotAPI() *slack.Client {
	return c.botAPI
}
