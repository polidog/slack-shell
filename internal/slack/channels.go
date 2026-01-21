package slack

import (
	"strings"

	"github.com/slack-go/slack"
)

type Channel struct {
	ID          string
	Name        string
	IsChannel   bool
	IsPrivate   bool
	IsIM        bool
	IsMpIM      bool
	IsExtShared bool   // Slack Connect (externally shared) channel
	UserID      string // For DMs, the other user's ID
}

func (c *Client) GetChannels() ([]Channel, error) {
	var channels []Channel

	// Get public and private channels that user is a member of
	params := &slack.GetConversationsParameters{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           200,
	}

	convs, cursor, err := c.api.GetConversations(params)
	if err != nil {
		return nil, err
	}

	for _, conv := range convs {
		// Only include channels where user is a member
		if conv.IsMember {
			channels = append(channels, Channel{
				ID:        conv.ID,
				Name:      conv.Name,
				IsChannel: !conv.IsPrivate,
				IsPrivate: conv.IsPrivate,
			})
		}
	}

	// Handle pagination
	for cursor != "" {
		params.Cursor = cursor
		convs, cursor, err = c.api.GetConversations(params)
		if err != nil {
			break
		}
		for _, conv := range convs {
			if conv.IsMember {
				channels = append(channels, Channel{
					ID:        conv.ID,
					Name:      conv.Name,
					IsChannel: !conv.IsPrivate,
					IsPrivate: conv.IsPrivate,
				})
			}
		}
	}

	return channels, nil
}

func (c *Client) GetDMs() ([]Channel, error) {
	var channels []Channel

	// Get only recent/open DMs (limit to 50)
	params := &slack.GetConversationsParameters{
		Types: []string{"im"},
		Limit: 50,
	}

	convs, _, err := c.api.GetConversations(params)
	if err != nil {
		return nil, err
	}

	for _, conv := range convs {
		// Only include open/active DMs
		if conv.IsOpen {
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
	if len(userIDs) == 0 {
		return &[]slack.User{}, nil
	}

	// Slack API has a limit on how many users can be fetched at once
	// Process in batches of 30 to avoid "too_many_users" error
	const batchSize = 30
	var allUsers []slack.User

	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}

		batch := userIDs[i:end]
		users, err := c.api.GetUsersInfo(batch...)
		if err != nil {
			return nil, err
		}
		if users != nil {
			allUsers = append(allUsers, *users...)
		}
	}

	return &allUsers, nil
}

// GetUserByName finds a user or bot/app by their display name or real name
// Returns user ID and name if found, empty strings if not found
// Prioritizes human users over bots when names match
func (c *Client) GetUserByName(name string) (userID string, userName string, err error) {
	// Use users.list API to search for users
	users, err := c.api.GetUsers()
	if err != nil {
		return "", "", err
	}

	// Normalize the search name (lowercase)
	searchName := strings.ToLower(name)

	// First pass: look for human users only
	for _, user := range users {
		// Skip deleted users and bots in first pass
		if user.Deleted || user.IsBot {
			continue
		}

		// Check various name fields (case-insensitive)
		if strings.ToLower(user.Name) == searchName ||
			strings.ToLower(user.Profile.DisplayName) == searchName ||
			strings.ToLower(user.Profile.DisplayNameNormalized) == searchName ||
			strings.ToLower(user.RealName) == searchName {
			return user.ID, user.Name, nil
		}
	}

	// Second pass: look for bots/apps if no human user found
	for _, user := range users {
		// Skip deleted users, only look at bots
		if user.Deleted || !user.IsBot {
			continue
		}

		// Check various name fields (case-insensitive)
		if strings.ToLower(user.Name) == searchName ||
			strings.ToLower(user.Profile.DisplayName) == searchName ||
			strings.ToLower(user.Profile.DisplayNameNormalized) == searchName ||
			strings.ToLower(user.RealName) == searchName {
			return user.ID, user.Name, nil
		}
	}

	return "", "", nil
}

func (c *Client) CreateChannel(name string, isPrivate bool) (*Channel, error) {
	channel, err := c.api.CreateConversation(slack.CreateConversationParams{
		ChannelName: name,
		IsPrivate:   isPrivate,
	})
	if err != nil {
		return nil, err
	}
	return &Channel{
		ID:        channel.ID,
		Name:      channel.Name,
		IsChannel: !isPrivate,
		IsPrivate: isPrivate,
	}, nil
}

// GetAllPublicChannels returns all public channels in the workspace (not just ones the user is a member of)
func (c *Client) GetAllPublicChannels() ([]Channel, error) {
	var channels []Channel

	params := &slack.GetConversationsParameters{
		Types:           []string{"public_channel"},
		ExcludeArchived: true,
		Limit:           200,
	}

	for {
		convs, cursor, err := c.api.GetConversations(params)
		if err != nil {
			return nil, err
		}

		for _, conv := range convs {
			channels = append(channels, Channel{
				ID:          conv.ID,
				Name:        conv.Name,
				IsChannel:   true,
				IsPrivate:   false,
				IsExtShared: conv.IsExtShared,
			})
		}

		if cursor == "" {
			break
		}
		params.Cursor = cursor
	}

	return channels, nil
}

// JoinChannel joins a channel (bot joins itself)
// Uses bot token if available, otherwise falls back to user token
func (c *Client) JoinChannel(channelID string) error {
	api := c.api
	if c.botAPI != nil {
		api = c.botAPI
	}
	_, _, _, err := api.JoinConversation(channelID)
	return err
}

// LeaveChannel leaves a channel
func (c *Client) LeaveChannel(channelID string) (bool, error) {
	return c.api.LeaveConversation(channelID)
}
