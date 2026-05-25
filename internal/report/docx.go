package report

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"infra-audit/internal/model"
	"infra-audit/internal/rules"
)

// ── Assets ───────────────────────────────────────────────────────────────────

type docxAssets struct {
	LogoBytes []byte
	LogoExt   string
}

func loadDOCXAssetsFromMeta(reportPath string, meta model.ReportMeta) docxAssets {
	baseDir := filepath.Dir(reportPath)
	logoPath := filepath.Join(baseDir, filepath.FromSlash(meta.LogoPath))
	logoBytes, _ := os.ReadFile(logoPath)
	ext := strings.ToLower(filepath.Ext(logoPath))
	if ext == "" {
		ext = ".png"
	}
	return docxAssets{LogoBytes: logoBytes, LogoExt: ext}
}

func loadDOCXAssets(reportPath string, r model.Report) docxAssets {
	baseDir := filepath.Dir(reportPath)
	logoPath := filepath.Join(baseDir, filepath.FromSlash(r.Meta.LogoPath))
	logoBytes, _ := os.ReadFile(logoPath)
	ext := strings.ToLower(filepath.Ext(logoPath))
	if ext == "" {
		ext = ".png"
	}
	return docxAssets{LogoBytes: logoBytes, LogoExt: ext}
}

// ── Main entry ───────────────────────────────────────────────────────────────

func WriteDOCX(path string, r model.Report) error {
	assets := loadDOCXAssets(path, r)
	var body strings.Builder
	c := rules.CountBySeverity(r.Findings)

	addCoverPage(&body, r, assets)
	addPageBreak(&body)

	addH1(&body, "Summary")
	summary := strings.TrimSpace(r.Summary)
	if summary == "" {
		summary = "The assessment was generated from exported cloud evidence and mapped to common control families from ISO/IEC 27001, NIST Cybersecurity Framework and CIS Controls."
	}
	addPara(&body, summary)

	addTableWithWidths(&body,
		[]string{"Severity", "Count", "Topics"},
		[]int{2400, 1200, 5800},
		[][]string{
			{"Critical", fmt.Sprint(c["Critical"]), topicsForSeverity(r.Findings, "Critical")},
			{"High", fmt.Sprint(c["High"]), topicsForSeverity(r.Findings, "High")},
			{"Medium", fmt.Sprint(c["Medium"]), topicsForSeverity(r.Findings, "Medium")},
			{"Low / Info", fmt.Sprint(c["Low"] + c["Info"]), topicsForSeverity(r.Findings, "Low")},
		},
	)

	addH2(&body, "Evidence snapshot")
	addEvidenceSnapshot(&body, r)
	addParaMuted(&body, "Framework mapping: ISO/IEC 27001:2022, NIST Cybersecurity Framework, CIS Critical Security Controls and CIS Kubernetes Benchmark where applicable. Evidence collected at "+r.Inventory.CollectedAt+".")

	if len(r.Findings) > 0 {
		addH1(&body, "Findings Register")
		header := []string{"ID", "Severity", "Finding", "Resource", "Timeline"}
		widths := []int{1000, 1400, 4400, 1800, 1600}
		var rows [][]string
		for _, f := range r.Findings {
			rows = append(rows, []string{
				f.ID, f.Severity, f.Title,
				nonEmpty(f.ResourceName, "N/A"),
				nonEmpty(f.Timeline, "Review"),
			})
		}
		addTableWithWidths(&body, header, widths, rows)
	}

	if len(r.Findings) > 0 {
		addH1(&body, "Detailed Findings")
		for _, sev := range []string{"Critical", "High", "Medium", "Low", "Info"} {
			var group []model.Finding
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
				addFindingDOCX(&body, f)
			}
		}
	}

	if len(r.Positives) > 0 {
		addH1(&body, "Positive Findings")
		addPara(&body, "The following positive observations were identified from the available evidence.")
		header := []string{"Area", "Status", "Evidence"}
		widths := []int{2800, 1800, 4800}
		var rows [][]string
		for _, p := range r.Positives {
			rows = append(rows, []string{p.Area, p.Status, p.Evidence})
		}
		addTableWithWidths(&body, header, widths, rows)
	}

	if len(r.Findings) > 0 {
		addH1(&body, "Recommended Actions")
		for _, group := range []string{"Immediate / today", "Short term / within 1 week", "Medium term / within 1 month", "Ongoing / backlog", "Informational"} {
			var items []model.Finding
			for _, f := range r.Findings {
				if strings.TrimSpace(f.Timeline) == group {
					items = append(items, f)
				}
			}
			if len(items) == 0 {
				continue
			}
			addH2(&body, group)
			for _, f := range items {
				text := strings.TrimSpace(f.Remediation)
				if text == "" {
					text = strings.TrimSpace(f.Recommendation)
				}
				if text == "" {
					text = f.Title
				}
				addBullet(&body, f.ID+": "+text)
			}
		}
	}

	addH1(&body, "Methodology & Limitations")
	addPara(&body, "The assessment is an automated, point-in-time configuration review based on cloud API evidence visible to the supplied audit token.")
	if len(r.Meta.Standards) > 0 {
		addH2(&body, "International standards alignment")
		for _, s := range r.Meta.Standards {
			addBullet(&body, s)
		}
	}
	if len(r.Limitations) > 0 {
		addH2(&body, "Limitations")
		for _, l := range r.Limitations {
			addBullet(&body, l)
		}
	}

	return writeDocxZip(path, body.String(), assets)
}

