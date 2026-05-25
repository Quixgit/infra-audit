package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── SSL / TLS audit entry point ───────────────────────────────────────────────

func (srv *server) runSSLAudit(jobID, connectionID, userID string) {
	ctx := context.Background()

	conn, err := srv.getConnection(ctx, connectionID)
	if err != nil {
		srv.updateJobFailed(ctx, jobID, "connection not found: "+err.Error())
		return
	}

	domains := parseDomains(conn.Domains)
	if len(domains) == 0 {
		srv.updateJobFailed(ctx, jobID, "no domains configured — please edit the connection and add at least one domain")
		return
	}

	dataDir := envOr("DATA_DIR", "/app/data")
	reportDir := filepath.Join(dataDir, "reports", jobID)
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		srv.updateJobFailed(ctx, jobID, "failed to create report directory: "+err.Error())
		return
	}

	srv.updateJobProgress(ctx, jobID, "running", fmt.Sprintf("Scanning SSL/TLS for %d domain(s)…", len(domains)))

	var findings []map[string]interface{}
	for i, domain := range domains {
		srv.updateJobProgress(ctx, jobID, "running",
			fmt.Sprintf("Checking %s (%d/%d)…", domain, i+1, len(domains)))
		findings = append(findings, scanSSLDomain(domain)...)
	}

	// Save findings.json
	if b, err := json.Marshal(findings); err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "findings.json"), b, 0644)
	}

	critical, high, medium, low := countFindings(findings)

	// Generate minimal HTML report
	htmlFile := filepath.Join(reportDir, "ssl_tls_audit_report.html")
	writeSimpleHTMLReport(htmlFile, conn.Name, "SSL / TLS Certificates", domains, findings)

	srv.updateJobDone(ctx, jobID, htmlFile, "", critical, high, medium, low)

	if user, err := srv.getUser(ctx, userID); err == nil {
		if user.NotifyEmail && user.AuditorEmail != "" {
			go sendJobEmail(user.AuditorEmail, conn.Name, jobID, critical, high, medium, low, false, "")
		}
	}
}

// ── Domain scanner ────────────────────────────────────────────────────────────

func scanSSLDomain(rawDomain string) []map[string]interface{} {
	// Normalise
	domain := rawDomain
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.Split(domain, "/")[0]

	host := domain
	port := "443"
	if h, p, err := net.SplitHostPort(domain); err == nil {
		host, port = h, p
	}

	addr := host + ":" + port
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	// ── Check 1: TLS connectivity & certificate validity ──────────────────────
	tlsCfg := &tls.Config{ServerName: host}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	if err != nil {
		// Retry without verification to still get cert data
		tlsCfgInsecure := &tls.Config{ServerName: host, InsecureSkipVerify: true}
		conn2, err2 := tls.DialWithDialer(dialer, "tcp", addr, tlsCfgInsecure)
		if err2 != nil {
			return []map[string]interface{}{sslFinding(
				"critical",
				"HTTPS Not Accessible",
				rawDomain,
				"Cannot establish TLS connection. The service may be down or not running HTTPS on port "+port+".",
				err.Error(),
				"Ensure HTTPS is running and a valid TLS certificate is installed.",
			)}
		}
		defer conn2.Close()
		return append([]map[string]interface{}{sslFinding(
			"critical",
			"Invalid or Untrusted TLS Certificate",
			rawDomain,
			"The TLS certificate is invalid, self-signed, or does not match the domain. Browsers will show security warnings.",
			err.Error(),
			"Install a valid certificate from a trusted CA (e.g. Let's Encrypt, DigiCert).",
		)}, sslChecks(rawDomain, host, port, conn2)...)
	}
	defer conn.Close()
	return sslChecks(rawDomain, host, port, conn)
}

