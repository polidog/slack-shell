# Slack Shell

A terminal-based Slack client built with Go and Bubble Tea.
Navigate Slack with familiar shell commands.

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)

[日本語版 README](README.ja.md)

## Features

- Browse channels and direct messages
- View and send messages
- Real-time message streaming with `tail` command
- **`browse` command** - Interactive message browser with thread view and reply support
- Real-time updates via Socket Mode (optional)
- **OAuth authentication** - Easy browser-based login
- **Shell-like UI** - Familiar command interface
- **Channel creation** - Create public/private channels with `mkdir` command
- **Multi-workspace support** - Switch with `source` command
- **Pipe support** - Search with `ls | grep` and `cat | grep`
- **Notifications** - Terminal bell, desktop notifications, title updates, visual alerts
- **Tab completion** - Auto-complete channel and user names with `cd`

## Quick Start

### 1. Install

```bash
# Install with go install
go install github.com/polidog/slack-shell/cmd/slack-shell@latest

# Or build from source
git clone https://github.com/polidog/slack-shell.git
cd slack-shell
go build ./cmd/slack-shell
```

### 2. Create a Slack App

1. Go to https://api.slack.com/apps
2. Click **Create New App** → **From scratch**
3. Enter an App name (e.g., `My TUI Client`) and select your workspace
4. Click **Create App**

### 3. Configure Scopes

1. Select **OAuth & Permissions** from the left menu
2. Scroll to the **Scopes** section
3. Add the following **User Token Scopes**:

| Scope | Description |
|-------|-------------|
| `channels:read` | List public channels |
| `channels:write` | Create public channels |
| `channels:history` | Read public channel messages |
| `groups:read` | List private channels |
| `groups:write` | Create private channels |
| `groups:history` | Read private channel messages |
| `im:read` | List DMs |
| `im:history` | Read DM messages |
| `im:write` | Send DMs |
| `mpim:read` | List group DMs |
| `mpim:history` | Read group DM messages |
| `users:read` | View user info |
| `chat:write` | Send messages |
| `team:read` | View workspace info (for prompt display) |

### 4. Set Redirect URL

1. On the **OAuth & Permissions** page, find **Redirect URLs**
2. Click **Add New Redirect URL**
3. Enter the following and click **Add** → **Save URLs**:
   ```
   https://localhost:8080/callback
   ```

### 5. Get Client ID and Secret

1. Select **Basic Information** from the left menu
2. In the **App Credentials** section:
   - Copy the `Client ID`
   - Click **Show** on `Client Secret` and copy it

### 6. Run the App

```bash
# Set environment variables
export SLACK_CLIENT_ID="your-client-id"
export SLACK_CLIENT_SECRET="your-client-secret"

# Run
./slack-shell
```

A browser will open automatically with the Slack authorization page.
Click **Allow** to complete authentication.

> **Note**: You may see a "This connection is not secure" warning during the OAuth callback.
> This is because a self-signed certificate is used. Click "Advanced" → "Proceed to localhost" to continue.

## Basic Usage

### Commands

```
slack> ls                    # List channels
slack> ls dm                 # List DMs only
slack> cd #general           # Enter a channel
slack> cd @john              # Enter a DM
slack> ..                    # Go back to channel list
slack> mkdir #new-channel    # Create a public channel
slack> mkdir -p #private     # Create a private channel
slack> cat                   # Show messages (default 20)
slack> cat -n 50             # Show 50 messages
slack> tail                  # Stream new messages in real-time
slack> tail -n 10            # Show last 10, then stream
slack> browse                # Interactive message browser
slack> send Hello world      # Send a message
slack> pwd                   # Show current channel
slack> source ~/work.yaml    # Switch workspace
slack> help                  # Show help
slack> exit                  # Exit

# Pipe support
slack> ls | grep dev         # Search channels by name
slack> cat | grep error      # Search messages by content
```

### Example Session

```
slack> ls
Channels:
  # general
  # random
  # dev

Direct Messages:
  @ alice
  @ bob

slack> cd #general
Entered #general

#general> cat
[10:30] alice: Good morning everyone
[10:32] bob: Morning!
        └─ 3 replies

#general> send Hello!
Message sent.

#general> tail
[10:30] alice: Good morning everyone
[10:32] bob: Morning!
Tailing messages... (press 'q' or Ctrl+C to stop)
>>> Watching for new messages (q to quit) <<<
```

## CLI Options