// ── Cover page ───────────────────────────────────────────────────────────────

func addCoverPage(b *strings.Builder, r model.Report, assets docxAssets) {
	// Top row: org name left, logo right
	logoCell := `<w:p><w:pPr><w:jc w:val="right"/><w:spacing w:before="240" w:after="0"/></w:pPr><w:r><w:rPr><w:sz w:val="22"/></w:rPr><w:t></w:t></w:r></w:p>`
	if len(assets.LogoBytes) > 0 {
		logoCell = `<w:p><w:pPr><w:jc w:val="right"/><w:spacing w:before="240" w:after="0"/></w:pPr>` +
			`<w:r><w:rPr><w:noProof/></w:rPr>` +
			`<w:drawing><wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" distT="0" distB="0" distL="0" distR="0">` +
			`<wp:extent cx="1224000" cy="1224000"/>` +
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>` +
			`<wp:docPr id="1" name="logo"/>` +
			`<wp:cNvGraphicFramePr><a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/></wp:cNvGraphicFramePr>` +
			`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
			`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
			`<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
			`<pic:nvPicPr><pic:cNvPr id="1" name="logo"/><pic:cNvPicPr/></pic:nvPicPr>` +
			`<pic:blipFill><a:blip r:embed="rIdLogo"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
			`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1224000" cy="1224000"/></a:xfrm>` +
			`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>` +
			`</pic:pic></a:graphicData></a:graphic>` +
			`</wp:inline></w:drawing>` +
			`</w:r></w:p>`
	}

	b.WriteString(`<w:tbl><w:tblPr>` +
		`<w:tblW w:w="9400" w:type="dxa"/>` +
		`<w:tblBorders><w:top w:val="none"/><w:left w:val="none"/><w:bottom w:val="none"/><w:right w:val="none"/><w:insideH w:val="none"/><w:insideV w:val="none"/></w:tblBorders>` +
		`</w:tblPr><w:tr><w:trPr><w:cantSplit/></w:trPr>` +
		`<w:tc><w:tcPr><w:tcW w:w="7000" w:type="dxa"/></w:tcPr>` +
		`<w:p><w:pPr><w:spacing w:before="480" w:after="120"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="50"/><w:szCs w:val="50"/><w:color w:val="111827"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(r.Meta.AuditorOrg) + `</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:spacing w:before="0" w:after="60"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="18"/><w:color w:val="4B5563"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(strings.ReplaceAll(r.Meta.AuditorAddress, "\n", " | ")) + `</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:spacing w:before="0" w:after="60"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="18"/><w:color w:val="4B5563"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(r.Meta.AuditorEmail) + `</w:t></w:r></w:p>` +
		`</w:tc>` +
		`<w:tc><w:tcPr><w:tcW w:w="2400" w:type="dxa"/></w:tcPr>` +
		logoCell +
		`</w:tc></w:tr></w:tbl><w:p/>`)

	addSpacerPara(b, 800)

	b.WriteString(`<w:p><w:pPr><w:jc w:val="center"/><w:spacing w:before="0" w:after="240"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="46"/><w:szCs w:val="46"/><w:color w:val="111827"/></w:rPr>` +
		`<w:t>Security &amp; Infrastructure Audit Report</w:t></w:r></w:p>`)

	addSpacerPara(b, 400)

	coverRows := [][]string{
		{"Prepared for", r.Meta.Client},
		{"Project", r.Meta.ProjectName},
		{"Assessment period", r.Meta.AssessmentPeriod},
		{"Classification", r.Meta.Classification},
		{"Prepared by", r.Meta.PreparedBy},
	}
	b.WriteString(`<w:tbl><w:tblPr>` +
		`<w:tblW w:w="6000" w:type="dxa"/><w:jc w:val="center"/>` +
		`<w:tblBorders>` +
		`<w:top w:val="single" w:sz="4" w:color="CBD5E1"/>` +
		`<w:bottom w:val="single" w:sz="4" w:color="CBD5E1"/>` +
		`<w:insideH w:val="single" w:sz="4" w:color="E2E8F0"/>` +
		`</w:tblBorders></w:tblPr>`)
	for _, row := range coverRows {
		b.WriteString(`<w:tr><w:trPr><w:cantSplit/></w:trPr>` +
			`<w:tc><w:tcPr><w:tcW w:w="2200" w:type="dxa"/></w:tcPr>` +
			`<w:p><w:pPr><w:spacing w:before="60" w:after="60"/></w:pPr>` +
			`<w:r><w:rPr><w:b/><w:color w:val="4B5563"/><w:sz w:val="20"/></w:rPr>` +
			`<w:t xml:space="preserve">` + xesc(row[0]) + `</w:t></w:r></w:p></w:tc>` +
			`<w:tc><w:tcPr><w:tcW w:w="3800" w:type="dxa"/></w:tcPr>` +
			`<w:p><w:pPr><w:spacing w:before="60" w:after="60"/></w:pPr>` +
			`<w:r><w:rPr><w:sz w:val="20"/></w:rPr>` +
			`<w:t xml:space="preserve">` + xesc(row[1]) + `</w:t></w:r></w:p></w:tc>` +
			`</w:tr>`)
	}
	b.WriteString(`</w:tbl><w:p/>`)
}

// ── Finding ──────────────────────────────────────────────────────────────────

func addFindingDOCX(b *strings.Builder, f model.Finding) {
	compact := f.Severity == "Medium" || f.Severity == "Low" || f.Severity == "Info"
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="200" w:after="80"/></w:pPr>` +
		`<w:r><w:rPr><w:b/><w:sz w:val="26"/><w:szCs w:val="26"/><w:color w:val="111827"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(f.ID) + `. ` + xesc(f.Title) + `</w:t></w:r></w:p>`)

	component := "N/A"
	if len(f.AffectedComponents) > 0 {
		component = strings.Join(f.AffectedComponents, ", ")
	} else if strings.TrimSpace(f.ResourceName) != "" {
		component = f.ResourceName
	}

	if !compact {
		metaRows := [][]string{
			{"Component", component},
			{"Finding", nonEmpty(f.Evidence, f.Title)},
			{"Status", nonEmpty(f.Status, "Open")},
			{"Standard", nonEmpty(f.Standard, "ISO 27001 / NIST CSF / CIS Controls")},
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
		addParaSmallMuted(b, "Component: "+component+" · Standard: "+nonEmpty(f.Standard, "ISO 27001 / NIST CSF / CIS Controls"))
	}

	if strings.TrimSpace(f.Risk) != "" {
		addPara(b, f.Risk)
	}
	if strings.TrimSpace(f.BusinessImpact) != "" && !compact {
		addPara(b, f.BusinessImpact)
	}
	addParaBold(b, "Recommendations:")
	if strings.TrimSpace(f.Recommendation) != "" {
		addBullet(b, f.Recommendation)
	}
	if strings.TrimSpace(f.Remediation) != "" && f.Remediation != f.Recommendation && !compact {
		addBullet(b, f.Remediation)
	}
	if strings.TrimSpace(f.Validation) != "" && !compact {
		addBullet(b, f.Validation)
	}
}

// ── Evidence snapshot ────────────────────────────────────────────────────────

func addEvidenceSnapshot(b *strings.Builder, r model.Report) {
	type item struct {
		Label string
		Key   string
	}
	items := []item{
		{"Apps", "apps"},
		{"Databases", "databases"},
		{"Droplets", "droplets"},
		{"Firewalls", "firewalls"},
		{"VPCs", "vpcs"},
		{"Projects", "projects"},
		{"Domains", "domains"},
		{"Reserved IPs", "reserved_ips"},
		{"Spaces Buckets", "spaces"},
		{"Container Registry", "container_registry"},
	}
	header := []string{"Evidence area", "Count", "Evidence area", "Count"}
	widths := []int{3000, 900, 3000, 900}
	var rows [][]string
	for i := 0; i < len(items); i += 2 {
		left := items[i]
		rightLabel, rightCount := "", ""
		if i+1 < len(items) {
			rightLabel = items[i+1].Label
			rightCount = fmt.Sprint(resourceCount(r.Inventory.Resources[items[i+1].Key]))
		}
		rows = append(rows, []string{left.Label, fmt.Sprint(resourceCount(r.Inventory.Resources[left.Key])), rightLabel, rightCount})
	}
	addTableWithWidths(b, header, widths, rows)
}

// ── Table ────────────────────────────────────────────────────────────────────

func addTableWithWidths(b *strings.Builder, header []string, widths []int, rows [][]string) {
	b.WriteString(`<w:tbl><w:tblPr>` +
		`<w:tblW w:w="0" w:type="auto"/>` +
		`<w:tblBorders>` +
		`<w:top w:val="single" w:sz="6" w:color="111827"/>` +
		`<w:left w:val="none"/><w:right w:val="none"/>` +
		`<w:bottom w:val="single" w:sz="6" w:color="111827"/>` +
		`<w:insideH w:val="single" w:sz="4" w:color="D8DEE8"/>` +
		`<w:insideV w:val="none"/>` +
		`</w:tblBorders></w:tblPr>`)

	b.WriteString(`<w:tr><w:trPr><w:cantSplit/></w:trPr>`)
	for i, cell := range header {
		wAttr := ""
		if i < len(widths) && widths[i] > 0 {
			wAttr = fmt.Sprintf(`<w:tcW w:w="%d" w:type="dxa"/>`, widths[i])
		}
		b.WriteString(`<w:tc><w:tcPr>` + wAttr + `<w:shd w:fill="F8FAFC"/></w:tcPr>` +
			`<w:p><w:pPr><w:spacing w:before="60" w:after="60"/></w:pPr>` +
			`<w:r><w:rPr><w:b/><w:sz w:val="19"/></w:rPr>` +
			`<w:t xml:space="preserve">` + xesc(cell) + `</w:t></w:r></w:p></w:tc>`)
	}
	b.WriteString(`</w:tr>`)

	for _, row := range rows {
		b.WriteString(`<w:tr><w:trPr><w:cantSplit/></w:trPr>`)
		for i, cell := range row {
			wAttr := ""
			if i < len(widths) && widths[i] > 0 {
				wAttr = fmt.Sprintf(`<w:tcW w:w="%d" w:type="dxa"/>`, widths[i])
			}
			b.WriteString(`<w:tc><w:tcPr>` + wAttr + `</w:tcPr>` +
				`<w:p><w:pPr><w:spacing w:before="50" w:after="50"/></w:pPr>` +
				`<w:r><w:rPr><w:sz w:val="19"/></w:rPr>` +
				`<w:t xml:space="preserve">` + xesc(cell) + `</w:t></w:r></w:p></w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl><w:p/>`)
}

// ── Typography ───────────────────────────────────────────────────────────────

func addH1(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="320" w:after="160"/>` +
		`<w:pBdr><w:bottom w:val="single" w:sz="4" w:space="4" w:color="111827"/></w:pBdr></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="38"/><w:szCs w:val="38"/><w:color w:val="111827"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(text) + `</w:t></w:r></w:p>`)
}

func addH2(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="200" w:after="80"/></w:pPr>` +
		`<w:r><w:rPr><w:b/><w:sz w:val="27"/><w:szCs w:val="27"/><w:color w:val="334155"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(text) + `</w:t></w:r></w:p>`)
}

func addSeverityHeading(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="280" w:after="120"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="36"/><w:szCs w:val="36"/><w:color w:val="111827"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(text) + `</w:t></w:r></w:p>`)
}

func addPara(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="100"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="22"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(text) + `</w:t></w:r></w:p>`)
}

func addParaMuted(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="80"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="18"/><w:color w:val="4B5563"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(text) + `</w:t></w:r></w:p>`)
}

func addParaSmallMuted(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="60"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="18"/><w:color w:val="4B5563"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(text) + `</w:t></w:r></w:p>`)
}

func addParaBold(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="80" w:after="40"/></w:pPr>` +
		`<w:r><w:rPr><w:b/><w:sz w:val="22"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc(text) + `</w:t></w:r></w:p>`)
}

func addBullet(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:ind w:left="480" w:hanging="240"/><w:spacing w:before="40" w:after="40"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="20"/></w:rPr>` +
		`<w:t xml:space="preserve">` + xesc("• "+text) + `</w:t></w:r></w:p>`)
}

