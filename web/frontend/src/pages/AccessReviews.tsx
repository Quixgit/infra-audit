import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Plus, Users, Clock, CheckCircle2, AlertTriangle, CalendarClock, Trash2, ExternalLink } from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { accessReviewsApi, connectionsApi, type AccessReview } from '@/lib/api'
import { formatDate } from '@/lib/utils'

// ── Status badge ──────────────────────────────────────────────────────────────

function ReviewStatusBadge({ status }: { status: AccessReview['status'] }) {
  const map: Record<string, { label: string; cls: string }> = {
    in_progress: { label: 'In Progress', cls: 'bg-blue-500/15 text-blue-400 border-blue-500/30' },
    completed:   { label: 'Completed',   cls: 'bg-green-500/15 text-green-400 border-green-500/30' },
    overdue:     { label: 'Overdue',     cls: 'bg-red-500/15 text-red-400 border-red-500/30' },
  }
  const { label, cls } = map[status] ?? { label: status, cls: 'bg-muted text-muted-foreground border-border' }
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium ${cls}`}>
      {label}
    </span>
  )
}

// ── Create dialog ─────────────────────────────────────────────────────────────

function CreateReviewDialog({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient()
  const { data: connections = [] } = useQuery({
    queryKey: ['connections'],
    queryFn: connectionsApi.list,
  })
  const doConnections = connections.filter((c) => c.conn_type === 'do')

  const [form, setForm] = useState({
    name: '',
    description: '',
    review_type: 'manual',
    connection_id: '',
    due_date: '',
    github_org: '',
    github_token: '',
  })

  const createMutation = useMutation({
    mutationFn: () => accessReviewsApi.create({
      name: form.name,
      description: form.description,
      review_type: form.review_type,
      connection_id: form.connection_id || null,
      due_date: form.due_date || null,
      github_org: form.github_org,
      github_token: form.github_token,
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['access-reviews'] })
      qc.invalidateQueries({ queryKey: ['access-reviews-stats'] })
      toast.success('Access review created')
      onClose()
    },
    onError: () => toast.error('Failed to create review'),
  })

  const set = (k: string, v: string) => setForm((f) => ({ ...f, [k]: v }))

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-lg rounded-xl border bg-card shadow-xl p-6 space-y-4">
        <h2 className="text-lg font-semibold">New Access Review</h2>

        <div className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground font-medium">Review Name *</label>
            <input
              className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder="e.g. Q2 2026 Access Review"
              value={form.name}
              onChange={(e) => set('name', e.target.value)}
            />
          </div>

          <div>
            <label className="text-xs text-muted-foreground font-medium">Description</label>
            <textarea
              className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring resize-none"
              rows={2}
              placeholder="Optional context for reviewers"
              value={form.description}
              onChange={(e) => set('description', e.target.value)}
            />
          </div>

          <div>
            <label className="text-xs text-muted-foreground font-medium">Review Type</label>
            <select
              className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              value={form.review_type}
              onChange={(e) => set('review_type', e.target.value)}
            >
              <option value="manual">Manual (add members yourself)</option>
              <option value="do_team">Import from DigitalOcean Team</option>
              <option value="github_org">Import from GitHub Org</option>
            </select>
          </div>

          {form.review_type === 'do_team' && (
            <div>
              <label className="text-xs text-muted-foreground font-medium">DigitalOcean Connection</label>
              <select
                className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={form.connection_id}
                onChange={(e) => set('connection_id', e.target.value)}
              >
                <option value="">— select connection —</option>
                {doConnections.map((c) => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            </div>
          )}

          {form.review_type === 'github_org' && (
            <>
              <div>
                <label className="text-xs text-muted-foreground font-medium">GitHub Org Name *</label>
                <input
                  className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                  placeholder="e.g. my-company"
                  value={form.github_org}
                  onChange={(e) => set('github_org', e.target.value)}
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground font-medium">GitHub Token (optional, for private org)</label>
                <input
                  type="password"
                  className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                  placeholder="ghp_..."
                  value={form.github_token}
                  onChange={(e) => set('github_token', e.target.value)}
                />
              </div>
            </>
          )}

          <div>
            <label className="text-xs text-muted-foreground font-medium">Due Date</label>
            <input
              type="date"
              className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              value={form.due_date}
              onChange={(e) => set('due_date', e.target.value)}
            />
          </div>
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <button
            onClick={onClose}
            className="rounded-md border px-4 py-2 text-sm hover:bg-muted transition-colors"
          >
            Cancel
          </button>
          <button
            disabled={!form.name || createMutation.isPending}
            onClick={() => createMutation.mutate()}
            className="rounded-md bg-indigo-500 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-600 transition-colors disabled:opacity-50"
          >
            {createMutation.isPending ? 'Creating…' : 'Create Review'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export function AccessReviews() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)

  const { data: stats } = useQuery({
    queryKey: ['access-reviews-stats'],
    queryFn: accessReviewsApi.stats,
  })

  const { data: reviews = [], isLoading } = useQuery({
    queryKey: ['access-reviews'],
    queryFn: accessReviewsApi.list,
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => accessReviewsApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['access-reviews'] })
      qc.invalidateQueries({ queryKey: ['access-reviews-stats'] })
      toast.success('Review deleted')
    },
    onError: () => toast.error('Delete failed'),
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Access Reviews</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Periodically review and certify user access to your infrastructure and repositories
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 rounded-md bg-indigo-500 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-600 transition-colors"
        >
          <Plus className="h-4 w-4" />
          New Review
        </button>
      </div>

      {/* Stat cards */}
      {stats && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
          {[
            { label: 'Total', value: stats.total, icon: Users, cls: '' },
            { label: 'In Progress', value: stats.in_progress, icon: Clock, cls: 'text-blue-400' },
            { label: 'Completed', value: stats.completed, icon: CheckCircle2, cls: 'text-green-400' },
            { label: 'Overdue', value: stats.overdue, icon: AlertTriangle, cls: 'text-red-400' },
            { label: 'Due This Month', value: stats.due_this_month, icon: CalendarClock, cls: 'text-yellow-400' },
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
      )}

      {/* Reviews table */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">All Reviews</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-muted-foreground text-sm py-8 text-center">Loading…</p>
          ) : reviews.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-12 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                <Users className="h-6 w-6 text-muted-foreground" />
              </div>
              <div>
                <p className="font-medium text-sm">No access reviews yet</p>
                <p className="text-xs text-muted-foreground mt-1">
                  Create a review to start certifying user access.
                </p>
              </div>
              <button
                onClick={() => setShowCreate(true)}
                className="rounded-md bg-indigo-500 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-600 transition-colors"
              >
                Create First Review
              </button>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="pb-2 font-medium pr-4">Name</th>
                    <th className="pb-2 font-medium pr-4">Type</th>
                    <th className="pb-2 font-medium pr-4">Status</th>
                    <th className="pb-2 font-medium pr-4">Progress</th>
                    <th className="pb-2 font-medium pr-4">Due Date</th>
                    <th className="pb-2 font-medium pr-4">Created</th>
                    <th className="pb-2 font-medium" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {reviews.map((rv) => {
                    const pct = rv.item_count > 0 ? Math.round((rv.reviewed_count / rv.item_count) * 100) : 0
                    const typeLabels: Record<string, string> = {
                      manual: 'Manual',
                      do_team: 'DigitalOcean',
                      github_org: 'GitHub Org',
                    }
                    return (
                      <tr
                        key={rv.id}
                        className="hover:bg-muted/50 cursor-pointer transition-colors"
                        onClick={() => navigate(`/access-reviews/${rv.id}`)}
                      >
                        <td className="py-3 pr-4">
                          <p className="font-medium">{rv.name}</p>
                          {rv.description && (
                            <p className="text-xs text-muted-foreground truncate max-w-xs">{rv.description}</p>
                          )}
                        </td>
                        <td className="py-3 pr-4 text-muted-foreground">
                          {typeLabels[rv.review_type] ?? rv.review_type}
                        </td>
                        <td className="py-3 pr-4">
                          <ReviewStatusBadge status={rv.status} />
                        </td>
                        <td className="py-3 pr-4">
                          <div className="flex items-center gap-2">
                            <div className="h-1.5 w-20 rounded-full bg-muted overflow-hidden">
                              <div
                                className={`h-1.5 rounded-full ${pct === 100 ? 'bg-green-500' : 'bg-indigo-500'}`}
                                style={{ width: `${pct}%` }}
                              />
                            </div>
                            <span className="text-xs text-muted-foreground whitespace-nowrap">
                              {rv.reviewed_count}/{rv.item_count}
                            </span>
                          </div>
                        </td>
                        <td className="py-3 pr-4 text-muted-foreground">
                          {rv.due_date ? (
                            <span className={new Date(rv.due_date) < new Date() && rv.status !== 'completed' ? 'text-red-400 font-medium' : ''}>
                              {rv.due_date}
                            </span>
                          ) : '—'}
                        </td>
                        <td className="py-3 pr-4 text-muted-foreground text-xs">
                          {formatDate(rv.created_at)}
                        </td>
                        <td className="py-3">
                          <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                            <button
                              onClick={() => navigate(`/access-reviews/${rv.id}`)}
                              className="p-1.5 rounded hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
                              title="Open"
                            >
                              <ExternalLink className="h-3.5 w-3.5" />
                            </button>
                            <button
                              onClick={() => {
                                if (confirm('Delete this access review?')) deleteMutation.mutate(rv.id)
                              }}
                              className="p-1.5 rounded hover:bg-muted transition-colors text-muted-foreground hover:text-red-400"
                              title="Delete"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </button>
                          </div>
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

      {showCreate && <CreateReviewDialog onClose={() => setShowCreate(false)} />}
    </div>
  )
}
