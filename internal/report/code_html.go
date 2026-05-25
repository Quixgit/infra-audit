package report

import (
	"fmt"
	"os"
	"strings"

	"infra-audit/internal/model"
)

func WriteCodeHTML(path string, r model.CodeReport) error {
	assets := loadHTMLAssets(path, model.Report{Meta: r.Meta})
	var b strings.Builder

	b.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + esc(r.Meta.ArtifactBase) + `</title>
<style>
/* Ubuntu via system fonts fallback */
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
*{ box-sizing:border-box; }
html,body{ margin:0; padding:0; background:var(--bg); color:var(--text); font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','Helvetica Neue',Arial,sans-serif; font-size:11pt; line-height:1.31; }
.toolbar{ position:sticky; top:0; z-index:999; display:flex; gap:10px; align-items:center; padding:9px 14px; background:#111827; color:#fff; box-shadow:0 4px 20px rgba(0,0,0,.12); }
.toolbar .title{ margin-right:auto; font-weight:700; font-size:13px; }
.toolbar button{ border:1px solid rgba(255,255,255,.18); background:#fff; color:#111827; border-radius:8px; font-size:12px; font-weight:700; padding:7px 11px; cursor:pointer; }
.pages{ width:100%; }
.page{ position:relative; width:var(--page-width); height:var(--page-height); margin:10mm auto; background:#fff; overflow:hidden; border:1px solid #d5dbe3; page-break-after:always; }
.page-inner{ position:relative; z-index:2; height:100%; padding:22mm 22mm 25mm 22mm; overflow:hidden; }
.cover .page-inner{ padding:24mm 22mm 25mm 22mm; }
.watermark{ position:absolute; z-index:0; left:10mm; right:10mm; top:28mm; bottom:24mm; background-repeat:no-repeat; background-position:center; background-size:92%; opacity:.57; pointer-events:none; }
.cover-logo{ position:absolute; z-index:3; top:21mm; right:23mm; width:34mm; height:auto; }
.company{ position:relative; z-index:3; margin-top:7mm; max-width:86mm; }
.company-name{ font-size:25pt; font-weight:400; line-height:1.05; margin:0 0 9mm 0; }
.company-meta{ font-size:11pt; line-height:1.3; }
.cover-title{ position:absolute; z-index:3; left:22mm; right:22mm; top:96mm; text-align:center; font-size:23pt; line-height:1.22; font-weight:400; }
.cover-meta{ position:absolute; z-index:3; left:42mm; right:42mm; top:143mm; border-top:1px solid rgba(17,24,39,.55); border-bottom:1px solid rgba(17,24,39,.55); padding:6mm 0; font-size:10.5pt; }
.cover-meta-row{ display:grid; grid-template-columns:39mm 1fr; gap:6mm; margin-bottom:3mm; }
.cover-meta-row:last-child{ margin-bottom:0; }
.cover-meta-label{ color:#4b5563; font-weight:700; }
h1{ margin:0 0 8mm 0; font-size:19pt; line-height:1.18; font-weight:400; }
h2{ margin:6mm 0 3.5mm 0; font-size:13.5pt; font-weight:700; }
h3{ margin:5mm 0 2.5mm 0; font-size:12pt; font-weight:700; }
p{ margin:0 0 3.6mm 0; }
.small{ font-size:9pt; }
.muted{ color:var(--muted); }
ul{ margin:0 0 3.8mm 0; padding-left:6mm; }
li{ margin-bottom:1.6mm; }
table{ width:100%; border-collapse:collapse; margin:4mm 0; }
th,td{ text-align:left; vertical-align:top; padding:2mm 2.6mm; border-bottom:1px solid var(--line); border-right:1px solid var(--line); }
th:last-child,td:last-child{ border-right:none; }
th{ font-weight:700; background:rgba(248,250,252,.82); }
.badge{ display:inline-block; padding:.8mm 2.2mm; border-radius:999px; font-size:8.2pt; font-weight:700; line-height:1.2; }
.badge.Critical{ background:#7f1d1d; color:#fff; }
.badge.High{ background:#dc2626; color:#fff; }
.badge.Medium{ background:#f59e0b; color:#111; }
.badge.Low{ background:#2563eb; color:#fff; }
.badge.Info{ background:#64748b; color:#fff; }
.finding{ margin-bottom:7mm; page-break-inside:avoid; }
.finding-title{ font-size:12pt; font-weight:400; margin:0 0 3mm 0; } .finding-rule{ font-size:9pt; font-weight:400; color:#4b5563; margin-bottom:1mm; }
.finding-meta th{ width:33mm; }
.finding-meta th,.finding-meta td{ padding:1.6mm 2.5mm; }
.finding.compact{ margin-bottom:5mm; }
.finding.compact .finding-title{ font-size:12.4pt; }
.stack-tag{ display:inline-block; background:#e0f2fe; color:#0369a1; border-radius:4px; padding:1mm 3mm; font-size:9pt; font-weight:700; margin-right:2mm; }
.footer{ position:absolute; left:0; right:0; bottom:0; height:21mm; z-index:4; overflow:hidden; }
.footer-bg{ position:absolute; left:0; top:0; bottom:0; width:100%; clip-path:polygon(0 0, 84% 0, 100% 100%, 0% 100%); background:linear-gradient(90deg, rgba(128,218,215,.84), rgba(232,248,247,.96) 68%, rgba(255,255,255,.98) 100%); }
.footer-inner{ position:relative; z-index:2; height:100%; display:grid; grid-template-columns:1.35fr 1fr 1fr 17mm; align-items:center; gap:6mm; padding:3.4mm 16mm 5.4mm 16mm; }
.footer-item{ font-size:8.8pt; line-height:1.15; white-space:nowrap; display:flex; align-items:center; gap:2mm; }
.page-no{ text-align:right; font-size:9pt; }
.footer-copy{ position:absolute; z-index:3; left:0; right:0; bottom:1.8mm; text-align:center; font-size:7.2pt; color:#2f3a44; }
.flow-source{ width:100%; }
.flow-block{ position:relative; width:var(--page-width); margin:10mm auto; background:#fff; border:1px solid #d5dbe3; padding:22mm 22mm 25mm 22mm; page-break-after:always; overflow:hidden; }
.footer-icon{ width:4.2mm; height:4.2mm; flex:0 0 auto; }
@page{ size:A4; margin:0; }
@media print{ body{ background:#fff; } .toolbar{ display:none; } .page{ margin:0; border:none; } }
</style>
</head>
<body>
<div class="toolbar">
  <div class="title">` + esc(r.Meta.Client) + ` — Code Security Audit</div>
  <button onclick="window.print()">Save PDF</button>
</div>
<div class="pages">
`)

	b.WriteString(renderCodeCover(r, assets))

	b.WriteString(`</div>
<template id="page-template">
  <section class="page">
`)
	if assets.Watermark != "" {
		b.WriteString(`    <div class="watermark" style="background-image:url('` + escAttr(assets.Watermark) + `')"></div>`)
	}
	b.WriteString(`    <div class="page-inner"></div>
`)
	b.WriteString(renderFooter(model.Report{Meta: r.Meta}, assets, "0"))
	b.WriteString(`
  </section>
</template>
<div id="flow-source" class="flow-source">
`)
	b.WriteString(codeSourceBlocks(r))
	b.WriteString(`
</div>
<script>
function paginateReport(){
  const pages=document.getElementById('pages');
  const source=document.getElementById('flow-source');
  const template=document.getElementById('page-template');
  if(!pages||!source||!template)return;
  function newPage(){const node=template.content.firstElementChild.cloneNode(true);pages.appendChild(node);return node.querySelector('.page-inner');}
  function overflows(c){return c.scrollHeight>c.clientHeight+2;}
  let current=newPage();
  Array.from(source.children).forEach(orig=>{
    const block=orig.cloneNode(true);
    current.appendChild(block);
    if(overflows(current)&&current.children.length>1){current.removeChild(block);current=newPage();current.appendChild(block);}
  });
  document.querySelectorAll('.page-number').forEach((el,i)=>el.textContent=String(i+1));
}
window.addEventListener('DOMContentLoaded',paginateReport);
</script>
</body></html>`)

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func renderCodeCover(r model.CodeReport, assets htmlAssets) string {
	var b strings.Builder
	b.WriteString(`<section class="page cover">`)
	if assets.Watermark != "" {
		b.WriteString(`<div class="watermark" style="background-image:url('` + escAttr(assets.Watermark) + `')"></div>`)
	}
	if assets.Logo != "" {
		b.WriteString(`<img class="cover-logo" src="` + escAttr(assets.Logo) + `" alt="">`)
	}
	b.WriteString(`<div class="page-inner">
<div class="company">
  <div class="company-name">` + esc(r.Meta.AuditorOrg) + `</div>
  <div class="company-meta">` + nl2br(esc(r.Meta.AuditorAddress)) + `<br><a href="mailto:` + escAttr(r.Meta.AuditorEmail) + `">` + esc(r.Meta.AuditorEmail) + `</a></div>
</div>
<div class="cover-title">Code &amp; Infrastructure<br>Security Audit Report</div>
<div class="cover-meta">
  <div class="cover-meta-row"><div class="cover-meta-label">Prepared for</div><div>` + esc(r.Meta.Client) + `</div></div>
  <div class="cover-meta-row"><div class="cover-meta-label">Project</div><div>` + esc(r.Meta.ProjectName) + `</div></div>
  <div class="cover-meta-row"><div class="cover-meta-label">Assessment period</div><div>` + esc(r.Meta.AssessmentPeriod) + `</div></div>
  <div class="cover-meta-row"><div class="cover-meta-label">Classification</div><div>` + esc(r.Meta.Classification) + `</div></div>
  <div class="cover-meta-row"><div class="cover-meta-label">Prepared by</div><div>` + esc(r.Meta.PreparedBy) + `</div></div>
</div>
</div>`)
	b.WriteString(renderFooter(model.Report{Meta: r.Meta}, assets, "1"))
	b.WriteString(`</section>`)
	return b.String()
}

