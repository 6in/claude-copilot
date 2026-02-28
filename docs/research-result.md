# リサーチ: Claude Code と GitHub Copilot の統合

## 1. 概要
本リサーチの目的は、GitHub Copilot のモデルを Anthropic の Claude Code CLI のバックエンドとして利用することです。
Claude Code はネイティブで Anthropic 互換の API を期待しているため、これを実現するには以下の要素が必要になります。
1. Anthropic Messages API のフォーマットを GitHub Copilot API のフォーマットに変換する API プロキシ/サーバー。
2. Anthropic の公式エンドポイントの代わりに、このローカルのカスタムサーバーを指すように Claude Code を設定すること。

## 2. Anthropic 互換 Web API サーバーの構築
Claude Code と GitHub Copilot 間のブリッジとして機能するカスタム API サーバーを構築するには、以下の手順が必要です。

- **開発言語とフレームワーク:** プロキシサーバーの開発には **Go (Golang)** を採用します。GitHub Copilot SDK は Go を公式サポートしており、標準ライブラリの `net/http` や軽量なルーティングライブラリを使用して高速な API サーバーを構築可能です。
- **エンドポイント:** サーバーは、Claude Code の Anthropic API リクエストをインターセプトするために、標準の `POST /v1/messages` エンドポイントを公開する必要があります。
- **ペイロード変換レイヤー:**
  - **入力の解釈:** Claude Code は、`model`、`messages` 配列 (`role` と `content` ブロックを含む)、`max_tokens`、`system` プロンプトを含む、Anthropic フォーマットの JSON を送信します。
  - **リクエストの変換:** プロキシは、`system` プロンプトや複雑な `content` ブロックを、GitHub Copilot API が期待する会話フォーマット (通常、OpenAI Chat Completions フォーマットに準拠) にマッピングする必要があります。
  - **ストリームの書き換え (最重要):** Claude Code は Server-Sent Events (SSE) に依存しています。プロキシは GitHub Copilot からのストリーミングレスポンスを消費し、イベントタイプを Anthropic の構造 (`message_start`, `content_block_delta`, `message_delta`, `message_stop` など) に一致するように書き換える必要があります。
- **認証のセットアップ:** プロキシは、GitHubのデバイス認証（Device Flow）を利用してGitHub Copilot APIエンドポイントと動的に認証する必要があります。サーバー起動時にこのデバイス認証フローを開始し、ユーザーにブラウザでの認証を促して有効なトークンを取得する仕組みにします。

*参考:* 既存のオープンソースコミュニティによるプロキシ (例: `ericc-ch/copilot-api`) は、同様のリバースエンジニアリングされたラッパーを実装しており、正確な API ペイロードのマッピングの優れた参考になります。

## 3. Claude Code の設定
Claude Code は標準的な Anthropic SDK の動作に基づいて構築されているため、カスタム API ルーティング用に環境変数を尊重します。
新しく構築したプロキシサーバーにリクエストをルーティングするには、Claude Code を実行する前に `ANTHROPIC_BASE_URL` 環境変数を設定する必要があります。

**使用例:**
ローカルプロキシサーバーがポート 3000 で実行されている場合。

```bash
# 現在のターミナルセッションに対して設定する
export ANTHROPIC_BASE_URL="http://localhost:3000"
claude
```

```bash
# インラインで環境変数を指定して実行する
ANTHROPIC_BASE_URL="http://localhost:3000" claude
```

## 4. 提案される次のステップ
1. **プロキシプロジェクトの初期化:** このリポジトリに Web API サーバー用の新しい **Go パッケージ**（`go mod init`）を作成する。
2. **認証の実装:** サーバー起動時に実行されるGitHubデバイス認証フローを実装し、Copilotへのアクセストークンを安全に取得・保持するロジックを追加する。
3. **変換ロジックの実装:** 正確なパースと SSE ストリームの再構成に重点を置き、`/v1/messages` ハンドラーを作成する。
4. **統合テスト:** サーバーを起動し、`ANTHROPIC_BASE_URL` を設定し、Claude Code 経由でコマンドを発行して、正しいレスポンスのストリーミングと、必要に応じて関数呼び出し (Function Calling) のサポートを検証する。
