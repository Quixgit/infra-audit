import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  DndContext,
  DragEndEvent,
  DragOverEvent,
  DragOverlay,
  DragStartEvent,
  PointerSensor,
  useSensor,
  useSensors,
  closestCorners,
} from '@dnd-kit/core'
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  AlertTriangle, CheckCircle2, Clock, MessageSquare, Plus,
  Trash2, UserCircle2, RefreshCw, Download, ChevronDown,
  X, Send, ExternalLink, Sparkles, Terminal, Link,
} from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { remediationApi, type RemediationTask, type RemediationComment, type AISuggestion } from '@/lib/api'
import { formatDate } from '@/lib/utils'

// ── Constants ─────────────────────────────────────────────────────────────────

const LANES: { id: RemediationTask['lane']; label: string; color: string; headerColor: string }[] = [
  { id: 'immediate',  label: 'Immediate',   color: 'border-red-500/30',    headerColor: 'bg-red-500/10 text-red-400' },
  { id: 'this_week',  label: 'This Week',   color: 'border-orange-500/30', headerColor: 'bg-orange-500/10 text-orange-400' },
  { id: 'this_month', label: 'This Month',  color: 'border-yellow-500/30', headerColor: 'bg-yellow-500/10 text-yellow-400' },
  { id: 'backlog',    label: 'Backlog',     color: 'border-border',        headerColor: 'bg-muted text-muted-foreground' },
  { id: 'done',       label: 'Done',        color: 'border-green-500/30',  headerColor: 'bg-green-500/10 text-green-400' },
]

const SEV_COLOR: Record<string, string> = {
  critical: 'border-red-500/40 text-red-400 bg-red-500/10',
  high:     'border-orange-500/40 text-orange-400 bg-orange-500/10',
  medium:   'border-yellow-500/40 text-yellow-400 bg-yellow-500/10',
  low:      'border-blue-500/40 text-blue-400 bg-blue-500/10',
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function initials(email: string) {
  if (!email) return '?'
  const parts = email.split('@')[0].split(/[._-]/)
  return parts.map((p) => p[0]?.toUpperCase() ?? '').join('').slice(0, 2) || '?'
}

function isOverdue(dueDate?: string) {
  if (!dueDate) return false
  return new Date(dueDate) < new Date(new Date().toDateString())
}

function isDueThisWeek(dueDate?: string) {
  if (!dueDate) return false
  const d = new Date(dueDate)
  const now = new Date()
  const week = new Date(now)
  week.setDate(now.getDate() + 7)
  return d >= now && d <= week
}

// ── Task Card ─────────────────────────────────────────────────────────────────

function TaskCard({
  task,
  onClick,
  isDragging = false,
}: {
  task: RemediationTask
  onClick: () => void
  isDragging?: boolean
}) {
  const sev = task.severity?.toLowerCase() ?? 'medium'
  const overdue = isOverdue(task.due_date)

  return (
    <div
      onClick={onClick}
      className={`rounded-lg border bg-card p-3 cursor-pointer hover:border-indigo-500/40 transition-all space-y-2 ${
        isDragging ? 'opacity-50' : ''
      }`}
    >
      {/* Severity + title */}
      <div className="flex items-start gap-2">
        <Badge className={`${SEV_COLOR[sev] ?? SEV_COLOR.medium} border text-xs capitalize shrink-0 mt-0.5`}>
          {task.severity}
        </Badge>
        <p className="text-xs font-medium leading-snug line-clamp-2 flex-1">{task.title}</p>
      </div>

      {/* Connection + source badge */}
      <div className="flex items-center gap-2">
        {task.connection_name && (
          <p className="text-xs text-muted-foreground truncate flex-1">{task.connection_name}</p>
        )}
        {task.job_id && (
          <span className="text-[9px] px-1 py-0.5 rounded bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 shrink-0">
            from scan
          </span>
        )}
      </div>

      {/* Footer row */}
      <div className="flex items-center justify-between gap-2 pt-1 border-t border-border/40">
        {/* Assignee */}
        {task.assigned_email ? (
          <div className="flex items-center gap-1.5">
            <div className="h-5 w-5 rounded-full bg-indigo-500/20 text-indigo-400 flex items-center justify-center text-[10px] font-bold shrink-0">
              {initials(task.assigned_email)}
            </div>
            <span className="text-xs text-muted-foreground truncate max-w-[80px]">
              {task.assigned_email.split('@')[0]}
            </span>
          </div>
        ) : (
          <div className="flex items-center gap-1 text-muted-foreground/50">
            <UserCircle2 className="h-3.5 w-3.5" />
            <span className="text-xs">Unassigned</span>
          </div>
        )}

        <div className="flex items-center gap-2">
          {/* Due date */}
          {task.due_date && (
            <span className={`text-xs ${overdue ? 'text-red-400 font-medium' : 'text-muted-foreground'}`}>
              {overdue && '⚠ '}
              {new Date(task.due_date + 'T00:00:00').toLocaleDateString('en', { month: 'short', day: 'numeric' })}
            </span>
          )}

          {/* Comment count */}
          {task.comment_count > 0 && (
            <div className="flex items-center gap-0.5 text-muted-foreground">
              <MessageSquare className="h-3 w-3" />
              <span className="text-xs">{task.comment_count}</span>
            </div>
          )}
        </div>
      </div>

      {/* Verify status */}
      {task.verify_status === 'not_found' && (
        <div className="text-xs text-green-400 flex items-center gap-1">
          <CheckCircle2 className="h-3 w-3" /> Not found in re-scan
        </div>
      )}
      {task.verify_status === 'still_present' && (
        <div className="text-xs text-red-400 flex items-center gap-1">
          <AlertTriangle className="h-3 w-3" /> Still present
        </div>
      )}
    </div>
  )
}

// ── Sortable task card wrapper ─────────────────────────────────────────────────

function SortableTaskCard({ task, onClick }: { task: RemediationTask; onClick: () => void }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: task.id,
    data: { lane: task.lane },
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <TaskCard task={task} onClick={onClick} isDragging={isDragging} />
    </div>
  )
}

