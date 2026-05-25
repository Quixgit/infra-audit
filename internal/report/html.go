package report

import (
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"infra-audit/internal/model"
	"infra-audit/internal/rules"
)

type htmlAssets struct {
	Logo      string
	Watermark string
	FooterBg  string
}

func WriteHTML(path string, r model.Report) error {
	assets := loadHTMLAssets(path, r)

	var b strings.Builder

	b.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + esc(r.Meta.ArtifactBase) + `</title>
<style>
@import url('https://fonts.googleapis.com/css2?family=Ubuntu:wght@400;500;700&display=swap');

:root{
  --page-width:210mm;
  --page-height:297mm;
  --text:#080808;
  --muted:#4b5563;
  --line:#111827;
  --soft-line:#d8dee8;
  --teal:#9edfde;
  --bg:#e8eef2;
}

*{
  box-sizing:border-box;
}

html, body{
  margin:0;
  padding:0;
  background:var(--bg);
  color:var(--text);
  font-family:'Ubuntu', Arial, sans-serif;
  font-size:11pt;
  line-height:1.31;
}

.toolbar{
  position:sticky;
  top:0;
  z-index:999;
  display:flex;
  gap:10px;
  align-items:center;
  padding:9px 14px;
  background:#111827;
  color:#fff;
  box-shadow:0 4px 20px rgba(0,0,0,.12);
}

.toolbar .title{
  margin-right:auto;
  font-weight:700;
  font-size:13px;
}

.toolbar button,
.toolbar a{
  border:1px solid rgba(255,255,255,.18);
  background:#fff;
  color:#111827;
  text-decoration:none;
  border-radius:8px;
  font-size:12px;
  font-weight:700;
  padding:7px 11px;
  cursor:pointer;
}

.pages{
  width:100%;
}

.page{
  position:relative;
  width:var(--page-width);
  height:var(--page-height);
  margin:10mm auto;
  background:#fff;
  overflow:hidden;
  border:1px solid #d5dbe3;
  page-break-after:always;
}

.page-inner{
  position:relative;
  z-index:2;
  height:100%;
  padding:22mm 22mm 25mm 22mm;
  overflow:hidden;
}

.cover .page-inner{
  padding:24mm 22mm 25mm 22mm;
}

.watermark{
  position:absolute;
  z-index:0;
  left:10mm;
  right:10mm;
  top:28mm;
  bottom:24mm;
  background-repeat:no-repeat;
  background-position:center;
  background-size:92%;
  opacity:.57;
  pointer-events:none;
}

.cover .watermark{
  top:30mm;
  bottom:25mm;
  background-size:92%;
  opacity:.49;
}

.cover-logo{
  position:absolute;
  z-index:3;
  top:21mm;
  right:23mm;
  width:34mm;
  height:auto;
}

.company{
  position:relative;
  z-index:3;
  margin-top:7mm;
  max-width:86mm;
}

.company-name{
  font-size:25pt;
  font-weight:400;
  line-height:1.05;
  margin:0 0 9mm 0;
}

.company-meta{
  font-size:11pt;
  line-height:1.3;
}

.company-meta a{
  color:#111;
}

.cover-title{
  position:absolute;
  z-index:3;
  left:22mm;
  right:22mm;
  top:96mm;
  text-align:center;
  font-size:23pt;
  line-height:1.22;
  font-weight:400;
}

.cover-meta{
  position:absolute;
  z-index:3;
  left:42mm;
  right:42mm;
  top:143mm;
  border-top:1px solid rgba(17,24,39,.55);
  border-bottom:1px solid rgba(17,24,39,.55);
  padding:6mm 0;
  font-size:10.5pt;
  background:rgba(255,255,255,.16);
}

.cover-meta-row{
  display:grid;
  grid-template-columns:39mm 1fr;
  gap:6mm;
  margin-bottom:3mm;
}

.cover-meta-row:last-child{
  margin-bottom:0;
}

.cover-meta-label{
  color:#4b5563;
  font-weight:700;
}

h1{
  margin:0 0 8mm 0;
  font-size:19pt;
  line-height:1.18;
  font-weight:400;
}

h2{
  margin:6mm 0 3.5mm 0;
  font-size:13.5pt;
  line-height:1.25;
  font-weight:700;
}

h3{
  margin:5mm 0 2.5mm 0;
  font-size:12pt;
  line-height:1.25;
  font-weight:700;
}

p{
  margin:0 0 3.6mm 0;
}

.small{
  font-size:9pt;
}

.muted{
  color:var(--muted);
}

ul{
  margin:0 0 3.8mm 0;
  padding-left:6mm;
}

li{
  margin-bottom:1.6mm;
}

table{
  width:100%;
  border-collapse:collapse;
  margin:4mm 0 4mm 0;
}

th, td{
  text-align:left;
  vertical-align:top;
  padding:2mm 2.6mm;
  border-bottom:1px solid var(--line);
  border-right:1px solid var(--line);
}

th:last-child,
td:last-child{
  border-right:none;
}

th{
  font-weight:700;
  background:rgba(248,250,252,.82);
}

.summary-table th:nth-child(1),
.summary-table td:nth-child(1){
  width:29mm;
}

.summary-table th:nth-child(2),
.summary-table td:nth-child(2){
  width:18mm;
  text-align:center;
  font-weight:700;
}

.evidence-table{
  margin-top:3mm;
  margin-bottom:4mm;
  font-size:10pt;
}

.evidence-table th,
.evidence-table td{
  padding:1.8mm 2.4mm;
}

.evidence-table td:nth-child(2),
.evidence-table td:nth-child(4),
.evidence-table th:nth-child(2),
.evidence-table th:nth-child(4){
  width:18mm;
  text-align:center;
  font-weight:700;
}

.register-table{
  font-size:9.2pt;
}

.register-table th,
.register-table td{
  padding:1.55mm 1.9mm;
}

.register-table th:nth-child(1),
.register-table td:nth-child(1){
  width:15mm;
}

.register-table th:nth-child(2),
.register-table td:nth-child(2){
  width:20mm;
}

.register-table th:nth-child(4),
.register-table td:nth-child(4){
  width:35mm;
}

.register-table th:nth-child(5),
.register-table td:nth-child(5){
  width:34mm;
}

.severity-heading{
  font-size:18pt;
  margin:0 0 6mm 0;
  font-weight:400;
  letter-spacing:.3px;
}

.finding{
  margin-bottom:7mm;
  page-break-inside:avoid;
  break-inside:avoid;
}

.finding-title{
  font-size:13.2pt;
  line-height:1.25;
  font-weight:700;
  margin:0 0 3mm 0;
}

.finding-meta{
  margin:2.5mm 0 3mm 0;
}

.finding-meta th{
  width:33mm;
}

.finding-meta th,
.finding-meta td{
  padding:1.6mm 2.5mm;
}

.finding.compact{
  margin-bottom:5.5mm;
}

.finding.compact .finding-title{
  font-size:12.4pt;
  margin-bottom:2.5mm;
}

.finding.compact p{
  margin-bottom:2.8mm;
}

.recommendations-title{
  font-weight:700;
  margin:3mm 0 2mm 0;
}

.badge{
  display:inline-block;
  padding:.8mm 2.2mm;
  border-radius:999px;
  font-size:8.2pt;
  font-weight:700;
  line-height:1.2;
}

.badge.Critical{ background:#7f1d1d; color:#fff; }
.badge.High{ background:#dc2626; color:#fff; }
.badge.Medium{ background:#f59e0b; color:#111; }
.badge.Low{ background:#2563eb; color:#fff; }
.badge.Info{ background:#64748b; color:#fff; }

.positive-table th,
.positive-table td{
  padding:1.8mm 2.4mm;
}

.footer{
  position:absolute;
  left:0;
  right:0;
  bottom:0;
  height:21mm;
  border-top:none;
  z-index:4;
  overflow:hidden;
}

.footer-bg{
  position:absolute;
  left:0;
  top:0;
  bottom:0;
  width:100%;
  clip-path:polygon(0 0, 84% 0, 100% 100%, 0% 100%);
  background:linear-gradient(90deg, rgba(128,218,215,.84), rgba(232,248,247,.96) 68%, rgba(255,255,255,.98) 100%);
  background-position:center;
  background-size:cover;
  background-repeat:no-repeat;
}

.footer-inner{
  position:relative;
  z-index:2;
  height:100%;
  display:grid;
  grid-template-columns:1.35fr 1fr 1fr 17mm;
  align-items:center;
  gap:6mm;
  padding:3.4mm 16mm 5.4mm 16mm;
}

.footer-item{
  font-size:8.8pt;
  line-height:1.15;
  white-space:nowrap;
  display:flex;
  align-items:center;
  gap:2mm;
}

.footer-item a{
  color:#1565c0;
  text-decoration:underline;
}

.footer-icon{
  width:4.2mm;
  height:4.2mm;
  color:#111827;
  flex:0 0 auto;
}

.page-no{
  text-align:right;
  font-size:9pt;
  white-space:nowrap;
}

.footer-copy{
  position:absolute;
  z-index:3;
  left:0;
  right:0;
  bottom:1.8mm;
  text-align:center;
  font-size:7.2pt;
  color:#2f3a44;
}

.section-note{
  padding:3mm 3.5mm;
  background:rgba(248,250,251,.88);
  border:1px solid #e4e9ee;
  border-radius:5px;
}

.flow-source{
  display:none;
}

.flow-block{
  page-break-inside:avoid;
  break-inside:avoid;
}

@page{
  size:A4;
  margin:0;
}

@media print{
  body{
    background:#fff;
  }
  .toolbar{
    display:none;
  }
  .page{
    margin:0;
    border:none;
  }
}
</style>
<script>
function saveHTML(){
  const content = '<!doctype html>\n' + document.documentElement.outerHTML;
  const blob = new Blob([content], {type:'text/html'});
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = document.title + '.html';
  a.click();
  URL.revokeObjectURL(a.href);
}

function paginateReport(){
  const pages = document.getElementById('pages');
  const source = document.getElementById('flow-source');
  const template = document.getElementById('page-template');

  if (!pages || !source || !template) return;

  function newPage(){
    const node = template.content.firstElementChild.cloneNode(true);
    pages.appendChild(node);
    return node.querySelector('.page-inner');
  }

  function overflows(container){
    return container.scrollHeight > container.clientHeight + 2;
  }

  let current = newPage();

  Array.from(source.children).forEach((original) => {
    const block = original.cloneNode(true);
    current.appendChild(block);

    if (overflows(current) && current.children.length > 1) {
      current.removeChild(block);
      current = newPage();
      current.appendChild(block);
    }
  });

  document.querySelectorAll('.page-number').forEach((el, idx) => {
    el.textContent = String(idx + 1);
  });
}

window.addEventListener('DOMContentLoaded', paginateReport);
</script>
</head>
<body>
<div class="toolbar no-print">
  <div class="title">` + esc(r.Meta.Client) + ` — Security Audit Report</div>
  <button onclick="window.print()">Save PDF</button>
  <a href="#" onclick="this.href=window.location.href.replace(/\.html($|[?#].*)/, '.docx$1')" download>Download DOCX</a>
</div>
<div id="pages" class="pages">
`)

	b.WriteString(renderCover(r, assets))

	b.WriteString(`</div>
<template id="page-template">
  <section class="page">
`)

	if assets.Watermark != "" {
		b.WriteString(`    <div class="watermark" style="background-image:url('` + escAttr(assets.Watermark) + `')"></div>
`)
	}

	b.WriteString(`    <div class="page-inner"></div>
`)
	b.WriteString(renderFooter(r, assets, "0"))
	b.WriteString(`
  </section>
</template>

<div id="flow-source" class="flow-source">
`)
	b.WriteString(sourceBlocksHTML(r))
	b.WriteString(`
</div>









</body>
</html>`)

	return os.WriteFile(path, []byte(b.String()), 0644)
}

