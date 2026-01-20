package slack

import (
	"time"

	"github.com/slack-go/slack"
)

type Message struct {
	Timestamp   string
	User        string
	UserName    string
	Text        string
	ThreadTS    string
	ReplyCount  int
	Reactions   []Reaction
	Attachments []Attachment
	IsBot       bool
	BotID       string
}

type Reaction struct {
	Name  string
	Count int
	Users []string
}

type Attachment struct {
	Title string
	Text  string
	Color string
}

func (c *Client) GetMessages(channelID string, limit int) ([]Message, error) {
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Limit:     limit,
	}

	history, err := c.api.GetConversationHistory(params)
	if err != nil {
		return nil, err
	}

	var messages []Message
	for _, msg := range history.Messages {
		m := Message{
			Timestamp:  msg.Timestamp,
			User:       msg.User,
			Text:       msg.Text,
			ThreadTS:   msg.ThreadTimestamp,
			ReplyCount: msg.ReplyCount,
			IsBot:      msg.BotID != "",
			BotID:      msg.BotID,
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

	// Reverse to show oldest first
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (c *Client) PostMessage(channelID, text string) (string, error) {
	_, ts, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
	)
	return ts, err
}

func (c *Client) PostThreadReply(channelID, threadTS, text string) (string, error) {
	_, ts, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(threadTS),
	)
	return ts, err
}

func ParseTimestamp(ts string) time.Time {
	// Slack timestamps are in format "1234567890.123456"
	var sec, nsec int64
	_, _ = sscanf(ts, "%d.%d", &sec, &nsec)
	return time.Unix(sec, nsec*1000)
}

func sscanf(s, format string, a ...interface{}) (int, error) {
	var n int
	_, err := time.Parse("2006-01-02", s)
	if err == nil {
		return 0, nil
	}
	// Simple parsing for Slack timestamp format
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			if len(a) > 0 {
				if p, ok := a[0].(*int64); ok {
					var val int64
					for j := 0; j < i; j++ {
						val = val*10 + int64(s[j]-'0')
					}
					*p = val
					n++
				}
			}
			if len(a) > 1 {
				if p, ok := a[1].(*int64); ok {
					var val int64
					for j := i + 1; j < len(s); j++ {
						val = val*10 + int64(s[j]-'0')
					}
					*p = val
					n++
				}
			}
			break
		}
	}
	return n, nil
}
