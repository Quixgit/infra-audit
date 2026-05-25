package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

var errNotFound = errors.New("not found")

// ── Models ────────────────────────────────────────────────────────────────────

type RemediationTask struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	JobID           *string    `json:"job_id,omitempty"`
	Source          string     `json:"source"`
	FindingIndex    int        `json:"finding_index"`
	ConnectionID    *string    `json:"connection_id,omitempty"`
	ConnectionName  string     `json:"connection_name"`
	Title           string     `json:"title"`
	Severity        string     `json:"severity"`
	ResourceName    string     `json:"resource_name"`
	Description     string     `json:"description"`
	RemediationText string     `json:"remediation_text"`
	RiskText        string     `json:"risk_text"`
	AssignedTo      *string    `json:"assigned_to,omitempty"`
	AssignedEmail   string     `json:"assigned_email"`
	Lane            string     `json:"lane"` // immediate,this_week,this_month,backlog,done
	DueDate         *string    `json:"due_date,omitempty"`
	VerifyJobID     *string    `json:"verify_job_id,omitempty"`
	VerifyStatus    string     `json:"verify_status"` // "",pending,still_present,not_found
	VerifiedAt      *time.Time `json:"verified_at,omitempty"`
	CommentCount    int        `json:"comment_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type RemediationComment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// timelineToLane maps a finding's "timeline" text to a Kanban lane.
func timelineToLane(timeline string) string {
	t := strings.ToLower(timeline)
	switch {
	case strings.Contains(t, "immediate") || strings.Contains(t, "today"):
		return "immediate"
	case strings.Contains(t, "week"):
		return "this_week"
	case strings.Contains(t, "month"):
		return "this_month"
	default:
		return "backlog"
	}
}

var validLanes = map[string]bool{
	"immediate": true, "this_week": true, "this_month": true, "backlog": true, "done": true,
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (srv *server) listRemediationTasks(ctx context.Context, tenantID string) ([]RemediationTask, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT t.id, t.tenant_id, t.job_id, t.source, t.finding_index,
		       t.connection_id, t.connection_name, t.title, t.severity,
		       t.resource_name, t.description, t.remediation_text, t.risk_text,
		       t.assigned_to, COALESCE(u.email,'') as assigned_email,
		       t.lane, t.due_date::text, t.verify_job_id, t.verify_status, t.verified_at,
		       (SELECT COUNT(*) FROM remediation_comments WHERE task_id=t.id),
		       t.created_at, t.updated_at
		FROM remediation_tasks t
		LEFT JOIN users u ON u.id = t.assigned_to
		WHERE t.tenant_id=$1
		ORDER BY t.created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RemediationTask
	for rows.Next() {
		var task RemediationTask
		if err := rows.Scan(
			&task.ID, &task.TenantID, &task.JobID, &task.Source, &task.FindingIndex,
			&task.ConnectionID, &task.ConnectionName, &task.Title, &task.Severity,
			&task.ResourceName, &task.Description, &task.RemediationText, &task.RiskText,
			&task.AssignedTo, &task.AssignedEmail,
			&task.Lane, &task.DueDate, &task.VerifyJobID, &task.VerifyStatus, &task.VerifiedAt,
			&task.CommentCount, &task.CreatedAt, &task.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, rows.Err()
}

func (srv *server) getRemediationTask(ctx context.Context, id, tenantID string) (RemediationTask, error) {
	var task RemediationTask
	err := srv.db.QueryRow(ctx, `
		SELECT t.id, t.tenant_id, t.job_id, t.source, t.finding_index,
		       t.connection_id, t.connection_name, t.title, t.severity,
		       t.resource_name, t.description, t.remediation_text, t.risk_text,
		       t.assigned_to, COALESCE(u.email,'') as assigned_email,
		       t.lane, t.due_date::text, t.verify_job_id, t.verify_status, t.verified_at,
		       (SELECT COUNT(*) FROM remediation_comments WHERE task_id=t.id),
		       t.created_at, t.updated_at
		FROM remediation_tasks t
		LEFT JOIN users u ON u.id = t.assigned_to
		WHERE t.id=$1 AND t.tenant_id=$2`, id, tenantID).Scan(
		&task.ID, &task.TenantID, &task.JobID, &task.Source, &task.FindingIndex,
		&task.ConnectionID, &task.ConnectionName, &task.Title, &task.Severity,
		&task.ResourceName, &task.Description, &task.RemediationText, &task.RiskText,
		&task.AssignedTo, &task.AssignedEmail,
		&task.Lane, &task.DueDate, &task.VerifyJobID, &task.VerifyStatus, &task.VerifiedAt,
		&task.CommentCount, &task.CreatedAt, &task.UpdatedAt,
	)
	return task, err
}

// createRemediationTaskFromFinding auto-creates a task; skips if one already exists.
func (srv *server) createRemediationTaskFromFinding(ctx context.Context, tenantID string, af AggregatedFinding, timeline string) error {
	lane := timelineToLane(timeline)
	_, err := srv.db.Exec(ctx, `
		INSERT INTO remediation_tasks
		  (tenant_id, job_id, source, finding_index, connection_id, connection_name,
		   title, severity, resource_name, description, remediation_text, risk_text, lane)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (tenant_id, job_id, source, finding_index) WHERE job_id IS NOT NULL DO NOTHING`,
		tenantID, af.JobID, af.Source, af.FindingIndex,
		af.ConnectionID, af.ConnectionName,
		af.Title, af.Severity, af.ResourceName,
		af.Evidence, af.Remediation, "",
		lane,
	)
	return err
}

func (srv *server) updateRemediationTask(ctx context.Context, id, tenantID string, lane, assignedTo, dueDate *string) (RemediationTask, error) {
	_, err := srv.db.Exec(ctx, `
		UPDATE remediation_tasks SET
		  lane        = COALESCE($3, lane),
		  assigned_to = CASE WHEN $4::text IS NOT NULL THEN $4::uuid ELSE assigned_to END,
		  due_date    = CASE WHEN $5::text IS NOT NULL THEN $5::date ELSE due_date END,
		  updated_at  = NOW()
		WHERE id=$1 AND tenant_id=$2`,
		id, tenantID, lane, assignedTo, dueDate)
	if err != nil {
		return RemediationTask{}, err
	}
	return srv.getRemediationTask(ctx, id, tenantID)
}

func (srv *server) deleteRemediationTask(ctx context.Context, id, tenantID string) error {
	tag, err := srv.db.Exec(ctx, `DELETE FROM remediation_tasks WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

