import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, RotateCcw, ChevronDown, ChevronUp, Share2, TrendingUp, TrendingDown, Lock } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { StatusBadge } from '@/components/StatusBadge'
import { FindingsBadges } from '@/components/FindingsBadges'
import { DownloadButtons } from '@/components/DownloadButtons'
import { ProgressBar } from '@/components/ProgressBar'
import { jobsApi, findingsApi, codeFindingsApi, tfFindingsApi, shareApi, compareApi, licenseApi, type AuditJob, type Finding, type CodeFinding } from '@/lib/api'
import { formatDate, formatDuration, copyToClipboard } from '@/lib/utils'

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
      <div
        className="flex items-start justify-between gap-3 cursor-pointer"
        onClick={() => setOpen((o) => !o)}
      >
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <Badge variant="outline" className={`${cfg.color} text-xs capitalize`}>
              {f.severity}
            </Badge>
            {f.category && (
              <span className="text-xs text-muted-foreground">{f.category}</span>
            )}
          </div>
          <p className="font-medium text-sm mt-1">{f.title}</p>
          {f.resource_name && (
            <p className="text-xs text-muted-foreground mt-0.5">
              {f.resource_type}: {f.resource_name}
            </p>
          )}
        </div>
        {open ? (
          <ChevronUp className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
        ) : (
          <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
        )}
      </div>

      {open && (
        <div className="mt-4 space-y-3 text-sm border-t border-white/5 pt-3">
          {f.risk && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Risk</p>
              <p className="text-foreground/80">{f.risk}</p>
            </div>
          )}
          {f.business_impact && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Business impact</p>
              <p className="text-foreground/80">{f.business_impact}</p>
            </div>
          )}
          {f.evidence && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Evidence</p>
              <p className="font-mono text-xs bg-black/20 rounded p-2 whitespace-pre-wrap">{f.evidence}</p>
            </div>
          )}
          {f.recommendation && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Recommendation</p>
              <p className="text-foreground/80">{f.recommendation}</p>
            </div>
          )}
          {f.remediation && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Remediation</p>
              <p className="text-foreground/80">{f.remediation}</p>
            </div>
          )}
          {f.timeline && (
            <p className="text-xs text-muted-foreground">Timeline: {f.timeline}</p>
          )}
        </div>
      )}
    </div>
  )
}

const toolColors: Record<string, string> = {
  gitleaks: 'bg-red-500/20 text-red-300',
  semgrep: 'bg-purple-500/20 text-purple-300',
  'npm audit': 'bg-yellow-500/20 text-yellow-300',
  trivy: 'bg-blue-500/20 text-blue-300',
  hclscan: 'bg-cyan-500/20 text-cyan-300',
}

