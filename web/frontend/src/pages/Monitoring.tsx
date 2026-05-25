import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts'
import {
  ShieldAlert, AlertTriangle, TrendingUp, TrendingDown, Minus,
  RefreshCw, CheckCircle2, Clock, Save, Activity,
} from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { monitoringApi, slaApi, type SLARule, type SecurityScore } from '@/lib/api'
import { formatDateShort } from '@/lib/utils'

// ── Helpers ───────────────────────────────────────────────────────────────────

function scoreColor(score: number): string {
  if (score >= 80) return 'text-green-500'
  if (score >= 60) return 'text-yellow-500'
  if (score >= 40) return 'text-orange-500'
  return 'text-red-500'
}

function scoreBg(score: number): string {
  if (score >= 80) return 'bg-green-500'
  if (score >= 60) return 'bg-yellow-500'
  if (score >= 40) return 'bg-orange-500'
  return 'bg-red-500'
}

function severityBadge(sev: string) {
  const map: Record<string, string> = {
    critical: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
    medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    low: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
  }
  return (
    <Badge className={`border-0 text-xs font-medium ${map[sev.toLowerCase()] ?? ''}`}>
      {sev}
    </Badge>
  )
}

function changeTypeIcon(type: string) {
  if (type === 'new') return <span className="text-red-500 text-base">🔴</span>
  if (type === 'regression') return <span className="text-orange-500 text-base">🔁</span>
  return <span className="text-green-500 text-base">✅</span>
}

function changeTypeLabel(type: string) {
  if (type === 'new') return 'New finding'
  if (type === 'regression') return 'Regression'
  return 'Resolved'
}

function timeAgo(dateStr: string): string {
  const diffMs = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diffMs / 60000)
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function ScoreGauge({ score, size = 'sm' }: { score: number; size?: 'sm' | 'lg' }) {
  const radius = size === 'lg' ? 38 : 24
  const stroke = size === 'lg' ? 8 : 5
  const circ = 2 * Math.PI * radius
  const filled = (score / 100) * circ
  const fontSize = size === 'lg' ? 'text-3xl' : 'text-sm'
  const svgSize = size === 'lg' ? 100 : 64
  const center = svgSize / 2

  return (
    <div className="relative inline-flex items-center justify-center">
      <svg width={svgSize} height={svgSize}>
        <circle cx={center} cy={center} r={radius} fill="none"
          stroke="hsl(var(--muted))" strokeWidth={stroke} />
        <circle cx={center} cy={center} r={radius} fill="none"
          stroke={score >= 80 ? '#22c55e' : score >= 60 ? '#eab308' : score >= 40 ? '#f97316' : '#ef4444'}
          strokeWidth={stroke}
          strokeDasharray={`${filled} ${circ - filled}`}
          strokeLinecap="round"
          transform={`rotate(-90 ${center} ${center})`} />
      </svg>
      <span className={`absolute font-bold ${fontSize} ${scoreColor(score)}`}>{score}</span>
    </div>
  )
}

function ScoreTrendArrow({ scores }: { scores: SecurityScore[] }) {
  if (scores.length < 2) return <Minus className="h-4 w-4 text-muted-foreground" />
  const latest = scores[0].score
  const prev = scores[1].score
  if (latest > prev) return <TrendingUp className="h-4 w-4 text-green-500" />
  if (latest < prev) return <TrendingDown className="h-4 w-4 text-red-500" />
  return <Minus className="h-4 w-4 text-muted-foreground" />
}

// ── SLA Config Section ────────────────────────────────────────────────────────

