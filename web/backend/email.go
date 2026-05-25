package main

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
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
