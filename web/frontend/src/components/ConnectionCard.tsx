import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Play, Pencil, Trash2, Globe, Clock, BarChart2, Lock, Code2, GitBranch, FolderOpen, ShieldCheck, Server } from 'lucide-react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import { ConfirmDialog } from './ConfirmDialog'
import { connectionsApi, jobsApi, schedulesApi, connectionHistoryApi, licenseApi, type Connection, type AuditJob } from '@/lib/api'
import { formatDate } from '@/lib/utils'
import { StatusBadge } from './StatusBadge'

interface Props {
  connection: Connection
  selected?: boolean
  onToggleSelect?: () => void
}

export function ConnectionCard({ connection: conn, selected, onToggleSelect }: Props) {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [scheduleOpen, setScheduleOpen] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)

  const { data: license } = useQuery({ queryKey: ['license'], queryFn: licenseApi.get })
  const canSchedule = license?.features?.includes('scheduled_audits') ?? false

  const deleteMutation = useMutation({
    mutationFn: () => connectionsApi.delete(conn.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['connections'] })
      toast.success('Connection deleted')
    },
    onError: () => toast.error('Failed to delete connection'),
  })

  const runMutation = useMutation({
    mutationFn: () => jobsApi.run(conn.id),
    onSuccess: (job) => {
      qc.invalidateQueries({ queryKey: ['jobs'] })
      qc.invalidateQueries({ queryKey: ['dashboard'] })
      toast.success('Audit started')
      navigate(`/jobs/${job.id}`)
    },
    onError: () => toast.error('Failed to start audit'),
  })

  const isCode = conn.conn_type === 'code'
  const isAWS = conn.conn_type === 'aws'
  const isNetworkScan = conn.conn_type === 'ssl' || conn.conn_type === 'dns'

  const scopeColor: Record<string, string> = {
    project: 'secondary',
    hybrid: 'info',
    account: 'warning',
  }

  let stackBadges: string[] = []
  if (isCode && conn.last_stack_detected) {
    try { stackBadges = JSON.parse(conn.last_stack_detected) } catch { /* ignore */ }
  }

  const repoLabel = isCode
    ? conn.repo_source === 'git'
      ? conn.repo_url?.replace(/^https?:\/\//, '')
      : conn.repo_local_path
    : null

  return (
    <>
      <Card className={`group flex flex-col transition-shadow hover:shadow-md ${selected ? 'ring-2 ring-indigo-500' : ''}`}>
        <CardHeader className="pb-3">
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-2">
              {onToggleSelect && (
                <input
                  type="checkbox"
                  checked={!!selected}
                  onChange={onToggleSelect}
                  className="h-4 w-4 rounded border-border accent-indigo-500 shrink-0"
                  onClick={(e) => e.stopPropagation()}
                />
              )}
              <CardTitle className="text-base">{conn.name}</CardTitle>
            </div>
            {isCode ? (
              <Badge variant="secondary" className="flex items-center gap-1">
                <Code2 className="h-3 w-3" />
                Code
              </Badge>
            ) : conn.conn_type === 'ssl' ? (
              <Badge variant="secondary" className="flex items-center gap-1 text-blue-400">
                <Lock className="h-3 w-3" />
                SSL/TLS
              </Badge>
            ) : conn.conn_type === 'dns' ? (
              <Badge variant="secondary" className="flex items-center gap-1 text-orange-400">
                <Globe className="h-3 w-3" />
                DNS
              </Badge>
            ) : isAWS ? (
              <Badge variant="secondary" className="flex items-center gap-1 text-yellow-400">
                <Server className="h-3 w-3" />
                AWS
              </Badge>
            ) : (
              <Badge variant={(scopeColor[conn.scope_mode] as 'secondary' | 'info' | 'warning') ?? 'secondary'}>
                {conn.scope_mode}
              </Badge>
            )}
          </div>
          <CardDescription className="flex items-center gap-1 text-xs truncate">
            {isCode ? (
              conn.repo_source === 'local'
                ? <><FolderOpen className="h-3 w-3 shrink-0" /><span className="truncate">{repoLabel}</span></>
                : <><Globe className="h-3 w-3 shrink-0" /><span className="truncate">{repoLabel}</span></>
            ) : isNetworkScan ? (
              <><ShieldCheck className="h-3 w-3 shrink-0" /><span className="truncate">{conn.domains || 'No domains configured'}</span></>
            ) : isAWS ? (
              <><Server className="h-3 w-3 shrink-0" /><span className="truncate font-mono">{conn.aws_access_key_masked || 'AWS'} · {conn.aws_region || 'us-east-1'}</span></>
            ) : (
              <><Globe className="h-3 w-3" />{conn.do_token_masked}</>
            )}
          </CardDescription>
        </CardHeader>

        <CardContent className="flex-1 pb-3">
          {isCode ? (
            <div className="space-y-2">
              {conn.repo_source === 'git' && conn.repo_branch && (
                <div className="flex items-center gap-1 text-xs text-muted-foreground">
                  <GitBranch className="h-3 w-3" />
                  {conn.repo_branch}
                </div>
              )}
              {stackBadges.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  {stackBadges.map((s) => (
                    <span key={s} className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                      {s}
                    </span>
                  ))}
                </div>
              )}
            </div>
          ) : isNetworkScan ? (
            <div className="flex flex-wrap gap-1">
              {(conn.domains ?? '').split(',').filter(Boolean).map((d) => (
                <span key={d} className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground truncate max-w-[140px]">
                  {d.trim()}
                </span>
              ))}
            </div>
          ) : isAWS ? (
            <div className="flex flex-wrap gap-1">
              {['EC2', 'S3', 'IAM', 'RDS', 'SGs', 'VPC'].map((svc) => (
                <span key={svc} className="rounded bg-yellow-500/10 border border-yellow-500/20 px-1.5 py-0.5 text-xs text-yellow-400/80">
                  {svc}
                </span>
              ))}
            </div>
          ) : (
            <>
              {conn.project_id && (
                <p className="text-xs text-muted-foreground truncate" title={conn.project_id}>
                  Project: {conn.project_id}
                </p>
              )}
              {conn.spaces_buckets && (
                <p className="text-xs text-muted-foreground truncate mt-1">
                  Spaces: {conn.spaces_buckets}
                </p>
              )}
            </>
          )}
        </CardContent>

        <CardFooter className="gap-2 pt-0">
          <Button
            size="sm"
            className="flex-1 bg-indigo-500 text-white hover:bg-indigo-600"
            onClick={() => runMutation.mutate()}
            disabled={runMutation.isPending}
          >
            <Play className="mr-1.5 h-3.5 w-3.5" />
            {runMutation.isPending ? 'Starting...' : 'Run Audit'}
          </Button>

          <Button size="icon" variant="ghost" title="History" onClick={() => setHistoryOpen(true)}>
            <BarChart2 className="h-4 w-4" />
          </Button>

          <Button
            size="icon"
            variant="ghost"
            title={canSchedule ? 'Schedule' : 'Scheduled audits require a paid plan'}
            onClick={() => canSchedule ? setScheduleOpen(true) : toast.error('Upgrade to Professional to use scheduled audits')}
          >
            {canSchedule
              ? <Clock className="h-4 w-4" />
              : <Lock className="h-4 w-4 text-muted-foreground/40" />
            }
          </Button>

          <Button size="icon" variant="ghost" onClick={() => navigate(`/connections/${conn.id}/edit`)}>
            <Pencil className="h-4 w-4" />
          </Button>

          <Button
            size="icon" variant="ghost"
            className="text-destructive hover:text-destructive"
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </CardFooter>
      </Card>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Delete connection"
        description={`Delete "${conn.name}"? This will also delete all associated audit jobs.`}
        onConfirm={() => deleteMutation.mutate()}
        loading={deleteMutation.isPending}
      />

      <ScheduleDialog
        open={scheduleOpen}
        onOpenChange={setScheduleOpen}
        connectionId={conn.id}
        connectionName={conn.name}
      />

      <HistoryDialog
        open={historyOpen}
        onOpenChange={setHistoryOpen}
        connectionId={conn.id}
        connectionName={conn.name}
      />
    </>
  )
}

