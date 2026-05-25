import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  ShieldCheck, LayoutDashboard, ClipboardCheck, FolderOpen, ScrollText,
  ShieldAlert, MessageSquare, Download, CheckCircle2, XCircle, AlertCircle,
  Info, Clock, ExternalLink, Send,
} from 'lucide-react'
import { toast } from 'sonner'
import { auditorPortalApi, type AuditorPortalInfo, type AuditorPermission } from '@/lib/api'

// ── Severity helpers ──────────────────────────────────────────────────────────

const SEV_COLORS: Record<string, string> = {
  critical: '#ef4444',
  high:     '#f97316',
  medium:   '#eab308',
  low:      '#3b82f6',
}

function SevBar({ label, count, total }: { label: string; count: number; total: number }) {
  const pct = total > 0 ? Math.round((count / total) * 100) : 0
  return (
    <div className="flex items-center gap-3">
      <span className="w-16 text-xs capitalize" style={{ color: SEV_COLORS[label] }}>{label}</span>
      <div className="flex-1 h-3 rounded-full bg-gray-800 overflow-hidden">
        <div
          className="h-3 rounded-full transition-all"
          style={{ width: `${pct}%`, background: SEV_COLORS[label] }}
        />
      </div>
      <span className="w-8 text-right text-sm font-bold text-white">{count}</span>
    </div>
  )
}

// ── Compliance status badge ───────────────────────────────────────────────────

function CtrlStatus({ status }: { status: string }) {
  const map: Record<string, { cls: string; icon: React.ElementType }> = {
    met:          { cls: 'text-green-400', icon: CheckCircle2 },
    partial:      { cls: 'text-yellow-400', icon: AlertCircle },
    not_met:      { cls: 'text-red-400', icon: XCircle },
    not_assessed: { cls: 'text-gray-500', icon: Info },
  }
  const { cls, icon: Icon } = map[status] ?? map.not_assessed
  return <Icon className={`h-4 w-4 ${cls}`} />
}

// ── Section: Overview ─────────────────────────────────────────────────────────

