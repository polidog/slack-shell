# Slack TUI

GoとBubble Teaで作られたターミナルベースのSlackクライアント。

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)

## 機能

- 📺 チャンネルとダイレクトメッセージの閲覧
- 💬 メッセージ履歴の表示・投稿
- 🧵 スレッドの表示と返信
- ⚡ Socket Modeによるリアルタイム更新（オプション）
- 🔐 **OAuth認証対応** - ブラウザで簡単ログイン

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
   http://localhost:8080/callback
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

## 基本的な使い方

### 画面構成

```
┌─────────────────┬────────────────────────────────────┐
│                 │                                    │
│   サイドバー     │         メッセージエリア            │
│                 │                                    │
│  Channels       │  # general                         │
│  # general      │  ──────────────────────────────    │
│  # random       │                                    │
│  # dev          │  Alice                    10:30    │
│                 │  おはようございます！                │
│  Direct Messages│                                    │
│  💬 Bob         │  Bob                      10:32    │
│  💬 Carol       │  おはよう〜                         │
│                 │    💬 3 replies                    │
│                 │                                    │
├─────────────────┴────────────────────────────────────┤
│  Type a message...                                   │
├──────────────────────────────────────────────────────┤
│ ● Connected  tab:switch enter:select i:input q:quit │
└──────────────────────────────────────────────────────┘
```

### 操作の流れ

1. **起動** → サイドバーにフォーカス
2. **j/k** でチャンネルを選択
3. **Enter** でチャンネルを開く → メッセージエリアにフォーカス
4. **j/k** でメッセージをスクロール
5. **i** で入力モード → メッセージを入力 → **Enter** で送信
6. **Esc** で入力モード終了
7. **r** でスレッド返信モード
8. **q** で終了

### スレッドを見る

1. スレッドのあるメッセージ（`💬 X replies` 表示）を選択
2. **Enter** でスレッドパネルを開く
3. **Tab** でスレッドパネルにフォーカス
4. **Esc** でスレッドを閉じる

## キーバインド一覧

### 移動

| キー | 操作 |
|------|------|
| `j` / `↓` | 下に移動 |
| `k` / `↑` | 上に移動 |
| `g` | 先頭に移動 |
| `G` | 末尾に移動 |
| `Ctrl+u` / `PgUp` | ページアップ |
| `Ctrl+d` / `PgDown` | ページダウン |

### 操作

| キー | 操作 |
|------|------|
| `Tab` | パネル切り替え（サイドバー ↔ メッセージ ↔ スレッド） |
| `Enter` | チャンネル選択 / スレッド展開 |
| `i` | 入力モードに入る |
| `r` | 選択中のメッセージにスレッド返信 |
| `Esc` | モード終了（入力/スレッド） |
| `q` / `Ctrl+C` | アプリ終了 |

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

新着メッセージをリアルタイムで受信するには:

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

# Socket Mode用（オプション）
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
│   └── ui/                   # TUIコンポーネント
│       ├── model.go          # メインUIモデル
│       ├── views/
│       │   ├── sidebar.go
│       │   ├── messages.go
│       │   ├── input.go
│       │   └── thread.go
│       └── styles/styles.go
├── go.mod
├── go.sum
└── README.md
```

## ライセンス

MIT
