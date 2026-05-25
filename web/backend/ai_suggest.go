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

// ── AI remediation suggestions (Anthropic Claude API) ─────────────────────────

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type AISuggestion struct {
	Commands    []string `json:"commands"`
	Explanation string   `json:"explanation"`
	DocLinks    []string `json:"doc_links"`
	Difficulty  string   `json:"difficulty"` // easy | medium | hard
	EstTime     string   `json:"est_time"`
}

func callAnthropicAPI(ctx context.Context, prompt, system string) (string, error) {
	apiKey := envOr("ANTHROPIC_API_KEY", "")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not configured")
	}

	reqBody := anthropicRequest{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1024,
		System:    system,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic api error %d: %s", resp.StatusCode, string(body))
	}

	var result anthropicResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}
	return result.Content[0].Text, nil
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
		ResourceType    string
		ResourceName    string
		ConnType        string
	}
	err := srv.db.QueryRow(r.Context(), `
		SELECT rt.title, rt.description, rt.remediation_text, rt.severity,
		       rt.resource_name, rt.connection_name,
		       COALESCE(c.conn_type, 'do') as conn_type
		FROM remediation_tasks rt
		LEFT JOIN connections c ON c.id = rt.connection_id
		WHERE rt.id=$1 AND rt.tenant_id=$2`, taskID, tenantID,
	).Scan(&task.Title, &task.Description, &task.RemediationText,
		&task.Severity, &task.ResourceName, &task.ConnType, &task.ConnType)
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

	prompt := fmt.Sprintf(`Security finding requiring remediation:

Title: %s
Severity: %s
Resource: %s
Provider: %s
Description: %s
Existing guidance: %s

Provide a concrete, actionable remediation plan. Include:
1. Exact commands to fix this (with placeholders where needed)
2. Step-by-step explanation
3. Links to official documentation
4. Difficulty estimate (easy/medium/hard)
5. Time estimate

Format your response as JSON with these keys:
{
  "commands": ["command1", "command2"],
  "explanation": "step by step explanation",
  "doc_links": ["https://..."],
  "difficulty": "easy|medium|hard",
  "est_time": "5 minutes|1 hour|etc"
}`,
		task.Title, task.Severity, task.ResourceName, providerHint,
		task.Description, task.RemediationText)

	system := `You are a cloud security expert. You provide precise, actionable security remediation instructions.
Always respond with valid JSON only, no markdown. Provide real, working commands specific to the platform.`

	log.Printf("ai-suggest: generating for task %s", taskID)

	response, err := callAnthropicAPI(r.Context(), prompt, system)
	if err != nil {
		// Return structured error so UI can handle gracefully
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"error":       err.Error(),
			"fallback":    task.RemediationText,
			"commands":    []string{},
			"explanation": task.RemediationText,
			"doc_links":   []string{},
			"difficulty":  "medium",
			"est_time":    "Unknown",
		})
		return
	}

	// Try to parse as JSON
	var suggestion AISuggestion
	// Claude might wrap in markdown code blocks, strip them
	cleaned := strings.TrimSpace(response)
	if idx := strings.Index(cleaned, "{"); idx > 0 {
		cleaned = cleaned[idx:]
	}
	if idx := strings.LastIndex(cleaned, "}"); idx >= 0 {
		cleaned = cleaned[:idx+1]
	}

	if err := json.Unmarshal([]byte(cleaned), &suggestion); err != nil {
		// Return raw text as explanation
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
