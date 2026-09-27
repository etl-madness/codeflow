package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"codeflow/pkg/model"
)

// HTTPClient interface for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Enricher enriches technical AST steps into business-readable descriptions using AI.
type Enricher struct {
	client     HTTPClient
	geminiKey  string
	openAIKey  string
	mockPrompt func(prompt string) (string, error)
}

// New creates a new AI Enricher with environment keys.
func New() *Enricher {
	return &Enricher{
		client:    &http.Client{Timeout: 30 * time.Second},
		geminiKey: os.Getenv("GEMINI_API_KEY"),
		openAIKey: os.Getenv("OPENAI_API_KEY"),
	}
}

// SetClient allows injecting a custom HTTPClient for testing.
func (e *Enricher) SetClient(client HTTPClient) {
	e.client = client
}

// SetMockHandler allows mock AI responses in unit tests.
func (e *Enricher) SetMockHandler(fn func(prompt string) (string, error)) {
	e.mockPrompt = fn
}

type StepInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Language    string `json:"language"`
	Description string `json:"description"`
}

type StepOutput struct {
	ID                  string `json:"id"`
	BusinessDescription string `json:"business_description"`
}

// Enrich processes steps in ProcessModel and updates their descriptions.
func (e *Enricher) Enrich(ctx context.Context, pm *model.ProcessModel) error {
	if len(pm.Steps) == 0 {
		return nil
	}

	// Prepare inputs
	inputs := make([]StepInput, len(pm.Steps))
	for i, s := range pm.Steps {
		inputs[i] = StepInput{
			ID:          s.ID,
			Name:        s.Name,
			Type:        s.Type,
			Language:    s.Language,
			Description: s.Description,
		}
	}

	inputBytes, _ := json.Marshal(inputs)
	prompt := fmt.Sprintf(
		"You are an enterprise software architect. Convert the following technical code steps and method names into concise, business-readable process step descriptions (1 sentence each). Respond strictly with valid JSON array of objects: [{\"id\": \"...\", \"business_description\": \"...\"}].\n\nInput:\n%s",
		string(inputBytes),
	)

	var responseText string
	var err error

	if e.mockPrompt != nil {
		responseText, err = e.mockPrompt(prompt)
	} else if e.geminiKey != "" {
		responseText, err = e.callGemini(ctx, prompt)
	} else if e.openAIKey != "" {
		responseText, err = e.callOpenAI(ctx, prompt)
	} else {
		// Fallback: rule-based semantic enrichment
		e.fallbackRuleBasedEnrichment(pm)
		return nil
	}

	if err != nil {
		// If network call fails, fall back gracefully
		e.fallbackRuleBasedEnrichment(pm)
		return fmt.Errorf("AI enrichment call failed, applied semantic fallback: %w", err)
	}

	// Parse JSON array of descriptions
	cleanedJSON := extractJSONFromResponse(responseText)
	var outputs []StepOutput
	if err := json.Unmarshal([]byte(cleanedJSON), &outputs); err != nil {
		e.fallbackRuleBasedEnrichment(pm)
		return nil
	}

	outputMap := make(map[string]string)
	for _, o := range outputs {
		if o.BusinessDescription != "" {
			outputMap[o.ID] = o.BusinessDescription
		}
	}

	for i := range pm.Steps {
		if desc, ok := outputMap[pm.Steps[i].ID]; ok {
			pm.Steps[i].Description = desc
		}
	}

	return nil
}

func (e *Enricher) callGemini(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", e.geminiKey)
	reqBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": prompt},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respData, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(respData))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		return geminiResp.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("empty response from Gemini API")
}

func (e *Enricher) callOpenAI(ctx context.Context, prompt string) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"
	reqBody := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "system", "content": "You are a business process analyst."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.openAIKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respData, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(respData))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) > 0 {
		return openAIResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("empty response from OpenAI API")
}

func (e *Enricher) fallbackRuleBasedEnrichment(pm *model.ProcessModel) {
	for i := range pm.Steps {
		s := &pm.Steps[i]
		s.Description = translateToBusinessTerms(s.Name, s.Type)
	}
}

func translateToBusinessTerms(name, stepType string) string {
	cleanName := splitCamelCase(name)
	lower := strings.ToLower(cleanName)

	switch {
	case strings.Contains(lower, "order"):
		return fmt.Sprintf("Process and manage customer order: %s", cleanName)
	case strings.Contains(lower, "pay") || strings.Contains(lower, "checkout") || strings.Contains(lower, "billing"):
		return fmt.Sprintf("Process payment transaction and financial authorization: %s", cleanName)
	case strings.Contains(lower, "user") || strings.Contains(lower, "customer") || strings.Contains(lower, "auth"):
		return fmt.Sprintf("Authenticate user and verify account profile: %s", cleanName)
	case strings.Contains(lower, "query") || strings.Contains(lower, "select") || strings.Contains(lower, "get") || strings.Contains(lower, "find"):
		return fmt.Sprintf("Query and retrieve operational data records: %s", cleanName)
	case strings.Contains(lower, "insert") || strings.Contains(lower, "save") || strings.Contains(lower, "add") || strings.Contains(lower, "create"):
		return fmt.Sprintf("Store and persist transactional record: %s", cleanName)
	case strings.Contains(lower, "update") || strings.Contains(lower, "modify"):
		return fmt.Sprintf("Update existing business records: %s", cleanName)
	case strings.Contains(lower, "delete") || strings.Contains(lower, "remove"):
		return fmt.Sprintf("Archive or remove business record: %s", cleanName)
	case stepType == "Endpoint":
		return fmt.Sprintf("Service API Endpoint for: %s", cleanName)
	default:
		return fmt.Sprintf("Execute business operation: %s", cleanName)
	}
}

func splitCamelCase(s string) string {
	// Replaces camelCase or PascalCase or snake_case with spaced words
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "/", " ")
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	s = re.ReplaceAllString(s, `$1 $2`)
	return strings.TrimSpace(s)
}

func extractJSONFromResponse(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
