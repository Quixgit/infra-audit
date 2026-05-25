import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, CheckCircle2, XCircle, AlertCircle, Clock, UserPlus, Download, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { accessReviewsApi, connectionsApi, type AccessReviewItem, type AccessReviewDecision } from '@/lib/api'
import { formatDate } from '@/lib/utils'

// ── Decision badge ────────────────────────────────────────────────────────────

function DecisionBadge({ decision }: { decision: AccessReviewDecision }) {
  const map: Record<AccessReviewDecision, { label: string; icon: React.ElementType; cls: string }> = {
    pending:        { label: 'Pending',         icon: Clock,         cls: 'text-muted-foreground' },
    approved:       { label: 'Approved',        icon: CheckCircle2,  cls: 'text-green-400' },
    revoked:        { label: 'Revoked',         icon: XCircle,       cls: 'text-red-400' },
    needs_followup: { label: 'Needs Follow-up', icon: AlertCircle,   cls: 'text-yellow-400' },
  }
  const { label, icon: Icon, cls } = map[decision] ?? map.pending
  return (
    <span className={`flex items-center gap-1 text-xs font-medium ${cls}`}>
      <Icon className="h-3.5 w-3.5" />
      {label}
    </span>
  )
}

// ── Decision select ───────────────────────────────────────────────────────────

function DecisionSelect({
  item,
  reviewId,
}: {
  item: AccessReviewItem
  reviewId: string
}) {
  const qc = useQueryClient()
  const [notes, setNotes] = useState(item.notes)
  const [editNotes, setEditNotes] = useState(false)

  const updateMutation = useMutation({
    mutationFn: (decision: AccessReviewDecision) =>
      accessReviewsApi.updateItem(reviewId, item.id, { decision, notes }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['review-items', reviewId] })
      qc.invalidateQueries({ queryKey: ['access-review', reviewId] })
    },
    onError: () => toast.error('Update failed'),
  })

  const saveNotesMutation = useMutation({
    mutationFn: () =>
      accessReviewsApi.updateItem(reviewId, item.id, { decision: item.decision, notes }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['review-items', reviewId] })
      setEditNotes(false)
      toast.success('Notes saved')
    },
    onError: () => toast.error('Save failed'),
  })

  return (
    <div className="space-y-1">
      <select
        value={item.decision}
        onChange={(e) => updateMutation.mutate(e.target.value as AccessReviewDecision)}
        disabled={updateMutation.isPending}
        className={`rounded border px-2 py-1 text-xs font-medium bg-background focus:outline-none focus:ring-1 focus:ring-ring transition-colors ${
          item.decision === 'approved' ? 'border-green-500/40 text-green-400' :
          item.decision === 'revoked' ? 'border-red-500/40 text-red-400' :
          item.decision === 'needs_followup' ? 'border-yellow-500/40 text-yellow-400' :
          'border-border text-muted-foreground'
        }`}
      >
        <option value="pending">Pending</option>
        <option value="approved">Approved</option>
        <option value="revoked">Revoked</option>
        <option value="needs_followup">Needs Follow-up</option>
      </select>

      {editNotes ? (
        <div className="flex items-center gap-1">
          <input
            className="text-xs rounded border bg-background px-2 py-0.5 focus:outline-none focus:ring-1 focus:ring-ring w-40"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Add note…"
            onKeyDown={(e) => e.key === 'Enter' && saveNotesMutation.mutate()}
          />
          <button
            onClick={() => saveNotesMutation.mutate()}
            className="text-xs text-indigo-400 hover:underline"
          >
            Save
          </button>
          <button
            onClick={() => { setNotes(item.notes); setEditNotes(false) }}
            className="text-xs text-muted-foreground hover:underline"
          >
            Cancel
          </button>
        </div>
      ) : (
        <button
          onClick={() => setEditNotes(true)}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {notes ? notes : '+ note'}
        </button>
      )}
    </div>
  )
}

// ── Add item dialog ───────────────────────────────────────────────────────────

