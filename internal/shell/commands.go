package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/polidog/slack-tui/internal/config"
	"github.com/polidog/slack-tui/internal/slack"
)

// Executor handles command execution
type Executor struct {
	client         *slack.Client
	channels       []slack.Channel
	dms            []slack.Channel
	userNames      map[string]string
	currentChannel *slack.Channel
	workspaceName  string
}

// NewExecutor creates a new command executor
func NewExecutor(client *slack.Client) *Executor {
	workspaceName := "slack"
	if info, err := client.GetTeamInfo(); err == nil && info != nil {
		workspaceName = info.Name
	}

	return &Executor{
		client:        client,
		userNames:     make(map[string]string),
		workspaceName: workspaceName,
	}
}

// ExecuteResult represents the result of a command execution
type ExecuteResult struct {
	Output          string
	Exit            bool
	Error           error
	NeedLoad        bool         // Indicates if we need to load data first
	SwitchWorkspace *SwitchWorkspaceResult // Indicates workspace switch is requested
}

// SwitchWorkspaceResult contains info for switching workspace
type SwitchWorkspaceResult struct {
	Config   *config.Config
	Client   *slack.Client
	TeamName string
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
	case CmdSource:
		return e.executeSource(cmd)
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

// GetWorkspaceName returns the current workspace name
func (e *Executor) GetWorkspaceName() string {
	return e.workspaceName
}

// GetPrompt returns the current prompt string
func (e *Executor) GetPrompt() string {
	if e.currentChannel == nil {
		return fmt.Sprintf("%s> ", e.workspaceName)
	}

	if e.currentChannel.IsIM {
		name := e.userNames[e.currentChannel.UserID]
		if name == "" {
			name = e.currentChannel.UserID
		}
		return fmt.Sprintf("%s @%s> ", e.workspaceName, name)
	}

	return fmt.Sprintf("%s #%s> ", e.workspaceName, e.currentChannel.Name)
}

func (e *Executor) executeSource(cmd Command) ExecuteResult {
	if len(cmd.Args) == 0 {
		return ExecuteResult{Output: "Usage: source <config-file-path>"}
	}

	path := cmd.Args[0]

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ExecuteResult{Error: fmt.Errorf("failed to get home directory: %w", err)}
		}
		path = filepath.Join(home, path[1:])
	}

	// Make absolute path
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return ExecuteResult{Error: fmt.Errorf("failed to get current directory: %w", err)}
		}
		path = filepath.Join(cwd, path)
	}

	// Load config from file
	cfg, err := config.LoadFromPath(path)
	if err != nil {
		return ExecuteResult{Error: fmt.Errorf("failed to load config: %w", err)}
	}

	// Get token from config
	token := cfg.SlackToken
	if token == "" {
		return ExecuteResult{Error: fmt.Errorf("slack_token not found in config file")}
	}

	// Create new client
	client, err := slack.NewClient(token)
	if err != nil {
		return ExecuteResult{Error: fmt.Errorf("failed to create Slack client: %w", err)}
	}

	// Get team info for display
	teamName := "Unknown"
	if info, err := client.GetTeamInfo(); err == nil && info != nil {
		teamName = info.Name
	}

	return ExecuteResult{
		SwitchWorkspace: &SwitchWorkspaceResult{
			Config:   cfg,
			Client:   client,
			TeamName: teamName,
		},
	}
}

