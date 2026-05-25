package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"infra-audit/internal/doapi"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type AccessReview struct {
	ID            string     `json:"id"`
	UserID        string     `json:"-"`
	TenantID      string     `json:"tenant_id,omitempty"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	ReviewType    string     `json:"review_type"` // manual | do_team | github_org
	ConnectionID  *string    `json:"connection_id,omitempty"`
	Status        string     `json:"status"` // draft | in_progress | completed | overdue
	DueDate       *string    `json:"due_date,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ItemCount     int        `json:"item_count"`
	ReviewedCount int        `json:"reviewed_count"`
	CreatedAt     time.Time  `json:"created_at"`
}

type AccessReviewItem struct {
	ID              string     `json:"id"`
	ReviewID        string     `json:"review_id"`
	SubjectName     string     `json:"subject_name"`
	SubjectEmail    string     `json:"subject_email"`
	SubjectRole     string     `json:"subject_role"`
	AccessLevel     string     `json:"access_level"`
	LastActiveAt    *time.Time `json:"last_active_at,omitempty"`
	Decision        string     `json:"decision"` // pending | approved | revoked | needs_followup
	DecidedByUserID *string    `json:"decided_by_user_id,omitempty"`
	DecidedByEmail  string     `json:"decided_by_email,omitempty"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
	Notes           string     `json:"notes"`
}

// ── DB helpers — reviews ──────────────────────────────────────────────────────

func (srv *server) listAccessReviews(ctx context.Context, tenantID string) ([]AccessReview, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT r.id, r.user_id, r.tenant_id, r.name, r.description, r.review_type,
		       r.connection_id, r.status, to_char(r.due_date,'YYYY-MM-DD'),
		       r.completed_at, r.created_at,
		       COUNT(i.id) as item_count,
		       COUNT(i.id) FILTER (WHERE i.decision <> 'pending') as reviewed_count
		FROM access_reviews r
		LEFT JOIN access_review_items i ON i.review_id = r.id
		WHERE r.tenant_id = $1
		GROUP BY r.id
		ORDER BY r.created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccessReview
	for rows.Next() {
		var rv AccessReview
		if err := rows.Scan(
			&rv.ID, &rv.UserID, &rv.TenantID, &rv.Name, &rv.Description, &rv.ReviewType,
			&rv.ConnectionID, &rv.Status, &rv.DueDate, &rv.CompletedAt, &rv.CreatedAt,
			&rv.ItemCount, &rv.ReviewedCount,
		); err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, rows.Err()
}

func (srv *server) getAccessReview(ctx context.Context, id, tenantID string) (AccessReview, error) {
	var rv AccessReview
	err := srv.db.QueryRow(ctx, `
		SELECT r.id, r.user_id, r.tenant_id, r.name, r.description, r.review_type,
		       r.connection_id, r.status, to_char(r.due_date,'YYYY-MM-DD'),
		       r.completed_at, r.created_at,
		       COUNT(i.id) as item_count,
		       COUNT(i.id) FILTER (WHERE i.decision <> 'pending') as reviewed_count
		FROM access_reviews r
		LEFT JOIN access_review_items i ON i.review_id = r.id
		WHERE r.id = $1 AND r.tenant_id = $2
		GROUP BY r.id`, id, tenantID,
	).Scan(
		&rv.ID, &rv.UserID, &rv.TenantID, &rv.Name, &rv.Description, &rv.ReviewType,
		&rv.ConnectionID, &rv.Status, &rv.DueDate, &rv.CompletedAt, &rv.CreatedAt,
		&rv.ItemCount, &rv.ReviewedCount,
	)
	return rv, err
}

func (srv *server) createAccessReview(ctx context.Context, tenantID, userID string, req createAccessReviewRequest) (AccessReview, error) {
	var id string
	err := srv.db.QueryRow(ctx, `
		INSERT INTO access_reviews(tenant_id, user_id, name, description, review_type, connection_id, status, due_date)
		VALUES($1,$2,$3,$4,$5,$6,'in_progress',$7)
		RETURNING id`,
		tenantID, userID, req.Name, req.Description, req.ReviewType,
		req.ConnectionID, req.DueDate,
	).Scan(&id)
	if err != nil {
		return AccessReview{}, err
	}
	return srv.getAccessReview(ctx, id, tenantID)
}

func (srv *server) updateAccessReview(ctx context.Context, id, tenantID string, req updateAccessReviewRequest) (AccessReview, error) {
	_, err := srv.db.Exec(ctx, `
		UPDATE access_reviews SET name=$3, description=$4, due_date=$5
		WHERE id=$1 AND tenant_id=$2`, id, tenantID, req.Name, req.Description, req.DueDate)
	if err != nil {
		return AccessReview{}, err
	}
	return srv.getAccessReview(ctx, id, tenantID)
}

func (srv *server) deleteAccessReview(ctx context.Context, id, tenantID string) error {
	tag, err := srv.db.Exec(ctx, `DELETE FROM access_reviews WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (srv *server) completeAccessReview(ctx context.Context, id, tenantID, userID string) (AccessReview, error) {
	_, err := srv.db.Exec(ctx, `
		UPDATE access_reviews SET status='completed', completed_at=NOW()
		WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return AccessReview{}, err
	}
	rv, err := srv.getAccessReview(ctx, id, tenantID)
	if err != nil {
		return rv, err
	}
	// Auto-link to evidence as SOC2 CC6.1 / ISO A.9.2
	go srv.autoLinkReviewEvidence(context.Background(), tenantID, userID, rv)
	return rv, nil
}

// ── DB helpers — review items ─────────────────────────────────────────────────

func (srv *server) listReviewItems(ctx context.Context, reviewID, tenantID string) ([]AccessReviewItem, error) {
	// Verify review belongs to tenant
	var count int
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM access_reviews WHERE id=$1 AND tenant_id=$2`, reviewID, tenantID).Scan(&count)
	if count == 0 {
		return nil, fmt.Errorf("not found")
	}
	rows, err := srv.db.Query(ctx, `
		SELECT i.id, i.review_id, i.subject_name, i.subject_email, i.subject_role,
		       i.access_level, i.last_active_at, i.decision,
		       i.decided_by_user_id, COALESCE(u.email,'') as decided_by_email,
		       i.decided_at, i.notes
		FROM access_review_items i
		LEFT JOIN users u ON u.id = i.decided_by_user_id
		WHERE i.review_id = $1
		ORDER BY i.subject_name`, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccessReviewItem
	for rows.Next() {
		var it AccessReviewItem
		if err := rows.Scan(
			&it.ID, &it.ReviewID, &it.SubjectName, &it.SubjectEmail, &it.SubjectRole,
			&it.AccessLevel, &it.LastActiveAt, &it.Decision,
			&it.DecidedByUserID, &it.DecidedByEmail, &it.DecidedAt, &it.Notes,
		); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (srv *server) upsertReviewItem(ctx context.Context, reviewID string, item AccessReviewItem) (AccessReviewItem, error) {
	var id string
	err := srv.db.QueryRow(ctx, `
		INSERT INTO access_review_items
		  (review_id, subject_name, subject_email, subject_role, access_level, last_active_at, decision, notes)
		VALUES($1,$2,$3,$4,$5,$6,'pending','')
		ON CONFLICT(review_id, subject_email) DO UPDATE
		  SET subject_name=$2, subject_role=$4, access_level=$5, last_active_at=$6
		RETURNING id`,
		reviewID, item.SubjectName, item.SubjectEmail, item.SubjectRole,
		item.AccessLevel, item.LastActiveAt,
	).Scan(&id)
	if err != nil {
		return AccessReviewItem{}, err
	}
	item.ID = id
	item.ReviewID = reviewID
	return item, nil
}

func (srv *server) updateReviewItem(ctx context.Context, itemID, reviewID, tenantID, userID, decision, notes string) (AccessReviewItem, error) {
	// Verify item belongs to review which belongs to tenant
	var count int
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM access_review_items i
		JOIN access_reviews r ON r.id=i.review_id
		WHERE i.id=$1 AND i.review_id=$2 AND r.tenant_id=$3`,
		itemID, reviewID, tenantID).Scan(&count)
	if count == 0 {
		return AccessReviewItem{}, fmt.Errorf("not found")
	}

	validDecisions := map[string]bool{"pending": true, "approved": true, "revoked": true, "needs_followup": true}
	if !validDecisions[decision] {
		return AccessReviewItem{}, fmt.Errorf("invalid decision")
	}

	var decidedByID *string
	var decidedAt *time.Time
	if decision != "pending" {
		decidedByID = &userID
		now := time.Now().UTC()
		decidedAt = &now
	}

	var it AccessReviewItem
	err := srv.db.QueryRow(ctx, `
		UPDATE access_review_items
		SET decision=$3, decided_by_user_id=$4, decided_at=$5, notes=$6
		WHERE id=$1 AND review_id=$2
		RETURNING id, review_id, subject_name, subject_email, subject_role,
		          access_level, last_active_at, decision, decided_by_user_id, decided_at, notes`,
		itemID, reviewID, decision, decidedByID, decidedAt, notes,
	).Scan(
		&it.ID, &it.ReviewID, &it.SubjectName, &it.SubjectEmail, &it.SubjectRole,
		&it.AccessLevel, &it.LastActiveAt, &it.Decision,
		&it.DecidedByUserID, &it.DecidedAt, &it.Notes,
	)
	return it, err
}

// ── Import helpers ────────────────────────────────────────────────────────────

func (srv *server) importDOTeam(ctx context.Context, reviewID, tenantID, connectionID string) (int, error) {
	conn, err := srv.getConnection(ctx, connectionID)
	if err != nil {
		return 0, fmt.Errorf("connection not found")
	}
	if conn.TenantID != tenantID {
		return 0, fmt.Errorf("connection not found")
	}

	client := doapi.New(conn.DOToken)
	imported := 0

	// 1. Try to get team members via /v2/teams
	if teams, err := client.GetList("/v2/teams", "teams"); err == nil && len(teams) > 0 {
		if first, ok := teams[0].(map[string]interface{}); ok {
			teamID := strField(first, "id")
			if teamID != "" {
				if members, err := client.GetList("/v2/teams/"+teamID+"/members", "members"); err == nil {
					for _, m := range members {
						mem, ok := m.(map[string]interface{})
						if !ok {
							continue
						}
						email := strField(mem, "email")
						name := strField(mem, "username")
						if name == "" {
							name = email
						}
						role := strField(mem, "role")
						if role == "" {
							role = "member"
						}
						_, _ = srv.upsertReviewItem(ctx, reviewID, AccessReviewItem{
							SubjectName:  name,
							SubjectEmail: email,
							SubjectRole:  role,
							AccessLevel:  "Team Member",
						})
						imported++
					}
				}
			}
		}
	}

	// 2. Always include the account owner
	if account, err := client.GetObject("/v2/account", "account"); err == nil {
		if acc, ok := account.(map[string]interface{}); ok {
			email := strField(acc, "email")
			name := strField(acc, "name")
			if name == "" {
				name = email
			}
			if email != "" {
				_, _ = srv.upsertReviewItem(ctx, reviewID, AccessReviewItem{
					SubjectName:  name,
					SubjectEmail: email,
					SubjectRole:  "Owner",
					AccessLevel:  "Full Access",
				})
				imported++
			}
		}
	}

	if imported == 0 {
		return 0, fmt.Errorf("no team members found — ensure the DO token has team read permissions")
	}
	return imported, nil
}

func importGitHubOrg(reviewID, org, token string, upsert func(item AccessReviewItem) error) (int, error) {
	if org == "" {
		return 0, fmt.Errorf("org is required")
	}
	url := fmt.Sprintf("https://api.github.com/orgs/%s/members?per_page=100", org)
	client := &http.Client{Timeout: 30 * time.Second}

	imported := 0
	for url != "" {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return imported, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "infra-audit/0.1")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			return imported, err
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return imported, fmt.Errorf("GitHub API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var members []map[string]interface{}
		if err := json.Unmarshal(body, &members); err != nil {
			return imported, err
		}

		for _, m := range members {
			login, _ := m["login"].(string)
			htmlURL, _ := m["html_url"].(string)
			_ = htmlURL
			role := "member"
			if r, ok := m["role"].(string); ok {
				role = r
			}
			if login == "" {
				continue
			}
			email := login + "@github.com" // GitHub org API doesn't return emails without extra scope
			if err := upsert(AccessReviewItem{
				SubjectName:  login,
				SubjectEmail: email,
				SubjectRole:  role,
				AccessLevel:  "Org Member",
			}); err != nil {
				log.Printf("importGitHubOrg: upsert %s: %v", login, err)
			}
			imported++
		}

		// Handle pagination via Link header
		url = ""
		if link := resp.Header.Get("Link"); link != "" {
			for _, part := range strings.Split(link, ",") {
				part = strings.TrimSpace(part)
				if strings.Contains(part, `rel="next"`) {
					start := strings.Index(part, "<")
					end := strings.Index(part, ">")
					if start >= 0 && end > start {
						url = part[start+1 : end]
					}
				}
			}
		}
	}
	return imported, nil
}

// ── Evidence auto-link ────────────────────────────────────────────────────────

func (srv *server) autoLinkReviewEvidence(ctx context.Context, tenantID, userID string, rv AccessReview) {
	// Create an evidence item for the completed review
	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	description := fmt.Sprintf("Access Review completed: %s. %d of %d items reviewed.",
		rv.Name, rv.ReviewedCount, rv.ItemCount)
	content := fmt.Sprintf(
		`{"review_id":"%s","review_name":"%s","completed_at":"%s","item_count":%d,"reviewed_count":%d}`,
		rv.ID, rv.Name, time.Now().UTC().Format(time.RFC3339), rv.ItemCount, rv.ReviewedCount)

	var evidenceID string
	err := srv.db.QueryRow(ctx, `
		INSERT INTO evidence_items(tenant_id, user_id, source, evidence_type, name, description,
		                           content_type, size, data, expires_at)
		VALUES($1,$2,'manual','policy',$3,$4,'application/json',$5,$6,$7)
		RETURNING id`,
		tenantID, userID,
		"Access Review: "+rv.Name,
		description,
		len(content), []byte(content),
		expiresAt,
	).Scan(&evidenceID)
	if err != nil {
		log.Printf("autoLinkReviewEvidence: insert evidence: %v", err)
		return
	}

	// Map to SOC2 CC6.1 and ISO A.9.2
	mappings := []struct{ fw, ctrl string }{
		{"soc2", "CC6.1"},
		{"soc2", "CC6.2"},
		{"soc2", "CC6.3"},
		{"iso27001", "A.9.2"},
	}
	for _, m := range mappings {
		_, _ = srv.db.Exec(ctx, `
			INSERT INTO evidence_mappings(evidence_id, framework_slug, ctrl_id)
			VALUES($1,$2,$3) ON CONFLICT DO NOTHING`,
			evidenceID, m.fw, m.ctrl)
	}
	log.Printf("autoLinkReviewEvidence: created evidence %s for review %s", evidenceID, rv.ID)
}

// ── Access review stats for dashboard ────────────────────────────────────────

type AccessReviewStats struct {
	Total        int `json:"total"`
	InProgress   int `json:"in_progress"`
	Overdue      int `json:"overdue"`
	Completed    int `json:"completed"`
	DueThisMonth int `json:"due_this_month"`
}

func (srv *server) getAccessReviewStats(ctx context.Context, tenantID string) (AccessReviewStats, error) {
	var s AccessReviewStats
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM access_reviews WHERE tenant_id=$1`, tenantID).Scan(&s.Total)
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM access_reviews WHERE tenant_id=$1 AND status='in_progress'`, tenantID).Scan(&s.InProgress)
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM access_reviews WHERE tenant_id=$1 AND status='completed'`, tenantID).Scan(&s.Completed)
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM access_reviews
		WHERE tenant_id=$1 AND status='in_progress' AND due_date < CURRENT_DATE`, tenantID).Scan(&s.Overdue)
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM access_reviews
		WHERE tenant_id=$1 AND due_date >= CURRENT_DATE AND due_date < DATE_TRUNC('month',CURRENT_DATE) + INTERVAL '1 month'`, tenantID).Scan(&s.DueThisMonth)
	return s, nil
}

// ── Request types ─────────────────────────────────────────────────────────────

type createAccessReviewRequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	ReviewType   string  `json:"review_type"`
	ConnectionID *string `json:"connection_id"`
	DueDate      *string `json:"due_date"`
	// GitHub import fields
	GitHubOrg   string `json:"github_org"`
	GitHubToken string `json:"github_token"`
}

type updateAccessReviewRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	DueDate     *string `json:"due_date"`
}

type updateReviewItemRequest struct {
	Decision string `json:"decision"`
	Notes    string `json:"notes"`
}

type importDORequest struct {
	ConnectionID string `json:"connection_id"`
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func (srv *server) handleListAccessReviews(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	reviews, err := srv.listAccessReviews(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	if reviews == nil {
		reviews = []AccessReview{}
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (srv *server) handleCreateAccessReview(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	userID := r.Context().Value(ctxUserID).(string)

	var req createAccessReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.ReviewType == "" {
		req.ReviewType = "manual"
	}

	review, err := srv.createAccessReview(r.Context(), tenantID, userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create failed: "+err.Error())
		return
	}

	// Auto-import based on review type
	switch req.ReviewType {
	case "do_team":
		if req.ConnectionID != nil && *req.ConnectionID != "" {
			n, err := srv.importDOTeam(r.Context(), review.ID, tenantID, *req.ConnectionID)
			if err != nil {
				log.Printf("do_team import: %v", err)
			} else {
				log.Printf("do_team import: %d members imported", n)
			}
		}
	case "github_org":
		if req.GitHubOrg != "" {
			upsertFn := func(item AccessReviewItem) error {
				_, err := srv.upsertReviewItem(r.Context(), review.ID, item)
				return err
			}
			n, err := importGitHubOrg(review.ID, req.GitHubOrg, req.GitHubToken, upsertFn)
			if err != nil {
				log.Printf("github_org import: %v", err)
			} else {
				log.Printf("github_org import: %d members imported", n)
			}
		}
	}

	// Re-fetch to get updated item count
	review, _ = srv.getAccessReview(r.Context(), review.ID, tenantID)
	writeJSON(w, http.StatusCreated, review)
}

func (srv *server) handleUpdateAccessReview(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	var req updateAccessReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	review, err := srv.updateAccessReview(r.Context(), id, tenantID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (srv *server) handleDeleteAccessReview(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")
	if err := srv.deleteAccessReview(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleGetReviewItems(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	reviewID := chi.URLParam(r, "id")
	items, err := srv.listReviewItems(r.Context(), reviewID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "review not found")
		return
	}
	if items == nil {
		items = []AccessReviewItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (srv *server) handleUpdateReviewItem(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	userID := r.Context().Value(ctxUserID).(string)
	reviewID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemId")

	var req updateReviewItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	item, err := srv.updateReviewItem(r.Context(), itemID, reviewID, tenantID, userID, req.Decision, req.Notes)
	if err != nil {
		if err.Error() == "not found" {
			writeError(w, http.StatusNotFound, "item not found")
		} else if err.Error() == "invalid decision" {
			writeError(w, http.StatusBadRequest, "invalid decision")
		} else {
			writeError(w, http.StatusInternalServerError, "update failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (srv *server) handleCompleteAccessReview(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	userID := r.Context().Value(ctxUserID).(string)
	id := chi.URLParam(r, "id")

	review, err := srv.completeAccessReview(r.Context(), id, tenantID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "complete failed")
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (srv *server) handleImportDOTeam(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	reviewID := chi.URLParam(r, "id")

	var req importDORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.ConnectionID == "" {
		writeError(w, http.StatusBadRequest, "connection_id required")
		return
	}

	n, err := srv.importDOTeam(r.Context(), reviewID, tenantID, req.ConnectionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	review, _ := srv.getAccessReview(r.Context(), reviewID, tenantID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported": n,
		"review":   review,
	})
}

func (srv *server) handleAddReviewItem(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	reviewID := chi.URLParam(r, "id")

	// Verify review ownership
	if _, err := srv.getAccessReview(r.Context(), reviewID, tenantID); err != nil {
		writeError(w, http.StatusNotFound, "review not found")
		return
	}

	var item AccessReviewItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if item.SubjectName == "" && item.SubjectEmail == "" {
		writeError(w, http.StatusBadRequest, "subject_name or subject_email required")
		return
	}
	if item.SubjectEmail == "" {
		item.SubjectEmail = strings.ToLower(strings.ReplaceAll(item.SubjectName, " ", ".")) + "@manual"
	}

	result, err := srv.upsertReviewItem(r.Context(), reviewID, item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "add failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (srv *server) handleGetAccessReviewStats(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	stats, err := srv.getAccessReviewStats(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
