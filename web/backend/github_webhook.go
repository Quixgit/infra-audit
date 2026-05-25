package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// githubWebhookPayload is the subset of fields we care about from GitHub push/PR events.
type githubWebhookPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	PullRequest struct {
		Number int    `json:"number"`
	} `json:"pull_request"`
	Action string `json:"action"`
}

// handleGitHubWebhook receives GitHub push/pull_request webhooks and triggers audits.
// Authenticated via HMAC-SHA256 X-Hub-Signature-256 header.
func (srv *server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	connectionID := chi.URLParam(r, "connectionId")

	// Look up the connection to get the webhook secret and tenant
	conn, err := srv.getConnection(r.Context(), connectionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	// Read body first (needed for signature verification)
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB max
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// Verify HMAC signature if webhook secret is configured
	if conn.GitHubWebhookSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if sig == "" {
			writeError(w, http.StatusUnauthorized, "missing X-Hub-Signature-256")
			return
		}
		if !verifyGitHubSignature(body, conn.GitHubWebhookSecret, sig) {
			writeError(w, http.StatusUnauthorized, "invalid signature")
			return
		}
	}

	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

	// Store the event in the database
	_, _ = srv.db.Exec(r.Context(), `
		INSERT INTO github_webhook_events(connection_id, tenant_id, event_type, delivery_id, payload)
		VALUES($1, $2, $3, $4, $5)`,
		connectionID, conn.TenantID, eventType, deliveryID, string(body))

	// Parse the event
	var event githubWebhookPayload
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	// Decide whether to trigger an audit
	shouldAudit := false
	switch eventType {
	case "push":
		// Only audit pushes to default branch (main/master)
		ref := strings.TrimPrefix(event.Ref, "refs/heads/")
		if ref == "main" || ref == "master" {
			shouldAudit = true
		}
	case "pull_request":
		// Audit on PR opened/synchronize
		if event.Action == "opened" || event.Action == "synchronize" {
			shouldAudit = true
		}
	}

	if shouldAudit {
		log.Printf("github-webhook: triggering audit for connection %s (event: %s, delivery: %s)",
			connectionID, eventType, deliveryID)
		go srv.runCodeAuditFromWebhook(conn.TenantID, connectionID, conn.ConnType)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"received":     true,
		"event":        eventType,
		"audit_queued": shouldAudit,
		"received_at":  time.Now().UTC(),
	})
}

// verifyGitHubSignature verifies the HMAC-SHA256 signature from GitHub.
func verifyGitHubSignature(body []byte, secret, signatureHeader string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	sigHex := signatureHeader[len(prefix):]
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, sigBytes)
}

// runCodeAuditFromWebhook creates and runs an audit job triggered by a GitHub webhook.
func (srv *server) runCodeAuditFromWebhook(tenantID, connectionID, connType string) {
	ctx := context.Background()

	// Fetch the connection's owner userID (needed for job creation — user_id is NOT NULL)
	conn, err := srv.getConnection(ctx, connectionID)
	if err != nil {
		log.Printf("github-webhook: getConnection %s: %v", connectionID, err)
		return
	}

	// Create the job using createJob
	job, err := srv.createJob(ctx, connectionID, conn.UserID, tenantID, connType)
	if err != nil {
		log.Printf("github-webhook: createJob for conn %s: %v", connectionID, err)
		return
	}

	log.Printf("github-webhook: starting audit job %s for conn %s (type: %s)", job.ID, connectionID, connType)

	switch connType {
	case "code", "github", "gitlab":
		srv.runCodeAudit(job.ID, connectionID, conn.UserID)
	case "aws":
		srv.runAWSAudit(job.ID, connectionID, conn.UserID)
	default:
		srv.runAudit(job.ID, connectionID, conn.UserID)
	}
}
