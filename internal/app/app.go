package app

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/polidog/slack-shell/internal/config"
	"github.com/polidog/slack-shell/internal/notification"
	"github.com/polidog/slack-shell/internal/oauth"
	"github.com/polidog/slack-shell/internal/shell"
	"github.com/polidog/slack-shell/internal/slack"
)

type App struct {
	config              *config.Config
	slackClient         *slack.Client
	realtimeClient      *slack.RealtimeClient
	notificationManager *notification.Manager
	program             *tea.Program
	nonInteractive      bool
}

// Option is a functional option for App
type Option func(*App)

// WithNonInteractive sets the app to non-interactive mode (suppresses startup messages)
func WithNonInteractive() Option {
	return func(a *App) {
		a.nonInteractive = true
	}
}

func New(opts ...Option) (*App, error) {
	app := &App{}
	for _, opt := range opts {
		opt(app)
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("設定の読み込みに失敗しました: %w", err)
	}

	// Get tokens
	token, botToken, err := getTokens(cfg, app.nonInteractive)
	if err != nil {
		return nil, err
	}

	slackClient, err := slack.NewClientWithBotToken(token, botToken)
	if err != nil {
		return nil, fmt.Errorf("Slackクライアントの作成に失敗しました: %w", err)
	}

	app.config = cfg
	app.slackClient = slackClient
	return app, nil
}

func getTokens(cfg *config.Config, nonInteractive bool) (string, string, error) {
	// 1. Check for direct token (environment variable or config file)
	if cfg.HasDirectToken() {
		return cfg.SlackToken, "", nil
	}

	// 2. Check for saved credentials
	creds, err := config.LoadCredentials()
	if err == nil && creds.AccessToken != "" {
		if !nonInteractive {
			fmt.Printf("保存済みの認証情報を使用します (ワークスペース: %s)\n", creds.TeamName)
		}
		return creds.AccessToken, creds.BotToken, nil
	}

	// 3. OAuth flow
	if cfg.HasOAuthConfig() {
		if !nonInteractive {
			fmt.Println("OAuth認証を開始します...")
		}

		oauthFlow, err := oauth.NewOAuthFlow(cfg)
		if err != nil {
			return "", "", fmt.Errorf("OAuth初期化に失敗しました: %w", err)
		}

		creds, err := oauthFlow.Start()
		if err != nil {
			return "", "", fmt.Errorf("OAuth認証に失敗しました: %w", err)
		}

		// Save credentials
		if err := config.SaveCredentials(creds); err != nil {
			if !nonInteractive {
				fmt.Printf("警告: 認証情報の保存に失敗しました: %v\n", err)
			}
		} else {
			if !nonInteractive {
				fmt.Println("認証情報を保存しました。")
			}
		}

		return creds.AccessToken, creds.BotToken, nil
	}

	// 4. No authentication method available
	return "", "", fmt.Errorf(`認証情報が見つかりません。

以下のいずれかの方法で認証を設定してください:

1. 環境変数を設定:
   export SLACK_TOKEN="xoxp-your-token"

2. OAuth認証を使用 (推奨):
   export SLACK_CLIENT_ID="your-client-id"
   export SLACK_CLIENT_SECRET="your-client-secret"

3. 設定ファイルを作成 (~/.slack-shell/config.yaml):
   slack_token: xoxp-your-token
   または
   client_id: your-client-id
   client_secret: your-client-secret`)
}

func (a *App) Run() error {
	// Initialize notification manager
	notifyCfg := a.config.GetNotificationConfig()
	a.notificationManager = notification.NewManager(notifyCfg)

	model := shell.NewModel(a.slackClient, a.notificationManager, a.config.GetPromptConfig(), a.config.GetStartupConfig(), a.config.AppToken != "")

	// Set up realtime client if app token is available
	if a.config.Debug {
		if a.config.AppToken != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] App token found, setting up Socket Mode...\n")
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] No app token found, Socket Mode disabled\n")
		}
	}
	if a.config.AppToken != "" {
		a.realtimeClient = slack.NewRealtimeClient(
			a.slackClient,
			a.config.AppToken,
			func(event interface{}) {
				if a.program != nil {
					cmd := model.HandleRealtimeEvent(event)
					if cmd != nil {
						a.program.Send(cmd())
					}
				}
			},
			a.config.Debug,
		)
		model.SetRealtimeClient(a.realtimeClient)

		go func() {
			if a.config.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Starting Socket Mode connection...\n")
			}
			if err := a.realtimeClient.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Socket Mode error: %v\n", err)
			}
		}()
	}

	a.program = tea.NewProgram(model)

	_, err := a.program.Run()
	return err
}

func (a *App) Stop() {
	if a.realtimeClient != nil {
		a.realtimeClient.Stop()
	}
	if a.notificationManager != nil {
		a.notificationManager.Close()
	}
}

// Logout removes saved credentials
func Logout() error {
	if err := config.DeleteCredentials(); err != nil {
		return fmt.Errorf("ログアウトに失敗しました: %w", err)
	}
	fmt.Println("ログアウトしました。")
	return nil
}

// RunCommand executes a command string and exits (non-interactive mode)
func (a *App) RunCommand(commandStr string) error {
	executor := shell.NewExecutor(a.slackClient, a.config.GetPromptConfig(), a.config.AppToken != "")

	// Split by && or ; for multiple commands
	commands := splitCommands(commandStr)

	for _, cmdStr := range commands {
		cmdStr = trimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}

		// Parse the command
		pipeline := shell.ParsePipeline(cmdStr)
		if len(pipeline.Commands) == 0 {
			continue
		}

		// Execute the pipeline
		result := executor.ExecutePipeline(pipeline)

		if result.Error != nil {
			return result.Error
		}

		if result.Output != "" {
			fmt.Println(result.Output)
		}

		if result.Exit {
			break
		}
	}

	return nil
}

// splitCommands splits a command string by && or ;
func splitCommands(s string) []string {
	var result []string
	var current string
	inQuote := false
	quoteChar := rune(0)

	for i, r := range s {
		if (r == '"' || r == '\'') && (i == 0 || s[i-1] != '\\') {
			if !inQuote {
				inQuote = true
				quoteChar = r
			} else if r == quoteChar {
				inQuote = false
			}
			current += string(r)
			continue
		}

		if !inQuote {
			// Check for &&
			if r == '&' && i+1 < len(s) && s[i+1] == '&' {
				result = append(result, current)
				current = ""
				continue
			}
			// Skip the second &
			if r == '&' && i > 0 && s[i-1] == '&' {
				continue
			}
			// Check for ;
			if r == ';' {
				result = append(result, current)
				current = ""
				continue
			}
		}

		current += string(r)
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