// SwitchClient switches the executor to use a new client
func (e *Executor) SwitchClient(client *slack.Client) {
	e.client = client
	e.channels = nil
	e.dms = nil
	e.userNames = make(map[string]string)
	e.currentChannel = nil

	// Update workspace name
	e.workspaceName = "slack"
	if info, err := client.GetTeamInfo(); err == nil && info != nil {
		e.workspaceName = info.Name
	}
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

// ExecutePipeline executes a pipeline of commands
func (e *Executor) ExecutePipeline(pipeline Pipeline) ExecuteResult {
	if len(pipeline.Commands) == 0 {
		return ExecuteResult{Output: ""}
	}

	// Execute first command
	result := e.Execute(pipeline.Commands[0])
	if result.Error != nil || result.Exit || len(pipeline.Commands) == 1 {
		return result
	}

	// Pipe output through remaining commands
	currentOutput := result.Output
	for i := 1; i < len(pipeline.Commands); i++ {
		cmd := pipeline.Commands[i]
		switch cmd.Type {
		case CmdGrep:
			currentOutput = e.executeGrep(cmd, currentOutput)
		default:
			return ExecuteResult{Error: fmt.Errorf("cannot pipe to '%s'", getCommandName(cmd.Type))}
		}
	}

	return ExecuteResult{Output: currentOutput}
}

// executeGrep filters input by pattern
func (e *Executor) executeGrep(cmd Command, input string) string {
	if len(cmd.Args) == 0 {
		return input
	}

	pattern := strings.ToLower(cmd.Args[0])
	caseInsensitive := cmd.GetFlagBool("i")

	lines := strings.Split(input, "\n")
	var matched []string

	for _, line := range lines {
		searchLine := line
		searchPattern := pattern
		if caseInsensitive || true { // Always case-insensitive for now
			searchLine = strings.ToLower(line)
			searchPattern = strings.ToLower(pattern)
		}
		if strings.Contains(searchLine, searchPattern) {
			matched = append(matched, line)
		}
	}

	if len(matched) == 0 {
		return "No matches found."
	}

	return strings.Join(matched, "\n")
}

// getCommandName returns the name of a command type
func getCommandName(t CommandType) string {
	switch t {
	case CmdLs:
		return "ls"
	case CmdCd:
		return "cd"
	case CmdBack:
		return ".."
	case CmdCat:
		return "cat"
	case CmdTail:
		return "tail"
	case CmdSend:
		return "send"
	case CmdPwd:
		return "pwd"
	case CmdHelp:
		return "help"
	case CmdExit:
		return "exit"
	case CmdSource:
		return "source"
	case CmdGrep:
		return "grep"
	default:
		return "unknown"
	}
}

// GetChannelName returns the name of a channel by its ID
func (e *Executor) GetChannelName(channelID string) string {
	// Check in regular channels
	for _, ch := range e.channels {
		if ch.ID == channelID {
			return ch.Name
		}
	}

	// Check in DMs
	for _, dm := range e.dms {
		if dm.ID == channelID {
			if name, ok := e.userNames[dm.UserID]; ok {
				return name
			}
			return dm.UserID
		}
	}

	return channelID
}

// GetUserName returns the name of a user by their ID
func (e *Executor) GetUserName(userID string) string {
	if name, ok := e.userNames[userID]; ok {
		return name
	}

	// Try to fetch user info
	user, err := e.client.GetUserInfo(userID)
	if err == nil && user != nil {
		e.userNames[userID] = user.Name
		return user.Name
	}

	return userID
}

// IsMentionedInMessage checks if the current user is mentioned in the message
func (e *Executor) IsMentionedInMessage(text string) bool {
	// Check for @here, @channel, @everyone
	if strings.Contains(text, "<!here>") ||
		strings.Contains(text, "<!channel>") ||
		strings.Contains(text, "<!everyone>") {
		return true
	}

	// Check for direct mention (<@USER_ID>)
	// We would need the current user ID to check this properly
	// For now, return false as we don't have access to current user ID here
	return false
}

// IsIMChannel checks if a channel ID is a direct message channel
func (e *Executor) IsIMChannel(channelID string) bool {
	for _, dm := range e.dms {
		if dm.ID == channelID {
			return true
		}
	}
	return false
}

// GetCompletions returns completion candidates for cd command
func (e *Executor) GetCompletions(prefix string) []string {
	var candidates []string

	// Determine what to complete based on prefix
	showChannels := true
	showUsers := true
	searchTerm := prefix

	if strings.HasPrefix(prefix, "#") {
		showUsers = false
		searchTerm = strings.TrimPrefix(prefix, "#")
	} else if strings.HasPrefix(prefix, "@") {
		showChannels = false
		searchTerm = strings.TrimPrefix(prefix, "@")
	}

	searchTerm = strings.ToLower(searchTerm)

	// Add channel candidates
	if showChannels && e.channels != nil {
		for _, ch := range e.channels {
			name := "#" + ch.Name
			if searchTerm == "" || strings.HasPrefix(strings.ToLower(ch.Name), searchTerm) {
				candidates = append(candidates, name)
			}
		}
	}

	// Add user candidates (from DMs)
	if showUsers && e.dms != nil {
		for _, dm := range e.dms {
			userName := e.userNames[dm.UserID]
			if userName == "" {
				continue
			}
			name := "@" + userName
			if searchTerm == "" || strings.HasPrefix(strings.ToLower(userName), searchTerm) {
				candidates = append(candidates, name)
			}
		}
	}

	return candidates
}
