import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { ShieldCheck, ChevronDown, ChevronUp } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/StatusBadge'
import { FindingsBadges } from '@/components/FindingsBadges'
import { shareApi, type Finding } from '@/lib/api'
import { formatDate } from '@/lib/utils'

const severityConfig: Record<string, { color: string; bg: string }> = {
  critical: { color: 'text-red-400', bg: 'bg-red-500/10 border-red-500/30' },
  high:     { color: 'text-orange-400', bg: 'bg-orange-500/10 border-orange-500/30' },
  medium:   { color: 'text-yellow-400', bg: 'bg-yellow-500/10 border-yellow-500/30' },
  low:      { color: 'text-blue-400', bg: 'bg-blue-500/10 border-blue-500/30' },
}

function FindingCard({ f }: { f: Finding }) {
  const [open, setOpen] = useState(false)
  const sev = f.severity.toLowerCase()
  const cfg = severityConfig[sev] ?? { color: 'text-muted-foreground', bg: 'bg-muted' }

  return (
    <div className={`rounded-lg border p-4 ${cfg.bg}`}>
      <div className="flex items-start justify-between gap-3 cursor-pointer" onClick={() => setOpen((o) => !o)}>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <Badge variant="outline" className={`${cfg.color} text-xs capitalize`}>{f.severity}</Badge>
            {f.category && <span className="text-xs text-muted-foreground">{f.category}</span>}
          </div>
          <p className="font-medium text-sm mt-1">{f.title}</p>
          {f.resource_name && (
            <p className="text-xs text-muted-foreground mt-0.5">{f.resource_type}: {f.resource_name}</p>
          )}
        </div>
        {open ? <ChevronUp className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" /> : <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />}
      </div>
      {open && (
        <div className="mt-4 space-y-3 text-sm border-t border-white/5 pt-3">
          {f.risk && <div><p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Risk</p><p className="text-foreground/80">{f.risk}</p></div>}
          {f.business_impact && <div><p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Business impact</p><p className="text-foreground/80">{f.business_impact}</p></div>}
          {f.evidence && <div><p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Evidence</p><p className="font-mono text-xs bg-black/20 rounded p-2 whitespace-pre-wrap">{f.evidence}</p></div>}
          {f.recommendation && <div><p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Recommendation</p><p className="text-foreground/80">{f.recommendation}</p></div>}
          {f.remediation && <div><p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Remediation</p><p className="text-foreground/80">{f.remediation}</p></div>}
        </div>
      )}
    </div>
  )
}

export function ShareView() {
  const { token } = useParams<{ token: string }>()
  const [severityFilter, setSeverityFilter] = useState('all')

  const { data, isLoading, isError } = useQuery({
    queryKey: ['share', token],
    queryFn: () => shareApi.get(token!),
    enabled: !!token,
  })

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <div className="text-center space-y-2">
          <p className="text-lg font-semibold">Link not found</p>
          <p className="text-sm text-muted-foreground">This share link is invalid or has expired.</p>
        </div>
      </div>
    )
  }

  const { job, findings } = data
  const safefindings = Array.isArray(findings) ? findings : []

  const filtered = safefindings.filter((f) =>
    severityFilter === 'all' || f.severity.toLowerCase() === severityFilter
  )

  const counts = safefindings.reduce((acc, f) => {
    const s = f.severity.toLowerCase()
    acc[s] = (acc[s] ?? 0) + 1
    return acc
  }, {} as Record<string, number>)

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b bg-card px-6 py-4 flex items-center gap-3">
        <ShieldCheck className="h-6 w-6 text-indigo-400" strokeWidth={1.8} />
        <span className="text-base font-bold tracking-tight">
          CloudSec<span className="text-indigo-400">Guard</span>
        </span>
        <span className="text-muted-foreground text-sm ml-2">— Shared Audit Report</span>
      </header>

      <main className="max-w-3xl mx-auto px-4 py-8 space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold">{job.connection_name}</h1>
            <p className="text-sm text-muted-foreground">{formatDate(job.started_at)}</p>
          </div>
          <StatusBadge status={job.status} />
        </div>

        <Card>
          <CardHeader><CardTitle className="text-base">Findings summary</CardTitle></CardHeader>
          <CardContent>
            <div className="grid grid-cols-4 gap-4">
              {[
                { label: 'Critical', count: job.findings_critical, color: 'bg-red-500/10 text-red-400 border-red-500/20' },
                { label: 'High', count: job.findings_high, color: 'bg-orange-500/10 text-orange-400 border-orange-500/20' },
                { label: 'Medium', count: job.findings_medium, color: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20' },
                { label: 'Low', count: job.findings_low, color: 'bg-blue-500/10 text-blue-400 border-blue-500/20' },
              ].map(({ label, count, color }) => (
                <div key={label} className={`rounded-lg border p-4 text-center ${color}`}>
                  <p className="text-3xl font-bold">{count}</p>
                  <p className="text-sm mt-1">{label}</p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {safefindings.length > 0 && (
          <div className="space-y-3">
            <div className="flex flex-wrap gap-2">
              {['all', 'critical', 'high', 'medium', 'low'].map((sev) => (
                <button
                  key={sev}
                  onClick={() => setSeverityFilter(sev)}
                  className={`rounded px-3 py-1 text-xs capitalize border transition-colors ${
                    severityFilter === sev
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'border-border text-muted-foreground hover:text-foreground'
                  }`}
                >
                  {sev === 'all' ? 'All' : `${sev} ${counts[sev] ?? 0}`}
                </button>
              ))}
            </div>
            <div className="space-y-2">
              {filtered.map((f, i) => <FindingCard key={i} f={f} />)}
            </div>
          </div>
        )}
      </main>
    </div>
  )
}
