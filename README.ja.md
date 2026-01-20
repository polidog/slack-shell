# Slack Shell

GoとBubble Teaで作られたターミナルベースのSlackクライアント。
シェルコマンド風のインターフェースで直感的に操作できます。

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)

[English README](README.md)

## 機能

- 📺 チャンネルとダイレクトメッセージの閲覧
- 💬 メッセージ履歴の表示・投稿
- 🔄 `tail` コマンドでメッセージをリアルタイムストリーミング
- 🗂️ **`browse` コマンド** - インタラクティブなメッセージブラウザ（スレッド表示・返信対応）
- ⚡ Socket Modeによるリアルタイム更新（オプション）
- 🔐 **OAuth認証対応** - ブラウザで簡単ログイン
- 🐚 **シェルライクなUI** - 使い慣れたコマンド操作
- 📁 **チャンネル作成** - `mkdir`コマンドでパブリック/プライベートチャンネルを作成
- 🔀 **マルチワークスペース対応** - `source`コマンドで切り替え
- 🔍 **パイプ対応** - `ls | grep` や `cat | grep` で検索
- 🔔 **通知システム** - ターミナルベル、デスクトップ通知、タイトル更新、ビジュアル通知
- ⌨️ **Tab補完** - `cd`コマンドでチャンネル名・ユーザー名を補完

## クイックスタート

### 1. インストール

```bash
# Go installでインストール
go install github.com/polidog/slack-shell/cmd/slack-shell@latest

# または、ソースからビルド
git clone https://github.com/polidog/slack-shell.git
cd slack-shell
go build ./cmd/slack-shell
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
| `channels:write` | パブリックチャンネル作成 |
| `channels:history` | パブリックチャンネルのメッセージ |
| `groups:read` | プライベートチャンネル一覧 |
| `groups:write` | プライベートチャンネル作成 |
| `groups:history` | プライベートチャンネルのメッセージ |
| `im:read` | DM一覧 |
| `im:history` | DMのメッセージ |
| `im:write` | DMを送信 |
| `mpim:read` | グループDM一覧 |
| `mpim:history` | グループDMのメッセージ |
| `users:read` | ユーザー情報 |
| `chat:write` | メッセージ送信 |
| `team:read` | ワークスペース情報（プロンプト表示用） |

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
./slack-shell
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
slack> mkdir #new-channel    # パブリックチャンネルを作成
slack> mkdir -p #private     # プライベートチャンネルを作成
slack> cat                   # メッセージ表示（デフォルト20件）
slack> cat -n 50             # 50件表示
slack> tail                  # 新着メッセージをリアルタイム表示
slack> tail -n 10            # 直近10件表示後、リアルタイム表示
slack> browse                # インタラクティブメッセージブラウザ
slack> send Hello world      # メッセージ送信
slack> pwd                   # 現在のチャンネル表示
slack> source ~/work.yaml    # ワークスペースを切り替え
slack> help                  # ヘルプ
slack> exit                  # 終了

# パイプ対応
slack> ls | grep dev         # チャンネル名で検索
slack> cat | grep エラー     # メッセージ内容で検索
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

### browseコマンド（インタラクティブブラウザ）

`browse` コマンドを使うと、メッセージをインタラクティブに閲覧・操作できます。

```bash
#general> browse         # インタラクティブブラウザを起動
```

**browseモードのキー操作:**

| キー | 操作 |
|------|------|
| `↑` / `k` | 上のメッセージに移動 |
| `↓` / `j` | 下のメッセージに移動 |
| `Enter` | スレッドを表示 |
| `r` | 選択中のメッセージに返信（スレッド作成/返信） |
| `Esc` | スレッド表示を閉じる / 入力キャンセル |
| `q` | browseモードを終了 |

**機能:**
- メッセージ一覧を矢印キーまたは `j`/`k` で移動
- `Enter` でスレッドのリプライを表示
- `r` で直接返信（新規スレッド作成または既存スレッドへの返信）
- スレッド表示中も `r` で返信可能

### sourceコマンド（マルチワークスペース）

`source` コマンドを使うと、設定ファイルを読み込んで別のワークスペースに切り替えられます。

```bash
# ワークスペース用の設定ファイルを作成
cat > ~/work-slack.yaml << EOF
slack_token: xoxp-your-work-token
EOF

cat > ~/personal-slack.yaml << EOF
slack_token: xoxp-your-personal-token
EOF

# アプリ内で切り替え
slack> source ~/work-slack.yaml
Switched to workspace: Work Inc.