func loadHTMLAssets(reportPath string, r model.Report) htmlAssets {
	baseDir := filepath.Dir(reportPath)

	return htmlAssets{
		Logo:      fileToDataURI(filepath.Join(baseDir, filepath.FromSlash(r.Meta.LogoPath))),
		Watermark: fileToDataURI(filepath.Join(baseDir, filepath.FromSlash(r.Meta.WatermarkPath))),
		FooterBg:  fileToDataURI(filepath.Join(baseDir, filepath.FromSlash(r.Meta.FooterBgPath))),
	}
}

func fileToDataURI(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	ext := strings.ToLower(filepath.Ext(path))
	mime := "image/png"

	switch ext {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".webp":
		mime = "image/webp"
	case ".svg":
		mime = "image/svg+xml"
	}

	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func renderCover(r model.Report, assets htmlAssets) string {
	var b strings.Builder

	b.WriteString(`<section class="page cover">`)

	if assets.Watermark != "" {
		b.WriteString(`<div class="watermark" style="background-image:url('` + escAttr(assets.Watermark) + `')"></div>`)
	}

	if assets.Logo != "" {
		b.WriteString(`<img class="cover-logo" src="` + escAttr(assets.Logo) + `" alt="">`)
	}

	b.WriteString(`<div class="page-inner">`)
	b.WriteString(`<div class="company">
  <div class="company-name">` + esc(r.Meta.AuditorOrg) + `</div>
  <div class="company-meta">` + nl2br(esc(r.Meta.AuditorAddress)) + `<br><a href="mailto:` + escAttr(r.Meta.AuditorEmail) + `">` + esc(r.Meta.AuditorEmail) + `</a></div>
</div>
<div class="cover-title">
  Security &amp;<br>
  Infrastructure Audit Report
</div>
<div class="cover-meta">
  <div class="cover-meta-row"><div class="cover-meta-label">Prepared for</div><div>` + esc(r.Meta.Client) + `</div></div>
  <div class="cover-meta-row"><div class="cover-meta-label">Project</div><div>` + esc(r.Meta.ProjectName) + `</div></div>
  <div class="cover-meta-row"><div class="cover-meta-label">Assessment period</div><div>` + esc(r.Meta.AssessmentPeriod) + `</div></div>
  <div class="cover-meta-row"><div class="cover-meta-label">Classification</div><div>` + esc(r.Meta.Classification) + `</div></div>
  <div class="cover-meta-row"><div class="cover-meta-label">Prepared by</div><div>` + esc(r.Meta.PreparedBy) + `</div></div>
</div>`)
	b.WriteString(`</div>`)

	b.WriteString(renderFooter(r, assets, "1"))
	b.WriteString(`</section>`)

	return b.String()
}

func renderFooter(r model.Report, assets htmlAssets, pageNo string) string {
	bg := ""
	if assets.FooterBg != "" {
		bg = ` style="background-image:linear-gradient(90deg, rgba(128,218,215,.84), rgba(232,248,247,.96) 68%, rgba(255,255,255,.98) 100%), url('` + escAttr(assets.FooterBg) + `')"`
	}

	mailIcon := `<svg class="footer-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/></svg>`
	webIcon := `<svg class="footer-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3c2.4 2.6 3.6 5.6 3.6 9s-1.2 6.4-3.6 9"/><path d="M12 3C9.6 5.6 8.4 8.6 8.4 12s1.2 6.4 3.6 9"/></svg>`
	phoneIcon := `<svg class="footer-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M22 16.9v3a2 2 0 0 1-2.2 2 19.8 19.8 0 0 1-8.6-3.1 19.4 19.4 0 0 1-6-6A19.8 19.8 0 0 1 2.1 4.2 2 2 0 0 1 4.1 2h3a2 2 0 0 1 2 1.7c.1.9.3 1.7.6 2.5a2 2 0 0 1-.5 2.1L8 9.5a16 16 0 0 0 6.5 6.5l1.2-1.2a2 2 0 0 1 2.1-.5c.8.3 1.6.5 2.5.6A2 2 0 0 1 22 16.9z"/></svg>`

	return `<footer class="footer">
  <div class="footer-bg"` + bg + `></div>
  <div class="footer-inner">
    <div class="footer-item">` + mailIcon + `<a href="mailto:` + escAttr(r.Meta.AuditorEmail) + `">` + esc(r.Meta.AuditorEmail) + `</a></div>
    <div class="footer-item">` + webIcon + `<a href="https://` + escAttr(r.Meta.AuditorWebsite) + `">` + esc(r.Meta.AuditorWebsite) + `</a></div>
    <div class="footer-item">` + phoneIcon + `<span>` + esc(r.Meta.AuditorPhone) + `</span></div>
    <div class="page-no">Page <span class="page-number">` + esc(pageNo) + `</span></div>
  </div>
  <div class="footer-copy">© 2026 ` + esc(orgNoTrailingDot(r.Meta.AuditorOrg)) + `. All rights reserved.</div>
</footer>`
}

func sourceBlocksHTML(r model.Report) string {
	var b strings.Builder

	b.WriteString(summaryBlock(r))

	if len(r.Findings) > 0 {
		b.WriteString(findingsRegisterBlocks(r))
		b.WriteString(findingsFlowBlocks(r))
	} else {
		b.WriteString(`<section class="flow-block"><h1>Findings</h1><div class="section-note">No security findings were generated from the available evidence.</div></section>`)
	}

	if len(r.Positives) > 0 {
		b.WriteString(positiveBlocks(r))
	}

	if len(r.Findings) > 0 {
		b.WriteString(roadmapBlock(r))
	}

	b.WriteString(methodologyBlock(r))

	return b.String()
}

func summaryBlock(r model.Report) string {
	counts := rules.CountBySeverity(r.Findings)

	var b strings.Builder
	b.WriteString(`<section class="flow-block">`)
	b.WriteString(`<h1>Summary</h1>`)

	summary := strings.TrimSpace(r.Summary)
	if summary == "" {
		summary = "The assessment was generated from exported cloud evidence and mapped to common control families from ISO/IEC 27001, NIST Cybersecurity Framework and CIS Controls."
	}
	b.WriteString(`<p>` + esc(summary) + `</p>`)

	b.WriteString(`<table class="summary-table">
<tr><th>Severity</th><th>Count</th><th>Topics</th></tr>`)

	for _, sev := range []string{"Critical", "High", "Medium", "Low"} {
		label := sev
		count := counts[sev]
		if sev == "Low" {
			label = "Low / Info"
			count += counts["Info"]
		}

		b.WriteString(`<tr><td><b>` + esc(label) + `</b></td><td>` + fmt.Sprint(count) + `</td><td>` + esc(topicsForSeverity(r.Findings, sev)) + `</td></tr>`)
	}

	b.WriteString(`</table>`)

	b.WriteString(`<h2>Evidence snapshot</h2>`)
	b.WriteString(evidenceSnapshot(r))

	b.WriteString(`<p class="small muted">Framework mapping: ISO/IEC 27001:2022, NIST Cybersecurity Framework, CIS Critical Security Controls and CIS Kubernetes Benchmark where applicable. Evidence collected at ` + esc(r.Inventory.CollectedAt) + `.</p>`)
	b.WriteString(`</section>`)

	return b.String()
}

func evidenceSnapshot(r model.Report) string {
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

	var b strings.Builder
	b.WriteString(`<table class="evidence-table">
<tr><th>Evidence area</th><th>Count</th><th>Evidence area</th><th>Count</th></tr>`)

	for i := 0; i < len(items); i += 2 {
		left := items[i]
		right := item{}
		if i+1 < len(items) {
			right = items[i+1]
		}

		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + esc(left.Label) + `</td><td>` + fmt.Sprint(resourceCount(r.Inventory.Resources[left.Key])) + `</td>`)

		if right.Label != "" {
			b.WriteString(`<td>` + esc(right.Label) + `</td><td>` + fmt.Sprint(resourceCount(r.Inventory.Resources[right.Key])) + `</td>`)
		} else {
			b.WriteString(`<td></td><td></td>`)
		}

		b.WriteString(`</tr>`)
	}

	b.WriteString(`</table>`)
	return b.String()
}

