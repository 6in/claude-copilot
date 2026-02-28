# Codebase Summary — claude-copilot

> 🤖 **Note:** このファイルは、Claude Code (GPT-5 mini) によって出力・生成されたアーキテクチャ概要です。

## 概要
このリポジトリは、Anthropic の Claude Code（Anthropic Messages API 互換の CLI）からのリクエストを受け取り、GitHub Copilot SDK を使ってローカルで Copilot（例: GPT-5 mini）へ中継するローカルプロキシサーバーです。

主目的は、claude コマンドを変更せずに ANTHROPIC_BASE_URL をローカルに差し替え、Copilot をバックエンドとして利用できるようにすることです（ローカル利用が前提、トークンは平文で保存されるため共有環境での利用は推奨されません）。

## 主要ファイル・ディレクトリ
- main.go: エントリポイント。設定ロード、デバイス認証（auth パッケージ経由）、Copilot SDK の起動、HTTP サーバー (/v1/messages) の登録を行う。handlers を api.Handler として登録。
- api/: HTTP ハンドラ（POST /v1/messages を処理）。
- translator/: Anthropic のリクエスト/レスポンス形式と Copilot SDK とのマッピング（変換ロジック）。
- models/: リクエスト/レスポンスの型定義（models.go 等）。
- config/: 設定管理とトークン永続化（~/.claude_copilot_proxy.json の読み書き）。
- auth/: GitHub Copilot のデバイス認証フロー（token の取得/更新ロジック）。
- docs/: ドキュメントやブログドラフト（docs/blog-draft.md に利用手順や設計概要あり）。
- Makefile / bin/: ビルドやクロスコンパイルの仕組み（make build, build-all など）。

## 動作フロー（高レベル）
1. 起動時に設定をロードし、必要ならデバイス認証を行う。
2. GitHub Copilot CLI/SDK を子プロセスとして起動し、SDK クライアントを初期化。
3. /v1/messages エンドポイントで受け取った Anthropic 互換リクエストを translator で Copilot SDK 用に変換。
4. Copilot からの応答を受け、SSE ストリーミングやワンショット応答として Claude Code 側へ返却する。

## 前提・注意点
- Go 1.24+ を想定。
- GitHub Copilot のサブスクリプションおよび Copilot CLI が必要（SDK が内部で CLI を起動する設計）。
- ローカル利用が前提。トークンはローカルファイルに保存されるため取り扱いに注意。

## 開発上の確認ポイント（将来的に見るべき箇所）
- auth/token.go: デバイスフローとトークン永続化の安全性（ファイルパーミッション等）。
- translator/: Anthropic → Copilot 変換でのエッジケース（ストリーミングイベント、エラー伝搬）。
- api/handlers.go: SSE ストリーミングの実装とタイムアウト/キャンセルの扱い。
- main.go: 環境変数（PROXY_PORT, HTTPS_PROXY 等）のプロパゲーションと SDK の Env 設定。

---
生成日時: 2026-02-28T05:23:08Z
