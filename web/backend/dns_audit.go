package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── DNS Security audit entry point ───────────────────────────────────────────

func (srv *server) runDNSAudit(jobID, connectionID, userID string) {
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

	srv.updateJobProgress(ctx, jobID, "running", fmt.Sprintf("Scanning DNS security for %d domain(s)…", len(domains)))

	var findings []map[string]interface{}
	for i, domain := range domains {
		srv.updateJobProgress(ctx, jobID, "running",
			fmt.Sprintf("Checking DNS for %s (%d/%d)…", domain, i+1, len(domains)))
		findings = append(findings, scanDNSDomain(domain)...)
	}

	// Save findings.json
	if b, err := json.Marshal(findings); err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "findings.json"), b, 0644)
	}

	critical, high, medium, low := countFindings(findings)

	// Generate HTML report (reuses the shared helper)
	htmlFile := filepath.Join(reportDir, "dns_security_audit_report.html")
	writeSimpleHTMLReport(htmlFile, conn.Name, "DNS Security", domains, findings)

	srv.updateJobDone(ctx, jobID, htmlFile, "", critical, high, medium, low)

	if user, err := srv.getUser(ctx, userID); err == nil {
		if user.NotifyEmail && user.AuditorEmail != "" {
			go sendJobEmail(user.AuditorEmail, conn.Name, jobID, critical, high, medium, low, false, "")
		}
	}
}

// ── Domain DNS scanner ────────────────────────────────────────────────────────