func findingsRegisterBlocks(r model.Report) string {
	var b strings.Builder

	b.WriteString(`<section class="flow-block">`)
	b.WriteString(`<h1>Findings Register</h1>`)
	b.WriteString(`<table class="register-table">
<tr><th>ID</th><th>Severity</th><th>Finding</th><th>Resource</th><th>Timeline</th></tr>`)

	for _, f := range r.Findings {
		b.WriteString(`<tr>
<td>` + esc(f.ID) + `</td>
<td><span class="badge ` + esc(f.Severity) + `">` + esc(f.Severity) + `</span></td>
<td>` + esc(f.Title) + `</td>
<td>` + esc(nonEmpty(f.ResourceName, "N/A")) + `</td>
<td>` + esc(nonEmpty(f.Timeline, "Review")) + `</td>
</tr>`)
	}

	b.WriteString(`</table>`)
	b.WriteString(`</section>`)

	return b.String()
}

func findingsFlowBlocks(r model.Report) string {
	var b strings.Builder

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

		for i, f := range group {
			b.WriteString(`<section class="flow-block">`)
			if i == 0 {
				b.WriteString(`<div class="severity-heading">` + esc(strings.ToUpper(sev)) + `</div>`)
			}
			b.WriteString(renderFinding(f))
			b.WriteString(`</section>`)
		}
	}

	return b.String()
}