function HistoryDialog({
  open, onOpenChange, connectionId, connectionName,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  connectionId: string
  connectionName: string
}) {
  const navigate = useNavigate()

  const { data: jobs = [] } = useQuery({
    queryKey: ['history', connectionId],
    queryFn: () => connectionHistoryApi.get(connectionId),
    enabled: open,
  })

  const doneJobs = jobs.filter((j) => j.status === 'done')

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>History — {connectionName}</DialogTitle>
        </DialogHeader>

        <div className="space-y-2 max-h-96 overflow-y-auto">
          {jobs.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">No audits yet.</p>
          ) : (
            jobs.map((j) => (
              <div
                key={j.id}
                className="flex items-center justify-between rounded-lg border p-3 cursor-pointer hover:bg-muted transition-colors"
                onClick={() => { navigate(`/jobs/${j.id}`); onOpenChange(false) }}
              >
                <div>
                  <p className="text-sm font-medium">{formatDate(j.started_at)}</p>
                  {j.status === 'done' && (
                    <p className="text-xs text-muted-foreground">
                      C:{j.findings_critical} H:{j.findings_high} M:{j.findings_medium} L:{j.findings_low}
                    </p>
                  )}
                </div>
                <StatusBadge status={j.status} />
              </div>
            ))
          )}
        </div>

        {doneJobs.length >= 2 && (
          <div className="border-t pt-3">
            <div className="grid grid-cols-4 gap-2 text-center text-xs">
              {(['findings_critical', 'findings_high', 'findings_medium', 'findings_low'] as const).map((key) => {
                const label = key.replace('findings_', '')
                const colorMap: Record<string, string> = {
                  critical: 'text-red-400',
                  high: 'text-orange-400',
                  medium: 'text-yellow-400',
                  low: 'text-blue-400',
                }
                const latest = doneJobs[0][key]
                const prev = doneJobs[1][key]
                const delta = latest - prev
                return (
                  <div key={key}>
                    <p className={`font-bold text-lg ${colorMap[label]}`}>{latest}</p>
                    <p className="text-muted-foreground capitalize">{label}</p>
                    {delta !== 0 && (
                      <p className={delta > 0 ? 'text-red-400' : 'text-green-400'}>
                        {delta > 0 ? '+' : ''}{delta}
                      </p>
                    )}
                  </div>
                )
              })}
            </div>
            <p className="text-xs text-muted-foreground text-center mt-2">vs. previous audit</p>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function ScheduleDialog({
  open, onOpenChange, connectionId, connectionName,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  connectionId: string
  connectionName: string
}) {
  const qc = useQueryClient()

  const { data: schedules = [] } = useQuery({
    queryKey: ['schedules'],
    queryFn: schedulesApi.list,
    enabled: open,
  })

  const existing = schedules.find((s) => s.connection_id === connectionId)

  const [localInterval, setLocalInterval] = useState('daily')
  const [localEnabled, setLocalEnabled] = useState(true)

  useEffect(() => {
    if (!open) return
    setLocalInterval(existing?.interval ?? 'daily')
    setLocalEnabled(existing?.enabled ?? true)
  }, [existing?.id, existing?.interval, existing?.enabled, open])

  const deleteMutation = useMutation({
    mutationFn: () => schedulesApi.delete(existing!.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['schedules'] })
      toast.success('Schedule removed')
      onOpenChange(false)
    },
    onError: () => toast.error('Failed to remove schedule'),
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Schedule — {connectionName}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {existing && (
            <div className="text-xs text-muted-foreground space-y-1 rounded-md bg-muted p-3">
              <p>Next run: {formatDate(existing.next_run_at)}</p>
              {existing.last_run_at && <p>Last run: {formatDate(existing.last_run_at)}</p>}
            </div>
          )}

          <div className="space-y-1.5">
            <Label>Frequency</Label>
            <Select value={localInterval} onValueChange={setLocalInterval}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="daily">Daily (every 24 hours)</SelectItem>
                <SelectItem value="weekly">Weekly (every 7 days)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex items-center justify-between">
            <Label htmlFor="sched-enabled">Enabled</Label>
            <Switch
              id="sched-enabled"
              checked={localEnabled}
              onCheckedChange={setLocalEnabled}
            />
          </div>
        </div>

        <DialogFooter className="gap-2">
          {existing && (
            <Button
              variant="ghost" size="sm"
              className="text-destructive hover:text-destructive mr-auto"
              onClick={() => deleteMutation.mutate()}
              disabled={deleteMutation.isPending}
            >
              Remove schedule
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              const onErr = (err: any) => {
                const code = err?.response?.data?.error
                if (code === 'feature_not_available') {
                  toast.error('Scheduled audits require a paid plan — upgrade in Settings → Team → License.')
                } else {
                  toast.error(err?.response?.data?.error ?? 'Failed to save schedule')
                }
              }
              if (existing) {
                schedulesApi.update(existing.id, { interval: localInterval, enabled: localEnabled })
                  .then(() => { qc.invalidateQueries({ queryKey: ['schedules'] }); toast.success('Schedule updated'); onOpenChange(false) })
                  .catch(onErr)
              } else {
                schedulesApi.create({ connection_id: connectionId, interval: localInterval, enabled: localEnabled })
                  .then(() => { qc.invalidateQueries({ queryKey: ['schedules'] }); toast.success('Schedule created'); onOpenChange(false) })
                  .catch(onErr)
              }
            }}
          >
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
