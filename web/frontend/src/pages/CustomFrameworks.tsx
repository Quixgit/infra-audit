import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Plus, Trash2, Edit2, ChevronDown, ChevronRight, Loader2,
  ShieldCheck, Download, Upload, Tag, BookOpen, X, Check,
} from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import { customFrameworksApi, type CustomFramework, type CustomControl } from '@/lib/api'

// ── Framework Form Dialog ─────────────────────────────────────────────────────

function FrameworkDialog({
  open,
  onClose,
  initial,
}: {
  open: boolean
  onClose: () => void
  initial?: CustomFramework
}) {
  const qc = useQueryClient()
  const isEdit = Boolean(initial)
  const [name, setName] = useState(initial?.name ?? '')
  const [version, setVersion] = useState(initial?.version ?? '1.0')
  const [description, setDescription] = useState(initial?.description ?? '')

  const mutation = useMutation({
    mutationFn: () =>
      isEdit
        ? customFrameworksApi.update(initial!.id, { name, version, description })
        : customFrameworksApi.create({ name, version, description }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['custom-frameworks'] })
      toast.success(isEdit ? 'Framework updated' : 'Framework created')
      onClose()
    },
    onError: (e: any) => {
      toast.error(e?.response?.data?.error ?? 'Failed to save framework')
    },
  })

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Framework' : 'New Custom Framework'}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>Framework name <span className="text-red-400">*</span></Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Internal Security Standard" />
          </div>
          <div className="space-y-1.5">
            <Label>Version</Label>
            <Input value={version} onChange={(e) => setVersion(e.target.value)} placeholder="1.0" className="w-32" />
          </div>
          <div className="space-y-1.5">
            <Label>Description</Label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Brief description of this framework…"
              rows={3}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button
            disabled={!name.trim() || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {mutation.isPending ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            {isEdit ? 'Save changes' : 'Create framework'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Control Form Dialog ───────────────────────────────────────────────────────

function ControlDialog({
  open,
  onClose,
  frameworkId,
  initial,
}: {
  open: boolean
  onClose: () => void
  frameworkId: string
  initial?: CustomControl
}) {
  const qc = useQueryClient()
  const isEdit = Boolean(initial)
  const [ctrlId, setCtrlId] = useState(initial?.ctrl_id ?? '')
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [category, setCategory] = useState(initial?.category ?? '')

  const mutation = useMutation({
    mutationFn: () =>
      isEdit
        ? customFrameworksApi.updateControl(frameworkId, initial!.id, { name, description, category })
        : customFrameworksApi.createControl(frameworkId, { ctrl_id: ctrlId, name, description, category }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['custom-framework', frameworkId] })
      toast.success(isEdit ? 'Control updated' : 'Control created')
      onClose()
    },
    onError: (e: any) => {
      toast.error(e?.response?.data?.error ?? 'Failed to save control')
    },
  })

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Control' : 'New Control'}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          {!isEdit && (
            <div className="space-y-1.5">
              <Label>Control ID <span className="text-red-400">*</span></Label>
              <Input value={ctrlId} onChange={(e) => setCtrlId(e.target.value)} placeholder="e.g. AC-1.1" className="font-mono" />
              <p className="text-[11px] text-muted-foreground/70">Must be unique within the framework.</p>
            </div>
          )}
          <div className="space-y-1.5">
            <Label>Name <span className="text-red-400">*</span></Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Access Control Policy" />
          </div>
          <div className="space-y-1.5">
            <Label>Category</Label>
            <Input value={category} onChange={(e) => setCategory(e.target.value)} placeholder="e.g. Access Control" />
          </div>
          <div className="space-y-1.5">
            <Label>Description</Label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What this control requires…"
              rows={3}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button
            disabled={!name.trim() || (!isEdit && !ctrlId.trim()) || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {mutation.isPending ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            {isEdit ? 'Save changes' : 'Add control'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Import Dialog ─────────────────────────────────────────────────────────────

function ImportDialog({
  open,
  onClose,
  frameworkId,
}: {
  open: boolean
  onClose: () => void
  frameworkId: string
}) {
  const qc = useQueryClient()
  const [jsonText, setJsonText] = useState('')
  const [preview, setPreview] = useState<{ ctrl_id: string; name: string }[] | null>(null)
  const [parseError, setParseError] = useState('')

  const handleParse = () => {
    setParseError('')
    try {
      const parsed = JSON.parse(jsonText)
      const controls = Array.isArray(parsed) ? parsed : parsed.controls
      if (!Array.isArray(controls)) {
        setParseError('Expected a JSON array or an object with a "controls" array')
        return
      }
      setPreview(controls.filter((c: any) => c.ctrl_id && c.name))
    } catch {
      setParseError('Invalid JSON. Please check the format.')
    }
  }

  const importMutation = useMutation({
    mutationFn: () => customFrameworksApi.importControls(frameworkId, preview!),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['custom-framework', frameworkId] })
      toast.success(`Imported ${data.imported} of ${data.total} controls`)
      onClose()
    },
    onError: (e: any) => {
      toast.error(e?.response?.data?.error ?? 'Import failed')
    },
  })

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Bulk Import Controls</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <p className="text-xs text-muted-foreground">
            Paste a JSON array of controls. Each item needs at least <code className="bg-muted px-1 rounded">ctrl_id</code> and <code className="bg-muted px-1 rounded">name</code>.
          </p>
          <div className="space-y-1.5">
            <Label>JSON payload</Label>
            <Textarea
              value={jsonText}
              onChange={(e) => { setJsonText(e.target.value); setPreview(null); setParseError('') }}
              placeholder={`[\n  {"ctrl_id":"AC-1","name":"Access Control","category":"Access","description":"..."}\n]`}
              rows={8}
              className="font-mono text-xs"
            />
            {parseError && <p className="text-xs text-red-400">{parseError}</p>}
          </div>
          {preview && (
            <div className="rounded-lg border bg-muted/20 p-3 space-y-1 max-h-40 overflow-y-auto">
              <p className="text-xs font-medium text-muted-foreground mb-2">{preview.length} controls parsed:</p>
              {preview.map((c) => (
                <div key={c.ctrl_id} className="flex items-center gap-2 text-xs">
                  <Check className="h-3 w-3 text-green-400 shrink-0" />
                  <code className="text-[10px] bg-muted px-1 rounded">{c.ctrl_id}</code>
                  <span className="text-muted-foreground truncate">{c.name}</span>
                </div>
              ))}
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          {!preview ? (
            <Button onClick={handleParse} disabled={!jsonText.trim()}>Parse JSON</Button>
          ) : (
            <Button
              onClick={() => importMutation.mutate()}
              disabled={importMutation.isPending || preview.length === 0}
            >
              {importMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
              Import {preview.length} controls
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Framework Detail ──────────────────────────────────────────────────────────

function FrameworkDetail({ framework, onBack }: { framework: CustomFramework; onBack: () => void }) {
  const qc = useQueryClient()
  const [showAddControl, setShowAddControl] = useState(false)
  const [editControl, setEditControl] = useState<CustomControl | null>(null)
  const [showImport, setShowImport] = useState(false)

  const { data: detail } = useQuery({
    queryKey: ['custom-framework', framework.id],
    queryFn: () => customFrameworksApi.get(framework.id),
  })

  const deleteControlMutation = useMutation({
    mutationFn: (controlId: string) => customFrameworksApi.deleteControl(framework.id, controlId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['custom-framework', framework.id] })
      toast.success('Control deleted')
    },
  })

  const controls = detail?.controls ?? []
  const categories = [...new Set(controls.map((c) => c.category).filter(Boolean))]

  const exportJSON = () => {
    const blob = new Blob([JSON.stringify(controls, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${framework.slug}-controls.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <button
            onClick={onBack}
            className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1 mb-2 transition-colors"
          >
            <ChevronRight className="h-3 w-3 rotate-180" />
            Back to frameworks
          </button>
          <div className="flex items-center gap-3">
            <h2 className="text-xl font-bold">{detail?.name ?? framework.name}</h2>
            <Badge variant="outline" className="font-mono text-xs">v{detail?.version ?? framework.version}</Badge>
          </div>
          {detail?.description && (
            <p className="text-sm text-muted-foreground mt-1">{detail.description}</p>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <Button variant="outline" size="sm" onClick={exportJSON}>
            <Download className="h-3.5 w-3.5 mr-1.5" />Export
          </Button>
          <Button variant="outline" size="sm" onClick={() => setShowImport(true)}>
            <Upload className="h-3.5 w-3.5 mr-1.5" />Import
          </Button>
          <Button size="sm" onClick={() => setShowAddControl(true)}>
            <Plus className="h-3.5 w-3.5 mr-1.5" />Add control
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3">
        {[
          { label: 'Total controls', value: controls.length },
          { label: 'Categories', value: categories.length },
          { label: 'Framework version', value: `v${detail?.version ?? framework.version}` },
        ].map(({ label, value }) => (
          <div key={label} className="rounded-xl border bg-card p-4">
            <p className="text-xs text-muted-foreground">{label}</p>
            <p className="text-2xl font-bold mt-1">{value}</p>
          </div>
        ))}
      </div>

      {/* Controls list */}
      <div className="rounded-xl border bg-card overflow-hidden">
        {controls.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <BookOpen className="h-8 w-8 text-muted-foreground/30 mb-3" />
            <p className="text-sm font-medium text-muted-foreground">No controls yet</p>
            <p className="text-xs text-muted-foreground/60 mt-1 mb-4">Add controls manually or import from JSON</p>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => setShowAddControl(true)}>
                <Plus className="h-3.5 w-3.5 mr-1.5" />Add control
              </Button>
              <Button variant="outline" size="sm" onClick={() => setShowImport(true)}>
                <Upload className="h-3.5 w-3.5 mr-1.5" />Import JSON
              </Button>
            </div>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/30">
              <tr>
                <th className="py-2.5 px-4 text-left text-xs font-medium text-muted-foreground w-28">ID</th>
                <th className="py-2.5 px-4 text-left text-xs font-medium text-muted-foreground">Name</th>
                <th className="py-2.5 px-4 text-left text-xs font-medium text-muted-foreground w-36 hidden sm:table-cell">Category</th>
                <th className="py-2.5 px-4 text-right text-xs font-medium text-muted-foreground w-20">Actions</th>
              </tr>
            </thead>
            <tbody>
              {controls.map((control) => (
                <tr key={control.id} className="border-b last:border-0 hover:bg-muted/20 transition-colors">
                  <td className="py-2.5 px-4">
                    <code className="text-xs bg-muted px-1.5 py-0.5 rounded font-mono">{control.ctrl_id}</code>
                  </td>
                  <td className="py-2.5 px-4">
                    <p className="font-medium text-sm leading-snug">{control.name}</p>
                    {control.description && (
                      <p className="text-xs text-muted-foreground/70 mt-0.5 line-clamp-1">{control.description}</p>
                    )}
                  </td>
                  <td className="py-2.5 px-4 hidden sm:table-cell">
                    {control.category && (
                      <Badge variant="outline" className="text-[10px]">{control.category}</Badge>
                    )}
                  </td>
                  <td className="py-2.5 px-4">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost" size="icon"
                        className="h-7 w-7 text-muted-foreground hover:text-foreground"
                        onClick={() => setEditControl(control)}
                      >
                        <Edit2 className="h-3 w-3" />
                      </Button>
                      <Button
                        variant="ghost" size="icon"
                        className="h-7 w-7 text-muted-foreground hover:text-red-400"
                        onClick={() => {
                          if (confirm(`Delete control "${control.ctrl_id}"?`)) {
                            deleteControlMutation.mutate(control.id)
                          }
                        }}
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {showAddControl && (
        <ControlDialog
          open
          onClose={() => setShowAddControl(false)}
          frameworkId={framework.id}
        />
      )}
      {editControl && (
        <ControlDialog
          open
          onClose={() => setEditControl(null)}
          frameworkId={framework.id}
          initial={editControl}
        />
      )}
      {showImport && (
        <ImportDialog
          open
          onClose={() => setShowImport(false)}
          frameworkId={framework.id}
        />
      )}
    </div>
  )
}

// ── Main Page ─────────────────────────────────────────────────────────────────

export function CustomFrameworks() {
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [editFramework, setEditFramework] = useState<CustomFramework | null>(null)
  const [selected, setSelected] = useState<CustomFramework | null>(null)

  const { data: frameworks, isLoading } = useQuery({
    queryKey: ['custom-frameworks'],
    queryFn: customFrameworksApi.list,
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => customFrameworksApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['custom-frameworks'] })
      toast.success('Framework deleted')
    },
  })

  if (selected) {
    return <FrameworkDetail framework={selected} onBack={() => setSelected(null)} />
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">Custom Frameworks</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Define your own compliance frameworks and map controls to your specific requirements.
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="h-4 w-4 mr-2" />New framework
        </Button>
      </div>

      {/* Framework list */}
      {isLoading ? (
        <div className="flex h-48 items-center justify-center">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      ) : !frameworks?.length ? (
        <div className="flex flex-col items-center justify-center rounded-xl border bg-card py-20 text-center">
          <ShieldCheck className="h-10 w-10 text-muted-foreground/30 mb-4" />
          <p className="font-medium text-muted-foreground">No custom frameworks yet</p>
          <p className="text-xs text-muted-foreground/60 mt-1 mb-6 max-w-sm">
            Create a framework to define your own controls and compliance requirements for your organization.
          </p>
          <Button onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4 mr-2" />Create your first framework
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {frameworks.map((fw) => (
            <div
              key={fw.id}
              className="group rounded-xl border bg-card p-5 hover:border-foreground/20 transition-all cursor-pointer"
              onClick={() => setSelected(fw)}
            >
              <div className="flex items-start justify-between mb-3">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-indigo-500/10">
                    <ShieldCheck className="h-4 w-4 text-indigo-400" />
                  </div>
                  <div>
                    <p className="font-semibold text-sm leading-tight">{fw.name}</p>
                    <div className="flex items-center gap-1.5 mt-0.5">
                      <Badge variant="outline" className="font-mono text-[10px] py-0">v{fw.version}</Badge>
                      {fw.slug && (
                        <span className="text-[10px] text-muted-foreground/50 font-mono">{fw.slug}</span>
                      )}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity" onClick={(e) => e.stopPropagation()}>
                  <Button
                    variant="ghost" size="icon"
                    className="h-7 w-7 text-muted-foreground hover:text-foreground"
                    onClick={() => setEditFramework(fw)}
                  >
                    <Edit2 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost" size="icon"
                    className="h-7 w-7 text-muted-foreground hover:text-red-400"
                    onClick={() => {
                      if (confirm(`Delete framework "${fw.name}" and all its controls?`)) {
                        deleteMutation.mutate(fw.id)
                      }
                    }}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>

              {fw.description && (
                <p className="text-xs text-muted-foreground/70 line-clamp-2 mb-3">{fw.description}</p>
              )}

              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span className="flex items-center gap-1">
                  <Tag className="h-3 w-3" />
                  {fw.controls?.length ?? 0} controls
                </span>
                <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/40 group-hover:text-muted-foreground transition-colors" />
              </div>
            </div>
          ))}
        </div>
      )}

      {showCreate && (
        <FrameworkDialog open onClose={() => setShowCreate(false)} />
      )}
      {editFramework && (
        <FrameworkDialog open onClose={() => setEditFramework(null)} initial={editFramework} />
      )}
    </div>
  )
}
