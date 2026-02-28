# claude-copilot

GitHub Copilot をバックエンドに、Claude Code (Anthropic Messages API) を透過的に利用するためのローカルプロキシサーバーです。  
公式 [GitHub Copilot SDK for Go](https://github.com/github/copilot-sdk/go) を使用しています。

> ⚠️ **本ツールはローカル環境での利用を前提としています。**  
> 個人の開発マシン上で起動し、同一マシンの Claude Code から接続する設計です。  
> リモートサーバーへのデプロイや、外部ネットワークへの公開は想定していません。  
> 認証トークンはローカルファイル（`~/.claude_copilot_proxy.json`）に平文で保存されるため、共有環境での使用は避けてください。

## アーキテクチャ

```
┌──────────────┐      Anthropic API        ┌─────────────────┐      Copilot SDK       ┌──────────────────┐
│  Claude Code │  ──── POST /v1/messages ──▶│  claude-copilot  │  ──── Session.Send ──▶ │  GitHub Copilot  │
│  (CLI)       │  ◀── SSE Stream ──────────│  localhost:8080  │  ◀── SessionEvent ──── │  (GPT-5 mini等)  │
└──────────────┘                           └─────────────────┘                        └──────────────────┘
```

## 前提条件

- Go 1.24+
- GitHub アカウント（Copilot サブスクリプション付き）
- **GitHub Copilot CLI** (インストールは[こちら](https://github.com/features/copilot/cli?locale=ja))
  - Copilot SDK は内部で Copilot CLI を子プロセスとして起動するため必須です

## セットアップ

```bash
git clone https://github.com/6in/claude-copilot
cd claude-copilot
make build
```

## 使い方

### サーバー起動

```bash
# デフォルトポート (8080)
./bin/claude-copilot

# ポート指定
./bin/claude-copilot -port 3000
```

### 初回起動時のデバイス認証

初回起動時に GitHub Copilot のデバイス認証フローが発生します。

1. ターミナルに認証URL（`https://github.com/login/device`）とワンタイムコードが表示されます
2. ブラウザで上記URLを開き、表示されたコードを入力して認証を完了します
3. 認証成功後、トークンが自動的に設定ファイルに保存されます

**2回目以降の起動では認証は不要です。**

### 設定ファイル

認証トークンやポート設定は以下の JSON ファイルに保存されます。

```
~/.claude_copilot_proxy.json
```

```json
{
  "port": "8080",
  "github_token": "ghu_xxxxxxxxxxxx"
}
```

> ⚠️ このファイルにはトークンが含まれるため、パーミッションは `0600`（所有者のみ読み書き可）で作成されます。

### 企業プロキシ環境での利用

認証情報付き `HTTPS_PROXY` に対応しています。

```bash
export HTTPS_PROXY="http://user:password@proxy.corp.example.com:8080"
./bin/claude-copilot
```

起動時にプロキシ設定が検出されるとログに表示されます。

### Claude Code から利用

```bash
ANTHROPIC_AUTH_TOKEN=dummy \
ANTHROPIC_BASE_URL=http://localhost:8080 \
CLAUDE_CONFIG_DIR=~/.claude_copilot \
claude --model "GPT-5 mini"
```

### ワンショット実行

```bash
ANTHROPIC_AUTH_TOKEN=dummy \
ANTHROPIC_BASE_URL=http://localhost:8080 \
CLAUDE_CONFIG_DIR=~/.claude_copilot \
claude --model "GPT-5 mini" -p "Pythonでhello worldを書いて"
```

### エイリアス設定（推奨）

```bash
# ~/.zshrc に追加
alias claude-copilot='ANTHROPIC_AUTH_TOKEN=dummy ANTHROPIC_BASE_URL=http://localhost:8080 CLAUDE_CONFIG_DIR=~/.claude_copilot claude --model "GPT-5 mini"'
```

### 💡 CLAUDE_CONFIG_DIR について（推奨）
このプロキシ経由で利用する際には、起動時の引数で `CLAUDE_CONFIG_DIR=~/.claude_copilot` を指定することを推奨しています。
Claude Code は通常 `~/.claude/` に様々なデータ（`settings.json` や過去のチャットログ、グローバルインストールされたスキル等）を保持します。環境変数を指定することで、公式の Claude API 用の環境と完全に分離されたサンドボックス化されたプロファイルが自動生成されます。これにより、公式環境用の不要なロードを省き、エラーやコンフリクトを未然に防ぎます。

## クロスプラットフォームビルド

```bash
make build-all    # Windows / macOS / Linux 全て (amd64 + arm64)
make build-linux
make build-windows
make build-darwin
```

出力先: `bin/` ディレクトリ

## 環境変数

| 変数名 | 説明 | デフォルト |
|--------|------|-----------|
| `PROXY_PORT` | プロキシの待受ポート | `8080` |
| `HTTPS_PROXY` | 企業プロキシURL（認証情報付き可） | なし |
| `HTTP_PROXY` | HTTPプロキシURL | なし |
| `NO_PROXY` | プロキシ除外ホスト | なし |

## CLI オプション

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `-port` | 待受ポート番号（環境変数より優先） | `8080` |
| `-logoff` | 認証情報（`~/.claude_copilot_proxy.json`）を削除してログアウト | - |

ログアウト例:
```bash
./bin/claude-copilot -logoff
# → ✅ 認証情報を削除しました: /Users/<user>/.claude_copilot_proxy.json
```

## プロジェクト構成

```
.
├── main.go              # エントリポイント（SDK初期化 & HTTPサーバー）
├── api/handlers.go      # POST /v1/messages ハンドラ
├── translator/           # Anthropic ↔ Copilot SDK 変換ロジック
├── models/models.go     # リクエスト/レスポンスの型定義
├── config/config.go     # 設定管理 & トークン永続化
├── auth/                # デバイス認証フロー
├── docs/api_specs.md    # APIマッピング仕様
└── Makefile             # クロスプラットフォームビルド
```

## ライセンス

MIT License (Personal Use Only) — 個人利用・非商用目的に限ります。詳細は [LICENSE](LICENSE) を参照してください。
