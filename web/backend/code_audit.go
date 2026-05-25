package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"infra-audit/internal/model"
	"infra-audit/internal/report"
	codescanner "infra-audit/internal/scanner/code"
	"infra-audit/internal/util"
)

func (srv *server) runCodeAudit(jobID, connectionID, userID string) {
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

	var scanPath string

	if conn.RepoSource == "git" {
		clonePath := filepath.Join(dataDir, "repos", jobID)
		defer os.RemoveAll(clonePath)

		srv.updateJobProgress(ctx, jobID, "running", "Cloning repository...")

		token, err := decryptToken(conn.RepoToken)
		if err != nil {
			srv.updateJobFailed(ctx, jobID, "failed to decrypt token: "+err.Error())
			return
		}
		authURL, err := buildAuthURL(conn.RepoURL, token)
		if err != nil {
			srv.updateJobFailed(ctx, jobID, "invalid repository URL: "+err.Error())
			return
		}
		if err := gitClone(clonePath, authURL, conn.RepoBranch); err != nil {
			srv.updateJobFailed(ctx, jobID, "clone failed: "+err.Error())
			return
		}
		scanPath = clonePath

	} else { // local
		if err := validateLocalPath(conn.RepoLocalPath); err != nil {
			srv.updateJobFailed(ctx, jobID, "invalid local path: "+err.Error())
			return
		}
		scanPath = conn.RepoLocalPath
	}

	// 1. Detect stack
	stack := codescanner.DetectStack(scanPath)
	stackNames := stackToNames(stack)
	stackLabel := "none"
	if len(stackNames) > 0 {
		stackLabel = strings.Join(stackNames, ", ")
	}
	srv.updateJobProgress(ctx, jobID, "running", "Detecting stack: "+stackLabel+"...")

	// 2. Secret scan
	srv.updateJobProgress(ctx, jobID, "running", "Running secret scan (gitleaks)...")
	codeFindings := make([]model.CodeFinding, 0)
	gf, err := codescanner.RunGitleaks(scanPath)
	if err != nil {
		log.Printf("gitleaks [%s]: %v", jobID, err)
	}
	codeFindings = append(codeFindings, gf...)

	// 3. Static analysis
	srv.updateJobProgress(ctx, jobID, "running", "Running static analysis (semgrep)...")
	sf, err := codescanner.RunSemgrep(scanPath)
	if err != nil {
		log.Printf("semgrep [%s]: %v", jobID, err)
	}
	codeFindings = append(codeFindings, sf...)

	// 4. Dependency audit
	if stack.HasNode {
		srv.updateJobProgress(ctx, jobID, "running", "Running dependency audit (npm)...")
		for _, dir := range stack.NodeDirs {
			nf, err := codescanner.RunNpmAudit(dir)
			if err != nil {
				log.Printf("npm audit [%s]: %v", jobID, err)
			}
			codeFindings = append(codeFindings, nf...)
		}
	}

	// 5. Terraform
	tfFindings := make([]model.CodeFinding, 0)
	if stack.HasTerraform {
		srv.updateJobProgress(ctx, jobID, "running", "Scanning Terraform (trivy + hclscan)...")
		hf, err := codescanner.RunHCLScan(scanPath)
		if err != nil {
			log.Printf("hclscan [%s]: %v", jobID, err)
		}
		tfFindings = append(tfFindings, hf...)
		for _, dir := range stack.TerraformDirs {
			tf, err := codescanner.RunTrivy(dir)
			if err != nil {
				log.Printf("trivy [%s]: %v", jobID, err)
			}
			tfFindings = append(tfFindings, tf...)
		}
	}

	// Save findings JSON
	if b, err := json.Marshal(codeFindings); err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "findings.json"), b, 0644)
	}
	if b, err := json.Marshal(tfFindings); err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "tf_findings.json"), b, 0644)
	}

	// Count severities
	critical, high, medium, low := countCodeSeverities(codeFindings, tfFindings)

	// Store stack on connection
	if len(stackNames) > 0 {
		if b, err := json.Marshal(stackNames); err == nil {
			srv.updateConnectionStack(ctx, connectionID, string(b))
		}
	}

	// Save stack on job
	if b, err := json.Marshal(stackNames); err == nil {
		srv.updateJobStack(ctx, jobID, string(b))
	}

	// Build reports
	now := time.Now().UTC()
	base := util.SafeName(conn.Name + "_code_security_audit")

	assetsDir := filepath.Join(reportDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		srv.updateJobFailed(ctx, jobID, "failed to create assets dir: "+err.Error())
		return
	}
	logoRel := resolveAsset(userID, "logo.png", assetsDir)
	watermarkRel := resolveAsset(userID, "watermark.png", assetsDir)
	footerBgRel := resolveAsset(userID, "footer-bg.png", assetsDir)

	meta := model.ReportMeta{
		Client:           conn.Name,
		ProjectName:      repoDisplayName(conn),
		Provider:         "Source Code",
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
	}

	codeReport := model.CodeReport{
		Meta:       meta,
		RepoPath:   scanPath,
		Stack:      stackNames,
		Findings:   codeFindings,
		TFFindings: tfFindings,
	}

	htmlFile := filepath.Join(reportDir, base+".html")
	docxFile := filepath.Join(reportDir, base+".docx")

	srv.updateJobProgress(ctx, jobID, "running", "Generating HTML report...")
	if err := report.WriteCodeHTML(htmlFile, codeReport); err != nil {
		srv.updateJobFailed(ctx, jobID, "HTML generation failed: "+err.Error())
		return
	}

	srv.updateJobProgress(ctx, jobID, "running", "Generating DOCX report...")
	if err := report.WriteCodeDOCX(docxFile, codeReport); err != nil {
		log.Printf("WARN: WriteCodeDOCX [%s]: %v", jobID, err)
	}

	srv.updateJobDone(ctx, jobID, htmlFile, docxFile, critical, high, medium, low)

	if completedJob, err := srv.getJob(ctx, jobID); err == nil {
		go srv.postJobMonitoring(context.Background(), completedJob)
	}

	go srv.autoCollectEvidence(context.Background(), jobID, userID, reportDir, htmlFile, docxFile, "code")

	if user.NotifyEmail && user.AuditorEmail != "" {
		go sendJobEmail(user.AuditorEmail, conn.Name, jobID, critical, high, medium, low, false, "")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func buildAuthURL(repoURL, token string) (string, error) {
	if token == "" {
		return repoURL, nil
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword("oauth2", token)
	return u.String(), nil
}

var reToken = regexp.MustCompile(`oauth2:[^@]+@`)

func gitClone(clonePath, authURL, branch string) error {
	args := []string{"clone", "--depth=1"}
	if branch != "" {
		args = append(args, "--branch="+branch)
	}
	args = append(args, authURL, clonePath)
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		sanitized := reToken.ReplaceAllString(string(out), "oauth2:***@")
		return fmt.Errorf("%s", strings.TrimSpace(sanitized))
	}
	return nil
}

func validateLocalPath(p string) error {
	if p == "" {
		return fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("absolute path required")
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("invalid path")
	}
	blocklist := []string{
		"/etc", "/root", "/proc", "/sys", "/dev",
		"/run", "/boot", "/usr", "/var",
	}
	for _, bad := range blocklist {
		if strings.HasPrefix(abs+"/", bad+"/") || abs == bad {
			return fmt.Errorf("access denied: %s is a restricted system path", bad)
		}
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("path not found")
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	return nil
}

func stackToNames(s codescanner.Stack) []string {
	var names []string
	if s.HasNode {
		names = append(names, "Node.js")
	}
	if s.HasTypeScript {
		names = append(names, "TypeScript")
	}
	if s.HasGo {
		names = append(names, "Go")
	}
	if s.HasPython {
		names = append(names, "Python")
	}
	if s.HasTerraform {
		names = append(names, "Terraform")
	}
	if s.HasDocker {
		names = append(names, "Docker")
	}
	return names
}

func countCodeSeverities(code, tf []model.CodeFinding) (critical, high, medium, low int) {
	for _, f := range append(code, tf...) {
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
	return
}

func repoDisplayName(conn Connection) string {
	if conn.RepoSource == "git" && conn.RepoURL != "" {
		u, err := url.Parse(conn.RepoURL)
		if err == nil {
			return u.Path
		}
		return conn.RepoURL
	}
	if conn.RepoLocalPath != "" {
		return conn.RepoLocalPath
	}
	return conn.Name
}
