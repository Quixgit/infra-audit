package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type AuditorInvite struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id,omitempty"`
	UserID         string     `json:"user_id,omitempty"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Token          string     `json:"token,omitempty"`
	Permissions    []string   `json:"permissions"`
	ExpiresAt      time.Time  `json:"expires_at"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	// Populated on verify
	TenantName string `json:"tenant_name,omitempty"`
	AppURL     string `json:"app_url,omitempty"`
}

type AuditorComment struct {
	ID        string    `json:"id"`
	InviteID  string    `json:"invite_id"`
	TenantID  string    `json:"tenant_id,omitempty"`
	AuditorName string  `json:"auditor_name"`
	Section   string    `json:"section"`
	ItemID    string    `json:"item_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type FindingsSummary struct {
	Critical int                      `json:"critical"`
	High     int                      `json:"high"`
	Medium   int                      `json:"medium"`
	Low      int                      `json:"low"`
	Total    int                      `json:"total"`
	ByConnection []ConnectionFindingsSummary `json:"by_connection"`
}

type ConnectionFindingsSummary struct {
	ConnectionID   string `json:"connection_id"`
	ConnectionName string `json:"connection_name"`
	Critical       int    `json:"critical"`
	High           int    `json:"high"`
	Medium         int    `json:"medium"`
	Low            int    `json:"low"`
}

// ── Token generation ──────────────────────────────────────────────────────────

func generateAuditorToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ── DB helpers — invites ──────────────────────────────────────────────────────

func (srv *server) createAuditorInvite(ctx context.Context, tenantID, userID string, req createAuditorInviteRequest) (AuditorInvite, error) {
	token, err := generateAuditorToken()
	if err != nil {
		return AuditorInvite{}, err
	}

	days := req.ExpiryDays
	if days <= 0 {
		days = 30
	}
	expiresAt := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)

	permsJSON, _ := json.Marshal(req.Permissions)

	var id string
	err = srv.db.QueryRow(ctx, `
		INSERT INTO auditor_invites(tenant_id, user_id, name, email, token, permissions, expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		tenantID, userID, req.Name, req.Email, token, string(permsJSON), expiresAt,
	).Scan(&id)
	if err != nil {
		return AuditorInvite{}, err
	}
	return srv.getAuditorInviteByID(ctx, id, tenantID)
}

