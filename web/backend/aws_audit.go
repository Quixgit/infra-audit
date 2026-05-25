package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	awsrules "infra-audit/internal/rules"
	awsscanner "infra-audit/internal/scanner/aws"
	"infra-audit/internal/model"
	"infra-audit/internal/report"
	"infra-audit/internal/util"
)

func (srv *server) runAWSAudit(jobID, connectionID, userID string) {
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

	// Decrypt credentials
	accessKey, err := decryptToken(conn.AWSAccessKeyID)
	if err != nil {
		srv.updateJobFailed(ctx, jobID, "decrypt access key: "+err.Error())
		return
	}
	secretKey, err := decryptToken(conn.AWSSecretKey)
	if err != nil {
		srv.updateJobFailed(ctx, jobID, "decrypt secret key: "+err.Error())
		return
	}

	region := conn.AWSRegion
	if region == "" {
		region = "us-east-1"
	}

	srv.updateJobProgress(ctx, jobID, "running", fmt.Sprintf("Connecting to AWS (%s)...", region))

	inv, err := awsscanner.Scan(conn.Name, awsscanner.ScanOptions{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Region:          region,
	})
	if err != nil {
		srv.updateJobFailed(ctx, jobID, "AWS scan failed: "+err.Error())
		return
	}

	// Check for auth errors
	for _, e := range inv.Errors {
		if strings.Contains(e, "NoCredentialProviders") || strings.Contains(e, "InvalidClientTokenId") || strings.Contains(e, "AuthFailure") {
			srv.updateJobFailed(ctx, jobID, "AWS authentication failed — check Access Key ID and Secret Access Key")
			return
		}
	}

	srv.updateJobProgress(ctx, jobID, "running", "Evaluating AWS security rules...")

	findings := awsrules.EvaluateAWS(inv)
	positives := awsrules.AWSPositiveFindings(inv)

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

	// Save inventory and findings JSON
	if b, err := json.Marshal(inv); err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "aws_inventory.json"), b, 0644)
	}
	if b, err := json.Marshal(findings); err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "findings.json"), b, 0644)
	}

	srv.updateJobProgress(ctx, jobID, "running", "Generating AWS security report...")

	assetsDir := filepath.Join(reportDir, "assets")
	_ = os.MkdirAll(assetsDir, 0755)

	logoRel := resolveAsset(userID, "logo.png", assetsDir)
	watermarkRel := resolveAsset(userID, "watermark.png", assetsDir)
	footerBgRel := resolveAsset(userID, "footer-bg.png", assetsDir)
	for _, icon := range []string{"icon-email.png", "icon-phone.png", "icon-web.png"} {
		resolveAsset(userID, icon, assetsDir)
	}

	base := util.SafeName(conn.Name + "_aws_security_audit")
	now := time.Now().UTC()

	r := model.Report{
		Meta: model.ReportMeta{
			Client:           conn.Name,
			ProjectName:      fmt.Sprintf("AWS %s Security Assessment", region),
			Provider:         "AWS",
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
				"CIS AWS Foundations Benchmark v1.5",
				"NIST Cybersecurity Framework",
				"ISO/IEC 27001:2022",
				"SOC 2 Type II",
			},
		},
		Inventory: inv,
		Findings:  findings,
		Positives: positives,
		Limitations: []string{
			"Scan limited to region: " + region,
			"IAM inline policies and complex permission chains require manual review",
			"CloudTrail and GuardDuty status require additional permissions",
		},
	}

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
		// Non-fatal
		_ = err
	}

	srv.updateJobDone(ctx, jobID, htmlFile, docxFile, critical, high, medium, low)

	if completedJob, err := srv.getJob(ctx, jobID); err == nil {
		go srv.postJobMonitoring(context.Background(), completedJob)
	}
	go srv.autoCollectEvidence(context.Background(), jobID, userID, reportDir, htmlFile, docxFile, "aws")

	if user.NotifyEmail && user.AuditorEmail != "" {
		go sendJobEmail(user.AuditorEmail, conn.Name, jobID, critical, high, medium, low, false, "")
	}

	// Slack notification
	go srv.notifySlackJobComplete(ctx, conn.TenantID, conn.Name, jobID, critical, high, medium, low, false)
}
