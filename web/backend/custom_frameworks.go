package main

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ── Custom compliance framework types ─────────────────────────────────────────

type CustomFramework struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Controls    []CustomControl `json:"controls,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CustomControl struct {
	ID          string    `json:"id"`
	FrameworkID string    `json:"framework_id"`
	TenantID    string    `json:"tenant_id"`
	CtrlID      string    `json:"ctrl_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9-]`)

func toSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (srv *server) handleListCustomFrameworks(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	rows, err := srv.db.Query(r.Context(), `
		SELECT id, tenant_id, name, slug, version, description, created_at, updated_at
		FROM custom_frameworks WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()
	var out []CustomFramework
	for rows.Next() {
		var f CustomFramework
		if err := rows.Scan(&f.ID, &f.TenantID, &f.Name, &f.Slug, &f.Version, &f.Description, &f.CreatedAt, &f.UpdatedAt); err != nil {
			continue
		}
		out = append(out, f)
	}
	if out == nil {
		out = []CustomFramework{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (srv *server) handleCreateCustomFramework(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Slug == "" {
		req.Slug = toSlug(req.Name)
	}
	if req.Version == "" {
		req.Version = "1.0"
	}
	var f CustomFramework
	err := srv.db.QueryRow(r.Context(), `
		INSERT INTO custom_frameworks(tenant_id, name, slug, version, description)
		VALUES($1,$2,$3,$4,$5)
		RETURNING id, tenant_id, name, slug, version, description, created_at, updated_at`,
		tenantID, req.Name, req.Slug, req.Version, req.Description,
	).Scan(&f.ID, &f.TenantID, &f.Name, &f.Slug, &f.Version, &f.Description, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusConflict, "framework already exists or db error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (srv *server) handleGetCustomFramework(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")
	var f CustomFramework
	err := srv.db.QueryRow(r.Context(), `
		SELECT id, tenant_id, name, slug, version, description, created_at, updated_at
		FROM custom_frameworks WHERE id=$1 AND tenant_id=$2`, id, tenantID,
	).Scan(&f.ID, &f.TenantID, &f.Name, &f.Slug, &f.Version, &f.Description, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	// Load controls
	f.Controls, _ = srv.listCustomControls(r.Context(), f.ID, tenantID)
	writeJSON(w, http.StatusOK, f)
}

func (srv *server) handleUpdateCustomFramework(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")
	var req struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	var f CustomFramework
	err := srv.db.QueryRow(r.Context(), `
		UPDATE custom_frameworks SET name=$3, version=$4, description=$5, updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2
		RETURNING id, tenant_id, name, slug, version, description, created_at, updated_at`,
		id, tenantID, req.Name, req.Version, req.Description,
	).Scan(&f.ID, &f.TenantID, &f.Name, &f.Slug, &f.Version, &f.Description, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (srv *server) handleDeleteCustomFramework(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")
	tag, err := srv.db.Exec(r.Context(), `DELETE FROM custom_frameworks WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Controls ──────────────────────────────────────────────────────────────────

func (srv *server) listCustomControls(ctx context.Context, frameworkID, tenantID string) ([]CustomControl, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT id, framework_id, tenant_id, ctrl_id, name, description, category, created_at
		FROM custom_controls WHERE framework_id=$1 AND tenant_id=$2 ORDER BY ctrl_id`, frameworkID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomControl
	for rows.Next() {
		var c CustomControl
		if err := rows.Scan(&c.ID, &c.FrameworkID, &c.TenantID, &c.CtrlID, &c.Name, &c.Description, &c.Category, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (srv *server) handleListCustomControls(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	frameworkID := chi.URLParam(r, "frameworkId")
	controls, err := srv.listCustomControls(r.Context(), frameworkID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if controls == nil {
		controls = []CustomControl{}
	}
	writeJSON(w, http.StatusOK, controls)
}

func (srv *server) handleCreateCustomControl(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	frameworkID := chi.URLParam(r, "frameworkId")

	// Verify framework belongs to tenant
	var count int
	if err := srv.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM custom_frameworks WHERE id=$1 AND tenant_id=$2`, frameworkID, tenantID).Scan(&count); err != nil || count == 0 {
		writeError(w, http.StatusNotFound, "framework not found")
		return
	}

	var req struct {
		CtrlID      string `json:"ctrl_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CtrlID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "ctrl_id and name are required")
		return
	}

	var c CustomControl
	err := srv.db.QueryRow(r.Context(), `
		INSERT INTO custom_controls(framework_id, tenant_id, ctrl_id, name, description, category)
		VALUES($1,$2,$3,$4,$5,$6)
		RETURNING id, framework_id, tenant_id, ctrl_id, name, description, category, created_at`,
		frameworkID, tenantID, req.CtrlID, req.Name, req.Description, req.Category,
	).Scan(&c.ID, &c.FrameworkID, &c.TenantID, &c.CtrlID, &c.Name, &c.Description, &c.Category, &c.CreatedAt)
	if err != nil {
		writeError(w, http.StatusConflict, "control already exists or db error")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (srv *server) handleUpdateCustomControl(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	frameworkID := chi.URLParam(r, "frameworkId")
	controlID := chi.URLParam(r, "controlId")

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	var c CustomControl
	err := srv.db.QueryRow(r.Context(), `
		UPDATE custom_controls SET name=$3, description=$4, category=$5
		WHERE id=$1 AND framework_id=$2 AND tenant_id=$6
		RETURNING id, framework_id, tenant_id, ctrl_id, name, description, category, created_at`,
		controlID, frameworkID, req.Name, req.Description, req.Category, tenantID,
	).Scan(&c.ID, &c.FrameworkID, &c.TenantID, &c.CtrlID, &c.Name, &c.Description, &c.Category, &c.CreatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (srv *server) handleDeleteCustomControl(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	frameworkID := chi.URLParam(r, "frameworkId")
	controlID := chi.URLParam(r, "controlId")
	tag, err := srv.db.Exec(r.Context(),
		`DELETE FROM custom_controls WHERE id=$1 AND framework_id=$2 AND tenant_id=$3`,
		controlID, frameworkID, tenantID)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Bulk import controls (CSV-like) ──────────────────────────────────────────

func (srv *server) handleImportCustomControls(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	frameworkID := chi.URLParam(r, "frameworkId")

	var count int
	if err := srv.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM custom_frameworks WHERE id=$1 AND tenant_id=$2`, frameworkID, tenantID).Scan(&count); err != nil || count == 0 {
		writeError(w, http.StatusNotFound, "framework not found")
		return
	}

	var req struct {
		Controls []struct {
			CtrlID      string `json:"ctrl_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Category    string `json:"category"`
		} `json:"controls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Controls) == 0 {
		writeError(w, http.StatusBadRequest, "controls array required")
		return
	}

	imported := 0
	for _, c := range req.Controls {
		if c.CtrlID == "" || c.Name == "" {
			continue
		}
		_, err := srv.db.Exec(r.Context(), `
			INSERT INTO custom_controls(framework_id, tenant_id, ctrl_id, name, description, category)
			VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT(framework_id, ctrl_id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, category=EXCLUDED.category`,
			frameworkID, tenantID, c.CtrlID, c.Name, c.Description, c.Category)
		if err == nil {
			imported++
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported": imported,
		"total":    len(req.Controls),
	})
}