// ── Kanban Column ─────────────────────────────────────────────────────────────

function KanbanColumn({
  lane,
  tasks,
  onCardClick,
  onAddTask,
}: {
  lane: typeof LANES[number]
  tasks: RemediationTask[]
  onCardClick: (task: RemediationTask) => void
  onAddTask: (lane: RemediationTask['lane']) => void
}) {
  return (
    <div className={`flex flex-col rounded-xl border ${lane.color} bg-card/50 w-72 shrink-0`}>
      {/* Header */}
      <div className={`flex items-center justify-between px-4 py-3 rounded-t-xl ${lane.headerColor}`}>
        <span className="text-xs font-semibold uppercase tracking-wide">{lane.label}</span>
        <span className="text-xs font-bold px-1.5 py-0.5 rounded bg-black/10">{tasks.length}</span>
      </div>

      {/* Cards */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2 min-h-[120px]">
        <SortableContext items={tasks.map((t) => t.id)} strategy={verticalListSortingStrategy}>
          {tasks.map((task) => (
            <SortableTaskCard key={task.id} task={task} onClick={() => onCardClick(task)} />
          ))}
        </SortableContext>
        {tasks.length === 0 && (
          <p className="text-xs text-muted-foreground/40 text-center py-4">No tasks</p>
        )}
      </div>

      {/* Add button */}
      <div className="p-2 border-t border-border/30">
        <Button
          variant="ghost"
          size="sm"
          className="w-full h-7 text-xs text-muted-foreground hover:text-foreground justify-start"
          onClick={() => onAddTask(lane.id)}
        >
          <Plus className="h-3 w-3 mr-1" /> Add task
        </Button>
      </div>
    </div>
  )
}

// ── Task Detail Drawer ────────────────────────────────────────────────────────

function TaskDrawer({
  task,
  onClose,
  onUpdated,
}: {
  task: RemediationTask
  onClose: () => void
  onUpdated: () => void
}) {
  const qc = useQueryClient()
  const [commentBody, setCommentBody] = useState('')
  const [verifyState, setVerifyState] = useState<'idle' | 'running' | 'done'>('idle')
  const [verifyJobId, setVerifyJobId] = useState<string | null>(null)
  const [aiSuggestion, setAISuggestion] = useState<AISuggestion | null>(null)
  const [aiLoading, setAILoading] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const { data: comments = [] } = useQuery({
    queryKey: ['remediation-comments', task.id],
    queryFn: () => remediationApi.listComments(task.id),
  })

  const { data: teamData } = useQuery({
    queryKey: ['team'],
    queryFn: async () => {
      try {
        const { default: api } = await import('@/lib/api')
        const r = await api.get('/team')
        return r.data as { id: string; email: string }[]
      } catch { return [] }
    },
  })

  const addCommentMut = useMutation({
    mutationFn: (body: string) => remediationApi.addComment(task.id, body),
    onSuccess: () => {
      setCommentBody('')
      qc.invalidateQueries({ queryKey: ['remediation-comments', task.id] })
      qc.invalidateQueries({ queryKey: ['remediation-tasks'] })
    },
  })

  const updateMut = useMutation({
    mutationFn: (data: Parameters<typeof remediationApi.updateTask>[1]) =>
      remediationApi.updateTask(task.id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['remediation-tasks'] })
      onUpdated()
    },
  })

  const deleteMut = useMutation({
    mutationFn: () => remediationApi.deleteTask(task.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['remediation-tasks'] })
      onClose()
      toast.success('Task deleted')
    },
  })

  const startVerify = async () => {
    try {
      setVerifyState('running')
      const { verify_job_id } = await remediationApi.verify(task.id)
      setVerifyJobId(verify_job_id)
      // Poll verify result every 5s
      pollRef.current = setInterval(async () => {
        try {
          const { verify_status } = await remediationApi.verifyResult(task.id)
          if (verify_status !== 'pending') {
            clearInterval(pollRef.current!)
            setVerifyState('done')
            qc.invalidateQueries({ queryKey: ['remediation-tasks'] })
            onUpdated()
          }
        } catch { /* keep polling */ }
      }, 5000)
    } catch {
      setVerifyState('idle')
      toast.error('Failed to start verification scan')
    }
  }

  const sev = task.severity?.toLowerCase() ?? 'medium'

  return (
    <div className="fixed inset-0 z-50 flex">
      {/* Backdrop */}
      <div className="flex-1 bg-black/40" onClick={onClose} />
      {/* Drawer */}
      <div className="w-full max-w-lg bg-background border-l shadow-xl flex flex-col overflow-hidden">
        {/* Header */}
        <div className="flex items-start justify-between gap-3 p-5 border-b">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1.5">
              <Badge className={`${SEV_COLOR[sev] ?? SEV_COLOR.medium} border text-xs capitalize`}>
                {task.severity}
              </Badge>
              <span className="text-xs text-muted-foreground">{task.connection_name}</span>
            </div>
            <h2 className="font-semibold text-sm leading-snug">{task.title}</h2>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground shrink-0">
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Scrollable body */}
        <div className="flex-1 overflow-y-auto p-5 space-y-5">
          {/* Source scan badge */}
          {task.job_id && (
            <div className="flex items-center gap-2 text-xs rounded-lg bg-indigo-500/5 border border-indigo-500/20 px-3 py-2">
              <span className="text-indigo-400">🔗 Linked to scan finding</span>
              <span className="text-muted-foreground">—</span>
              <span className="text-muted-foreground">status syncs automatically with Findings page</span>
            </div>
          )}

          {/* Details */}
          {task.resource_name && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Resource</p>
              <p className="text-sm font-mono text-foreground/80">{task.resource_name}</p>
            </div>
          )}
          {task.description && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Description</p>
              <p className="text-sm text-foreground/80 leading-relaxed">{task.description}</p>
            </div>
          )}
          {task.remediation_text && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Remediation</p>
              <p className="text-sm text-foreground/80 leading-relaxed">{task.remediation_text}</p>
            </div>
          )}
          {task.risk_text && (
            <div>
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1">Risk</p>
              <p className="text-sm text-foreground/80 leading-relaxed">{task.risk_text}</p>
            </div>
          )}

          {/* AI Remediation Suggestion */}
          <div className="rounded-xl border border-indigo-500/20 bg-indigo-500/5 p-4 space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Sparkles className="h-4 w-4 text-indigo-400" />
                <span className="text-sm font-semibold text-indigo-300">AI Remediation Guide</span>
              </div>
              {!aiSuggestion && (
                <Button
                  size="sm"
                  variant="outline"
                  className="h-7 text-xs border-indigo-500/30 text-indigo-400 hover:bg-indigo-500/10"
                  onClick={async () => {
                    setAILoading(true)
                    try {
                      const s = await remediationApi.getAISuggestion(task.id)
                      setAISuggestion(s)
                    } catch {
                      toast.error('AI suggestion unavailable')
                    } finally {
                      setAILoading(false)
                    }
                  }}
                  disabled={aiLoading}
                >
                  {aiLoading ? <><RefreshCw className="h-3 w-3 mr-1 animate-spin" />Generating…</> : <><Sparkles className="h-3 w-3 mr-1" />Get AI guide</>}
                </Button>
              )}
              {aiSuggestion && (
                <Button
                  size="sm" variant="ghost"
                  className="h-7 text-xs text-muted-foreground"
                  onClick={() => setAISuggestion(null)}
                >
                  Refresh
                </Button>
              )}
            </div>

            {!aiSuggestion && !aiLoading && (
              <p className="text-xs text-muted-foreground/70">
                Click "Get AI guide" to generate step-by-step remediation instructions powered by Claude.
              </p>
            )}

            {aiSuggestion && !aiSuggestion.error && (
              <div className="space-y-3">
                <div className="flex items-center gap-3 text-xs">
                  <span className="rounded-full bg-indigo-500/15 px-2 py-0.5 text-indigo-300 font-medium capitalize">
                    {aiSuggestion.difficulty}
                  </span>
                  <span className="text-muted-foreground">⏱ {aiSuggestion.est_time}</span>
                </div>

                {aiSuggestion.explanation && (
                  <p className="text-xs text-foreground/80 leading-relaxed">{aiSuggestion.explanation}</p>
                )}

                {aiSuggestion.commands.length > 0 && (
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                      <Terminal className="h-3 w-3" />Commands
                    </div>
                    <div className="rounded-lg bg-black/40 border border-border/30 p-3 space-y-1">
                      {aiSuggestion.commands.map((cmd, i) => (
                        <div key={i} className="font-mono text-xs text-green-300 break-all">{cmd}</div>
                      ))}
                    </div>
                  </div>
                )}

                {aiSuggestion.doc_links.length > 0 && (
                  <div className="space-y-1">
                    <div className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                      <Link className="h-3 w-3" />Documentation
                    </div>
                    {aiSuggestion.doc_links.map((link, i) => (
                      <a
                        key={i}
                        href={link}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-1 text-xs text-indigo-400 hover:underline truncate"
                      >
                        <ExternalLink className="h-3 w-3 shrink-0" />{link}
                      </a>
                    ))}
                  </div>
                )}
              </div>
            )}

            {aiSuggestion?.error && (
              <p className="text-xs text-muted-foreground/70">
                {aiSuggestion.fallback || 'AI suggestion unavailable — check ANTHROPIC_API_KEY configuration.'}
              </p>
            )}
          </div>

          {/* Controls */}
          <div className="grid grid-cols-2 gap-3">
            {/* Assign to */}
            <div>
              <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1 block">
                Assign to
              </label>
              <select
                className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs text-foreground"
                value={task.assigned_to ?? ''}
                onChange={(e) => updateMut.mutate({ assigned_to: e.target.value || null })}
              >
                <option value="">Unassigned</option>
                {(teamData ?? []).map((m) => (
                  <option key={m.id} value={m.id}>{m.email}</option>
                ))}
              </select>
            </div>

            {/* Lane */}
            <div>
              <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1 block">
                Lane
              </label>
              <select
                className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs text-foreground"
                value={task.lane}
                onChange={(e) => updateMut.mutate({ lane: e.target.value })}
              >
                {LANES.map((l) => (
                  <option key={l.id} value={l.id}>{l.label}</option>
                ))}
              </select>
            </div>

            {/* Due date */}
            <div>
              <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1 block">
                Due date
              </label>
              <input
                type="date"
                className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs text-foreground"
                value={task.due_date ?? ''}
                onChange={(e) => updateMut.mutate({ due_date: e.target.value || null })}
              />
            </div>

            {/* Verify fix */}
            <div>
              <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-1 block">
                Verify Fix
              </label>
              <Button
                size="sm"
                variant="outline"
                className="w-full h-8 text-xs"
                onClick={startVerify}
                disabled={verifyState === 'running' || !task.connection_id}
              >
                {verifyState === 'running' ? (
                  <><RefreshCw className="h-3 w-3 mr-1 animate-spin" /> Re-scanning...</>
                ) : (
                  <><RefreshCw className="h-3 w-3 mr-1" /> Verify Fix</>
                )}
              </Button>
              {task.verify_status === 'not_found' && (
                <p className="text-xs text-green-400 mt-1">✓ Not found in last scan</p>
              )}
              {task.verify_status === 'still_present' && (
                <p className="text-xs text-red-400 mt-1">⚠ Still present in last scan</p>
              )}
              {verifyJobId && (
                <a
                  href={`/jobs/${verifyJobId}`}
                  className="text-xs text-indigo-400 hover:underline mt-1 flex items-center gap-1"
                >
                  <ExternalLink className="h-3 w-3" /> View scan job
                </a>
              )}
            </div>
          </div>

          {/* Comments */}
          <div>
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
              Comments ({comments.length})
            </p>
            <div className="space-y-3 mb-3">
              {comments.map((c) => (
                <div key={c.id} className="flex gap-2.5">
                  <div className="h-6 w-6 rounded-full bg-indigo-500/20 text-indigo-400 flex items-center justify-center text-[10px] font-bold shrink-0 mt-0.5">
                    {initials(c.user_email)}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span className="text-xs font-medium">{c.user_email.split('@')[0]}</span>
                      <span className="text-xs text-muted-foreground">{formatDate(c.created_at)}</span>
                    </div>
                    <p className="text-xs text-foreground/80 leading-relaxed whitespace-pre-wrap">{c.body}</p>
                  </div>
                </div>
              ))}
              {comments.length === 0 && (
                <p className="text-xs text-muted-foreground/50">No comments yet</p>
              )}
            </div>
            <div className="flex gap-2">
              <Input
                placeholder="Add a comment..."
                className="h-8 text-xs flex-1"
                value={commentBody}
                onChange={(e) => setCommentBody(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey && commentBody.trim()) {
                    e.preventDefault()
                    addCommentMut.mutate(commentBody.trim())
                  }
                }}
              />
              <Button
                size="icon"
                className="h-8 w-8"
                onClick={() => commentBody.trim() && addCommentMut.mutate(commentBody.trim())}
                disabled={!commentBody.trim() || addCommentMut.isPending}
              >
                <Send className="h-3 w-3" />
              </Button>
            </div>
          </div>
        </div>

        {/* Footer actions */}
        <div className="p-4 border-t flex items-center justify-between">
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive hover:text-destructive text-xs"
            onClick={() => deleteMut.mutate()}
            disabled={deleteMut.isPending}
          >
            <Trash2 className="h-3.5 w-3.5 mr-1" /> Delete task
          </Button>
          <span className="text-xs text-muted-foreground">
            Created {formatDate(task.created_at)}
          </span>
        </div>
      </div>
    </div>
  )
}

