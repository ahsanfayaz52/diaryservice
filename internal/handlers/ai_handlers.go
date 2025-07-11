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
	Summary      string       `json:"Summary"`
	KeyPoints    []string     `json:"KeyPoints"`
	ActionItems  []ActionItem `json:"ActionItems"`
	Participants []string     `json:"Participants"`
	Decisions    []Decision   `json:"Decisions"`
	FollowUps    []FollowUp   `json:"FollowUps"`
}

type ActionItem struct {
	Task         string   `json:"Task"`
	Owner        string   `json:"Owner"`
	Deadline     string   `json:"Deadline"`
	Dependencies []string `json:"Dependencies"`
}

type Decision struct {
	Description  string   `json:"Description"`
	Rationale    string   `json:"Rationale"`
	Alternatives []string `json:"Alternatives"`
}

type FollowUp struct {
	Action      string `json:"Action"`
	Responsible string `json:"Responsible"`
	Timeline    string `json:"Timeline"`
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
6. Return as properly formatted HTML with proper tags
7. remove any extra whitespaces and empty bullet points

Text to enhance:
` + req.Text

	case "summarize":
		prompt = `Create a well-formatted summary:
1. Use <h3> for section headers
2. Format with <ul> and <li> for bullet points
3. Include 1-2 sentence overview first
4. Keep concise but comprehensive
5. Return as HTML with proper tags

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

	// Create structured prompt with correct backticks
	prompt := fmt.Sprintf(`Analyze this meeting transcript thoroughly and provide a detailed breakdown in JSON format. Follow these instructions carefully:

1. SUMMARY (3-4 paragraphs):
   - Capture the main themes, decisions, and conclusions
   - Include important context and rationale
   - Highlight any opposing viewpoints discussed

2. KEY POINTS (comprehensive list):
   - Every substantive topic discussed
   - Technical details mentioned
   - Important questions raised
   - Concerns or objections voiced
   - Include direct quotes for critical statements

3. ACTION ITEMS (detailed):
   - All tasks mentioned with clear owners
   - Deadlines/deliverables if specified
   - Required resources noted
   - Dependencies between tasks

4. ADDITIONAL SECTIONS:
   - "participants": List all detected participants
   - "decisions": Clear decisions made with rationale
   - "follow_ups": Any agreed follow-up actions

Structure the response as valid JSON according to this Go struct:
type MeetingSummary struct {
    Summary      string       `+"`"+`json:"summary"`+"`"+`
    KeyPoints    []string     `+"`"+`json:"key_points"`+"`"+` 
    ActionItems  []ActionItem `+"`"+`json:"action_items"`+"`"+`
    Participants []string     `+"`"+`json:"participants"`+"`"+`
    Decisions    []Decision   `+"`"+`json:"decisions"`+"`"+`
    FollowUps    []FollowUp   `+"`"+`json:"follow_ups"`+"`"+`
}

type ActionItem struct {
    Task        string   `+"`"+`json:"task"`+"`"+`
    Owner       string   `+"`"+`json:"owner"`+"`"+`
    Deadline    string   `+"`"+`json:"deadline"`+"`"+`
    Dependencies []string `+"`"+`json:"dependencies"`+"`"+`
}

type Decision struct {
    Description string   `+"`"+`json:"description"`+"`"+`
    Rationale   string   `+"`"+`json:"rationale"`+"`"+`
    Alternatives []string `+"`"+`json:"alternatives"`+"`"+`
}

type FollowUp struct {
    Action      string `+"`"+`json:"action"`+"`"+`
    Responsible string `+"`"+`json:"responsible"`+"`"+`
    Timeline    string `+"`"+`json:"timeline"`+"`"+`
}

Rules:
- Be exhaustive - don't omit minor points
- Preserve technical specifics
- Maintain original terminology
- Include timestamps for key moments (format: [HH:MM])
- Extract all numbers, metrics and data points mentioned

Meeting transcript:
%s`, req.Transcript)

	// Call OpenAI API
	resp, err := client.CreateChatCompletion(r.Context(), openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.2,
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

	if !strings.Contains(responseText, "{") {
		// Handle cases where the response isn't properly formatted JSON
		summaryResponse = MeetingSummaryResponse{
			Summary:      "Could not parse summary",
			KeyPoints:    []string{"Error parsing key points"},
			ActionItems:  []ActionItem{{Task: "Error parsing action items"}},
			Participants: []string{"Error parsing participants"},
			Decisions:    []Decision{{Description: "Error parsing decisions"}},
			FollowUps:    []FollowUp{{Action: "Error parsing follow ups"}},
		}
	} else {
		// Try to parse the JSON
		if err := json.Unmarshal([]byte(responseText), &summaryResponse); err != nil {
			fmt.Println("Parsing error:", err.Error())
			http.Error(w, "Failed to parse AI response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Ensure we have at least minimal content
	if summaryResponse.Summary == "" {
		summaryResponse.Summary = "No summary generated"
	}
	if len(summaryResponse.KeyPoints) == 0 {
		summaryResponse.KeyPoints = []string{"No key points identified"}
	}
	if len(summaryResponse.ActionItems) == 0 {
		summaryResponse.ActionItems = []ActionItem{{Task: "No action items identified"}}
	}
	if len(summaryResponse.Participants) == 0 {
		summaryResponse.Participants = []string{"No participants identified"}
	}
	if len(summaryResponse.Decisions) == 0 {
		summaryResponse.Decisions = []Decision{{Description: "No decisions identified"}}
	}
	if len(summaryResponse.FollowUps) == 0 {
		summaryResponse.FollowUps = []FollowUp{{Action: "No follow ups identified"}}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summaryResponse); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
