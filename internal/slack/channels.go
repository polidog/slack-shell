package slack

import (
	"github.com/slack-go/slack"
)

type Channel struct {
	ID        string
	Name      string
	IsChannel bool
	IsPrivate bool
	IsIM      bool
	IsMpIM    bool
	UserID    string // For DMs, the other user's ID
}

func (c *Client) GetChannels() ([]Channel, error) {
	var channels []Channel

	// Get public and private channels
	params := &slack.GetConversationsParameters{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           1000,
	}

	convs, cursor, err := c.api.GetConversations(params)
	if err != nil {
		return nil, err
	}

	for _, conv := range convs {
		channels = append(channels, Channel{
			ID:        conv.ID,
			Name:      conv.Name,
			IsChannel: !conv.IsPrivate,
			IsPrivate: conv.IsPrivate,
		})
	}

	// Handle pagination
	for cursor != "" {
		params.Cursor = cursor
		convs, cursor, err = c.api.GetConversations(params)
		if err != nil {
			break
		}
		for _, conv := range convs {
			channels = append(channels, Channel{
				ID:        conv.ID,
				Name:      conv.Name,
				IsChannel: !conv.IsPrivate,
				IsPrivate: conv.IsPrivate,
			})
		}
	}

	return channels, nil
}

func (c *Client) GetDMs() ([]Channel, error) {
	var channels []Channel

	params := &slack.GetConversationsParameters{
		Types: []string{"im"},
		Limit: 1000,
	}

	convs, cursor, err := c.api.GetConversations(params)
	if err != nil {
		return nil, err
	}

	for _, conv := range convs {
		channels = append(channels, Channel{
			ID:     conv.ID,
			Name:   conv.User,
			IsIM:   true,
			UserID: conv.User,
		})
	}

	// Handle pagination
	for cursor != "" {
		params.Cursor = cursor
		convs, cursor, err = c.api.GetConversations(params)
		if err != nil {
			break
		}
		for _, conv := range convs {
			channels = append(channels, Channel{
				ID:     conv.ID,
				Name:   conv.User,
				IsIM:   true,
				UserID: conv.User,
			})
		}
	}

	return channels, nil
}

func (c *Client) GetMpIMs() ([]Channel, error) {
	var channels []Channel

	params := &slack.GetConversationsParameters{
		Types: []string{"mpim"},
		Limit: 1000,
	}

	convs, cursor, err := c.api.GetConversations(params)
	if err != nil {
		return nil, err
	}

	for _, conv := range convs {
		channels = append(channels, Channel{
			ID:     conv.ID,
			Name:   conv.Name,
			IsMpIM: true,
		})
	}

	// Handle pagination
	for cursor != "" {
		params.Cursor = cursor
		convs, cursor, err = c.api.GetConversations(params)
		if err != nil {
			break
		}
		for _, conv := range convs {
			channels = append(channels, Channel{
				ID:     conv.ID,
				Name:   conv.Name,
				IsMpIM: true,
			})
		}
	}

	return channels, nil
}

func (c *Client) GetUserInfo(userID string) (*slack.User, error) {
	return c.api.GetUserInfo(userID)
}

func (c *Client) GetUsersInfo(userIDs []string) (*[]slack.User, error) {
	return c.api.GetUsersInfo(userIDs...)
}
