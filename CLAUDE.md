# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Slack Shell is a terminal-based Slack client written in Go using the Bubble Tea TUI framework. It provides a shell-like interface with Unix-style commands (ls, cd, cat, etc.) for navigating Slack workspaces.

## Build and Run Commands

```bash
# Build
go build ./cmd/slack-shell

# Run (requires SLACK_CLIENT_ID and SLACK_CLIENT_SECRET env vars for OAuth)
./slack-shell

# Run with non-interactive command execution
./slack-shell -c "ls"
./slack-shell -c "cd #general && cat -n 5"

# Logout (delete saved credentials)
./slack-shell logout

# Run tests
go test ./...
```

## Architecture

### Entry Point and Application Layer
- `cmd/slack-shell/main.go` - CLI entry point, handles `-c` flag for non-interactive mode and `logout` subcommand
- `internal/app/app.go` - Application initialization, wires together Slack client, realtime client, notification manager, and shell model

### Shell Layer (`internal/shell/`)
The shell package implements the TUI using Bubble Tea:
- `model.go` - Main Bubble Tea model, handles key events, browse mode, live mode, and tab completion
- `commands.go` - `Executor` struct that executes shell commands (ls, cd, cat, send, etc.) and manages state (current channel, cached channels/DMs/usernames)
- `parser.go` - Command parsing, handles flags and arguments for each command type
- `output.go` - Formats command output (channel lists, messages, errors, help text)
- `browse.go` - Browse mode model for interactive message viewing with thread support
- `live.go` - Live mode model for real-time message viewing and sending

### Slack API Layer (`internal/slack/`)
- `client.go` - Main Slack API client wrapper, wraps `slack-go/slack`
- `channels.go` - Channel listing (public, private, group DMs)
- `messages.go` - Message fetching and posting
- `threads.go` - Thread message handling
- `realtime.go` - Socket Mode client for real-time message streaming (used by `live` command)

### Supporting Packages
- `internal/config/` - Configuration loading from `~/.slack-shell/config.yaml` and environment variables
- `internal/oauth/` - OAuth flow implementation with local HTTPS callback server
- `internal/notification/` - Notification system (bell, desktop, terminal title, visual notifications)

### Data Flow
1. User types command -> `shell.Model.Update()` captures input
2. Command parsed by `shell.ParseCommand()` or `shell.ParsePipeline()`
3. `shell.Executor.Execute()` runs command, calls appropriate Slack API methods
4. Output formatted by `shell.Format*()` functions
5. Result displayed via `shell.Model.View()`

For real-time messages (`live` command):
1. `slack.RealtimeClient` receives events via Socket Mode
2. Events sent to Bubble Tea program via callback
3. `shell.LiveModel` displays incoming messages in real-time

## Key Design Patterns

- **Shell metaphor**: Channels are directories, messages are file contents. `cd #channel` enters a channel, `cat` shows messages.
- **Lazy loading**: Channels, DMs, and user names are fetched on first use and cached in `Executor`
- **Pipeline support**: Commands can be piped (e.g., `ls | grep dev`, `cat | grep keyword`)
- **Multi-workspace**: `source <config-file>` switches workspaces by loading a different token

## Environment Variables

- `SLACK_TOKEN` - Direct user token (xoxp-...)
- `SLACK_CLIENT_ID` / `SLACK_CLIENT_SECRET` - OAuth credentials
- `SLACK_APP_TOKEN` - App-level token (xapp-...) for Socket Mode (enables `live` command)
