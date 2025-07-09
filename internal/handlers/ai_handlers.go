package handlers

import (
	"encoding/json"
	"fmt"
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

type MeetingSummaryRequest struct {
	Transcript       string `json:"transcript"`
	IdentifySpeakers bool   `json:"identify_speakers"`
}

type MeetingSummaryResponse struct {
	Summary     string   `json:"summary"`
	KeyPoints   []string `json:"key_points"`
	ActionItems []string `json:"action_items"`
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

func SummarizeMeetingHandler(w http.ResponseWriter, r *http.Request) {
	cfg := config.LoadConfig()

	// Parse request
	var req MeetingSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Transcript == "" {
		http.Error(w, "Transcript is required", http.StatusBadRequest)
		return
	}

	// Initialize OpenAI client
	client := openai.NewClient(cfg.OpenAIKey)

	// Create structured prompt for meeting summary
	prompt := fmt.Sprintf(`Analyze this meeting transcript and provide:
1. A concise summary (2-3 paragraphs)
2. important discussion points as bullet points
3. Clear action items with owners if specified
4. Format the response as JSON with "summary", "key_points", and "action_items" fields according to this type MeetingSummaryResponse struct {
	Summary     string  
	KeyPoints   []string
	ActionItems []string
}

%s

Respond ONLY with valid JSON containing these three fields. The transcript is:`, req.Transcript)

	// Call OpenAI API
	resp, err := client.CreateChatCompletion(r.Context(), openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.2, // Very low for consistent JSON output
		TopP:        0.8,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		http.Error(w, "AI processing failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the JSON response
	var summaryResponse MeetingSummaryResponse
	responseText := resp.Choices[0].Message.Content

	// Clean up the response if needed
	if !strings.Contains(responseText, "{") {
		// Handle cases where the response isn't properly formatted JSON
		summaryResponse = MeetingSummaryResponse{
			Summary:     "Could not parse summary",
			KeyPoints:   []string{"Error parsing key points"},
			ActionItems: []string{"Error parsing action items"},
		}
	} else {
		// Try to parse the JSON
		if err := json.Unmarshal([]byte(responseText), &summaryResponse); err != nil {
			fmt.Println(err.Error())
			http.Error(w, "Failed to parse AI response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Ensure we have at least minimal content
	if len(summaryResponse.KeyPoints) == 0 {
		summaryResponse.KeyPoints = []string{"No key points identified"}
	}
	if len(summaryResponse.ActionItems) == 0 {
		summaryResponse.ActionItems = []string{"No action items identified"}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaryResponse)
}
