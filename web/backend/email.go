package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

func sendJobEmail(to, connectionName, jobID string, critical, high, medium, low int, failed bool, errMsg string) {
	host := envOr("SMTP_HOST", "")
	if host == "" || to == "" {
		return
	}
	port := envOr("SMTP_PORT", "587")
	from := envOr("SMTP_FROM", envOr("SMTP_USER", ""))
	user := envOr("SMTP_USER", "")
	pass := envOr("SMTP_PASS", "")
	appURL := envOr("APP_URL", "http://localhost:3000")

	var subject, body string
	if failed {
		subject = fmt.Sprintf("CloudSecGuard: Audit Failed — %s", connectionName)
		body = fmt.Sprintf(
			`<h2 style="font-family:sans-serif">Audit Failed: %s</h2><p style="font-family:sans-serif">%s</p>`+
				`<p><a href="%s/jobs/%s" style="font-family:sans-serif">View details →</a></p>`,
			connectionName, errMsg, appURL, jobID)
	} else {
		subject = fmt.Sprintf("CloudSecGuard: Audit Complete — %s", connectionName)
		total := critical + high + medium + low
		body = fmt.Sprintf(`
<h2 style="font-family:sans-serif;color:#0d1f2d">Audit Complete: %s</h2>
<table style="font-family:sans-serif;border-collapse:collapse;margin:16px 0">
<tr>
  <td style="padding:10px 18px;background:#ef4444;color:#fff;border-radius:6px 0 0 6px;font-weight:bold">Critical: %d</td>
  <td style="padding:10px 18px;background:#f97316;color:#fff;font-weight:bold">High: %d</td>
  <td style="padding:10px 18px;background:#eab308;color:#fff;font-weight:bold">Medium: %d</td>
  <td style="padding:10px 18px;background:#3b82f6;color:#fff;border-radius:0 6px 6px 0;font-weight:bold">Low: %d</td>
</tr>
</table>
<p style="font-family:sans-serif">Total findings: <strong>%d</strong></p>
<p><a href="%s/jobs/%s" style="background:#9edfde;color:#0d1f2d;padding:10px 20px;border-radius:6px;text-decoration:none;font-family:sans-serif;font-weight:bold;display:inline-block;margin-top:8px">View Report →</a></p>`,
			connectionName, critical, high, medium, low, total, appURL, jobID)
	}

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := host + ":" + port
	var auth smtp.Auth
	if user != "" && pass != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		log.Printf("sendJobEmail to %s: %v", to, err)
	}
}

// ── Slack notifications ───────────────────────────────────────────────────────

func sendSlack(webhookURL, text string, blocks ...map[string]interface{}) {
	if webhookURL == "" {
		return
	}
	payload := map[string]interface{}{"text": text}
	if len(blocks) > 0 {
		payload["blocks"] = blocks
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(b))
	if err != nil {
		log.Printf("slack notify: %v", err)
		return
	}
	resp.Body.Close()
}

func (srv *server) notifySlackJobComplete(ctx context.Context, tenantID, connName, jobID string, critical, high, medium, low int, failed bool) {
	ws, err := srv.getTenant(ctx, tenantID)
	if err != nil || ws.SlackWebhookURL == "" {
		return
	}
	appURL := envOr("APP_URL", "http://localhost:3000")
	total := critical + high + medium + low
	emoji := ":white_check_mark:"
	if failed {
		emoji = ":x:"
	} else if critical > 0 {
		emoji = ":rotating_light:"
	} else if high > 0 {
		emoji = ":warning:"
	}
	var text string
	if failed {
		text = fmt.Sprintf("%s *Audit Failed* — %s\nCheck the dashboard for details.", emoji, connName)
	} else {
		text = fmt.Sprintf("%s *Audit Complete* — %s\n:red_circle: Critical: %d  :orange_circle: High: %d  :yellow_circle: Medium: %d  :blue_circle: Low: %d\nTotal: %d findings\n<%s/jobs/%s|View Report →>",
			emoji, connName, critical, high, medium, low, total, appURL, jobID)
	}
	sendSlack(ws.SlackWebhookURL, text)
}

