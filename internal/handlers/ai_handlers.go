package handlers

import (
	"encoding/json"
	"github.com/ahsanfayaz52/diaryservice/internal/config"
	"net/http"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type AIRequest struct {
	Text   string `json:"text"`
	Action string `json:"action"`
}

type AIResponse struct {
	Text string `json:"text"`
}

func AIProcessHandler(w http.ResponseWriter, r *http.Request) {
	cfg := config.LoadConfig()

	// Parse request
	var req AIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "Text is required", http.StatusBadRequest)
		return
	}

	// Initialize OpenAI client
	client := openai.NewClient(cfg.OpenAIKey)

	// Create prompt with explicit formatting instructions
	var prompt string
	switch req.Action {
	case "enhance":
		prompt = `Please enhance this text with beautiful formatting:
1. Use proper paragraphs and line breaks
2. Add section headers where appropriate
3. Format lists with bullet points
4. Improve readability with spacing
5. Maintain original meaning
6. Return as properly formatted HTML with <p> tags

Text to enhance:
` + req.Text

	case "summarize":
		prompt = `Create a well-formatted summary:
1. Use <h3> for section headers
2. Format with <ul> and <li> for bullet points
3. Include 1-2 sentence overview first
4. Keep concise but comprehensive
5. Return as HTML

Text to summarize:
` + req.Text

	case "fix":
		prompt = `Correct grammar and spelling while:
1. Preserving all formatting
2. Maintaining original structure
3. Improving readability
4. Returning as HTML with proper <p> tags

Text to correct:
` + req.Text

	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	// Call OpenAI API with better parameters
	resp, err := client.CreateChatCompletion(r.Context(), openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.3, // Lower for more consistent formatting
		TopP:        0.9,
	})
	if err != nil {
		http.Error(w, "AI processing failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Process response to ensure proper HTML
	responseText := resp.Choices[0].Message.Content

	// Basic cleanup if needed
	if !strings.Contains(responseText, "<p>") {
		// Add basic paragraph formatting if missing
		responseText = "<p>" + strings.ReplaceAll(responseText, "\n\n", "</p><p>") + "</p>"
	}

	// Prepare response
	response := AIResponse{
		Text: responseText,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
