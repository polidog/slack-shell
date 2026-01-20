# Slack TUI

GoとBubble Teaで作られたターミナルベースのSlackクライアント。
シェルコマンド風のインターフェースで直感的に操作できます。

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)

## 機能

- 📺 チャンネルとダイレクトメッセージの閲覧
- 💬 メッセージ履歴の表示・投稿
- 🔄 `tail` コマンドでメッセージをリアルタイムストリーミング
- ⚡ Socket Modeによるリアルタイム更新（オプション）
- 🔐 **OAuth認証対応** - ブラウザで簡単ログイン
- 🐚 **シェルライクなUI** - 使い慣れたコマンド操作

## クイックスタート

### 1. インストール

```bash
# Go installでインストール
go install github.com/polidog/slack-tui/cmd/slack-tui@latest

# または、ソースからビルド
git clone https://github.com/polidog/slack-tui.git
cd slack-tui
go build ./cmd/slack-tui
```

### 2. Slack Appを作成

1. https://api.slack.com/apps にアクセス
2. **Create New App** → **From scratch** を選択
3. App名（例: `My TUI Client`）とワークスペースを選択
4. **Create App** をクリック

### 3. スコープを設定

1. 左メニューから **OAuth & Permissions** を選択
2. **Scopes** セクションまでスクロール
3. **User Token Scopes** に以下を追加:

| スコープ | 説明 |
|----------|------|
| `channels:read` | パブリックチャンネル一覧 |
| `channels:history` | パブリックチャンネルのメッセージ |
| `groups:read` | プライベートチャンネル一覧 |
| `groups:history` | プライベートチャンネルのメッセージ |
| `im:read` | DM一覧 |
| `im:history` | DMのメッセージ |
| `im:write` | DMを送信 |
| `mpim:read` | グループDM一覧 |
| `mpim:history` | グループDMのメッセージ |
| `users:read` | ユーザー情報 |
| `chat:write` | メッセージ送信 |

### 4. リダイレクトURLを設定

1. **OAuth & Permissions** ページで **Redirect URLs** セクションを見つける
2. **Add New Redirect URL** をクリック
3. 以下を入力して **Add** → **Save URLs**:
   ```
   https://localhost:8080/callback
   ```

### 5. Client IDとSecretを取得

1. 左メニューから **Basic Information** を選択
2. **App Credentials** セクションから:
   - `Client ID` をコピー
   - `Client Secret` の **Show** をクリックしてコピー

### 6. アプリを起動

```bash
# 環境変数を設定
export SLACK_CLIENT_ID="あなたのClient ID"
export SLACK_CLIENT_SECRET="あなたのClient Secret"

# 起動
./slack-tui
```

ブラウザが自動で開き、Slackの認証ページが表示されます。
**許可する** をクリックすると認証完了です。

> ⚠️ **注意**: 認証コールバック時にブラウザで「この接続は安全ではありません」と表示される場合があります。
> これは自己署名証明書を使用しているためです。「詳細設定」→「localhostにアクセスする」をクリックして続行してください。

## 基本的な使い方

### コマンド一覧

```
slack> ls                    # チャンネル一覧を表示
slack> ls dm                 # DM一覧のみ表示
slack> cd #general           # チャンネルに入る
slack> cd @john              # DMに入る
slack> ..                    # チャンネル一覧に戻る
slack> cat                   # メッセージ表示（デフォルト20件）
slack> cat -n 50             # 50件表示
slack> tail                  # 新着メッセージをリアルタイム表示
slack> tail -n 10            # 直近10件表示後、リアルタイム表示
slack> send Hello world      # メッセージ送信
slack> pwd                   # 現在のチャンネル表示
slack> help                  # ヘルプ
slack> exit                  # 終了
```

### 操作例

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
[10:30] alice: おはようございます
[10:32] bob: おはよう！
        └─ 3 replies

#general> send こんにちは
Message sent.

