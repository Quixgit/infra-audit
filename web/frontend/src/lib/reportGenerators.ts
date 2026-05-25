import type { AggregatedFinding, ComplianceFramework, AuditJob, Connection } from './api'

// ── Shared styles ─────────────────────────────────────────────────────────────

const CSS = `
:root { color-scheme: light; }
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
       color: #0f172a; background: #fff; font-size: 13px; line-height: 1.6; }
header { background: #0f172a; color: #fff; padding: 20px 48px;
         display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; }
header h1 { font-size: 19px; font-weight: 700; margin-bottom: 3px; }
.tagline { font-size: 10px; text-transform: uppercase; letter-spacing: .12em; opacity: .45; margin-bottom: 5px; }
.subtitle { font-size: 12px; opacity: .6; margin-top: 4px; }
.meta { text-align: right; font-size: 11px; opacity: .55; line-height: 1.7; white-space: nowrap; }
main { max-width: 1060px; margin: 0 auto; padding: 32px 48px 72px; }
h2 { font-size: 14px; font-weight: 700; margin: 28px 0 12px; padding-bottom: 8px;
     border-bottom: 2px solid #f1f5f9; color: #0f172a; letter-spacing: -.01em; }
h2:first-child { margin-top: 0; }
h3 { font-size: 13px; font-weight: 600; margin: 16px 0 8px; color: #1e293b; }
p { color: #475569; margin-bottom: 8px; font-size: 13px; }
small { font-size: 11px; color: #94a3b8; }
code { font-family: 'SFMono-Regular', Consolas, monospace; font-size: 11px;
       background: #f1f5f9; padding: 1px 5px; border-radius: 3px; }
table { width: 100%; border-collapse: collapse; margin-bottom: 24px; font-size: 12px; }
thead tr { background: #f8fafc; }
th { padding: 8px 13px; text-align: left; font-size: 10px; font-weight: 700;
     text-transform: uppercase; letter-spacing: .06em; color: #64748b;
     border-bottom: 2px solid #e2e8f0; white-space: nowrap; }
td { padding: 9px 13px; border-bottom: 1px solid #f1f5f9; vertical-align: top; }
tr:last-child td { border-bottom: none; }
tbody tr:hover { background: #f8fafc; }
.badge { display: inline-block; padding: 2px 9px; border-radius: 20px;
         font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; white-space: nowrap; }
.met     { background: #dcfce7; color: #15803d; }
.partial { background: #fef9c3; color: #a16207; }
.not-met { background: #fee2e2; color: #b91c1c; }
.critical { background: #fee2e2; color: #b91c1c; }
.high     { background: #ffedd5; color: #c2410c; }
.medium   { background: #fef3c7; color: #b45309; }
.low      { background: #dbeafe; color: #1d4ed8; }
.open       { background: #fee2e2; color: #b91c1c; }
.in-progress { background: #fef3c7; color: #b45309; }
.fixed      { background: #dcfce7; color: #15803d; }
.accepted-risk { background: #ede9fe; color: #6d28d9; }
.false-positive { background: #f1f5f9; color: #475569; }
.stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px,1fr)); gap: 12px; margin-bottom: 28px; }
.stat { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; padding: 16px 12px; text-align: center; }
.stat-value { font-size: 30px; font-weight: 800; line-height: 1; }
.stat-label { font-size: 11px; color: #64748b; margin-top: 6px; }
.callout { background: #f8fafc; border-left: 3px solid #94a3b8; padding: 11px 16px; margin-bottom: 16px; border-radius: 0 6px 6px 0; }
.callout.warn { border-color: #f59e0b; background: #fffbeb; }
.callout.good { border-color: #22c55e; background: #f0fdf4; }
.progress-wrap { height: 7px; background: #e2e8f0; border-radius: 4px; overflow: hidden; margin-top: 5px; }
.progress-fill { height: 100%; border-radius: 4px; }
.green  { background: #22c55e; }
.yellow { background: #f59e0b; }
.red    { background: #ef4444; }
footer { margin-top: 40px; padding-top: 14px; border-top: 1px solid #e2e8f0;
         font-size: 11px; color: #94a3b8; }
.print-btn { position: fixed; top: 14px; right: 14px; background: #0f172a; color: #fff;
             border: none; padding: 9px 18px; border-radius: 6px; cursor: pointer;
             font-size: 12px; font-weight: 600; z-index: 100; box-shadow: 0 2px 8px rgba(0,0,0,.25); }
.print-btn:hover { background: #1e293b; }
@media print { .print-btn { display: none; } @page { margin: 18mm 20mm; } }
`

