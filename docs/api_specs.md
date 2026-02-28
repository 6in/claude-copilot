# API マッピング仕様

このプロキシは **Anthropic Messages API** のリクエストを受け取り、**GitHub Copilot SDK** のセッションベース API に変換します。

## エンドポイント

| プロキシ側 | メソッド | 説明 |
|-----------|---------|------|
| `/v1/messages` | POST | Anthropic Messages API 互換エンドポイント |
| `/` | GET | ヘルスチェック |

---

## リクエスト変換

### Anthropic → Copilot SDK

#### 入力: Anthropic `POST /v1/messages`

```json
{
  "model": "GPT-5 mini",
  "max_tokens": 1024,
  "stream": true,
  "system": "You are a helpful assistant.",
  "messages": [
    {"role": "user", "content": "Hello!"},
    {"role": "assistant", "content": "Hi!"},
    {"role": "user", "content": "Write code"}
  ]
}
```

#### 変換先: Copilot SDK `SessionConfig` + `MessageOptions`

| Anthropic フィールド | Copilot SDK マッピング | 備考 |
|---------------------|----------------------|------|
| `model` | `SessionConfig.Model` | そのまま転送。空の場合は `"GPT-5 mini"` |
| `system` | プロンプト先頭に `"System: ..."` として結合 | string / array 両形式対応 |
| `messages[].role` | プロンプト内に `"role: content"` 形式で結合 | `user` / `assistant` |
| `messages[].content` | テキスト抽出して結合 | string / content_block array 対応 |
| `stream` | ストリーム/非ストリーム分岐 | `true`: SSE, `false`: JSON |
| `max_tokens` | *(未使用)* | SDK側で制御 |
| `temperature` | *(未使用)* | SDK側で制御 |

---

## レスポンス変換

### ストリーミング (`stream: true`)

Copilot SDK の `SessionEvent` を Anthropic SSE 形式に変換します。

#### イベントフロー

```
SDK: Session.Send()
  ↓
SDK: copilot.AssistantMessage (テキスト受信)
  ↓
SDK: copilot.SessionIdle (完了)
```

↓ 変換後

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_copilot_sdk_<session_id>","type":"message","role":"assistant","usage":{}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta        ← AssistantMessage ごとに繰り返し
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"..."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}

event: message_stop
data: {"type":"message_stop"}
```

#### SDK イベント → Anthropic SSE マッピング

| Copilot SDK Event | Anthropic SSE Event | 説明 |
|-------------------|---------------------|------|
| *(初期化時)* | `message_start` | セッションID付きメッセージ開始 |
| *(初期化時)* | `content_block_start` | テキストブロック開始 |
| `AssistantMessage` | `content_block_delta` | テキスト差分の送信 |
| `SessionIdle` | `content_block_stop` + `message_delta` + `message_stop` | 完了シーケンス |
| `SessionError` | *(ログ出力 + 完了)* | エラー処理 |

---

### 非ストリーミング (`stream: false`)

SDK の全レスポンスを結合し、Anthropic JSON 形式で一括返却します。

#### レスポンス例

```json
{
  "id": "msg_copilot_sdk_<session_id>",
  "type": "message",
  "role": "assistant",
  "model": "GPT-5 mini",
  "content": [
    {
      "type": "text",
      "text": "print(\"Hello, world!\")"
    }
  ],
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 10,
    "output_tokens": 10
  }
}
```

---

## 認証

| ヘッダー | 値 | 備考 |
|---------|-----|------|
| `x-api-key` | 任意の値 (例: `"dummy"`) | プロキシ側では検証しない |
| `anthropic-version` | `2023-06-01` | Claude Code が送信 |

実際の GitHub Copilot 認証は SDK が自動管理します（`gh auth login` または初回デバイス認証で取得したトークンを使用）。
