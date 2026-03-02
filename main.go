package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	copilot "github.com/github/copilot-sdk/go"

	"claude-copilot/api"
	"claude-copilot/auth"
	"claude-copilot/config"
)

func main() {
	// CLI arguments
	port := flag.Int("port", 0, "ポート番号 (デフォルト: 8080、環境変数 PROXY_PORT でも指定可)")
	logoff := flag.Bool("logoff", false, "認証情報を削除してログアウト")
	debug := flag.Bool("debug", false, "詳細なデバッグログ（プロンプトの中身など）を出力する")
	insecure := flag.Bool("insecure", false, "プロキシ環境などで TLS 証明書検証をスキップする（NODE_TLS_REJECT_UNAUTHORIZED=0）")
	caCert := flag.String("ca-cert", "", "追加のCA証明書ファイルパス（NODE_EXTRA_CA_CERTS に設定）")
	copilotCLIPath := flag.String("copilot-cli", "", "Copilot CLI のパス（環境変数 COPILOT_CLI_PATH でも指定可）")
	nodeOptions := flag.String("node-options", "", "Node.js の追加オプション（NODE_OPTIONS に設定）")
	nodePath := flag.String("node-path", "", "Node.js のモジュールパス（NODE_PATH に設定）")
	nodeBin := flag.String("node-bin", "", "Node.js のbinディレクトリをPATHの先頭に追加")
	cliInstallVerbose := flag.Bool("cli-install-verbose", false, "埋め込みCLIのインストールログを詳細化（COPILOT_CLI_INSTALL_VERBOSE=1）")
	sdkDebug := flag.Bool("sdk-debug", false, "Copilot SDK のログレベルを debug に設定")
	cliStderr := flag.String("cli-stderr", "", "Copilot CLI のstderrを保存するファイルパス")
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

	// 2. Propagate --insecure to auth package (for Go HTTP requests)
	if *insecure {
		auth.Insecure = true
	}

	// 3. Ensure GitHub Copilot authentication (Device Auth flow)
	if err := auth.EnsureToken(cfg); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// 4. Build Copilot SDK ClientOptions
	opts := &copilot.ClientOptions{
		GitHubToken: cfg.GitHubToken, // Pass our device-auth token to SDK
	}
	if *copilotCLIPath != "" {
		opts.CLIPath = *copilotCLIPath
	}
	if *sdkDebug {
		opts.LogLevel = "debug"
	}

	// Build environment variables for the embedded CLI process
	cliEnv := os.Environ()

	// Show proxy configuration (mask credentials)
	hasProxy := false
	for _, key := range []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy"} {
		if v := os.Getenv(key); v != "" {
			hasProxy = true
			fmt.Printf("🌐 Proxy: %s=%s\n", key, sanitizeProxyValue(v))
		}
	}

	// --insecure: Skip TLS certificate verification for the embedded Node.js CLI
	// (useful when corporate proxy performs SSL interception)
	if *insecure || os.Getenv("NODE_TLS_REJECT_UNAUTHORIZED") == "0" {
		cliEnv = append(cliEnv, "NODE_TLS_REJECT_UNAUTHORIZED=0")
		fmt.Println("⚠️  TLS証明書検証を無効化しています (--insecure)")
	} else if hasProxy {
		fmt.Println("💡 プロキシ環境でTLSエラーが発生する場合は --insecure オプションを試してください")
		fmt.Println("   より安全な方法: --ca-cert /path/to/corporate-ca.pem")
	}

	// --ca-cert or NODE_EXTRA_CA_CERTS: Add custom CA certificate
	if *caCert != "" {
		cliEnv = setEnvValue(cliEnv, "NODE_EXTRA_CA_CERTS", *caCert)
		fmt.Printf("🔐 CA証明書を追加: %s\n", *caCert)
	} else if v := os.Getenv("NODE_EXTRA_CA_CERTS"); v != "" {
		fmt.Printf("🔐 CA証明書 (env): %s\n", v)
	}

	if *cliInstallVerbose {
		cliEnv = setEnvValue(cliEnv, "COPILOT_CLI_INSTALL_VERBOSE", "1")
		fmt.Println("🧩 Copilot CLI install verbose enabled")
	}

	// Copilot CLI path override (flag > env)
	if *copilotCLIPath != "" {
		cliEnv = setEnvValue(cliEnv, "COPILOT_CLI_PATH", *copilotCLIPath)
		fmt.Printf("🧭 Copilot CLI path: %s\n", *copilotCLIPath)
	} else if v := os.Getenv("COPILOT_CLI_PATH"); v != "" {
		cliEnv = setEnvValue(cliEnv, "COPILOT_CLI_PATH", v)
		fmt.Printf("🧭 Copilot CLI path (env): %s\n", v)
	}

	// Capture CLI stderr if requested (requires explicit CLI path)
	if *cliStderr != "" {
		resolvedCLIPath := *copilotCLIPath
		if resolvedCLIPath == "" {
			resolvedCLIPath = os.Getenv("COPILOT_CLI_PATH")
		}
		if resolvedCLIPath == "" {
			fmt.Println("⚠️  --cli-stderr を指定する場合は --copilot-cli も指定してください")
		} else {
			wrapperPath, err := createCLIWrapper(resolvedCLIPath, *cliStderr)
			if err != nil {
				fmt.Printf("⚠️  CLI stderr wrapper 作成に失敗: %v\n", err)
			} else {
				opts.CLIPath = wrapperPath
				cliEnv = setEnvValue(cliEnv, "COPILOT_CLI_PATH", wrapperPath)
				fmt.Printf("🧾 CLI stderr: %s\n", *cliStderr)
			}
		}
	}

	// Node.js runtime overrides
	if *nodeOptions != "" {
		cliEnv = setEnvValue(cliEnv, "NODE_OPTIONS", *nodeOptions)
		fmt.Println("🧪 NODE_OPTIONS set")
	}
	if *nodePath != "" {
		cliEnv = setEnvValue(cliEnv, "NODE_PATH", *nodePath)
		fmt.Printf("🧭 NODE_PATH: %s\n", *nodePath)
	}
	if *nodeBin != "" {
		pathValue := *nodeBin + string(os.PathListSeparator) + os.Getenv("PATH")
		cliEnv = setEnvValue(cliEnv, "PATH", pathValue)
		fmt.Printf("🧭 PATH (prepend): %s\n", *nodeBin)
	}

	opts.Env = cliEnv

	client := copilot.NewClient(opts)
	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		fmt.Println("\n❌ Copilot CLI の起動に失敗しました。")
		if hasProxy {
			fmt.Println("\n📋 プロキシ環境での対処法:")
			fmt.Println("  1. --insecure フラグを付けて再実行:")
			fmt.Printf("     %s --insecure\n", os.Args[0])
			fmt.Println("  2. 企業CA証明書を指定して再実行 (推奨):")
			fmt.Printf("     %s --ca-cert /path/to/corporate-ca.pem\n", os.Args[0])
			fmt.Println("  3. 環境変数で指定:")
			fmt.Println("     NODE_TLS_REJECT_UNAUTHORIZED=0", os.Args[0])
		}
		log.Fatalf("Failed to start embedded Copilot CLI: %v", err)
	}
	defer client.Stop()

	// 5. Verify auth status
	authStatus, err := client.GetAuthStatus(ctx)
	if err != nil {
		fmt.Printf("⚠️  認証状態の確認に失敗: %v\n", err)
	} else if !authStatus.IsAuthenticated {
		fmt.Println("⚠️  認証されていません。トークンが期限切れの可能性があります。")
		fmt.Println("   -logoff で一度ログアウトしてから再起動してください。")
	} else {
		fmt.Println("✅ GitHub Copilot 認証OK")
	}

	// 6. Setup HTTP API Handlers
	handler := &api.Handler{
		CopilotClient: client,
		Debug:         *debug,
	}

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

	// 7. Determine port: CLI flag > env var > config file > default
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
	fmt.Printf("    ANTHROPIC_AUTH_TOKEN=dummy \\\n")
	fmt.Printf("    ANTHROPIC_BASE_URL=\"http://localhost%s\" \\\n", addr)
	fmt.Printf("    CLAUDE_CONFIG_DIR=~/.claude_copilot \\\n")
	fmt.Printf("    claude --model \"GPT-5 mini\"\n")

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Server failed: %v\n", err)
		os.Exit(1)
	}
}

func sanitizeProxyValue(value string) string {
	parsed, err := url.Parse(value)
	if err == nil && parsed.User != nil {
		parsed.User = url.User("****")
		return parsed.String()
	}

	if strings.Contains(value, "@") {
		parts := strings.SplitN(value, "@", 2)
		return "****@" + parts[1]
	}

	return value
}

func setEnvValue(env []string, key string, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func createCLIWrapper(cliPath string, stderrPath string) (string, error) {
	wrapper, err := os.CreateTemp(os.TempDir(), "copilot-cli-wrapper-*.sh")
	if err != nil {
		return "", err
	}
	defer wrapper.Close()

	if err := os.MkdirAll(filepath.Dir(stderrPath), 0755); err != nil {
		return "", err
	}

	cliQuoted := shellQuote(cliPath)
	stderrQuoted := shellQuote(stderrPath)
	script := "#!/bin/sh\nset -e\nexec " + cliQuoted + " \"$@\" 2>>" + stderrQuoted + "\n"
	if _, err := wrapper.WriteString(script); err != nil {
		return "", err
	}
	if err := os.Chmod(wrapper.Name(), 0700); err != nil {
		return "", err
	}

	return wrapper.Name(), nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