// ── Helpers ───────────────────────────────────────────────────────────────────

function sevRank(s: string): number {
  return ({ critical: 4, high: 3, medium: 2, low: 1 } as Record<string, number>)[s?.toLowerCase()] ?? 0
}

function sevBadge(s: string) {
  return `<span class="badge ${s.toLowerCase()}">${s}</span>`
}

function statusBadge(s: string) {
  const cls: Record<string, string> = {
    met: 'met', partial: 'partial', not_met: 'not-met',
    open: 'open', in_progress: 'in-progress', fixed: 'fixed',
    accepted_risk: 'accepted-risk', false_positive: 'false-positive',
  }
  const label: Record<string, string> = {
    met: 'Met', partial: 'Partial', not_met: 'Not Met',
    open: 'Open', in_progress: 'In Progress', fixed: 'Fixed',
    accepted_risk: 'Accepted Risk', false_positive: 'False Positive',
  }
  return `<span class="badge ${cls[s] ?? ''}">${label[s] ?? s}</span>`
}

function esc(s: string | undefined) {
  return (s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function filterFindings(findings: AggregatedFinding[], connFilter: string) {
  return connFilter === 'all' ? findings : findings.filter(f => f.connection_id === connFilter)
}

function connScope(connFilter: string, connName: string) {
  return connFilter === 'all' ? 'All connections' : connName
}

function wrap(title: string, subtitle: string, scope: string, date: string, body: string) {
  return `<!DOCTYPE html>
<html lang="en"><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>${esc(title)}</title>
<style>${CSS}</style>
</head><body>
<header>
  <div>
    <div class="tagline">CloudSecGuard Security Platform</div>
    <h1>${esc(title)}</h1>
    <div class="subtitle">${esc(subtitle)}</div>
  </div>
  <div class="meta">
    <div><strong>Scope:</strong> ${esc(scope)}</div>
    <div><strong>Generated:</strong> ${esc(date)}</div>
  </div>
</header>
<main>${body}</main>
<button class="print-btn" onclick="window.print()">🖨&nbsp; Print / Save as PDF</button>
</body></html>`
}

export function openReport(html: string) {
  const win = window.open('', '_blank')
  if (!win) { alert('Allow popups to generate reports.'); return }
  win.document.write(html)
  win.document.close()
}

// ── 1. Framework Readiness Report (SOC 2, ISO 27001, etc.) ───────────────────

export function generateFrameworkReport(
  fw: ComplianceFramework,
  connFilter: string,
  connName: string,
  date: string,
): string {
  const controls = fw.controls ?? []
  const scope = connScope(connFilter, connName)

  const met = controls.filter(c => c.status === 'met').length
  const partial = controls.filter(c => c.status === 'partial').length
  const notMet = controls.filter(c => c.status === 'not_met').length
  const scoreColor = fw.score >= 80 ? '#15803d' : fw.score >= 50 ? '#a16207' : '#b91c1c'
  const barCls = fw.score >= 80 ? 'green' : fw.score >= 50 ? 'yellow' : 'red'

  const statsHtml = `
<div class="stat-grid">
  <div class="stat"><div class="stat-value" style="color:${scoreColor}">${fw.score}%</div><div class="stat-label">Readiness Score</div></div>
  <div class="stat"><div class="stat-value" style="color:#15803d">${met}</div><div class="stat-label">Controls Met</div></div>
  <div class="stat"><div class="stat-value" style="color:#a16207">${partial}</div><div class="stat-label">Partial</div></div>
  <div class="stat"><div class="stat-value" style="color:#b91c1c">${notMet}</div><div class="stat-label">Not Met</div></div>
  <div class="stat"><div class="stat-value">${controls.length}</div><div class="stat-label">Total Controls</div></div>
</div>
<div class="progress-wrap" style="max-width:400px;margin-bottom:28px">
  <div class="progress-fill ${barCls}" style="width:${fw.score}%"></div>
</div>`

  // Controls table
  let ctrlRows = ''
  for (const c of controls) {
    const openStr = c.open_count > 0
      ? `<strong style="color:#b91c1c">${c.open_count}</strong>`
      : `<span style="color:#94a3b8">0</span>`
    ctrlRows += `<tr>
      <td><code>${esc(c.ctrl_id)}</code></td>
      <td><strong>${esc(c.name)}</strong><br><small>${esc(c.description)}</small></td>
      <td>${statusBadge(c.status)}</td>
      <td style="text-align:center">${c.finding_count}</td>
      <td style="text-align:center">${openStr}</td>
    </tr>`
  }

  const ctrlTable = `
<h2>Control Status</h2>
<table>
  <thead><tr><th width="100">Control</th><th>Name &amp; Description</th><th width="90">Status</th><th width="70" style="text-align:center">Findings</th><th width="70" style="text-align:center">Open</th></tr></thead>
  <tbody>${ctrlRows}</tbody>
</table>`

  // Gap section — not_met controls with their open findings
  const gapControls = controls.filter(c => c.status !== 'met' && c.open_count > 0)
  let gapSection = ''
  if (gapControls.length > 0) {
    gapSection = '<h2>Gap Details &amp; Findings</h2>'
    for (const c of gapControls) {
      const openF = (c.findings ?? []).filter(f => f.status === 'open' || f.status === 'in_progress')
      if (!openF.length) continue
      gapSection += `<h3>${esc(c.ctrl_id)} — ${esc(c.name)}</h3>`
      let fRows = ''
      for (const f of [...openF].sort((a, b) => sevRank(b.severity) - sevRank(a.severity))) {
        const resource = f.file ? `${esc(f.file)}${f.line ? ':' + f.line : ''}` : esc(f.resource_name)
        const rec = esc(f.recommendation || f.remediation || '')
        fRows += `<tr>
          <td>${sevBadge(f.severity)}</td>
          <td><strong>${esc(f.title)}</strong>${rec ? `<br><small style="color:#64748b">${rec}</small>` : ''}</td>
          <td><code>${resource || '—'}</code></td>
          <td>${esc(f.connection_name)}</td>
          <td>${statusBadge(f.status)}</td>
        </tr>`
      }
      gapSection += `<table>
        <thead><tr><th width="80">Severity</th><th>Finding &amp; Recommendation</th><th width="180">Resource</th><th width="150">Connection</th><th width="90">Status</th></tr></thead>
        <tbody>${fRows}</tbody>
      </table>`
    }
  } else {
    gapSection = '<div class="callout good"><strong>✓ No open findings mapped to this framework.</strong> All controls are met or have no associated findings.</div>'
  }

  const footer = `<footer>
    <p>This report was generated automatically by CloudSecGuard based on findings from completed audits${connFilter !== 'all' ? ` for connection <strong>${esc(connName)}</strong>` : ' across all connections'}. Scores reflect open findings mapped to ${esc(fw.name)} ${esc(fw.version)} controls. This is an automated readiness estimate — not a substitute for a formal audit engagement.</p>
  </footer>`

  const body = statsHtml + ctrlTable + gapSection + footer
  return wrap(`${fw.name} Readiness Report`, `${fw.name} ${fw.version} — Automated Readiness Assessment`, scope, date, body)
}

// ── 2. Executive Summary ──────────────────────────────────────────────────────

export function generateExecutiveSummary(
  findings: AggregatedFinding[],
  frameworks: ComplianceFramework[],
  jobs: AuditJob[],
  connFilter: string,
  connName: string,
  date: string,
): string {
  const filtered = filterFindings(findings, connFilter)
  const doneJobs = jobs.filter(j => j.status === 'done' && (connFilter === 'all' || j.connection_id === connFilter))
  const scope = connScope(connFilter, connName)

  const bySev = filtered.reduce((acc, f) => {
    const s = f.severity?.toLowerCase() ?? 'unknown'
    acc[s] = (acc[s] ?? 0) + 1; return acc
  }, {} as Record<string, number>)

  const openFindings = filtered.filter(f => f.status === 'open' || f.status === 'in_progress')
  const resolvedFindings = filtered.filter(f => f.status === 'fixed' || f.status === 'accepted_risk' || f.status === 'false_positive')

  const statsHtml = `
<div class="stat-grid">
  <div class="stat"><div class="stat-value">${doneJobs.length}</div><div class="stat-label">Completed Audits</div></div>
  <div class="stat"><div class="stat-value">${filtered.length}</div><div class="stat-label">Total Findings</div></div>
  <div class="stat"><div class="stat-value" style="color:#b91c1c">${bySev.critical ?? 0}</div><div class="stat-label">Critical</div></div>
  <div class="stat"><div class="stat-value" style="color:#c2410c">${bySev.high ?? 0}</div><div class="stat-label">High</div></div>
  <div class="stat"><div class="stat-value" style="color:#b45309">${bySev.medium ?? 0}</div><div class="stat-label">Medium</div></div>
  <div class="stat"><div class="stat-value" style="color:#1d4ed8">${bySev.low ?? 0}</div><div class="stat-label">Low</div></div>
  <div class="stat"><div class="stat-value" style="color:#15803d">${resolvedFindings.length}</div><div class="stat-label">Resolved</div></div>
</div>`

  // Top open findings
  const topFindings = [...openFindings]
    .sort((a, b) => sevRank(b.severity) - sevRank(a.severity))
    .slice(0, 15)

  let topRows = ''
  for (const f of topFindings) {
    const resource = f.file ? `${esc(f.file)}${f.line ? ':' + f.line : ''}` : esc(f.resource_name)
    topRows += `<tr>
      <td>${sevBadge(f.severity)}</td>
      <td><strong>${esc(f.title)}</strong>${f.category ? `<br><small>${esc(f.category)}</small>` : ''}</td>
      <td><code>${resource || '—'}</code></td>
      <td>${esc(f.connection_name)}</td>
      <td>${statusBadge(f.status)}</td>
    </tr>`
  }

  const topTable = topRows ? `
<h2>Top Open Findings</h2>
<table>
  <thead><tr><th width="80">Severity</th><th>Title</th><th width="200">Resource</th><th width="160">Connection</th><th width="100">Status</th></tr></thead>
  <tbody>${topRows}</tbody>
</table>` : ''

  // Compliance overview
  const fwNames: Record<string, string> = {
    soc2: 'SOC 2', iso27001: 'ISO 27001', 'nist-csf': 'NIST CSF',
    'cis-v8': 'CIS v8', hipaa: 'HIPAA', 'pci-dss': 'PCI DSS', gdpr: 'GDPR',
  }
  let fwRows = ''
  for (const fw of frameworks) {
    const barCls = fw.score >= 80 ? 'green' : fw.score >= 50 ? 'yellow' : 'red'
    const scoreColor = fw.score >= 80 ? '#15803d' : fw.score >= 50 ? '#a16207' : '#b91c1c'
    fwRows += `<tr>
      <td><strong>${esc(fwNames[fw.slug] ?? fw.slug)}</strong><br><small>${esc(fw.name)} ${esc(fw.version)}</small></td>
      <td style="width:200px">
        <div style="display:flex;align-items:center;gap:8px">
          <div class="progress-wrap" style="flex:1"><div class="progress-fill ${barCls}" style="width:${fw.score}%"></div></div>
          <strong style="color:${scoreColor};width:36px;text-align:right">${fw.score}%</strong>
        </div>
      </td>
      <td style="text-align:center">${fw.met_count}/${fw.total_count}</td>
    </tr>`
  }

  const fwTable = fwRows ? `
<h2>Compliance Readiness Overview</h2>
<table>
  <thead><tr><th>Framework</th><th>Score</th><th width="90" style="text-align:center">Controls Met</th></tr></thead>
  <tbody>${fwRows}</tbody>
</table>` : ''

  // Audit history
  let auditRows = ''
  for (const j of doneJobs.slice(0, 10)) {
    const d = new Date(j.started_at).toLocaleDateString()
    auditRows += `<tr>
      <td>${esc(j.connection_name)}</td>
      <td>${d}</td>
      <td><span class="badge critical">${j.findings_critical}</span> <span class="badge high">${j.findings_high}</span> <span class="badge medium">${j.findings_medium}</span> <span class="badge low">${j.findings_low}</span></td>
    </tr>`
  }

  const auditTable = auditRows ? `
<h2>Audit History</h2>
<table>
  <thead><tr><th>Connection</th><th width="120">Date</th><th>Findings (C / H / M / L)</th></tr></thead>
  <tbody>${auditRows}</tbody>
</table>` : ''

  const footer = `<footer>
    <p>Executive summary generated by CloudSecGuard for scope: <strong>${esc(scope)}</strong>. Findings counts reflect the current status of all completed audits. Compliance scores are automated estimates based on finding-to-control mappings.</p>
  </footer>`

  const body = statsHtml + topTable + fwTable + auditTable + footer
  return wrap('Executive Summary', 'Security posture overview across all completed audits', scope, date, body)
}

// ── 3. Remediation Plan ───────────────────────────────────────────────────────

export function generateRemediationPlan(
  findings: AggregatedFinding[],
  connFilter: string,
  connName: string,
  date: string,
): string {
  const open = filterFindings(findings, connFilter)
    .filter(f => f.status === 'open' || f.status === 'in_progress')
    .sort((a, b) => sevRank(b.severity) - sevRank(a.severity))

  const scope = connScope(connFilter, connName)

  const bySev = open.reduce((acc, f) => {
    const s = f.severity?.toLowerCase() ?? 'unknown'
    acc[s] = (acc[s] ?? 0) + 1; return acc
  }, {} as Record<string, number>)

  const statsHtml = `
<div class="stat-grid">
  <div class="stat"><div class="stat-value">${open.length}</div><div class="stat-label">Open Findings</div></div>
  <div class="stat"><div class="stat-value" style="color:#b91c1c">${bySev.critical ?? 0}</div><div class="stat-label">Critical</div></div>
  <div class="stat"><div class="stat-value" style="color:#c2410c">${bySev.high ?? 0}</div><div class="stat-label">High</div></div>
  <div class="stat"><div class="stat-value" style="color:#b45309">${bySev.medium ?? 0}</div><div class="stat-label">Medium</div></div>
  <div class="stat"><div class="stat-value" style="color:#1d4ed8">${bySev.low ?? 0}</div><div class="stat-label">Low</div></div>
</div>`

  if (open.length === 0) {
    const body = statsHtml + '<div class="callout good"><strong>✓ No open findings.</strong> All findings have been resolved or no audits have been run yet.</div>'
    return wrap('Remediation Plan', 'Prioritized action plan for open security findings', scope, date, body)
  }

  const effort = (sev: string) => {
    return ({ critical: 'Immediate', high: '1 week', medium: '1 month', low: 'Backlog' } as Record<string, string>)[sev.toLowerCase()] ?? '—'
  }

  let rows = ''
  for (const f of open) {
    const resource = f.file ? `${esc(f.file)}${f.line ? ':' + f.line : ''}` : esc(f.resource_name)
    const rec = f.recommendation || f.remediation || ''
    rows += `<tr>
      <td>${sevBadge(f.severity)}</td>
      <td>
        <strong>${esc(f.title)}</strong>
        ${f.category ? `<br><small style="color:#64748b">${esc(f.category)}</small>` : ''}
        ${rec ? `<br><small style="color:#0f172a;margin-top:2px;display:block">${esc(rec)}</small>` : ''}
      </td>
      <td><code>${resource || '—'}</code></td>
      <td>${esc(f.connection_name)}</td>
      <td style="text-align:center;white-space:nowrap"><strong>${effort(f.severity)}</strong></td>
    </tr>`
  }

  const table = `
<h2>Action Items (${open.length} open findings)</h2>
<table>
  <thead><tr><th width="80">Severity</th><th>Finding &amp; Recommendation</th><th width="200">Resource / File</th><th width="150">Connection</th><th width="90" style="text-align:center">Timeline</th></tr></thead>
  <tbody>${rows}</tbody>
</table>`

  const footer = `<footer>
    <p>Remediation plan generated by CloudSecGuard for scope: <strong>${esc(scope)}</strong>. Only open and in-progress findings are included. Mark findings as Fixed, Accepted Risk, or False Positive in CloudSecGuard to remove them from future plans.</p>
  </footer>`

  const body = statsHtml + table + footer
  return wrap('Remediation Plan', 'Prioritized action plan for open security findings', scope, date, body)
}