// sslChecks runs all TLS checks on an already-established connection.
func sslChecks(rawDomain, host, port string, conn *tls.Conn) []map[string]interface{} {
	var findings []map[string]interface{}
	state := conn.ConnectionState()

	if len(state.PeerCertificates) == 0 {
		return findings
	}
	cert := state.PeerCertificates[0]

	// ── Cert expiry ───────────────────────────────────────────────────────────
	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	switch {
	case daysLeft < 0:
		findings = append(findings, sslFinding("critical",
			"TLS Certificate Expired",
			rawDomain,
			fmt.Sprintf("Certificate expired %d day(s) ago (%s). All visitors see browser warnings.", -daysLeft, cert.NotAfter.Format("2006-01-02")),
			fmt.Sprintf("Expired: %s | CN: %s", cert.NotAfter.Format(time.RFC3339), cert.Subject.CommonName),
			"Renew or replace the certificate immediately.",
		))
	case daysLeft < 14:
		findings = append(findings, sslFinding("critical",
			"TLS Certificate Expires in < 14 Days",
			rawDomain,
			fmt.Sprintf("Certificate expires in %d day(s) (%s). Urgent renewal required.", daysLeft, cert.NotAfter.Format("2006-01-02")),
			fmt.Sprintf("Expires: %s | CN: %s", cert.NotAfter.Format(time.RFC3339), cert.Subject.CommonName),
			"Renew immediately to prevent service disruption.",
		))
	case daysLeft < 30:
		findings = append(findings, sslFinding("high",
			"TLS Certificate Expires in < 30 Days",
			rawDomain,
			fmt.Sprintf("Certificate expires in %d day(s) (%s). Renew soon.", daysLeft, cert.NotAfter.Format("2006-01-02")),
			fmt.Sprintf("Expires: %s | CN: %s", cert.NotAfter.Format(time.RFC3339), cert.Subject.CommonName),
			"Renew the certificate before expiry to avoid user-facing errors.",
		))
	case daysLeft < 90:
		findings = append(findings, sslFinding("medium",
			"TLS Certificate Expires in < 90 Days",
			rawDomain,
			fmt.Sprintf("Certificate expires in %d day(s) (%s). Plan renewal.", daysLeft, cert.NotAfter.Format("2006-01-02")),
			fmt.Sprintf("Expires: %s | CN: %s", cert.NotAfter.Format(time.RFC3339), cert.Subject.CommonName),
			"Renew the certificate proactively before the 30-day window.",
		))
	}

	// ── TLS version ───────────────────────────────────────────────────────────
	versionNames := map[uint16]string{
		tls.VersionTLS10: "TLS 1.0",
		tls.VersionTLS11: "TLS 1.1",
		tls.VersionTLS12: "TLS 1.2",
		tls.VersionTLS13: "TLS 1.3",
	}
	switch state.Version {
	case tls.VersionTLS10:
		findings = append(findings, sslFinding("high",
			"Deprecated TLS 1.0 Negotiated",
			rawDomain,
			"TLS 1.0 has known vulnerabilities (BEAST, POODLE) and is prohibited by PCI DSS 3.2+.",
			"Negotiated version: TLS 1.0",
			"Disable TLS 1.0 and TLS 1.1 in your web server config. Allow only TLS 1.2 and TLS 1.3.",
		))
	case tls.VersionTLS11:
		findings = append(findings, sslFinding("high",
			"Deprecated TLS 1.1 Negotiated",
			rawDomain,
			"TLS 1.1 is deprecated with known cryptographic weaknesses. Most browsers no longer support it.",
			"Negotiated version: TLS 1.1",
			"Disable TLS 1.0 and TLS 1.1. Configure only TLS 1.2 and TLS 1.3.",
		))
	}
	_ = versionNames

	// ── Weak cipher suites ────────────────────────────────────────────────────
	weakCiphers := map[uint16]string{
		tls.TLS_RSA_WITH_RC4_128_SHA:         "TLS_RSA_WITH_RC4_128_SHA (RC4 — broken)",
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA:    "TLS_RSA_WITH_3DES_EDE_CBC_SHA (3DES — Sweet32)",
		tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA:   "TLS_ECDHE_RSA_WITH_RC4_128_SHA (RC4 — broken)",
		tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA: "TLS_ECDHE_ECDSA_WITH_RC4_128_SHA (RC4 — broken)",
	}
	if desc, bad := weakCiphers[state.CipherSuite]; bad {
		findings = append(findings, sslFinding("high",
			"Weak TLS Cipher Suite Negotiated",
			rawDomain,
			"The negotiated cipher suite is considered weak and vulnerable to known attacks.",
			fmt.Sprintf("Cipher: %s", desc),
			"Configure the server to prefer ECDHE+AES-GCM or ChaCha20-Poly1305 cipher suites only.",
		))
	}

	// ── HSTS & HTTP→HTTPS redirect ────────────────────────────────────────────
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}

	if resp, err := httpClient.Get("https://" + host + ":" + port + "/"); err == nil {
		defer resp.Body.Close()
		if resp.Header.Get("Strict-Transport-Security") == "" {
			findings = append(findings, sslFinding("medium",
				"Missing HSTS Header (Strict-Transport-Security)",
				rawDomain,
				"Without HSTS, browsers may connect over HTTP and are vulnerable to SSL-stripping attacks.",
				"Header 'Strict-Transport-Security' absent from HTTPS response",
				"Add: Strict-Transport-Security: max-age=31536000; includeSubDomains; preload",
			))
		}
	}

	// HTTP→HTTPS redirect check (only for standard ports)
	if port == "443" {
		if resp, err := httpClient.Get("http://" + host + "/"); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode != 301 && resp.StatusCode != 302 && resp.StatusCode != 307 && resp.StatusCode != 308 {
				findings = append(findings, sslFinding("medium",
					"HTTP Traffic Not Redirected to HTTPS",
					rawDomain,
					"The server does not redirect plain-HTTP visitors to HTTPS, leaving them unprotected.",
					fmt.Sprintf("HTTP GET / returned status %d (expected 30x redirect)", resp.StatusCode),
					"Configure a permanent (301) redirect from http:// to https:// at the web server level.",
				))
			}
		}
	}

	return findings
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func sslFinding(severity, title, domain, risk, evidence, recommendation string) map[string]interface{} {
	return map[string]interface{}{
		"severity":        severity,
		"title":           title,
		"resource_name":   domain,
		"resource_type":   "Domain",
		"category":        "SSL / TLS Configuration",
		"risk":            risk,
		"evidence":        evidence,
		"recommendation":  recommendation,
	}
}