func codeSourceBlocks(r model.CodeReport) string {
	var b strings.Builder

	// Summary
	b.WriteString(`<section class="flow-block">`)
	b.WriteString(`<h1>Summary</h1>`)
	b.WriteString(`<p>` + esc(codeExecSummary(r)) + `</p>`)

	// Stack
	b.WriteString(`<p><strong>Technology stack detected:</strong> `)
	for _, s := range r.Stack {
		b.WriteString(`<span class="stack-tag">` + esc(s) + `</span>`)
	}
	b.WriteString(`</p>`)

	// Severity table
	cc := countCodeBySeverity(r.Findings)
	tc := countCodeBySeverity(r.TFFindings)
	b.WriteString(`<table>
<tr><th>Category</th><th>Critical</th><th>High</th><th>Medium</th><th>Low</th><th>Info</th><th>Total</th></tr>
<tr><td>Code Security</td>
<td>` + fmt.Sprint(cc["Critical"]) + `</td>
<td>` + fmt.Sprint(cc["High"]) + `</td>
<td>` + fmt.Sprint(cc["Medium"]) + `</td>
<td>` + fmt.Sprint(cc["Low"]) + `</td>
<td>` + fmt.Sprint(cc["Info"]) + `</td>
<td>` + fmt.Sprint(len(r.Findings)) + `</td></tr>
<tr><td>Infrastructure (Terraform)</td>
<td>` + fmt.Sprint(tc["Critical"]) + `</td>
<td>` + fmt.Sprint(tc["High"]) + `</td>
<td>` + fmt.Sprint(tc["Medium"]) + `</td>
<td>` + fmt.Sprint(tc["Low"]) + `</td>
<td>` + fmt.Sprint(tc["Info"]) + `</td>
<td>` + fmt.Sprint(len(r.TFFindings)) + `</td></tr>
</table>`)
	b.WriteString(`</section>`)

	// Code findings
	if len(r.Findings) > 0 {
		first := true
		
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
			b.WriteString(`<section class="flow-block">`)
			if first {
				b.WriteString(`<h1>Code Security Findings</h1>`)
				first = false
			}
			b.WriteString(`<div style="font-size:18pt;margin:0 0 6mm 0;font-weight:400">` + strings.ToUpper(sev) + `</div>`)
			for i, f := range group {
				b.WriteString(renderCodeFinding(f, i+1))
			}
			b.WriteString(`</section>`)
		}
	}

	// Terraform findings
	if len(r.TFFindings) > 0 {
		firstTF := true
		
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
			b.WriteString(`<section class="flow-block">`)
			if firstTF {
				b.WriteString(`<h1>Infrastructure Security Findings (Terraform)</h1>`)
				firstTF = false
			}
			b.WriteString(`<div style="font-size:18pt;margin:0 0 6mm 0;font-weight:400">` + strings.ToUpper(sev) + `</div>`)
			for i, f := range group {
				b.WriteString(renderCodeFinding(f, i+1))
			}
			b.WriteString(`</section>`)
		}
	}

	// Methodology
	b.WriteString(`<section class="flow-block">`)
	b.WriteString(`<h1>Methodology &amp; Limitations</h1>`)
	b.WriteString(`<p>This assessment combines automated static analysis tools with manual review of infrastructure-as-code. The following tools were used:</p>`)
	b.WriteString(`<ul>
<li><strong>gitleaks</strong> — secret and credential detection across the repository history and working tree</li>
<li><strong>semgrep</strong> — static analysis using community and custom rules for Node.js, TypeScript, and Docker</li>
<li><strong>trivy</strong> — infrastructure misconfiguration scanning for Terraform</li>
<li><strong>hclscan</strong> — custom DigitalOcean-specific Terraform rule engine</li>
<li><strong>npm audit</strong> — dependency vulnerability scanning</li>
</ul>`)
	b.WriteString(`<h2>Standards alignment</h2><ul>`)
	for _, s := range r.Meta.Standards {
		b.WriteString(`<li>` + esc(s) + `</li>`)
	}
	b.WriteString(`</ul>`)
	b.WriteString(`<h2>Limitations</h2>
<ul>
<li>This is a point-in-time static analysis. Dynamic vulnerabilities (runtime, authentication bypass, business logic) require manual penetration testing.</li>
<li>npm audit returned no vulnerabilities at the time of assessment. Dependencies should be re-checked before each release.</li>
<li>Terraform findings are based on static HCL analysis. Actual deployed state may differ.</li>
</ul>`)
	b.WriteString(`</section>`)

	return b.String()
}