// ── Add Task Dialog ───────────────────────────────────────────────────────────

function AddTaskDialog({
  defaultLane,
  onClose,
}: {
  defaultLane: RemediationTask['lane']
  onClose: () => void
}) {
  const qc = useQueryClient()
  const [title, setTitle] = useState('')
  const [severity, setSeverity] = useState('medium')
  const [lane, setLane] = useState<RemediationTask['lane']>(defaultLane)
  const [connectionName, setConnectionName] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [remediation, setRemediation] = useState('')

  const createMut = useMutation({
    mutationFn: () =>
      remediationApi.createTask({
        title,
        severity,
        lane,
        connection_name: connectionName,
        due_date: dueDate || undefined,
        remediation_text: remediation,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['remediation-tasks'] })
      onClose()
      toast.success('Task created')
    },
    onError: () => toast.error('Failed to create task'),
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-background border rounded-xl shadow-2xl w-full max-w-md p-6 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">Add Remediation Task</h3>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground mb-1 block">Title *</label>
            <Input
              placeholder="Describe what needs to be fixed"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="h-8 text-sm"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-muted-foreground mb-1 block">Severity</label>
              <select
                className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs text-foreground"
                value={severity}
                onChange={(e) => setSeverity(e.target.value)}
              >
                <option value="critical">Critical</option>
                <option value="high">High</option>
                <option value="medium">Medium</option>
                <option value="low">Low</option>
              </select>
            </div>
            <div>
              <label className="text-xs text-muted-foreground mb-1 block">Lane</label>
              <select
                className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs text-foreground"
                value={lane}
                onChange={(e) => setLane(e.target.value as RemediationTask['lane'])}
              >
                {LANES.map((l) => (
                  <option key={l.id} value={l.id}>{l.label}</option>
                ))}
              </select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-muted-foreground mb-1 block">Connection</label>
              <Input
                placeholder="e.g. Production"
                value={connectionName}
                onChange={(e) => setConnectionName(e.target.value)}
                className="h-8 text-xs"
              />
            </div>
            <div>
              <label className="text-xs text-muted-foreground mb-1 block">Due date</label>
              <input
                type="date"
                className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs text-foreground"
                value={dueDate}
                onChange={(e) => setDueDate(e.target.value)}
              />
            </div>
          </div>
          <div>
            <label className="text-xs text-muted-foreground mb-1 block">Remediation notes</label>
            <textarea
              placeholder="Steps to remediate..."
              rows={3}
              className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-xs text-foreground resize-none"
              value={remediation}
              onChange={(e) => setRemediation(e.target.value)}
            />
          </div>
        </div>

        <div className="flex gap-2 justify-end">
          <Button variant="outline" size="sm" onClick={onClose}>Cancel</Button>
          <Button
            size="sm"
            onClick={() => createMut.mutate()}
            disabled={!title.trim() || createMut.isPending}
          >
            Create task
          </Button>
        </div>
      </div>
    </div>
  )
}