func renderFinding(f model.Finding) string {
	component := "N/A"
	if len(f.AffectedComponents) > 0 {
		component = strings.Join(f.AffectedComponents, ", ")
	} else if strings.TrimSpace(f.ResourceName) != "" {
		component = f.ResourceName
	}

	compact := f.Severity == "Medium" || f.Severity == "Low" || f.Severity == "Info"

	var b strings.Builder
	if compact {
		b.WriteString(`<div class="finding compact">`)
	} else {
		b.WriteString(`<div class="finding">`)
	}

	b.WriteString(`<div class="finding-title">` + esc(f.ID) + `. ` + esc(f.Title) + `</div>`)

	if !compact {
		b.WriteString(`<table class="finding-meta">
<tr><th>Component</th><td>` + esc(component) + `</td></tr>
<tr><th>Finding</th><td>` + esc(nonEmpty(f.Evidence, f.Title)) + `</td></tr>
<tr><th>Status</th><td>` + esc(nonEmpty(f.Status, "Open")) + `</td></tr>
<tr><th>Standard</th><td>` + esc(nonEmpty(f.Standard, "ISO 27001 / NIST CSF / CIS Controls")) + `</td></tr>
</table>`)
	} else {
		b.WriteString(`<p class="small muted"><b>Component:</b> ` + esc(component) + ` · <b>Standard:</b> ` + esc(nonEmpty(f.Standard, "ISO 27001 / NIST CSF / CIS Controls")) + `</p>`)
	}

	if strings.TrimSpace(f.Risk) != "" {
		b.WriteString(`<p>` + esc(f.Risk) + `</p>`)
	}
	if strings.TrimSpace(f.BusinessImpact) != "" && !compact {
		b.WriteString(`<p>` + esc(f.BusinessImpact) + `</p>`)
	}

	b.WriteString(`<div class="recommendations-title">Recommendations:</div><ul>`)

	if strings.TrimSpace(f.Recommendation) != "" {
		b.WriteString(`<li>` + esc(f.Recommendation) + `</li>`)
	}
	if strings.TrimSpace(f.Remediation) != "" && f.Remediation != f.Recommendation && !compact {
		b.WriteString(`<li>` + esc(f.Remediation) + `</li>`)
	}
	if strings.TrimSpace(f.Validation) != "" && !compact {
		b.WriteString(`<li>` + esc(f.Validation) + `</li>`)
	}

	b.WriteString(`</ul></div>`)

	return b.String()
}

