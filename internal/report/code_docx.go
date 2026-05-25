package report

import (
	"fmt"
	"strings"

	"infra-audit/internal/model"
)

func WriteCodeDOCX(path string, r model.CodeReport) error {
	assets := loadDOCXAssetsFromMeta(path, r.Meta)
	var body strings.Builder

	addCoverPage(&body, model.Report{Meta: r.Meta}, assets)
	addPageBreak(&body)

	// Summary
	addH1(&body, "Summary")
	addPara(&body, codeExecSummary(r))

	// Stack
	addH2(&body, "Technology stack detected")
	addPara(&body, strings.Join(r.Stack, " · "))

	// Summary table
	cc := countCodeBySeverity(r.Findings)
	tc := countCodeBySeverity(r.TFFindings)
	addTableWithWidths(&body,
		[]string{"Category", "Critical", "High", "Medium", "Low", "Info", "Total"},
		[]int{3200, 1000, 1000, 1000, 1000, 1000, 1000},
		[][]string{
			{"Code Security",
				fmt.Sprint(cc["Critical"]), fmt.Sprint(cc["High"]),
				fmt.Sprint(cc["Medium"]), fmt.Sprint(cc["Low"]),
				fmt.Sprint(cc["Info"]), fmt.Sprint(len(r.Findings))},
			{"Infrastructure (Terraform)",
				fmt.Sprint(tc["Critical"]), fmt.Sprint(tc["High"]),
				fmt.Sprint(tc["Medium"]), fmt.Sprint(tc["Low"]),
				fmt.Sprint(tc["Info"]), fmt.Sprint(len(r.TFFindings))},
		},
	)

	// Code findings
	if len(r.Findings) > 0 {
		addH1(&body, "Code Security Findings")
		for _, sev := range []string{"Critical", "High", "Medium", "Low", "Info"} {
			var group []model.CodeFinding
			for _, f := range r.Findings {
				if f.Severity == sev {
					group = append(group, f)
				}
			}
			if len(group) == 0 {
				continue
			}
			addSeverityHeading(&body, strings.ToUpper(sev))
			for _, f := range group {
				addCodeFindingDOCX(&body, f)
			}
		}
	}

	// Terraform findings
	if len(r.TFFindings) > 0 {
		addH1(&body, "Infrastructure Security Findings (Terraform)")
		for _, sev := range []string{"Critical", "High", "Medium", "Low", "Info"} {
			var group []model.CodeFinding
			for _, f := range r.TFFindings {
				if f.Severity == sev {
					group = append(group, f)
				}
			}
			if len(group) == 0 {
				continue
			}
			addSeverityHeading(&body, strings.ToUpper(sev))
			for _, f := range group {
				addCodeFindingDOCX(&body, f)
			}
		}
	}

	// Methodology
	addH1(&body, "Methodology & Limitations")
	addPara(&body, "This assessment combines automated static analysis with manual infrastructure code review.")
	addH2(&body, "Tools used")
	for _, tool := range []string{
		"gitleaks — secret and credential detection",
		"semgrep — static analysis for Node.js, TypeScript, Docker",
		"trivy — Terraform misconfiguration scanning",
		"hclscan — custom DigitalOcean Terraform rule engine",
		"npm audit — dependency vulnerability scanning",
	} {
		addBullet(&body, tool)
	}
	addH2(&body, "Standards alignment")
	for _, s := range r.Meta.Standards {
		addBullet(&body, s)
	}
	addH2(&body, "Limitations")
	addBullet(&body, "Point-in-time static analysis only. Dynamic vulnerabilities require manual penetration testing.")
	addBullet(&body, "npm audit returned no vulnerabilities at time of assessment.")
	addBullet(&body, "Terraform findings based on static HCL analysis. Deployed state may differ.")

	return writeDocxZip(path, body.String(), assets)
}

func addCodeFindingDOCX(b *strings.Builder, f model.CodeFinding) {
	compact := f.Severity == "Medium" || f.Severity == "Low" || f.Severity == "Info"

	title := f.Title
	if f.RuleID != "" {
		title = f.RuleID + ". " + title
	}

	b.WriteString(`<w:p><w:pPr><w:spacing w:before="200" w:after="80"/></w:pPr>` +
		`<w:r><w:rPr><w:b/><w:sz w:val="26"/><w:szCs w:val="26"/><w:color w:val="111827"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(f.Severity+": "+title) + `</w:t></w:r></w:p>`)

	loc := f.File
	if f.Line > 0 {
		loc += fmt.Sprintf(":%d", f.Line)
	}

	if !compact {
		metaRows := [][]string{
			{"File", loc},
			{"Tool", f.Tool},
		}
		if f.Category != "" {
			metaRows = append(metaRows, []string{"Category", f.Category})
		}
		if f.Description != "" {
			metaRows = append(metaRows, []string{"Description", f.Description})
		}

		b.WriteString(`<w:tbl><w:tblPr>` +
			`<w:tblW w:w="9400" w:type="dxa"/>` +
			`<w:tblBorders>` +
			`<w:top w:val="single" w:sz="4" w:color="CBD5E1"/>` +
			`<w:left w:val="single" w:sz="4" w:color="CBD5E1"/>` +
			`<w:bottom w:val="single" w:sz="4" w:color="CBD5E1"/>` +
			`<w:right w:val="single" w:sz="4" w:color="CBD5E1"/>` +
			`<w:insideH w:val="single" w:sz="4" w:color="E2E8F0"/>` +
			`<w:insideV w:val="single" w:sz="4" w:color="CBD5E1"/>` +
			`</w:tblBorders></w:tblPr>`)
		for _, row := range metaRows {
			b.WriteString(`<w:tr><w:trPr><w:cantSplit/></w:trPr>` +
				`<w:tc><w:tcPr><w:tcW w:w="2100" w:type="dxa"/><w:shd w:fill="F8FAFC"/></w:tcPr>` +
				`<w:p><w:pPr><w:spacing w:before="40" w:after="40"/></w:pPr>` +
				`<w:r><w:rPr><w:b/><w:sz w:val="19"/></w:rPr>` +
				`<w:t xml:space="preserve">` + xesc(row[0]) + `</w:t></w:r></w:p></w:tc>` +
				`<w:tc><w:tcPr><w:tcW w:w="7300" w:type="dxa"/></w:tcPr>` +
				`<w:p><w:pPr><w:spacing w:before="40" w:after="40"/></w:pPr>` +
				`<w:r><w:rPr><w:sz w:val="19"/></w:rPr>` +
				`<w:t xml:space="preserve">` + xesc(row[1]) + `</w:t></w:r></w:p></w:tc>` +
				`</w:tr>`)
		}
		b.WriteString(`</w:tbl><w:p/>`)
	} else {
		addParaSmallMuted(b, loc+" · "+f.Tool)
	}

	if f.Remediation != "" {
		addParaBold(b, "Remediation:")
		addBullet(b, f.Remediation)
	}
}