func renderCodeFinding(f model.CodeFinding, idx int) string {
	compact := f.Severity == "Medium" || f.Severity == "Low" || f.Severity == "Info"
	var b strings.Builder

	cls := "finding"
	if compact {
		cls += " compact"
	}
	b.WriteString(`<div class="` + cls + `">`)

	// Rule ID отдельно маленьким текстом
	ruleStr := ""
	if f.RuleID != "" {
		ruleStr = esc(f.RuleID)
	}
	b.WriteString(`<div style="margin-bottom:1.5mm">`)
	b.WriteString(`<span class="badge ` + esc(f.Severity) + `">` + esc(f.Severity) + `</span>`)
	if ruleStr != "" {
		b.WriteString(` <span class="small muted">` + ruleStr + `</span>`)
	}
	b.WriteString(`</div>`)

	// Title нормальным весом
	b.WriteString(`<div class="finding-title" style="font-weight:500">` + esc(f.Title) + `</div>`)

	if !compact {
		b.WriteString(`<table class="finding-meta">
<tr><th>File</th><td>` + esc(f.File))
		if f.Line > 0 {
			b.WriteString(` <span class="muted small">line ` + fmt.Sprint(f.Line) + `</span>`)
		}
		b.WriteString(`</td></tr>
<tr><th>Tool</th><td>` + esc(f.Tool) + `</td></tr>`)
		if f.Category != "" {
			b.WriteString(`<tr><th>Category</th><td>` + esc(f.Category) + `</td></tr>`)
		}
		b.WriteString(`</table>`)
		if f.Description != "" {
			b.WriteString(`<p>` + esc(f.Description) + `</p>`)
		}
	} else {
		loc := f.File
		if f.Line > 0 {
			loc += fmt.Sprintf(":%d", f.Line)
		}
		b.WriteString(`<p class="small muted">` + esc(loc) + ` · ` + esc(f.Tool) + `</p>`)
	}

	if f.Remediation != "" {
		b.WriteString(`<div style="font-weight:500;margin:3mm 0 2mm 0;font-size:10pt">Remediation:</div>`)
		b.WriteString(`<ul><li>` + esc(f.Remediation) + `</li></ul>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func codeExecSummary(r model.CodeReport) string {
	cc := countCodeBySeverity(r.Findings)
	tc := countCodeBySeverity(r.TFFindings)
	total := len(r.Findings) + len(r.TFFindings)
	return fmt.Sprintf(
		"The assessment identified %d findings across code and infrastructure: %d Critical, %d High, %d Medium, %d Low/Info in application code; and %d High, %d Medium, %d Low/Info in Terraform infrastructure code. Priority actions should focus on removing committed secrets, fixing container security, and addressing hardcoded identifiers in Terraform.",
		total,
		cc["Critical"], cc["High"], cc["Medium"], cc["Low"]+cc["Info"],
		tc["High"], tc["Medium"], tc["Low"]+tc["Info"],
	)
}

func countCodeBySeverity(findings []model.CodeFinding) map[string]int {
	m := map[string]int{"Critical": 0, "High": 0, "Medium": 0, "Low": 0, "Info": 0}
	for _, f := range findings {
		m[f.Severity]++
	}
	return m
}