work> ls
Channels:
  # general
  # engineering

work> source ~/personal-slack.yaml
Switched to workspace: Personal
personal>
```

### キーボードショートカット

| キー | 操作 |
|------|------|
| `↑` / `↓` | コマンド履歴の移動 |
| `Tab` | `cd` コマンドの補完（チャンネル名・ユーザー名） |
| `Ctrl+C` | 終了（tailモード中は監視終了） |
| `q` | tail/browseモード終了 |
| `j` / `k` | browseモードでメッセージ移動 |
| `Enter` | browseモードでスレッド表示 |
| `r` | browseモードで返信 |

### Tab補完

`cd` コマンド入力時にTabキーを押すと、チャンネル名やユーザー名を補完できます。

```
slack> cd #          # Tab押下 → チャンネル名候補を表示
slack> cd #gen       # Tab押下 → #general に補完
slack> cd @          # Tab押下 → ユーザー名候補を表示
slack> cd @ali       # Tab押下 → @alice に補完
```

- **`cd #` + Tab**: チャンネル名のみ補完
- **`cd @` + Tab**: ユーザー名（DM相手）のみ補完
- **`cd ` + Tab**: 両方の候補を表示
- **Tab連打**: 次の候補に切り替え（循環）

## 通知システム

他のチャンネルに新着メッセージが届いたときに、4種類の方法で通知を受け取ることができます。

### 通知タイプ

| タイプ | 説明 |
|--------|------|
| **ターミナルベル** | `\a` 文字でビープ音を鳴らす |
| **デスクトップ通知** | OS標準の通知を表示（Linux/macOS/Windows対応） |
| **ターミナルタイトル** | 未読数をタイトルに表示（例: `Slack Shell (3)`） |
| **ビジュアル通知** | 画面上部に通知エリアを表示 |

### 通知の動作

- 現在表示中のチャンネル以外からのメッセージで通知
- `cd` でチャンネルに入ると、そのチャンネルの未読がクリア
- `mentions_only` オプションで @メンションのみ通知可能
- チャンネルごとのミュート設定
- DND（Do Not Disturb）モードで全通知を一時停止

### 通知設定

`~/.config/slack-shell/config.yaml` に `notifications` セクションを追加:

```yaml
notifications:
  enabled: true              # 通知システム全体の有効/無効

  bell:
    enabled: true            # ターミナルベルの有効/無効
    mentions_only: false     # @メンションのみ通知

  desktop:
    enabled: true            # デスクトップ通知の有効/無効
    mentions_only: false     # @メンションのみ通知

  title:
    enabled: true            # タイトル更新の有効/無効
    format: "Slack Shell (%d)" # 未読数表示フォーマット
    base_title: "Slack Shell"  # 未読がない時のタイトル

  visual:
    enabled: true            # ビジュアル通知の有効/無効
    max_items: 5             # 表示する通知の最大数
    dismiss_after: 10        # 自動消去までの秒数（0=自動消去なし）

  mute_channels: []          # 通知しないチャンネル名のリスト
  dnd: false                 # Do Not Disturbモード
```

## 認証方法

### 方法1: OAuth認証（推奨）

環境変数:
```bash
export SLACK_CLIENT_ID="your-client-id"
export SLACK_CLIENT_SECRET="your-client-secret"
./slack-shell
```

または設定ファイル `~/.config/slack-shell/config.yaml`:
```yaml
client_id: your-client-id
client_secret: your-client-secret
```

### 方法2: トークン直接指定

```bash
export SLACK_TOKEN="xoxp-your-token"
./slack-shell
```

## コマンド

```bash
# 通常起動
./slack-shell

# ワンライナー実行（-c オプション）
./slack-shell -c "ls"
./slack-shell -c "cd #general && cat -n 5"
./slack-shell -c "cd @john && send おはよう"
./slack-shell -c "ls | grep dev"

# ログアウト（保存された認証情報を削除）
./slack-shell logout

# サンプル設定ファイルを生成
./slack-shell config init                    # ~/.config/slack-shell/config.yaml に作成
./slack-shell config init ~/work.yaml        # 指定パスに作成
./slack-shell config init ~/work.yaml -f     # 既存ファイルを上書き
```

### config init

すべての設定オプションがコメント付きで記載されたサンプル設定ファイルを生成します。

