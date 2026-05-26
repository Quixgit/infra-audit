import { useState, useEffect } from 'react'
import { useNavigate, useParams, useSearchParams, Link } from 'react-router-dom'
import {
  ArrowLeft, Loader2, CheckCircle2, XCircle, Cloud, Code2, Globe, Lock,
  AlertTriangle, Shield, GitBranch, FolderOpen, Key, Tag, Database,
  ChevronRight, Info,
} from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { connectionsApi, licenseApi, type ConnectionFormData } from '@/lib/api'

type ConnType = 'do' | 'code' | 'ssl' | 'dns' | 'aws'

const defaultForm: ConnectionFormData = {
  conn_type: 'do',
  name: '',
  do_token: '',
  project_id: '',
  scope_mode: 'project',
  spaces_buckets: '',
  repo_source: 'git',
  repo_url: '',
  repo_token: '',
  repo_branch: '',
  repo_local_path: '',
  domains: '',
  aws_access_key_id: '',
  aws_secret_key: '',
  aws_region: 'us-east-1',
  github_webhook_secret: '',
  github_repo_url: '',
}

type TestState = 'idle' | 'loading' | 'ok' | 'error'

const connectionListPath = (connType: ConnType) =>
  connType === 'code' ? '/code-iac' : '/cloud-audits'

const AWS_REGIONS = [
  'us-east-1', 'us-east-2', 'us-west-1', 'us-west-2',
  'eu-west-1', 'eu-west-2', 'eu-west-3', 'eu-central-1', 'eu-north-1',
  'ap-northeast-1', 'ap-northeast-2', 'ap-southeast-1', 'ap-southeast-2',
  'ap-south-1', 'sa-east-1', 'ca-central-1', 'me-south-1', 'af-south-1',
]

interface TypeMeta {
  type: ConnType
  label: string
  sublabel: string
  icon: React.ElementType
  iconBg: string
  iconColor: string
  activeBorder: string
  activeBg: string
  badgeColor: string
  requiresFeature?: string
}

const typeMeta: TypeMeta[] = [
  {
    type: 'do',
    label: 'DigitalOcean',
    sublabel: 'Infrastructure security audit',
    icon: Cloud,
    iconBg: 'bg-indigo-500/20',
    iconColor: 'text-indigo-400',
    activeBorder: 'border-indigo-500/60',
    activeBg: 'bg-indigo-500/8',
    badgeColor: 'bg-indigo-500/15 text-indigo-400',
  },
  {
    type: 'code',
    label: 'Code & IaC',
    sublabel: 'SAST, secrets & Terraform',
    icon: Code2,
    iconBg: 'bg-purple-500/20',
    iconColor: 'text-purple-400',
    activeBorder: 'border-purple-500/60',
    activeBg: 'bg-purple-500/8',
    badgeColor: 'bg-purple-500/15 text-purple-400',
    requiresFeature: 'code_audit',
  },
  {
    type: 'ssl',
    label: 'SSL / TLS',
    sublabel: 'Certificate & cipher audit',
    icon: Lock,
    iconBg: 'bg-blue-500/20',
    iconColor: 'text-blue-400',
    activeBorder: 'border-blue-500/60',
    activeBg: 'bg-blue-500/8',
    badgeColor: 'bg-blue-500/15 text-blue-400',
  },
  {
    type: 'dns',
    label: 'DNS Security',
    sublabel: 'SPF, DMARC & zone transfer',
    icon: Globe,
    iconBg: 'bg-orange-500/20',
    iconColor: 'text-orange-400',
    activeBorder: 'border-orange-500/60',
    activeBg: 'bg-orange-500/8',
    badgeColor: 'bg-orange-500/15 text-orange-400',
  },
  {
    type: 'aws' as ConnType,
    label: 'AWS',
    sublabel: 'CIS AWS Foundations audit',
    icon: Shield,
    iconBg: 'bg-yellow-500/20',
    iconColor: 'text-yellow-400',
    activeBorder: 'border-yellow-500/60',
    activeBg: 'bg-yellow-500/8',
    badgeColor: 'bg-yellow-500/15 text-yellow-400',
    requiresFeature: 'aws_audit',
  },
]