func positiveBlocks(r model.Report) string {
	var b strings.Builder

	b.WriteString(`<section class="flow-block">`)
	b.WriteString(`<h1>Positive Findings</h1>`)
	b.WriteString(`<p>The following positive observations were identified from the available evidence. These items do not prove the environment is fully secure, but they show useful foundations that support the security program.</p>`)

	b.WriteString(`<table class="positive-table"><tr><th>Area</th><th>Status</th><th>Evidence</th></tr>`)

	for _, p := range r.Positives {
		b.WriteString(`<tr><td>` + esc(p.Area) + `</td><td>` + esc(p.Status) + `</td><td>` + esc(p.Evidence) + `</td></tr>`)
	}

	b.WriteString(`</table>`)
	b.WriteString(`</section>`)

	return b.String()
}

func roadmapBlock(r model.Report) string {
	groups := []string{
		"Immediate / today",
		"Short term / within 1 week",
		"Medium term / within 1 month",
		"Ongoing / backlog",
		"Informational",
	}

	var b strings.Builder
	var hasItems bool

	b.WriteString(`<section class="flow-block">`)
	b.WriteString(`<h1>Recommended Actions</h1>`)

	for _, group := range groups {
		var items []model.Finding
		for _, f := range r.Findings {
			if strings.TrimSpace(f.Timeline) == group {
				items = append(items, f)
			}
		}

		if len(items) == 0 {
			continue
		}

		hasItems = true
		b.WriteString(`<h2>` + esc(group) + `</h2><ul>`)

		for _, f := range items {
			text := strings.TrimSpace(f.Remediation)
			if text == "" {
				text = strings.TrimSpace(f.Recommendation)
			}
			if text == "" {
				text = f.Title
			}

			b.WriteString(`<li><b>` + esc(f.ID) + `:</b> ` + esc(text) + `</li>`)
		}

		b.WriteString(`</ul>`)
	}

	b.WriteString(`</section>`)

	if !hasItems {
		return ""
	}

	return b.String()
}

