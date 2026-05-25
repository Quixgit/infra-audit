package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
)

const maxEvidenceUpload = 10 << 20 // 10 MB

// ── Types ─────────────────────────────────────────────────────────────────────

type EvidenceItem struct {
	ID           string            `json:"id"`
	UserID       string            `json:"-"`
	TenantID     string            `json:"tenant_id,omitempty"`
	JobID        *string           `json:"job_id,omitempty"`
	JobName      string            `json:"job_name,omitempty"`
	Source       string            `json:"source"`
	EvidenceType string            `json:"evidence_type"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	ContentType  string            `json:"content_type"`
	Size         int64             `json:"size"`
	FilePath     string            `json:"-"`
	ExpiresAt    time.Time         `json:"expires_at"`
	CreatedAt    time.Time         `json:"created_at"`
	Status       string            `json:"status"`
	Mappings     []EvidenceMapping `json:"mappings"`
}

type EvidenceMapping struct {
	ID            string    `json:"id"`
	EvidenceID    string    `json:"evidence_id"`
	FrameworkSlug string    `json:"framework_slug"`
	CtrlID        string    `json:"ctrl_id"`
	CreatedAt     time.Time `json:"created_at"`
}

func evidenceStatus(e EvidenceItem) string {
	now := time.Now()
	if now.After(e.ExpiresAt) {
		return "expired"
	}
	// Stale = expires within next 6 months
	if now.Add(6 * 30 * 24 * time.Hour).After(e.ExpiresAt) {
		return "stale"
	}
	return "fresh"
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (srv *server) listEvidenceItems(ctx context.Context, tenantID string) ([]EvidenceItem, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT e.id, e.user_id, e.tenant_id, e.job_id,
		       COALESCE(c.name || ' / ' || to_char(j.started_at,'YYYY-MM-DD'), '') as job_name,
		       e.source, e.evidence_type, e.name, e.description,
		       e.content_type, e.size, e.file_path, e.expires_at, e.created_at
		FROM evidence_items e
		LEFT JOIN audit_jobs j ON j.id = e.job_id
		LEFT JOIN connections c ON c.id = j.connection_id
		WHERE e.tenant_id = $1
		ORDER BY e.created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []EvidenceItem
	for rows.Next() {
		var it EvidenceItem
		var jobID *string
		if err := rows.Scan(&it.ID, &it.UserID, &it.TenantID, &jobID, &it.JobName,
			&it.Source, &it.EvidenceType, &it.Name, &it.Description,
			&it.ContentType, &it.Size, &it.FilePath, &it.ExpiresAt, &it.CreatedAt,
		); err != nil {
			return nil, err
		}
		it.JobID = jobID
		it.Status = evidenceStatus(it)
		items = append(items, it)
	}
	return items, rows.Err()
}

func (srv *server) getEvidenceItem(ctx context.Context, id, userID string) (EvidenceItem, error) {
	var it EvidenceItem
	var jobID *string
	err := srv.db.QueryRow(ctx, `
		SELECT id, user_id, job_id, source, evidence_type, name, description,
		       content_type, size, file_path, expires_at, created_at
		FROM evidence_items WHERE id=$1 AND tenant_id=$2`, id, userID,
	).Scan(&it.ID, &it.UserID, &jobID, &it.Source, &it.EvidenceType,
		&it.Name, &it.Description, &it.ContentType, &it.Size,
		&it.FilePath, &it.ExpiresAt, &it.CreatedAt)
	if err != nil {
		return it, err
	}
	it.JobID = jobID
	it.Status = evidenceStatus(it)
	return it, nil
}

func (srv *server) getEvidenceData(ctx context.Context, id, userID string) ([]byte, EvidenceItem, error) {
	var it EvidenceItem
	var data []byte
	var jobID *string
	err := srv.db.QueryRow(ctx, `
		SELECT id, user_id, job_id, source, evidence_type, name, description,
		       content_type, size, data, file_path, expires_at, created_at
		FROM evidence_items WHERE id=$1 AND tenant_id=$2`, id, userID,
	).Scan(&it.ID, &it.UserID, &jobID, &it.Source, &it.EvidenceType,
		&it.Name, &it.Description, &it.ContentType, &it.Size,
		&data, &it.FilePath, &it.ExpiresAt, &it.CreatedAt)
	it.JobID = jobID
	return data, it, err
}

func (srv *server) insertEvidenceItem(ctx context.Context, userID string, jobID *string,
	source, evType, name, description, contentType string, size int64,
	data []byte, filePath string, expiresAt time.Time,
) (EvidenceItem, error) {
	var it EvidenceItem
	err := srv.db.QueryRow(ctx, `
		INSERT INTO evidence_items(user_id,tenant_id,job_id,source,evidence_type,name,description,
		                           content_type,size,data,file_path,expires_at)
		VALUES($1,COALESCE((SELECT tenant_id FROM audit_jobs WHERE id=$2),(SELECT tenant_id FROM tenant_members WHERE user_id=$1 LIMIT 1)),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at`,
		userID, jobID, source, evType, name, description,
		contentType, size, data, filePath, expiresAt,
	).Scan(&it.ID, &it.CreatedAt)
	if err != nil {
		return it, err
	}
	it.UserID = userID
	it.JobID = jobID
	it.Source = source
	it.EvidenceType = evType
	it.Name = name
	it.Description = description
	it.ContentType = contentType
	it.Size = size
	it.FilePath = filePath
	it.ExpiresAt = expiresAt
	it.Status = evidenceStatus(it)
	return it, nil
}

func (srv *server) deleteEvidenceItem(ctx context.Context, id, userID string) error {
	tag, err := srv.db.Exec(ctx,
		`DELETE FROM evidence_items WHERE id=$1 AND tenant_id=$2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (srv *server) listEvidenceMappings(ctx context.Context, evidenceID string) ([]EvidenceMapping, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT id, evidence_id, framework_slug, ctrl_id, created_at
		 FROM evidence_mappings WHERE evidence_id=$1 ORDER BY framework_slug, ctrl_id`, evidenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EvidenceMapping
	for rows.Next() {
		var m EvidenceMapping
		if err := rows.Scan(&m.ID, &m.EvidenceID, &m.FrameworkSlug, &m.CtrlID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (srv *server) setEvidenceMappings(ctx context.Context, evidenceID string, mappings []struct {
	FrameworkSlug string `json:"framework_slug"`
	CtrlID        string `json:"ctrl_id"`
}) error {
	tx, err := srv.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM evidence_mappings WHERE evidence_id=$1`, evidenceID); err != nil {
		return err
	}
	for _, m := range mappings {
		if m.FrameworkSlug == "" || m.CtrlID == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO evidence_mappings(evidence_id,framework_slug,ctrl_id) VALUES($1,$2,$3)
			 ON CONFLICT DO NOTHING`, evidenceID, m.FrameworkSlug, m.CtrlID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (srv *server) listEvidenceByControl(ctx context.Context, userID, frameworkSlug, ctrlID string) ([]EvidenceItem, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT e.id, e.job_id, e.source, e.evidence_type, e.name, e.description,
		       e.content_type, e.size, e.file_path, e.expires_at, e.created_at
		FROM evidence_items e
		JOIN evidence_mappings m ON m.evidence_id = e.id
		WHERE e.tenant_id=$1 AND m.framework_slug=$2 AND m.ctrl_id=$3
		ORDER BY e.created_at DESC`, userID, frameworkSlug, ctrlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []EvidenceItem
	for rows.Next() {
		var it EvidenceItem
		var jobID *string
		if err := rows.Scan(&it.ID, &jobID, &it.Source, &it.EvidenceType,
			&it.Name, &it.Description, &it.ContentType, &it.Size,
			&it.FilePath, &it.ExpiresAt, &it.CreatedAt); err != nil {
			return nil, err
		}
		it.UserID = userID
		it.JobID = jobID
		it.Status = evidenceStatus(it)
		items = append(items, it)
	}
	return items, rows.Err()
}

// countEvidenceByControls returns a map of "frameworkSlug/ctrlID" → count for all user evidence.
func (srv *server) countEvidenceByControls(ctx context.Context, userID string) (map[string]int, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT m.framework_slug, m.ctrl_id, COUNT(*) as cnt
		FROM evidence_mappings m
		JOIN evidence_items e ON e.id = m.evidence_id
		WHERE e.tenant_id=$1
		GROUP BY m.framework_slug, m.ctrl_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var fw, ctrl string
		var cnt int
		if err := rows.Scan(&fw, &ctrl, &cnt); err != nil {
			return nil, err
		}
		out[fw+"/"+ctrl] = cnt
	}
	return out, rows.Err()
}

// ── Auto-collect ──────────────────────────────────────────────────────────────

// autoCollectEvidence creates evidence items automatically when a job completes.
func (srv *server) autoCollectEvidence(ctx context.Context, jobID, userID, reportDir, htmlPath, docxPath, connType string) {
	jid := jobID
	expiry := time.Now().AddDate(1, 0, 0) // 12 months

	// do_inventory.json
	if connType == "do" {
		if data, err := os.ReadFile(filepath.Join(reportDir, "do_inventory.json")); err == nil {
			_, err = srv.insertEvidenceItem(ctx, userID, &jid, "auto", "inventory",
				"DigitalOcean Inventory (JSON)", "Auto-collected DigitalOcean resource inventory from audit scan",
				"application/json", int64(len(data)), data, "", expiry)
			if err != nil {
				log.Printf("autoCollectEvidence do_inventory: %v", err)
			}
		}
	}

	// findings.json
	if data, err := os.ReadFile(filepath.Join(reportDir, "findings.json")); err == nil {
		_, err = srv.insertEvidenceItem(ctx, userID, &jid, "auto", "findings",
			"Security Findings (JSON)", "Auto-collected findings from audit scan",
			"application/json", int64(len(data)), data, "", expiry)
		if err != nil {
			log.Printf("autoCollectEvidence findings: %v", err)
		}
	}

	// tf_findings.json (code jobs)
	if connType == "code" {
		if data, err := os.ReadFile(filepath.Join(reportDir, "tf_findings.json")); err == nil {
			_, err = srv.insertEvidenceItem(ctx, userID, &jid, "auto", "findings",
				"Terraform/IaC Findings (JSON)", "Auto-collected IaC findings from code audit",
				"application/json", int64(len(data)), data, "", expiry)
			if err != nil {
				log.Printf("autoCollectEvidence tf_findings: %v", err)
			}
		}
	}

	// HTML report (file reference, not copied to DB)
	if htmlPath != "" {
		if info, err := os.Stat(htmlPath); err == nil {
			_, err = srv.insertEvidenceItem(ctx, userID, &jid, "auto", "report",
				"Audit Report (HTML)", "Auto-collected HTML audit report",
				"text/html", info.Size(), nil, htmlPath, expiry)
			if err != nil {
				log.Printf("autoCollectEvidence html: %v", err)
			}
		}
	}

	// DOCX report (file reference)
	if docxPath != "" {
		if info, err := os.Stat(docxPath); err == nil {
			_, err = srv.insertEvidenceItem(ctx, userID, &jid, "auto", "report",
				"Audit Report (DOCX)", "Auto-collected DOCX audit report",
				"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				info.Size(), nil, docxPath, expiry)
			if err != nil {
				log.Printf("autoCollectEvidence docx: %v", err)
			}
		}
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (srv *server) handleListEvidence(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	items, err := srv.listEvidenceItems(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if items == nil {
		items = []EvidenceItem{}
	}

	// Attach mappings for each item
	for i := range items {
		mappings, _ := srv.listEvidenceMappings(r.Context(), items[i].ID)
		if mappings != nil {
			items[i].Mappings = mappings
		} else {
			items[i].Mappings = []EvidenceMapping{}
		}
	}

	writeJSON(w, http.StatusOK, items)
}

func (srv *server) handleUploadEvidence(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	r.Body = http.MaxBytesReader(w, r.Body, maxEvidenceUpload)
	if err := r.ParseMultipartForm(maxEvidenceUpload); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 10 MB)")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = "Unnamed evidence"
	}
	description := r.FormValue("description")
	evType := r.FormValue("evidence_type")
	if evType == "" {
		evType = "other"
	}

	expiresAt := time.Now().AddDate(1, 0, 0)
	if expStr := r.FormValue("expires_at"); expStr != "" {
		if t, err := time.Parse("2006-01-02", expStr); err == nil {
			expiresAt = t
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	ct := header.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		ct = "application/octet-stream"
	}
	if name == "Unnamed evidence" && header.Filename != "" {
		name = header.Filename
	}

	item, err := srv.insertEvidenceItem(r.Context(), userID, nil, "manual", evType,
		name, description, ct, int64(len(data)), data, "", expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	item.Mappings = []EvidenceMapping{}
	writeJSON(w, http.StatusCreated, item)
}

func (srv *server) handleDownloadEvidence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tenantID := r.Context().Value(ctxTenantID).(string)
	data, item, err := srv.getEvidenceData(r.Context(), id, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(item.Name)+`"`)
	w.Header().Set("Content-Type", item.ContentType)

	if len(data) > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.Write(data)
		return
	}

	if item.FilePath != "" {
		http.ServeFile(w, r, item.FilePath)
		return
	}

	writeError(w, http.StatusGone, "file no longer available")
}

func (srv *server) handleDeleteEvidence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := srv.deleteEvidenceItem(r.Context(), id, r.Context().Value(ctxTenantID).(string)); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleSetEvidenceMappings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify ownership
	tenantID := r.Context().Value(ctxTenantID).(string)
	if _, err := srv.getEvidenceItem(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req struct {
		Mappings []struct {
			FrameworkSlug string `json:"framework_slug"`
			CtrlID        string `json:"ctrl_id"`
		} `json:"mappings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := srv.setEvidenceMappings(r.Context(), id, req.Mappings); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	mappings, _ := srv.listEvidenceMappings(r.Context(), id)
	if mappings == nil {
		mappings = []EvidenceMapping{}
	}
	writeJSON(w, http.StatusOK, mappings)
}

func (srv *server) handleGetEvidenceByControl(w http.ResponseWriter, r *http.Request) {
	fw := chi.URLParam(r, "framework")
	ctrl := chi.URLParam(r, "ctrl_id")

	tenantID := r.Context().Value(ctxTenantID).(string)
	items, err := srv.listEvidenceByControl(r.Context(), tenantID, fw, ctrl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (srv *server) handleGetEvidenceCounts(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	counts, err := srv.countEvidenceByControls(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, counts)
}

// sanitizeFilename removes path separators from a filename.
func sanitizeFilename(name string) string {
	for _, ch := range []byte{'/', '\\', ':'} {
		for i := range []byte(name) {
			if name[i] == ch {
				b := []byte(name)
				b[i] = '_'
				name = string(b)
			}
		}
	}
	return name
}
