package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	copilot "github.com/github/copilot-sdk/go"

	"copilot-proxy/api"
	"copilot-proxy/auth"
	"copilot-proxy/config"
)

func main() {
	// CLI arguments
	port := flag.Int("port", 0, "ポート番号 (デフォルト: 8080、環境変数 PROXY_PORT でも指定可)")
	logoff := flag.Bool("logoff", false, "認証情報を削除してログアウト")
	flag.Parse()

	// Handle -logoff
	if *logoff {
		if err := config.DeleteConfig(); err != nil {
			fmt.Printf("❌ 設定ファイルの削除に失敗しました: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ 認証情報を削除しました: %s\n", config.GetConfigPath())
		os.Exit(0)
	}

	fmt.Println("Starting Copilot Proxy (Official SDK version)...")

	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Ensure GitHub Copilot authentication (Device Auth flow)
	if err := auth.EnsureToken(cfg); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// 3. Build Copilot SDK ClientOptions
	opts := &copilot.ClientOptions{
		GitHubToken: cfg.GitHubToken, // Pass our device-auth token to SDK
	}

	// Forward HTTPS_PROXY / HTTP_PROXY / NO_PROXY to the embedded CLI process
	var envVars []string
	for _, key := range []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy"} {
		if v := os.Getenv(key); v != "" {
			envVars = append(envVars, key+"="+v)
			fmt.Printf("🌐 Proxy: %s=%s\n", key, v)
		}
	}
	if len(envVars) > 0 {
		opts.Env = append(os.Environ(), envVars...)
	}

	client := copilot.NewClient(opts)
	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		log.Fatalf("Failed to start embedded Copilot CLI: %v", err)
	}
	defer client.Stop()

	// 4. Verify auth status
	authStatus, err := client.GetAuthStatus(ctx)
	if err != nil {
		fmt.Printf("⚠️  認証状態の確認に失敗: %v\n", err)
	} else if !authStatus.IsAuthenticated {
		fmt.Println("⚠️  認証されていません。トークンが期限切れの可能性があります。")
		fmt.Println("   -logoff で一度ログアウトしてから再起動してください。")
	} else {
		fmt.Println("✅ GitHub Copilot 認証OK")
	}

	// 5. Setup HTTP API Handlers
	handler := &api.Handler{CopilotClient: client}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", handler.HandleMessages)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Copilot Proxy is running"))
	})

	// 6. Determine port: CLI flag > env var > config file > default
	portStr := fmt.Sprintf("%d", *port)
	if *port == 0 {
		portStr = cfg.Port
		if portStr == "" {
			portStr = "8080"
		}
	}
	addr := ":" + portStr

	fmt.Printf("🚀 Server is running on http://localhost%s\n", addr)
	fmt.Printf("Configure Claude Code:\n")
	fmt.Printf("    ANTHROPIC_AUTH_TOKEN=dummy ANTHROPIC_BASE_URL=\"http://localhost%s\" claude --model \"GPT-5 mini\"\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Server failed: %v\n", err)
		os.Exit(1)
	}
}
