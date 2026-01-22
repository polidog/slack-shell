package shell

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyokomi/emoji/v2"
	"github.com/polidog/slack-shell/internal/slack"
)

// ConvertEmoji converts Slack emoji codes (e.g., :smile:) to Unicode emoji
func ConvertEmoji(text string) string {
	return emoji.Sprint(text)
}

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
			if msg.IsBot && msg.BotName != "" {
				userName = msg.BotName
			} else if name, ok := userNames[msg.User]; ok {
				userName = name
			} else {
				userName = msg.User
			}
		}
		if userName == "" && msg.IsBot {
			userName = "bot"
		}

		// Resolve mentions in text and convert emoji
		text := ConvertEmoji(ResolveMentions(msg.Text, userNames))

		// Format the message
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, userName, text))

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
				emojiStr := ConvertEmoji(fmt.Sprintf(":%s:", r.Name))
				reactions = append(reactions, fmt.Sprintf("%s %d", emojiStr, r.Count))
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

  ls              List channels and DMs (uses cache)
  ls -r           List channels and DMs (refresh cache)
  ls dm           List DMs only
  cd #channel     Enter a channel
  cd @user        Enter a DM
  ..              Go back to channel list
  mkdir #channel  Create a public channel
  mkdir -p #chan  Create a private channel
  cat             Show messages (default 20)
  cat -n 50       Show 50 messages
  show            Show channel info and members (default 20)
  show -n 50      Show channel info with 50 members
  browse          Interactive message browser
                  (j/k: navigate, Enter: view thread, r: reply, q: exit)
  live            Live mode with real-time updates and message sending
                  (i: new message, Enter: view thread, r: reply, j/k: navigate, q: exit)
  send <message>  Send a message
  pwd             Show current channel
  source <file>   Switch workspace using config file
  help            Show this help
  exit            Exit the application

Admin commands:
  sudo app install              Join all public channels (for Socket Mode)
  sudo app install #ch1 #ch2    Join specific channels
  sudo app remove               Leave all public channels
  sudo app remove #ch1 #ch2     Leave specific channels
  whoami                        Show current authentication info

Pipe support:
  ls | grep <pattern>     Search channels/DMs by name
  cat | grep <pattern>    Search messages by content

Keyboard shortcuts:
  Ctrl+L                  Refresh screen
  Ctrl+C                  Exit application
  Tab                     Auto-complete
  Up/Down                 Navigate command history
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

// ResolveMentions replaces <@USER_ID> patterns with @username
func ResolveMentions(text string, userNames map[string]string) string {
	// Match <@U12345> or <@U12345|display_name> patterns
	re := regexp.MustCompile(`<@([A-Z0-9]+)(?:\|[^>]*)?>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		// Extract user ID from the match
		matches := re.FindStringSubmatch(match)
		if len(matches) > 1 {
			userID := matches[1]
			if name, ok := userNames[userID]; ok {
				return "@" + name
			}
		}
		return match
	})
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

// FormatChannelInfo formats channel information for display
func FormatChannelInfo(info *slack.ChannelInfo, memberIDs []string, userNames map[string]string, creatorName string, memberLimit int) string {
	var sb strings.Builder

	// Define styles using ANSI colors (adapts to terminal theme)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")) // bright cyan

	privateStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("11")) // bright yellow

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // bright black (gray)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")) // bright white

	accentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")) // bright green

	userStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")) // bright red

	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // bright black (gray)

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("13")) // bright magenta

	// Channel name and type
	if info.IsPrivate {
		sb.WriteString(privateStyle.Render("🔒 "+info.Name) + "\n")
	} else {
		sb.WriteString(titleStyle.Render("# "+info.Name) + "\n")
	}
	sb.WriteString(mutedStyle.Render(strings.Repeat("─", len(info.Name)+4)) + "\n\n")

	// Basic info
	if info.Purpose != "" {
		sb.WriteString(labelStyle.Render("Purpose: ") + valueStyle.Render(info.Purpose) + "\n")
	}
	if info.Topic != "" {
		sb.WriteString(labelStyle.Render("Topic:   ") + valueStyle.Render(info.Topic) + "\n")
	}

	// Creator and created time
	if creatorName != "" {
		created := time.Unix(info.Created, 0)
		sb.WriteString(labelStyle.Render("Created: ") +
			valueStyle.Render(created.Format("2006-01-02")) +
			labelStyle.Render(" by ") +
			userStyle.Render("@"+creatorName) + "\n")
	}

	// Member count
	sb.WriteString(labelStyle.Render("Members: ") + accentStyle.Render(fmt.Sprintf("%d", info.MemberCount)) + "\n")

	// Flags
	if info.IsGeneral {
		sb.WriteString(labelStyle.Render("Type:    ") + accentStyle.Render("General channel (default)") + "\n")
	}
	if info.IsArchived {
		sb.WriteString(labelStyle.Render("Status:  ") + privateStyle.Render("Archived") + "\n")
	}

	// Member list
	if len(memberIDs) > 0 {
		sb.WriteString("\n" + sectionStyle.Render("Member list:") + "\n")
		for _, id := range memberIDs {
			name := id
			if userName, ok := userNames[id]; ok {
				name = userName
			}
			sb.WriteString("  " + userStyle.Render("@"+name) + "\n")
		}
		if info.MemberCount > memberLimit {
			sb.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more (use -n to show more)\n", info.MemberCount-memberLimit)))
		}
	}

	return sb.String()
}