// ── Section label ─────────────────────────────────────────────────────────────

function SectionLabel({ icon: Icon, label }: { icon: React.ElementType; label: string }) {
  return (
    <div className="flex items-center gap-2 mb-4">
      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-muted">
        <Icon className="h-3 w-3 text-muted-foreground" />
      </span>
      <span className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/60">
        {label}
      </span>
      <div className="flex-1 h-px bg-border/50" />
    </div>
  )
}

// ── Hint box ─────────────────────────────────────────────────────────────────

function HintBox({
  variant = 'info',
  children,
}: {
  variant?: 'info' | 'warn' | 'success'
  children: React.ReactNode
}) {
  const styles = {
    info:    { wrap: 'border-indigo-500/20 bg-indigo-500/5',  icon: 'text-indigo-400',  Icon: Info },
    warn:    { wrap: 'border-amber-500/25 bg-amber-500/8',    icon: 'text-amber-400',   Icon: AlertTriangle },
    success: { wrap: 'border-green-500/25 bg-green-500/8',    icon: 'text-green-400',   Icon: CheckCircle2 },
  }
  const { wrap, icon, Icon } = styles[variant]
  return (
    <div className={cn('flex items-start gap-3 rounded-xl border p-3.5 text-xs', wrap)}>
      <Icon className={cn('mt-0.5 h-3.5 w-3.5 shrink-0', icon)} />
      <div className="leading-relaxed text-muted-foreground">{children}</div>
    </div>
  )
}

// ── Type picker ───────────────────────────────────────────────────────────────

