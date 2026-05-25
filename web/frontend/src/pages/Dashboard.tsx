import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts'
import {
  PlugZap, Briefcase, ShieldAlert, Layers, ClipboardCheck, Kanban,
  ScrollText, CheckCircle2, Clock, Activity, AlertTriangle, UserCheck,
  ArrowUpRight, Rocket, TrendingUp, ChevronRight, Shield,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/StatusBadge'
import { FindingsBadges } from '@/components/FindingsBadges'
import {
  dashboardApi, licenseApi, complianceApi, remediationApi,
  policiesApi, monitoringApi, accessReviewsApi,
} from '@/lib/api'
import { useAuthStore } from '@/store/useAuthStore'
import { formatDate } from '@/lib/utils'

// ── Stat card ─────────────────────────────────────────────────────────────────

interface StatCardProps {
  label: string
  value: number | string
  icon: React.ElementType
  iconBg: string
  iconColor: string
  sub?: string
  href?: string
  alert?: boolean
}

function StatCard({ label, value, icon: Icon, iconBg, iconColor, sub, href, alert }: StatCardProps) {
  const navigate = useNavigate()
  return (
    <Card
      className={`relative overflow-hidden transition-all duration-200 ${href ? 'cursor-pointer hover:shadow-md hover:-translate-y-px' : ''} ${alert ? 'border-orange-500/30' : ''}`}
      onClick={href ? () => navigate(href) : undefined}
    >
      <CardContent className="pt-5 pb-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-muted-foreground font-medium mb-1.5 uppercase tracking-wide">{label}</p>
            <p className={`text-3xl font-bold leading-none ${alert && Number(value) > 0 ? 'text-orange-400' : ''}`}>
              {value}
            </p>
            {sub && <p className="text-xs text-muted-foreground mt-1.5">{sub}</p>}
          </div>
          <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${iconBg}`}>
            <Icon className={`h-5 w-5 ${iconColor}`} />
          </div>
        </div>
      </CardContent>
      {href && (
        <div className="absolute bottom-3 right-3">
          <ArrowUpRight className="h-3.5 w-3.5 text-muted-foreground/30" />
        </div>
      )}
    </Card>
  )
}

// ── Progress bar ──────────────────────────────────────────────────────────────

function ProgressBar({ value, color = 'bg-indigo-500' }: { value: number; color?: string }) {
  return (
    <div className="h-1.5 w-full rounded-full bg-muted overflow-hidden">
      <div
        className={`h-full rounded-full transition-all duration-500 ${color}`}
        style={{ width: `${Math.min(100, value)}%` }}
      />
    </div>
  )
}

// ── Score ring ────────────────────────────────────────────────────────────────

function ScoreRing({ score }: { score: number }) {
  const r = 20
  const circ = 2 * Math.PI * r
  const dash = (score / 100) * circ
  const color = score >= 80 ? '#22c55e' : score >= 60 ? '#eab308' : '#ef4444'
  return (
    <div className="relative flex items-center justify-center h-14 w-14">
      <svg className="-rotate-90" width="56" height="56">
        <circle cx="28" cy="28" r={r} fill="none" stroke="hsl(var(--muted))" strokeWidth="4" />
        <circle
          cx="28" cy="28" r={r} fill="none"
          stroke={color} strokeWidth="4"
          strokeDasharray={`${dash} ${circ}`}
          strokeLinecap="round"
        />
      </svg>
      <span className="absolute text-sm font-bold leading-none" style={{ color }}>{score}</span>
    </div>
  )
}

// ── Custom tooltip ────────────────────────────────────────────────────────────

function ChartTooltip({ active, payload, label }: any) {
  if (!active || !payload?.length) return null
  return (
    <div className="rounded-lg border bg-popover px-3 py-2 text-xs shadow-xl">
      <p className="font-semibold mb-1.5 text-muted-foreground">{label}</p>
      {payload.map((p: any) => (
        <div key={p.dataKey} className="flex items-center gap-2">
          <span className="h-2 w-2 rounded-full shrink-0" style={{ background: p.color }} />
          <span className="capitalize text-muted-foreground">{p.dataKey}:</span>
          <span className="font-semibold ml-auto pl-2">{p.value}</span>
        </div>
      ))}
    </div>
  )
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export function Dashboard() {
  const navigate = useNavigate()
  const { user } = useAuthStore()

  const { data, isLoading } = useQuery({
    queryKey: ['dashboard'],
    queryFn: dashboardApi.get,
    refetchInterval: 30_000,
  })
  const { data: license } = useQuery({ queryKey: ['license'], queryFn: licenseApi.get })
  const { data: complianceSummaries } = useQuery({ queryKey: ['compliance-frameworks'], queryFn: complianceApi.listFrameworks })
  const { data: remediationTasks = [] } = useQuery({ queryKey: ['remediation-tasks'], queryFn: remediationApi.listTasks })
  const { data: policyStats } = useQuery({ queryKey: ['policy-stats'], queryFn: policiesApi.stats })
  const { data: monitoringOverview } = useQuery({ queryKey: ['monitoring-overview'], queryFn: monitoringApi.overview })
  const { data: accessReviewStats } = useQuery({ queryKey: ['access-reviews-stats'], queryFn: accessReviewsApi.stats })

  if (isLoading || !data) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-indigo-500 border-t-transparent" />
          <p className="text-sm text-muted-foreground">Loading dashboard…</p>
        </div>
      </div>
    )
  }

  const trendData = (data.findings_trend ?? []).map((d) => ({
    ...d,
    date: d.date.slice(5),
  }))
  const hasTrendData = trendData.some(d => d.critical + d.high + d.medium + d.low > 0)

  const isCommunity = !license || license.plan === 'community'
  const hour = new Date().getHours()
  const greeting = hour < 12 ? 'Morning' : hour < 17 ? 'Hey' : 'Evening'
  const firstName = user?.prepared_by?.split(' ')[0] || user?.email?.split('@')[0] || 'there'

  // Remediation stats
  const remDone = remediationTasks.filter(t => t.lane === 'done').length
  const remTotal = remediationTasks.length
  const remPct = remTotal > 0 ? Math.round((remDone / remTotal) * 100) : 0
  const remOverdue = remediationTasks.filter(t => t.lane !== 'done' && t.due_date && new Date(t.due_date) < new Date()).length

  const shortNames: Record<string, string> = {
    'soc2': 'SOC 2', 'iso27001': 'ISO 27001', 'nist-csf': 'NIST CSF', 'cis-v8': 'CIS v8',
  }

  return (
    <div className="space-y-5 max-w-6xl">

      {/* ── Header ── */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {greeting}, {firstName}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Security overview for your workspace
          </p>
        </div>
        {/* Quick action */}
        <button
          onClick={() => navigate('/audit-types')}
          className="shrink-0 flex items-center gap-2 rounded-lg bg-indigo-500 px-3.5 py-2 text-xs font-semibold text-white shadow-sm hover:bg-indigo-600 transition-colors"
        >
          <Rocket className="h-3.5 w-3.5" />
          New Audit
        </button>
      </div>

      {/* ── Upgrade banner ── */}
      {isCommunity && (
        <div className="flex items-center justify-between rounded-xl border border-indigo-500/25 bg-gradient-to-r from-indigo-500/8 to-purple-500/5 px-4 py-3 gap-4">
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-indigo-500/15">
              <TrendingUp className="h-4 w-4 text-indigo-400" />
            </div>
            <div className="min-w-0">
              <p className="text-sm font-semibold text-indigo-300">Free plan</p>
              <p className="text-xs text-muted-foreground truncate">
                {license?.max_connections ?? 5} connections · {license?.max_audits_month ?? 20} audits/month.
                Scheduled audits, code scanning and team access require a paid plan.
              </p>
            </div>
          </div>
          <button
            onClick={() => navigate('/plans')}
            className="shrink-0 flex items-center gap-1.5 rounded-lg border border-indigo-500/40 bg-indigo-500/10 px-3 py-1.5 text-xs font-semibold text-indigo-300 hover:bg-indigo-500/20 transition-colors"
          >
            Upgrade <ChevronRight className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* ── Stat cards ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          label="Connections"
          value={data.total_connections}
          icon={PlugZap}
          iconBg="bg-indigo-500/10"
          iconColor="text-indigo-400"
          sub={`${license?.max_connections ?? '∞'} max on your plan`}
          href="/connections"
        />
        <StatCard
          label="Audits this week"
          value={data.jobs_this_week}
          icon={Briefcase}
          iconBg="bg-blue-500/10"
          iconColor="text-blue-400"
          sub="Last 7 days"
          href="/reports"
        />
        <StatCard
          label="Total findings"
          value={data.total_findings}
          icon={ShieldAlert}
          iconBg={data.total_findings > 0 ? 'bg-orange-500/10' : 'bg-green-500/10'}
          iconColor={data.total_findings > 0 ? 'text-orange-400' : 'text-green-400'}
          sub={data.total_findings === 0 ? 'Nothing flagged' : 'Across all scans'}
          href="/findings"
          alert={data.total_findings > 0}
        />
      </div>

      {/* ── Main content grid ── */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-5">

        {/* Findings trend — spans 2 cols */}
        <Card className="xl:col-span-2">
          <CardHeader className="pb-2 flex flex-row items-center justify-between">
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <div className="flex h-6 w-6 items-center justify-center rounded-md bg-orange-500/10">
                <TrendingUp className="h-3.5 w-3.5 text-orange-400" />
              </div>
              Findings Trend
            </CardTitle>
            <span className="text-xs text-muted-foreground">Last 30 days</span>
          </CardHeader>
          <CardContent>
            {hasTrendData ? (
              <ResponsiveContainer width="100%" height={210}>
                <AreaChart data={trendData} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
                  <defs>
                    {[
                      { id: 'critical', color: '#ef4444' },
                      { id: 'high',     color: '#f97316' },
                      { id: 'medium',   color: '#eab308' },
                      { id: 'low',      color: '#6366f1' },
                    ].map(({ id, color }) => (
                      <linearGradient key={id} id={`grad-${id}`} x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%"  stopColor={color} stopOpacity={0.25} />
                        <stop offset="95%" stopColor={color} stopOpacity={0} />
                      </linearGradient>
                    ))}
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                  <XAxis dataKey="date" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
                  <YAxis allowDecimals={false} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
                  <Tooltip content={<ChartTooltip />} />
                  <Area type="monotone" dataKey="critical" stroke="#ef4444" fill="url(#grad-critical)" strokeWidth={1.5} dot={false} />
                  <Area type="monotone" dataKey="high"     stroke="#f97316" fill="url(#grad-high)"     strokeWidth={1.5} dot={false} />
                  <Area type="monotone" dataKey="medium"   stroke="#eab308" fill="url(#grad-medium)"   strokeWidth={1.5} dot={false} />
                  <Area type="monotone" dataKey="low"      stroke="#6366f1" fill="url(#grad-low)"      strokeWidth={1.5} dot={false} />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-[210px] flex-col items-center justify-center gap-2 text-muted-foreground">
                <Shield className="h-8 w-8 opacity-20" />
                <p className="text-sm">Nothing to show yet</p>
              </div>
            )}
            {/* Legend */}
            {hasTrendData && (
              <div className="flex items-center gap-4 mt-2 justify-center flex-wrap">
                {[
                  { key: 'critical', color: '#ef4444', label: 'Critical' },
                  { key: 'high',     color: '#f97316', label: 'High' },
                  { key: 'medium',   color: '#eab308', label: 'Medium' },
                  { key: 'low',      color: '#6366f1', label: 'Low' },
                ].map(({ color, label }) => (
                  <div key={label} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <span className="h-2 w-2 rounded-full" style={{ background: color }} />
                    {label}
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Monitoring score — 1 col */}
        {monitoringOverview ? (
          <Card
            className="cursor-pointer hover:shadow-md hover:-translate-y-px transition-all duration-200"
            onClick={() => navigate('/monitoring')}
          >
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <div className="flex h-6 w-6 items-center justify-center rounded-md bg-blue-500/10">
                  <Activity className="h-3.5 w-3.5 text-blue-400" />
                </div>
                Security Score
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center gap-4">
                <ScoreRing score={monitoringOverview.avg_score} />
                <div className="space-y-1 flex-1">
                  <p className="text-xs text-muted-foreground">Avg. across connections</p>
                  <div className="space-y-1.5">
                    {monitoringOverview.sla_breach_count > 0 && (
                      <div className="flex items-center gap-1.5 text-xs text-red-400 font-medium">
                        <AlertTriangle className="h-3.5 w-3.5" />
                        {monitoringOverview.sla_breach_count} SLA breach{monitoringOverview.sla_breach_count !== 1 ? 'es' : ''}
                      </div>
                    )}
                    {monitoringOverview.new_findings_this_week > 0 && (
                      <div className="flex items-center gap-1.5 text-xs text-orange-400 font-medium">
                        <ShieldAlert className="h-3.5 w-3.5" />
                        {monitoringOverview.new_findings_this_week} new this week
                      </div>
                    )}
                    {monitoringOverview.sla_breach_count === 0 && monitoringOverview.new_findings_this_week === 0 && (
                      <div className="flex items-center gap-1.5 text-xs text-green-400 font-medium">
                        <CheckCircle2 className="h-3.5 w-3.5" />
                        All within SLA
                      </div>
                    )}
                  </div>
                </div>
              </div>
              <div className="flex items-center justify-end text-xs text-muted-foreground/50 gap-1">
                View details <ArrowUpRight className="h-3 w-3" />
              </div>
            </CardContent>
          </Card>
        ) : (
          <Card className="flex flex-col items-center justify-center py-10 gap-3 text-center">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-muted">
              <Activity className="h-5 w-5 text-muted-foreground/40" />
            </div>
            <div>
              <p className="text-sm font-medium">No data yet</p>
              <p className="text-xs text-muted-foreground mt-1">Scores appear after the first audit</p>
            </div>
          </Card>
        )}
      </div>

      {/* ── Compliance + Remediation + Policies + Access Reviews ── */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-5">

        {/* Compliance readiness */}
        {complianceSummaries && complianceSummaries.length > 0 && (
          <Card
            className="cursor-pointer hover:shadow-md hover:-translate-y-px transition-all duration-200"
            onClick={() => navigate('/compliance')}
          >
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <div className="flex h-6 w-6 items-center justify-center rounded-md bg-purple-500/10">
                  <ClipboardCheck className="h-3.5 w-3.5 text-purple-400" />
                </div>
                Compliance Readiness
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {complianceSummaries.map((fw) => {
                const pct = fw.score
                const barColor = pct >= 80 ? 'bg-green-500' : pct >= 50 ? 'bg-yellow-500' : 'bg-red-500'
                return (
                  <div key={fw.slug} className="space-y-1">
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-muted-foreground font-medium">{shortNames[fw.slug] ?? fw.slug}</span>
                      <div className="flex items-center gap-2">
                        <span className="text-muted-foreground/50 text-[10px]">{fw.met_count}/{fw.total_count}</span>
                        <span className={`font-bold ${pct >= 80 ? 'text-green-400' : pct >= 50 ? 'text-yellow-400' : 'text-red-400'}`}>
                          {pct}%
                        </span>
                      </div>
                    </div>
                    <ProgressBar value={pct} color={barColor} />
                  </div>
                )
              })}
              <div className="flex items-center justify-end pt-1 text-xs text-muted-foreground/50 gap-1">
                View frameworks <ArrowUpRight className="h-3 w-3" />
              </div>
            </CardContent>
          </Card>
        )}

        {/* Remediation */}
        {remediationTasks.length > 0 && (
          <Card
            className="cursor-pointer hover:shadow-md hover:-translate-y-px transition-all duration-200"
            onClick={() => navigate('/remediation')}
          >
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <div className="flex h-6 w-6 items-center justify-center rounded-md bg-teal-500/10">
                  <Kanban className="h-3.5 w-3.5 text-teal-400" />
                </div>
                Remediation Progress
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center gap-4">
                <div className="flex-1 space-y-2">
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">{remDone} of {remTotal} resolved</span>
                    <span className={`font-bold text-sm ${remPct >= 80 ? 'text-green-400' : remPct >= 40 ? 'text-yellow-400' : 'text-muted-foreground'}`}>
                      {remPct}%
                    </span>
                  </div>
                  <ProgressBar
                    value={remPct}
                    color={remPct >= 80 ? 'bg-green-500' : remPct >= 40 ? 'bg-yellow-500' : 'bg-indigo-500'}
                  />
                </div>
              </div>
              <div className="flex items-center gap-4 text-xs">
                <span className="text-muted-foreground">{remTotal - remDone} open</span>
                {remOverdue > 0 && (
                  <span className="flex items-center gap-1 text-red-400 font-medium">
                    <AlertTriangle className="h-3 w-3" />
                    {remOverdue} overdue
                  </span>
                )}
                <div className="ml-auto flex items-center gap-1 text-muted-foreground/50">
                  View board <ArrowUpRight className="h-3 w-3" />
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Policies */}
        {policyStats && policyStats.total > 0 && (
          <Card
            className="cursor-pointer hover:shadow-md hover:-translate-y-px transition-all duration-200"
            onClick={() => navigate('/policies')}
          >
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <div className="flex h-6 w-6 items-center justify-center rounded-md bg-blue-500/10">
                  <ScrollText className="h-3.5 w-3.5 text-blue-400" />
                </div>
                Security Policies
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-3 gap-3 mb-3">
                {[
                  { label: 'Total',    value: policyStats.total,    color: 'text-foreground' },
                  { label: 'Approved', value: policyStats.approved, color: 'text-green-400' },
                  { label: 'Review',   value: policyStats.review_due, color: policyStats.review_due > 0 ? 'text-yellow-400' : 'text-muted-foreground' },
                ].map(({ label, value, color }) => (
                  <div key={label} className="text-center">
                    <p className={`text-2xl font-bold ${color}`}>{value}</p>
                    <p className="text-[10px] text-muted-foreground uppercase tracking-wide mt-0.5">{label}</p>
                  </div>
                ))}
              </div>
              {policyStats.expired > 0 && (
                <div className="flex items-center gap-1.5 text-xs text-red-400 font-medium rounded-md bg-red-500/10 px-2 py-1.5">
                  <AlertTriangle className="h-3 w-3" />
                  {policyStats.expired} expired
                </div>
              )}
              <div className="flex items-center justify-end mt-2 text-xs text-muted-foreground/50 gap-1">
                View all <ArrowUpRight className="h-3 w-3" />
              </div>
            </CardContent>
          </Card>
        )}

        {/* Access Reviews */}
        {accessReviewStats && accessReviewStats.total > 0 && (
          <Card
            className="cursor-pointer hover:shadow-md hover:-translate-y-px transition-all duration-200"
            onClick={() => navigate('/access-reviews')}
          >
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <div className="flex h-6 w-6 items-center justify-center rounded-md bg-violet-500/10">
                  <UserCheck className="h-3.5 w-3.5 text-violet-400" />
                </div>
                Access Reviews
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-3 gap-3 mb-3">
                {[
                  { label: 'Total',      value: accessReviewStats.total,       color: 'text-foreground' },
                  { label: 'In Progress',value: accessReviewStats.in_progress, color: 'text-blue-400' },
                  { label: 'Completed',  value: accessReviewStats.completed,   color: 'text-green-400' },
                ].map(({ label, value, color }) => (
                  <div key={label} className="text-center">
                    <p className={`text-2xl font-bold ${color}`}>{value}</p>
                    <p className="text-[10px] text-muted-foreground uppercase tracking-wide mt-0.5">{label}</p>
                  </div>
                ))}
              </div>
              <div className="flex items-center gap-3 text-xs flex-wrap">
                {accessReviewStats.overdue > 0 && (
                  <span className="flex items-center gap-1 text-red-400 font-medium">
                    <AlertTriangle className="h-3 w-3" />
                    {accessReviewStats.overdue} overdue
                  </span>
                )}
                {accessReviewStats.due_this_month > 0 && (
                  <span className="flex items-center gap-1 text-yellow-400 font-medium">
                    <Clock className="h-3 w-3" />
                    {accessReviewStats.due_this_month} due this month
                  </span>
                )}
                {accessReviewStats.overdue === 0 && accessReviewStats.in_progress === 0 && (
                  <span className="flex items-center gap-1 text-green-400 font-medium">
                    <CheckCircle2 className="h-3 w-3" />
                    All complete
                  </span>
                )}
                <div className="ml-auto flex items-center gap-1 text-muted-foreground/50">
                  View all <ArrowUpRight className="h-3 w-3" />
                </div>
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      {/* ── Recent audits ── */}
      <Card>
        <CardHeader className="pb-3 flex flex-row items-center justify-between">
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <div className="flex h-6 w-6 items-center justify-center rounded-md bg-indigo-500/10">
              <Briefcase className="h-3.5 w-3.5 text-indigo-400" />
            </div>
            Recent Audits
          </CardTitle>
          <button
            onClick={() => navigate('/reports')}
            className="text-xs text-muted-foreground/50 hover:text-foreground transition-colors flex items-center gap-1"
          >
            View all <ArrowUpRight className="h-3 w-3" />
          </button>
        </CardHeader>
        <CardContent>
          {data.recent_jobs.length === 0 ? (
            <div className="flex flex-col items-center gap-4 py-10 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-muted">
                <Layers className="h-7 w-7 text-muted-foreground/30" />
              </div>
              <div>
                <p className="font-semibold text-sm">No audits yet</p>
                <p className="text-xs text-muted-foreground mt-1 max-w-xs">
                  Add a connection and kick off your first scan.
                </p>
              </div>
              <button
                onClick={() => navigate('/audit-types')}
                className="flex items-center gap-2 rounded-lg bg-indigo-500 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-600 transition-colors"
              >
                <Layers className="h-4 w-4" />
                Choose Audit Type
              </button>
            </div>
          ) : (
            <div className="divide-y divide-border/50">
              {data.recent_jobs.map((job) => (
                <div
                  key={job.id}
                  className="flex items-center gap-4 py-3 first:pt-0 last:pb-0 cursor-pointer rounded-lg hover:bg-muted/30 transition-colors px-2 -mx-2"
                  onClick={() => navigate(`/jobs/${job.id}`)}
                >
                  <StatusBadge status={job.status} />
                  <div className="flex-1 min-w-0">
                    <p className="font-medium text-sm truncate">{job.connection_name}</p>
                    <p className="text-xs text-muted-foreground">{formatDate(job.started_at)}</p>
                  </div>
                  {job.status === 'done' && (
                    <FindingsBadges
                      critical={job.findings_critical}
                      high={job.findings_high}
                      medium={job.findings_medium}
                      low={job.findings_low}
                    />
                  )}
                  <ChevronRight className="h-4 w-4 text-muted-foreground/30 shrink-0" />
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
