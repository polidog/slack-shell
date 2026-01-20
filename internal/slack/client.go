package slack

import (
	"github.com/slack-go/slack"
)

type Client struct {
	api      *slack.Client
	userID   string
	userName string
}

func NewClient(token string) (*Client, error) {
	api := slack.New(token)

	// Test authentication and get user info
	authTest, err := api.AuthTest()
	if err != nil {
		return nil, err
	}

	return &Client{
		api:      api,
		userID:   authTest.UserID,
		userName: authTest.User,
	}, nil
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
