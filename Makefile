APP_NAME := copilot-proxy
MODULE   := copilot-proxy
BIN_DIR  := bin
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all build build-all build-linux build-windows build-darwin clean run tidy test

## デフォルト: 現在のOS向けビルド
all: build

## 現在のOS/Arch向けビルド
build:
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME) .
	@echo "Built: $(BIN_DIR)/$(APP_NAME)"

## 全プラットフォーム一括ビルド
build-all: build-linux build-windows build-darwin
	@echo "All platforms built in $(BIN_DIR)/"

## Linux (amd64 / arm64)
build-linux:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-linux-arm64 .
	@echo "Built: Linux (amd64, arm64)"

## Windows (amd64 / arm64)
build-windows:
	@mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-windows-arm64.exe .
	@echo "Built: Windows (amd64, arm64)"

## macOS (amd64 / arm64)
build-darwin:
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-darwin-arm64 .
	@echo "Built: macOS (amd64, arm64)"

## サーバー起動
run: build
	./$(BIN_DIR)/$(APP_NAME)

## 依存整理
tidy:
	go mod tidy

## テスト実行
test:
	go test ./...

## ビルド成果物削除
clean:
	rm -rf $(BIN_DIR)
	@echo "Cleaned $(BIN_DIR)/"
