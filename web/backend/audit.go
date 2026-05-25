package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"infra-audit/internal/model"
	"infra-audit/internal/report"
	"infra-audit/internal/rules"
	"infra-audit/internal/scanner"
	"infra-audit/internal/util"
)

func (srv *server) runAudit(jobID, connectionID, userID string) {
	ctx := context.Background()
	dataDir := envOr("DATA_DIR", "/app/data")
	reportDir := filepath.Join(dataDir, "reports", jobID)

	if err := os.MkdirAll(reportDir, 0755); err != nil {
		srv.updateJobFailed(ctx, jobID, "failed to create report directory: "+err.Error())
		return
	}

	conn, err := srv.getConnection(ctx, connectionID)
	if err != nil {
		srv.updateJobFailed(ctx, jobID, "connection not found: "+err.Error())
		return
	}

	user, err := srv.getUser(ctx, userID)
	if err != nil {
		srv.updateJobFailed(ctx, jobID, "user not found: "+err.Error())
		return
	}

	srv.updateJobProgress(ctx, jobID, "running", "Connecting to DigitalOcean...")

	inv, err := scanner.ScanDigitalOceanWithOptions(conn.Name, conn.DOToken, scanner.DOScanOptions{
		ProjectID:     conn.ProjectID,
		ScopeMode:     conn.ScopeMode,
		SpacesBuckets: conn.SpacesBuckets,
	})
	if err != nil {
		srv.updateJobFailed(ctx, jobID, "scan failed: "+err.Error())
		return
	}

	if hasAuthErr(inv.Errors) {
		srv.updateJobFailed(ctx, jobID, "DigitalOcean authentication failed — check DO token")
		return
	}

	srv.updateJobProgress(ctx, jobID, "running", "Evaluating security rules...")

	findings := rules.Evaluate(inv)
	positives := rules.PositiveFindings(inv)
	limitations := rules.Limitations(inv)

	var critical, high, medium, low int
	for _, f := range findings {
		switch strings.ToLower(f.Severity) {
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

	// Save inventory and findings JSON for in-browser viewer and evidence collection.
	if b, err := json.Marshal(inv); err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "do_inventory.json"), b, 0644)
	}
	if b, err := json.Marshal(findings); err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "findings.json"), b, 0644)
	}

	srv.updateJobProgress(ctx, jobID, "running", "Generating reports...")

	assetsDir := filepath.Join(reportDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		srv.updateJobFailed(ctx, jobID, "failed to create assets dir: "+err.Error())
		return
	}

	logoRel := resolveAsset(userID, "logo.png", assetsDir)
	watermarkRel := resolveAsset(userID, "watermark.png", assetsDir)
	footerBgRel := resolveAsset(userID, "footer-bg.png", assetsDir)
	for _, icon := range []string{"icon-email.png", "icon-phone.png", "icon-web.png"} {
		resolveAsset(userID, icon, assetsDir)
	}

	base := util.SafeName(conn.Name + "_infrastructure_security_audit")
	now := time.Now().UTC()

	r := model.Report{
		Meta: model.ReportMeta{
			Client:           conn.Name,
			ProjectName:      "Infrastructure Environment",
			Provider:         "DigitalOcean",
			PreparedBy:       user.PreparedBy,
			AuditorOrg:       user.AuditorOrg,
			AuditorAddress:   user.AuditorAddress,
			AuditorEmail:     user.AuditorEmail,
			AuditorWebsite:   user.AuditorWebsite,
			AuditorPhone:     user.AuditorPhone,
			Classification:   "Confidential",
			AssessmentPeriod: "Point-in-time automated assessment",
			GeneratedAt:      now.Format(time.RFC3339),
			ReportDate:       now.Format("2006-01-02"),
			Version:          "1.0",
			ArtifactBase:     base,
			LogoPath:         logoRel,
			WatermarkPath:    watermarkRel,
			FooterBgPath:     footerBgRel,
			Standards: []string{
				"ISO/IEC 27001:2022",
				"NIST Cybersecurity Framework",
				"CIS Critical Security Controls",
				"CIS Kubernetes Benchmark, where applicable",
			},
		},
		Inventory:   inv,
		Findings:    findings,
		Positives:   positives,
		Limitations: limitations,
	}
	r.Summary = rules.SummaryText(r)

	htmlFile := filepath.Join(reportDir, base+".html")
	docxFile := filepath.Join(reportDir, base+".docx")

	if err := report.WriteHTML(htmlFile, r); err != nil {
		srv.updateJobFailed(ctx, jobID, "HTML generation failed: "+err.Error())
		return
	}

	if err := report.WriteDOCX(docxFile, r); err != nil {
		srv.updateJobFailed(ctx, jobID, "DOCX generation failed: "+err.Error())
		return
	}

	if err := report.ApplyDOCXBranding(docxFile, reportDir, r.Meta); err != nil {
		log.Printf("WARN: ApplyDOCXBranding: %v", err)
	}

	srv.updateJobDone(ctx, jobID, htmlFile, docxFile, critical, high, medium, low)

	// Fetch completed job for monitoring (needs TenantID, ConnectionID etc.)
	if completedJob, err := srv.getJob(ctx, jobID); err == nil {
		go srv.postJobMonitoring(context.Background(), completedJob)
	}

	go srv.autoCollectEvidence(context.Background(), jobID, userID, reportDir, htmlFile, docxFile, "do")

	if user.NotifyEmail && user.AuditorEmail != "" {
		go sendJobEmail(user.AuditorEmail, conn.Name, jobID, critical, high, medium, low, false, "")
	}
}

func hasAuthErr(errs []string) bool {
	for _, e := range errs {
		if strings.Contains(e, "HTTP 401") || strings.Contains(e, "Unauthorized") {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