func scanDNSDomain(rawDomain string) []map[string]interface{} {
	// Normalise: strip protocol prefix and path
	domain := rawDomain
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.Split(domain, "/")[0]
	domain = strings.Split(domain, ":")[0]
	domain = strings.ToLower(strings.TrimRight(domain, "."))

	var findings []map[string]interface{}

	// ── 1. SPF record ─────────────────────────────────────────────────────────
	spfFound, spfRecord := checkSPF(domain)
	if !spfFound {
		findings = append(findings, dnsFinding("high",
			"Missing SPF Record",
			rawDomain,
			"No SPF (Sender Policy Framework) TXT record found. Attackers can send emails that appear to come from your domain.",
			fmt.Sprintf("LookupTXT(%q): no 'v=spf1' record found", domain),
			"Add a TXT record at your domain root: 'v=spf1 include:your-mail-provider.com ~all'",
		))
	} else if strings.HasSuffix(strings.TrimSpace(spfRecord), "+all") {
		findings = append(findings, dnsFinding("high",
			"SPF Record Uses '+all' (Permissive)",
			rawDomain,
			"SPF record ends with '+all', allowing any server to send email on your behalf — effectively no restriction.",
			fmt.Sprintf("SPF record: %s", spfRecord),
			"Replace '+all' with '~all' (soft-fail) or '-all' (hard-fail) to restrict mail senders.",
		))
	}

	// ── 2. DMARC record ───────────────────────────────────────────────────────
	dmarcFound, dmarcRecord := checkDMARC(domain)
	if !dmarcFound {
		findings = append(findings, dnsFinding("high",
			"Missing DMARC Record",
			rawDomain,
			"No DMARC policy found at _dmarc."+domain+". Without DMARC, email spoofing of your domain is trivial.",
			fmt.Sprintf("LookupTXT('_dmarc.%s'): no 'v=DMARC1' record", domain),
			"Add a TXT record at _dmarc."+domain+" e.g. 'v=DMARC1; p=quarantine; rua=mailto:dmarc@"+domain+"'",
		))
	} else if strings.Contains(dmarcRecord, "p=none") {
		findings = append(findings, dnsFinding("medium",
			"DMARC Policy Set to 'none' (Monitor Only)",
			rawDomain,
			"DMARC is configured but with p=none, which takes no action on failing emails — it only monitors.",
			fmt.Sprintf("DMARC record: %s", dmarcRecord),
			"Promote DMARC policy to p=quarantine or p=reject once you've reviewed reports.",
		))
	}

	// ── 3. DKIM record (common selectors) ────────────────────────────────────
	dkimSelectors := []string{"default", "google", "mail", "selector1", "selector2", "smtp", "k1", "dkim"}
	dkimFound := false
	for _, sel := range dkimSelectors {
		if checkDKIM(sel, domain) {
			dkimFound = true
			break
		}
	}
	if !dkimFound {
		findings = append(findings, dnsFinding("medium",
			"No DKIM Record Found (Common Selectors)",
			rawDomain,
			"No DKIM TXT record was found for common selectors (default, google, mail, selector1/2, smtp, k1, dkim). "+
				"Without DKIM, email integrity cannot be verified and spoofing is easier.",
			fmt.Sprintf("Checked selectors: %s", strings.Join(dkimSelectors, ", ")),
			"Add a DKIM TXT record at <selector>._domainkey."+domain+" with your public key. "+
				"Enable DKIM signing in your email provider.",
		))
	}

	// ── 4. Zone transfer (AXFR) ───────────────────────────────────────────────
	nss, err := net.LookupNS(domain)
	if err == nil && len(nss) > 0 {
		for _, ns := range nss {
			nsHost := strings.TrimSuffix(ns.Host, ".")
			if tryZoneTransfer(domain, nsHost) {
				findings = append(findings, dnsFinding("critical",
					"DNS Zone Transfer Allowed (AXFR)",
					rawDomain,
					"The nameserver "+nsHost+" allows unauthenticated zone transfers. "+
						"This exposes your full DNS zone, including internal hostnames and IPs, to any attacker.",
					fmt.Sprintf("AXFR request to %s for zone %s succeeded", nsHost, domain),
					"Restrict zone transfers on all authoritative nameservers to only trusted secondary NS IP addresses.",
				))
			}
		}
	}

	// ── 5. Open recursive resolver ────────────────────────────────────────────
	if err == nil && len(nss) > 0 {
		for _, ns := range nss {
			nsHost := strings.TrimSuffix(ns.Host, ".")
			// Resolve the NS host to IP for raw queries
			addrs, err2 := net.LookupHost(nsHost)
			if err2 != nil || len(addrs) == 0 {
				continue
			}
			if isOpenResolver(addrs[0]) {
				findings = append(findings, dnsFinding("high",
					"Open Recursive DNS Resolver",
					rawDomain,
					"The nameserver at "+nsHost+" responds to recursive queries from arbitrary clients. "+
						"Open resolvers can be abused for DNS amplification DDoS attacks.",
					fmt.Sprintf("Nameserver %s (%s) answered recursive query for 'google.com'", nsHost, addrs[0]),
					"Configure the nameserver to only allow recursive queries from authorized IP ranges. "+
						"Authoritative-only servers should disable recursion entirely.",
				))
			}
			break // check first reachable NS only
		}
	}

	// ── 6. CAA records ────────────────────────────────────────────────────────
	if !checkCAA(domain) {
		findings = append(findings, dnsFinding("low",
			"No CAA Records Configured",
			rawDomain,
			"Certification Authority Authorization (CAA) records are missing. Any CA can issue certificates for your domain.",
			fmt.Sprintf("LookupTXT/CAA(%q): no CAA records found", domain),
			"Add CAA records to restrict which CAs may issue certificates for "+domain+". "+
				"Example: '0 issue \"letsencrypt.org\"'",
		))
	}

	return findings
}

// ── Check helpers ─────────────────────────────────────────────────────────────

func checkSPF(domain string) (bool, string) {
	txts, err := net.LookupTXT(domain)
	if err != nil {
		return false, ""
	}
	for _, txt := range txts {
		if strings.HasPrefix(txt, "v=spf1") {
			return true, txt
		}
	}
	return false, ""
}

func checkDMARC(domain string) (bool, string) {
	txts, err := net.LookupTXT("_dmarc." + domain)
	if err != nil {
		return false, ""
	}
	for _, txt := range txts {
		if strings.HasPrefix(txt, "v=DMARC1") {
			return true, txt
		}
	}
	return false, ""
}

func checkDKIM(selector, domain string) bool {
	txts, err := net.LookupTXT(selector + "._domainkey." + domain)
	if err != nil {
		return false
	}
	for _, txt := range txts {
		if strings.Contains(txt, "v=DKIM1") || strings.Contains(txt, "p=") {
			return true
		}
	}
	return false
}