func (srv *server) getAuditorInviteByID(ctx context.Context, id, tenantID string) (AuditorInvite, error) {
	var inv AuditorInvite
	var permsJSON string
	err := srv.db.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, name, email, token, permissions, expires_at, last_accessed_at, created_at
		FROM auditor_invites WHERE id=$1 AND tenant_id=$2`, id, tenantID,
	).Scan(&inv.ID, &inv.TenantID, &inv.UserID, &inv.Name, &inv.Email, &inv.Token,
		&permsJSON, &inv.ExpiresAt, &inv.LastAccessedAt, &inv.CreatedAt)
	if err != nil {
		return inv, err
	}
	_ = json.Unmarshal([]byte(permsJSON), &inv.Permissions)
	if inv.Permissions == nil {
		inv.Permissions = []string{}
	}
	return inv, nil
}

func (srv *server) getAuditorInviteByToken(ctx context.Context, token string) (AuditorInvite, error) {
	var inv AuditorInvite
	var permsJSON string
	err := srv.db.QueryRow(ctx, `
		SELECT ai.id, ai.tenant_id, ai.user_id, ai.name, ai.email, ai.token,
		       ai.permissions, ai.expires_at, ai.last_accessed_at, ai.created_at,
		       COALESCE(t.name,'') as tenant_name
		FROM auditor_invites ai
		JOIN tenants t ON t.id = ai.tenant_id
		WHERE ai.token=$1 AND ai.expires_at > NOW()`, token,
	).Scan(&inv.ID, &inv.TenantID, &inv.UserID, &inv.Name, &inv.Email, &inv.Token,
		&permsJSON, &inv.ExpiresAt, &inv.LastAccessedAt, &inv.CreatedAt, &inv.TenantName)
	if err != nil {
		return inv, err
	}
	_ = json.Unmarshal([]byte(permsJSON), &inv.Permissions)
	if inv.Permissions == nil {
		inv.Permissions = []string{}
	}
	return inv, nil
}

func (srv *server) listAuditorInvites(ctx context.Context, tenantID string) ([]AuditorInvite, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT id, tenant_id, user_id, name, email, token, permissions, expires_at, last_accessed_at, created_at
		FROM auditor_invites WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditorInvite
	for rows.Next() {
		var inv AuditorInvite
		var permsJSON string
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.UserID, &inv.Name, &inv.Email, &inv.Token,
			&permsJSON, &inv.ExpiresAt, &inv.LastAccessedAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(permsJSON), &inv.Permissions)
		if inv.Permissions == nil {
			inv.Permissions = []string{}
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (srv *server) deleteAuditorInvite(ctx context.Context, id, tenantID string) error {
	tag, err := srv.db.Exec(ctx, `DELETE FROM auditor_invites WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (srv *server) touchAuditorInvite(ctx context.Context, token string) {
	_, _ = srv.db.Exec(ctx, `UPDATE auditor_invites SET last_accessed_at=NOW() WHERE token=$1`, token)
}

// ── DB helpers — comments ─────────────────────────────────────────────────────

func (srv *server) createAuditorComment(ctx context.Context, inviteID, tenantID, auditorName, section, itemID, body string) (AuditorComment, error) {
	var id string
	err := srv.db.QueryRow(ctx, `
		INSERT INTO auditor_comments(invite_id, tenant_id, auditor_name, section, item_id, body)
		VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		inviteID, tenantID, auditorName, section, itemID, body,
	).Scan(&id)
	if err != nil {
		return AuditorComment{}, err
	}
	var c AuditorComment
	err = srv.db.QueryRow(ctx, `
		SELECT id, invite_id, tenant_id, auditor_name, section, item_id, body, created_at
		FROM auditor_comments WHERE id=$1`, id,
	).Scan(&c.ID, &c.InviteID, &c.TenantID, &c.AuditorName, &c.Section, &c.ItemID, &c.Body, &c.CreatedAt)
	return c, err
}

func (srv *server) listAuditorComments(ctx context.Context, inviteID string) ([]AuditorComment, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT id, invite_id, tenant_id, auditor_name, section, item_id, body, created_at
		FROM auditor_comments WHERE invite_id=$1 ORDER BY created_at ASC`, inviteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditorComment
	for rows.Next() {
		var c AuditorComment
		if err := rows.Scan(&c.ID, &c.InviteID, &c.TenantID, &c.AuditorName, &c.Section, &c.ItemID, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (srv *server) listAllInviteComments(ctx context.Context, inviteID, tenantID string) ([]AuditorComment, error) {
	// Verify invite belongs to tenant
	var count int
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM auditor_invites WHERE id=$1 AND tenant_id=$2`, inviteID, tenantID).Scan(&count)
	if count == 0 {
		return nil, fmt.Errorf("not found")
	}
	return srv.listAuditorComments(ctx, inviteID)
}

// ── Permission check helper ───────────────────────────────────────────────────

func hasPerm(inv AuditorInvite, perm string) bool {
	for _, p := range inv.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// ── Request types ─────────────────────────────────────────────────────────────

type createAuditorInviteRequest struct {
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	ExpiryDays  int      `json:"expiry_days"`
	Permissions []string `json:"permissions"`
}

type addAuditorCommentRequest struct {
	Section string `json:"section"`
	ItemID  string `json:"item_id"`
	Body    string `json:"body"`
}

// ── Public handlers (token-based, no auth) ────────────────────────────────────

func (srv *server) handleVerifyAuditorToken(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}
	go srv.touchAuditorInvite(context.Background(), token)

	appURL := envOr("APP_URL", "http://localhost:3000")
	writeJSON(w, http.StatusOK, map[string]any{
		"name":        inv.Name,
		"company":     inv.TenantName,
		"expires_at":  inv.ExpiresAt,
		"permissions": inv.Permissions,
		"app_url":     appURL,
	})
}

func (srv *server) handleAuditorCompliance(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}
	if !hasPerm(inv, "compliance") {
		writeError(w, http.StatusForbidden, "not permitted")
		return
	}
	go srv.touchAuditorInvite(context.Background(), token)

	allFindings, err := srv.getAllAggregatedFindings(r.Context(), inv.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	result := make([]ComplianceFrameworkResponse, 0, len(complianceFrameworks))
	for _, fw := range complianceFrameworks {
		result = append(result, buildComplianceFramework(fw.Slug, allFindings, false))
	}
	writeJSON(w, http.StatusOK, result)
}

func (srv *server) handleAuditorEvidence(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}
	if !hasPerm(inv, "evidence") {
		writeError(w, http.StatusForbidden, "not permitted")
		return
	}
	go srv.touchAuditorInvite(context.Background(), token)

	items, err := srv.listEvidenceItems(r.Context(), inv.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (srv *server) handleAuditorPolicies(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}
	if !hasPerm(inv, "policies") {
		writeError(w, http.StatusForbidden, "not permitted")
		return
	}
	go srv.touchAuditorInvite(context.Background(), token)

	allPolicies, err := srv.listPolicies(r.Context(), inv.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	// Only approved policies visible to auditors
	var approved []Policy
	for _, p := range allPolicies {
		if p.Status == "Approved" {
			p.ContentHTML = "" // strip HTML body for list view
			approved = append(approved, p)
		}
	}
	if approved == nil {
		approved = []Policy{}
	}
	writeJSON(w, http.StatusOK, approved)
}

func (srv *server) handleAuditorFindingsSummary(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}
	if !hasPerm(inv, "findings") {
		writeError(w, http.StatusForbidden, "not permitted")
		return
	}
	go srv.touchAuditorInvite(context.Background(), token)

	allFindings, err := srv.getAllAggregatedFindings(r.Context(), inv.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	summary := FindingsSummary{}
	byConn := map[string]*ConnectionFindingsSummary{}
	for _, f := range allFindings {
		switch f.Severity {
		case "critical":
			summary.Critical++
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		case "low":
			summary.Low++
		}
		summary.Total++

		if _, ok := byConn[f.ConnectionID]; !ok {
			byConn[f.ConnectionID] = &ConnectionFindingsSummary{
				ConnectionID:   f.ConnectionID,
				ConnectionName: f.ConnectionName,
			}
		}
		cs := byConn[f.ConnectionID]
		switch f.Severity {
		case "critical":
			cs.Critical++
		case "high":
			cs.High++
		case "medium":
			cs.Medium++
		case "low":
			cs.Low++
		}
	}
	for _, cs := range byConn {
		summary.ByConnection = append(summary.ByConnection, *cs)
	}
	if summary.ByConnection == nil {
		summary.ByConnection = []ConnectionFindingsSummary{}
	}
	writeJSON(w, http.StatusOK, summary)
}

func (srv *server) handleAuditorDownloadEvidence(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}
	if !hasPerm(inv, "evidence") {
		writeError(w, http.StatusForbidden, "not permitted")
		return
	}

	id := chi.URLParam(r, "id")
	data, item, err := srv.getEvidenceData(r.Context(), id, inv.TenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "evidence not found")
		return
	}
	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+item.Name+`"`)
	_, _ = w.Write(data)
}

func (srv *server) handleAuditorDownloadPolicy(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}
	if !hasPerm(inv, "policies") {
		writeError(w, http.StatusForbidden, "not permitted")
		return
	}

	id := chi.URLParam(r, "id")
	// Get the policy and check it's approved
	policies, err := srv.listPolicies(r.Context(), inv.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	var found *Policy
	for i := range policies {
		if policies[i].ID == id && policies[i].Status == "Approved" {
			found = &policies[i]
			break
		}
	}
	if found == nil {
		writeError(w, http.StatusNotFound, "policy not found")
		return
	}
	// Serve as HTML if generated, or file if uploaded
	if found.FilePath != "" {
		http.ServeFile(w, r, found.FilePath)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+found.Name+`.html"`)
	_, _ = w.Write([]byte(found.ContentHTML))
}

func (srv *server) handleAuditorAddComment(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}

	var req addAuditorCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}

	c, err := srv.createAuditorComment(r.Context(), inv.ID, inv.TenantID, inv.Name, req.Section, req.ItemID, req.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save failed")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (srv *server) handleAuditorListComments(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := srv.getAuditorInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "portal not found or expired")
		return
	}

	comments, err := srv.listAuditorComments(r.Context(), inv.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	if comments == nil {
		comments = []AuditorComment{}
	}
	writeJSON(w, http.StatusOK, comments)
}

// ── Authenticated management handlers ─────────────────────────────────────────

func (srv *server) handleListAuditorInvites(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	invites, err := srv.listAuditorInvites(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	if invites == nil {
		invites = []AuditorInvite{}
	}
	writeJSON(w, http.StatusOK, invites)
}

func (srv *server) handleCreateAuditorInvite(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	userID := r.Context().Value(ctxUserID).(string)

	var req createAuditorInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Permissions) == 0 {
		req.Permissions = []string{"compliance", "evidence", "policies"}
	}

	inv, err := srv.createAuditorInvite(r.Context(), tenantID, userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create failed: "+err.Error())
		return
	}

	appURL := envOr("APP_URL", "http://localhost:3000")
	inv.AppURL = appURL + "/auditor/" + inv.Token
	writeJSON(w, http.StatusCreated, inv)
}

func (srv *server) handleDeleteAuditorInvite(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")
	if err := srv.deleteAuditorInvite(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleListInviteComments(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	inviteID := chi.URLParam(r, "id")
	comments, err := srv.listAllInviteComments(r.Context(), inviteID, tenantID)
	if err != nil {
		if err.Error() == "not found" {
			writeError(w, http.StatusNotFound, "invite not found")
		} else {
			writeError(w, http.StatusInternalServerError, "fetch failed")
		}
		return
	}
	if comments == nil {
		comments = []AuditorComment{}
	}
	writeJSON(w, http.StatusOK, comments)
}
