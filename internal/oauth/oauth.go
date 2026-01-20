package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/polidog/slack-tui/internal/config"
)

const (
	slackAuthorizeURL = "https://slack.com/oauth/v2/authorize"
	slackTokenURL     = "https://slack.com/api/oauth.v2.access"
)

// Required scopes for the application
var requiredScopes = []string{
	"channels:read",
	"channels:history",
	"groups:read",
	"groups:history",
	"im:read",
	"im:history",
	"im:write",
	"mpim:read",
	"mpim:history",
	"users:read",
	"chat:write",
}

type OAuthFlow struct {
	clientID     string
	clientSecret string
	redirectPort int
	state        string
	server       *http.Server
	resultChan   chan *OAuthResult
}

type OAuthResult struct {
	Credentials *config.Credentials
	Error       error
}

type tokenResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	AuthedUser  struct {
		ID          string `json:"id"`
		Scope       string `json:"scope"`
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	} `json:"authed_user"`
	Team struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
}

func NewOAuthFlow(cfg *config.Config) (*OAuthFlow, error) {
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	return &OAuthFlow{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectPort: cfg.RedirectPort,
		state:        state,
		resultChan:   make(chan *OAuthResult, 1),
	}, nil
}

func (o *OAuthFlow) Start() (*config.Credentials, error) {
	// Start local HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", o.handleCallback)
	mux.HandleFunc("/", o.handleRoot)

	o.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", o.redirectPort),
		Handler: mux,
	}

	go func() {
		if err := o.server.ListenAndServe(); err != http.ErrServerClosed {
			o.resultChan <- &OAuthResult{Error: err}
		}
	}()

	// Wait a moment for the server to start
	time.Sleep(100 * time.Millisecond)

	// Open browser
	authURL := o.buildAuthURL()
	fmt.Printf("\n認証のためブラウザを開いています...\n")
	fmt.Printf("自動で開かない場合は以下のURLにアクセスしてください:\n%s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("ブラウザを開けませんでした: %v\n", err)
	}

	// Wait for result
	result := <-o.resultChan

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	o.server.Shutdown(ctx)

	if result.Error != nil {
		return nil, result.Error
	}

	return result.Credentials, nil
}

func (o *OAuthFlow) buildAuthURL() string {
	params := url.Values{}
	params.Set("client_id", o.clientID)
	params.Set("scope", strings.Join(requiredScopes, ","))
	params.Set("user_scope", strings.Join(requiredScopes, ","))
	params.Set("redirect_uri", fmt.Sprintf("http://localhost:%d/callback", o.redirectPort))
	params.Set("state", o.state)

	return fmt.Sprintf("%s?%s", slackAuthorizeURL, params.Encode())
}

func (o *OAuthFlow) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Slack TUI - OAuth</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1>Slack TUI OAuth</h1>
<p>認証を開始するには <a href="%s">こちら</a> をクリックしてください。</p>
</body>
</html>`, o.buildAuthURL())
}

func (o *OAuthFlow) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Check for error
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		o.sendError(w, fmt.Errorf("OAuth error: %s", errMsg))
		return
	}

	// Verify state
	state := r.URL.Query().Get("state")
	if state != o.state {
		o.sendError(w, fmt.Errorf("invalid state parameter"))
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		o.sendError(w, fmt.Errorf("no authorization code received"))
		return
	}

	// Exchange code for token
	creds, err := o.exchangeCodeForToken(code)
	if err != nil {
		o.sendError(w, err)
		return
	}

	// Success
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Slack TUI - 認証成功</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1>✅ 認証成功!</h1>
<p>ワークスペース: <strong>%s</strong></p>
<p>このウィンドウを閉じて、ターミナルに戻ってください。</p>
<script>setTimeout(function() { window.close(); }, 3000);</script>
</body>
</html>`, creds.TeamName)

	o.resultChan <- &OAuthResult{Credentials: creds}
}

func (o *OAuthFlow) sendError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Slack TUI - エラー</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1>❌ エラー</h1>
<p>%s</p>
<p>ターミナルに戻って再度お試しください。</p>
</body>
</html>`, err.Error())

	o.resultChan <- &OAuthResult{Error: err}
}

func (o *OAuthFlow) exchangeCodeForToken(code string) (*config.Credentials, error) {
	data := url.Values{}
	data.Set("client_id", o.clientID)
	data.Set("client_secret", o.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", fmt.Sprintf("http://localhost:%d/callback", o.redirectPort))

	resp, err := http.PostForm(slackTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !tokenResp.OK {
		return nil, fmt.Errorf("token exchange failed: %s", tokenResp.Error)
	}

	// Use user token if available, otherwise use bot token
	accessToken := tokenResp.AuthedUser.AccessToken
	if accessToken == "" {
		accessToken = tokenResp.AccessToken
	}

	scope := tokenResp.AuthedUser.Scope
	if scope == "" {
		scope = tokenResp.Scope
	}

	return &config.Credentials{
		AccessToken: accessToken,
		TokenType:   tokenResp.TokenType,
		Scope:       scope,
		UserID:      tokenResp.AuthedUser.ID,
		TeamID:      tokenResp.Team.ID,
		TeamName:    tokenResp.Team.Name,
	}, nil
}

func generateState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
