package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/polidog/slack-tui/internal/config"
	"github.com/polidog/slack-tui/internal/oauth"
	"github.com/polidog/slack-tui/internal/shell"
	"github.com/polidog/slack-tui/internal/slack"
)

type App struct {
	config         *config.Config
	slackClient    *slack.Client
	realtimeClient *slack.RealtimeClient
	program        *tea.Program
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("設定の読み込みに失敗しました: %w", err)
	}

	// Get token
	token, err := getToken(cfg)
	if err != nil {
		return nil, err
	}

	slackClient, err := slack.NewClient(token)
	if err != nil {
		return nil, fmt.Errorf("Slackクライアントの作成に失敗しました: %w", err)
	}

	return &App{
		config:      cfg,
		slackClient: slackClient,
	}, nil
}

func getToken(cfg *config.Config) (string, error) {
	// 1. Check for direct token (environment variable or config file)
	if cfg.HasDirectToken() {
		return cfg.SlackToken, nil
	}

	// 2. Check for saved credentials
	creds, err := config.LoadCredentials()
	if err == nil && creds.AccessToken != "" {
		fmt.Printf("保存済みの認証情報を使用します (ワークスペース: %s)\n", creds.TeamName)
		return creds.AccessToken, nil
	}

	// 3. OAuth flow
	if cfg.HasOAuthConfig() {
		fmt.Println("OAuth認証を開始します...")

		oauthFlow, err := oauth.NewOAuthFlow(cfg)
		if err != nil {
			return "", fmt.Errorf("OAuth初期化に失敗しました: %w", err)
		}

		creds, err := oauthFlow.Start()
		if err != nil {
			return "", fmt.Errorf("OAuth認証に失敗しました: %w", err)
		}

		// Save credentials
		if err := config.SaveCredentials(creds); err != nil {
			fmt.Printf("警告: 認証情報の保存に失敗しました: %v\n", err)
		} else {
			fmt.Println("認証情報を保存しました。")
		}

		return creds.AccessToken, nil
	}

	// 4. No authentication method available
	return "", fmt.Errorf(`認証情報が見つかりません。

以下のいずれかの方法で認証を設定してください:

1. 環境変数を設定:
   export SLACK_TOKEN="xoxp-your-token"

2. OAuth認証を使用 (推奨):
   export SLACK_CLIENT_ID="your-client-id"
   export SLACK_CLIENT_SECRET="your-client-secret"

3. 設定ファイルを作成 (~/.slack-tui/config.yaml):
   slack_token: xoxp-your-token
   または
   client_id: your-client-id
   client_secret: your-client-secret`)
}

func (a *App) Run() error {
	model := shell.NewModel(a.slackClient)

	// Set up realtime client if app token is available
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
		)
		model.SetRealtimeClient(a.realtimeClient)

		go func() {
			if err := a.realtimeClient.Start(); err != nil {
				// Handle error silently for now
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
}

// Logout removes saved credentials
func Logout() error {
	if err := config.DeleteCredentials(); err != nil {
		return fmt.Errorf("ログアウトに失敗しました: %w", err)
	}
	fmt.Println("ログアウトしました。")
	return nil
}