```bash
# デフォルトの設定ファイルを作成
./slack-shell config init

# source コマンドで使用するワークスペース別の設定ファイルを作成
./slack-shell config init ~/work-slack.yaml
./slack-shell config init ~/personal-slack.yaml

# アプリ内でワークスペースを切り替え
slack> source ~/work-slack.yaml
```

### -c オプション

`-c` オプションを使うと、対話モードを起動せずにコマンドを実行して終了できます。
シェルスクリプトやcronとの連携に便利です。

```bash
# 複数コマンドを && または ; で連結
./slack-shell -c "cd #times-polidog && send 朝のあいさつ"

# パイプも使用可能
./slack-shell -c "cd #general && cat | grep 会議"

# 例: cronで定時メッセージ
0 9 * * 1-5 /path/to/slack-shell -c "cd #general && send おはようございます"
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

`~/.config/slack-shell/config.yaml`（または `$XDG_CONFIG_HOME/slack-shell/config.yaml`）:

> **注意**: 後方互換性のため、`~/.slack-shell/config.yaml` もサポートされています。

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

# プロンプトのカスタマイズ（オプション）
prompt:
  format: "{workspace} {location}> "
```

## プロンプトのカスタマイズ

`~/.config/slack-shell/config.yaml` でプロンプトの表示形式をカスタマイズできます：

```yaml
prompt:
  format: "slack-shell > {workspace}{location}"
```

### 使用可能な変数

| 変数 | 説明 | 例 |
|------|------|-----|
| `{workspace}` | ワークスペース名 | `MyCompany` |
| `{location}` | 現在のチャンネル/DM（プレフィックス付き） | `#general`, `@alice`, または空 |
| `{channel}` | チャンネル名のみ（プレフィックスなし） | `general` |
| `{user}` | ユーザー名のみ（プレフィックスなし） | `alice` |

### フォーマット例

```yaml
# デフォルト
prompt:
  format: "{workspace} {location}> "
# 結果: MyCompany #general>

# シェル風
prompt:
  format: "slack-shell > {workspace}{location}"
# 結果: slack-shell > MyCompany#general

# シンプル
prompt:
  format: "{location}$ "
# 結果: #general$

# 括弧付き
prompt:
  format: "[{workspace}:{channel}]$ "
# 結果: [MyCompany:general]$
```

## 起動時のカスタマイズ

`~/.config/slack-shell/config.yaml` で起動時のメッセージ、バナー、自動実行コマンドをカスタマイズできます：

```yaml
startup:
  # 1行のウェルカムメッセージ
  # 使用可能な変数: {workspace}
  message: "Welcome to Slack Shell - {workspace}"

  # 複数行のバナー（messageより優先されます）
  banner: |
    ╔═══════════════════════════════════════╗
    ║  Slack Shell v1.0                     ║
    ║  ワークスペース: {workspace}          ║
    ╚═══════════════════════════════════════╝

  # 起動時に自動実行するコマンド
  init_commands:
    - "cd #general"
    - "cat -n 5"
```

### オプション

| オプション | 説明 |
|------------|------|
| `message` | 1行のウェルカムメッセージ（デフォルト: "Welcome to Slack Shell - {workspace}"） |
| `banner` | 複数行のASCIIアートバナー（設定すると `message` より優先） |
| `init_commands` | 起動時に自動実行するコマンドリスト（`.bashrc` のように） |

### 例: 自動でチャンネルに入る

```yaml
startup:
  message: "おかえりなさい！#generalに入ります..."
  init_commands:
    - "cd #general"
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
./slack-shell logout
```

## 開発

### ビルド

```bash
go build ./cmd/slack-shell
```

### テスト

```bash
go test ./...
```

## ファイル構成

```
slack-shell/
├── cmd/slack-shell/main.go     # エントリーポイント
├── internal/
│   ├── app/app.go            # アプリケーション初期化
│   ├── config/config.go      # 設定管理
│   ├── oauth/oauth.go        # OAuth認証フロー
│   ├── notification/         # 通知システム
│   │   ├── config.go         # 通知設定
│   │   ├── notification.go   # Message型、インターフェース
│   │   ├── manager.go        # 通知マネージャー
│   │   ├── bell.go           # ターミナルベル通知
│   │   ├── desktop.go        # デスクトップ通知
│   │   ├── title.go          # ターミナルタイトル通知
│   │   └── visual.go         # ビジュアル通知
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
│       ├── browse.go         # browseモードUI
│       └── output.go         # 出力フォーマット
├── go.mod
├── go.sum
└── README.md
```

## ライセンス

MIT
