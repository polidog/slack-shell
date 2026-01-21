package slack

import (
	"github.com/slack-go/slack"
)

func (c *Client) GetThreadReplies(channelID, threadTS string) ([]Message, error) {
	params := &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
		Limit:     100,
	}

	msgs, _, _, err := c.api.GetConversationReplies(params)
	if err != nil {
		return nil, err
	}

	var messages []Message
	for _, msg := range msgs {
		// Get bot name from BotProfile or Username field
		botName := msg.Username
		if msg.BotProfile != nil && msg.BotProfile.Name != "" {
			botName = msg.BotProfile.Name
		}

		m := Message{
			Timestamp:  msg.Timestamp,
			User:       msg.User,
			Text:       msg.Text,
			ThreadTS:   msg.ThreadTimestamp,
			ReplyCount: msg.ReplyCount,
			IsBot:      msg.BotID != "",
			BotID:      msg.BotID,
			BotName:    botName,
		}

		for _, r := range msg.Reactions {
			m.Reactions = append(m.Reactions, Reaction{
				Name:  r.Name,
				Count: r.Count,
				Users: r.Users,
			})
		}

		for _, a := range msg.Attachments {
			m.Attachments = append(m.Attachments, Attachment{
				Title: a.Title,
				Text:  a.Text,
				Color: a.Color,
			})
		}

		messages = append(messages, m)
	}

	return messages, nil
}
