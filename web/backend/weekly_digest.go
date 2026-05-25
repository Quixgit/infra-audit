package main

import (
	"context"
	"log"
	"time"
)

// startWeeklyDigest runs a weekly security digest email and Slack post for each tenant.
// It fires every Monday at 09:00 UTC (checked once per hour by the scheduler).
func (srv *server) startWeeklyDigest(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	log.Println("weekly-digest: scheduler started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UTC()
			// Fire on Monday, 9am UTC
			if now.Weekday() == time.Monday && now.Hour() == 9 {
				srv.runWeeklyDigests(ctx)
			}
		}
	}
}

func (srv *server) runWeeklyDigests(ctx context.Context) {
	// Get all tenants
	rows, err := srv.db.Query(ctx, `SELECT id, name, slack_webhook_url FROM tenants`)
	if err != nil {
		log.Printf("weekly-digest: list tenants: %v", err)
		return
	}
	defer rows.Close()

	type tenantRow struct {
		ID              string
		Name            string
		SlackWebhookURL string
	}
	var tenants []tenantRow
	for rows.Next() {
		var t tenantRow
		if err := rows.Scan(&t.ID, &t.Name, &t.SlackWebhookURL); err == nil {
			tenants = append(tenants, t)
		}
	}
	_ = rows.Err()

	for _, t := range tenants {
		// Check if already sent this week
		var lastSent *time.Time
		_ = srv.db.QueryRow(ctx,
			`SELECT sent_at FROM digest_log WHERE tenant_id=$1 ORDER BY sent_at DESC LIMIT 1`,
			t.ID,
		).Scan(&lastSent)
		if lastSent != nil && time.Since(*lastSent) < 6*24*time.Hour {
			continue // already sent this week
		}

		stats := srv.computeDigestStats(ctx, t.ID, t.Name)

		// Email digest to all tenant owners/admins who have notify_email=true
		ownerRows, err := srv.db.Query(ctx, `
			SELECT u.auditor_email, u.email
			FROM tenant_members tm
			JOIN users u ON u.id = tm.user_id
			WHERE tm.tenant_id=$1 AND tm.role IN ('owner','admin') AND u.notify_email=true`, t.ID)
		if err == nil {
			defer ownerRows.Close()
			for ownerRows.Next() {
				var auditorEmail, email string
				_ = ownerRows.Scan(&auditorEmail, &email)
				to := auditorEmail
				if to == "" {
					to = email
				}
				if to != "" {
					go sendWeeklyDigestEmail(to, stats)
				}
			}
		}

		// Slack digest
		if t.SlackWebhookURL != "" {
			go srv.sendSlackWeeklyDigest(ctx, t.SlackWebhookURL, stats)
		}

		// Record sent
		_, _ = srv.db.Exec(ctx, `INSERT INTO digest_log(tenant_id) VALUES($1)`, t.ID)
		log.Printf("weekly-digest: sent for tenant %s (%s)", t.ID, t.Name)
	}
}

func (srv *server) computeDigestStats(ctx context.Context, tenantID, tenantName string) DigestStats {
	stats := DigestStats{TenantName: tenantName}

	// Average security score
	_ = srv.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(score)::INT, 0)
		FROM security_scores
		WHERE tenant_id=$1 AND calculated_at > NOW() - INTERVAL '7 days'`, tenantID,
	).Scan(&stats.AvgScore)

	// Score delta vs last week
	var prevScore int
	_ = srv.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(score)::INT, 0)
		FROM security_scores
		WHERE tenant_id=$1 AND calculated_at BETWEEN NOW() - INTERVAL '14 days' AND NOW() - INTERVAL '7 days'`, tenantID,
	).Scan(&prevScore)
	if prevScore > 0 {
		stats.ScoreDelta = stats.AvgScore - prevScore
	}

	// New critical/high this week
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM finding_changes
		WHERE tenant_id=$1 AND severity='critical' AND change_type='new' AND occurred_at > NOW() - INTERVAL '7 days'`,
		tenantID).Scan(&stats.NewCritical)
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM finding_changes
		WHERE tenant_id=$1 AND severity='high' AND change_type='new' AND occurred_at > NOW() - INTERVAL '7 days'`,
		tenantID).Scan(&stats.NewHigh)

	// Total open findings
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM finding_overrides fo
		WHERE fo.tenant_id=$1 AND fo.status = 'open'`, tenantID,
	).Scan(&stats.TotalOpen)

	// Remediation done this week
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM remediation_tasks
		WHERE tenant_id=$1 AND lane='done' AND updated_at > NOW() - INTERVAL '7 days'`, tenantID,
	).Scan(&stats.RemediationDone)

	// Remediation open
	_ = srv.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM remediation_tasks
		WHERE tenant_id=$1 AND lane != 'done'`, tenantID,
	).Scan(&stats.RemediationOpen)

	return stats
}

func (srv *server) sendSlackWeeklyDigest(ctx context.Context, webhookURL string, stats DigestStats) {
	if webhookURL == "" {
		return
	}
	appURL := envOr("APP_URL", "http://localhost:3000")
	scoreEmoji := ":green_circle:"
	if stats.AvgScore < 70 {
		scoreEmoji = ":yellow_circle:"
	}
	if stats.AvgScore < 50 {
		scoreEmoji = ":red_circle:"
	}

	deltaStr := ""
	if stats.ScoreDelta > 0 {
		deltaStr = " ↑ " + itoa(stats.ScoreDelta)
	} else if stats.ScoreDelta < 0 {
		deltaStr = " ↓ " + itoa(-stats.ScoreDelta)
	}

	text := "*🔒 Weekly Security Digest — " + stats.TenantName + "*\n" +
		scoreEmoji + " *Security Score:* " + itoa(stats.AvgScore) + deltaStr + "\n" +
		":rotating_light: New Critical: " + itoa(stats.NewCritical) +
		"  :warning: New High: " + itoa(stats.NewHigh) + "\n" +
		":clipboard: Open Findings: " + itoa(stats.TotalOpen) +
		"  :white_check_mark: Remediated: " + itoa(stats.RemediationDone) + " this week\n" +
		"<" + appURL + "/monitoring|View Full Dashboard →>"

	sendSlack(webhookURL, text)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
