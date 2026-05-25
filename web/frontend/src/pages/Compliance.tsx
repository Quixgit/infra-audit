import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, ClipboardCheck, ShieldCheck, ShieldAlert, ShieldX, Minus } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { complianceApi, type ComplianceFramework, type ComplianceControl, type ComplianceStatus } from '@/lib/api'

const FRAMEWORKS = [
  { slug: 'soc2',     shortName: 'SOC 2' },
  { slug: 'iso27001', shortName: 'ISO 27001' },
  { slug: 'nist-csf', shortName: 'NIST CSF' },
  { slug: 'cis-v8',   shortName: 'CIS v8' },
  { slug: 'hipaa',    shortName: 'HIPAA' },
  { slug: 'pci-dss',  shortName: 'PCI DSS' },
  { slug: 'gdpr',     shortName: 'GDPR' },
]

const STATUS_CONFIG: Record<ComplianceStatus, { label: string; cls: string; icon: React.ElementType }> = {
  met:          { label: 'Met',          cls: 'border-green-500/30 text-green-400 bg-green-500/10',        icon: ShieldCheck },
  partial:      { label: 'Partial',      cls: 'border-yellow-500/30 text-yellow-400 bg-yellow-500/10',    icon: ShieldAlert },
  not_met:      { label: 'Not Met',      cls: 'border-red-500/30 text-red-400 bg-red-500/10',             icon: ShieldX },
  not_assessed: { label: 'Not Assessed', cls: 'border-border/40 text-muted-foreground/60 bg-muted/20',    icon: Minus },
}

const SEV_COLOR: Record<string, string> = {
  critical: 'text-red-400',
  high:     'text-orange-400',
  medium:   'text-yellow-400',
  low:      'text-blue-400',
}

function ScoreRing({ score }: { score: number }) {
  const r = 28
  const circ = 2 * Math.PI * r
  const offset = circ * (1 - score / 100)
  const color = score >= 80 ? '#4ade80' : score >= 50 ? '#facc15' : '#f87171'
  return (
    <svg width={72} height={72} viewBox="0 0 72 72">
      <circle cx={36} cy={36} r={r} fill="none" stroke="hsl(var(--border))" strokeWidth={6} />
      <circle
        cx={36} cy={36} r={r} fill="none"
        stroke={color} strokeWidth={6}
        strokeDasharray={circ}
        strokeDashoffset={offset}
        strokeLinecap="round"
        transform="rotate(-90 36 36)"
        style={{ transition: 'stroke-dashoffset 0.6s ease' }}
      />
      <text x={36} y={40} textAnchor="middle" fontSize={14} fontWeight="bold" fill="currentColor">{score}%</text>
    </svg>
  )
}

function ControlRow({ ctrl }: { ctrl: ComplianceControl }) {
  const [open, setOpen] = useState(false)
  const cfg = STATUS_CONFIG[ctrl.status] ?? STATUS_CONFIG.not_met
  const Icon = cfg.icon
  const hasFindingDetails = ctrl.findings && ctrl.findings.length > 0

  return (
    <>
      <tr
        className={`border-b border-border/40 transition-colors ${hasFindingDetails ? 'cursor-pointer hover:bg-muted/30' : ''}`}
        onClick={() => hasFindingDetails && setOpen((v) => !v)}
      >
        <td className="px-4 py-3 w-8">
          {hasFindingDetails && (
            open ? <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" /> : <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
          )}
        </td>
        <td className="px-2 py-3 w-28">
          <span className="font-mono text-xs text-muted-foreground">{ctrl.ctrl_id}</span>
        </td>
        <td className="px-2 py-3">
          <p className="text-sm font-medium">{ctrl.name}</p>
          <p className="text-xs text-muted-foreground/70 mt-0.5 line-clamp-1">{ctrl.description}</p>
        </td>
        <td className="px-2 py-3 text-center w-28">
          <Badge className={`${cfg.cls} border text-xs gap-1`}>
            <Icon className="h-3 w-3" />
            {cfg.label}
          </Badge>
        </td>
        <td className="px-4 py-3 text-center w-24">
          {ctrl.finding_count > 0 ? (
            <span className="text-xs text-muted-foreground">
              {ctrl.open_count} open / {ctrl.finding_count}
            </span>
          ) : (
            <span className="text-xs text-muted-foreground/40">—</span>
          )}
        </td>
      </tr>
      {open && hasFindingDetails && (
        <tr className="bg-muted/20">
          <td colSpan={5} className="px-8 py-3">
            <div className="space-y-1.5">
              {ctrl.findings!.map((f, i) => (
                <div key={i} className="flex items-center gap-3 text-xs">
                  <span className={`font-semibold capitalize w-16 shrink-0 ${SEV_COLOR[f.severity?.toLowerCase()] ?? 'text-muted-foreground'}`}>
                    {f.severity}
                  </span>
                  <span className="flex-1 truncate">{f.title}</span>
                  <span className="text-muted-foreground shrink-0">{f.connection_name}</span>
                  <span className={`px-1.5 py-0.5 rounded text-xs border ${
                    f.status === 'open' || f.status === 'in_progress'
                      ? 'border-red-500/30 text-red-400'
                      : 'border-green-500/30 text-green-400'
                  }`}>
                    {f.status.replace('_', ' ')}
                  </span>
                </div>
              ))}
            </div>
          </td>
        </tr>
      )}
    </>
  )
}