function AddItemDialog({ reviewId, onClose }: { reviewId: string; onClose: () => void }) {
  const qc = useQueryClient()
  const [form, setForm] = useState({ subject_name: '', subject_email: '', subject_role: '', access_level: '' })

  const createMutation = useMutation({
    mutationFn: () => accessReviewsApi.addItem(reviewId, form),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['review-items', reviewId] })
      qc.invalidateQueries({ queryKey: ['access-review', reviewId] })
      toast.success('Member added')
      onClose()
    },
    onError: () => toast.error('Add failed'),
  })

  const set = (k: string, v: string) => setForm((f) => ({ ...f, [k]: v }))

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-xl border bg-card shadow-xl p-6 space-y-4">
        <h2 className="text-lg font-semibold">Add Member</h2>
        <div className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground font-medium">Full Name *</label>
            <input
              className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              value={form.subject_name}
              onChange={(e) => set('subject_name', e.target.value)}
              placeholder="Jane Smith"
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground font-medium">Email</label>
            <input
              className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              value={form.subject_email}
              onChange={(e) => set('subject_email', e.target.value)}
              placeholder="jane@example.com"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-muted-foreground font-medium">Role</label>
              <input
                className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={form.subject_role}
                onChange={(e) => set('subject_role', e.target.value)}
                placeholder="e.g. admin"
              />
            </div>
            <div>
              <label className="text-xs text-muted-foreground font-medium">Access Level</label>
              <input
                className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={form.access_level}
                onChange={(e) => set('access_level', e.target.value)}
                placeholder="e.g. Full Access"
              />
            </div>
          </div>
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <button onClick={onClose} className="rounded-md border px-4 py-2 text-sm hover:bg-muted transition-colors">
            Cancel
          </button>
          <button
            disabled={!form.subject_name || createMutation.isPending}
            onClick={() => createMutation.mutate()}
            className="rounded-md bg-indigo-500 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-600 transition-colors disabled:opacity-50"
          >
            {createMutation.isPending ? 'Adding…' : 'Add Member'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ── CSV export ────────────────────────────────────────────────────────────────

function exportCSV(reviewName: string, items: AccessReviewItem[]) {
  const header = ['Name', 'Email', 'Role', 'Access Level', 'Decision', 'Decided By', 'Decided At', 'Notes']
  const rows = items.map((it) => [
    it.subject_name,
    it.subject_email,
    it.subject_role,
    it.access_level,
    it.decision,
    it.decided_by_email ?? '',
    it.decided_at ? new Date(it.decided_at).toLocaleDateString() : '',
    it.notes,
  ])
  const csv = [header, ...rows].map((r) => r.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `access-review-${reviewName.replace(/\s+/g, '-').toLowerCase()}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

// ── Main page ─────────────────────────────────────────────────────────────────

export function AccessReviewDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [showAddItem, setShowAddItem] = useState(false)
  const [selectedDecision, setSelectedDecision] = useState<string>('')
  const [bulkDecision, setBulkDecision] = useState<AccessReviewDecision>('approved')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const { data: review, isLoading: reviewLoading } = useQuery({
    queryKey: ['access-review', id],
    queryFn: () => accessReviewsApi.list().then((arr) => arr.find((r) => r.id === id)),
    enabled: !!id,
  })

  const { data: items = [], isLoading: itemsLoading } = useQuery({
    queryKey: ['review-items', id],
    queryFn: () => accessReviewsApi.listItems(id!),
    enabled: !!id,
  })

  const { data: connections = [] } = useQuery({
    queryKey: ['connections'],
    queryFn: connectionsApi.list,
  })
  const doConnections = connections.filter((c) => c.conn_type === 'do')

  const completeMutation = useMutation({
    mutationFn: () => accessReviewsApi.complete(id!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['access-review', id] })
      qc.invalidateQueries({ queryKey: ['access-reviews'] })
      qc.invalidateQueries({ queryKey: ['access-reviews-stats'] })
      toast.success('Review completed — evidence auto-linked to SOC 2 & ISO 27001')
    },
    onError: () => toast.error('Complete failed'),
  })

  const importDOMutation = useMutation({
    mutationFn: (connectionId: string) => accessReviewsApi.importDO(id!, connectionId),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ['review-items', id] })
      qc.invalidateQueries({ queryKey: ['access-review', id] })
      toast.success(`Imported ${res.imported} member(s) from DigitalOcean`)
    },
    onError: (e: unknown) => {
      const msg = e instanceof Error ? e.message : 'Import failed'
      toast.error(msg)
    },
  })

  const bulkApproveMutation = useMutation({
    mutationFn: async () => {
      const pending = items.filter((it) => it.decision === 'pending')
      for (const it of pending) {
        await accessReviewsApi.updateItem(id!, it.id, { decision: 'approved' })
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['review-items', id] })
      qc.invalidateQueries({ queryKey: ['access-review', id] })
      toast.success('All pending items approved')
    },
    onError: () => toast.error('Bulk approve failed'),
  })

  const bulkDecisionMutation = useMutation({
    mutationFn: async () => {
      for (const itemId of Array.from(selectedIds)) {
        await accessReviewsApi.updateItem(id!, itemId, { decision: bulkDecision })
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['review-items', id] })
      qc.invalidateQueries({ queryKey: ['access-review', id] })
      setSelectedIds(new Set())
      toast.success(`Applied "${bulkDecision}" to ${selectedIds.size} item(s)`)
    },
    onError: () => toast.error('Bulk decision failed'),
  })

  if (reviewLoading) {
    return <div className="flex h-64 items-center justify-center text-muted-foreground">Loading…</div>
  }
  if (!review) {
    return (
      <div className="flex flex-col items-center gap-4 py-24 text-center">
        <p className="text-muted-foreground">Review not found.</p>
        <button onClick={() => navigate('/access-reviews')} className="text-sm underline text-muted-foreground hover:text-foreground">
          Back to Access Reviews
        </button>
      </div>
    )
  }

  const pct = review.item_count > 0 ? Math.round((review.reviewed_count / review.item_count) * 100) : 0
  const pendingItems = items.filter((it) => it.decision === 'pending')
  const filteredItems = selectedDecision ? items.filter((it) => it.decision === selectedDecision) : items

  const toggleSelect = (itemId: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(itemId)) next.delete(itemId)
      else next.add(itemId)
      return next
    })
  }

  const toggleAll = () => {
    if (selectedIds.size === filteredItems.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(filteredItems.map((it) => it.id)))
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <button
            onClick={() => navigate('/access-reviews')}
            className="mt-1 p-1.5 rounded hover:bg-muted transition-colors text-muted-foreground"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div>
            <h1 className="text-2xl font-bold">{review.name}</h1>
            {review.description && (
              <p className="text-sm text-muted-foreground mt-0.5">{review.description}</p>
            )}
            <div className="flex items-center gap-3 mt-2 text-xs text-muted-foreground">
              <span>Type: <span className="text-foreground capitalize">{review.review_type.replace('_', ' ')}</span></span>
              {review.due_date && (
                <span>Due: <span className={new Date(review.due_date) < new Date() && review.status !== 'completed' ? 'text-red-400 font-medium' : 'text-foreground'}>{review.due_date}</span></span>
              )}
              <span>Status: <span className={
                review.status === 'completed' ? 'text-green-400' :
                review.status === 'overdue' ? 'text-red-400' : 'text-blue-400'
              } style={{ textTransform: 'capitalize' }}>{review.status.replace('_', ' ')}</span></span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          {review.status !== 'completed' && doConnections.length > 0 && (
            <select
              className="rounded-md border bg-background px-2 py-1.5 text-xs focus:outline-none focus:ring-1 focus:ring-ring"
              defaultValue=""
              onChange={(e) => {
                if (e.target.value) {
                  importDOMutation.mutate(e.target.value)
                  e.target.value = ''
                }
              }}
            >
              <option value="" disabled>Import from DO…</option>
              {doConnections.map((c) => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          )}

          {review.status !== 'completed' && (
            <>
              <button
                onClick={() => setShowAddItem(true)}
                className="flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-xs font-medium hover:bg-muted transition-colors"
              >
                <UserPlus className="h-3.5 w-3.5" />
                Add Member
              </button>
              {pendingItems.length > 0 && (
                <button
                  disabled={bulkApproveMutation.isPending}
                  onClick={() => bulkApproveMutation.mutate()}
                  className="flex items-center gap-1.5 rounded-md border border-green-500/40 px-3 py-1.5 text-xs font-medium text-green-400 hover:bg-green-500/10 transition-colors disabled:opacity-50"
                >
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  Approve All Pending
                </button>
              )}
              <button
                disabled={completeMutation.isPending || review.item_count === 0}
                onClick={() => {
                  if (pendingItems.length > 0) {
                    if (!confirm(`${pendingItems.length} item(s) still pending. Mark review as complete anyway?`)) return
                  }
                  completeMutation.mutate()
                }}
                className="flex items-center gap-1.5 rounded-md bg-green-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-green-700 transition-colors disabled:opacity-50"
              >
                <CheckCircle2 className="h-3.5 w-3.5" />
                {completeMutation.isPending ? 'Completing…' : 'Complete Review'}
              </button>
            </>
          )}

          <button
            disabled={items.length === 0}
            onClick={() => exportCSV(review.name, items)}
            className="flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-xs font-medium hover:bg-muted transition-colors disabled:opacity-50"
          >
            <Download className="h-3.5 w-3.5" />
            Export CSV
          </button>
        </div>
      </div>

      {/* Progress bar */}
      <Card>
        <CardContent className="pt-4">
          <div className="flex items-center justify-between text-sm mb-2">
            <span className="text-muted-foreground">{review.reviewed_count} / {review.item_count} reviewed</span>
            <span className="font-bold">{pct}%</span>
          </div>
          <div className="h-2 rounded-full bg-muted overflow-hidden">
            <div
              className={`h-2 rounded-full transition-all ${pct === 100 ? 'bg-green-500' : 'bg-indigo-500'}`}
              style={{ width: `${pct}%` }}
            />
          </div>
          <div className="flex items-center gap-4 mt-2 text-xs text-muted-foreground">
            {(['pending', 'approved', 'revoked', 'needs_followup'] as const).map((d) => {
              const count = items.filter((it) => it.decision === d).length
              if (count === 0) return null
              return (
                <span key={d}>
                  <span className={
                    d === 'approved' ? 'text-green-400' :
                    d === 'revoked' ? 'text-red-400' :
                    d === 'needs_followup' ? 'text-yellow-400' : 'text-muted-foreground'
                  }>{count}</span> {d.replace('_', ' ')}
                </span>
              )
            })}
          </div>
        </CardContent>
      </Card>

      {/* Items table */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">Members ({items.length})</CardTitle>
            <div className="flex items-center gap-2">
              {/* Filter */}
              <select
                className="rounded-md border bg-background px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-ring"
                value={selectedDecision}
                onChange={(e) => setSelectedDecision(e.target.value)}
              >
                <option value="">All decisions</option>
                <option value="pending">Pending</option>
                <option value="approved">Approved</option>
                <option value="revoked">Revoked</option>
                <option value="needs_followup">Needs Follow-up</option>
              </select>

              {/* Bulk action */}
              {selectedIds.size > 0 && (
                <div className="flex items-center gap-1.5">
                  <span className="text-xs text-muted-foreground">{selectedIds.size} selected</span>
                  <select
                    className="rounded-md border bg-background px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-ring"
                    value={bulkDecision}
                    onChange={(e) => setBulkDecision(e.target.value as AccessReviewDecision)}
                  >
                    <option value="approved">Approved</option>
                    <option value="revoked">Revoked</option>
                    <option value="needs_followup">Needs Follow-up</option>
                    <option value="pending">Pending</option>
                  </select>
                  <button
                    disabled={bulkDecisionMutation.isPending}
                    onClick={() => bulkDecisionMutation.mutate()}
                    className="rounded-md bg-indigo-500 px-2.5 py-1 text-xs font-semibold text-white hover:bg-indigo-600 transition-colors disabled:opacity-50 flex items-center gap-1"
                  >
                    <RefreshCw className="h-3 w-3" />
                    Apply
                  </button>
                </div>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {itemsLoading ? (
            <p className="text-center text-muted-foreground text-sm py-8">Loading…</p>
          ) : filteredItems.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <p className="text-sm text-muted-foreground">
                {items.length === 0
                  ? 'No members added yet. Add members manually or import from a connection.'
                  : 'No items match the current filter.'}
              </p>
              {items.length === 0 && review.status !== 'completed' && (
                <button
                  onClick={() => setShowAddItem(true)}
                  className="rounded-md bg-indigo-500 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-600 transition-colors"
                >
                  Add First Member
                </button>
              )}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="pb-2 pr-2 w-8">
                      <input
                        type="checkbox"
                        checked={selectedIds.size === filteredItems.length && filteredItems.length > 0}
                        onChange={toggleAll}
                        className="rounded"
                      />
                    </th>
                    <th className="pb-2 font-medium pr-4">Name</th>
                    <th className="pb-2 font-medium pr-4">Email</th>
                    <th className="pb-2 font-medium pr-4">Role</th>
                    <th className="pb-2 font-medium pr-4">Access Level</th>
                    <th className="pb-2 font-medium pr-4">Last Active</th>
                    <th className="pb-2 font-medium">Decision / Notes</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {filteredItems.map((it) => (
                    <tr key={it.id} className={`${selectedIds.has(it.id) ? 'bg-muted/30' : 'hover:bg-muted/20'} transition-colors`}>
                      <td className="py-3 pr-2">
                        <input
                          type="checkbox"
                          checked={selectedIds.has(it.id)}
                          onChange={() => toggleSelect(it.id)}
                          className="rounded"
                        />
                      </td>
                      <td className="py-3 pr-4">
                        <p className="font-medium">{it.subject_name || '—'}</p>
                      </td>
                      <td className="py-3 pr-4 text-muted-foreground text-xs">{it.subject_email || '—'}</td>
                      <td className="py-3 pr-4 text-muted-foreground">{it.subject_role || '—'}</td>
                      <td className="py-3 pr-4 text-muted-foreground">{it.access_level || '—'}</td>
                      <td className="py-3 pr-4 text-muted-foreground text-xs">
                        {it.last_active_at ? formatDate(it.last_active_at) : '—'}
                      </td>
                      <td className="py-3">
                        {review.status !== 'completed' ? (
                          <DecisionSelect item={it} reviewId={review.id} />
                        ) : (
                          <DecisionBadge decision={it.decision} />
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {showAddItem && id && <AddItemDialog reviewId={id} onClose={() => setShowAddItem(false)} />}
    </div>
  )
}
