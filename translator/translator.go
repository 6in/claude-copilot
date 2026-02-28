package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"copilot-proxy/models"

	copilot "github.com/github/copilot-sdk/go"
)

// HandleChatRequest processes incoming Anthropic requests and proxies them via the Copilot SDK Session
func HandleChatRequest(ctx context.Context, copilotClient *copilot.Client, anthropicReq *models.AnthropicRequest, w http.ResponseWriter) error {
	modelName := anthropicReq.Model
	if modelName == "" {
		modelName = "GPT-5 mini"
	}

	// 1. Create a Session with the Copilot CLI
	session, err := copilotClient.CreateSession(ctx, &copilot.SessionConfig{
		Model:               modelName,
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	})
	if err != nil {
		return fmt.Errorf("failed to create copilot session: %w", err)
	}
	defer session.Destroy()

	// 2. Build the aggregate Prompt
	// In the Anthropic API, system and user messages are separate.
	// For the SDK, we send a single MessageOptions request.
	// We'll concatenate them for the prompt to keep it simple for now, though
	// ideally the SDK would let us inject full chat history. The SDK `MessageOptions`
	// primarily takes a single string `Prompt`.

	fullPrompt := ""

	// Handle System Prompt
	if anthropicReq.System != nil {
		switch v := anthropicReq.System.(type) {
		case string:
			fullPrompt += "System: " + v + "\n\n"
		case []interface{}:
			for _, item := range v {
				if block, ok := item.(map[string]interface{}); ok {
					if t, ok := block["type"].(string); ok && t == "text" {
						if textVal, ok := block["text"].(string); ok {
							fullPrompt += "System: " + textVal + "\n\n"
						}
					}
				}
			}
		}
	}

	// Handle standard Messages (ignoring perfect role separation for the raw prompt text temporarily)
	for _, msg := range anthropicReq.Messages {
		contentStr := ""
		switch v := msg.Content.(type) {
		case string:
			contentStr = v
		case []interface{}:
			for _, item := range v {
				if block, ok := item.(map[string]interface{}); ok {
					if t, ok := block["type"].(string); ok && t == "text" {
						if textVal, ok := block["text"].(string); ok {
							contentStr += textVal
						}
					}
				}
			}
		}
		fullPrompt += fmt.Sprintf("%s: %s\n", msg.Role, contentStr)
	}

	if !anthropicReq.Stream {
		return handleNonStream(session, fullPrompt, w)
	}

	return handleStream(session, fullPrompt, w)
}

func handleNonStream(session *copilot.Session, prompt string, w http.ResponseWriter) error {
	ctx := context.Background()

	var finalResponse string
	done := make(chan struct{})

	// Register event listener
	unsubscribe := session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case copilot.AssistantMessage:
			if event.Data.Content != nil && *event.Data.Content != "" {
				finalResponse += *event.Data.Content
			}
		case copilot.SessionIdle:
			close(done)
		case copilot.SessionError:
			errMsg := "unknown error"
			if event.Data.Message != nil {
				errMsg = *event.Data.Message
			}
			fmt.Printf("Copilot SDK Error: %s\n", errMsg)
			close(done)
		}
	})
	defer unsubscribe()

	_, err := session.Send(ctx, copilot.MessageOptions{
		Prompt: prompt,
	})
	if err != nil {
		return fmt.Errorf("failed to send message via sdk: %w", err)
	}

	<-done

	resp := models.AnthropicResponse{
		ID:    "msg_copilot_sdk_" + session.SessionID,
		Type:  "message",
		Role:  "assistant",
		Model: "GPT-5 mini",
		Content: []models.AnthropicContent{
			{
				Type: "text",
				Text: finalResponse,
			},
		},
		StopReason:   "end_turn",
		StopSequence: nil,
		Usage: models.AnthropicUsage{
			InputTokens:  10, // Mock usage
			OutputTokens: 10,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(resp)
}

func handleStream(session *copilot.Session, prompt string, w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial message_start event immediately
	sendAnthropicEvent(w, flusher, "message_start", models.AnthropicEvent{
		Type: "message_start",
		Message: &models.AnthropicMessage{
			ID:   "msg_copilot_sdk_" + session.SessionID,
			Type: "message",
			Role: "assistant",
		},
	})

	// Send content block start (raw because of nested interface representation in models)
	startBlock := "event: content_block_start\n" + `data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}` + "\n\n"
	fmt.Fprint(w, startBlock)
	flusher.Flush()

	ctx := context.Background()

	// Create channels to handle sync execution and wait for the finish
	done := make(chan struct{})

	// Step 3: Register Session Event Listeners
	unsubscribe := session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		// Event: Assistant is streaming text back
		// In the current SDK, assistant.message typically emits when a message starts/stops,
		// but actual streaming data might be handled differently or batched.
		// Note: we assume event.Data.Content contains the newly streamed text.
		case copilot.AssistantMessage:
			if event.Data.Content != nil && *event.Data.Content != "" {
				sendAnthropicEvent(w, flusher, "content_block_delta", models.AnthropicEvent{
					Type:  "content_block_delta",
					Index: 0,
					Delta: &models.AnthropicDelta{
						Type: "text_delta",
						Text: *event.Data.Content,
					},
				})
			}

		case copilot.SessionIdle:
			// Stream finished
			close(done)
		case copilot.SessionError:
			errMsg := "unknown error"
			if event.Data.Message != nil {
				errMsg = *event.Data.Message
			}
			fmt.Printf("Copilot SDK Error: %s\n", errMsg)
			close(done)
		}
	})
	defer unsubscribe()

	// Send the prompt. The SDK will asynchronously fire the SessionEvent handlers above.
	_, err := session.Send(ctx, copilot.MessageOptions{
		Prompt: prompt,
	})

	if err != nil {
		return fmt.Errorf("failed to send message via sdk: %w", err)
	}

	// Wait for the stream to finish mapping
	<-done

	// Close content block
	endBlock := "event: content_block_stop\n" + `data: {"type": "content_block_stop", "index": 0}` + "\n\n"
	fmt.Fprint(w, endBlock)
	flusher.Flush()

	// Send message delta with stop reason
	sendAnthropicEvent(w, flusher, "message_delta", models.AnthropicEvent{
		Type: "message_delta",
		Delta: &models.AnthropicDelta{
			StopReason: "end_turn",
		},
	})

	// Send message_stop event
	sendAnthropicEvent(w, flusher, "message_stop", models.AnthropicEvent{
		Type: "message_stop",
	})

	return nil
}

// Helper to encode and send SSE events
func sendAnthropicEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, event models.AnthropicEvent) {
	if eventType == "content_block_start" {
		return
	}
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(data))
	flusher.Flush()
}
