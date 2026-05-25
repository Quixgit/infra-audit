import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  FileText, Shield, ShieldAlert, ShieldCheck, Lock, Key, Database,
  RefreshCw, Users, Server, Globe, Upload, Plus, Eye, Edit2,
  CheckCircle2, Download, Trash2, X, Link2, Clock, AlertTriangle,
  FileUp, ChevronRight,
} from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Separator } from '@/components/ui/separator'
import {
  policiesApi, complianceApi,
  type Policy, type PolicyTemplate, type PolicyControlMapping,
  type ComplianceFramework,
} from '@/lib/api'
import { formatDateShort } from '@/lib/utils'

// ── Constants ─────────────────────────────────────────────────────────────────

const POLICY_CATEGORIES = [
  'Information Security', 'Access Control', 'Incident Response',
  'Change Management', 'Backup & Recovery', 'Data Retention',
  'Acceptable Use', 'Vendor Management', 'Business Continuity',
  'Password', 'Encryption', 'Remote Work',
]

const STATUS_COLORS: Record<string, string> = {
  'Draft':        'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  'Under Review': 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
  'Approved':     'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
  'Expired':      'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  return (
    <Badge className={`border-0 text-xs font-medium ${STATUS_COLORS[status] ?? STATUS_COLORS['Draft']}`}>
      {status}
    </Badge>
  )
}

function ReviewDateCell({ dateStr }: { dateStr?: string }) {
  if (!dateStr) return <span className="text-muted-foreground text-xs">—</span>
  const date = new Date(dateStr)
  const now = new Date()
  const diffDays = Math.floor((date.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))

  if (diffDays < 0) {
    return <span className="text-xs font-medium text-red-500">Overdue</span>
  }
  if (diffDays <= 30) {
    return <span className="text-xs font-medium text-yellow-500">{formatDateShort(dateStr)}</span>
  }
  if (diffDays <= 90) {
    return <span className="text-xs font-medium text-yellow-400">{formatDateShort(dateStr)}</span>
  }
  return <span className="text-xs text-green-500">{formatDateShort(dateStr)}</span>
}

function categoryIcon(category: string) {
  const map: Record<string, React.ElementType> = {
    'Information Security': Shield,
    'Access Control': Lock,
    'Incident Response': ShieldAlert,
    'Change Management': RefreshCw,
    'Backup & Recovery': Database,
    'Data Retention': Server,
    'Acceptable Use': Globe,
    'Vendor Management': Users,
    'Business Continuity': RefreshCw,
    'Password': Key,
    'Encryption': Lock,
    'Remote Work': Globe,
  }
  const Icon = map[category] ?? FileText
  return <Icon className="h-5 w-5" />
}

function defaultReviewDate(): string {
  const d = new Date()
  d.setFullYear(d.getFullYear() + 1)
  return d.toISOString().split('T')[0]
}

function todayDate(): string {
  return new Date().toISOString().split('T')[0]
}

// ── Generate Dialog ───────────────────────────────────────────────────────────

