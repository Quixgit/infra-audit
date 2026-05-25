package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AggregatedFinding struct {
	JobID          string    `json:"job_id"`
	Source         string    `json:"source"`
	FindingIndex   int       `json:"finding_index"`
	JobDate        time.Time `json:"job_date"`
	ConnectionID   string    `json:"connection_id"`
	ConnectionName string    `json:"connection_name"`
	ConnType       string    `json:"conn_type"`

	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Category     string `json:"category,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	ResourceName string `json:"resource_name,omitempty"`
	Evidence     string `json:"evidence,omitempty"`

	Tool    string `json:"tool,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	RuleID  string `json:"rule_id,omitempty"`
	Package string `json:"package,omitempty"`
	CVE     string `json:"cve,omitempty"`

	Recommendation string `json:"recommendation,omitempty"`
	Remediation    string `json:"remediation,omitempty"`

	Status string `json:"status"`
	Note   string `json:"note"`
}

type FindingOverride struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	JobID        string    `json:"job_id"`
	Source       string    `json:"source"`
	FindingIndex int       `json:"finding_index"`
	Status       string    `json:"status"`
	Note         string    `json:"note"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func loadRawFindings(path string) []map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []map[string]interface{}
	_ = json.Unmarshal(data, &out)
	return out
}

func strField(raw map[string]interface{}, key string) string {
	v, _ := raw[key].(string)
	return v
}

func intField(raw map[string]interface{}, key string) int {
	v, _ := raw[key].(float64)
	return int(v)
}

func buildAggregatedFinding(job AuditJob, source string, idx int, raw map[string]interface{}) AggregatedFinding {
	title := strField(raw, "title")
	if title == "" {
		title = strField(raw, "rule_id")
	}
	return AggregatedFinding{
		JobID:          job.ID,
		Source:         source,
		FindingIndex:   idx,
		JobDate:        job.StartedAt,
		ConnectionID:   job.ConnectionID,
		ConnectionName: job.ConnectionName,
		ConnType:       job.ConnType,
		Severity:       strField(raw, "severity"),
		Title:          title,
		Category:       strField(raw, "category"),
		ResourceType:   strField(raw, "resource_type"),
		ResourceName:   strField(raw, "resource_name"),
		Evidence:       strField(raw, "evidence"),
		Tool:           strField(raw, "tool"),
		File:           strField(raw, "file"),
		Line:           intField(raw, "line"),
		RuleID:         strField(raw, "rule_id"),
		Package:        strField(raw, "package"),
		CVE:            strField(raw, "cve"),
		Recommendation: strField(raw, "recommendation"),
		Remediation:    strField(raw, "remediation"),
		Status:         "open",
	}
}

var validFindingStatuses = map[string]bool{
	"open":           true,
	"in_progress":    true,
	"fixed":          true,
	"accepted_risk":  true,
	"false_positive": true,
}

// getAllAggregatedFindings loads every finding across all done jobs for a tenant,
// merging in any status overrides from the DB. Used by findings and compliance handlers.
func (srv *server) getAllAggregatedFindings(ctx context.Context, tenantID string) ([]AggregatedFinding, error) {
	jobs, err := srv.listJobs(ctx, tenantID, 500)
	if err != nil {
		return nil, err
	}

	overrides, err := srv.listFindingOverrides(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	type ovKey struct {
		jobID  string
		source string
		idx    int
	}
	ovMap := map[ovKey]FindingOverride{}
	for _, o := range overrides {
		ovMap[ovKey{o.JobID, o.Source, o.FindingIndex}] = o
	}

	// Only use the most recent completed job per connection.
	// This prevents the same finding from appearing multiple times when a
	// connection has been audited more than once — newer findings supersede older ones.
	latestJob := map[string]AuditJob{} // connectionID → most recent done job
	for _, job := range jobs {
		if job.Status != "done" || job.HTMLPath == "" {
			continue
		}
		prev, ok := latestJob[job.ConnectionID]
		if !ok || job.StartedAt.After(prev.StartedAt) {
			latestJob[job.ConnectionID] = job
		}
	}

	var result []AggregatedFinding
	for _, job := range latestJob {
		dir := filepath.Dir(job.HTMLPath)
		sources := []string{"findings"}
		if job.ConnType == "code" {
			sources = append(sources, "tf_findings")
		}
		for _, src := range sources {
			rawList := loadRawFindings(filepath.Join(dir, src+".json"))
			for i, raw := range rawList {
				af := buildAggregatedFinding(job, src, i, raw)
				if af.Title == "" {
					continue
				}
				if ov, ok := ovMap[ovKey{job.ID, src, i}]; ok {
					af.Status = ov.Status
					af.Note = ov.Note
				}
				result = append(result, af)
			}
		}
	}
	return result, nil
}

func (srv *server) handleGetAggregatedFindings(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)

	q := r.URL.Query()
	severityFilter := strings.ToLower(q.Get("severity"))
	statusFilter := q.Get("status")
	connFilter := q.Get("connection_id")
	toolFilter := strings.ToLower(q.Get("tool"))
	search := strings.ToLower(q.Get("search"))

	all, err := srv.getAllAggregatedFindings(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	result := []AggregatedFinding{}
	for _, af := range all {
		if connFilter != "" && af.ConnectionID != connFilter {
			continue
		}
		if severityFilter != "" && strings.ToLower(af.Severity) != severityFilter {
			continue
		}
		if statusFilter != "" && af.Status != statusFilter {
			continue
		}
		if toolFilter != "" && strings.ToLower(af.Tool) != toolFilter {
			continue
		}
		if search != "" {
			hay := strings.ToLower(af.Title + " " + af.File + " " + af.ResourceName + " " + af.RuleID + " " + af.Package)
			if !strings.Contains(hay, search) {
				continue
			}
		}
		result = append(result, af)
	}

	writeJSON(w, http.StatusOK, result)
}

func (srv *server) handleSetFindingOverride(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	tenantID := r.Context().Value(ctxTenantID).(string)

	var req struct {
		JobID        string `json:"job_id"`
		Source       string `json:"source"`
		FindingIndex int    `json:"finding_index"`
		Status       string `json:"status"`
		Note         string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if !validFindingStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	if req.Source == "" {
		req.Source = "findings"
	}
	// Whitelist allowed source values to prevent path traversal via req.Source+".json"
	var allowedSources = map[string]bool{"findings": true, "tf_findings": true}
	if !allowedSources[req.Source] {
		writeError(w, http.StatusBadRequest, "invalid source")
		return
	}

	// Verify the job belongs to the tenant (not just the individual user).
	job, err := srv.getJob(r.Context(), req.JobID)
	if err != nil || job.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	override, err := srv.upsertFindingOverride(r.Context(), userID, tenantID, req.JobID, req.Source, req.FindingIndex, req.Status, req.Note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	// Auto-create a remediation task when a finding is moved to in_progress.
	if req.Status == "in_progress" && job.HTMLPath != "" {
		dir := filepath.Dir(job.HTMLPath)
		rawList := loadRawFindings(filepath.Join(dir, req.Source+".json"))
		if req.FindingIndex >= 0 && req.FindingIndex < len(rawList) {
			af := buildAggregatedFinding(job, req.Source, req.FindingIndex, rawList[req.FindingIndex])
			timeline := strField(rawList[req.FindingIndex], "timeline")
			if af.Title != "" {
				_ = srv.createRemediationTaskFromFinding(r.Context(), tenantID, af, timeline)
			}
		}
	}

	// Sync finding status changes back to any linked remediation task.
	switch req.Status {
	case "fixed", "accepted_risk", "false_positive":
		// Finding resolved → move task to Done lane
		_, _ = srv.db.Exec(r.Context(),
			`UPDATE remediation_tasks SET lane='done', updated_at=NOW()
			 WHERE tenant_id=$1 AND job_id=$2 AND source=$3 AND finding_index=$4 AND lane!='done'`,
			tenantID, req.JobID, req.Source, req.FindingIndex)
	case "open":
		// Finding reopened → move task back to backlog if it was done
		_, _ = srv.db.Exec(r.Context(),
			`UPDATE remediation_tasks SET lane='backlog', updated_at=NOW()
			 WHERE tenant_id=$1 AND job_id=$2 AND source=$3 AND finding_index=$4 AND lane='done'`,
			tenantID, req.JobID, req.Source, req.FindingIndex)
	}

	writeJSON(w, http.StatusOK, override)
}
