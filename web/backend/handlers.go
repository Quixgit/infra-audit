package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"infra-audit/internal/scanner"
)

func (srv *server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	user, err := srv.getUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user not found")
		return
	}
	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, user)
}

func (srv *server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)

	var req updateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := srv.updateUserSettings(r.Context(), userID, req); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	user, _ := srv.getUser(r.Context(), userID)
	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, user)
}

func (srv *server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	user, err := srv.getUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user not found")
		return
	}

	if err := checkPassword(user.PasswordHash, req.CurrentPassword); err != nil {
		writeError(w, http.StatusUnauthorized, "current password incorrect")
		return
	}

	hash, err := hashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash error")
		return
	}

	if err := srv.updateUserPassword(r.Context(), userID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleUpdateNotify(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)

	var req updateNotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := srv.updateUserNotify(r.Context(), userID, req.NotifyEmail); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	user, _ := srv.getUser(r.Context(), userID)
	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, user)
}

func (srv *server) handleUploadAsset(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)

	// Custom branding is a paid feature
	if !hasFeature(srv.getEffectiveClaims(r.Context(), userID), "custom_branding") {
		writeFeatureError(w, "custom_branding")
		return
	}

	assetType := chi.URLParam(r, "assetType")

	allowed := map[string]string{
		"logo":      "logo.png",
		"watermark": "watermark.png",
		"footer-bg": "footer-bg.png",
	}

	filename, ok := allowed[assetType]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown asset type")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field missing")
		return
	}
	defer file.Close()

	dir := filepath.Join(envOr("DATA_DIR", "/app/data"), "users", userID, "assets")
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir error")
		return
	}

	dst, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "write error")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, "write error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "filename": filename})
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (srv *server) handleGetDashboard(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	data, err := srv.getDashboardData(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// ── Connections ───────────────────────────────────────────────────────────────

func (srv *server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	conns, err := srv.listConnections(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	resp := make([]connectionResponse, len(conns))
	for i, c := range conns {
		resp[i] = connToResponse(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)

	var req createConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	isNetworkScan := req.ConnType == "ssl" || req.ConnType == "dns"
	if !isNetworkScan && req.ConnType != "code" && req.DOToken == "" {
		writeError(w, http.StatusBadRequest, "do_token is required for DigitalOcean connections")
		return
	}
	if isNetworkScan && req.Domains == "" {
		writeError(w, http.StatusBadRequest, "domains are required for SSL/DNS connections (comma-separated)")
		return
	}
	if req.ScopeMode == "" {
		req.ScopeMode = "project"
	}

	lic := srv.getEffectiveClaims(r.Context(), userID)
	if maxConns := effectiveMaxConnections(lic); maxConns >= 0 {
		used, _, _ := srv.getLicenseUsage(r.Context(), tenantID)
		if used >= maxConns {
			writeError(w, http.StatusForbidden, fmt.Sprintf("license limit: max %d connections", maxConns))
			return
		}
	}

	conn, err := srv.createConnection(r.Context(), tenantID, userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	go func() {
		u, _ := srv.getUser(r.Context(), userID)
		srv.logActivity(r.Context(), tenantID, userID, u.Email, "connection.created", "connection", conn.ID, clientIP(r))
	}()
	writeJSON(w, http.StatusCreated, connToResponse(conn))
}

func (srv *server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	var req createConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	conn, err := srv.updateConnection(r.Context(), id, tenantID, req)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, connToResponse(conn))
}

func (srv *server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	if err := srv.deleteConnection(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	conn, err := srv.getConnection(r.Context(), id)
	if err != nil || conn.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	projects, err := scanner.ListDigitalOceanProjects(conn.DOToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "DigitalOcean API error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"projects": projects,
	})
}

// ── Audit jobs ────────────────────────────────────────────────────────────────

func (srv *server) handleRunAudit(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)
	connectionID := chi.URLParam(r, "connectionId")

	conn, err := srv.getConnection(r.Context(), connectionID)
	if err != nil || conn.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	lic := srv.getEffectiveClaims(r.Context(), userID)

	// Feature gate for code audits
	if conn.ConnType == "code" && !hasFeature(lic, "code_audit") {
		writeFeatureError(w, "code_audit")
		return
	}

	if maxAudits := effectiveMaxAuditsMonth(lic); maxAudits >= 0 {
		_, used, _ := srv.getLicenseUsage(r.Context(), tenantID)
		if used >= maxAudits {
			writeError(w, http.StatusForbidden, fmt.Sprintf("license limit: max %d audits per month", maxAudits))
			return
		}
	}

	job, err := srv.createJob(r.Context(), connectionID, userID, tenantID, conn.ConnType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	connType := conn.ConnType
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("audit panic for job %s: %v", job.ID, rec)
				srv.updateJobFailed(r.Context(), job.ID, "internal error")
			}
		}()
		switch connType {
		case "code":
			srv.runCodeAudit(job.ID, connectionID, userID)
		case "ssl":
			srv.runSSLAudit(job.ID, connectionID, userID)
		case "dns":
			srv.runDNSAudit(job.ID, connectionID, userID)
		default:
			srv.runAudit(job.ID, connectionID, userID)
		}
	}()

	// Log activity (non-blocking)
	go func() {
		u, _ := srv.getUser(r.Context(), userID)
		srv.logActivity(r.Context(), tenantID, userID, u.Email, "audit.run", "connection", connectionID, clientIP(r))
	}()

	writeJSON(w, http.StatusAccepted, job)
}

func (srv *server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := fmt.Sscanf(l, "%d", &limit); v == 0 || err != nil {
			limit = 100
		}
	}
	jobs, err := srv.listJobs(r.Context(), tenantID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if jobs == nil {
		jobs = []AuditJob{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (srv *server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (srv *server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if err := srv.deleteJob(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}

	if job.HTMLPath != "" {
		dir := filepath.Dir(job.HTMLPath)
		_ = os.RemoveAll(dir)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleDownloadHTML(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if job.HTMLPath == "" {
		writeError(w, http.StatusNotFound, "report not ready")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="report.html"`)
	http.ServeFile(w, r, job.HTMLPath)
}

func (srv *server) handleDownloadDOCX(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if job.DOCXPath == "" {
		writeError(w, http.StatusNotFound, "report not ready")
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", `attachment; filename="report.docx"`)
	http.ServeFile(w, r, job.DOCXPath)
}

func (srv *server) handleGetFindings(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if job.HTMLPath == "" {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}

	findingsPath := filepath.Join(filepath.Dir(job.HTMLPath), "findings.json")
	data, err := os.ReadFile(findingsPath)
	if err != nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ── Schedules ─────────────────────────────────────────────────────────────────

func (srv *server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	schedules, err := srv.listSchedules(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if schedules == nil {
		schedules = []Schedule{}
	}
	writeJSON(w, http.StatusOK, schedules)
}

func (srv *server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)
	if !hasFeature(srv.getEffectiveClaims(r.Context(), userID), "scheduled_audits") {
		writeFeatureError(w, "scheduled_audits")
		return
	}

	var req createScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Interval != "daily" && req.Interval != "weekly" {
		req.Interval = "daily"
	}

	conn, err := srv.getConnection(r.Context(), req.ConnectionID)
	if err != nil || conn.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	s, err := srv.createSchedule(r.Context(), tenantID, userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (srv *server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	var req updateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Interval != "daily" && req.Interval != "weekly" {
		req.Interval = "daily"
	}

	s, err := srv.updateSchedule(r.Context(), id, tenantID, req)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (srv *server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	if err := srv.deleteSchedule(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Bulk run ─────────────────────────────────────────────────────────────────

func (srv *server) handleBulkRun(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)

	var req struct {
		ConnectionIDs []string `json:"connection_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.ConnectionIDs) == 0 {
		writeError(w, http.StatusBadRequest, "connection_ids required")
		return
	}

	var jobs []AuditJob
	for _, connID := range req.ConnectionIDs {
		conn, err := srv.getConnection(r.Context(), connID)
		if err != nil || conn.TenantID != tenantID {
			continue
		}
		job, err := srv.createJob(r.Context(), connID, userID, tenantID, conn.ConnType)
		if err != nil {
			continue
		}
		go func(jID, cID, uID, cType string) {
			defer func() { recover() }()
			switch cType {
			case "code":
				srv.runCodeAudit(jID, cID, uID)
			case "ssl":
				srv.runSSLAudit(jID, cID, uID)
			case "dns":
				srv.runDNSAudit(jID, cID, uID)
			default:
				srv.runAudit(jID, cID, uID)
			}
		}(job.ID, connID, userID, conn.ConnType)
		jobs = append(jobs, job)
	}

	if jobs == nil {
		jobs = []AuditJob{}
	}
	writeJSON(w, http.StatusAccepted, jobs)
}

// ── Connection history ────────────────────────────────────────────────────────

func (srv *server) handleGetConnectionHistory(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	conn, err := srv.getConnection(r.Context(), id)
	if err != nil || conn.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	jobs, err := srv.getConnectionHistory(r.Context(), id, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if jobs == nil {
		jobs = []AuditJob{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

// ── Compare ───────────────────────────────────────────────────────────────────

func (srv *server) handleCompareJob(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	prev, err := srv.getPreviousJob(r.Context(), id, job.ConnectionID, tenantID)
	if err != nil {
		writeJSON(w, http.StatusOK, CompareResult{
			NewFindings:   []map[string]interface{}{},
			FixedFindings: []map[string]interface{}{},
		})
		return
	}

	loadFindings := func(j AuditJob) []map[string]interface{} {
		if j.HTMLPath == "" {
			return nil
		}
		p := filepath.Join(filepath.Dir(j.HTMLPath), "findings.json")
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var out []map[string]interface{}
		_ = json.Unmarshal(b, &out)
		return out
	}

	key := func(f map[string]interface{}) string {
		return fmt.Sprintf("%v|%v|%v", f["title"], f["resource_name"], f["category"])
	}

	currFindings := loadFindings(job)
	prevFindings := loadFindings(prev)

	prevSet := map[string]bool{}
	for _, f := range prevFindings {
		prevSet[key(f)] = true
	}
	currSet := map[string]bool{}
	for _, f := range currFindings {
		currSet[key(f)] = true
	}

	var newF, fixedF []map[string]interface{}
	for _, f := range currFindings {
		if !prevSet[key(f)] {
			newF = append(newF, f)
		}
	}
	for _, f := range prevFindings {
		if !currSet[key(f)] {
			fixedF = append(fixedF, f)
		}
	}

	if newF == nil {
		newF = []map[string]interface{}{}
	}
	if fixedF == nil {
		fixedF = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, CompareResult{
		NewFindings:   newF,
		FixedFindings: fixedF,
		PrevJobID:     prev.ID,
	})
}

// ── Share links ───────────────────────────────────────────────────────────────

func (srv *server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)
	if !hasFeature(srv.getEffectiveClaims(r.Context(), userID), "share_links") {
		writeFeatureError(w, "share_links")
		return
	}
	id := chi.URLParam(r, "id")

	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if job.Status != "done" {
		writeError(w, http.StatusBadRequest, "job must be done")
		return
	}

	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}
	token := hex.EncodeToString(b)

	link, err := srv.createShareLink(r.Context(), id, token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (srv *server) handleGetShare(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	link, err := srv.getShareLinkByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "share link not found")
		return
	}

	job, err := srv.getJob(r.Context(), link.JobID)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	var findings interface{} = []struct{}{}
	if job.HTMLPath != "" {
		p := filepath.Join(filepath.Dir(job.HTMLPath), "findings.json")
		if b, err := os.ReadFile(p); err == nil {
			var f interface{}
			if json.Unmarshal(b, &f) == nil {
				findings = f
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"job":      job,
		"findings": findings,
	})
}

// ── API tokens ────────────────────────────────────────────────────────────────

func (srv *server) handleListAPITokens(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	tokens, err := srv.listAPITokens(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if tokens == nil {
		tokens = []APIToken{}
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (srv *server) handleCreateAPIToken(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	if !hasFeature(srv.getEffectiveClaims(r.Context(), userID), "api_tokens") {
		writeFeatureError(w, "api_tokens")
		return
	}

	var req createAPITokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}
	raw := hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(h[:])
	prefix := raw[:8]

	tenantID := r.Context().Value(ctxTenantID).(string)
	tok, err := srv.createAPIToken(r.Context(), tenantID, userID, req.Name, hash, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           tok.ID,
		"user_id":      tok.UserID,
		"name":         tok.Name,
		"token_prefix": tok.TokenPrefix,
		"created_at":   tok.CreatedAt,
		"token":        raw, // only returned once
	})
}

func (srv *server) handleDeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tenantID := r.Context().Value(ctxTenantID).(string)
	if err := srv.deleteAPIToken(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── License ───────────────────────────────────────────────────────────────────

func (srv *server) handleGetLicense(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	// Use display claims so the admin preview plan is reflected in the Settings UI
	claims := srv.getDisplayClaims(r.Context(), userID)

	tenantID := r.Context().Value(ctxTenantID).(string)
	usedConns, usedAudits, _ := srv.getLicenseUsage(r.Context(), tenantID)
	usedUsers, _ := srv.getUserCount(r.Context())

	var expiresAt *time.Time
	if claims != nil && claims.ExpiresAt != nil {
		t := claims.ExpiresAt.Time
		expiresAt = &t
	}

	writeJSON(w, http.StatusOK, LicenseInfo{
		Plan: effectivePlan(claims),
		IssuedTo: func() string {
			if claims != nil {
				return claims.IssuedTo
			}
			return ""
		}(),
		ExpiresAt:       expiresAt,
		MaxConnections:  effectiveMaxConnections(claims),
		MaxUsers:        effectiveMaxUsers(claims),
		MaxAuditsMonth:  effectiveMaxAuditsMonth(claims),
		Features:        effectiveFeatures(claims),
		UsedConnections: usedConns,
		UsedAuditsMonth: usedAudits,
		UsedUsers:       usedUsers,
	})
}

func (srv *server) handleActivateLicense(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		writeError(w, http.StatusBadRequest, "key required")
		return
	}
	key := trimKey(req.Key)

	claims, err := parseLicenseJWT(key)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid license key: "+strings.ReplaceAll(err.Error(), "\n", " "))
		return
	}

	if err := srv.setSetting(r.Context(), "license_key", key); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store license")
		return
	}
	invalidateLicenseCache()

	tenantID := r.Context().Value(ctxTenantID).(string)
	usedConns, usedAudits, _ := srv.getLicenseUsage(r.Context(), tenantID)
	usedUsers, _ := srv.getUserCount(r.Context())

	var expiresAt *time.Time
	if claims.ExpiresAt != nil {
		t := claims.ExpiresAt.Time
		expiresAt = &t
	}

	userID2 := r.Context().Value(ctxUserID).(string)
	go func() {
		u, _ := srv.getUser(r.Context(), userID2)
		srv.logActivity(r.Context(), tenantID, userID2, u.Email, "license.activated", "license", claims.Plan, clientIP(r))
	}()

	writeJSON(w, http.StatusOK, LicenseInfo{
		Plan:            claims.Plan,
		IssuedTo:        claims.IssuedTo,
		ExpiresAt:       expiresAt,
		MaxConnections:  claims.MaxConnections,
		MaxUsers:        claims.MaxUsers,
		MaxAuditsMonth:  claims.MaxAuditsMonth,
		Features:        effectiveFeatures(claims),
		UsedConnections: usedConns,
		UsedAuditsMonth: usedAudits,
		UsedUsers:       usedUsers,
	})
}

// ── Team ──────────────────────────────────────────────────────────────────────

func (srv *server) handleListTeam(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	members, err := srv.listTeamMembers(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if members == nil {
		members = []TeamMember{}
	}
	writeJSON(w, http.StatusOK, members)
}

func (srv *server) handleInviteTeamMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	lic := srv.getEffectiveClaims(r.Context(), userID)
	if !hasFeature(lic, "team") {
		writeFeatureError(w, "team")
		return
	}
	if maxUsers := effectiveMaxUsers(lic); maxUsers >= 0 {
		count, _ := srv.getUserCount(r.Context())
		if count >= maxUsers {
			writeError(w, http.StatusForbidden, fmt.Sprintf("license limit: max %d users", maxUsers))
			return
		}
	}
	var req inviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}
	if req.Role != "admin" && req.Role != "viewer" {
		req.Role = "viewer"
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash error")
		return
	}

	tenantID := r.Context().Value(ctxTenantID).(string)
	member, err := srv.createTeamMember(r.Context(), tenantID, req.Email, hash, req.Role)
	if err != nil {
		writeError(w, http.StatusConflict, "user already exists or db error")
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

func (srv *server) handleDeleteTeamMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	id := chi.URLParam(r, "id")

	if id == userID {
		writeError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	tenantID := r.Context().Value(ctxTenantID).(string)
	if err := srv.deleteTeamMember(r.Context(), tenantID, id); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Code connection test routes ────────────────────────────────────────────────

func (srv *server) handleTestGit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"repo_url"`
		Token  string `json:"repo_token"`
		Branch string `json:"repo_branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeError(w, http.StatusBadRequest, "repo_url required")
		return
	}

	authURL, err := buildAuthURL(req.URL, req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid URL: "+err.Error())
		return
	}

	cmd := exec.CommandContext(r.Context(), "git", "ls-remote", "--heads", authURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": "cannot access repository: check URL and token"})
		return
	}

	if req.Branch != "" {
		if !strings.Contains(string(out), "refs/heads/"+req.Branch) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": "branch not found: " + req.Branch})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Repository accessible"})
}

func (srv *server) handleTestLocal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"repo_local_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "repo_local_path required")
		return
	}
	if err := validateLocalPath(req.Path); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Path is valid and accessible"})
}

// ── Code / TF findings endpoints ──────────────────────────────────────────────

func (srv *server) handleGetCodeFindings(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")
	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	serveJSONFile(w, job, "findings.json")
}

func (srv *server) handleGetTFFindings(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")
	job, err := srv.getJob(r.Context(), id)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	serveJSONFile(w, job, "tf_findings.json")
}

// ── Admin preview plan ────────────────────────────────────────────────────────

func (srv *server) handleSetAdminPreviewPlan(w http.ResponseWriter, r *http.Request) {
	// Only admins/owners can change the preview plan
	role, _ := r.Context().Value(ctxUserRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}

	var req struct {
		Plan string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Plan != "community" && req.Plan != "starter" && req.Plan != "professional" && req.Plan != "business" {
		writeError(w, http.StatusBadRequest, "plan must be community, starter, professional, or business")
		return
	}
	// Store empty string for community (= no override, use real license)
	val := req.Plan
	if val == "community" {
		val = ""
	}
	if err := srv.setSetting(r.Context(), "admin_preview_plan", val); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save setting")
		return
	}
	invalidateLicenseCache()
	writeJSON(w, http.StatusOK, map[string]string{"plan": req.Plan})
}

// ── Module toggles ────────────────────────────────────────────────────────────

var moduleKeys = []string{
	// nav modules
	"cloud_audits", "code_iac", "findings", "remediation", "monitoring",
	"compliance", "evidence", "policies", "access_reviews",
	"reports", "audit_types", "jobs",
	// scanners
	"scanner_do", "scanner_aws", "scanner_gcp", "scanner_azure", "scanner_k8s",
}

// stubModules are disabled by default (no real implementation yet)
var stubModules = map[string]bool{
	"scanner_aws": true, "scanner_gcp": true,
	"scanner_azure": true, "scanner_k8s": true,
}

func (srv *server) handleGetModules(w http.ResponseWriter, r *http.Request) {
	result := make(map[string]bool, len(moduleKeys))
	for _, m := range moduleKeys {
		val, err := srv.getSetting(r.Context(), "module_"+m)
		if err != nil || val == "" {
			// Default: real modules on, stubs off
			result[m] = !stubModules[m]
		} else {
			result[m] = val == "1"
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (srv *server) handleSetModules(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	user, err := srv.getUser(r.Context(), userID)
	if err != nil || (user.Role != "admin" && user.Role != "owner") {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	var req map[string]bool
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	for k, v := range req {
		val := "0"
		if v {
			val = "1"
		}
		_ = srv.setSetting(r.Context(), "module_"+k, val)
	}
	writeJSON(w, http.StatusOK, req)
}

// ── Notify me (coming soon audit types) ──────────────────────────────────────

func (srv *server) handleNotifyMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type  string `json:"type"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Type == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "type and email required")
		return
	}
	_, err := srv.db.Exec(r.Context(),
		`INSERT INTO notify_requests(type,email) VALUES($1,$2)`,
		req.Type, req.Email,
	)
	if err != nil {
		log.Printf("notify_requests insert: %v", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Workspace ─────────────────────────────────────────────────────────────────

func (srv *server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	ws, err := srv.getTenant(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (srv *server) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	userID := r.Context().Value(ctxUserID).(string)
	user, err := srv.getUser(r.Context(), userID)
	if err != nil || (user.Role != "admin" && user.Role != "owner") {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	var req struct {
		Name            string `json:"name"`
		SlackWebhookURL string `json:"slack_webhook_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	ws, err := srv.updateTenant(r.Context(), tenantID, req.Name, req.SlackWebhookURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update")
		return
	}
	srv.logActivity(r.Context(), tenantID, userID, user.Email, "workspace.update", "workspace", tenantID, clientIP(r))
	writeJSON(w, http.StatusOK, ws)
}

// ── Team member role update ───────────────────────────────────────────────────

func (srv *server) handleUpdateTeamMember(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	actorID := r.Context().Value(ctxUserID).(string)
	targetID := chi.URLParam(r, "id")

	// Cannot change own role
	if targetID == actorID {
		writeError(w, http.StatusBadRequest, "cannot change your own role")
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Role != "admin" && req.Role != "viewer" {
		writeError(w, http.StatusBadRequest, "role must be admin or viewer")
		return
	}
	if err := srv.updateTeamMemberRole(r.Context(), tenantID, targetID, req.Role); err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	actor, _ := srv.getUser(r.Context(), actorID)
	srv.logActivity(r.Context(), tenantID, actorID, actor.Email, "team.role_changed", "user", targetID, clientIP(r))
	writeJSON(w, http.StatusOK, map[string]string{"role": req.Role})
}

// ── Asset GET (for branding previews) ────────────────────────────────────────

func (srv *server) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	assetType := chi.URLParam(r, "assetType")
	allowed := map[string]string{
		"logo":      "logo.png",
		"watermark": "watermark.png",
		"footer-bg": "footer-bg.png",
	}
	filename, ok := allowed[assetType]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown asset type")
		return
	}
	// Try user-specific asset (same path as upload uses)
	path := filepath.Join(envOr("DATA_DIR", "/app/data"), "users", userID, "assets", filename)
	if _, err := os.Stat(path); err == nil {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-cache, no-store")
		http.ServeFile(w, r, path)
		return
	}
	// Fall back to default assets
	defPath := defaultAssetPath(filename)
	if _, err := os.Stat(defPath); err == nil {
		w.Header().Set("Content-Type", "image/png")
		http.ServeFile(w, r, defPath)
		return
	}
	http.NotFound(w, r)
}

// ── Activity log ──────────────────────────────────────────────────────────────

func (srv *server) handleGetActivityLog(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	entries, err := srv.listActivityLog(r.Context(), tenantID, 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if entries == nil {
		entries = []ActivityLogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func serveJSONFile(w http.ResponseWriter, job AuditJob, filename string) {
	if job.HTMLPath == "" {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	p := filepath.Join(filepath.Dir(job.HTMLPath), filename)
	data, err := os.ReadFile(p)
	if err != nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
