import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  FolderOpen, Upload, Trash2, Download, Link2, X, Plus,
  ShieldCheck, FileText, FileJson, Image, FileUp, AlertTriangle,
  CheckCircle2, Clock,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { EmptyState } from '@/components/EmptyState'
import { complianceApi, evidenceApi, type ComplianceFramework, type EvidenceItem } from '@/lib/api'
import { formatDate } from '@/lib/utils'

// ── Framework/control data for mapping UI ─────────────────────────────────────

type MappingFramework = {
  slug: string
  name: string
  controls: { id: string; name: string }[]
}

function toMappingFrameworks(frameworks: ComplianceFramework[] = []): MappingFramework[] {
  return frameworks
    .filter((f) => (f.controls?.length ?? 0) > 0)
    .map((f) => ({
      slug: f.slug,
      name: f.name,
      controls: (f.controls ?? []).map((c) => ({ id: c.ctrl_id, name: c.name })),
    }))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function statusBadge(status: string) {
  if (status === 'expired')
    return <Badge className="bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300 border-0">Expired</Badge>
  if (status === 'stale')
    return <Badge className="bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300 border-0">Stale</Badge>
  return <Badge className="bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300 border-0">Fresh</Badge>
}

function statusIcon(status: string) {
  if (status === 'expired') return <AlertTriangle className="h-4 w-4 text-red-500" />
  if (status === 'stale') return <Clock className="h-4 w-4 text-yellow-500" />
  return <CheckCircle2 className="h-4 w-4 text-green-500" />
}

function typeIcon(evType: string) {
  if (evType === 'inventory') return <FileJson className="h-4 w-4" />
  if (evType === 'findings') return <FileJson className="h-4 w-4" />
  if (evType === 'report') return <FileText className="h-4 w-4" />
  if (evType === 'screenshot') return <Image className="h-4 w-4" />
  if (evType === 'policy') return <ShieldCheck className="h-4 w-4" />
  return <FileUp className="h-4 w-4" />
}

function formatBytes(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

// ── Upload Dialog ─────────────────────────────────────────────────────────────

function UploadDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [evType, setEvType] = useState('policy')
  const [expiresAt, setExpiresAt] = useState(() => {
    const d = new Date(); d.setFullYear(d.getFullYear() + 1)
    return d.toISOString().split('T')[0]
  })
  const [file, setFile] = useState<File | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  const upload = useMutation({
    mutationFn: () => {
      if (!file) throw new Error('no file')
      const fd = new FormData()
      fd.append('file', file)
      fd.append('name', name || file.name)
      fd.append('description', description)
      fd.append('evidence_type', evType)
      fd.append('expires_at', expiresAt)
      return evidenceApi.upload(fd)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['evidence'] })
      onClose()
      setFile(null); setName(''); setDescription('')
    },
  })

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Upload Evidence</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div
            className="border-2 border-dashed rounded-lg p-6 text-center cursor-pointer hover:border-primary/50 transition-colors"
            onClick={() => fileRef.current?.click()}
          >
            <Upload className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
            {file ? (
              <p className="text-sm font-medium">{file.name} <span className="text-muted-foreground">({formatBytes(file.size)})</span></p>
            ) : (
              <p className="text-sm text-muted-foreground">Click to choose a file (max 10 MB)</p>
            )}
            <input ref={fileRef} type="file" className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0] ?? null
                setFile(f)
                if (f && !name) setName(f.name)
              }} />
          </div>

          <div className="space-y-1">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Access Control Policy" />
          </div>

          <div className="space-y-1">
            <Label>Type</Label>
            <Select value={evType} onValueChange={setEvType}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="policy">Policy document</SelectItem>
                <SelectItem value="screenshot">Screenshot / screen capture</SelectItem>
                <SelectItem value="config">Configuration export</SelectItem>
                <SelectItem value="report">Report / audit output</SelectItem>
                <SelectItem value="other">Other</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <Label>Description <span className="text-muted-foreground font-normal">(optional)</span></Label>
            <Textarea value={description} onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setDescription(e.target.value)}
              placeholder="What does this evidence demonstrate?" rows={2} />
          </div>

          <div className="space-y-1">
            <Label>Expires on</Label>
            <Input type="date" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button
            disabled={!file || upload.isPending}
            onClick={() => upload.mutate()}
          >
            {upload.isPending ? 'Uploading…' : 'Upload'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Mapping Dialog ────────────────────────────────────────────────────────────

function MappingDialog({
  item,
  frameworks,
  onClose,
}: {
  item: EvidenceItem
  frameworks: MappingFramework[]
  onClose: () => void
}) {
  const qc = useQueryClient()
  const [mappings, setMappings] = useState<{ framework_slug: string; ctrl_id: string }[]>(
    (item.mappings ?? []).map((m) => ({ framework_slug: m.framework_slug, ctrl_id: m.ctrl_id }))
  )
  const [selFw, setSelFw] = useState(frameworks[0]?.slug ?? '')
  const [selCtrl, setSelCtrl] = useState('')

  const fwControls = frameworks.find((f) => f.slug === selFw)?.controls ?? []

  const save = useMutation({
    mutationFn: () => evidenceApi.setMappings(item.id, mappings),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['evidence'] })
      onClose()
    },
  })

  function addMapping() {
    if (!selCtrl) return
    const exists = mappings.some((m) => m.framework_slug === selFw && m.ctrl_id === selCtrl)
    if (!exists) setMappings([...mappings, { framework_slug: selFw, ctrl_id: selCtrl }])
    setSelCtrl('')
  }

  function removeMapping(i: number) {
    setMappings(mappings.filter((_, idx) => idx !== i))
  }

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Map to Controls</DialogTitle>
          <p className="text-sm text-muted-foreground pt-1">{item.name}</p>
        </DialogHeader>
        <div className="space-y-4 py-2">
          {/* Current mappings */}
          {mappings.length > 0 && (
            <div className="space-y-1">
              <Label>Mapped controls</Label>
              <div className="flex flex-wrap gap-2 mt-1">
                {mappings.map((m, i) => {
                  const fwName = frameworks.find((f) => f.slug === m.framework_slug)?.name ?? m.framework_slug
                  return (
                    <span key={i} className="inline-flex items-center gap-1 bg-muted text-xs rounded-full px-2.5 py-1">
                      <span className="font-medium">{fwName}</span>
                      <span className="text-muted-foreground">·</span>
                      {m.ctrl_id}
                      <button onClick={() => removeMapping(i)} className="ml-1 hover:text-destructive">
                        <X className="h-3 w-3" />
                      </button>
                    </span>
                  )
                })}
              </div>
            </div>
          )}

          {/* Add mapping */}
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
              <Button size="icon" variant="outline" onClick={addMapping} disabled={!selCtrl}>
                <Plus className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button disabled={save.isPending} onClick={() => save.mutate()}>
            {save.isPending ? 'Saving…' : 'Save Mappings'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────────

type FilterStatus = 'all' | 'fresh' | 'stale' | 'expired'
type FilterSource = 'all' | 'auto' | 'manual'

export function Evidence() {
  const qc = useQueryClient()
  const [showUpload, setShowUpload] = useState(false)
  const [mappingItem, setMappingItem] = useState<EvidenceItem | null>(null)
  const [filterStatus, setFilterStatus] = useState<FilterStatus>('all')
  const [filterSource, setFilterSource] = useState<FilterSource>('all')
  const [filterType, setFilterType] = useState('all')
  const [search, setSearch] = useState('')

  const { data: items = [], isLoading } = useQuery({
    queryKey: ['evidence'],
    queryFn: evidenceApi.list,
  })
  const { data: complianceFrameworks = [] } = useQuery({
    queryKey: ['compliance-frameworks'],
    queryFn: complianceApi.listFrameworks,
  })
  const frameworks = toMappingFrameworks(complianceFrameworks)

  const deleteMut = useMutation({
    mutationFn: evidenceApi.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['evidence'] }),
  })

  const filtered = items.filter((it) => {
    if (filterStatus !== 'all' && it.status !== filterStatus) return false
    if (filterSource !== 'all' && it.source !== filterSource) return false
    if (filterType !== 'all' && it.evidence_type !== filterType) return false
    if (search) {
      const hay = (it.name + ' ' + it.description + ' ' + (it.job_name ?? '')).toLowerCase()
      if (!hay.includes(search.toLowerCase())) return false
    }
    return true
  })

  const counts = {
    fresh: items.filter((i) => i.status === 'fresh').length,
    stale: items.filter((i) => i.status === 'stale').length,
    expired: items.filter((i) => i.status === 'expired').length,
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Evidence</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Audit evidence artifacts mapped to compliance controls
          </p>
        </div>
        <Button size="sm" onClick={() => setShowUpload(true)}>
          <Upload className="h-4 w-4 mr-2" />
          Upload Evidence
        </Button>
      </div>

      {/* Status summary cards */}
      <div className="grid grid-cols-3 gap-3">
        {[
          { key: 'fresh', label: 'Fresh', count: counts.fresh, cls: 'text-green-600 dark:text-green-400', bg: 'bg-green-50 dark:bg-green-950/30', icon: CheckCircle2 },
          { key: 'stale', label: 'Stale', count: counts.stale, cls: 'text-yellow-600 dark:text-yellow-400', bg: 'bg-yellow-50 dark:bg-yellow-950/30', icon: Clock },
          { key: 'expired', label: 'Expired', count: counts.expired, cls: 'text-red-600 dark:text-red-400', bg: 'bg-red-50 dark:bg-red-950/30', icon: AlertTriangle },
        ].map((s) => (
          <button
            key={s.key}
            onClick={() => setFilterStatus(filterStatus === s.key ? 'all' : s.key as FilterStatus)}
            className={`flex items-center gap-3 rounded-lg border p-4 text-left transition-colors hover:border-primary/40 ${filterStatus === s.key ? 'border-primary ring-1 ring-primary/20' : ''}`}
          >
            <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${s.bg} ${s.cls}`}>
              <s.icon className="h-5 w-5" />
            </div>
            <div>
              <p className={`text-2xl font-bold leading-none ${s.cls}`}>{s.count}</p>
              <p className="text-xs text-muted-foreground mt-1">{s.label}</p>
            </div>
          </button>
        ))}
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-2 items-center">
        <Input
          className="h-8 w-52 text-xs"
          placeholder="Search evidence…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Select value={filterSource} onValueChange={(v) => setFilterSource(v as FilterSource)}>
          <SelectTrigger className="h-8 w-36 text-xs"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All sources</SelectItem>
            <SelectItem value="auto">Auto-collected</SelectItem>
            <SelectItem value="manual">Manual upload</SelectItem>
          </SelectContent>
        </Select>
        <Select value={filterType} onValueChange={setFilterType}>
          <SelectTrigger className="h-8 w-36 text-xs"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All types</SelectItem>
            <SelectItem value="inventory">Inventory JSON</SelectItem>
            <SelectItem value="findings">Findings JSON</SelectItem>
            <SelectItem value="report">Report</SelectItem>
            <SelectItem value="policy">Policy</SelectItem>
            <SelectItem value="screenshot">Screenshot</SelectItem>
            <SelectItem value="config">Config export</SelectItem>
            <SelectItem value="other">Other</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Evidence list */}
      {isLoading ? (
        <div className="space-y-2">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-16 rounded-lg bg-muted animate-pulse" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        items.length === 0 ? (
          <EmptyState
            icon={FolderOpen}
            title="No evidence yet"
            description="Evidence is auto-collected when audits complete, or upload files manually."
          />
        ) : (
          <EmptyState
            icon={FolderOpen}
            title="No evidence matches filters"
            description="Try adjusting your search or filter criteria."
          />
        )
      ) : (
        <Card>
          <CardContent className="p-0">
            <div className="divide-y divide-border">
              {filtered.map((item) => (
                <EvidenceRow
                  key={item.id}
                  item={item}
                  frameworks={frameworks}
                  onMap={() => setMappingItem(item)}
                  onDelete={() => {
                    if (confirm(`Delete "${item.name}"?`)) deleteMut.mutate(item.id)
                  }}
                />
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      <UploadDialog open={showUpload} onClose={() => setShowUpload(false)} />
      {mappingItem && <MappingDialog item={mappingItem} frameworks={frameworks} onClose={() => setMappingItem(null)} />}
    </div>
  )
}

// ── Evidence row ──────────────────────────────────────────────────────────────

function EvidenceRow({
  item,
  frameworks,
  onMap,
  onDelete,
}: {
  item: EvidenceItem
  frameworks: MappingFramework[]
  onMap: () => void
  onDelete: () => void
}) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div>
      <div
        className="flex items-center gap-4 px-5 py-3 hover:bg-muted/30 cursor-pointer"
        onClick={() => setExpanded((v) => !v)}
      >
        {/* Status icon */}
        <div className="shrink-0">{statusIcon(item.status)}</div>

        {/* Type icon */}
        <div className="shrink-0 text-muted-foreground">{typeIcon(item.evidence_type)}</div>

        {/* Name + meta */}
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium truncate">{item.name}</p>
          <p className="text-xs text-muted-foreground">
            {item.source === 'auto' ? 'Auto-collected' : 'Manual upload'}
            {item.job_name ? ` · ${item.job_name}` : ''}
            {' · '}{formatBytes(item.size)}
            {' · '}{item.evidence_type}
          </p>
        </div>

        {/* Mappings count */}
        <div className="shrink-0">
          {(item.mappings ?? []).length > 0 ? (
            <Badge variant="secondary" className="text-xs">
              <Link2 className="h-3 w-3 mr-1" />
              {(item.mappings ?? []).length} control{(item.mappings ?? []).length !== 1 ? 's' : ''}
            </Badge>
          ) : (
            <span className="text-xs text-muted-foreground">No mappings</span>
          )}
        </div>

        {/* Status badge */}
        <div className="shrink-0">{statusBadge(item.status)}</div>

        {/* Expires */}
        <div className="shrink-0 text-xs text-muted-foreground w-24 text-right">
          exp. {formatDate(item.expires_at)}
        </div>

        {/* Actions */}
        <div
          className="flex items-center gap-1 shrink-0"
          onClick={(e) => e.stopPropagation()}
        >
          <Button size="icon" variant="ghost" className="h-7 w-7" asChild>
            <a href={evidenceApi.downloadUrl(item.id)} download>
              <Download className="h-3.5 w-3.5" />
            </a>
          </Button>
          <Button size="icon" variant="ghost" className="h-7 w-7" onClick={onMap}>
            <Link2 className="h-3.5 w-3.5" />
          </Button>
          <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive" onClick={onDelete}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* Expanded: mappings + description */}
      {expanded && (
        <div className="px-14 pb-3 space-y-2 border-t bg-muted/20">
          {item.description && (
            <p className="text-xs text-muted-foreground pt-2">{item.description}</p>
          )}
          {(item.mappings ?? []).length > 0 && (
            <div className="flex flex-wrap gap-1.5 pt-1">
              {(item.mappings ?? []).map((m) => {
                const fwName = frameworks.find((f) => f.slug === m.framework_slug)?.name ?? m.framework_slug
                return (
                  <span key={m.id} className="inline-flex items-center gap-1 bg-background border text-xs rounded-full px-2.5 py-0.5">
                    <span className="font-medium text-muted-foreground">{fwName}</span>
                    <span className="text-muted-foreground">·</span>
                    <span className="font-mono">{m.ctrl_id}</span>
                  </span>
                )
              })}
            </div>
          )}
          {(item.mappings ?? []).length === 0 && (
            <p className="text-xs text-muted-foreground pt-2 italic">
              Not yet mapped to any compliance controls.{' '}
              <button className="underline hover:text-foreground" onClick={(e) => { e.stopPropagation(); onMap() }}>
                Add mapping
              </button>
            </p>
          )}
        </div>
      )}
    </div>
  )
}
