package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type SLARule struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Severity    string    `json:"severity"`
	MaxDaysOpen int       `json:"max_days_open"`
	NotifyEmail bool      `json:"notify_email"`
	NotifySlack bool      `json:"notify_slack"`
	CreatedAt   time.Time `json:"created_at"`
}

type SLABreach struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	JobID         string     `json:"job_id"`
	ConnectionID  string     `json:"connection_id"`
	ConnectionName string    `json:"connection_name"`
	FindingIndex  int        `json:"finding_index"`
	Source        string     `json:"source"`
	Title         string     `json:"title"`
	Severity      string     `json:"severity"`
	OpenedAt      time.Time  `json:"opened_at"`
	BreachedAt    time.Time  `json:"breached_at"`
	NotifiedAt    *time.Time `json:"notified_at,omitempty"`
	DaysOverdue   int        `json:"days_overdue"`
	Status        string     `json:"status"`
}

type SecurityScore struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	ConnectionID  string    `json:"connection_id"`
	ConnectionName string   `json:"connection_name,omitempty"`
	JobID         string    `json:"job_id"`
	Score         int       `json:"score"`
	CriticalCount int       `json:"critical_count"`
	HighCount     int       `json:"high_count"`
	MediumCount   int       `json:"medium_count"`
	LowCount      int       `json:"low_count"`
	CalculatedAt  time.Time `json:"calculated_at"`
}

type ScoreTrendDay struct {
	Date     string  `json:"date"`
	AvgScore float64 `json:"avg_score"`
}

type FindingChange struct {
	JobID          string    `json:"job_id"`
	ConnectionID   string    `json:"connection_id"`
	ConnectionName string    `json:"connection_name"`
	Title          string    `json:"title"`
	Severity       string    `json:"severity"`
	ChangeType     string    `json:"change_type"` // new | regression | resolved
	OccurredAt     time.Time `json:"occurred_at"`
}

type MonitoringOverview struct {
	AvgScore                 int             `json:"avg_score"`
	SLABreachCount           int             `json:"sla_breach_count"`
	NewFindingsThisWeek      int             `json:"new_findings_this_week"`
	RegressionsFindingsCount int             `json:"regressions_findings_count"`
	SLABreachesBySeverity    []SLABySeverity `json:"sla_breaches_by_severity"`
	ScoreTrend               []ScoreTrendDay `json:"score_trend"`
	RecentChanges            []FindingChange `json:"recent_changes"`
}

type SLABySeverity struct {
	Severity   string `json:"severity"`
	Count      int    `json:"count"`
	OldestDays int    `json:"oldest_days"`
}

// ── DB helpers — SLA rules ────────────────────────────────────────────────────

func (srv *server) ensureDefaultSLARules(ctx context.Context, tenantID string) error {
	defaults := []struct {
		severity string
		days     int
	}{
		{"critical", 3},
		{"high", 7},
		{"medium", 30},
		{"low", 90},
	}
	for _, d := range defaults {
		_, err := srv.db.Exec(ctx, `
			INSERT INTO sla_rules(tenant_id, severity, max_days_open, notify_email, notify_slack)
			VALUES($1,$2,$3,true,false)
			ON CONFLICT(tenant_id, severity) DO NOTHING`,
			tenantID, d.severity, d.days)
		if err != nil {
			return err
		}
	}
	return nil
}