```bash
# Normal startup (interactive mode)
./slack-shell

# One-liner execution (-c option)
./slack-shell -c "ls"
./slack-shell -c "cd #general && cat -n 5"
./slack-shell -c "cd @john && send Good morning"
./slack-shell -c "ls | grep dev"

# Logout (delete saved credentials)
./slack-shell logout

# Generate sample config file
./slack-shell config init                    # Create at ~/.config/slack-shell/config.yaml
./slack-shell config init ~/work.yaml        # Create at specified path
./slack-shell config init ~/work.yaml -f     # Overwrite if exists
```

### config init

Generate a sample configuration file with all available options documented.

```bash
# Create default config
./slack-shell config init

# Create workspace-specific config for use with `source` command
./slack-shell config init ~/work-slack.yaml
./slack-shell config init ~/personal-slack.yaml

# Then switch workspaces in the app
slack> source ~/work-slack.yaml
```

### -c Option

The `-c` option executes commands without entering interactive mode.
Useful for scripting and cron jobs.

```bash
# Chain multiple commands with && or ;
./slack-shell -c "cd #general && send Daily standup starting!"

# Pipes work too
./slack-shell -c "cd #general && cat | grep meeting"

# Example: Scheduled message with cron
0 9 * * 1-5 /path/to/slack-shell -c "cd #general && send Good morning everyone!"
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate command history |
| `Tab` | Auto-complete channel/user names for `cd` |
| `Ctrl+C` | Exit (or stop tail mode) |
| `q` | Stop tail/browse mode |
| `j` / `k` | Navigate messages in browse mode |
| `Enter` | View thread in browse mode |
| `r` | Reply in browse mode |

## Browse Command

The `browse` command provides an interactive interface for browsing and replying to messages.

```bash
#general> browse         # Start interactive browser
```

**Browse mode key bindings:**

| Key | Action |
|-----|--------|
| `↑` / `k` | Move to previous message |
| `↓` / `j` | Move to next message |
| `Enter` | View thread replies |
| `r` | Reply to selected message (creates/extends thread) |
| `Esc` | Close thread view / cancel input |
| `q` | Exit browse mode |

## Tab Completion

Press Tab while typing a `cd` command to auto-complete channel and user names.

```
slack> cd #          # Tab → show channel candidates
slack> cd #gen       # Tab → complete to #general
slack> cd @          # Tab → show user candidates
slack> cd @ali       # Tab → complete to @alice
```

- **`cd #` + Tab**: Complete channel names only
- **`cd @` + Tab**: Complete user names (DM recipients) only
- **`cd ` + Tab**: Show both channels and users
- **Multiple Tabs**: Cycle through candidates

## Multi-Workspace

Use the `source` command to switch between workspaces.

```bash
# Create workspace config files
cat > ~/work-slack.yaml << EOF
slack_token: xoxp-your-work-token
EOF

cat > ~/personal-slack.yaml << EOF
slack_token: xoxp-your-personal-token
EOF

# Switch in the app
slack> source ~/work-slack.yaml
Switched to workspace: Work Inc.

work> source ~/personal-slack.yaml
Switched to workspace: Personal
personal>
```

## Notifications

Receive notifications when messages arrive in other channels.

### Notification Types

| Type | Description |
|------|-------------|
| **Terminal Bell** | Beep sound via `\a` character |
| **Desktop** | OS native notifications (Linux/macOS/Windows) |
| **Title** | Show unread count in terminal title (e.g., `Slack Shell (3)`) |
| **Visual** | Display notification area at top of screen |

### Configuration

Add a `notifications` section to `~/.config/slack-shell/config.yaml`:

```yaml
notifications:
  enabled: true

  bell:
    enabled: true
    mentions_only: false

  desktop:
    enabled: true
    mentions_only: false

  title:
    enabled: true
    format: "Slack Shell (%d)"
    base_title: "Slack Shell"

  visual:
    enabled: true
    max_items: 5
    dismiss_after: 10

  mute_channels: []
  dnd: false
```

## Real-time Updates (Socket Mode)

For real-time message streaming (required for `tail` command):

1. Enable **Socket Mode** in your Slack App settings
2. Go to **Basic Information** → **App-Level Tokens** and create a new token
   - Token Name: any name (e.g., `socket-token`)
   - Scope: `connections:write`
3. Set the generated token (`xapp-` prefix):

```bash
export SLACK_APP_TOKEN="xapp-your-app-token"
```

## Configuration File

`~/.config/slack-shell/config.yaml` (or `$XDG_CONFIG_HOME/slack-shell/config.yaml`):