func (srv *server) notifySlackCriticalFinding(ctx context.Context, tenantID, connName, findingTitle string) {
	ws, err := srv.getTenant(ctx, tenantID)
	if err != nil || ws.SlackWebhookURL == "" {
		return
	}
	text := fmt.Sprintf(":rotating_light: *Critical Finding Detected* in %s\n_%s_\nLogin to your dashboard to review and remediate.", connName, findingTitle)
	sendSlack(ws.SlackWebhookURL, text)
}

// ── Weekly security digest email ──────────────────────────────────────────────

type DigestStats struct {
	TenantName      string
	AvgScore        int
	ScoreDelta      int
	NewCritical     int
	NewHigh         int
	TotalOpen       int
	RemediationDone int
	RemediationOpen int
	TopConnections  []string
}

func sendWeeklyDigestEmail(to string, stats DigestStats) {
	host := envOr("SMTP_HOST", "")
	if host == "" || to == "" {
		return
	}
	port := envOr("SMTP_PORT", "587")
	from := envOr("SMTP_FROM", envOr("SMTP_USER", ""))
	user := envOr("SMTP_USER", "")
	pass := envOr("SMTP_PASS", "")
	appURL := envOr("APP_URL", "http://localhost:3000")

	scoreDeltaStr := ""
	if stats.ScoreDelta > 0 {
		scoreDeltaStr = fmt.Sprintf(`<span style="color:#22c55e">↑ %d</span>`, stats.ScoreDelta)
	} else if stats.ScoreDelta < 0 {
		scoreDeltaStr = fmt.Sprintf(`<span style="color:#ef4444">↓ %d</span>`, -stats.ScoreDelta)
	}

	body := fmt.Sprintf(`
<div style="font-family:sans-serif;max-width:600px;margin:0 auto;background:#0d1f2d;color:#e2e8f0;padding:24px;border-radius:12px">
  <h2 style="color:#9edfde;margin:0 0 16px">🔒 Weekly Security Digest</h2>
  <p style="color:#94a3b8;margin:0 0 24px">%s — week ending %s</p>

  <div style="background:#1e3a4c;border-radius:8px;padding:20px;margin-bottom:16px">
    <h3 style="margin:0 0 12px;color:#e2e8f0">Security Score</h3>
    <div style="font-size:36px;font-weight:bold;color:#9edfde">%d %s</div>
  </div>

  <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
    <div style="background:#1e3a4c;border-radius:8px;padding:16px">
      <p style="margin:0;color:#94a3b8;font-size:12px">New Critical</p>
      <p style="margin:4px 0 0;font-size:24px;font-weight:bold;color:#ef4444">%d</p>
    </div>
    <div style="background:#1e3a4c;border-radius:8px;padding:16px">
      <p style="margin:0;color:#94a3b8;font-size:12px">New High</p>
      <p style="margin:4px 0 0;font-size:24px;font-weight:bold;color:#f97316">%d</p>
    </div>
    <div style="background:#1e3a4c;border-radius:8px;padding:16px">
      <p style="margin:0;color:#94a3b8;font-size:12px">Total Open Findings</p>
      <p style="margin:4px 0 0;font-size:24px;font-weight:bold;color:#e2e8f0">%d</p>
    </div>
    <div style="background:#1e3a4c;border-radius:8px;padding:16px">
      <p style="margin:0;color:#94a3b8;font-size:12px">Remediation Done</p>
      <p style="margin:4px 0 0;font-size:24px;font-weight:bold;color:#22c55e">%d</p>
    </div>
  </div>

  <p style="text-align:center">
    <a href="%s/monitoring" style="background:#9edfde;color:#0d1f2d;padding:12px 24px;border-radius:8px;text-decoration:none;font-weight:bold;display:inline-block">
      View Full Report →
    </a>
  </p>
</div>`,
		stats.TenantName,
		time.Now().Format("2006-01-02"),
		stats.AvgScore, scoreDeltaStr,
		stats.NewCritical,
		stats.NewHigh,
		stats.TotalOpen,
		stats.RemediationDone,
		appURL,
	)

	subject := fmt.Sprintf("InfraJump Weekly Digest — Score: %d", stats.AvgScore)
	if stats.NewCritical > 0 {
		subject = fmt.Sprintf("🚨 InfraJump Weekly Digest — %d New Critical Findings", stats.NewCritical)
	}

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := host + ":" + port
	var auth smtp.Auth
	if user != "" && pass != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		log.Printf("sendWeeklyDigestEmail to %s: %v", to, err)
	}
}