function CodeFindingCard({ f }: { f: CodeFinding }) {
  const [open, setOpen] = useState(false)
  const sev = (f.severity ?? '').toLowerCase()
  const cfg = severityConfig[sev] ?? { color: 'text-muted-foreground', bg: 'bg-muted' }
  const toolCls = toolColors[f.tool?.toLowerCase()] ?? 'bg-muted text-muted-foreground'

  return (
    <div className={`rounded-lg border p-4 ${cfg.bg}`}>
      <div
        className="flex items-start justify-between gap-3 cursor-pointer"
        onClick={() => setOpen((o) => !o)}
      >
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" className={`${cfg.color} text-xs capitalize`}>
              {f.severity}
            </Badge>
            {f.tool && (
              <span className={`rounded px-1.5 py-0.5 text-xs ${toolCls}`}>{f.tool}</span>
            )}
            {f.rule_id && (
              <span className="font-mono text-xs text-muted-foreground">{f.rule_id}</span>
            )}
          </div>
          <p className="font-medium text-sm mt-1">{f.title || f.rule_id}</p>
          {f.file && (
            <p className="font-mono text-xs text-muted-foreground mt-0.5 truncate">
              {f.file}{f.line ? `:${f.line}` : ''}
            </p>
          )}
          {f.package && (
            <p className="text-xs text-muted-foreground mt-0.5">
              {f.package}{f.version ? ` @ ${f.version}` : ''}
              {f.cve ? ` — ${f.cve}` : ''}
            </p>
          )}
        </div>
        {open ? (
          <ChevronUp className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
        ) : (
          <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
        )}
      </div>

      {open && (
        <div className="mt-4 space-y-3 text-sm border-t border-white/5 pt-3">
          {f.description && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Description</p>
              <p className="text-foreground/80">{f.description}</p>
            </div>
          )}
          {f.remediation && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Remediation</p>
              <p className="text-foreground/80">{f.remediation}</p>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function CodeFindingsTab({ jobId, queryKey }: { jobId: string; queryKey: string }) {
  const [severityFilter, setSeverityFilter] = useState('all')
  const [toolFilter, setToolFilter] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')

  const fetchFn = queryKey === 'code-findings' ? codeFindingsApi.get : tfFindingsApi.get

  const { data: rawFindings, isLoading } = useQuery({
    queryKey: [queryKey, jobId],
    queryFn: () => fetchFn(jobId),
  })
  const findings = rawFindings ?? []

  const tools = Array.from(new Set(findings.map((f) => f.tool).filter(Boolean)))

  const filtered = findings.filter((f) => {
    if (severityFilter !== 'all' && (f.severity ?? '').toLowerCase() !== severityFilter) return false
    if (toolFilter !== 'all' && f.tool?.toLowerCase() !== toolFilter) return false
    if (searchQuery) {
      const q = searchQuery.toLowerCase()
      return (
        (f.title ?? '').toLowerCase().includes(q) ||
        (f.rule_id ?? '').toLowerCase().includes(q) ||
        (f.file ?? '').toLowerCase().includes(q) ||
        (f.package ?? '').toLowerCase().includes(q)
      )
    }
    return true
  })

  const sevCounts = findings.reduce((acc, f) => {
    const s = (f.severity ?? '').toLowerCase()
    acc[s] = (acc[s] ?? 0) + 1
    return acc
  }, {} as Record<string, number>)

  if (isLoading) {
    return <p className="text-sm text-muted-foreground py-8 text-center">Loading findings...</p>
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2 items-center">
        <Input
          placeholder="Search findings..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="h-8 w-48 text-sm"
        />
        {['all', 'critical', 'high', 'medium', 'low'].map((sev) => (
          <Button
            key={sev}
            size="sm"
            variant={severityFilter === sev ? 'default' : 'outline'}
            className="h-8 text-xs capitalize"
            onClick={() => setSeverityFilter(sev)}
          >
            {sev === 'all' ? 'All' : `${sev} ${sevCounts[sev] ?? 0}`}
          </Button>
        ))}
        {tools.length > 1 && (
          <select
            value={toolFilter}
            onChange={(e) => setToolFilter(e.target.value)}
            className="h-8 rounded-md border border-border bg-background px-2 text-xs text-foreground"
          >
            <option value="all">All tools</option>
            {tools.map((t) => <option key={t} value={t!.toLowerCase()}>{t}</option>)}
          </select>
        )}
      </div>

      {filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground py-8 text-center">
          {findings.length === 0 ? 'No findings.' : 'No findings match the current filter.'}
        </p>
      ) : (
        <div className="space-y-2">
          {filtered.map((f, i) => <CodeFindingCard key={i} f={f} />)}
        </div>
      )}
    </div>
  )
}

export function JobDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)
  const [liveJob, setLiveJob] = useState<AuditJob | null>(null)
  const [severityFilter, setSeverityFilter] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')

  const [, setTick] = useState(0)

  const { data: job } = useQuery({
    queryKey: ['jobs', id],
    queryFn: () => jobsApi.get(id!),
    enabled: !!id,
  })

  const { data: rawFindings } = useQuery({
    queryKey: ['findings', id],
    queryFn: () => findingsApi.get(id!),
    enabled: !!id && (liveJob?.status === 'done' || job?.status === 'done'),
  })
  const findings = rawFindings ?? []

  const displayed = liveJob ?? job

  useEffect(() => {
    if (!id || !job) return
    if (job.status === 'done' || job.status === 'failed') return

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const token = localStorage.getItem('access_token') ?? ''
    const wsUrl = `${proto}//${host}/api/ws/jobs/${id}?token=${encodeURIComponent(token)}`

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        setLiveJob((prev) => ({
          ...(prev ?? job),
          status: msg.status ?? prev?.status ?? job.status,
          progress_msg: msg.progress_msg ?? prev?.progress_msg ?? job.progress_msg,
          error_msg: msg.error_msg ?? prev?.error_msg ?? job.error_msg,
          finished_at: msg.finished_at ?? prev?.finished_at ?? job.finished_at,
          findings_critical: msg.findings?.critical ?? prev?.findings_critical ?? job.findings_critical,
          findings_high: msg.findings?.high ?? prev?.findings_high ?? job.findings_high,
          findings_medium: msg.findings?.medium ?? prev?.findings_medium ?? job.findings_medium,
          findings_low: msg.findings?.low ?? prev?.findings_low ?? job.findings_low,
        }))
        if (msg.status === 'done' || msg.status === 'failed') {
          qc.invalidateQueries({ queryKey: ['jobs', id] })
          qc.invalidateQueries({ queryKey: ['jobs'] })
          qc.invalidateQueries({ queryKey: ['findings', id] })
          qc.invalidateQueries({ queryKey: ['dashboard'] })
        }
      } catch {
        // ignore
      }
    }

    ws.onerror = () => ws.close()

    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [id, job?.status])

  // Tick every second so the elapsed duration stays live.
  // We run this for the full component lifetime — formatDuration uses finished_at
  // once it's available, so the value naturally freezes when the job is done.
  useEffect(() => {
    const interval = setInterval(() => setTick((t) => t + 1), 1000)
    return () => clearInterval(interval)
  }, [])

  const rerunMutation = useMutation({
    mutationFn: () => jobsApi.run(job!.connection_id),
    onSuccess: (newJob) => {
      qc.invalidateQueries({ queryKey: ['jobs'] })
      toast.success('New audit started')
      navigate(`/jobs/${newJob.id}`)
    },
    onError: () => toast.error('Failed to start audit'),
  })

  const [shareUrl, setShareUrl] = useState<string | null>(null)
  const shareInputRef = useRef<HTMLInputElement>(null)

  const shareMutation = useMutation({
    mutationFn: () => shareApi.create(id!),
    onSuccess: (link) => {
      const url = `${window.location.origin}/share/${link.token}`
      setShareUrl(url)
      // Don't auto-copy here — the dialog provides an explicit Copy button
    },
    onError: () => toast.error('Failed to create share link'),
  })

  const { data: license } = useQuery({ queryKey: ['license'], queryFn: licenseApi.get })
  const canShare = license?.features?.includes('share_links') ?? false

  const { data: compareData } = useQuery({
    queryKey: ['compare', id],
    queryFn: () => compareApi.get(id!),
    enabled: !!id && displayed?.status === 'done',
  })

  if (!displayed) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    )
  }

  const isRunning = displayed.status === 'running' || displayed.status === 'pending'

  const filteredFindings = findings.filter((f) => {
    if (severityFilter !== 'all' && (f.severity ?? '').toLowerCase() !== severityFilter) return false
    if (searchQuery) {
      const q = searchQuery.toLowerCase()
      return (
        f.title.toLowerCase().includes(q) ||
        f.resource_name?.toLowerCase().includes(q) ||
        f.category?.toLowerCase().includes(q)
      )
    }
    return true
  })

  const severityCounts = findings.reduce(
    (acc, f) => {
      const s = (f.severity ?? '').toLowerCase()
      acc[s] = (acc[s] ?? 0) + 1
      return acc
    },
    {} as Record<string, number>
  )

  return (
    <div className="max-w-3xl space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => navigate('/jobs')}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <h1 className="text-2xl font-bold">{displayed.connection_name}</h1>
          <p className="text-sm text-muted-foreground">Job ID: {displayed.id}</p>
        </div>
        <StatusBadge status={displayed.status} />
      </div>

      {/* Progress */}
      {isRunning && (
        <Card>
          <CardContent className="pt-6 space-y-3">
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">{displayed.progress_msg || 'Working...'}</span>
            </div>
            <ProgressBar />
          </CardContent>
        </Card>
      )}

      {/* Error */}
      {displayed.status === 'failed' && displayed.error_msg && (
        <Card className="border-destructive/50">
          <CardContent className="pt-6">
            <p className="text-sm text-destructive">{displayed.error_msg}</p>
          </CardContent>
        </Card>
      )}

      {/* Tabs: Overview / Findings */}
      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          {displayed.status === 'done' && displayed.conn_type === 'code' && (
            <TabsTrigger value="code-findings">Code Findings</TabsTrigger>
          )}
          {displayed.status === 'done' && displayed.conn_type === 'code' && (
            <TabsTrigger value="tf-findings">Terraform</TabsTrigger>
          )}
          {displayed.status === 'done' && displayed.conn_type !== 'code' && (
            <TabsTrigger value="findings">
              Findings ({findings.length})
            </TabsTrigger>
          )}
          {displayed.status === 'done' && compareData && compareData.prev_job_id && (
            <TabsTrigger value="compare">
              Compare
            </TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          {/* Job info */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Job details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <p className="text-muted-foreground">Started</p>
                  <p className="font-medium">{formatDate(displayed.started_at)}</p>
                </div>
                <div>
                  <p className="text-muted-foreground">Duration</p>
                  <p className="font-medium">
                    {formatDuration(displayed.started_at, displayed.finished_at)}
                  </p>
                </div>
                {displayed.finished_at && (
                  <div>
                    <p className="text-muted-foreground">Finished</p>
                    <p className="font-medium">{formatDate(displayed.finished_at)}</p>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

          {/* Findings summary */}
          {displayed.status === 'done' && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Findings summary</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-4 gap-4">
                  {[
                    { label: 'Critical', count: displayed.findings_critical, color: 'bg-red-500/10 text-red-400 border-red-500/20' },
                    { label: 'High', count: displayed.findings_high, color: 'bg-orange-500/10 text-orange-400 border-orange-500/20' },
                    { label: 'Medium', count: displayed.findings_medium, color: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20' },
                    { label: 'Low', count: displayed.findings_low, color: 'bg-blue-500/10 text-blue-400 border-blue-500/20' },
                  ].map(({ label, count, color }) => (
                    <div key={label} className={`rounded-lg border p-4 text-center ${color}`}>
                      <p className="text-3xl font-bold">{count}</p>
                      <p className="text-sm mt-1">{label}</p>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Actions */}
          <div className="flex flex-wrap gap-3">
            {displayed.status === 'done' && <DownloadButtons jobId={displayed.id} />}
            {displayed.status === 'done' && (
              <Button
                variant="outline"
                onClick={() => {
                  if (!canShare) {
                    toast.error('Upgrade to Professional to use share links')
                    return
                  }
                  shareMutation.mutate()
                }}
                disabled={shareMutation.isPending}
                title={canShare ? 'Share report' : 'Requires Professional plan'}
              >
                {canShare
                  ? <Share2 className="mr-2 h-4 w-4" />
                  : <Lock className="mr-2 h-4 w-4 text-muted-foreground" />
                }
                Share
              </Button>
            )}
            {(displayed.status === 'done' || displayed.status === 'failed') && (
              <Button
                variant="outline"
                onClick={() => rerunMutation.mutate()}
                disabled={rerunMutation.isPending}
              >
                <RotateCcw className="mr-2 h-4 w-4" />
                Re-run audit
              </Button>
            )}
          </div>
        </TabsContent>

        <TabsContent value="findings" className="space-y-4">
          {/* Filters */}
          <div className="flex flex-wrap gap-2 items-center">
            <Input
              placeholder="Search findings..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="h-8 w-48 text-sm"
            />
            {['all', 'critical', 'high', 'medium', 'low'].map((sev) => (
              <Button
                key={sev}
                size="sm"
                variant={severityFilter === sev ? 'default' : 'outline'}
                className="h-8 text-xs capitalize"
                onClick={() => setSeverityFilter(sev)}
              >
                {sev === 'all' ? 'All' : `${sev} ${severityCounts[sev] ?? 0}`}
              </Button>
            ))}
          </div>

          {filteredFindings.length === 0 ? (
            <p className="text-sm text-muted-foreground py-8 text-center">
              No findings match the current filter.
            </p>
          ) : (
            <div className="space-y-2">
              {filteredFindings.map((f) => (
                <FindingCard key={f.id} f={f} />
              ))}
            </div>
          )}
        </TabsContent>

        {displayed.conn_type === 'code' && (
          <TabsContent value="code-findings">
            <CodeFindingsTab jobId={displayed.id} queryKey="code-findings" />
          </TabsContent>
        )}

        {displayed.conn_type === 'code' && (
          <TabsContent value="tf-findings">
            <CodeFindingsTab jobId={displayed.id} queryKey="tf-findings" />
          </TabsContent>
        )}

        {compareData && compareData.prev_job_id && (
          <TabsContent value="compare" className="space-y-4">
            <p className="text-xs text-muted-foreground">
              Compared with previous job{' '}
              <button
                className="underline text-indigo-400"
                onClick={() => navigate(`/jobs/${compareData.prev_job_id}`)}
              >
                {compareData.prev_job_id.slice(0, 8)}…
              </button>
            </p>

            <Card>
              <CardHeader>
                <CardTitle className="text-base flex items-center gap-2">
                  <TrendingUp className="h-4 w-4 text-red-400" />
                  New findings ({compareData.new_findings.length})
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {compareData.new_findings.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No new findings — great job!</p>
                ) : (
                  compareData.new_findings.map((f: any, i: number) => (
                    <FindingCard key={i} f={f} />
                  ))
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base flex items-center gap-2">
                  <TrendingDown className="h-4 w-4 text-green-400" />
                  Fixed findings ({compareData.fixed_findings.length})
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {compareData.fixed_findings.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No findings resolved since last audit.</p>
                ) : (
                  compareData.fixed_findings.map((f: any, i: number) => (
                    <FindingCard key={i} f={f} />
                  ))
                )}
              </CardContent>
            </Card>
          </TabsContent>
        )}
      </Tabs>

      <Dialog open={!!shareUrl} onOpenChange={(o) => !o && setShareUrl(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Share link created</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              Anyone with this link can view the audit findings without logging in.
            </p>
            <div className="flex gap-2">
              <Input
                ref={shareInputRef}
                readOnly
                value={shareUrl ?? ''}
                className="flex-1 font-mono text-xs"
                onFocus={(e) => e.target.select()}
              />
              <Button
                size="sm"
                onClick={async () => {
                  // Pass the visible input ref so the fallback can copy from it directly
                  const ok = await copyToClipboard(shareUrl ?? '', shareInputRef)
                  if (ok) {
                    toast.success('Link copied!')
                  } else {
                    toast.error('Copy failed — please select and copy the link manually')
                  }
                }}
              >
                Copy
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