func (srv *server) listSLARules(ctx context.Context, tenantID string) ([]SLARule, error) {
	_ = srv.ensureDefaultSLARules(ctx, tenantID)
	rows, err := srv.db.Query(ctx,
		`SELECT id, tenant_id, severity, max_days_open, notify_email, notify_slack, created_at
		 FROM sla_rules WHERE tenant_id=$1
		 ORDER BY CASE severity WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END`,
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SLARule
	for rows.Next() {
		var r SLARule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Severity, &r.MaxDaysOpen, &r.NotifyEmail, &r.NotifySlack, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (srv *server) upsertSLARule(ctx context.Context, tenantID, severity string, maxDays int, notifyEmail, notifySlack bool) (SLARule, error) {
	var r SLARule
	err := srv.db.QueryRow(ctx, `
		INSERT INTO sla_rules(tenant_id, severity, max_days_open, notify_email, notify_slack)
		VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(tenant_id, severity) DO UPDATE
		  SET max_days_open=$3, notify_email=$4, notify_slack=$5
		RETURNING id, tenant_id, severity, max_days_open, notify_email, notify_slack, created_at`,
		tenantID, severity, maxDays, notifyEmail, notifySlack,
	).Scan(&r.ID, &r.TenantID, &r.Severity, &r.MaxDaysOpen, &r.NotifyEmail, &r.NotifySlack, &r.CreatedAt)
	return r, err
}

// ── DB helpers — SLA breaches ─────────────────────────────────────────────────

func (srv *server) createSLABreach(ctx context.Context, tenantID, jobID, connectionID, source, title, severity string, findingIdx int, openedAt time.Time) error {
	_, err := srv.db.Exec(ctx, `
		INSERT INTO finding_sla_breaches(tenant_id, job_id, connection_id, source, finding_index, title, severity, opened_at, breached_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT(tenant_id, job_id, source, finding_index) DO NOTHING`,
		tenantID, jobID, connectionID, source, findingIdx, title, severity, openedAt)
	return err
}

func (srv *server) markSLABreachNotified(ctx context.Context, id string) error {
	_, err := srv.db.Exec(ctx, `UPDATE finding_sla_breaches SET notified_at=NOW() WHERE id=$1`, id)
	return err
}

func (srv *server) listSLABreaches(ctx context.Context, tenantID string) ([]SLABreach, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT b.id, b.tenant_id, b.job_id, b.connection_id,
		       COALESCE(c.name,'') as connection_name,
		       b.finding_index, b.source, b.title, b.severity,
		       b.opened_at, b.breached_at, b.notified_at,
		       EXTRACT(EPOCH FROM (NOW()-b.breached_at))/86400 as days_overdue,
		       COALESCE(fo.status,'open') as status
		FROM finding_sla_breaches b
		LEFT JOIN connections c ON c.id = b.connection_id
		LEFT JOIN finding_overrides fo ON fo.job_id = b.job_id
		  AND fo.source = b.source AND fo.finding_index = b.finding_index
		WHERE b.tenant_id=$1
		ORDER BY b.breached_at ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SLABreach
	for rows.Next() {
		var b SLABreach
		var daysOverdue float64
		if err := rows.Scan(&b.ID, &b.TenantID, &b.JobID, &b.ConnectionID,
			&b.ConnectionName, &b.FindingIndex, &b.Source, &b.Title, &b.Severity,
			&b.OpenedAt, &b.BreachedAt, &b.NotifiedAt,
			&daysOverdue, &b.Status); err != nil {
			return nil, err
		}
		b.DaysOverdue = int(math.Max(0, daysOverdue))
		out = append(out, b)
	}
	return out, rows.Err()
}

// ── DB helpers — security scores ──────────────────────────────────────────────

func calcScore(critical, high, medium, low int) int {
	score := 100 - (critical*20 + high*10 + medium*5 + low*2)
	if score < 0 {
		score = 0
	}
	return score
}

func (srv *server) saveSecurityScore(ctx context.Context, tenantID, connectionID, jobID string, critical, high, medium, low int) error {
	score := calcScore(critical, high, medium, low)
	_, err := srv.db.Exec(ctx, `
		INSERT INTO security_scores(tenant_id, connection_id, job_id, score,
		                            critical_count, high_count, medium_count, low_count, calculated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT(job_id) DO UPDATE
		  SET score=$4, critical_count=$5, high_count=$6, medium_count=$7, low_count=$8, calculated_at=NOW()`,
		tenantID, connectionID, jobID, score, critical, high, medium, low)
	return err
}

func (srv *server) getScoreHistory(ctx context.Context, tenantID, connectionID string) ([]SecurityScore, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT s.id, s.tenant_id, s.connection_id, s.job_id,
		       s.score, s.critical_count, s.high_count, s.medium_count, s.low_count, s.calculated_at
		FROM security_scores s
		WHERE s.tenant_id=$1 AND s.connection_id=$2
		ORDER BY s.calculated_at DESC
		LIMIT 30`, tenantID, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SecurityScore
	for rows.Next() {
		var s SecurityScore
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ConnectionID, &s.JobID,
			&s.Score, &s.CriticalCount, &s.HighCount, &s.MediumCount, &s.LowCount, &s.CalculatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (srv *server) getScoreTrend(ctx context.Context, tenantID string) ([]ScoreTrendDay, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT
		  gs.dt::date::text,
		  COALESCE(AVG(s.score)::int, NULL)
		FROM generate_series(NOW()-INTERVAL '29 days', NOW(), INTERVAL '1 day') gs(dt)
		LEFT JOIN security_scores s
		  ON s.calculated_at::date = gs.dt::date AND s.tenant_id = $1
		GROUP BY gs.dt::date
		ORDER BY gs.dt::date`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScoreTrendDay
	for rows.Next() {
		var d ScoreTrendDay
		var avgScore *float64
		if err := rows.Scan(&d.Date, &avgScore); err != nil {
			return nil, err
		}
		if avgScore != nil {
			d.AvgScore = *avgScore
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (srv *server) getLatestScorePerConnection(ctx context.Context, tenantID string) ([]SecurityScore, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT DISTINCT ON (s.connection_id)
		  s.id, s.tenant_id, s.connection_id, c.name, s.job_id,
		  s.score, s.critical_count, s.high_count, s.medium_count, s.low_count, s.calculated_at
		FROM security_scores s
		JOIN connections c ON c.id = s.connection_id
		WHERE s.tenant_id=$1
		ORDER BY s.connection_id, s.calculated_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SecurityScore
	for rows.Next() {
		var s SecurityScore
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ConnectionID, &s.ConnectionName, &s.JobID,
			&s.Score, &s.CriticalCount, &s.HighCount, &s.MediumCount, &s.LowCount, &s.CalculatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ── DB helpers — finding changes ──────────────────────────────────────────────

func (srv *server) saveJobCounts(ctx context.Context, jobID string, newCount, regressionCount int) error {
	_, err := srv.db.Exec(ctx,
		`UPDATE audit_jobs SET new_findings_count=$2, regression_findings_count=$3 WHERE id=$1`,
		jobID, newCount, regressionCount)
	return err
}

func (srv *server) getRecentChanges(ctx context.Context, tenantID string, days int) ([]FindingChange, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT fc.job_id, fc.connection_id, COALESCE(c.name,'') as conn_name,
		       fc.title, fc.severity, fc.change_type, fc.occurred_at
		FROM finding_changes fc
		LEFT JOIN connections c ON c.id = fc.connection_id
		WHERE fc.tenant_id=$1 AND fc.occurred_at > NOW() - ($2 || ' days')::INTERVAL
		ORDER BY fc.occurred_at DESC
		LIMIT 50`, tenantID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FindingChange
	for rows.Next() {
		var fc FindingChange
		if err := rows.Scan(&fc.JobID, &fc.ConnectionID, &fc.ConnectionName,
			&fc.Title, &fc.Severity, &fc.ChangeType, &fc.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, fc)
	}
	return out, rows.Err()
}

func (srv *server) saveFindingChange(ctx context.Context, tenantID, jobID, connectionID, title, severity, changeType string) error {
	_, err := srv.db.Exec(ctx, `
		INSERT INTO finding_changes(tenant_id, job_id, connection_id, title, severity, change_type, occurred_at)
		VALUES($1,$2,$3,$4,$5,$6,NOW())`,
		tenantID, jobID, connectionID, title, severity, changeType)
	return err
}

// ── New findings detection ────────────────────────────────────────────────────

// fingerprintFindings returns a set of "title|file" strings for deduplication
func fingerprintFindings(rawList []map[string]interface{}) map[string]int {
	result := make(map[string]int)
	for i, raw := range rawList {
		title := strField(raw, "title")
		if title == "" {
			title = strField(raw, "rule_id")
		}
		file := strField(raw, "file")
		key := strings.ToLower(title + "|" + file)
		if key != "|" {
			result[key] = i
		}
	}
	return result
}

// detectFindingChanges compares current job findings with the previous job.
// Returns new, regression, and resolved counts.
func (srv *server) detectFindingChanges(ctx context.Context, job AuditJob) (newCount, regressionCount, resolvedCount int) {
	if job.HTMLPath == "" {
		return
	}
	reportDir := filepath.Dir(job.HTMLPath)
	tenantID := job.TenantID

	// Load overrides for the previous job to detect regressions
	overrides, _ := srv.listFindingOverrides(ctx, tenantID)
	type ovKey struct{ jobID, source string; idx int }
	ovMap := map[ovKey]string{}
	for _, o := range overrides {
		ovMap[ovKey{o.JobID, o.Source, o.FindingIndex}] = o.Status
	}

	// Get the previous completed job for same connection
	prevJob, err := srv.getPreviousJob(ctx, job.ID, job.ConnectionID, tenantID)
	if err != nil || prevJob.HTMLPath == "" {
		return // first audit for this connection
	}
	prevDir := filepath.Dir(prevJob.HTMLPath)

	sources := []string{"findings"}
	if job.ConnType == "code" {
		sources = append(sources, "tf_findings")
	}

	for _, src := range sources {
		currList := loadRawFindings(filepath.Join(reportDir, src+".json"))
		prevList := loadRawFindings(filepath.Join(prevDir, src+".json"))

		currFP := fingerprintFindings(currList)
		prevFP := fingerprintFindings(prevList)

		// New findings: in current but not in previous
		for fp, idx := range currFP {
			if _, wasPrev := prevFP[fp]; !wasPrev {
				raw := currList[idx]
				title := strField(raw, "title")
				if title == "" {
					title = strField(raw, "rule_id")
				}
				severity := strings.ToLower(strField(raw, "severity"))
				newCount++
				_ = srv.saveFindingChange(ctx, tenantID, job.ID, job.ConnectionID, title, severity, "new")
			}
		}

		// Regression: in current AND was 'fixed' in previous job
		for fp, idx := range currFP {
			if prevIdx, wasPrev := prevFP[fp]; wasPrev {
				prevStatus := ovMap[ovKey{prevJob.ID, src, prevIdx}]
				if prevStatus == "fixed" {
					raw := currList[idx]
					title := strField(raw, "title")
					if title == "" {
						title = strField(raw, "rule_id")
					}
					severity := strings.ToLower(strField(raw, "severity"))
					regressionCount++
					_ = srv.saveFindingChange(ctx, tenantID, job.ID, job.ConnectionID, title, severity, "regression")
				}
			}
		}

		// Resolved: in previous but NOT in current
		for fp, idx := range prevFP {
			if _, isCurr := currFP[fp]; !isCurr {
				// Only count as resolved if it wasn't already marked fixed
				prevStatus := ovMap[ovKey{prevJob.ID, src, idx}]
				if prevStatus != "fixed" && prevStatus != "accepted_risk" && prevStatus != "false_positive" {
					raw := prevList[idx]
					title := strField(raw, "title")
					if title == "" {
						title = strField(raw, "rule_id")
					}
					severity := strings.ToLower(strField(raw, "severity"))
					resolvedCount++
					_ = srv.saveFindingChange(ctx, tenantID, job.ID, job.ConnectionID, title, severity, "resolved")
				}
			}
		}
	}
	return
}

// postJobMonitoring runs after a job completes: detect changes, save score, notify
func (srv *server) postJobMonitoring(ctx context.Context, job AuditJob) {
	newCount, regressionCount, _ := srv.detectFindingChanges(ctx, job)

	if err := srv.saveJobCounts(ctx, job.ID, newCount, regressionCount); err != nil {
		log.Printf("monitoring: saveJobCounts: %v", err)
	}

	if err := srv.saveSecurityScore(ctx, job.TenantID, job.ConnectionID, job.ID,
		job.FindingsCritical, job.FindingsHigh, job.FindingsMedium, job.FindingsLow); err != nil {
		log.Printf("monitoring: saveSecurityScore: %v", err)
	}

	// Notify on new critical/high
	if newCount > 0 && job.FindingsCritical+job.FindingsHigh > 0 {
		user, err := srv.getUser(ctx, job.UserID)
		if err == nil && user.NotifyEmail && user.AuditorEmail != "" {
			go sendMonitoringAlert(user.AuditorEmail, job.ConnectionName, job.ID, "new_findings",
				fmt.Sprintf("%d new finding(s) detected — Critical: %d, High: %d",
					newCount, job.FindingsCritical, job.FindingsHigh))
		}
	}

	// Notify on regressions
	if regressionCount > 0 {
		user, err := srv.getUser(ctx, job.UserID)
		if err == nil && user.NotifyEmail && user.AuditorEmail != "" {
			go sendMonitoringAlert(user.AuditorEmail, job.ConnectionName, job.ID, "regression",
				fmt.Sprintf("%d regression(s): previously fixed finding(s) reappeared", regressionCount))
		}
	}
}

// ── SLA breach checker ────────────────────────────────────────────────────────

func (srv *server) startSLAChecker(ctx context.Context) {
	// Run at startup then every hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	log.Println("sla-checker: started")
	srv.runSLACheck(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.runSLACheck(ctx)
		}
	}
}

func (srv *server) runSLACheck(ctx context.Context) {
	// Get all tenants
	tenantRows, err := srv.db.Query(ctx, `SELECT DISTINCT tenant_id FROM audit_jobs WHERE status='done'`)
	if err != nil {
		return
	}
	var tenantIDs []string
	for tenantRows.Next() {
		var id string
		if err := tenantRows.Scan(&id); err == nil {
			tenantIDs = append(tenantIDs, id)
		}
	}
	tenantRows.Close()

	for _, tenantID := range tenantIDs {
		srv.checkSLAForTenant(ctx, tenantID)
	}
}

func (srv *server) checkSLAForTenant(ctx context.Context, tenantID string) {
	rules, err := srv.listSLARules(ctx, tenantID)
	if err != nil {
		return
	}
	ruleMap := map[string]SLARule{}
	for _, r := range rules {
		ruleMap[r.Severity] = r
	}

	// Get all open findings for this tenant
	allFindings, err := srv.getAllAggregatedFindings(ctx, tenantID)
	if err != nil {
		return
	}

	for _, af := range allFindings {
		if af.Status != "open" && af.Status != "in_progress" {
			continue
		}
		severity := strings.ToLower(af.Severity)
		rule, ok := ruleMap[severity]
		if !ok {
			continue
		}

		// days open = now - job.finished_at (fallback to job.started_at)
		job, err := srv.getJob(ctx, af.JobID)
		if err != nil {
			continue
		}
		openedAt := job.StartedAt
		if job.FinishedAt != nil {
			openedAt = *job.FinishedAt
		}
		daysOpen := int(time.Since(openedAt).Hours() / 24)

		if daysOpen <= rule.MaxDaysOpen {
			continue
		}

		// Create breach record (ON CONFLICT DO NOTHING prevents duplicates)
		if err := srv.createSLABreach(ctx, tenantID, af.JobID, af.ConnectionID, af.Source,
			af.Title, severity, af.FindingIndex, openedAt); err != nil {
			log.Printf("sla-checker: createSLABreach: %v", err)
			continue
		}

		// Send notification once per breach
		if rule.NotifyEmail || rule.NotifySlack {
			srv.sendSLABreachNotification(ctx, tenantID, af, daysOpen, rule)
		}
	}
}

func (srv *server) sendSLABreachNotification(ctx context.Context, tenantID string, af AggregatedFinding, daysOpen int, rule SLARule) {
	// Check if already notified
	var notifiedAt *time.Time
	_ = srv.db.QueryRow(ctx,
		`SELECT notified_at FROM finding_sla_breaches
		 WHERE tenant_id=$1 AND job_id=$2 AND source=$3 AND finding_index=$4`,
		tenantID, af.JobID, af.Source, af.FindingIndex,
	).Scan(&notifiedAt)

	if notifiedAt != nil {
		return // already notified
	}

	msg := fmt.Sprintf("SLA Breach: %s finding '%s' in %s is %d days open (limit: %d)",
		strings.ToUpper(af.Severity[:1]) + strings.ToLower(af.Severity[1:]), af.Title, af.ConnectionName, daysOpen, rule.MaxDaysOpen)

	if rule.NotifyEmail {
		user, err := srv.getUser(ctx, "")
		if err == nil && user.AuditorEmail != "" {
			go sendMonitoringAlert(user.AuditorEmail, af.ConnectionName, af.JobID, "sla_breach", msg)
		}
	}
	if rule.NotifySlack {
		slackURL := envOr("SLACK_WEBHOOK_URL", "")
		if slackURL != "" {
			go sendSlackAlert(slackURL, msg)
		}
	}

	// Mark notified
	_ = srv.db.QueryRow(ctx,
		`UPDATE finding_sla_breaches SET notified_at=NOW()
		 WHERE tenant_id=$1 AND job_id=$2 AND source=$3 AND finding_index=$4`,
		tenantID, af.JobID, af.Source, af.FindingIndex)
}

// ── Email / Slack alerts ──────────────────────────────────────────────────────

func sendMonitoringAlert(to, connectionName, jobID, alertType, message string) {
	host := envOr("SMTP_HOST", "")
	if host == "" || to == "" {
		return
	}
	appURL := envOr("APP_URL", "http://localhost:3000")
	titles := map[string]string{
		"new_findings": "🔴 New Findings Detected",
		"regression":   "🔁 Regression: Fixed Finding Reappeared",
		"sla_breach":   "⚠️ SLA Breach Alert",
	}
	title := titles[alertType]
	if title == "" {
		title = "Security Alert"
	}
	body := fmt.Sprintf(
		`<h2 style="font-family:sans-serif;color:#0d1f2d">%s</h2>`+
			`<p style="font-family:sans-serif"><strong>Connection:</strong> %s</p>`+
			`<p style="font-family:sans-serif">%s</p>`+
			`<p><a href="%s/monitoring" style="background:#9edfde;color:#0d1f2d;padding:10px 20px;border-radius:6px;`+
			`text-decoration:none;font-family:sans-serif;font-weight:bold;display:inline-block">View in Monitoring →</a></p>`,
		title, connectionName, message, appURL)
	port := envOr("SMTP_PORT", "587")
	from := envOr("SMTP_FROM", envOr("SMTP_USER", ""))
	smtpUser := envOr("SMTP_USER", "")
	smtpPass := envOr("SMTP_PASS", "")
	addr := host + ":" + port
	msg := strings.Join([]string{
		"From: " + from, "To: " + to,
		"Subject: " + title + " — " + connectionName,
		"MIME-Version: 1.0", "Content-Type: text/html; charset=UTF-8", "", body,
	}, "\r\n")
	var smtpA smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		smtpA = smtp.PlainAuth("", smtpUser, smtpPass, host)
	}
	if err := smtp.SendMail(addr, smtpA, from, []string{to}, []byte(msg)); err != nil {
		log.Printf("sendMonitoringAlert to %s: %v", to, err)
	}
	_ = jobID
}

func sendSlackAlert(webhookURL, message string) {
	if webhookURL == "" {
		return
	}
	payload := map[string]interface{}{
		"text": message,
		"attachments": []map[string]interface{}{
			{
				"color": "#ef4444",
				"text":  message,
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(b))
	if err != nil {
		log.Printf("sendSlackAlert: %v", err)
		return
	}
	_ = resp.Body.Close()
}

// ── Monitoring overview ───────────────────────────────────────────────────────

func (srv *server) buildMonitoringOverview(ctx context.Context, tenantID string) (MonitoringOverview, error) {
	var ov MonitoringOverview

	// Avg score across all connections
	_ = srv.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(sub.score)::int, 0) FROM (
		  SELECT DISTINCT ON(connection_id) score
		  FROM security_scores
		  WHERE tenant_id=$1
		  ORDER BY connection_id, calculated_at DESC
		) sub`, tenantID,
	).Scan(&ov.AvgScore)

	// SLA breaches count (open findings only)
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM finding_sla_breaches b
		LEFT JOIN finding_overrides fo ON fo.job_id=b.job_id AND fo.source=b.source AND fo.finding_index=b.finding_index
		WHERE b.tenant_id=$1 AND COALESCE(fo.status,'open') IN ('open','in_progress')`, tenantID,
	).Scan(&ov.SLABreachCount)

	// New findings this week
	_ = srv.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(new_findings_count),0) FROM audit_jobs
		WHERE tenant_id=$1 AND started_at > NOW()-INTERVAL '7 days'`, tenantID,
	).Scan(&ov.NewFindingsThisWeek)

	// Regression count this week
	_ = srv.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(regression_findings_count),0) FROM audit_jobs
		WHERE tenant_id=$1 AND started_at > NOW()-INTERVAL '7 days'`, tenantID,
	).Scan(&ov.RegressionsFindingsCount)

	// SLA breaches by severity
	bySevRows, err := srv.db.Query(ctx, `
		SELECT b.severity,
		       COUNT(*) FILTER (WHERE COALESCE(fo.status,'open') IN ('open','in_progress')) as cnt,
		       MAX(EXTRACT(EPOCH FROM (NOW()-b.breached_at))/86400)::int as oldest_days
		FROM finding_sla_breaches b
		LEFT JOIN finding_overrides fo ON fo.job_id=b.job_id AND fo.source=b.source AND fo.finding_index=b.finding_index
		WHERE b.tenant_id=$1
		GROUP BY b.severity
		ORDER BY CASE b.severity WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END`,
		tenantID)
	if err == nil {
		defer bySevRows.Close()
		for bySevRows.Next() {
			var s SLABySeverity
			if err := bySevRows.Scan(&s.Severity, &s.Count, &s.OldestDays); err == nil {
				ov.SLABreachesBySeverity = append(ov.SLABreachesBySeverity, s)
			}
		}
	}
	if ov.SLABreachesBySeverity == nil {
		ov.SLABreachesBySeverity = []SLABySeverity{}
	}

	// Score trend
	trend, err := srv.getScoreTrend(ctx, tenantID)
	if err == nil {
		ov.ScoreTrend = trend
	}
	if ov.ScoreTrend == nil {
		ov.ScoreTrend = []ScoreTrendDay{}
	}

	// Recent changes (last 14 days)
	changes, err := srv.getRecentChanges(ctx, tenantID, 14)
	if err == nil {
		ov.RecentChanges = changes
	}
	if ov.RecentChanges == nil {
		ov.RecentChanges = []FindingChange{}
	}

	return ov, nil
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func (srv *server) handleGetMonitoringOverview(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	ov, err := srv.buildMonitoringOverview(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

func (srv *server) handleGetConnectionScores(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	connID := chi.URLParam(r, "connectionId")

	scores, err := srv.getScoreHistory(r.Context(), tenantID, connID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	if scores == nil {
		scores = []SecurityScore{}
	}
	writeJSON(w, http.StatusOK, scores)
}

func (srv *server) handleGetSLABreaches(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	breaches, err := srv.listSLABreaches(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	if breaches == nil {
		breaches = []SLABreach{}
	}
	writeJSON(w, http.StatusOK, breaches)
}

func (srv *server) handleGetSLARules(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	rules, err := srv.listSLARules(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (srv *server) handleUpdateSLARules(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)

	var req []struct {
		Severity    string `json:"severity"`
		MaxDaysOpen int    `json:"max_days_open"`
		NotifyEmail bool   `json:"notify_email"`
		NotifySlack bool   `json:"notify_slack"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	var updated []SLARule
	for _, item := range req {
		r, err := srv.upsertSLARule(r.Context(), tenantID, item.Severity, item.MaxDaysOpen, item.NotifyEmail, item.NotifySlack)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
		updated = append(updated, r)
	}
	writeJSON(w, http.StatusOK, updated)
}

func (srv *server) handleGetConnectionScoresList(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	scores, err := srv.getLatestScorePerConnection(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	if scores == nil {
		scores = []SecurityScore{}
	}
	writeJSON(w, http.StatusOK, scores)
}

// loadRawFindingsFromDir loads findings JSON given a report dir path (used in monitoring context)
func loadRawFindingsFromDir(dir, source string) []map[string]interface{} {
	return loadRawFindings(filepath.Join(dir, source+".json"))
}

// unused import guard
var _ = os.Getenv