func methodologyBlock(r model.Report) string {
	var b strings.Builder

	b.WriteString(`<section class="flow-block">`)
	b.WriteString(`<h1>Methodology & Limitations</h1>`)
	b.WriteString(`<p>The assessment is an automated, point-in-time configuration review based on cloud API evidence visible to the supplied audit token. Findings are mapped to common control families across asset management, identity and access, network protection, data protection, resilience, monitoring and secure configuration.</p>`)

	b.WriteString(`<h2>International standards alignment</h2><ul>`)
	for _, s := range r.Meta.Standards {
		b.WriteString(`<li>` + esc(s) + `</li>`)
	}
	b.WriteString(`</ul>`)

	b.WriteString(`<h2>Limitations</h2><ul>`)
	for _, l := range r.Limitations {
		b.WriteString(`<li>` + esc(l) + `</li>`)
	}
	b.WriteString(`</ul>`)
	b.WriteString(`</section>`)

	return b.String()
}

func topicsForSeverity(findings []model.Finding, sev string) string {
	var topics []string
	seen := map[string]bool{}

	for _, f := range findings {
		if f.Severity != sev && !(sev == "Low" && f.Severity == "Info") {
			continue
		}

		title := shortTopic(f.Title)
		if title == "" || seen[title] {
			continue
		}

		seen[title] = true
		topics = append(topics, title)
	}

	if len(topics) == 0 {
		return "None identified from available evidence"
	}

	if len(topics) > 4 {
		topics = topics[:4]
	}

	return strings.Join(topics, ", ")
}

