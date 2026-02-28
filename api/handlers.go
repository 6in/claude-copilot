package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"claude-copilot/models"
	"claude-copilot/translator"

	copilot "github.com/github/copilot-sdk/go"
)

// Handler wraps the copilot SDK client and provides HTTP endpoints
type Handler struct {
	CopilotClient *copilot.Client
}

// HandleMessages processes POST /v1/messages requests from Claude Code
func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Decode incoming Anthropic request
	var anthropicReq models.AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// 2. Translate and execute via Copilot SDK
	if err := translator.HandleChatRequest(r.Context(), h.CopilotClient, &anthropicReq, w); err != nil {
		log.Printf("Error proxying request: %v", err)
		http.Error(w, fmt.Sprintf("Error proxying request: %v", err), http.StatusInternalServerError)
		return
	}
}
