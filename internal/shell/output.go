package shell

import (
	"fmt"
	"strings"
	"time"

	"github.com/polidog/slack-tui/internal/slack"
)

// FormatChannelList formats a list of channels for display
func FormatChannelList(channels []slack.Channel, dms []slack.Channel, userNames map[string]string) string {
	var sb strings.Builder

	if len(channels) > 0 {
		sb.WriteString("Channels:\n")
		for _, ch := range channels {
			prefix := "#"
			if ch.IsPrivate {
				prefix = "🔒"
			}
			sb.WriteString(fmt.Sprintf("  %s %s\n", prefix, ch.Name))
		}
	}

	if len(dms) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Direct Messages:\n")
		for _, dm := range dms {
			name := dm.UserID
			if userName, ok := userNames[dm.UserID]; ok {
				name = userName
			}
			sb.WriteString(fmt.Sprintf("  @ %s\n", name))
		}
	}

	if sb.Len() == 0 {
		return "No channels found."
	}

	return sb.String()
}

// FormatDMList formats only DMs for display
func FormatDMList(dms []slack.Channel, userNames map[string]string) string {
	var sb strings.Builder

	if len(dms) == 0 {
		return "No direct messages found."
	}

	sb.WriteString("Direct Messages:\n")
	for _, dm := range dms {
		name := dm.UserID
		if userName, ok := userNames[dm.UserID]; ok {
			name = userName
		}
		sb.WriteString(fmt.Sprintf("  @ %s\n", name))
	}

	return sb.String()
}

// FormatMessages formats a list of messages for display
func FormatMessages(messages []slack.Message, userNames map[string]string) string {
	var sb strings.Builder

	if len(messages) == 0 {
		return "No messages."
	}

	for _, msg := range messages {
		// Parse timestamp
		ts := parseTimestamp(msg.Timestamp)
		timeStr := ts.Format("15:04")

		// Get user name
		userName := msg.UserName
		if userName == "" {
			if name, ok := userNames[msg.User]; ok {
				userName = name
			} else {
				userName = msg.User
			}
		}
		if userName == "" && msg.IsBot {
			userName = "bot"
		}

		// Format the message
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, userName, msg.Text))

		// Show attachments
		for _, att := range msg.Attachments {
			if att.Title != "" {
				sb.WriteString(fmt.Sprintf("        📎 %s\n", att.Title))
			}
			if att.Text != "" {
				sb.WriteString(fmt.Sprintf("           %s\n", att.Text))
			}
		}

		// Show reactions
		if len(msg.Reactions) > 0 {
			var reactions []string
			for _, r := range msg.Reactions {
				reactions = append(reactions, fmt.Sprintf(":%s: %d", r.Name, r.Count))
			}
			sb.WriteString(fmt.Sprintf("        %s\n", strings.Join(reactions, " ")))
		}

		// Show thread indicator
		if msg.ReplyCount > 0 {
			sb.WriteString(fmt.Sprintf("        └─ %d replies\n", msg.ReplyCount))
		}
	}

	return sb.String()
}

// FormatHelp returns the help text
func FormatHelp() string {
	return `Available commands:

  ls              List channels and DMs
  ls dm           List DMs only
  cd #channel     Enter a channel
  cd @user        Enter a DM
  ..              Go back to channel list
  cat             Show messages (default 20)
  cat -n 50       Show 50 messages
  tail            Stream new messages (press 'q' to stop)
  tail -n 10      Show last 10 messages, then stream
  send <message>  Send a message
  pwd             Show current channel
  source <file>   Switch workspace using config file
  help            Show this help
  exit            Exit the application

Pipe support:
  ls | grep <pattern>     Search channels/DMs by name
  cat | grep <pattern>    Search messages by content
`
}

// FormatError formats an error message
func FormatError(err error) string {
	return fmt.Sprintf("Error: %s", err.Error())
}

// FormatSuccess formats a success message
func FormatSuccess(msg string) string {
	return msg
}

func parseTimestamp(ts string) time.Time {
	// Slack timestamps are in format "1234567890.123456"
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	return time.Unix(sec, 0)
}
