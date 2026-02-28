# copilot-proxy

GitHub Copilot をバックエンドに、Claude Code (Anthropic Messages API) を透過的に利用するためのローカルプロキシサーバーです。  
公式 [GitHub Copilot SDK for Go](https://github.com/github/copilot-sdk/go) を使用しています。

> ⚠️ **本ツールはローカル環境での利用を前提としています。**  
> 個人の開発マシン上で起動し、同一マシンの Claude Code から接続する設計です。  
> リモートサーバーへのデプロイや、外部ネットワークへの公開は想定していません。  
> 認証トークンはローカルファイル（`~/.claude_copilot_proxy.json`）に平文で保存されるため、共有環境での使用は避けてください。

## アーキテクチャ

```
┌──────────────┐      Anthropic API        ┌─────────────────┐      Copilot SDK       ┌──────────────────┐
│  Claude Code │  ──── POST /v1/messages ──▶│  copilot-proxy  │  ──── Session.Send ──▶ │  GitHub Copilot  │
│  (CLI)       │  ◀── SSE Stream ──────────│  localhost:8080  │  ◀── SessionEvent ──── │  (GPT-5 mini等)  │
└──────────────┘                           └─────────────────┘                        └──────────────────┘
```

## 前提条件

- Go 1.24+
- GitHub アカウント（Copilot サブスクリプション付き）
- `gh auth login` 済み、または初回起動時にデバイス認証を実施

## セットアップ

```bash
git clone <repository-url>
cd claude-copilot
make build
```

## 使い方

### サーバー起動

```bash
# デフォルトポート (8080)
./bin/copilot-proxy

# ポート指定
./bin/copilot-proxy -port 3000
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
./bin/copilot-proxy
```

起動時にプロキシ設定が検出されるとログに表示されます。

### Claude Code から利用

```bash
ANTHROPIC_AUTH_TOKEN=dummy ANTHROPIC_BASE_URL=http://localhost:8080 claude --model "GPT-5 mini"
```

### ワンショット実行

```bash
ANTHROPIC_AUTH_TOKEN=dummy ANTHROPIC_BASE_URL=http://localhost:8080 \
  claude --model "GPT-5 mini" -p "Pythonでhello worldを書いて"
```

### エイリアス設定（推奨）

```bash
# ~/.zshrc に追加
alias claude-copilot='ANTHROPIC_AUTH_TOKEN=dummy ANTHROPIC_BASE_URL=http://localhost:8080 claude --model "GPT-5 mini"'
```

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
./bin/copilot-proxy -logoff
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

MIT