function GenerateDialog({
  template,
  onClose,
}: {
  template: PolicyTemplate
  onClose: () => void
}) {
  const qc = useQueryClient()
  const [companyName, setCompanyName] = useState('')
  const [effectiveDate, setEffectiveDate] = useState(todayDate)
  const [reviewDate, setReviewDate] = useState(defaultReviewDate)
  const [ownerName, setOwnerName] = useState('')
  const [ownerTitle, setOwnerTitle] = useState('CISO')

  const generate = useMutation({
    mutationFn: () =>
      policiesApi.create({
        template_slug: template.slug,
        company_name: companyName,
        effective_date: effectiveDate,
        review_date: reviewDate,
        owner_name: ownerName,
        owner_title: ownerTitle,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      qc.invalidateQueries({ queryKey: ['policy-stats'] })
      toast.success('Policy generated and saved as draft')
      onClose()
    },
    onError: () => toast.error('Failed to generate policy'),
  })

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Generate: {template.name}</DialogTitle>
          <p className="text-sm text-muted-foreground pt-1">{template.description}</p>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1">
            <Label>Company Name <span className="text-red-400">*</span></Label>
            <Input value={companyName} onChange={(e) => setCompanyName(e.target.value)}
              placeholder="Acme Corp" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label>Effective Date</Label>
              <Input type="date" value={effectiveDate} onChange={(e) => setEffectiveDate(e.target.value)} />
            </div>
            <div className="space-y-1">
              <Label>Review Date</Label>
              <Input type="date" value={reviewDate} onChange={(e) => setReviewDate(e.target.value)} />
            </div>
          </div>
          <div className="space-y-1">
            <Label>Policy Owner Name <span className="text-red-400">*</span></Label>
            <Input value={ownerName} onChange={(e) => setOwnerName(e.target.value)}
              placeholder="Jane Smith" />
          </div>
          <div className="space-y-1">
            <Label>Owner Title</Label>
            <Input value={ownerTitle} onChange={(e) => setOwnerTitle(e.target.value)}
              placeholder="CISO" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button
            disabled={!companyName.trim() || !ownerName.trim() || generate.isPending}
            onClick={() => generate.mutate()}
          >
            {generate.isPending ? 'Generating…' : 'Generate & Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Upload Dialog ─────────────────────────────────────────────────────────────

function UploadDialog({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [name, setName] = useState('')
  const [category, setCategory] = useState('')
  const [reviewDate, setReviewDate] = useState(defaultReviewDate)

  const upload = useMutation({
    mutationFn: () => {
      if (!file) throw new Error('no file')
      const fd = new FormData()
      fd.append('file', file)
      fd.append('name', name || file.name)
      fd.append('category', category)
      fd.append('review_date', reviewDate)
      return policiesApi.upload(fd)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      qc.invalidateQueries({ queryKey: ['policy-stats'] })
      toast.success('Policy uploaded')
      onClose()
    },
    onError: () => toast.error('Upload failed'),
  })

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Upload Policy Document</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div
            className="border-2 border-dashed rounded-lg p-6 text-center cursor-pointer hover:border-primary/50 transition-colors"
            onClick={() => fileRef.current?.click()}
          >
            <FileUp className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
            {file ? (
              <p className="text-sm font-medium">{file.name}</p>
            ) : (
              <p className="text-sm text-muted-foreground">Click to choose a file (PDF, HTML, DOCX — max 20 MB)</p>
            )}
            <input ref={fileRef} type="file"
              accept=".pdf,.html,.htm,.docx,.doc,.txt"
              className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0] ?? null
                setFile(f)
                if (f && !name) setName(f.name.replace(/\.[^/.]+$/, ''))
              }} />
          </div>
          <div className="space-y-1">
            <Label>Policy Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Access Control Policy" />
          </div>
          <div className="space-y-1">
            <Label>Category</Label>
            <Select value={category} onValueChange={setCategory}>
              <SelectTrigger><SelectValue placeholder="Select category…" /></SelectTrigger>
              <SelectContent>
                {POLICY_CATEGORIES.map((c) => (
                  <SelectItem key={c} value={c}>{c}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Review Date</Label>
            <Input type="date" value={reviewDate} onChange={(e) => setReviewDate(e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button disabled={!file || upload.isPending} onClick={() => upload.mutate()}>
            {upload.isPending ? 'Uploading…' : 'Upload'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Detail Drawer ─────────────────────────────────────────────────────────────

type MappingFramework = { slug: string; name: string; controls: { id: string; name: string }[] }

function toMappingFrameworks(frameworks: ComplianceFramework[] = []): MappingFramework[] {
  return frameworks
    .filter((f) => (f.controls?.length ?? 0) > 0)
    .map((f) => ({
      slug: f.slug,
      name: f.name,
      controls: (f.controls ?? []).map((c) => ({ id: c.ctrl_id, name: c.name })),
    }))
}

function PolicyDrawer({
  policy,
  frameworks,
  onClose,
  onUpdated,
}: {
  policy: Policy
  frameworks: MappingFramework[]
  onClose: () => void
  onUpdated: (p: Policy) => void
}) {
  const qc = useQueryClient()
  const [tab, setTab] = useState<'content' | 'approval' | 'review' | 'controls'>('content')
  const [editing, setEditing] = useState(false)
  const [editContent, setEditContent] = useState(policy.content_html ?? '')
  const [reviewDate, setReviewDate] = useState(policy.review_date ?? defaultReviewDate())

  // Controls mapping state
  const [controls, setControls] = useState<PolicyControlMapping[]>(policy.controls ?? [])
  const [selFw, setSelFw] = useState(frameworks[0]?.slug ?? '')
  const [selCtrl, setSelCtrl] = useState('')
  const fwControls = frameworks.find((f) => f.slug === selFw)?.controls ?? []

  const saveMut = useMutation({
    mutationFn: () =>
      policiesApi.update(policy.id, {
        name: policy.name,
        category: policy.category,
        content_html: editContent,
        status: policy.status,
        review_date: policy.review_date,
        controls,
      }),
    onSuccess: (updated) => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      onUpdated(updated)
      setEditing(false)
      toast.success('Policy saved')
    },
    onError: () => toast.error('Save failed'),
  })

  const approveMut = useMutation({
    mutationFn: () => policiesApi.approve(policy.id),
    onSuccess: (updated) => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      qc.invalidateQueries({ queryKey: ['policy-stats'] })
      onUpdated(updated)
      toast.success('Policy approved')
    },
    onError: () => toast.error('Approve failed'),
  })

  const reviewMut = useMutation({
    mutationFn: () => policiesApi.markReviewed(policy.id, reviewDate),
    onSuccess: (updated) => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      onUpdated(updated)
      toast.success('Marked as reviewed')
    },
    onError: () => toast.error('Failed'),
  })

  const saveControlsMut = useMutation({
    mutationFn: () =>
      policiesApi.update(policy.id, {
        name: policy.name,
        category: policy.category,
        content_html: policy.content_html,
        status: policy.status,
        review_date: policy.review_date,
        controls,
      }),
    onSuccess: (updated) => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      onUpdated(updated)
      toast.success('Controls saved')
    },
    onError: () => toast.error('Save failed'),
  })

  function addControl() {
    if (!selCtrl) return
    const exists = controls.some((c) => c.framework_slug === selFw && c.control_code === selCtrl)
    if (!exists) {
      setControls([...controls, { policy_id: policy.id, framework_slug: selFw, control_code: selCtrl }])
    }
    setSelCtrl('')
  }

  function removeControl(i: number) {
    setControls(controls.filter((_, idx) => idx !== i))
  }

  const TABS = [
    { key: 'content', label: 'Content' },
    { key: 'approval', label: 'Approval' },
    { key: 'review', label: 'Review' },
    { key: 'controls', label: 'Controls' },
  ] as const

  return (
    <div className="fixed inset-0 z-50 flex">
      {/* Backdrop */}
      <div className="flex-1 bg-black/40 cursor-pointer" onClick={onClose} />

      {/* Drawer */}
      <div className="w-full max-w-2xl bg-background border-l flex flex-col h-full overflow-hidden shadow-2xl">
        {/* Header */}
        <div className="flex items-start justify-between gap-4 px-6 py-4 border-b shrink-0">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <StatusBadge status={policy.status} />
              <span className="text-xs text-muted-foreground">v{policy.version}</span>
            </div>
            <h2 className="text-lg font-bold truncate">{policy.name}</h2>
            <p className="text-sm text-muted-foreground">{policy.category}</p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <Button size="sm" variant="outline" asChild>
              <a href={policiesApi.downloadUrl(policy.id)} download>
                <Download className="h-4 w-4 mr-1.5" /> Download
              </a>
            </Button>
            <Button size="icon" variant="ghost" onClick={onClose}>
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex border-b px-6 shrink-0">
          {TABS.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                tab === t.key
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        {/* Tab content */}
        <div className="flex-1 overflow-y-auto">

          {/* ── Content tab ─────────────────────────────── */}
          {tab === 'content' && (
            <div className="p-6 space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-xs text-muted-foreground">
                  Created {formatDateShort(policy.created_at)}
                  {policy.updated_at !== policy.created_at && ` · Updated ${formatDateShort(policy.updated_at)}`}
                </p>
                {!editing && (
                  <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
                    <Edit2 className="h-3.5 w-3.5 mr-1.5" /> Edit
                  </Button>
                )}
                {editing && (
                  <div className="flex gap-2">
                    <Button size="sm" variant="outline" onClick={() => { setEditing(false); setEditContent(policy.content_html ?? '') }}>
                      Cancel
                    </Button>
                    <Button size="sm" disabled={saveMut.isPending} onClick={() => saveMut.mutate()}>
                      {saveMut.isPending ? 'Saving…' : 'Save'}
                    </Button>
                  </div>
                )}
              </div>

              {editing ? (
                <Textarea
                  value={editContent}
                  onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setEditContent(e.target.value)}
                  className="font-mono text-xs min-h-[500px]"
                  placeholder="HTML content…"
                />
              ) : policy.content_html ? (
                <div
                  className="prose prose-sm max-w-none dark:prose-invert border rounded-lg p-4 bg-muted/20 overflow-x-auto text-sm"
                  dangerouslySetInnerHTML={{ __html: policy.content_html }}
                />
              ) : (
                <div className="flex flex-col items-center gap-3 py-12 text-center">
                  <FileText className="h-10 w-10 text-muted-foreground" />
                  <p className="text-sm text-muted-foreground">
                    {policy.file_name
                      ? `Uploaded file: ${policy.file_name}`
                      : 'No content available. Upload a file or generate from template.'}
                  </p>
                </div>
              )}
            </div>
          )}

          {/* ── Approval tab ────────────────────────────── */}
          {tab === 'approval' && (
            <div className="p-6 space-y-5">
              <div className="rounded-lg border p-4 space-y-3">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-medium">Current Status</p>
                  <StatusBadge status={policy.status} />
                </div>
                {policy.approved_at && (
                  <div className="text-sm text-muted-foreground space-y-1">
                    <p>
                      <span className="font-medium text-foreground">Approved by:</span>{' '}
                      {policy.approved_by_email || 'Unknown'}
                    </p>
                    <p>
                      <span className="font-medium text-foreground">Approved on:</span>{' '}
                      {formatDateShort(policy.approved_at)}
                    </p>
                  </div>
                )}
              </div>

              {policy.status !== 'Approved' && (
                <div className="space-y-2">
                  <p className="text-sm text-muted-foreground">
                    Approving this policy will set its status to <strong>Approved</strong> and record your name and the approval date.
                  </p>
                  <Button
                    disabled={approveMut.isPending}
                    onClick={() => approveMut.mutate()}
                    className="w-full"
                  >
                    <CheckCircle2 className="h-4 w-4 mr-2" />
                    {approveMut.isPending ? 'Approving…' : 'Approve Policy'}
                  </Button>
                </div>
              )}

              {policy.status === 'Approved' && (
                <div className="flex items-center gap-2 text-sm text-green-600 dark:text-green-400">
                  <CheckCircle2 className="h-4 w-4" />
                  This policy is approved and in effect.
                </div>
              )}
            </div>
          )}

          {/* ── Review tab ──────────────────────────────── */}
          {tab === 'review' && (
            <div className="p-6 space-y-5">
              {policy.last_reviewed_at && (
                <div className="rounded-lg border p-4 text-sm space-y-1">
                  <p>
                    <span className="font-medium">Last reviewed:</span>{' '}
                    {formatDateShort(policy.last_reviewed_at)}
                  </p>
                </div>
              )}

              <div className="space-y-3">
                <div className="space-y-1">
                  <Label>Next Review Date</Label>
                  <Input
                    type="date"
                    value={reviewDate}
                    onChange={(e) => setReviewDate(e.target.value)}
                  />
                </div>
                <p className="text-xs text-muted-foreground">
                  Current scheduled review: {policy.review_date ? formatDateShort(policy.review_date) : 'Not set'}
                </p>
                <Button
                  disabled={reviewMut.isPending}
                  onClick={() => reviewMut.mutate()}
                  className="w-full"
                >
                  <RefreshCw className="h-4 w-4 mr-2" />
                  {reviewMut.isPending ? 'Saving…' : 'Mark as Reviewed'}
                </Button>
              </div>
            </div>
          )}

          {/* ── Controls tab ────────────────────────────── */}
          {tab === 'controls' && (
            <div className="p-6 space-y-5">
              {/* Current mappings */}
              {controls.length > 0 && (
                <div className="space-y-2">
                  <Label>Mapped controls ({controls.length})</Label>
                  <div className="flex flex-wrap gap-2">
                    {controls.map((c, i) => {
                      const fwName = frameworks.find((f) => f.slug === c.framework_slug)?.name ?? c.framework_slug
                      return (
                        <span key={i} className="inline-flex items-center gap-1 bg-muted text-xs rounded-full px-2.5 py-1">
                          <span className="font-medium">{fwName}</span>
                          <span className="text-muted-foreground">·</span>
                          {c.control_code}
                          <button onClick={() => removeControl(i)} className="ml-1 hover:text-destructive">
                            <X className="h-3 w-3" />
                          </button>
                        </span>
                      )
                    })}
                  </div>
                </div>
              )}

              {frameworks.length > 0 ? (
                <div className="space-y-2">
                  <Label>Add control mapping</Label>
                  <div className="flex gap-2">
                    <Select value={selFw} onValueChange={(v) => { setSelFw(v); setSelCtrl('') }}>
                      <SelectTrigger className="w-36 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {frameworks.map((f) => (
                          <SelectItem key={f.slug} value={f.slug}>{f.name}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Select value={selCtrl} onValueChange={setSelCtrl}>
                      <SelectTrigger className="flex-1 text-xs">
                        <SelectValue placeholder="Select control…" />
                      </SelectTrigger>
                      <SelectContent>
                        {fwControls.map((c) => (
                          <SelectItem key={c.id} value={c.id}>
                            <span className="font-mono text-xs">{c.id}</span> — {c.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Button size="icon" variant="outline" onClick={addControl} disabled={!selCtrl}>
                      <Plus className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">No compliance frameworks loaded.</p>
              )}

              <Separator />
              <Button
                disabled={saveControlsMut.isPending}
                onClick={() => saveControlsMut.mutate()}
                className="w-full"
              >
                <Link2 className="h-4 w-4 mr-2" />
                {saveControlsMut.isPending ? 'Saving…' : 'Save Controls Mapping'}
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ── Edit Policy Dialog ────────────────────────────────────────────────────────

function EditPolicyDialog({
  policy,
  onClose,
  onSaved,
}: {
  policy: Policy
  onClose: () => void
  onSaved: (p: Policy) => void
}) {
  const qc = useQueryClient()
  const [name, setName] = useState(policy.name)
  const [category, setCategory] = useState(policy.category)
  const [status, setStatus] = useState(policy.status)
  const [reviewDate, setReviewDate] = useState(policy.review_date ?? '')

  const save = useMutation({
    mutationFn: () =>
      policiesApi.update(policy.id, {
        name,
        category,
        content_html: policy.content_html,
        status,
        review_date: reviewDate || undefined,
        controls: policy.controls,
      }),
    onSuccess: (updated) => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      qc.invalidateQueries({ queryKey: ['policy-stats'] })
      onSaved(updated)
      toast.success('Policy updated')
    },
    onError: () => toast.error('Update failed'),
  })

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Edit Policy</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label>Category</Label>
            <Select value={category} onValueChange={setCategory}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {POLICY_CATEGORIES.map((c) => (
                  <SelectItem key={c} value={c}>{c}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Status</Label>
            <Select value={status} onValueChange={(v) => setStatus(v as Policy['status'])}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="Draft">Draft</SelectItem>
                <SelectItem value="Under Review">Under Review</SelectItem>
                <SelectItem value="Approved">Approved</SelectItem>
                <SelectItem value="Expired">Expired</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Review Date</Label>
            <Input type="date" value={reviewDate} onChange={(e) => setReviewDate(e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button disabled={save.isPending} onClick={() => save.mutate()}>
            {save.isPending ? 'Saving…' : 'Save Changes'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Main Page ─────────────────────────────────────────────────────────────────

export function Policies() {
  const qc = useQueryClient()

  const [generateTemplate, setGenerateTemplate] = useState<PolicyTemplate | null>(null)
  const [showUpload, setShowUpload] = useState(false)
  const [drawerPolicy, setDrawerPolicy] = useState<Policy | null>(null)
  const [editPolicy, setEditPolicy] = useState<Policy | null>(null)
  const [search, setSearch] = useState('')
  const [filterStatus, setFilterStatus] = useState('all')
  const [filterCategory, setFilterCategory] = useState('all')

  const { data: templates = [] } = useQuery({
    queryKey: ['policy-templates'],
    queryFn: policiesApi.listTemplates,
  })

  const { data: policies = [], isLoading } = useQuery({
    queryKey: ['policies'],
    queryFn: policiesApi.list,
  })

  const { data: stats } = useQuery({
    queryKey: ['policy-stats'],
    queryFn: policiesApi.stats,
  })

  const { data: complianceFrameworks = [] } = useQuery({
    queryKey: ['compliance-frameworks'],
    queryFn: complianceApi.listFrameworks,
  })
  const frameworks = toMappingFrameworks(complianceFrameworks)

  const deleteMut = useMutation({
    mutationFn: (id: string) => policiesApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      qc.invalidateQueries({ queryKey: ['policy-stats'] })
      toast.success('Policy deleted')
    },
    onError: () => toast.error('Delete failed'),
  })

  const approveMut = useMutation({
    mutationFn: (id: string) => policiesApi.approve(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['policies'] })
      qc.invalidateQueries({ queryKey: ['policy-stats'] })
      toast.success('Policy approved')
    },
    onError: () => toast.error('Approve failed'),
  })

  const filtered = policies.filter((p) => {
    if (filterStatus !== 'all' && p.status !== filterStatus) return false
    if (filterCategory !== 'all' && p.category !== filterCategory) return false
    if (search) {
      const hay = (p.name + ' ' + p.category + ' ' + p.status).toLowerCase()
      if (!hay.includes(search.toLowerCase())) return false
    }
    return true
  })

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Policies</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage, generate, and track your security policies
          </p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={() => setShowUpload(true)}>
            <Upload className="h-4 w-4 mr-2" />
            Upload
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
        {[
          { label: 'Total',      value: stats?.total      ?? 0, icon: FileText,     cls: '' },
          { label: 'Approved',   value: stats?.approved   ?? 0, icon: CheckCircle2, cls: 'text-green-400' },
          { label: 'Draft',      value: stats?.draft      ?? 0, icon: Edit2,        cls: 'text-muted-foreground' },
          { label: 'Expired',    value: stats?.expired    ?? 0, icon: AlertTriangle, cls: 'text-red-400' },
          { label: 'Review Due', value: stats?.review_due ?? 0, icon: Clock,        cls: 'text-yellow-400' },
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

      {/* Section 1 — Templates */}
      <div>
        <h2 className="text-base font-semibold mb-3 flex items-center gap-2">
          <FileText className="h-4 w-4 text-indigo-400" />
          Policy Templates
        </h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {templates.map((tmpl) => (
            <Card key={tmpl.slug} className="hover:border-indigo-500/40 transition-colors">
              <CardContent className="p-4 flex items-start gap-3">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-indigo-500/10 text-indigo-400">
                  {categoryIcon(tmpl.category)}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-semibold leading-tight">{tmpl.name}</p>
                  <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{tmpl.description}</p>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  className="shrink-0 h-7 text-xs"
                  onClick={() => setGenerateTemplate(tmpl)}
                >
                  Generate
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>

      {/* Section 2 — Your Policies */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-base font-semibold flex items-center gap-2">
            <ShieldCheck className="h-4 w-4 text-indigo-400" />
            Your Policies
            {policies.length > 0 && (
              <span className="text-xs font-normal text-muted-foreground">({policies.length})</span>
            )}
          </h2>
        </div>

        {/* Filters */}
        <div className="flex flex-wrap gap-2 mb-3">
          <Input
            className="h-8 w-48 text-xs"
            placeholder="Search policies…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <Select value={filterStatus} onValueChange={setFilterStatus}>
            <SelectTrigger className="h-8 w-36 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="Draft">Draft</SelectItem>
              <SelectItem value="Under Review">Under Review</SelectItem>
              <SelectItem value="Approved">Approved</SelectItem>
              <SelectItem value="Expired">Expired</SelectItem>
            </SelectContent>
          </Select>
          <Select value={filterCategory} onValueChange={setFilterCategory}>
            <SelectTrigger className="h-8 w-44 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All categories</SelectItem>
              {POLICY_CATEGORIES.map((c) => (
                <SelectItem key={c} value={c}>{c}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {isLoading ? (
          <div className="space-y-2">
            {[...Array(3)].map((_, i) => <div key={i} className="h-12 rounded-lg bg-muted animate-pulse" />)}
          </div>
        ) : filtered.length === 0 ? (
          policies.length === 0 ? (
            <Card>
              <CardContent className="py-12 flex flex-col items-center gap-3 text-center">
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                  <ShieldCheck className="h-6 w-6 text-muted-foreground" />
                </div>
                <div>
                  <p className="font-medium text-sm">No policies yet</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    Generate a policy from a template above, or upload an existing document.
                  </p>
                </div>
              </CardContent>
            </Card>
          ) : (
            <Card>
              <CardContent className="py-8 text-center text-sm text-muted-foreground">
                No policies match your filters.
              </CardContent>
            </Card>
          )
        ) : (
          <Card>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b bg-muted/30">
                      <th className="text-left px-4 py-2.5 text-xs font-semibold text-muted-foreground">Name</th>
                      <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Category</th>
                      <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground w-10">Ver</th>
                      <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Status</th>
                      <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Approved by</th>
                      <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Review Date</th>
                      <th className="text-left px-3 py-2.5 text-xs font-semibold text-muted-foreground">Controls</th>
                      <th className="text-right px-4 py-2.5 text-xs font-semibold text-muted-foreground">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {filtered.map((policy) => (
                      <tr
                        key={policy.id}
                        className="hover:bg-muted/30 cursor-pointer transition-colors"
                        onClick={() => setDrawerPolicy(policy)}
                      >
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <span className="text-muted-foreground">{categoryIcon(policy.category)}</span>
                            <span className="font-medium text-sm truncate max-w-[180px]">{policy.name}</span>
                          </div>
                        </td>
                        <td className="px-3 py-3">
                          <span className="text-xs text-muted-foreground">{policy.category || '—'}</span>
                        </td>
                        <td className="px-3 py-3">
                          <span className="text-xs text-muted-foreground">v{policy.version}</span>
                        </td>
                        <td className="px-3 py-3">
                          <StatusBadge status={policy.status} />
                        </td>
                        <td className="px-3 py-3">
                          <span className="text-xs text-muted-foreground truncate max-w-[120px] block">
                            {policy.approved_by_email || '—'}
                          </span>
                        </td>
                        <td className="px-3 py-3">
                          <ReviewDateCell dateStr={policy.review_date} />
                        </td>
                        <td className="px-3 py-3">
                          {(policy.controls?.length ?? 0) > 0 ? (
                            <Badge variant="secondary" className="text-xs">
                              <Link2 className="h-3 w-3 mr-1" />
                              {policy.controls.length}
                            </Badge>
                          ) : (
                            <span className="text-xs text-muted-foreground">—</span>
                          )}
                        </td>
                        <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                          <div className="flex items-center gap-1 justify-end">
                            <Button
                              size="icon" variant="ghost" className="h-7 w-7"
                              title="View"
                              onClick={() => setDrawerPolicy(policy)}
                            >
                              <Eye className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              size="icon" variant="ghost" className="h-7 w-7"
                              title="Edit"
                              onClick={() => setEditPolicy(policy)}
                            >
                              <Edit2 className="h-3.5 w-3.5" />
                            </Button>
                            {policy.status !== 'Approved' && (
                              <Button
                                size="icon" variant="ghost" className="h-7 w-7 text-green-600 hover:text-green-600"
                                title="Approve"
                                disabled={approveMut.isPending}
                                onClick={() => approveMut.mutate(policy.id)}
                              >
                                <CheckCircle2 className="h-3.5 w-3.5" />
                              </Button>
                            )}
                            <Button
                              size="icon" variant="ghost" className="h-7 w-7"
                              title="Download"
                              asChild
                            >
                              <a href={policiesApi.downloadUrl(policy.id)} download onClick={(e) => e.stopPropagation()}>
                                <Download className="h-3.5 w-3.5" />
                              </a>
                            </Button>
                            <Button
                              size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive"
                              title="Delete"
                              onClick={() => {
                                if (confirm(`Delete "${policy.name}"?`)) {
                                  deleteMut.mutate(policy.id)
                                  if (drawerPolicy?.id === policy.id) setDrawerPolicy(null)
                                }
                              }}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Modals & Drawer */}
      {generateTemplate && (
        <GenerateDialog
          template={generateTemplate}
          onClose={() => setGenerateTemplate(null)}
        />
      )}
      {showUpload && <UploadDialog onClose={() => setShowUpload(false)} />}
      {editPolicy && (
        <EditPolicyDialog
          policy={editPolicy}
          onClose={() => setEditPolicy(null)}
          onSaved={(updated) => {
            setEditPolicy(null)
            if (drawerPolicy?.id === updated.id) setDrawerPolicy(updated)
          }}
        />
      )}
      {drawerPolicy && (
        <PolicyDrawer
          policy={drawerPolicy}
          frameworks={frameworks}
          onClose={() => setDrawerPolicy(null)}
          onUpdated={(updated) => setDrawerPolicy(updated)}
        />
      )}
    </div>
  )
}
