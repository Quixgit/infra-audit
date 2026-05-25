import { useMemo, useState } from 'react'
import { AlertTriangle, Download, Search, X, ShieldAlert, ShieldX, Shield, Info } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { EmptyState } from '@/components/EmptyState'
import { aggregatedFindingsApi, connectionsApi, type AggregatedFinding, type FindingStatus } from '@/lib/api'
import { formatDate } from '@/lib/utils'

const STATUS_CONFIG: Record<FindingStatus, { label: string; cls: string }> = {
  open:           { label: 'Open',           cls: 'border-red-500/30 text-red-400 bg-red-500/10' },
  in_progress:    { label: 'In Progress',    cls: 'border-yellow-500/30 text-yellow-400 bg-yellow-500/10' },
  fixed:          { label: 'Fixed',          cls: 'border-green-500/30 text-green-400 bg-green-500/10' },
  accepted_risk:  { label: 'Accepted Risk',  cls: 'border-purple-500/30 text-purple-400 bg-purple-500/10' },
  false_positive: { label: 'False Positive', cls: 'border-border text-muted-foreground bg-muted' },
}

const SEV_COLOR: Record<string, string> = {
  critical: 'text-red-400',
  high:     'text-orange-400',
  medium:   'text-yellow-400',
  low:      'text-blue-400',
}