func (srv *server) listRemediationComments(ctx context.Context, taskID string) ([]RemediationComment, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT c.id, c.task_id, c.user_id, c.user_email, c.body, c.created_at
		FROM remediation_comments c
		WHERE c.task_id=$1
		ORDER BY c.created_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RemediationComment
	for rows.Next() {
		var c RemediationComment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.UserID, &c.UserEmail, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (srv *server) addRemediationComment(ctx context.Context, taskID, userID, userEmail, body string) (RemediationComment, error) {
	var c RemediationComment
	err := srv.db.QueryRow(ctx, `
		INSERT INTO remediation_comments(task_id, user_id, user_email, body)
		VALUES ($1,$2,$3,$4)
		RETURNING id, task_id, user_id, user_email, body, created_at`,
		taskID, userID, userEmail, body,
	).Scan(&c.ID, &c.TaskID, &c.UserID, &c.UserEmail, &c.Body, &c.CreatedAt)
	return c, err
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (srv *server) handleListRemediationTasks(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	tasks, err := srv.listRemediationTasks(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if tasks == nil {
		tasks = []RemediationTask{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (srv *server) handleCreateRemediationTask(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)

	var req struct {
		Title           string  `json:"title"`
		Severity        string  `json:"severity"`
		Lane            string  `json:"lane"`
		ConnectionName  string  `json:"connection_name"`
		ResourceName    string  `json:"resource_name"`
		Description     string  `json:"description"`
		RemediationText string  `json:"remediation_text"`
		RiskText        string  `json:"risk_text"`
		DueDate         *string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	if !validLanes[req.Lane] {
		req.Lane = "backlog"
	}
	if req.Severity == "" {
		req.Severity = "medium"
	}

	var task RemediationTask
	err := srv.db.QueryRow(r.Context(), `
		INSERT INTO remediation_tasks
		  (tenant_id, connection_name, title, severity, resource_name, description, remediation_text, risk_text, lane, due_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::date)
		RETURNING id, tenant_id, job_id, source, finding_index, connection_id, connection_name,
		          title, severity, resource_name, description, remediation_text, risk_text,
		          assigned_to, ''::text as assigned_email, lane, due_date::text,
		          verify_job_id, verify_status, verified_at, 0::bigint, created_at, updated_at`,
		tenantID, req.ConnectionName, req.Title, req.Severity, req.ResourceName,
		req.Description, req.RemediationText, req.RiskText, req.Lane, req.DueDate,
	).Scan(
		&task.ID, &task.TenantID, &task.JobID, &task.Source, &task.FindingIndex,
		&task.ConnectionID, &task.ConnectionName, &task.Title, &task.Severity,
		&task.ResourceName, &task.Description, &task.RemediationText, &task.RiskText,
		&task.AssignedTo, &task.AssignedEmail,
		&task.Lane, &task.DueDate, &task.VerifyJobID, &task.VerifyStatus, &task.VerifiedAt,
		&task.CommentCount, &task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (srv *server) handleUpdateRemediationTask(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	// Fetch task before update so we can detect lane transitions
	prevTask, err := srv.getRemediationTask(r.Context(), id, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req struct {
		Lane       *string `json:"lane"`
		AssignedTo *string `json:"assigned_to"`
		DueDate    *string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Lane != nil && !validLanes[*req.Lane] {
		writeError(w, http.StatusBadRequest, "invalid lane")
		return
	}

	task, err := srv.updateRemediationTask(r.Context(), id, tenantID, req.Lane, req.AssignedTo, req.DueDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	// ── Bidirectional sync ────────────────────────────────────────────────────
	// Task moved TO done → mark the linked finding as "fixed"
	if req.Lane != nil && *req.Lane == "done" && prevTask.Lane != "done" {
		srv.syncTaskToFinding(r.Context(), task, userID, tenantID, "fixed")
	}
	// Task moved OUT of done → reopen the linked finding as "in_progress"
	if req.Lane != nil && *req.Lane != "done" && prevTask.Lane == "done" {
		srv.syncTaskToFinding(r.Context(), task, userID, tenantID, "in_progress")
	}

	writeJSON(w, http.StatusOK, task)
}

// syncTaskToFinding propagates a lane change back to the source finding override.
func (srv *server) syncTaskToFinding(ctx context.Context, task RemediationTask, userID, tenantID, status string) {
	if task.JobID == nil || task.FindingIndex < 0 {
		return // manually-created task, no linked finding
	}
	_, _ = srv.upsertFindingOverride(ctx, userID, tenantID, *task.JobID, task.Source, task.FindingIndex, status, "")
}

func (srv *server) handleDeleteRemediationTask(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	if err := srv.deleteRemediationTask(r.Context(), id, tenantID); err != nil {
		if err == errNotFound {
			writeError(w, http.StatusNotFound, "not found")
		} else {
			writeError(w, http.StatusInternalServerError, "db error")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleListRemediationComments(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	taskID := chi.URLParam(r, "id")

	// Verify task belongs to tenant
	if _, err := srv.getRemediationTask(r.Context(), taskID, tenantID); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	comments, err := srv.listRemediationComments(r.Context(), taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if comments == nil {
		comments = []RemediationComment{}
	}
	writeJSON(w, http.StatusOK, comments)
}

func (srv *server) handleAddRemediationComment(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)
	taskID := chi.URLParam(r, "id")

	// Verify task belongs to tenant
	if _, err := srv.getRemediationTask(r.Context(), taskID, tenantID); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Body) == "" {
		writeError(w, http.StatusBadRequest, "body required")
		return
	}

	user, _ := srv.getUser(r.Context(), userID)
	comment, err := srv.addRemediationComment(r.Context(), taskID, userID, user.Email, req.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

// handleVerifyFix starts a new audit for the task's connection and sets verify_job_id.
func (srv *server) handleVerifyFix(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)
	taskID := chi.URLParam(r, "id")

	task, err := srv.getRemediationTask(r.Context(), taskID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if task.ConnectionID == nil {
		writeError(w, http.StatusBadRequest, "task has no connection")
		return
	}

	conn, err := srv.getConnection(r.Context(), *task.ConnectionID)
	if err != nil || conn.TenantID != tenantID {
		writeError(w, http.StatusBadRequest, "connection not found")
		return
	}

	newJob, err := srv.createJob(r.Context(), conn.ID, userID, tenantID, conn.ConnType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	// Save verify_job_id and reset status
	_, _ = srv.db.Exec(r.Context(), `
		UPDATE remediation_tasks SET verify_job_id=$2, verify_status='pending', updated_at=NOW()
		WHERE id=$1`, taskID, newJob.ID)

	// Start the audit in background
	connType := conn.ConnType
	connID := conn.ID
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("verify audit panic for job %s: %v", newJob.ID, rec)
			}
		}()
		if connType == "code" {
			srv.runCodeAudit(newJob.ID, connID, userID)
		} else {
			srv.runAudit(newJob.ID, connID, userID)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{"verify_job_id": newJob.ID})
}

// handleVerifyResult checks if the verify job finished and whether the finding is still present.
func (srv *server) handleVerifyResult(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	taskID := chi.URLParam(r, "id")

	task, err := srv.getRemediationTask(r.Context(), taskID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if task.VerifyJobID == nil {
		writeJSON(w, http.StatusOK, map[string]string{"verify_status": ""})
		return
	}
	if task.VerifyStatus != "pending" {
		writeJSON(w, http.StatusOK, map[string]string{"verify_status": task.VerifyStatus})
		return
	}

	job, err := srv.getJob(r.Context(), *task.VerifyJobID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"verify_status": "pending"})
		return
	}
	if job.Status != "done" {
		writeJSON(w, http.StatusOK, map[string]string{"verify_status": "pending"})
		return
	}

	// Job finished — check if our finding is still in the new results
	found := false
	if job.HTMLPath != "" {
		dir := filepath.Dir(job.HTMLPath)
		rawList := loadRawFindings(filepath.Join(dir, task.Source+".json"))
		titleLower := strings.ToLower(strings.TrimSpace(task.Title))
		for _, raw := range rawList {
			t := strings.ToLower(strings.TrimSpace(strField(raw, "title")))
			if t == titleLower {
				found = true
				break
			}
		}
	}

	verifyStatus := "not_found"
	newLane := "done"
	if found {
		verifyStatus = "still_present"
		newLane = task.Lane // keep current lane
	}

	now := time.Now().UTC()
	_, _ = srv.db.Exec(r.Context(), `
		UPDATE remediation_tasks SET
		  verify_status=$2, verified_at=$3,
		  lane=CASE WHEN $4='done' THEN 'done' ELSE lane END,
		  updated_at=NOW()
		WHERE id=$1`, taskID, verifyStatus, now, newLane)

	writeJSON(w, http.StatusOK, map[string]string{"verify_status": verifyStatus})
}