// ── Export dialog ─────────────────────────────────────────────────────────────

function ExportDialog({ tasks, onClose }: { tasks: RemediationTask[]; onClose: () => void }) {
  const [exportType, setExportType] = useState<'csv' | 'jira' | 'github' | 'linear'>('csv')
  const [fields, setFields] = useState({ url: '', token: '', project: '', repo: '', team: '' })

  const exportCSV = () => {
    const header = 'Title,Severity,Lane,Connection,Assignee,Due Date,Verify Status,Created\n'
    const rows = tasks.map((t) =>
      [
        `"${t.title.replace(/"/g, '""')}"`,
        t.severity,
        t.lane,
        t.connection_name,
        t.assigned_email || 'Unassigned',
        t.due_date || '',
        t.verify_status || 'none',
        t.created_at.slice(0, 10),
      ].join(',')
    ).join('\n')
    const blob = new Blob([header + rows], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'remediation-tasks.csv'
    a.click()
    URL.revokeObjectURL(url)
    onClose()
  }

  const exportToService = async () => {
    if (exportType === 'jira') {
      let failed = 0
      for (const task of tasks.filter((t) => t.lane !== 'done')) {
        try {
          await fetch(`${fields.url}/rest/api/3/issue`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Basic ${btoa(`${fields.token}`)}`,
            },
            body: JSON.stringify({
              fields: {
                project: { key: fields.project },
                summary: task.title,
                description: {
                  type: 'doc', version: 1,
                  content: [{ type: 'paragraph', content: [{ type: 'text', text: task.remediation_text || task.description || '' }] }],
                },
                issuetype: { name: 'Bug' },
                priority: { name: task.severity === 'critical' ? 'Highest' : task.severity === 'high' ? 'High' : task.severity === 'medium' ? 'Medium' : 'Low' },
              },
            }),
          })
        } catch { failed++ }
      }
      toast.success(`Exported ${tasks.length - failed} tasks to Jira`)
      onClose()
    } else if (exportType === 'github') {
      let ok = 0
      for (const task of tasks.filter((t) => t.lane !== 'done')) {
        try {
          const r = await fetch(`https://api.github.com/repos/${fields.repo}/issues`, {
            method: 'POST',
            headers: { Authorization: `token ${fields.token}`, 'Content-Type': 'application/json' },
            body: JSON.stringify({
              title: `[${task.severity.toUpperCase()}] ${task.title}`,
              body: `**Connection:** ${task.connection_name}\n**Severity:** ${task.severity}\n\n${task.remediation_text || task.description || ''}`,
              labels: [task.severity, 'security'],
            }),
          })
          if (r.ok) ok++
        } catch { /* skip */ }
      }
      toast.success(`Created ${ok} GitHub issues`)
      onClose()
    } else if (exportType === 'linear') {
      toast.info('Linear export: Use the Linear API key with the Linear SDK or import CSV from the exported file.')
      onClose()
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-background border rounded-xl shadow-2xl w-full max-w-sm p-6 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">Export Tasks</h3>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground"><X className="h-4 w-4" /></button>
        </div>

        <div className="grid grid-cols-2 gap-2">
          {(['csv', 'jira', 'github', 'linear'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setExportType(t)}
              className={`px-3 py-2 rounded-lg border text-xs font-medium transition-colors ${exportType === t ? 'border-indigo-500/50 bg-indigo-500/10 text-indigo-400' : 'border-border text-muted-foreground hover:border-border/80'}`}
            >
              {t === 'csv' ? 'Export CSV' : t === 'jira' ? 'Jira' : t === 'github' ? 'GitHub Issues' : 'Linear'}
            </button>
          ))}
        </div>

        {exportType === 'jira' && (
          <div className="space-y-2">
            <Input placeholder="Jira URL (e.g. https://company.atlassian.net)" value={fields.url} onChange={(e) => setFields({ ...fields, url: e.target.value })} className="h-8 text-xs" />
            <Input placeholder="email:api_token (base64 encoded)" value={fields.token} onChange={(e) => setFields({ ...fields, token: e.target.value })} className="h-8 text-xs" />
            <Input placeholder="Project key (e.g. SEC)" value={fields.project} onChange={(e) => setFields({ ...fields, project: e.target.value })} className="h-8 text-xs" />
          </div>
        )}
        {exportType === 'github' && (
          <div className="space-y-2">
            <Input placeholder="GitHub token (ghp_...)" value={fields.token} onChange={(e) => setFields({ ...fields, token: e.target.value })} className="h-8 text-xs" />
            <Input placeholder="owner/repo" value={fields.repo} onChange={(e) => setFields({ ...fields, repo: e.target.value })} className="h-8 text-xs" />
          </div>
        )}
        {exportType === 'linear' && (
          <div className="space-y-2">
            <Input placeholder="Linear API key" value={fields.token} onChange={(e) => setFields({ ...fields, token: e.target.value })} className="h-8 text-xs" />
            <Input placeholder="Team ID or name" value={fields.team} onChange={(e) => setFields({ ...fields, team: e.target.value })} className="h-8 text-xs" />
            <p className="text-xs text-muted-foreground">Note: Export to CSV then import via Linear's CSV importer for best results.</p>
          </div>
        )}

        <div className="flex gap-2 justify-end">
          <Button variant="outline" size="sm" onClick={onClose}>Cancel</Button>
          <Button size="sm" onClick={exportType === 'csv' ? exportCSV : exportToService}>
            Export
          </Button>
        </div>
      </div>
    </div>
  )
}