#general> tail
[10:30] alice: おはようございます
[10:32] bob: おはよう！
Tailing messages... (press 'q' or Ctrl+C to stop)
>>> Watching for new messages (q to quit) <<<
```

### tailコマンド（リアルタイムストリーミング）

`tail` コマンドを使うと、`tail -f` のようにメッセージをリアルタイムで監視できます。

```bash
#general> tail           # 直近10件表示後、新着を監視
#general> tail -n 20     # 直近20件表示後、新着を監視
```

- `q` キーまたは `Ctrl+C` で監視を終了
- リアルタイム機能には `SLACK_APP_TOKEN` が必要です

### キーボードショートカット

| キー | 操作 |
|------|------|
| `↑` / `↓` | コマンド履歴の移動 |
| `Ctrl+C` | 終了（tailモード中は監視終了） |
| `q` | tailモード終了 |

## 認証方法

### 方法1: OAuth認証（推奨）

環境変数:
```bash
export SLACK_CLIENT_ID="your-client-id"
export SLACK_CLIENT_SECRET="your-client-secret"
./slack-tui
```

または設定ファイル `~/.slack-tui/config.yaml`:
```yaml
client_id: your-client-id
client_secret: your-client-secret
```

### 方法2: トークン直接指定

```bash
export SLACK_TOKEN="xoxp-your-token"
./slack-tui
```

## コマンド

```bash
# 通常起動
./slack-tui

# ログアウト（保存された認証情報を削除）
./slack-tui logout
```

## リアルタイム更新（Socket Mode）

新着メッセージをリアルタイムで受信するには（`tail` コマンドに必要）:

1. Slack Appの設定で **Socket Mode** を有効化
2. **Basic Information** → **App-Level Tokens** で新しいトークンを作成
   - Token Name: 任意（例: `socket-token`）
   - Scope: `connections:write`
3. 生成されたトークン（`xapp-` で始まる）を設定:

```bash
export SLACK_APP_TOKEN="xapp-your-app-token"
```

## 設定ファイル

`~/.slack-tui/config.yaml`:

```yaml
# OAuth認証（推奨）
client_id: your-client-id
client_secret: your-client-secret

# または直接トークン指定
slack_token: xoxp-your-token

# Socket Mode用（オプション、tailコマンドに必要）
app_token: xapp-your-app-token

# コールバックポート（デフォルト: 8080）
redirect_port: 8080
```

## トラブルシューティング

### 「認証情報が見つかりません」エラー

- 環境変数または設定ファイルが正しく設定されているか確認
- OAuth認証の場合、Client IDとClient Secretの両方が必要

### ブラウザが開かない

- 手動でターミナルに表示されたURLをコピーしてブラウザで開く

### 「invalid_client_id」エラー

- Client IDが正しいか確認
- Slack Appが削除されていないか確認

### チャンネルが表示されない

- 必要なスコープがすべて追加されているか確認
- Slack Appをワークスペースに再インストール

### tailコマンドでリアルタイム更新されない

- `SLACK_APP_TOKEN` が設定されているか確認
- Socket Modeが有効になっているか確認

### ログアウトしたい

```bash
./slack-tui logout
```

## 開発

### ビルド

```bash
go build ./cmd/slack-tui
```

### テスト

```bash
go test ./...
```

## ファイル構成

```
slack-tui/
├── cmd/slack-tui/main.go     # エントリーポイント
├── internal/
│   ├── app/app.go            # アプリケーション初期化
│   ├── config/config.go      # 設定管理
│   ├── oauth/oauth.go        # OAuth認証フロー
│   ├── slack/                # Slack APIクライアント
│   │   ├── client.go
│   │   ├── channels.go
│   │   ├── messages.go
│   │   ├── threads.go
│   │   └── realtime.go
│   └── shell/                # シェルUIコンポーネント
│       ├── model.go          # Bubble Teaモデル
│       ├── commands.go       # コマンド実行
│       ├── parser.go         # コマンドパーサー
│       └── output.go         # 出力フォーマット
├── go.mod
├── go.sum
└── README.md
```

## ライセンス

MIT