function StatusCell({ finding }: { finding: AggregatedFinding }) {
  const qc = useQueryClient()

  const mutation = useMutation({
    mutationFn: (status: FindingStatus) =>
      aggregatedFindingsApi.setOverride({
        job_id: finding.job_id,
        source: finding.source,
        finding_index: finding.finding_index,
        status,
        note: finding.note,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['aggregated-findings'] }),
    onError: () => toast.error('Failed to update status'),
  })

  const cfg = STATUS_CONFIG[finding.status] ?? STATUS_CONFIG.open

  return (
    <Select
      value={finding.status}
      onValueChange={(v) => mutation.mutate(v as FindingStatus)}
      disabled={mutation.isPending}
    >
      <SelectTrigger className={`h-7 w-36 text-xs border ${cfg.cls}`}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {(Object.keys(STATUS_CONFIG) as FindingStatus[]).map((v) => (
          <SelectItem key={v} value={v} className="text-xs">
            {STATUS_CONFIG[v].label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

function exportCSV(findings: AggregatedFinding[]) {
  const headers = ['Severity', 'Title', 'Category', 'Resource/File', 'Tool', 'Connection', 'Job Date', 'Status']
  const escape = (v: string) => `"${(v ?? '').replace(/"/g, '""')}"`
  const rows = findings.map((f) => [
    f.severity,
    escape(f.title ?? ''),
    f.category ?? '',
    escape(f.file ? `${f.file}${f.line ? ':' + f.line : ''}` : f.resource_name ?? ''),
    f.tool ?? '',
    escape(f.connection_name),
    new Date(f.job_date).toLocaleDateString(),
    f.status,
  ])
  const csv = [headers, ...rows].map((r) => r.join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `findings-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

export function Findings() {
  const [severityFilter, setSeverityFilter] = useState('all')
  const [statusFilter, setStatusFilter] = useState('all')
  const [connFilter, setConnFilter] = useState('all')
  const [toolFilter, setToolFilter] = useState('all')
  const [search, setSearch] = useState('')

  const { data: findings = [], isLoading } = useQuery({
    queryKey: ['aggregated-findings'],
    queryFn: () => aggregatedFindingsApi.list(),
  })

  const { data: connections = [] } = useQuery({
    queryKey: ['connections'],
    queryFn: connectionsApi.list,
  })

  const tools = useMemo(
    () => Array.from(new Set(findings.map((f) => f.tool).filter(Boolean))) as string[],
    [findings]
  )

  const sevCounts = useMemo(
    () =>
      findings.reduce(
        (acc, f) => {
          const s = f.severity.toLowerCase()
          acc[s] = (acc[s] ?? 0) + 1
          return acc
        },
        {} as Record<string, number>
      ),
    [findings]
  )

  const filtered = useMemo(() => {
    const q = search.toLowerCase()
    return findings.filter((f) => {
      if (severityFilter !== 'all' && f.severity.toLowerCase() !== severityFilter) return false
      if (statusFilter !== 'all' && f.status !== statusFilter) return false
      if (connFilter !== 'all' && f.connection_id !== connFilter) return false
      if (toolFilter !== 'all' && (f.tool ?? '').toLowerCase() !== toolFilter) return false
      if (q) {
        const hay = [f.title, f.file, f.resource_name, f.rule_id, f.package]
          .filter(Boolean)
          .join(' ')
          .toLowerCase()
        if (!hay.includes(q)) return false
      }
      return true
    })
  }, [findings, severityFilter, statusFilter, connFilter, toolFilter, search])

  const hasFilters =
    severityFilter !== 'all' ||
    statusFilter !== 'all' ||
    connFilter !== 'all' ||
    toolFilter !== 'all' ||
    search !== ''

  const clearFilters = () => {
    setSeverityFilter('all')
    setStatusFilter('all')
    setConnFilter('all')
    setToolFilter('all')
    setSearch('')
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Findings</h1>
          <p className="text-sm text-muted-foreground mt-1">
            All findings across completed audits
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => exportCSV(filtered)}
          disabled={filtered.length === 0}
        >
          <Download className="mr-2 h-3.5 w-3.5" />
          Export CSV ({filtered.length})
        </Button>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {[
          { label: 'Critical', value: sevCounts.critical ?? 0, icon: ShieldX,     iconBg: 'bg-red-500/10',    iconColor: 'text-red-400',    valueColor: 'text-red-400' },
          { label: 'High',     value: sevCounts.high     ?? 0, icon: ShieldAlert, iconBg: 'bg-orange-500/10', iconColor: 'text-orange-400', valueColor: 'text-orange-400' },
          { label: 'Medium',   value: sevCounts.medium   ?? 0, icon: Shield,      iconBg: 'bg-yellow-500/10', iconColor: 'text-yellow-400', valueColor: 'text-yellow-400' },
          { label: 'Low',      value: sevCounts.low      ?? 0, icon: Info,        iconBg: 'bg-blue-500/10',   iconColor: 'text-blue-400',   valueColor: '' },
        ].map(({ label, value, icon: Icon, iconBg, iconColor, valueColor }) => (
          <Card key={label} className="relative overflow-hidden">
            <CardContent className="pt-5 pb-4">
              <div className="flex items-start justify-between gap-3">
                <div className="flex-1 min-w-0">
                  <p className="text-xs text-muted-foreground font-medium mb-1.5 uppercase tracking-wide">{label}</p>
                  <p className={`text-3xl font-bold leading-none ${value > 0 ? valueColor : ''}`}>{value}</p>
                </div>
                <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${iconBg}`}>
                  <Icon className={`h-5 w-5 ${iconColor}`} />
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Severity pills */}
      <div className="flex flex-wrap gap-2">
        {(['all', 'critical', 'high', 'medium', 'low'] as const).map((sev) => {
          const count = sev === 'all' ? findings.length : (sevCounts[sev] ?? 0)
          const label = sev === 'all' ? `All (${count})` : `${sev[0].toUpperCase()}${sev.slice(1)} (${count})`
          return (
            <button
              key={sev}
              onClick={() => setSeverityFilter(sev)}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                severityFilter === sev
                  ? 'bg-indigo-500/20 text-indigo-400'
                  : 'bg-muted text-muted-foreground hover:bg-muted/80'
              }`}
            >
              {label}
            </button>
          )
        })}
      </div>

      {/* Filter row */}
      <div className="flex flex-wrap gap-2 items-center">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
          <Input
            placeholder="Search findings..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-8 pl-8 w-52 text-sm"
          />
        </div>

        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="h-8 w-40 text-xs">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            {(Object.keys(STATUS_CONFIG) as FindingStatus[]).map((v) => (
              <SelectItem key={v} value={v} className="text-xs">
                {STATUS_CONFIG[v].label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={connFilter} onValueChange={setConnFilter}>
          <SelectTrigger className="h-8 w-48 text-xs">
            <SelectValue placeholder="All connections" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All connections</SelectItem>
            {connections.map((c) => (
              <SelectItem key={c.id} value={c.id} className="text-xs">
                {c.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {tools.length > 1 && (
          <Select value={toolFilter} onValueChange={setToolFilter}>
            <SelectTrigger className="h-8 w-36 text-xs">
              <SelectValue placeholder="All tools" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All tools</SelectItem>
              {tools.map((t) => (
                <SelectItem key={t} value={t.toLowerCase()} className="text-xs">
                  {t}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        {hasFilters && (
          <button
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
            onClick={clearFilters}
          >
            <X className="h-3 w-3" />
            Clear
          </button>
        )}
      </div>

      {/* Table */}
      {isLoading ? (
        <div className="space-y-2">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="h-11 rounded-lg bg-muted animate-pulse" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={AlertTriangle}
          title={findings.length === 0 ? 'No findings yet' : 'No findings match filters'}
          description={
            findings.length === 0
              ? 'Run an audit to see findings here.'
              : 'Try adjusting your filters.'
          }
        />
      ) : (
        <div className="rounded-lg border overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/40">
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground w-20">
                    Severity
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                    Title
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                    Resource / File
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground w-24">
                    Tool
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground w-36">
                    Connection
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground w-28">
                    Date
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground w-40">
                    Status
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filtered.map((f, i) => {
                  const resource = f.file
                    ? `${f.file}${f.line ? ':' + f.line : ''}`
                    : (f.resource_name ?? '—')
                  return (
                    <tr key={i} className="hover:bg-muted/30 transition-colors">
                      <td className="px-4 py-2.5">
                        <span
                          className={`text-xs font-semibold capitalize ${
                            SEV_COLOR[f.severity.toLowerCase()] ?? 'text-muted-foreground'
                          }`}
                        >
                          {f.severity}
                        </span>
                      </td>
                      <td className="px-4 py-2.5 max-w-xs">
                        <p className="truncate font-medium text-sm">{f.title}</p>
                        {f.category && (
                          <p className="text-xs text-muted-foreground">{f.category}</p>
                        )}
                      </td>
                      <td className="px-4 py-2.5 max-w-xs">
                        <p className="font-mono text-xs truncate text-muted-foreground" title={resource}>
                          {resource}
                        </p>
                      </td>
                      <td className="px-4 py-2.5">
                        {f.tool ? (
                          <span className="rounded bg-muted px-1.5 py-0.5 text-xs">{f.tool}</span>
                        ) : (
                          <span className="text-muted-foreground/40">—</span>
                        )}
                      </td>
                      <td className="px-4 py-2.5">
                        <p className="text-xs text-muted-foreground truncate">{f.connection_name}</p>
                      </td>
                      <td className="px-4 py-2.5">
                        <p className="text-xs text-muted-foreground">{formatDate(f.job_date)}</p>
                      </td>
                      <td className="px-4 py-2.5">
                        <StatusCell finding={f} />
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