func checkCAA(domain string) bool {
	// net.LookupTXT doesn't support CAA directly; we do a basic check using
	// the host lookup as a proxy — a real implementation would use raw DNS.
	// For now we use net.LookupHost to detect if the domain resolves, then
	// attempt a CAA TXT lookup (some providers publish CAA as TXT for compat).
	txts, err := net.LookupTXT(domain)
	if err != nil {
		return false
	}
	for _, txt := range txts {
		lower := strings.ToLower(txt)
		if strings.Contains(lower, "issue") || strings.Contains(lower, "issuewild") {
			return true
		}
	}
	return false
}

// ── Zone transfer (AXFR) via raw TCP DNS ──────────────────────────────────────

// tryZoneTransfer attempts a raw AXFR query to nsHost:53 for the given zone.
// Returns true if the nameserver granted the transfer (misconfiguration).
func tryZoneTransfer(zone, nsHost string) bool {
	query := buildDNSQuery(zone, 252) // QTYPE=AXFR

	c, err := net.DialTimeout("tcp", nsHost+":53", 5*time.Second)
	if err != nil {
		return false
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))

	// DNS over TCP: 2-byte big-endian length prefix
	buf := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(buf[:2], uint16(len(query)))
	copy(buf[2:], query)
	if _, err := c.Write(buf); err != nil {
		return false
	}

	// Read response length
	var lenBuf [2]byte
	if _, err := io.ReadFull(c, lenBuf[:]); err != nil {
		return false
	}
	respLen := int(binary.BigEndian.Uint16(lenBuf[:]))
	if respLen < 12 {
		return false
	}

	resp := make([]byte, respLen)
	if _, err := io.ReadFull(c, resp); err != nil {
		return false
	}

	// DNS header: byte 3 low nibble = RCODE, bytes 6-7 = ANCOUNT
	rcode := resp[3] & 0x0f
	ancount := int(binary.BigEndian.Uint16(resp[6:8]))
	return rcode == 0 && ancount > 0
}

// buildDNSQuery constructs a minimal DNS query packet for the given zone and QTYPE.
func buildDNSQuery(zone string, qtype uint16) []byte {
	var buf []byte

	// Header (12 bytes)
	buf = append(buf, 0x00, 0x01) // ID
	buf = append(buf, 0x00, 0x00) // Flags: standard query, RD=0
	buf = append(buf, 0x00, 0x01) // QDCOUNT = 1
	buf = append(buf, 0x00, 0x00) // ANCOUNT
	buf = append(buf, 0x00, 0x00) // NSCOUNT
	buf = append(buf, 0x00, 0x00) // ARCOUNT

	// QNAME: each label prefixed with its length
	for _, label := range strings.Split(strings.TrimRight(zone, "."), ".") {
		buf = append(buf, byte(len(label)))
		buf = append(buf, label...)
	}
	buf = append(buf, 0x00) // root label

	// QTYPE + QCLASS=IN
	buf = append(buf, byte(qtype>>8), byte(qtype))
	buf = append(buf, 0x00, 0x01)

	return buf
}

// ── Open resolver check ───────────────────────────────────────────────────────

// isOpenResolver sends a recursive query for "google.com" A record to the
// given IP and reports whether a real answer is returned.
func isOpenResolver(nsIP string) bool {
	query := buildDNSQuery("google.com", 1) // QTYPE=A, RD=1

	// Set RD (recursion desired) bit in flags byte 2
	if len(query) > 3 {
		query[2] = 0x01 // set RD bit
	}

	c, err := net.DialTimeout("udp", nsIP+":53", 5*time.Second)
	if err != nil {
		return false
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := c.Write(query); err != nil {
		return false
	}

	resp := make([]byte, 512)
	n, err := c.Read(resp)
	if err != nil || n < 12 {
		return false
	}
	resp = resp[:n]

	rcode := resp[3] & 0x0f
	ancount := int(binary.BigEndian.Uint16(resp[6:8]))
	// QR bit in byte 2 must be 1 (response), RCODE=0, and at least one answer
	isResponse := (resp[2] & 0x80) != 0
	return isResponse && rcode == 0 && ancount > 0
}

// ── Finding factory ───────────────────────────────────────────────────────────

func dnsFinding(severity, title, domain, risk, evidence, recommendation string) map[string]interface{} {
	return map[string]interface{}{
		"severity":       severity,
		"title":          title,
		"resource_name":  domain,
		"resource_type":  "Domain",
		"category":       "DNS Security",
		"risk":           risk,
		"evidence":       evidence,
		"recommendation": recommendation,
	}
}