function TypePicker({
  active, hasCodeAudit, hasAWSAudit, onChange,
}: {
  active: ConnType
  hasCodeAudit: boolean
  hasAWSAudit: boolean
  onChange: (t: ConnType) => void
}) {
  return (
    <div className="grid grid-cols-2 gap-2.5">
      {typeMeta.map((m) => {
        const locked =
          (m.requiresFeature === 'code_audit' && !hasCodeAudit) ||
          (m.requiresFeature === 'aws_audit' && !hasAWSAudit)
        const isActive = active === m.type
        const Icon = m.icon
        return (
          <button
            key={m.type}
            type="button"
            onClick={() => {
              if (locked) {
                if (m.requiresFeature === 'aws_audit') {
                  toast.error('AWS audit requires a Professional plan or higher')
                } else {
                  toast.error('Code Security audit requires a Professional plan or higher')
                }
                return
              }
              onChange(m.type)
            }}
            className={cn(
              'group relative flex items-center gap-3 rounded-xl border p-3.5 text-left transition-all duration-150',
              isActive
                ? `${m.activeBorder} ${m.activeBg} shadow-sm`
                : 'border-border bg-card/40 hover:border-border/80 hover:bg-card',
              locked && 'opacity-50 cursor-not-allowed'
            )}
          >
            {/* Active dot */}
            {isActive && (
              <span className="absolute right-3 top-3 h-1.5 w-1.5 rounded-full bg-current opacity-60" />
            )}
            <div className={cn(
              'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors',
              isActive ? m.iconBg : 'bg-muted'
            )}>
              <Icon className={cn('h-4 w-4', isActive ? m.iconColor : 'text-muted-foreground')} />
            </div>
            <div className="min-w-0 flex-1">
              <p className={cn('text-sm font-semibold leading-tight', isActive ? 'text-foreground' : 'text-foreground/70')}>
                {m.label}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5 leading-tight truncate">{m.sublabel}</p>
            </div>
            {locked && (
              <span className="ml-auto shrink-0 rounded-full border border-yellow-500/30 bg-yellow-500/10 px-1.5 py-0.5 text-[9px] font-semibold text-yellow-400 leading-none">
                PRO
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export function ConnectionForm() {
  const navigate = useNavigate()
  const { id } = useParams<{ id?: string }>()
  const [searchParams] = useSearchParams()
  const isEdit = Boolean(id)
  const qc = useQueryClient()

  const rawType = searchParams.get('type') as ConnType | null
  const initialType: ConnType =
    rawType && ['do', 'code', 'ssl', 'dns', 'aws'].includes(rawType) ? rawType : 'do'
  const fromAuditTypes = searchParams.has('type')

  const [form, setForm] = useState<ConnectionFormData>({ ...defaultForm, conn_type: initialType })
  const [testState, setTestState] = useState<TestState>('idle')
  const [testMsg, setTestMsg] = useState('')

  const { data: license } = useQuery({ queryKey: ['license'], queryFn: licenseApi.get })
  const hasCodeAudit = license?.features?.includes('code_audit') ?? false
  const hasAWSAudit = license?.features?.includes('aws_audit') ?? false

  const { data: existing } = useQuery({
    queryKey: ['connections'],
    queryFn: connectionsApi.list,
    enabled: isEdit,
  })

  useEffect(() => {
    if (isEdit && existing) {
      const conn = existing.find((c) => c.id === id)
      if (conn) {
        setForm({
          conn_type: conn.conn_type ?? 'do',
          name: conn.name,
          do_token: '',
          project_id: conn.project_id ?? '',
          scope_mode: conn.scope_mode ?? 'project',
          spaces_buckets: conn.spaces_buckets ?? '',
          repo_source: conn.repo_source ?? 'git',
          repo_url: conn.repo_url ?? '',
          repo_token: '',
          repo_branch: conn.repo_branch ?? '',
          repo_local_path: conn.repo_local_path ?? '',
          domains: conn.domains ?? '',
          aws_access_key_id: '',
          aws_secret_key: '',
          aws_region: conn.aws_region ?? 'us-east-1',
          github_webhook_secret: '',
          github_repo_url: conn.github_repo_url ?? '',
        })
      }
    }
  }, [isEdit, existing, id])

  const saveMutation = useMutation({
    mutationFn: () =>
      isEdit ? connectionsApi.update(id!, form) : connectionsApi.create(form),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['connections'] })
      toast.success(isEdit ? 'Connection updated' : 'Connection created')
      navigate(connectionListPath(form.conn_type))
    },
    onError: (e: unknown) => {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error
      toast.error(msg || 'Failed to save connection')
    },
  })

  const testMutation = useMutation({
    mutationFn: async () => {
      if (form.conn_type === 'code') {
        if (form.repo_source === 'git') {
          return connectionsApi.testGit({
            repo_url: form.repo_url,
            repo_token: form.repo_token,
            repo_branch: form.repo_branch,
          })
        } else {
          return connectionsApi.testLocal({ repo_local_path: form.repo_local_path })
        }
      } else if (form.conn_type === 'do' || form.conn_type === 'aws') {
        if (!id) return { ok: true, message: 'Save first to test' }
        const result = await connectionsApi.test(id)
        return {
          ok: result.status === 'ok',
          message: (result as any).message || result.status,
        }
      }
      return { ok: true, message: 'No test available for this type' }
    },
    onMutate: () => { setTestState('loading'); setTestMsg('') },
    onSuccess: (data) => { setTestState(data.ok ? 'ok' : 'error'); setTestMsg(data.message ?? '') },
    onError: (e: unknown) => {
      setTestState('error')
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error
      setTestMsg(msg || 'Connection test failed')
    },
  })

  const set = <K extends keyof ConnectionFormData>(key: K, value: ConnectionFormData[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  const switchType = (t: ConnType) => {
    setForm((f) => ({ ...f, conn_type: t }))
    setTestState('idle')
    setTestMsg('')
  }

  const isNetworkScan = form.conn_type === 'ssl' || form.conn_type === 'dns'
  const isAWS = form.conn_type === 'aws'
  const activeTypeMeta = typeMeta.find((m) => m.type === form.conn_type) ?? typeMeta[0]!

  const canTestNow =
    form.conn_type === 'code'
      ? form.repo_source === 'git' ? Boolean(form.repo_url) : Boolean(form.repo_local_path)
      : form.conn_type === 'do' || form.conn_type === 'aws'
      ? isEdit
      : false

  const backPath =
    fromAuditTypes && !isEdit ? '/audit-types' : connectionListPath(form.conn_type)

  return (
    <div className="max-w-2xl">

      {/* ── Page header ── */}
      <div className="mb-6 flex items-center gap-3">
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
          onClick={() => navigate(backPath)}
        >
          <ArrowLeft className="h-4 w-4" />
        </Button>

        <div className="flex-1 min-w-0">
          {/* Breadcrumb */}
          <div className="flex items-center gap-1 text-[11px] text-muted-foreground/60 mb-0.5">
            <span
              className="hover:text-muted-foreground cursor-pointer transition-colors"
              onClick={() => navigate(backPath)}
            >
              {fromAuditTypes && !isEdit ? 'Audit Types' : activeTypeMeta.label}
            </span>
            <ChevronRight className="h-3 w-3" />
            <span className="text-muted-foreground">{isEdit ? 'Edit' : 'New'}</span>
          </div>
          <h1 className="text-xl font-bold leading-tight">
            {isEdit ? 'Edit Connection' : 'New Connection'}
          </h1>
        </div>

        {/* Active type badge (edit mode) */}
        {isEdit && (
          <div className={cn(
            'hidden sm:flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium',
            activeTypeMeta.activeBorder, activeTypeMeta.activeBg
          )}>
            <activeTypeMeta.icon className={cn('h-3 w-3', activeTypeMeta.iconColor)} />
            <span className={activeTypeMeta.iconColor}>{activeTypeMeta.label}</span>
          </div>
        )}
      </div>

      {/* ── Type picker (new connections) ── */}
      {!isEdit && (
        <div className="mb-6">
          <p className="text-xs text-muted-foreground mb-3 font-medium uppercase tracking-wide">
            Connection type
          </p>
          <TypePicker
            active={form.conn_type}
            hasCodeAudit={hasCodeAudit}
            hasAWSAudit={hasAWSAudit}
            onChange={switchType}
          />
        </div>
      )}

      {/* ── Form card ── */}
      <div className="rounded-xl border bg-card">
        <form onSubmit={(e) => { e.preventDefault(); saveMutation.mutate() }}>
          <div className="p-5 space-y-6">

            {/* ── General ── */}
            <div className="space-y-4">
              <SectionLabel icon={Tag} label="Connection details" />
              <div className="space-y-1.5">
                <Label htmlFor="name" className="text-xs font-medium">
                  Connection name <span className="text-red-400">*</span>
                </Label>
                <Input
                  id="name"
                  placeholder={
                    form.conn_type === 'ssl' ? 'e.g. Company SSL scan' :
                    form.conn_type === 'dns' ? 'e.g. Company DNS audit' :
                    form.conn_type === 'code' ? 'e.g. Main repo scan' :
                    'e.g. Production DO account'
                  }
                  value={form.name}
                  onChange={(e) => set('name', e.target.value)}
                  required
                  className="h-9"
                />
                <p className="text-[11px] text-muted-foreground/70">
                  Shown in reports and the audit list.
                </p>
              </div>
            </div>

            {/* ── DigitalOcean fields ── */}
            {form.conn_type === 'do' && (
              <>
                {/* Authentication */}
                <div className="space-y-4">
                  <SectionLabel icon={Key} label="Authentication" />

                  <div className="space-y-1.5">
                    <Label htmlFor="do_token" className="text-xs font-medium">
                      DigitalOcean API token <span className="text-red-400">*</span>
                    </Label>
                    <Input
                      id="do_token"
                      type="password"
                      placeholder={isEdit ? '(unchanged — leave blank to keep)' : 'dop_v1_…'}
                      value={form.do_token}
                      onChange={(e) => set('do_token', e.target.value)}
                      required={!isEdit}
                      className="h-9 font-mono text-sm"
                    />
                    <p className="text-[11px] text-muted-foreground/70">
                      Read-only token from{' '}
                      <a
                        href="https://cloud.digitalocean.com/account/api/tokens"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="underline hover:text-foreground transition-colors"
                      >
                        DigitalOcean API settings
                      </a>
                      .
                    </p>
                  </div>
                </div>

                {/* Scope */}
                <div className="space-y-4">
                  <SectionLabel icon={Database} label="Scope & targeting" />

                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1.5">
                      <Label htmlFor="project_id" className="text-xs font-medium">
                        Project ID
                        <span className="text-muted-foreground font-normal ml-1">(optional)</span>
                      </Label>
                      <Input
                        id="project_id"
                        placeholder="xxxxxxxx-xxxx-…"
                        value={form.project_id}
                        onChange={(e) => set('project_id', e.target.value)}
                        className="h-9 font-mono text-xs"
                      />
                      <p className="text-[11px] text-muted-foreground/70">
                        Leave empty to scan all projects.
                      </p>
                    </div>

                    <div className="space-y-1.5">
                      <Label className="text-xs font-medium">Scan scope</Label>
                      <Select value={form.scope_mode} onValueChange={(v) => set('scope_mode', v)}>
                        <SelectTrigger className="h-9 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="project" className="text-xs">
                            <div>
                              <p className="font-medium">Project</p>
                              <p className="text-muted-foreground">Resources assigned to project</p>
                            </div>
                          </SelectItem>
                          <SelectItem value="hybrid" className="text-xs">
                            <div>
                              <p className="font-medium">Hybrid</p>
                              <p className="text-muted-foreground">Project + related resources</p>
                            </div>
                          </SelectItem>
                          <SelectItem value="account" className="text-xs">
                            <div>
                              <p className="font-medium">Account</p>
                              <p className="text-muted-foreground">Full account scan</p>
                            </div>
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="spaces_buckets" className="text-xs font-medium">
                      Spaces buckets
                      <span className="text-muted-foreground font-normal ml-1">(optional)</span>
                    </Label>
                    <Input
                      id="spaces_buckets"
                      placeholder="bucket1:nyc3:sensitive,bucket2:tor1"
                      value={form.spaces_buckets}
                      onChange={(e) => set('spaces_buckets', e.target.value)}
                      className="h-9 font-mono text-xs"
                    />
                    <p className="text-[11px] text-muted-foreground/70">
                      Comma-separated: <code className="bg-muted px-1 rounded text-[10px]">name:region[:sensitive]</code>
                    </p>
                  </div>

                  {/* Scope info */}
                  <HintBox variant="info">
                    <p className="font-medium text-foreground/80 mb-1">Scope guide</p>
                    <ul className="space-y-1">
                      <li><span className="font-medium text-foreground/70">Project</span> — audits only Droplets, databases and other resources explicitly assigned to the project.</li>
                      <li><span className="font-medium text-foreground/70">Hybrid</span> — project resources plus firewall rules, load balancers and VPCs that reference them.</li>
                      <li><span className="font-medium text-foreground/70">Account</span> — scans everything in your DigitalOcean account regardless of project.</li>
                    </ul>
                  </HintBox>
                </div>
              </>
            )}

            {/* ── Code fields ── */}
            {form.conn_type === 'code' && (
              <>
                <div className="space-y-4">
                  <SectionLabel icon={GitBranch} label="Repository source" />

                  {/* Source toggle */}
                  <div className="flex gap-2 p-1 bg-muted/50 rounded-lg">
                    {(['git', 'local'] as const).map((src) => (
                      <button
                        key={src}
                        type="button"
                        onClick={() => { set('repo_source', src); setTestState('idle'); setTestMsg('') }}
                        className={cn(
                          'flex-1 flex items-center justify-center gap-2 rounded-md py-2 text-sm font-medium transition-all',
                          form.repo_source === src
                            ? 'bg-card text-foreground shadow-sm border border-border'
                            : 'text-muted-foreground hover:text-foreground'
                        )}
                      >
                        {src === 'git'
                          ? <><GitBranch className="h-3.5 w-3.5" />Git repository</>
                          : <><FolderOpen className="h-3.5 w-3.5" />Local path</>
                        }
                      </button>
                    ))}
                  </div>

                  {form.repo_source === 'git' ? (
                    <div className="space-y-4">
                      <div className="space-y-1.5">
                        <Label htmlFor="repo_url" className="text-xs font-medium">
                          Repository URL <span className="text-red-400">*</span>
                        </Label>
                        <Input
                          id="repo_url"
                          placeholder="https://github.com/org/repo.git"
                          value={form.repo_url}
                          onChange={(e) => { set('repo_url', e.target.value); setTestState('idle') }}
                          required={form.conn_type === 'code' && form.repo_source === 'git'}
                          className="h-9 font-mono text-sm"
                        />
                      </div>

                      <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-1.5">
                          <Label htmlFor="repo_token" className="text-xs font-medium">
                            Access token
                            <span className="text-muted-foreground font-normal ml-1">(optional)</span>
                          </Label>
                          <Input
                            id="repo_token"
                            type="password"
                            placeholder={isEdit ? '(unchanged)' : 'ghp_… or GitLab PAT'}
                            value={form.repo_token}
                            onChange={(e) => { set('repo_token', e.target.value); setTestState('idle') }}
                            className="h-9 font-mono text-sm"
                          />
                          <p className="text-[11px] text-muted-foreground/70">Leave empty for public repos.</p>
                        </div>

                        <div className="space-y-1.5">
                          <Label htmlFor="repo_branch" className="text-xs font-medium">
                            Branch
                            <span className="text-muted-foreground font-normal ml-1">(optional)</span>
                          </Label>
                          <Input
                            id="repo_branch"
                            placeholder="main"
                            value={form.repo_branch}
                            onChange={(e) => { set('repo_branch', e.target.value); setTestState('idle') }}
                            className="h-9"
                          />
                          <p className="text-[11px] text-muted-foreground/70">Defaults to default branch.</p>
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="space-y-4">
                      <div className="space-y-1.5">
                        <Label htmlFor="repo_local_path" className="text-xs font-medium">
                          Local path <span className="text-red-400">*</span>
                        </Label>
                        <Input
                          id="repo_local_path"
                          placeholder="/home/user/myproject"
                          value={form.repo_local_path}
                          onChange={(e) => { set('repo_local_path', e.target.value); setTestState('idle') }}
                          required={form.conn_type === 'code' && form.repo_source === 'local'}
                          className="h-9 font-mono text-sm"
                        />
                      </div>
                      <HintBox variant="warn">
                        The path must be accessible from the <strong>server</strong> running the audit backend, not your local machine.
                      </HintBox>
                    </div>
                  )}
                </div>

                {/* What we scan */}
                <HintBox variant="info">
                  <p className="font-medium text-foreground/80 mb-1.5">Scanners included</p>
                  <div className="grid grid-cols-2 gap-x-4 gap-y-1">
                    {[
                      ['Trivy', 'Secrets & CVEs'],
                      ['Semgrep', 'SAST (100+ rules)'],
                      ['Checkov', 'Terraform / IaC'],
                      ['Gitleaks', 'Git history secrets'],
                    ].map(([tool, desc]) => (
                      <div key={tool} className="flex items-center gap-1.5">
                        <span className="h-1 w-1 rounded-full bg-purple-400/60 shrink-0" />
                        <span className="font-medium text-foreground/70">{tool}</span>
                        <span className="text-muted-foreground/70">— {desc}</span>
                      </div>
                    ))}
                  </div>
                </HintBox>
              </>
            )}

            {/* ── SSL / TLS & DNS fields ── */}
            {isNetworkScan && (
              <>
                <div className="space-y-4">
                  <SectionLabel icon={Globe} label="Target domains" />

                  <div className="space-y-1.5">
                    <Label htmlFor="domains" className="text-xs font-medium">
                      {form.conn_type === 'ssl' ? 'Domains to scan' : 'Domains to audit'}
                      {' '}<span className="text-red-400">*</span>
                    </Label>
                    <Input
                      id="domains"
                      placeholder="example.com, app.example.com, api.example.com"
                      value={form.domains}
                      onChange={(e) => set('domains', e.target.value)}
                      required
                      className="h-9"
                    />
                    <p className="text-[11px] text-muted-foreground/70">
                      {form.conn_type === 'ssl'
                        ? 'Comma-separated. Each domain is checked on port 443.'
                        : 'Comma-separated domain names (no https://).'}
                    </p>
                  </div>
                </div>

                {/* What we check */}
                <div className={cn('rounded-xl border p-4', activeTypeMeta.activeBorder, activeTypeMeta.activeBg)}>
                  <div className="flex items-center gap-2 mb-3">
                    <Shield className={cn('h-4 w-4', activeTypeMeta.iconColor)} />
                    <p className="text-sm font-semibold">
                      {form.conn_type === 'ssl' ? 'What we check' : 'What we audit'}
                    </p>
                  </div>
                  <div className="grid grid-cols-2 gap-x-4 gap-y-1.5">
                    {(form.conn_type === 'ssl' ? [
                      'Certificate validity & expiry',
                      'TLS 1.0 / 1.1 detection',
                      'Weak ciphers (RC4, 3DES)',
                      'Missing HSTS header',
                      'HTTP → HTTPS redirect',
                      'Certificate chain errors',
                    ] : [
                      'SPF record & policy',
                      'DMARC policy (p=none/reject)',
                      'DKIM selectors',
                      'Zone transfer (AXFR)',
                      'Open resolver check',
                      'CAA records',
                    ]).map((item) => (
                      <div key={item} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                        <CheckCircle2 className={cn('h-3 w-3 shrink-0', activeTypeMeta.iconColor)} />
                        {item}
                      </div>
                    ))}
                  </div>
                </div>
              </>
            )}

            {/* ── AWS fields ── */}
            {isAWS && (
              <>
                <div className="space-y-4">
                  <SectionLabel icon={Key} label="AWS credentials" />

                  <HintBox variant="warn">
                    Use a dedicated IAM user with <strong>read-only</strong> permissions (SecurityAudit + ReadOnlyAccess policies). Never use root credentials.
                  </HintBox>

                  <div className="space-y-1.5">
                    <Label htmlFor="aws_access_key_id" className="text-xs font-medium">
                      Access Key ID <span className="text-red-400">*</span>
                    </Label>
                    <Input
                      id="aws_access_key_id"
                      placeholder={isEdit ? '(unchanged — leave blank to keep)' : 'AKIAIOSFODNN7EXAMPLE'}
                      value={form.aws_access_key_id ?? ''}
                      onChange={(e) => set('aws_access_key_id', e.target.value)}
                      required={!isEdit}
                      className="h-9 font-mono text-sm"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="aws_secret_key" className="text-xs font-medium">
                      Secret Access Key <span className="text-red-400">*</span>
                    </Label>
                    <Input
                      id="aws_secret_key"
                      type="password"
                      placeholder={isEdit ? '(unchanged — leave blank to keep)' : 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'}
                      value={form.aws_secret_key ?? ''}
                      onChange={(e) => set('aws_secret_key', e.target.value)}
                      required={!isEdit}
                      className="h-9 font-mono text-sm"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="aws_region" className="text-xs font-medium">
                      Primary region <span className="text-red-400">*</span>
                    </Label>
                    <Select value={form.aws_region ?? 'us-east-1'} onValueChange={(v) => set('aws_region', v)}>
                      <SelectTrigger className="h-9 text-xs font-mono">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {AWS_REGIONS.map((r) => (
                          <SelectItem key={r} value={r} className="text-xs font-mono">{r}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <p className="text-[11px] text-muted-foreground/70">
                      The scanner will also check global resources (IAM, S3) regardless of region.
                    </p>
                  </div>
                </div>

                <div className="space-y-4">
                  <SectionLabel icon={GitBranch} label="GitHub webhook (optional)" />
                  <p className="text-xs text-muted-foreground/70">
                    Automatically trigger an audit when code is pushed to your repo. Leave blank to skip.
                  </p>
                  <div className="space-y-1.5">
                    <Label htmlFor="github_repo_url" className="text-xs font-medium">
                      Repository URL
                    </Label>
                    <Input
                      id="github_repo_url"
                      placeholder="https://github.com/org/repo"
                      value={form.github_repo_url ?? ''}
                      onChange={(e) => set('github_repo_url', e.target.value)}
                      className="h-9 font-mono text-sm"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="github_webhook_secret" className="text-xs font-medium">
                      Webhook secret
                    </Label>
                    <Input
                      id="github_webhook_secret"
                      type="password"
                      placeholder={isEdit ? '(unchanged — leave blank to keep)' : 'your-webhook-secret'}
                      value={form.github_webhook_secret ?? ''}
                      onChange={(e) => set('github_webhook_secret', e.target.value)}
                      className="h-9 font-mono text-sm"
                    />
                    <p className="text-[11px] text-muted-foreground/70">
                      Set this as the secret in GitHub webhook settings. Used to verify request authenticity.
                    </p>
                  </div>
                </div>

                <HintBox variant="info">
                  <p className="font-medium text-foreground/80 mb-1.5">CIS AWS Foundations checks</p>
                  <div className="grid grid-cols-2 gap-x-4 gap-y-1">
                    {[
                      ['EC2', 'IMDSv2, public exposure'],
                      ['S3', 'Public access, encryption'],
                      ['IAM', 'MFA, access key age'],
                      ['RDS', 'Public access, encryption'],
                      ['Security Groups', 'Open ports to 0.0.0.0/0'],
                      ['VPC', 'Flow logs, defaults'],
                    ].map(([svc, desc]) => (
                      <div key={svc} className="flex items-center gap-1.5">
                        <span className="h-1 w-1 rounded-full bg-yellow-400/60 shrink-0" />
                        <span className="font-medium text-foreground/70">{svc}</span>
                        <span className="text-muted-foreground/70">— {desc}</span>
                      </div>
                    ))}
                  </div>
                </HintBox>
              </>
            )}

            {/* ── Test result banner ── */}
            {testState !== 'idle' && testMsg && (
              <div className={cn(
                'flex items-start gap-2.5 rounded-xl border p-3.5 text-sm',
                testState === 'ok'
                  ? 'border-green-500/30 bg-green-500/8 text-green-300'
                  : testState === 'error'
                  ? 'border-red-500/30 bg-red-500/8 text-red-300'
                  : 'border-border bg-muted/60 text-muted-foreground'
              )}>
                {testState === 'ok'      && <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-green-400" />}
                {testState === 'error'   && <XCircle      className="mt-0.5 h-4 w-4 shrink-0 text-red-400" />}
                {testState === 'loading' && <Loader2      className="mt-0.5 h-4 w-4 shrink-0 animate-spin" />}
                <span>{testMsg}</span>
              </div>
            )}

          </div>

          {/* ── Footer action bar ── */}
          <div className="flex flex-wrap items-center gap-2 border-t bg-muted/20 px-5 py-4 rounded-b-xl">
            <Button
              type="submit"
              size="sm"
              disabled={saveMutation.isPending}
              className="bg-indigo-500 text-white hover:bg-indigo-600 shadow-sm"
            >
              {saveMutation.isPending
                ? <><Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />Saving…</>
                : isEdit ? 'Save changes' : 'Create connection'
              }
            </Button>

            {!isNetworkScan && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => testMutation.mutate()}
                disabled={testMutation.isPending || !canTestNow}
                title={
                  form.conn_type === 'do' && !isEdit
                    ? 'Save the connection first to run a test'
                    : undefined
                }
                className={cn(
                  testState === 'ok' && 'border-green-500/40 text-green-400 hover:text-green-300',
                  testState === 'error' && 'border-red-500/40 text-red-400 hover:text-red-300'
                )}
              >
                {testState === 'loading' && <Loader2      className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                {testState === 'ok'      && <CheckCircle2 className="mr-1.5 h-3.5 w-3.5" />}
                {testState === 'error'   && <XCircle      className="mr-1.5 h-3.5 w-3.5" />}
                {testState === 'idle'    && <Shield       className="mr-1.5 h-3.5 w-3.5" />}
                {form.conn_type === 'code' ? 'Test access' : 'Test connection'}
              </Button>
            )}

            <div className="flex-1" />

            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-muted-foreground"
              onClick={() => navigate(backPath)}
            >
              Cancel
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
