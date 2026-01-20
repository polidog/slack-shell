package shell

import (
	"fmt"
	"strings"

	"github.com/polidog/slack-tui/internal/slack"
)

// Executor handles command execution
type Executor struct {
	client         *slack.Client
	channels       []slack.Channel
	dms            []slack.Channel
	userNames      map[string]string
	currentChannel *slack.Channel
}

// NewExecutor creates a new command executor
func NewExecutor(client *slack.Client) *Executor {
	return &Executor{
		client:    client,
		userNames: make(map[string]string),
	}
}

// ExecuteResult represents the result of a command execution
type ExecuteResult struct {
	Output   string
	Exit     bool
	Error    error
	NeedLoad bool // Indicates if we need to load data first
}

// Execute runs the given command and returns the result
func (e *Executor) Execute(cmd Command) ExecuteResult {
	switch cmd.Type {
	case CmdLs:
		return e.executeLs(cmd)
	case CmdCd:
		return e.executeCd(cmd)
	case CmdBack:
		return e.executeBack()
	case CmdCat:
		return e.executeCat(cmd)
	case CmdSend:
		return e.executeSend(cmd)
	case CmdPwd:
		return e.executePwd()
	case CmdHelp:
		return ExecuteResult{Output: FormatHelp()}
	case CmdExit:
		return ExecuteResult{Exit: true}
	default:
		return ExecuteResult{Output: "Unknown command. Type 'help' for available commands."}
	}
}

func (e *Executor) executeLs(cmd Command) ExecuteResult {
	// Check if we should only show DMs
	dmOnly := len(cmd.Args) > 0 && cmd.Args[0] == "dm"

	var err error

	// Load channels if needed
	if !dmOnly && e.channels == nil {
		e.channels, err = e.client.GetChannels()
		if err != nil {
			return ExecuteResult{Error: fmt.Errorf("failed to load channels: %w", err)}
		}
	}

	// Load DMs
	if e.dms == nil {
		e.dms, err = e.client.GetDMs()
		if err != nil {
			return ExecuteResult{Error: fmt.Errorf("failed to load DMs: %w", err)}
		}
	}

	// Load user names for DMs
	if len(e.dms) > 0 {
		userIDs := make([]string, 0, len(e.dms))
		for _, dm := range e.dms {
			if dm.UserID != "" {
				userIDs = append(userIDs, dm.UserID)
			}
		}
		if len(userIDs) > 0 {
			users, err := e.client.GetUsersInfo(userIDs)
			if err == nil && users != nil {
				for _, u := range *users {
					e.userNames[u.ID] = u.Name
				}
			}
		}
	}

	if dmOnly {
		return ExecuteResult{Output: FormatDMList(e.dms, e.userNames)}
	}

	return ExecuteResult{Output: FormatChannelList(e.channels, e.dms, e.userNames)}
}

func (e *Executor) executeCd(cmd Command) ExecuteResult {
	if len(cmd.Args) == 0 {
		return ExecuteResult{Output: "Usage: cd #channel or cd @user"}
	}

	target := cmd.Args[0]

	// Handle channel
	if strings.HasPrefix(target, "#") {
		channelName := strings.TrimPrefix(target, "#")
		return e.enterChannel(channelName)
	}

	// Handle DM
	if strings.HasPrefix(target, "@") {
		userName := strings.TrimPrefix(target, "@")
		return e.enterDM(userName)
	}

	// Try without prefix - could be either
	// First try as channel
	result := e.enterChannel(target)
	if result.Error == nil {
		return result
	}

	// Then try as DM
	return e.enterDM(target)
}

func (e *Executor) enterChannel(name string) ExecuteResult {
	// Load channels if needed
	if e.channels == nil {
		channels, err := e.client.GetChannels()
		if err != nil {
			return ExecuteResult{Error: fmt.Errorf("failed to load channels: %w", err)}
		}
		e.channels = channels
	}

	// Find the channel
	for _, ch := range e.channels {
		if strings.EqualFold(ch.Name, name) {
			e.currentChannel = &ch
			return ExecuteResult{Output: fmt.Sprintf("Entered #%s", ch.Name)}
		}
	}

	return ExecuteResult{Error: fmt.Errorf("channel not found: %s", name)}
}