func addPageBreak(b *strings.Builder) {
	b.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
}

func addSpacerPara(b *strings.Builder, twips int) {
	b.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:spacing w:before="%d" w:after="0"/></w:pPr></w:p>`, twips))
}

// ── ZIP writer ───────────────────────────────────────────────────────────────

func writeDocxZip(path, body string, assets docxAssets) error {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	docRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`
	if len(assets.LogoBytes) > 0 {
		docRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rIdLogo" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/logo` + assets.LogoExt + `"/>` +
			`</Relationships>`
	}

	files := map[string][]byte{
		"[Content_Types].xml": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Default Extension="png" ContentType="image/png"/>` +
			`<Default Extension="jpg" ContentType="image/jpeg"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
			`<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>` +
			`</Types>`),
		"_rels/.rels": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
			`</Relationships>`),
		"word/_rels/document.xml.rels": []byte(docRels),
		"word/styles.xml":              []byte(docxStylesXML()),
		"word/settings.xml": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat>` +
			`</w:settings>`),
		"word/document.xml": []byte(docxDocumentXML(body)),
	}

	if len(assets.LogoBytes) > 0 {
		files["word/media/logo"+assets.LogoExt] = assets.LogoBytes
	}

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write(content); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func docxDocumentXML(body string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"` +
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<w:body>` + body +
		`<w:sectPr>` +
		`<w:pgSz w:w="11906" w:h="16838"/>` +
		`<w:pgMar w:top="1080" w:right="850" w:bottom="1080" w:left="850" w:header="708" w:footer="708" w:gutter="0"/>` +
		`</w:sectPr></w:body></w:document>`
}

func docxStylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:docDefaults><w:rPrDefault><w:rPr>` +
		`<w:rFonts w:ascii="Ubuntu" w:hAnsi="Ubuntu" w:cs="Ubuntu"/>` +
		`<w:sz w:val="22"/><w:szCs w:val="22"/>` +
		`<w:color w:val="080808"/>` +
		`</w:rPr></w:rPrDefault></w:docDefaults>` +
		`</w:styles>`
}

func xesc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
