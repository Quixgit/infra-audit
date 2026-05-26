package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ── AI remediation suggestions (Groq API — OpenAI-compatible) ─────────────────

type groqRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	Messages  []groqMessage  `json:"messages"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type AISuggestion struct {
	Commands    []string `json:"commands"`
	Explanation string   `json:"explanation"`
	DocLinks    []string `json:"doc_links"`
	Difficulty  string   `json:"difficulty"` // easy | medium | hard
	EstTime     string   `json:"est_time"`
	Error       string   `json:"error,omitempty"`
	Fallback    string   `json:"fallback,omitempty"`
}

func callGroqAPI(ctx context.Context, system, prompt string) (string, error) {
	apiKey := envOr("GROQ_API_KEY", "")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY not configured")
	}

	reqBody := groqRequest{
		Model:     "llama-3.1-8b-instant",
		MaxTokens: 1024,
		Messages: []groqMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
	}

	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq api error %d: %s", resp.StatusCode, string(body))
	}

	var result groqResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from Groq")
	}
	return result.Choices[0].Message.Content, nil
}

func (srv *server) handleGetAISuggestion(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	taskID := chi.URLParam(r, "id")

	// Fetch the remediation task
	var task struct {
		Title           string
		Description     string
		RemediationText string
		Severity        string
		ResourceName    string
		ConnType        string
	}
	err := srv.db.QueryRow(r.Context(), `
		SELECT rt.title, rt.description, rt.remediation_text, rt.severity,
		       rt.resource_name,
		       COALESCE(c.conn_type, 'do') as conn_type
		FROM remediation_tasks rt
		LEFT JOIN connections c ON c.id = rt.connection_id
		WHERE rt.id=$1 AND rt.tenant_id=$2`, taskID, tenantID,
	).Scan(&task.Title, &task.Description, &task.RemediationText,
		&task.Severity, &task.ResourceName, &task.ConnType)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	providerHint := "DigitalOcean"
	if task.ConnType == "aws" {
		providerHint = "AWS"
	} else if task.ConnType == "code" {
		providerHint = "application code"
	}

	system := `You are a cloud security expert. You provide precise, actionable security remediation instructions.
Always respond with valid JSON only — no markdown, no code fences, no extra text.
Provide real, working commands specific to the platform.`

	prompt := fmt.Sprintf(`Security finding requiring remediation:

Title: %s
Severity: %s
Resource: %s
Provider: %s
Description: %s
Existing guidance: %s

Provide a concrete, actionable remediation plan as JSON with exactly these keys:
{
  "commands": ["command1", "command2"],
  "explanation": "step by step explanation",
  "doc_links": ["https://..."],
  "difficulty": "easy|medium|hard",
  "est_time": "5 minutes"
}`,
		task.Title, task.Severity, task.ResourceName, providerHint,
		task.Description, task.RemediationText)

	log.Printf("ai-suggest: generating for task %s (groq)", taskID)

	response, err := callGroqAPI(r.Context(), system, prompt)
	if err != nil {
		writeJSON(w, http.StatusOK, AISuggestion{
			Error:       err.Error(),
			Fallback:    task.RemediationText,
			Commands:    []string{},
			Explanation: task.RemediationText,
			DocLinks:    []string{},
			Difficulty:  "medium",
			EstTime:     "Unknown",
		})
		return
	}

	// Strip markdown code fences if model added them anyway
	cleaned := strings.TrimSpace(response)
	if idx := strings.Index(cleaned, "{"); idx > 0 {
		cleaned = cleaned[idx:]
	}
	if idx := strings.LastIndex(cleaned, "}"); idx >= 0 {
		cleaned = cleaned[:idx+1]
	}

	var suggestion AISuggestion
	if err := json.Unmarshal([]byte(cleaned), &suggestion); err != nil {
		// Return raw text as explanation fallback
		writeJSON(w, http.StatusOK, AISuggestion{
			Explanation: response,
			Commands:    []string{},
			DocLinks:    []string{},
			Difficulty:  "medium",
			EstTime:     "varies",
		})
		return
	}

	writeJSON(w, http.StatusOK, suggestion)
}
