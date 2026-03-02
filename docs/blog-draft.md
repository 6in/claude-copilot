# 🚀 Claude Code × GitHub Copilot で"夢のタッグ"を実現してみた話

## はじめに 👋

こんにちは！

突然ですが、**Claude Code** 使ってますか？  
Anthropicが提供するCLIベースのAIコーディングアシスタントで、ターミナルからガンガンコードを書いてくれる最高のツールです。

ただ…… **会社では使えない** んですよね 😭

うちの会社では GitHub Copilot は利用が許可されていて、申請した人にはライセンスが配布されています。  
でも Anthropic の Claude は契約がなく、API キーも発行されていません。

そんなある日、**「Claude Code から Ollama（ローカルLLM）を呼べるようになった」** という記事を見かけました。  
Claude Code は `ANTHROPIC_BASE_URL` を差し替えることで、Anthropic以外のバックエンドに接続できるらしい。

💡 **「……ということは、Ollama の代わりに Copilot を繋げばいいのでは？」**

そう思って作ったのが、今回紹介する **`claude-copilot`** です。  
**しかも、すんなり動いてしまって自分でもびっくりしました** 😳

## 🛠️ 何を作ったの？

ひとことで言うと、**Claude Code と GitHub Copilot の間に立つ翻訳プロキシサーバー**です。

```
Claude Code  →  claude-copilot（Go製）  →  GitHub Copilot SDK
   ↑                                           ↓
   └──────── Anthropic SSE形式で返却 ←──────────┘
```

Claude Code は「Anthropicの `/v1/messages` API」を叩いている**つもり**。  
でも実際には、リクエストは手元のプロキシが受け取って **GitHub Copilot SDK** に変換し、Copilot のモデル（GPT-5 mini等）が応答を返してくれます。

## 🏗️ 技術スタック

- **Go** （標準ライブラリ + 公式SDK のみ！）
- **GitHub Copilot SDK for Go** (`github.com/github/copilot-sdk/go`)
- **Anthropic Messages API** 互換のSSEストリーミング

ポイントは、**どちらも公式が提供しているインターフェースだけで実現している**ということ。  
詳しくは後述の「これって大丈夫なの？」セクションで触れます ✨

## 🎬 実際に動かしてみる

### Step 1: セットアップ 🔨

#### シナリオA：開発チームがビルドする場合

Copilot SDK が使うCLIを用意して、バイナリを生成します。  
おすすめは **SDK公式の埋め込み方式** です（CLI互換問題を避けやすい）。

```bash
git clone https://github.com/6in/claude-copilot
cd claude-copilot

# Copilot CLIを埋め込み生成（初回のみ）
make bundler

# ビルド
make build
```

`go run .../bundler` 実行後は、`zcopilot_*.zst` / `zcopilot_*.license` / `zcopilot_<os>_<arch>.go` が生成されます。  
`-copilot-cli` を指定しない場合、この埋め込みCLIが使われます。

**Makefileを用意してあるのでコマンド一発** です。  
しかもクロスコンパイル対応なので、Windows / macOS / Linux 全部いけます。

```bash
make build-all
```

完成したバイナリ（`./bin/claude-copilot`）を会社内で配布します。

#### シナリオB：配布されたバイナリを使う場合

ビルドは **一切不要** です。受け取ったバイナリをそのまま使用できます：

```bash
# ビルド不要。配布されたバイナリをそのまま実行
export HTTPS_PROXY="http://user:password@proxy.corp.example.com:8080"
./bin/claude-copilot -insecure
```

Go 言語環境やビルドツールがなくても動きます 🎉

<!-- 📸 スクリーンショット: make build-all の実行結果 -->

### Step 2: 起動 🚀

```bash
./bin/claude-copilot -insecure
```

初回起動時には、GitHub への**ログイン認証**（デバイス認証フロー）が走ります。

<!-- 📸 スクリーンショット: デバイス認証のターミナル出力 -->

ブラウザでURLを開いて、表示されたコードを入力するだけ。  
認証が通ると `~/.claude_copilot_proxy.json` にトークンが保存されて、**2回目以降は自動的にスキップ**されます 🔐

### Step 3: Claude Code で接続 ⚡

```bash
ANTHROPIC_AUTH_TOKEN=dummy \
ANTHROPIC_BASE_URL="http://localhost:8081" \
CLAUDE_CONFIG_DIR=~/.claude_copilot \
claude --model "GPT-5 mini"
```

たったこれだけ。

<!-- 📸 スクリーンショット: Claude Code が起動して応答を返している画面 -->

<img src="./images/step1.png" style="width: 800px">

**え、動いた。普通に動いた。**  

最初は何かしらエラーが出るだろうと身構えていたんですが、すんなり動いてしまってびっくりしました 😳

ストリーミングもちゃんと動いていて、文字がスーッと流れるように表示されます。  
体感的には Anthropicの API を直接叩いているのとほぼ変わりません。

### Step 4: ワンショット実行もOK 🎯

```bash
ANTHROPIC_AUTH_TOKEN=dummy \
ANTHROPIC_BASE_URL="http://localhost:8081" \
CLAUDE_CONFIG_DIR=~/.claude_copilot \
claude --model "GPT-5 mini" -p "Pythonでフィボナッチ数列を出力して"
```

<!-- 📸 スクリーンショット: ワンショット実行の結果 -->

<img src="./images/step3.png" style="width: 800px">

`-p` オプションでのワンライナー実行もバッチリ。  
返ってきたコードもちゃんと動く、実用レベルのクオリティでした 🎉

## 🔧 こだわりポイント

### 企業プロキシ対応 🏢

