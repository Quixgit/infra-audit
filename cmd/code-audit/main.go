package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"infra-audit/internal/model"
	"infra-audit/internal/report"
	codescanner "infra-audit/internal/scanner/code"
	"infra-audit/internal/util"
)

func main() {
	repo         := flag.String("repo", "", "Path to repository (required)")
	client       := flag.String("client", "", "Client name")
	project      := flag.String("project", "", "Project name")
	preparedBy   := flag.String("prepared-by", "", "Prepared by")
	auditorOrg   := flag.String("auditor-org", "", "Auditor organisation")
	auditorAddr  := flag.String("auditor-address", "", "Auditor address")
	auditorEmail := flag.String("auditor-email", "", "Auditor email")
	auditorWeb   := flag.String("auditor-website", "", "Auditor website")
	auditorPhone := flag.String("auditor-phone", "", "Auditor phone")
	class        := flag.String("classification", "Confidential", "Document classification")
	period       := flag.String("period", "", "Assessment period")
	logo         := flag.String("logo", "", "Path to logo image")
	watermark    := flag.String("watermark", "", "Path to watermark image")
	footerBg     := flag.String("footer-bg", "", "Path to footer background image")
	out          := flag.String("out", "out/code-audit", "Output directory")
	flag.Parse()

	if *repo == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --repo is required")
		os.Exit(1)
	}

	fmt.Println("Scanning repository:", *repo)

	codeFindings, tfFindings, stack := codescanner.Scan(*repo)

	fmt.Printf("Stack: Node=%v TypeScript=%v Go=%v Python=%v Terraform=%v Docker=%v\n",
		stack.HasNode, stack.HasTypeScript, stack.HasGo, stack.HasPython, stack.HasTerraform, stack.HasDocker)
	fmt.Printf("Code findings: %d | Terraform findings: %d\n", len(codeFindings), len(tfFindings))

	reportDir := filepath.Join(*out, "report")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	// Copy assets
	now := time.Now().UTC()
	base := util.SafeName(*client + "_" + *project)

	logoRel, _     := prepareAsset(reportDir, *logo, "logo.png")
	watermarkRel, _ := prepareAsset(reportDir, *watermark, "watermark.png")
	footerRel, _   := prepareAsset(reportDir, *footerBg, "footer-bg.png")
	for _, icon := range []string{"icon-email.png", "icon-web.png", "icon-phone.png"} {
		src := icon
		if *logo != "" {
			src = filepath.Join(filepath.Dir(*logo), icon)
		}
		_, _ = prepareAsset(reportDir, src, icon)
	}

	meta := model.ReportMeta{
		Client:           *client,
		ProjectName:      *project,
		PreparedBy:       *preparedBy,
		AuditorOrg:       *auditorOrg,
		AuditorAddress:   *auditorAddr,
		AuditorEmail:     *auditorEmail,
		AuditorWebsite:   *auditorWeb,
		AuditorPhone:     *auditorPhone,
		Classification:   *class,
		AssessmentPeriod: *period,
		GeneratedAt:      now.Format(time.RFC3339),
		ReportDate:       now.Format("2006-01-02"),
		ArtifactBase:     base,
		LogoPath:         logoRel,
		WatermarkPath:    watermarkRel,
		FooterBgPath:     footerRel,
		Standards: []string{
			"OWASP Top 10 (2021)",
			"CIS Benchmarks",
			"NIST SP 800-190 (Container Security)",
			"Terraform Security Best Practices",
			"ISO/IEC 27001:2022",
		},
	}

	r := model.CodeReport{
		Meta:       meta,
		RepoPath:   *repo,
		Stack:      stackLabels(stack),
		Findings:   codeFindings,
		TFFindings: tfFindings,
	}

	htmlPath := filepath.Join(reportDir, base+".html")
	if err := report.WriteCodeHTML(htmlPath, r); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR writing HTML:", err)
		os.Exit(1)
	}

	docxPath := filepath.Join(reportDir, base+".docx")
	if err := report.WriteCodeDOCX(docxPath, r); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR writing DOCX:", err)
		os.Exit(1)
	}

	if err := report.ApplyDOCXBranding(docxPath, reportDir, r.Meta); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR applying branding:", err)
	}

	fmt.Println("Generated:")
	fmt.Println(" -", htmlPath)
	fmt.Println(" -", docxPath)
}

func stackLabels(s codescanner.Stack) []string {
	var labels []string
	if s.HasNode      { labels = append(labels, "Node.js") }
	if s.HasTypeScript { labels = append(labels, "TypeScript") }
	if s.HasGo        { labels = append(labels, "Go") }
	if s.HasPython    { labels = append(labels, "Python") }
	if s.HasTerraform { labels = append(labels, "Terraform") }
	if s.HasDocker    { labels = append(labels, "Docker") }
	return labels
}

func prepareAsset(outDir, src, filename string) (string, error) {
	src = filepath.Clean(src)
	if src == "" || src == "." {
		return "", nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	assetDir := filepath.Join(outDir, "assets")
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		return "", err
	}
	dst := filepath.Join(assetDir, filename)
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return "", err
	}
	return filepath.Join("assets", filename), nil
}