// ── Main Page ─────────────────────────────────────────────────────────────────

export function Remediation() {
  const qc = useQueryClient()
  const [activeTask, setActiveTask] = useState<RemediationTask | null>(null)
  const [draggingTask, setDraggingTask] = useState<RemediationTask | null>(null)
  const [addLane, setAddLane] = useState<RemediationTask['lane'] | null>(null)
  const [showExport, setShowExport] = useState(false)
  const [selectedTask, setSelectedTask] = useState<RemediationTask | null>(null)

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } })
  )

  const { data: tasks = [], isLoading } = useQuery({
    queryKey: ['remediation-tasks'],
    queryFn: remediationApi.listTasks,
    refetchInterval: 30_000,
  })

  const updateMut = useMutation({
    mutationFn: ({ id, lane }: { id: string; lane: string }) =>
      remediationApi.updateTask(id, { lane }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['remediation-tasks'] }),
  })

  // Stats
  const now = new Date()
  const openTasks = tasks.filter((t) => t.lane !== 'done')
  const overdueTasks = openTasks.filter((t) => t.due_date && new Date(t.due_date) < now)
  const dueThisWeek = openTasks.filter((t) => isDueThisWeek(t.due_date))
  const doneTasks = tasks.filter((t) => t.lane === 'done')

  const tasksByLane = (lane: RemediationTask['lane']) => tasks.filter((t) => t.lane === lane)

  const handleDragStart = (event: DragStartEvent) => {
    const task = tasks.find((t) => t.id === event.active.id)
    if (task) setDraggingTask(task)
  }

  const handleDragEnd = (event: DragEndEvent) => {
    setDraggingTask(null)
    const { active, over } = event
    if (!over || active.id === over.id) return

    // Determine target lane from the over item's data or column id
    const overTask = tasks.find((t) => t.id === over.id)
    const overLane = (overTask?.lane ?? over.id) as RemediationTask['lane']

    const draggedTask = tasks.find((t) => t.id === active.id)
    if (draggedTask && draggedTask.lane !== overLane && validLanes.has(overLane)) {
      updateMut.mutate({ id: draggedTask.id, lane: overLane })
    }
  }

  const handleDragOver = (event: DragOverEvent) => {
    const { over } = event
    if (!over) return
    // Could optimize by using this for visual feedback only
  }

  const validLanes = new Set(LANES.map((l) => l.id))

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">Loading...</div>
    )
  }

  return (
    <div className="space-y-5 h-full flex flex-col">
      {/* Header */}
      <div className="flex items-start justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold">Remediation</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Tasks auto-appear here when you mark a finding <span className="font-medium text-foreground">In Progress</span> on the{' '}
            <a href="/findings" className="text-indigo-400 hover:underline">Findings page</a>.
            Moving a card to <span className="font-medium text-green-400">Done</span> automatically marks the finding as Fixed — and vice versa.
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            onClick={() => setShowExport(true)}
          >
            <Download className="h-3.5 w-3.5" />
            Export
            <ChevronDown className="h-3 w-3" />
          </Button>
          <Button size="sm" className="gap-1.5" onClick={() => setAddLane('backlog')}>
            <Plus className="h-3.5 w-3.5" /> Add task
          </Button>
        </div>
      </div>

      {/* Stats bar */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {[
          { label: 'Open',          value: openTasks.length,     icon: AlertTriangle, cls: '' },
          { label: 'Overdue',       value: overdueTasks.length,  icon: AlertTriangle, cls: overdueTasks.length  > 0 ? 'text-red-400'    : 'text-muted-foreground' },
          { label: 'Due This Week', value: dueThisWeek.length,   icon: Clock,         cls: dueThisWeek.length   > 0 ? 'text-yellow-400' : 'text-muted-foreground' },
          { label: 'Done',          value: doneTasks.length,     icon: CheckCircle2,  cls: 'text-green-400' },
        ].map(({ label, value, icon: Icon, cls }) => (
          <Card key={label}>
            <CardContent className="pt-4">
              <div className="flex items-center gap-2 text-muted-foreground text-xs mb-1">
                <Icon className={`h-4 w-4 ${cls}`} />
                {label}
              </div>
              <p className={`text-2xl font-bold ${cls}`}>{value}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Kanban board */}
      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
        <div className="flex gap-4 overflow-x-auto pb-4 flex-1">
          {LANES.map((lane) => (
            <KanbanColumn
              key={lane.id}
              lane={lane}
              tasks={tasksByLane(lane.id)}
              onCardClick={(task) => setSelectedTask(task)}
              onAddTask={(l) => setAddLane(l)}
            />
          ))}
        </div>

        <DragOverlay>
          {draggingTask && (
            <TaskCard task={draggingTask} onClick={() => {}} isDragging />
          )}
        </DragOverlay>
      </DndContext>

      {/* Modals */}
      {selectedTask && (
        <TaskDrawer
          task={tasks.find((t) => t.id === selectedTask.id) ?? selectedTask}
          onClose={() => setSelectedTask(null)}
          onUpdated={() => {
            qc.invalidateQueries({ queryKey: ['remediation-tasks'] })
          }}
        />
      )}

      {addLane && (
        <AddTaskDialog defaultLane={addLane} onClose={() => setAddLane(null)} />
      )}

      {showExport && (
        <ExportDialog tasks={tasks} onClose={() => setShowExport(false)} />
      )}
    </div>
  )
}