func parseDomains(s string) []string {
	var out []string
	for _, d := range strings.Split(s, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

func countFindings(findings []map[string]interface{}) (critical, high, medium, low int) {
	for _, f := range findings {
		switch fmt.Sprintf("%v", f["severity"]) {
		case "critical":
			critical++
		case "high":
			high++
		case "medium":
			medium++
		case "low":
			low++
		}
	}
	return
}

// writeSimpleHTMLReport generates a self-contained HTML report used both as
// the downloadable artifact and as the anchor for findings.json lookup.
func writeSimpleHTMLReport(path, clientName, auditType string, domains []string, findings []map[string]interface{}) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()

	critical, high, medium, low := countFindings(findings)

	severityColor := map[string]string{
		"critical": "#ef4444",
		"high":     "#f97316",
		"medium":   "#eab308",
		"low":      "#3b82f6",
	}

	fmt.Fprintf(f, `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s — %s Security Audit</title>
<style>
  body{font-family:system-ui,sans-serif;background:#0f172a;color:#e2e8f0;margin:0;padding:24px}
  h1{font-size:1.5rem;margin-bottom:4px}
  .sub{color:#94a3b8;font-size:.9rem;margin-bottom:24px}
  .summary{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:24px}
  .box{border-radius:8px;padding:16px;text-align:center;border:1px solid}
  .num{font-size:2rem;font-weight:700}
  .label{font-size:.8rem;margin-top:4px}
  .finding{border-radius:8px;border:1px solid;padding:16px;margin-bottom:12px}
  .badge{display:inline-block;border-radius:4px;padding:2px 8px;font-size:.75rem;font-weight:600;text-transform:capitalize;border:1px solid}
  .title-row{margin:8px 0 4px}
  .section{font-size:.7rem;text-transform:uppercase;letter-spacing:.05em;color:#94a3b8;margin-top:12px;margin-bottom:4px}
  .evidence{font-family:monospace;font-size:.8rem;background:rgba(0,0,0,.3);border-radius:4px;padding:8px;white-space:pre-wrap;word-break:break-all}
</style>
</head>
<body>
<h1>%s</h1>
<div class="sub">%s · %s · Generated %s</div>
<div class="summary">
  <div class="box" style="background:rgba(239,68,68,.1);border-color:rgba(239,68,68,.3)">
    <div class="num" style="color:#ef4444">%d</div><div class="label">Critical</div></div>
  <div class="box" style="background:rgba(249,115,22,.1);border-color:rgba(249,115,22,.3)">
    <div class="num" style="color:#f97316">%d</div><div class="label">High</div></div>
  <div class="box" style="background:rgba(234,179,8,.1);border-color:rgba(234,179,8,.3)">
    <div class="num" style="color:#eab308">%d</div><div class="label">Medium</div></div>
  <div class="box" style="background:rgba(59,130,246,.1);border-color:rgba(59,130,246,.3)">
    <div class="num" style="color:#3b82f6">%d</div><div class="label">Low</div></div>
</div>
<p style="color:#94a3b8;font-size:.85rem">Domains scanned: %s</p>
`,
		htmlEsc(clientName), htmlEsc(auditType),
		htmlEsc(clientName), htmlEsc(auditType),
		strings.Join(domains, ", "),
		time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		critical, high, medium, low,
		htmlEsc(strings.Join(domains, ", ")),
	)

	for _, finding := range findings {
		sev := fmt.Sprintf("%v", finding["severity"])
		color := severityColor[sev]
		if color == "" {
			color = "#94a3b8"
		}
		fmt.Fprintf(f, `<div class="finding" style="background:%s20;border-color:%s40">
  <span class="badge" style="color:%s;border-color:%s40">%s</span>
  <div class="title-row"><strong>%s</strong></div>
  <div style="font-size:.85rem;color:#94a3b8">%s: %s</div>`,
			color, color, color, color,
			htmlEsc(sev),
			htmlEsc(fmt.Sprintf("%v", finding["title"])),
			htmlEsc(fmt.Sprintf("%v", finding["resource_type"])),
			htmlEsc(fmt.Sprintf("%v", finding["resource_name"])),
		)
		if r := fmt.Sprintf("%v", finding["risk"]); r != "" && r != "<nil>" {
			fmt.Fprintf(f, `<div class="section">Risk</div><div style="font-size:.85rem">%s</div>`, htmlEsc(r))
		}
		if e := fmt.Sprintf("%v", finding["evidence"]); e != "" && e != "<nil>" {
			fmt.Fprintf(f, `<div class="section">Evidence</div><div class="evidence">%s</div>`, htmlEsc(e))
		}
		if rec := fmt.Sprintf("%v", finding["recommendation"]); rec != "" && rec != "<nil>" {
			fmt.Fprintf(f, `<div class="section">Recommendation</div><div style="font-size:.85rem">%s</div>`, htmlEsc(rec))
		}
		fmt.Fprintln(f, `</div>`)
	}
	fmt.Fprintln(f, `</body></html>`)
}

func htmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}

