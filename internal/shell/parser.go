package shell

import (
	"strconv"
	"strings"
)

// CommandType represents the type of command
type CommandType int

const (
	CmdUnknown CommandType = iota
	CmdLs
	CmdCd
	CmdBack
	CmdCat
	CmdTail
	CmdSend
	CmdPwd
	CmdHelp
	CmdExit
)

// Command represents a parsed command
type Command struct {
	Type    CommandType
	Args    []string
	Flags   map[string]string
	RawArgs string
}

// ParseCommand parses a command string into a Command struct
func ParseCommand(input string) Command {
	input = strings.TrimSpace(input)
	if input == "" {
		return Command{Type: CmdUnknown}
	}

	// Handle ".." as a special case
	if input == ".." {
		return Command{Type: CmdBack}
	}

	parts := tokenize(input)
	if len(parts) == 0 {
		return Command{Type: CmdUnknown}
	}

	cmd := Command{
		Type:  parseCommandType(parts[0]),
		Args:  []string{},
		Flags: make(map[string]string),
	}

	// Parse remaining parts as flags and arguments
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if strings.HasPrefix(part, "-") {
			// It's a flag
			flagName := strings.TrimLeft(part, "-")
			// Check if next part is the flag value
			if i+1 < len(parts) && !strings.HasPrefix(parts[i+1], "-") {
				cmd.Flags[flagName] = parts[i+1]
				i++
			} else {
				cmd.Flags[flagName] = "true"
			}
		} else {
			cmd.Args = append(cmd.Args, part)
		}
	}

	// Store raw args for commands like "send" that need the full text
	if cmd.Type == CmdSend && len(parts) > 1 {
		// Find where "send" ends and the message begins
		idx := strings.Index(input, parts[0])
		if idx >= 0 {
			remainder := strings.TrimSpace(input[idx+len(parts[0]):])
			cmd.RawArgs = remainder
		}
	}

	return cmd
}

func parseCommandType(s string) CommandType {
	switch strings.ToLower(s) {
	case "ls":
		return CmdLs
	case "cd":
		return CmdCd
	case "cat":
		return CmdCat
	case "tail":
		return CmdTail
	case "send":
		return CmdSend
	case "pwd":
		return CmdPwd
	case "help":
		return CmdHelp
	case "exit", "quit", "q":
		return CmdExit
	default:
		return CmdUnknown
	}
}

// tokenize splits the input into tokens, respecting quotes
func tokenize(input string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		switch {
		case (r == '"' || r == '\'') && !inQuote:
			inQuote = true
			quoteChar = r
		case r == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// GetFlagInt returns the integer value of a flag, or the default if not set or invalid
func (c *Command) GetFlagInt(name string, defaultVal int) int {
	if val, ok := c.Flags[name]; ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// GetFlagBool returns true if the flag is set
func (c *Command) GetFlagBool(name string) bool {
	_, ok := c.Flags[name]
	return ok
}