> **Note**: For backward compatibility, `~/.slack-shell/config.yaml` is also supported.

```yaml
# OAuth authentication (recommended)
client_id: your-client-id
client_secret: your-client-secret

# Or direct token
slack_token: xoxp-your-token

# Socket Mode (optional, required for tail)
app_token: xapp-your-app-token

# Callback port (default: 8080)
redirect_port: 8080

# Prompt customization (optional)
prompt:
  format: "{workspace} {location}> "
```

## Prompt Customization

Customize the prompt display with template variables in `~/.config/slack-shell/config.yaml`:

```yaml
prompt:
  format: "slack-shell > {workspace}{location}"
```

### Available Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `{workspace}` | Workspace name | `MyCompany` |
| `{location}` | Current channel/DM with prefix | `#general`, `@alice`, or empty |
| `{channel}` | Channel name only (no prefix) | `general` |
| `{user}` | User name only (no prefix) | `alice` |

### Example Formats

```yaml
# Default format
prompt:
  format: "{workspace} {location}> "
# Result: MyCompany #general>

# Shell-style
prompt:
  format: "slack-shell > {workspace}{location}"
# Result: slack-shell > MyCompany#general

# Minimal
prompt:
  format: "{location}$ "
# Result: #general$

# With brackets
prompt:
  format: "[{workspace}:{channel}]$ "
# Result: [MyCompany:general]$
```

## Startup Customization

Customize the startup message, banner, and auto-execute commands in `~/.config/slack-shell/config.yaml`:

```yaml
startup:
  # Single line welcome message
  # Available variables: {workspace}
  message: "Welcome to Slack Shell - {workspace}"

  # Multi-line banner (overrides message if set)
  banner: |
    ╔═══════════════════════════════════════╗
    ║  Slack Shell v1.0                     ║
    ║  Workspace: {workspace}               ║
    ╚═══════════════════════════════════════╝

  # Commands to execute automatically at startup
  init_commands:
    - "cd #general"
    - "cat -n 5"
```

### Options

| Option | Description |
|--------|-------------|
| `message` | Single line welcome message (default: "Welcome to Slack Shell - {workspace}") |
| `banner` | Multi-line ASCII art banner (overrides `message` if set) |
| `init_commands` | List of commands to execute at startup (like `.bashrc`) |

### Example: Auto-enter Channel

```yaml
startup:
  message: "Welcome back! Entering #general..."
  init_commands:
    - "cd #general"
```

## Authentication Methods

### Method 1: OAuth (Recommended)

Environment variables:
```bash
export SLACK_CLIENT_ID="your-client-id"
export SLACK_CLIENT_SECRET="your-client-secret"
./slack-shell
```

Or config file `~/.config/slack-shell/config.yaml`:
```yaml
client_id: your-client-id
client_secret: your-client-secret
```

### Method 2: Direct Token

```bash
export SLACK_TOKEN="xoxp-your-token"
./slack-shell
```

## Troubleshooting

### "No credentials found" error
- Verify environment variables or config file are set correctly
- For OAuth, both Client ID and Client Secret are required

### Browser doesn't open
- Manually copy the URL shown in terminal and open in browser

### "invalid_client_id" error
- Verify Client ID is correct
- Ensure Slack App hasn't been deleted

### Channels not showing
- Verify all required scopes are added
- Reinstall the Slack App to your workspace

### tail command not updating in real-time
- Verify `SLACK_APP_TOKEN` is set
- Ensure Socket Mode is enabled

### Logout

```bash
./slack-shell logout
```

## Development

### Build

```bash
go build ./cmd/slack-shell
```

### Test

```bash
go test ./...
```

## Project Structure

```
slack-shell/
├── cmd/slack-shell/main.go     # Entry point
├── internal/
│   ├── app/app.go              # Application initialization
│   ├── config/config.go        # Configuration management
│   ├── oauth/oauth.go          # OAuth flow
│   ├── notification/           # Notification system
│   │   ├── config.go
│   │   ├── notification.go
│   │   ├── manager.go
│   │   ├── bell.go
│   │   ├── desktop.go
│   │   ├── title.go
│   │   └── visual.go
│   ├── slack/                  # Slack API client
│   │   ├── client.go
│   │   ├── channels.go
│   │   ├── messages.go
│   │   ├── threads.go
│   │   └── realtime.go
│   └── shell/                  # Shell UI components
│       ├── model.go
│       ├── commands.go
│       ├── parser.go
│       ├── browse.go
│       └── output.go
├── go.mod
├── go.sum
└── README.md
```

## License

MIT