会社のネットワークプロキシ配下でも使えるように、`HTTPS_PROXY`（認証情報付き）に対応しています。

#### 簡易方法: -insecure での起動

```bash
export HTTPS_PROXY="http://user:pass@proxy.corp.example.com:8080"
./bin/claude-copilot -insecure
```

この方法は TLS 検証をスキップするため、すぐに試せますが、中間者攻撃に対する脆弱性があります。  
環境によっては、以下の推奨方法への移行をお勧めします。

#### 推奨方法: CA証明書を指定

企業のセキュリティポリシーに従い、CA証明書を使う方法が安全です。

```bash
# 1. IT部門から企業CA証明書を入手
# 例: company-ca.pem

# 2. 証明書を指定して起動
export HTTPS_PROXY="http://user:pass@proxy.corp.example.com:8080"
./bin/claude-copilot -ca-cert /path/to/company-ca.pem
```

実運用では `-ca-cert` の利用が推奨されています。

#### Node.js 実行時の調整

Node.js の挙動がプロキシと相容れない場合、以下のオプションで対応できます。

```bash
# Node.js のパスを明示的に指定
./bin/claude-copilot -insecure -node-bin /opt/homebrew/opt/node/bin

# Node.js 実行時オプション（詳細ログなど）
./bin/claude-copilot -insecure -node-options "--trace-warnings"

# 複合例
export HTTPS_PROXY="http://user:pass@proxy.corp.example.com:8080"
./bin/claude-copilot -ca-cert /path/to/ca.pem -node-bin /usr/local/bin
```

### ポート番号の指定 🔌

```bash
./bin/claude-copilot -insecure -port 3000
```

デフォルトは `8081` ですが、`-port` フラグで好きなポートに変更できます。

### ログアウト機能 🚪

トークンをリセットしたくなったら：

```bash
./bin/claude-copilot -logoff
# ✅ 認証情報を削除しました
```

### プロンプト覗き見モード（デバッグ） 🕵️

実はこのツール、Claude Code が裏でどんなプロンプト（システムプロンプトやツール定義）を送っているかを完全に傍受できます。

```bash
./bin/claude-copilot -debug
```
このオプションをつけて起動すると、Claude Code が投げている **数千文字に及ぶ「秘伝のタレ（コンテキスト）」** がターミナルに垂れ流されます。AIエージェントのプロンプトエンジニアリングをリバースエンジニアリングする教材として最高に面白いので、ぜひ一度試してみてください！

### エイリアス設定で快適運用 ⚡

毎回長いコマンドを打つのは面倒なので、エイリアスを設定しておくと最高に快適です。

```bash
# ~/.zshrc に追加
alias claude-copilot='ANTHROPIC_AUTH_TOKEN=dummy ANTHROPIC_BASE_URL=http://localhost:8081 CLAUDE_CONFIG_DIR=~/.claude_copilot claude --model "GPT-5 mini"'
```

これで `claude-copilot` と打つだけで起動！

### 🤔 なぜ CLAUDE_CONFIG_DIR を指定するの？
ここが地味に大事なポイントです。コマンド例の中で `CLAUDE_CONFIG_DIR=~/.claude_copilot` という環境変数を指定しています。

これは **「Claude Code の設定フォルダ（履歴やインストール済みスキルなどを保存する場所）を、通常の Claude とは別の場所に分離する」** という設定です。
こうすることで、公式の Claude 向けのグローバル設定や重いスキルなどが読み込まれるのを防ぎ、エラーやコンフリクトを回避してピュアで軽量な環境を保つことができます。
（※フォルダは無くても初回起動時に自動作成されるので、自分で `mkdir` する必要はありません！）


## 🤔 これって大丈夫なの？

正直、最初は「グレーなハックでは……？」と思いました。  
でも冷静に考えると：

1. **Claude Code 側** → `ANTHROPIC_BASE_URL` を公式サポート。Ollamaとの連携も公式ドキュメントで紹介されている
2. **GitHub Copilot 側** → 公式に `copilot-sdk` パッケージを公開・提供している

つまり、**両社がそれぞれ公式に提供しているオープンなインターフェースを、正規の方法で組み合わせただけ**。  
リバースエンジニアリングも利用規約の回避もしていません。

とはいえ、本ツールはローカル環境での個人利用を前提としています。  
サーバーにデプロイしたり、外部に公開したりするのはやめましょう ⚠️

## 📊 感想まとめ

| 項目 | 評価 |
|------|------|
| セットアップの簡単さ | ⭐⭐⭐⭐⭐ びっくりするほど簡単 |
| 応答速度 | ⭐⭐⭐⭐ ほぼ体感差なし |
| コード品質 | ⭐⭐⭐⭐ 実用レベル |
| ワクワク度 | ⭐⭐⭐⭐⭐ 最高にCool 🔥 |

## おわりに 🎊

「Anthropicのエコシステム」と「GitHubのエコシステム」が、たった1本のGoバイナリで繋がる。  
しかも公式SDKだけで実現できてしまう。

**2026年、AI開発ツールのオープンさがここまで来たのか**、と感動しました。

Copilotのサブスクリプションを持っている方は、ぜひ試してみてください。  
「え、これで動くの？」という驚きを、きっと感じていただけると思います 🚀

---

*この記事で紹介した `claude-copilot` のソースコードは [GitHubリポジトリ](https://github.com/6in/claude-copilot) で公開しています。*  
*スター ⭐ いただけると励みになります！*

---

## おまけ

素直に、github copilot cli を使えばいいという話もあります。
まあ、ロマン枠ですね。
でも、今流行りのアレ(⽖)からは呼び出せそうな雰囲気はあります。