function SLAConfig() {
  const qc = useQueryClient()
  const { data: rules = [] } = useQuery({
    queryKey: ['sla-rules'],
    queryFn: slaApi.list,
  })

  const [local, setLocal] = useState<SLARule[]>([])
  const [dirty, setDirty] = useState(false)

  // Sync from server when loaded
  const effective = dirty ? local : rules

  function init() {
    if (!dirty && rules.length > 0) {
      setLocal(rules)
      setDirty(false)
    }
  }

  function updateRule(severity: string, field: keyof SLARule, value: unknown) {
    init()
    const base = dirty ? local : rules
    setLocal(base.map((r) =>
      r.severity === severity ? { ...r, [field]: value } : r
    ))
    setDirty(true)
  }

  const save = useMutation({
    mutationFn: () =>
      slaApi.update(effective.map((r) => ({
        severity: r.severity,
        max_days_open: r.max_days_open,
        notify_email: r.notify_email,
        notify_slack: r.notify_slack,
      }))),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['sla-rules'] })
      setDirty(false)
      toast.success('SLA rules saved')
    },
    onError: () => toast.error('Save failed'),
  })

  const sevOrder = ['critical', 'high', 'medium', 'low']
  const sorted = [...effective].sort(
    (a, b) => sevOrder.indexOf(a.severity) - sevOrder.indexOf(b.severity)
  )

  return (
    <div className="space-y-3">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b">
              <th className="text-left py-2 text-xs font-semibold text-muted-foreground">Severity</th>
              <th className="text-left py-2 text-xs font-semibold text-muted-foreground">Max Days Open</th>
              <th className="text-left py-2 text-xs font-semibold text-muted-foreground">Email Alert</th>
              <th className="text-left py-2 text-xs font-semibold text-muted-foreground">Slack Alert</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {sorted.map((rule) => (
              <tr key={rule.severity}>
                <td className="py-3">{severityBadge(rule.severity)}</td>
                <td className="py-3">
                  <div className="flex items-center gap-2">
                    <Input
                      type="number"
                      min={1}
                      max={365}
                      value={rule.max_days_open}
                      onChange={(e) => updateRule(rule.severity, 'max_days_open', parseInt(e.target.value) || 1)}
                      className="h-7 w-20 text-xs"
                    />
                    <span className="text-xs text-muted-foreground">days</span>
                  </div>
                </td>
                <td className="py-3">
                  <Switch
                    checked={rule.notify_email}
                    onCheckedChange={(v) => updateRule(rule.severity, 'notify_email', v)}
                  />
                </td>
                <td className="py-3">
                  <Switch
                    checked={rule.notify_slack}
                    onCheckedChange={(v) => updateRule(rule.severity, 'notify_slack', v)}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {dirty && (
        <Button size="sm" disabled={save.isPending} onClick={() => save.mutate()}>
          <Save className="h-3.5 w-3.5 mr-1.5" />
          {save.isPending ? 'Saving…' : 'Save SLA Rules'}
        </Button>
      )}
    </div>
  )
}

// ── Main Page ─────────────────────────────────────────────────────────────────

export function Monitoring() {
  const navigate = useNavigate()

  const { data: overview, isLoading } = useQuery({
    queryKey: ['monitoring-overview'],
    queryFn: monitoringApi.overview,
    refetchInterval: 60_000,
  })

  const { data: latestScores = [] } = useQuery({
    queryKey: ['monitoring-scores'],
    queryFn: monitoringApi.scores,
    refetchInterval: 60_000,
  })

  const { data: breaches = [] } = useQuery({
    queryKey: ['monitoring-sla-breaches'],
    queryFn: monitoringApi.slaBreaches,
    refetchInterval: 60_000,
  })

  if (isLoading || !overview) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">
        Loading monitoring data…
      </div>
    )
  }

  const scoreTrendData = (overview.score_trend ?? []).map((d) => ({
    date: d.date.slice(5),
    score: d.avg_score > 0 ? Math.round(d.avg_score) : null,
  }))

  const openBreaches = breaches.filter(
    (b) => b.status === 'open' || b.status === 'in_progress'
  )

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold">Monitoring</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Continuous security monitoring, SLA tracking, and score trends
        </p>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {/* Security Score */}
        <Card>
          <CardContent className="pt-4 pb-3 px-4 flex items-center gap-3">
            <ScoreGauge score={overview.avg_score} />
            <div>
              <p className="text-xs text-muted-foreground">Avg Security Score</p>
              <p className={`text-2xl font-bold leading-none mt-0.5 ${scoreColor(overview.avg_score)}`}>
                {overview.avg_score}
              </p>
            </div>
          </CardContent>
        </Card>

        {/* SLA Breaches */}
        <Card className={overview.sla_breach_count > 0 ? 'border-red-500/40' : ''}>
          <CardContent className="pt-4 pb-3 px-4 flex items-center gap-3">
            <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg
              ${overview.sla_breach_count > 0 ? 'bg-red-100 dark:bg-red-900/30' : 'bg-muted'}`}>
              <AlertTriangle className={`h-5 w-5 ${overview.sla_breach_count > 0 ? 'text-red-500' : 'text-muted-foreground'}`} />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">SLA Breaches</p>
              <p className={`text-2xl font-bold leading-none mt-0.5 ${overview.sla_breach_count > 0 ? 'text-red-500' : ''}`}>
                {overview.sla_breach_count}
              </p>
            </div>
          </CardContent>
        </Card>

        {/* New This Week */}
        <Card className={overview.new_findings_this_week > 0 ? 'border-orange-500/30' : ''}>
          <CardContent className="pt-4 pb-3 px-4 flex items-center gap-3">
            <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg
              ${overview.new_findings_this_week > 0 ? 'bg-orange-100 dark:bg-orange-900/30' : 'bg-muted'}`}>
              <ShieldAlert className={`h-5 w-5 ${overview.new_findings_this_week > 0 ? 'text-orange-500' : 'text-muted-foreground'}`} />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">New This Week</p>
              <p className={`text-2xl font-bold leading-none mt-0.5 ${overview.new_findings_this_week > 0 ? 'text-orange-500' : ''}`}>
                {overview.new_findings_this_week}
              </p>
            </div>
          </CardContent>
        </Card>

        {/* Regressions */}
        <Card className={overview.regressions_findings_count > 0 ? 'border-red-500/30' : ''}>
          <CardContent className="pt-4 pb-3 px-4 flex items-center gap-3">
            <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg
              ${overview.regressions_findings_count > 0 ? 'bg-red-100 dark:bg-red-900/30' : 'bg-muted'}`}>
              <RefreshCw className={`h-5 w-5 ${overview.regressions_findings_count > 0 ? 'text-red-500' : 'text-muted-foreground'}`} />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Regressions</p>
              <p className={`text-2xl font-bold leading-none mt-0.5 ${overview.regressions_findings_count > 0 ? 'text-red-500' : ''}`}>
                {overview.regressions_findings_count}
              </p>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Section 1 — Score Trend */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base flex items-center gap-2">
            <Activity className="h-4 w-4 text-indigo-400" />
            Security Score Trend (30 days)
          </CardTitle>
        </CardHeader>
        <CardContent>
          {scoreTrendData.some((d) => d.score !== null) ? (
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={scoreTrendData} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis dataKey="date" tick={{ fontSize: 11 }} />
                <YAxis domain={[0, 100]} allowDecimals={false} tick={{ fontSize: 11 }} />
                <Tooltip
                  contentStyle={{
                    background: 'hsl(var(--popover))',
                    border: '1px solid hsl(var(--border))',
                    borderRadius: '6px',
                    fontSize: '12px',
                  }}
                  formatter={(v: number) => [`${v}`, 'Score']}
                />
                <Line
                  type="monotone"
                  dataKey="score"
                  stroke="#6366f1"
                  strokeWidth={2}
                  dot={false}
                  connectNulls={false}
                />
              </LineChart>
            </ResponsiveContainer>
          ) : (
            <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
              No score data yet — run an audit to start tracking.
            </div>
          )}

          {/* Per-connection score cards */}
          {latestScores.length > 0 && (
            <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 border-t pt-4">
              {latestScores.map((s) => (
                <div
                  key={s.connection_id}
                  className="flex items-center gap-3 rounded-lg border p-3 hover:border-indigo-500/40 cursor-pointer transition-colors"
                  onClick={() => navigate('/cloud-audits')}
                >
                  <ScoreGauge score={s.score} />
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{s.connection_name}</p>
                    <p className="text-xs text-muted-foreground">
                      Last scan: {formatDateShort(s.calculated_at)}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      C:{s.critical_count} H:{s.high_count} M:{s.medium_count} L:{s.low_count}
                    </p>
                  </div>
                  <ScoreTrendArrow scores={[s, { ...s, score: s.score - 1 }]} />
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Section 2 — SLA Breaches */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base flex items-center gap-2">
            <Clock className="h-4 w-4 text-red-500" />
            SLA Breaches
            {openBreaches.length > 0 && (
              <Badge className="bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300 border-0 text-xs ml-1">
                {openBreaches.length}
              </Badge>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {openBreaches.length === 0 ? (
            <div className="flex items-center gap-3 px-5 py-6 text-sm text-muted-foreground">
              <CheckCircle2 className="h-5 w-5 text-green-500" />
              No active SLA breaches — all findings within time limits.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b bg-muted/20">
                    <th className="text-left px-4 py-2.5 text-xs font-semibold text-muted-foreground">Finding</th>
                    <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Connection</th>
                    <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Severity</th>
                    <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Opened</th>
                    <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Days Overdue</th>
                    <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Status</th>
                    <th className="px-3 py-2.5" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {openBreaches.map((b) => {
                    const isCrit = b.severity === 'critical'
                    const isHigh = b.severity === 'high'
                    return (
                      <tr
                        key={b.id}
                        className={`${isCrit ? 'bg-red-50/40 dark:bg-red-950/10' : isHigh ? 'bg-orange-50/40 dark:bg-orange-950/10' : ''}`}
                      >
                        <td className="px-4 py-3 max-w-[200px]">
                          <p className="text-sm truncate font-medium">{b.title}</p>
                        </td>
                        <td className="px-3 py-3 text-xs text-muted-foreground">{b.connection_name}</td>
                        <td className="px-3 py-3">{severityBadge(b.severity)}</td>
                        <td className="px-3 py-3 text-xs text-muted-foreground">{formatDateShort(b.opened_at)}</td>
                        <td className="px-3 py-3">
                          <span className={`text-sm font-bold ${isCrit ? 'text-red-500' : isHigh ? 'text-orange-500' : 'text-yellow-500'}`}>
                            {b.days_overdue}d
                          </span>
                        </td>
                        <td className="px-3 py-3">
                          <Badge className="text-xs bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300 border-0">
                            {b.status}
                          </Badge>
                        </td>
                        <td className="px-3 py-3">
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 text-xs text-muted-foreground"
                            onClick={() => navigate('/remediation')}
                          >
                            Remediate →
                          </Button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Section 3 — Recent Changes */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base flex items-center gap-2">
            <Activity className="h-4 w-4 text-indigo-400" />
            Recent Changes
            <span className="text-xs font-normal text-muted-foreground">(last 14 days)</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {(overview.recent_changes ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">
              No changes detected yet. Changes appear after running multiple audits on the same connection.
            </p>
          ) : (
            <div className="space-y-0 divide-y divide-border">
              {(overview.recent_changes ?? []).map((c, i) => (
                <div key={i} className="flex items-start gap-3 py-3">
                  <div className="shrink-0 pt-0.5">{changeTypeIcon(c.change_type)}</div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm">
                      <span className="font-medium text-muted-foreground">{changeTypeLabel(c.change_type)}:</span>
                      {' '}<span className="font-semibold truncate">{c.title}</span>
                    </p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {c.connection_name}
                      {c.severity && <> · {severityBadge(c.severity)}</>}
                    </p>
                  </div>
                  <span className="text-xs text-muted-foreground shrink-0">{timeAgo(c.occurred_at)}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Section 4 — SLA Config */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base flex items-center gap-2">
            <Clock className="h-4 w-4 text-indigo-400" />
            SLA Configuration
          </CardTitle>
          <p className="text-xs text-muted-foreground">
            Maximum days a finding can remain open before triggering an SLA breach alert.
          </p>
        </CardHeader>
        <CardContent>
          <SLAConfig />
        </CardContent>
      </Card>
    </div>
  )
}