function OverviewSection({ info }: { info: AuditorPortalInfo }) {
  const expires = new Date(info.expires_at)
  const daysLeft = Math.ceil((expires.getTime() - Date.now()) / (1000 * 60 * 60 * 24))

  return (
    <div className="space-y-6">
      <div className="rounded-xl border border-white/10 bg-white/5 p-6">
        <h2 className="text-xl font-semibold text-white mb-2">Welcome, {info.name}</h2>
        <p className="text-gray-400 text-sm leading-relaxed">
          You have been granted read-only access to the security documentation for{' '}
          <span className="text-white font-medium">{info.company}</span>. This portal provides
          compliance readiness data, evidence artifacts, and security policies for your review.
        </p>
      </div>

      <div className="rounded-xl border border-white/10 bg-white/5 p-4 flex items-center gap-3">
        <Clock className="h-5 w-5 shrink-0 text-yellow-400" />
        <div>
          <p className="text-sm font-medium text-white">Portal Access</p>
          <p className="text-xs text-gray-400">
            This portal expires on{' '}
            <span className="text-white font-medium">{expires.toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric' })}</span>
            {' '}({daysLeft} day{daysLeft !== 1 ? 's' : ''} remaining)
          </p>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-medium text-gray-400 mb-3 uppercase tracking-wider">Your Access</h3>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {(info.permissions as AuditorPermission[]).map((perm) => {
            const labels: Record<AuditorPermission, { label: string; icon: React.ElementType }> = {
              compliance: { label: 'Compliance Readiness', icon: ClipboardCheck },
              evidence:   { label: 'Evidence Library',    icon: FolderOpen },
              policies:   { label: 'Policies',            icon: ScrollText },
              findings:   { label: 'Findings Summary',    icon: ShieldAlert },
            }
            const { label, icon: Icon } = labels[perm] ?? { label: perm, icon: Info }
            return (
              <div key={perm} className="rounded-lg border border-indigo-500/30 bg-indigo-500/5 p-3 flex items-center gap-2">
                <Icon className="h-4 w-4 text-indigo-400 shrink-0" />
                <span className="text-xs text-white font-medium">{label}</span>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

// ── Section: Compliance ───────────────────────────────────────────────────────

function ComplianceSection({ token }: { token: string }) {
  const { data: frameworks, isLoading } = useQuery({
    queryKey: ['auditor-compliance', token],
    queryFn: () => auditorPortalApi.compliance(token),
  })
  const [activeSlug, setActiveSlug] = useState<string | null>(null)

  if (isLoading) return <p className="text-gray-400 text-sm py-8">Loading compliance data…</p>
  if (!frameworks?.length) return <p className="text-gray-400 text-sm py-8">No compliance data available.</p>

  const active = frameworks.find((f) => f.slug === activeSlug) ?? frameworks[0]

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        {frameworks.map((fw) => {
          const barColor = fw.score >= 80 ? 'bg-green-500' : fw.score >= 50 ? 'bg-yellow-500' : 'bg-red-500'
          const isActive = (activeSlug ?? frameworks[0].slug) === fw.slug
          return (
            <button
              key={fw.slug}
              onClick={() => setActiveSlug(fw.slug)}
              className={`flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm transition-colors ${
                isActive ? 'border-indigo-500/50 bg-indigo-500/10 text-white' : 'border-white/10 text-gray-400 hover:border-white/20 hover:text-white'
              }`}
            >
              <span>{fw.name}</span>
              <div className="flex items-center gap-1.5">
                <div className="h-1.5 w-12 rounded-full bg-gray-700">
                  <div className={`h-1.5 rounded-full ${barColor}`} style={{ width: `${fw.score}%` }} />
                </div>
                <span className="font-bold text-xs">{fw.score}%</span>
              </div>
            </button>
          )
        })}
      </div>

      {active && (
        <div className="rounded-xl border border-white/10 bg-white/5 overflow-hidden">
          <div className="p-4 border-b border-white/10">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="font-semibold text-white">{active.name} <span className="text-gray-400 font-normal text-sm">v{active.version}</span></h3>
                <p className="text-xs text-gray-400 mt-0.5">{active.description}</p>
              </div>
              <div className="text-right">
                <p className="text-2xl font-bold text-white">{active.score}%</p>
                <p className="text-xs text-gray-400">{active.met_count}/{active.total_count} controls met</p>
              </div>
            </div>
          </div>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/10 text-left">
                <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-24">Control</th>
                <th className="px-4 py-2.5 text-xs font-medium text-gray-400">Name</th>
                <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-28 text-right">Status</th>
                <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-24 text-right">Findings</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/5">
              {(active.controls ?? []).map((ctrl) => (
                <tr key={ctrl.ctrl_id} className="hover:bg-white/3 transition-colors">
                  <td className="px-4 py-3 font-mono text-xs text-gray-400">{ctrl.ctrl_id}</td>
                  <td className="px-4 py-3">
                    <p className="font-medium text-white text-xs">{ctrl.name}</p>
                    <p className="text-xs text-gray-500 mt-0.5 line-clamp-1">{ctrl.description}</p>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1.5">
                      <CtrlStatus status={ctrl.status} />
                      <span className="text-xs capitalize text-gray-400">
                        {ctrl.status.replace('_', ' ')}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-right">
                    {ctrl.finding_count > 0 ? (
                      <span className="text-xs text-red-400 font-medium">{ctrl.open_count} open</span>
                    ) : (
                      <span className="text-xs text-gray-500">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ── Section: Evidence ─────────────────────────────────────────────────────────

function EvidenceSection({ token }: { token: string }) {
  const { data: items, isLoading } = useQuery({
    queryKey: ['auditor-evidence', token],
    queryFn: () => auditorPortalApi.evidence(token),
  })

  if (isLoading) return <p className="text-gray-400 text-sm py-8">Loading evidence…</p>
  if (!items?.length) return <p className="text-gray-400 text-sm py-8">No evidence items available.</p>

  const typeIcon: Record<string, string> = {
    policy: '📄', screenshot: '🖼️', report: '📊', log: '📋', other: '📎',
  }

  return (
    <div className="rounded-xl border border-white/10 bg-white/5 overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-white/10 text-left">
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400">Name</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400">Type</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400">Description</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-28">Expires</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-24 text-right">Action</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-white/5">
          {items.map((it) => {
            const expires = new Date(it.expires_at)
            const expired = expires < new Date()
            return (
              <tr key={it.id} className="hover:bg-white/3 transition-colors">
                <td className="px-4 py-3">
                  <p className="text-white font-medium text-xs flex items-center gap-2">
                    <span>{typeIcon[it.evidence_type] ?? '📎'}</span>
                    {it.name}
                  </p>
                </td>
                <td className="px-4 py-3">
                  <span className="capitalize text-xs text-gray-400">{it.evidence_type}</span>
                </td>
                <td className="px-4 py-3 text-xs text-gray-400 max-w-xs truncate">{it.description || '—'}</td>
                <td className="px-4 py-3 text-xs">
                  <span className={expired ? 'text-red-400' : 'text-gray-400'}>
                    {expires.toLocaleDateString()}
                  </span>
                </td>
                <td className="px-4 py-3 text-right">
                  <a
                    href={auditorPortalApi.downloadEvidenceUrl(token, it.id)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 rounded border border-white/10 px-2.5 py-1 text-xs text-gray-300 hover:border-indigo-500/50 hover:text-indigo-400 transition-colors"
                  >
                    <Download className="h-3 w-3" />
                    Download
                  </a>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// ── Section: Policies ─────────────────────────────────────────────────────────

function PoliciesSection({ token }: { token: string }) {
  const { data: policies, isLoading } = useQuery({
    queryKey: ['auditor-policies', token],
    queryFn: () => auditorPortalApi.policies(token),
  })

  if (isLoading) return <p className="text-gray-400 text-sm py-8">Loading policies…</p>
  if (!policies?.length) return <p className="text-gray-400 text-sm py-8">No approved policies available.</p>

  return (
    <div className="rounded-xl border border-white/10 bg-white/5 overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-white/10 text-left">
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400">Policy</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400">Category</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-28">Version</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-32">Approved</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-28">Review Date</th>
            <th className="px-4 py-2.5 text-xs font-medium text-gray-400 w-24 text-right">Action</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-white/5">
          {policies.map((p) => (
            <tr key={p.id} className="hover:bg-white/3 transition-colors">
              <td className="px-4 py-3">
                <p className="text-white font-medium text-sm">{p.name}</p>
                {p.file_name && (
                  <p className="text-xs text-gray-500 mt-0.5">{p.file_name}</p>
                )}
              </td>
              <td className="px-4 py-3 text-xs text-gray-400 capitalize">{p.category || '—'}</td>
              <td className="px-4 py-3 text-xs text-gray-400">v{p.version}</td>
              <td className="px-4 py-3 text-xs text-gray-400">
                {p.approved_at ? new Date(p.approved_at).toLocaleDateString() : '—'}
              </td>
              <td className="px-4 py-3 text-xs">
                {p.review_date ? (
                  <span className={new Date(p.review_date) < new Date() ? 'text-yellow-400' : 'text-gray-400'}>
                    {p.review_date}
                  </span>
                ) : '—'}
              </td>
              <td className="px-4 py-3 text-right">
                <a
                  href={auditorPortalApi.downloadPolicyUrl(token, p.id)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 rounded border border-white/10 px-2.5 py-1 text-xs text-gray-300 hover:border-indigo-500/50 hover:text-indigo-400 transition-colors"
                >
                  <Download className="h-3 w-3" />
                  Download
                </a>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ── Section: Findings Summary ─────────────────────────────────────────────────

function FindingsSummarySection({ token }: { token: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ['auditor-findings', token],
    queryFn: () => auditorPortalApi.findingsSummary(token),
  })

  if (isLoading) return <p className="text-gray-400 text-sm py-8">Loading findings summary…</p>
  if (!data) return <p className="text-gray-400 text-sm py-8">No findings data available.</p>

  return (
    <div className="space-y-6">
      <div className="rounded-xl border border-white/10 bg-white/5 p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-medium text-white">Findings by Severity</h3>
          <span className="text-2xl font-bold text-white">{data.total} <span className="text-sm font-normal text-gray-400">total</span></span>
        </div>
        <div className="space-y-3">
          {(['critical', 'high', 'medium', 'low'] as const).map((s) => (
            <SevBar key={s} label={s} count={data[s]} total={data.total} />
          ))}
        </div>
      </div>

      {data.by_connection.length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-gray-400 mb-3 uppercase tracking-wider">By Connection</h3>
          <div className="grid gap-3 sm:grid-cols-2">
            {data.by_connection.map((c) => (
              <div key={c.connection_id} className="rounded-xl border border-white/10 bg-white/5 p-4">
                <p className="text-white font-medium text-sm mb-3">{c.connection_name}</p>
                <div className="grid grid-cols-4 gap-2 text-center">
                  {(['critical', 'high', 'medium', 'low'] as const).map((s) => (
                    <div key={s}>
                      <p className="text-lg font-bold" style={{ color: SEV_COLORS[s] }}>{c[s]}</p>
                      <p className="text-xs text-gray-500 capitalize">{s}</p>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="rounded-xl border border-yellow-500/20 bg-yellow-500/5 p-4 flex items-start gap-3">
        <Info className="h-4 w-4 text-yellow-400 shrink-0 mt-0.5" />
        <p className="text-xs text-yellow-200/80">
          This is a summary view showing only counts. Individual finding details including resource names,
          file paths, and remediation steps are not visible in this portal.
        </p>
      </div>
    </div>
  )
}

// ── Section: Comments ─────────────────────────────────────────────────────────

function CommentsSection({ token, company }: { token: string; company: string }) {
  const [body, setBody] = useState('')
  const [section, setSection] = useState('general')

  const { data: comments = [], refetch } = useQuery({
    queryKey: ['auditor-comments', token],
    queryFn: () => auditorPortalApi.listComments(token),
    refetchInterval: 30_000,
  })

  const addMutation = useMutation({
    mutationFn: () => auditorPortalApi.addComment(token, { section, body }),
    onSuccess: () => {
      setBody('')
      refetch()
      toast.success(`Message sent to ${company}`)
    },
    onError: () => toast.error('Failed to send comment'),
  })

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-white/10 bg-white/5 p-4 space-y-3">
        <h3 className="text-sm font-medium text-white">Send a message or question</h3>
        <div className="flex gap-2">
          <select
            className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-gray-300 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            value={section}
            onChange={(e) => setSection(e.target.value)}
          >
            <option value="general">General</option>
            <option value="compliance">Compliance</option>
            <option value="evidence">Evidence</option>
            <option value="policies">Policies</option>
            <option value="findings">Findings</option>
          </select>
          <textarea
            className="flex-1 rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white placeholder:text-gray-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-none"
            rows={3}
            placeholder="Ask a question or leave a comment for the security team…"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) addMutation.mutate()
            }}
          />
        </div>
        <div className="flex items-center justify-between">
          <p className="text-xs text-gray-500">⌘+Enter to send</p>
          <button
            disabled={!body.trim() || addMutation.isPending}
            onClick={() => addMutation.mutate()}
            className="flex items-center gap-2 rounded-lg bg-indigo-500 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-600 transition-colors disabled:opacity-50"
          >
            <Send className="h-3.5 w-3.5" />
            {addMutation.isPending ? 'Sending…' : 'Send'}
          </button>
        </div>
      </div>

      {comments.length > 0 && (
        <div className="space-y-3">
          <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider">Conversation</h3>
          {comments.map((c) => (
            <div key={c.id} className="rounded-xl border border-white/10 bg-white/5 p-4">
              <div className="flex items-start justify-between gap-2 mb-2">
                <div className="flex items-center gap-2">
                  <div className="h-7 w-7 rounded-full bg-indigo-500/20 flex items-center justify-center text-xs font-bold text-indigo-400">
                    {c.auditor_name.slice(0, 1).toUpperCase()}
                  </div>
                  <div>
                    <p className="text-xs font-medium text-white">{c.auditor_name}</p>
                    <p className="text-xs text-gray-500 capitalize">{c.section}</p>
                  </div>
                </div>
                <span className="text-xs text-gray-500 shrink-0">
                  {new Date(c.created_at).toLocaleString()}
                </span>
              </div>
              <p className="text-sm text-gray-300 leading-relaxed whitespace-pre-wrap">{c.body}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ── Main portal ───────────────────────────────────────────────────────────────

type Section = 'overview' | 'compliance' | 'evidence' | 'policies' | 'findings' | 'comments'

const NAV_ITEMS: { id: Section; label: string; icon: React.ElementType; perm?: AuditorPermission }[] = [
  { id: 'overview',    label: 'Overview',          icon: LayoutDashboard },
  { id: 'compliance',  label: 'Compliance',        icon: ClipboardCheck,  perm: 'compliance' },
  { id: 'evidence',    label: 'Evidence',          icon: FolderOpen,      perm: 'evidence'   },
  { id: 'policies',    label: 'Policies',          icon: ScrollText,      perm: 'policies'   },
  { id: 'findings',    label: 'Findings Summary',  icon: ShieldAlert,     perm: 'findings'   },
  { id: 'comments',    label: 'Comments',          icon: MessageSquare },
]

export function AuditorPortal() {
  const { token } = useParams<{ token: string }>()
  const [active, setActive] = useState<Section>('overview')

  const { data: info, isLoading, error } = useQuery({
    queryKey: ['auditor-verify', token],
    queryFn: () => auditorPortalApi.verify(token!),
    enabled: !!token,
    retry: false,
  })

  if (isLoading) {
    return (
      <div className="min-h-screen bg-[#0d1f2d] flex items-center justify-center">
        <div className="text-center space-y-3">
          <ShieldCheck className="h-10 w-10 text-indigo-400 mx-auto animate-pulse" />
          <p className="text-gray-400 text-sm">Loading secure portal…</p>
        </div>
      </div>
    )
  }

  if (error || !info) {
    return (
      <div className="min-h-screen bg-[#0d1f2d] flex items-center justify-center px-4">
        <div className="text-center space-y-4 max-w-sm">
          <div className="h-16 w-16 rounded-full bg-red-500/10 flex items-center justify-center mx-auto">
            <XCircle className="h-8 w-8 text-red-400" />
          </div>
          <h1 className="text-xl font-semibold text-white">Portal Not Found</h1>
          <p className="text-gray-400 text-sm">
            This auditor portal link is invalid or has expired. Please contact the security team for a new invite.
          </p>
        </div>
      </div>
    )
  }

  const permitted = (perm?: AuditorPermission) => !perm || info.permissions.includes(perm)
  const visibleNav = NAV_ITEMS.filter((n) => permitted(n.perm))

  return (
    <div className="min-h-screen bg-[#0d1f2d] text-white flex flex-col">
      {/* Top bar */}
      <header className="border-b border-white/10 bg-[#0d1f2d]/95 backdrop-blur sticky top-0 z-10">
        <div className="mx-auto max-w-7xl px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <ShieldCheck className="h-6 w-6 text-indigo-400" strokeWidth={1.8} />
            <div>
              <span className="font-bold text-white">CloudSec<span className="text-indigo-400">Guard</span></span>
              <span className="text-gray-400 text-sm ml-3">{info.company} Security Review Portal</span>
            </div>
          </div>
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <div className="h-7 w-7 rounded-full bg-indigo-500/20 flex items-center justify-center text-xs font-bold text-indigo-400">
              {info.name.slice(0, 1).toUpperCase()}
            </div>
            <span>{info.name}</span>
          </div>
        </div>
      </header>

      <div className="flex flex-1 mx-auto w-full max-w-7xl px-4 py-6 gap-6">
        {/* Sidebar */}
        <aside className="w-52 shrink-0">
          <nav className="space-y-0.5">
            {visibleNav.map((item) => {
              const Icon = item.icon
              const isActive = active === item.id
              return (
                <button
                  key={item.id}
                  onClick={() => setActive(item.id)}
                  className={`w-full flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-indigo-500/15 text-indigo-400'
                      : 'text-gray-400 hover:bg-white/5 hover:text-white'
                  }`}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  {item.label}
                </button>
              )
            })}
          </nav>

          <div className="mt-6 rounded-lg border border-white/10 bg-white/5 p-3">
            <p className="text-xs text-gray-500 flex items-center gap-1.5 mb-1">
              <Clock className="h-3 w-3" /> Portal expires
            </p>
            <p className="text-xs font-medium text-white">
              {new Date(info.expires_at).toLocaleDateString()}
            </p>
          </div>
        </aside>

        {/* Main content */}
        <main className="flex-1 min-w-0">
          {active === 'overview'   && <OverviewSection info={info} />}
          {active === 'compliance' && permitted('compliance') && <ComplianceSection token={token!} />}
          {active === 'evidence'   && permitted('evidence')   && <EvidenceSection token={token!} />}
          {active === 'policies'   && permitted('policies')   && <PoliciesSection token={token!} />}
          {active === 'findings'   && permitted('findings')   && <FindingsSummarySection token={token!} />}
          {active === 'comments'   && <CommentsSection token={token!} company={info.company} />}
        </main>
      </div>

      {/* Footer */}
      <footer className="border-t border-white/10 py-4">
        <div className="mx-auto max-w-7xl px-4 flex items-center justify-between text-xs text-gray-500">
          <div className="flex items-center gap-1.5">
            <ShieldCheck className="h-3.5 w-3.5 text-indigo-400" />
            Powered by <span className="text-white font-medium">CloudSecGuard</span>
          </div>
          <div className="flex items-center gap-3">
            <span>Portal expires {new Date(info.expires_at).toLocaleDateString()}</span>
            <a
              href="/"
              className="flex items-center gap-1 hover:text-white transition-colors"
            >
              <ExternalLink className="h-3 w-3" />
              CloudSecGuard App
            </a>
          </div>
        </div>
      </footer>
    </div>
  )
}