func shortTopic(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))

	switch {
	case strings.Contains(t, "credential"):
		return "Sensitive credentials in raw evidence"
	case strings.Contains(t, "reserved ip"):
		return "Unattached reserved IP"
	case strings.Contains(t, "public droplet"):
		return "Public Droplet without effective cloud firewall"
	case strings.Contains(t, "firewall allows") && strings.Contains(t, "internet"):
		return "Internet-exposed administrative access"
	case strings.Contains(t, "firewall is defined") || (strings.Contains(t, "cloud firewall") && strings.Contains(t, "not attached")):
		return "Unattached cloud firewall policy"
	case strings.Contains(t, "standby"):
		return "Single-node production database"
	case strings.Contains(t, "backup"):
		return "Droplet backup coverage"
	case strings.Contains(t, "retired") || strings.Contains(t, "deprecated"):
		return "Retired or deprecated OS image"
	case strings.Contains(t, "maintenance"):
		return "Pending database maintenance"
	case strings.Contains(t, "supabase"):
		return "Supabase service-role key validation"
	case strings.Contains(t, "cors"):
		return "CORS configuration drift"
	case strings.Contains(t, "tags"):
		return "Missing asset tags"
	case strings.Contains(t, "autoscaling"):
		return "Single-instance application services"
	default:
		return strings.TrimSpace(title)
	}
}

func resourceCount(v interface{}) int {
	switch t := v.(type) {
	case []interface{}:
		return len(t)
	case map[string]interface{}:
		if registry, ok := t["registry"].(map[string]interface{}); ok {
			if fmt.Sprintf("%v", registry["name"]) != "" {
				return 1
			}
		}
		return len(t)
	default:
		return 0
	}
}

func orgNoTrailingDot(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), ".")
}

func nonEmpty(v string, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func esc(s string) string {
	return html.EscapeString(s)
}

func escAttr(s string) string {
	return html.EscapeString(s)
}

func nl2br(s string) string {
	return strings.ReplaceAll(s, "\n", "<br>")
}
