package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error

	switch os.Args[1] {
	case "list-do-projects":
		err = cmdListDOProjects(os.Args[2:])
	case "scan-do":
		err = cmdScanDO(os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	case "all-do":
		err = cmdAllDO(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`infra-audit

Commands:
  list-do-projects
    Show DigitalOcean projects and IDs.

  scan-do --client "Client" --out out/client/evidence/do_inventory.json
    Collect DigitalOcean evidence only.

  report --client "Client" --project "Project" \
    --input out/client/evidence/do_inventory.json \
    --out out/client/report
    Generate report from existing evidence.

  all-do --client "Client" --project "Project" --out out/client
    Scan DigitalOcean and generate report.

Project filtering:
  --do-project-id "uuid"
  --do-project-name "Work Order Platform Web Application"
  --scope-mode project|hybrid|account
  --spaces-buckets "terraform-state-wo:tor1:sensitive,wo-files:tor1:sensitive"

Environment:
  export DO_TOKEN="dop_v1_xxx"
`)
}

type metaFlags struct {
	client           *string
	project          *string
	preparedBy       *string
	auditorOrg       *string
	auditorAddress   *string
	auditorEmail     *string
	auditorWebsite   *string
	auditorPhone     *string
	classification   *string
	assessmentPeriod *string
	version          *string
	logo             *string
	watermark        *string
	footerBg         *string
	doProjectID      *string
	doProjectName    *string
	scopeMode        *string
	spacesBuckets    *string
}

func addMetaFlags(fs *flag.FlagSet) metaFlags {
	return metaFlags{
		client:           fs.String("client", "Client Name", "client name"),
		project:          fs.String("project", "Infrastructure Environment", "project/application name"),
		preparedBy:       fs.String("prepared-by", "InfraJump Security Team", "person/team that prepared report"),
		auditorOrg:       fs.String("auditor-org", "InfraJump, Inc.", "auditor organization"),
		auditorAddress:   fs.String("auditor-address", "8 the Grn Ste A\nDover, DE 19901", "auditor address"),
		auditorEmail:     fs.String("auditor-email", "delivery@infrajump.com", "auditor email"),
		auditorWebsite:   fs.String("auditor-website", "infrajump.com", "auditor website"),
		auditorPhone:     fs.String("auditor-phone", "+1 650 4847938", "auditor phone"),
		classification:   fs.String("classification", "Confidential", "report classification"),
		assessmentPeriod: fs.String("period", "Point-in-time automated assessment", "assessment period"),
		version:          fs.String("version", "1.0", "report version"),
		logo:             fs.String("logo", "", "path to logo image"),
		watermark:        fs.String("watermark", "", "path to watermark image"),
		footerBg:         fs.String("footer-bg", "", "path to footer background image"),
		doProjectID:      fs.String("do-project-id", "", "DigitalOcean project ID to scan only one project"),
		doProjectName:    fs.String("do-project-name", "", "DigitalOcean project name to scan only one project"),
		scopeMode:        fs.String("scope-mode", "project", "scope mode: project, hybrid, account"),
		spacesBuckets:    fs.String("spaces-buckets", "", "comma-separated Spaces buckets as name:region[:sensitive], e.g. terraform-state-wo:tor1:sensitive,wo-files:tor1:sensitive"),
	}
}

func cmdListDOProjects(args []string) error {
	fs := flag.NewFlagSet("list-do-projects", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	projects, err := scanner.ListDigitalOceanProjects(token())
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("DigitalOcean projects")
	fmt.Println(strings.Repeat("─", 118))
	fmt.Printf("%-38s  %-38s  %-9s  %s\n", "ID", "Name", "Resources", "Description")
	fmt.Println(strings.Repeat("─", 118))

	for _, p := range projects {
		fmt.Printf("%-38s  %-38s  %-9d  %s\n", p.ID, trim(p.Name, 38), p.ResourceCount, trim(p.Description, 35))
	}

	fmt.Println(strings.Repeat("─", 118))
	fmt.Println()

	return nil
}

func cmdScanDO(args []string) error {
	fs := flag.NewFlagSet("scan-do", flag.ExitOnError)
	client := fs.String("client", "Client Name", "client name")
	out := fs.String("out", "out/evidence/do_inventory.json", "output JSON path")
	doProjectID := fs.String("do-project-id", "", "DigitalOcean project ID")
	doProjectName := fs.String("do-project-name", "", "DigitalOcean project name")
	spacesBuckets := fs.String("spaces-buckets", "", "comma-separated Spaces buckets as name:region[:sensitive]")

	if err := fs.Parse(args); err != nil {
		return err
	}

	inv, err := scanner.ScanDigitalOceanWithOptions(*client, token(), scanner.DOScanOptions{
		ProjectID:     *doProjectID,
		ProjectName:   *doProjectName,
		ScopeMode:     "project",
		SpacesBuckets: *spacesBuckets,
	})
	if err != nil {
		return err
	}

	if hasAuthErrors(inv.Errors) {
		return fmt.Errorf("DigitalOcean authentication failed. Check DO_TOKEN / DIGITALOCEAN_TOKEN. First error: %s", inv.Errors[0])
	}

	return writeJSON(*out, inv)
}

func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	meta := addMetaFlags(fs)
	input := fs.String("input", "out/evidence/do_inventory.json", "input inventory JSON")
	outDir := fs.String("out", "out/report", "report output directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return generateReport(meta, *input, *outDir)
}

func cmdAllDO(args []string) error {
	fs := flag.NewFlagSet("all-do", flag.ExitOnError)
	meta := addMetaFlags(fs)
	outDir := fs.String("out", "out/client", "client output directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	evidencePath := filepath.Join(*outDir, "evidence", "do_inventory.json")

	inv, err := scanner.ScanDigitalOceanWithOptions(*meta.client, token(), scanner.DOScanOptions{
		ProjectID:     *meta.doProjectID,
		ProjectName:   *meta.doProjectName,
		ScopeMode:     *meta.scopeMode,
		SpacesBuckets: *meta.spacesBuckets,
	})
	if err != nil {
		return err
	}

	if hasAuthErrors(inv.Errors) {
		return fmt.Errorf("DigitalOcean authentication failed. Check DO_TOKEN / DIGITALOCEAN_TOKEN. First error: %s", inv.Errors[0])
	}

	if err := writeJSON(evidencePath, inv); err != nil {
		return err
	}

	return generateReport(meta, evidencePath, filepath.Join(*outDir, "report"))
}

func hasAuthErrors(errors []string) bool {
	for _, e := range errors {
		if strings.Contains(e, "HTTP 401") || strings.Contains(e, "Unauthorized") {
			return true
		}
	}
	return false
}

func generateReport(meta metaFlags, input string, outDir string) error {
	var inv model.Inventory

	b, err := os.ReadFile(input)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(b, &inv); err != nil {
		return err
	}

	if strings.TrimSpace(*meta.client) != "" {
		inv.Client = *meta.client
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	logoRel, err := prepareAsset(outDir, *meta.logo, "logo.png")
	if err != nil {
		return err
	}

	watermarkRel, err := prepareAsset(outDir, *meta.watermark, "watermark.png")
	if err != nil {
		return err
	}

	footerRel, err := prepareAsset(outDir, *meta.footerBg, "footer-bg.png")
	if err != nil {
		return err
	}

	// Copy footer icons if they exist next to the source assets
	for _, icon := range []string{"icon-email.png", "icon-web.png", "icon-phone.png"} {
		src := icon
		if *meta.logo != "" {
			src = filepath.Join(filepath.Dir(*meta.logo), icon)
		}
		_, _ = prepareAsset(outDir, src, icon)
	}

	base := util.SafeName(*meta.client + "_" + *meta.project)
	now := time.Now().UTC()

	r := model.Report{
		Meta: model.ReportMeta{
			Client:           *meta.client,
			ProjectName:      *meta.project,
			Provider:         inv.Provider,
			PreparedBy:       *meta.preparedBy,
			AuditorOrg:       *meta.auditorOrg,
			AuditorAddress:   *meta.auditorAddress,
			AuditorEmail:     *meta.auditorEmail,
			AuditorWebsite:   *meta.auditorWebsite,
			AuditorPhone:     *meta.auditorPhone,
			Classification:   *meta.classification,
			AssessmentPeriod: *meta.assessmentPeriod,
			GeneratedAt:      now.Format(time.RFC3339),
			ReportDate:       now.Format("2006-01-02"),
			Version:          *meta.version,
			ArtifactBase:     base,
			LogoPath:         logoRel,
			WatermarkPath:    watermarkRel,
			FooterBgPath:     footerRel,
			Standards: []string{
				"ISO/IEC 27001:2022",
				"NIST Cybersecurity Framework",
				"CIS Critical Security Controls",
				"CIS Kubernetes Benchmark, where applicable",
			},
		},
		Inventory:   inv,
		Findings:    rules.Evaluate(inv),
		Positives:   rules.PositiveFindings(inv),
		Limitations: rules.Limitations(inv),
	}

	r.Summary = rules.SummaryText(r)

	if err := writeJSON(filepath.Join(outDir, base+"_findings.json"), r); err != nil {
		return err
	}

	if err := report.WriteDOCX(filepath.Join(outDir, base+".docx"), r); err != nil {
		return err
	}
	if err := report.ApplyDOCXBranding(filepath.Join(outDir, base+".docx"), outDir, r.Meta); err != nil {
		return err
	}

	if err := report.WriteHTML(filepath.Join(outDir, base+".html"), r); err != nil {
		return err
	}

	fmt.Println("Generated:")
	fmt.Println(" -", filepath.Join(outDir, base+".docx"))
	fmt.Println(" -", filepath.Join(outDir, base+".html"))
	fmt.Println(" -", filepath.Join(outDir, base+"_findings.json"))

	return nil
}

func prepareAsset(outDir, src, filename string) (string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", nil
	}

	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("asset not found: %s", src)
	}

	assetDir := filepath.Join(outDir, "assets")
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		return "", err
	}

	dst := filepath.Join(assetDir, filename)

	if err := copyFile(src, dst); err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Join("assets", filename)), nil
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

func token() string {
	if v := os.Getenv("DO_TOKEN"); strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(os.Getenv("DIGITALOCEAN_TOKEN"))
}

func writeJSON(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, b, 0644); err != nil {
		return err
	}

	fmt.Println("Wrote", path)
	return nil
}

func trim(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}

	r := []rune(s)
	if n <= 3 {
		return string(r[:n])
	}

	return string(r[:n-3]) + "..."
}