func (e *Executor) enterDM(userName string) ExecuteResult {
	// Load DMs if needed
	if e.dms == nil {
		dms, err := e.client.GetDMs()
		if err != nil {
			return ExecuteResult{Error: fmt.Errorf("failed to load DMs: %w", err)}
		}
		e.dms = dms

		// Load user names
		userIDs := make([]string, 0, len(e.dms))
		for _, dm := range e.dms {
			if dm.UserID != "" {
				userIDs = append(userIDs, dm.UserID)
			}
		}
		if len(userIDs) > 0 {
			users, err := e.client.GetUsersInfo(userIDs)
			if err == nil && users != nil {
				for _, u := range *users {
					e.userNames[u.ID] = u.Name
				}
			}
		}
	}

	// Find the DM by user name
	for _, dm := range e.dms {
		name := e.userNames[dm.UserID]
		if strings.EqualFold(name, userName) || strings.EqualFold(dm.UserID, userName) {
			e.currentChannel = &dm
			displayName := name
			if displayName == "" {
				displayName = dm.UserID
			}
			return ExecuteResult{Output: fmt.Sprintf("Entered DM with @%s", displayName)}
		}
	}

	return ExecuteResult{Error: fmt.Errorf("user not found: %s", userName)}
}

func (e *Executor) executeBack() ExecuteResult {
	if e.currentChannel == nil {
		return ExecuteResult{Output: "Already at channel list"}
	}
	e.currentChannel = nil
	return ExecuteResult{Output: "Returned to channel list"}
}

func (e *Executor) executeCat(cmd Command) ExecuteResult {
	if e.currentChannel == nil {
		return ExecuteResult{Output: "Not in a channel. Use 'cd #channel' first."}
	}

	// Get message count from -n flag
	limit := cmd.GetFlagInt("n", 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Get messages
	messages, err := e.client.GetMessages(e.currentChannel.ID, limit)
	if err != nil {
		return ExecuteResult{Error: fmt.Errorf("failed to load messages: %w", err)}
	}

	// Load user names for messages
	userIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.User != "" && msg.UserName == "" {
			userIDs[msg.User] = true
		}
	}

	if len(userIDs) > 0 {
		ids := make([]string, 0, len(userIDs))
		for id := range userIDs {
			ids = append(ids, id)
		}
		users, err := e.client.GetUsersInfo(ids)
		if err == nil && users != nil {
			for _, u := range *users {
				e.userNames[u.ID] = u.Name
			}
		}
	}

	return ExecuteResult{Output: FormatMessages(messages, e.userNames)}
}

func (e *Executor) executeSend(cmd Command) ExecuteResult {
	if e.currentChannel == nil {
		return ExecuteResult{Output: "Not in a channel. Use 'cd #channel' first."}
	}

	message := cmd.RawArgs
	if message == "" && len(cmd.Args) > 0 {
		message = strings.Join(cmd.Args, " ")
	}

	if message == "" {
		return ExecuteResult{Output: "Usage: send <message>"}
	}

	_, err := e.client.PostMessage(e.currentChannel.ID, message)
	if err != nil {
		return ExecuteResult{Error: fmt.Errorf("failed to send message: %w", err)}
	}

	return ExecuteResult{Output: "Message sent."}
}

func (e *Executor) executePwd() ExecuteResult {
	if e.currentChannel == nil {
		return ExecuteResult{Output: "Not in a channel"}
	}

	if e.currentChannel.IsIM {
		name := e.userNames[e.currentChannel.UserID]
		if name == "" {
			name = e.currentChannel.UserID
		}
		return ExecuteResult{Output: fmt.Sprintf("@%s", name)}
	}

	prefix := "#"
	if e.currentChannel.IsPrivate {
		prefix = "🔒"
	}
	return ExecuteResult{Output: fmt.Sprintf("%s%s", prefix, e.currentChannel.Name)}
}

// GetCurrentChannel returns the current channel
func (e *Executor) GetCurrentChannel() *slack.Channel {
	return e.currentChannel
}

// GetPrompt returns the current prompt string
func (e *Executor) GetPrompt() string {
	if e.currentChannel == nil {
		return "slack> "
	}

	if e.currentChannel.IsIM {
		name := e.userNames[e.currentChannel.UserID]
		if name == "" {
			name = e.currentChannel.UserID
		}
		return fmt.Sprintf("@%s> ", name)
	}

	return fmt.Sprintf("#%s> ", e.currentChannel.Name)
}

// HandleIncomingMessage handles a real-time message
func (e *Executor) HandleIncomingMessage(msg slack.IncomingMessage) string {
	// Only show if we're in the same channel
	if e.currentChannel == nil || e.currentChannel.ID != msg.ChannelID {
		return ""
	}

	// Get user name
	userName := msg.UserID
	if name, ok := e.userNames[msg.UserID]; ok {
		userName = name
	} else {
		// Try to fetch user info
		user, err := e.client.GetUserInfo(msg.UserID)
		if err == nil && user != nil {
			e.userNames[msg.UserID] = user.Name
			userName = user.Name
		}
	}

	return fmt.Sprintf("\n[new] %s: %s", userName, msg.Text)
}