function FrameworkDetail({ slug }: { slug: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ['compliance-framework', slug],
    queryFn: () => complianceApi.getFramework(slug),
  })

  if (isLoading) {
    return (
      <div className="space-y-2 mt-4">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="h-12 rounded bg-muted animate-pulse" />
        ))}
      </div>
    )
  }
  if (!data) return null

  const metCount          = data.controls?.filter((c) => c.status === 'met').length ?? 0
  const partialCount      = data.controls?.filter((c) => c.status === 'partial').length ?? 0
  const notMetCount       = data.controls?.filter((c) => c.status === 'not_met').length ?? 0
  const notAssessedCount  = data.controls?.filter((c) => c.status === 'not_assessed').length ?? 0

  return (
    <div className="mt-4 space-y-4">
      {/* Summary row */}
      <div className="flex items-center gap-6 rounded-lg border bg-muted/20 px-5 py-4">
        <ScoreRing score={data.score} />
        <div className="flex-1 grid grid-cols-4 gap-3">
          <div className="text-center">
            <p className="text-2xl font-bold text-green-400">{metCount}</p>
            <p className="text-xs text-muted-foreground">Met</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold text-yellow-400">{partialCount}</p>
            <p className="text-xs text-muted-foreground">Partial</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold text-red-400">{notMetCount}</p>
            <p className="text-xs text-muted-foreground">Not Met</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold text-muted-foreground/50">{notAssessedCount}</p>
            <p className="text-xs text-muted-foreground">Not Assessed</p>
          </div>
        </div>
        <div className="text-xs text-muted-foreground max-w-xs hidden lg:block">
          <p className="italic">{data.description}</p>
          <p className="mt-1 font-medium">Version {data.version}</p>
        </div>
      </div>

      {/* Controls table */}
      <div className="rounded-lg border overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40">
                <th className="px-4 py-2.5 w-8" />
                <th className="px-2 py-2.5 text-left text-xs font-medium text-muted-foreground w-28">Control</th>
                <th className="px-2 py-2.5 text-left text-xs font-medium text-muted-foreground">Name & Description</th>
                <th className="px-2 py-2.5 text-center text-xs font-medium text-muted-foreground w-28">Status</th>
                <th className="px-4 py-2.5 text-center text-xs font-medium text-muted-foreground w-24">Findings</th>
              </tr>
            </thead>
            <tbody>
              {(data.controls ?? []).map((ctrl) => (
                <ControlRow key={ctrl.ctrl_id} ctrl={ctrl} />
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

export function Compliance() {
  const [activeSlug, setActiveSlug] = useState('soc2')

  const { data: summaries, isLoading } = useQuery({
    queryKey: ['compliance-frameworks'],
    queryFn: complianceApi.listFrameworks,
  })

  const summaryMap = Object.fromEntries((summaries ?? []).map((f) => [f.slug, f]))

  return (
    <div className="space-y-6 max-w-5xl">
      <div>
        <h1 className="text-2xl font-bold">Compliance Readiness</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Framework coverage based on findings across all completed audits.
        </p>
      </div>

      {/* Framework score cards */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
        {FRAMEWORKS.map(({ slug, shortName }) => {
          const fw: ComplianceFramework | undefined = summaryMap[slug]
          const score = fw?.score ?? 0
          const isActive = activeSlug === slug
          const barColor = score >= 80 ? 'bg-green-500' : score >= 50 ? 'bg-yellow-500' : 'bg-red-500'
          return (
            <button
              key={slug}
              onClick={() => setActiveSlug(slug)}
              className={`rounded-lg border p-4 text-left transition-all ${
                isActive ? 'border-indigo-500/50 bg-indigo-500/5' : 'border-border hover:border-border/80 hover:bg-muted/30'
              }`}
            >
              <p className="text-xs font-semibold text-muted-foreground">{shortName}</p>
              {isLoading ? (
                <div className="mt-2 h-5 w-12 rounded bg-muted animate-pulse" />
              ) : (
                <>
                  <p className="mt-1 text-xl font-bold">{score}%</p>
                  <div className="mt-2 h-1.5 rounded-full bg-muted">
                    <div
                      className={`h-1.5 rounded-full transition-all ${barColor}`}
                      style={{ width: `${score}%` }}
                    />
                  </div>
                  {fw && (
                    <p className="mt-1.5 text-xs text-muted-foreground">
                      {fw.met_count}/{fw.total_count} controls met
                    </p>
                  )}
                </>
              )}
            </button>
          )
        })}
      </div>

      {/* Active framework detail */}
      <Card>
        <CardHeader className="pb-2">
          <div className="flex items-center gap-2">
            <ClipboardCheck className="h-4 w-4 text-indigo-400" />
            <CardTitle className="text-base">
              {summaryMap[activeSlug]?.name ?? FRAMEWORKS.find((f) => f.slug === activeSlug)?.shortName}
            </CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <FrameworkDetail key={activeSlug} slug={activeSlug} />
        </CardContent>
      </Card>

      {/* Methodology note */}
      <Card className="border-border/50 bg-muted/20">
        <CardContent className="py-4 text-xs text-muted-foreground space-y-1">
          <p>• Scores are calculated dynamically from findings across all completed audits — only over assessed controls.</p>
          <p>• <span className="text-green-400 font-medium">Met</span> — audited, all findings resolved. <span className="text-yellow-400 font-medium">Partial</span> — some findings still open. <span className="text-red-400 font-medium">Not Met</span> — open findings exist. <span className="font-medium text-muted-foreground">Not Assessed</span> — the automated scanner does not cover this control area (e.g. HR processes, physical security).</p>
          <p>• Marking a finding as "Accepted Risk" or "False Positive" removes it from the open count and improves the score.</p>
          <p>• Upload manual evidence (policies, screenshots) in the Evidence section and map it to controls.</p>
          <p>• This tool provides automated configuration readiness estimates — not a substitute for a formal audit.</p>
        </CardContent>
      </Card>
    </div>
  )
}
