import { useState, useRef, useEffect, useCallback } from 'react'
import QRCode from 'qrcode'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Loader2, Upload, CheckCircle2, Trash2, UserPlus, Shield, Eye, Key, Copy,
  AlertTriangle, Lock, ExternalLink, CheckCircle, UserCheck, Link, User,
  Bell, Palette, Users, CreditCard, Globe, LayoutGrid,
  Cloud, Code2, ClipboardCheck, Kanban, ScrollText, Activity, FileText,
  FolderOpen, Layers, Building2, KeyRound, Clock, ShieldCheck,
} from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Badge } from '@/components/ui/badge'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import {
  authApi, teamApi, apiTokensApi, licenseApi, auditorInvitesApi, modulesApi,
  workspaceApi, activityApi,
  type User as UserType, type TeamMember, type APIToken, type LicenseFeature,
  type LicensePlan, type AuditorInvite, type AuditorPermission,
} from '@/lib/api'
import { useAuthStore } from '@/store/useAuthStore'
import { formatDate } from '@/lib/utils'

// ── Helpers ───────────────────────────────────────────────────────────────────

type AssetKey = 'logo' | 'watermark' | 'footer-bg'

const assetLabels: Record<AssetKey, string> = {
  logo: 'Logo',
  watermark: 'Watermark',
  'footer-bg': 'Footer background',
}

// ── Asset Upload ──────────────────────────────────────────────────────────────

function useAssetPreview(type: AssetKey, refreshKey: number) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null)
  const load = useCallback(() => {
    const token = localStorage.getItem('access_token')
    if (!token) return
    fetch(`/api/me/assets/${type}`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.ok ? r.blob() : null)
      .then(blob => {
        if (blob) setBlobUrl(prev => { if (prev) URL.revokeObjectURL(prev); return URL.createObjectURL(blob) })
      })
      .catch(() => {})
  }, [type])
  useEffect(() => { load() }, [load, refreshKey])
  return blobUrl
}

function AssetUpload({ type }: { type: AssetKey }) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [localPreview, setLocalPreview] = useState<string | null>(null)
  const [refreshKey, setRefreshKey] = useState(0)
  const serverPreview = useAssetPreview(type, refreshKey)

  const mutation = useMutation({
    mutationFn: (file: File) => authApi.uploadAsset(type, file),
    onSuccess: () => { setRefreshKey(k => k + 1); toast.success(`${assetLabels[type]} uploaded`) },
    onError: () => toast.error(`Failed to upload ${assetLabels[type]}`),
  })

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => setLocalPreview(ev.target?.result as string)
    reader.readAsDataURL(file)
    mutation.mutate(file)
  }

  const displayImg = localPreview ?? serverPreview

  return (
    <div className="flex items-center gap-4">
      {displayImg ? (
        <img src={displayImg} alt={type} className="h-14 w-20 rounded border object-contain bg-muted shrink-0" />
      ) : (
        <div className="flex h-14 w-20 items-center justify-center rounded border bg-muted text-xs text-muted-foreground shrink-0">
          No file
        </div>
      )}
      <div>
        <p className="text-sm font-medium mb-1">{assetLabels[type]}</p>
        <p className="text-xs text-muted-foreground mb-2">
          {serverPreview && !localPreview ? 'Custom asset active' : 'Using default asset'}
        </p>
        <Button type="button" variant="outline" size="sm" onClick={() => inputRef.current?.click()} disabled={mutation.isPending}>
          {mutation.isPending ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            : <Upload className="mr-1.5 h-3.5 w-3.5" />}
          Upload PNG
        </Button>
        <input ref={inputRef} type="file" accept="image/png" className="hidden" onChange={handleChange} />
      </div>
    </div>
  )
}

// ── MFA Section ───────────────────────────────────────────────────────────────

function MFASection({
  mfaEnabled, mfaSetup, mfaCode, setMfaCode,
  onSetup, onVerify, onDisable,
  setupPending, verifyPending, disablePending,
}: {
  mfaEnabled: boolean
  mfaSetup: { secret: string; otpauth_url: string } | null
  mfaCode: string
  setMfaCode: (v: string) => void
  onSetup: () => void; onVerify: () => void; onDisable: () => void
  setupPending: boolean; verifyPending: boolean; disablePending: boolean
}) {
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null)
  const [showSecret, setShowSecret] = useState(false)
  const [copiedSecret, setCopiedSecret] = useState(false)

  useEffect(() => {
    if (!mfaSetup?.otpauth_url) { setQrDataUrl(null); return }
    QRCode.toDataURL(mfaSetup.otpauth_url, {
      width: 200, margin: 1, color: { dark: '#000000', light: '#ffffff' }, errorCorrectionLevel: 'M',
    }).then(setQrDataUrl).catch(() => setQrDataUrl(null))
  }, [mfaSetup?.otpauth_url])

  const copySecret = () => {
    if (!mfaSetup?.secret) return
    navigator.clipboard.writeText(mfaSetup.secret)
    setCopiedSecret(true)
    setTimeout(() => setCopiedSecret(false), 2000)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between rounded-lg border p-4">
        <div className="flex items-center gap-3">
          <div className={`h-2.5 w-2.5 rounded-full ${mfaEnabled ? 'bg-green-500' : 'bg-muted-foreground/40'}`} />
          <div>
            <p className="text-sm font-medium">Authenticator app (TOTP)</p>
            <p className="text-xs text-muted-foreground">
              {mfaEnabled ? 'Enabled — protected with MFA' : 'Disabled — password only'}
            </p>
          </div>
        </div>
        {!mfaEnabled && (
          <Button type="button" variant="outline" size="sm" onClick={onSetup} disabled={setupPending}>
            {setupPending ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Generating…</>
              : <><Shield className="mr-2 h-4 w-4" />Set up MFA</>}
          </Button>
        )}
      </div>

      {mfaSetup && !mfaEnabled && (
        <div className="rounded-lg border bg-muted/20 p-4 space-y-4">
          <div className="flex items-start gap-6">
            <div className="shrink-0">
              {qrDataUrl ? (
                <div className="rounded-lg overflow-hidden border bg-white p-1.5 w-[120px] h-[120px]">
                  <img src={qrDataUrl} alt="MFA QR Code" className="w-full h-full" />
                </div>
              ) : (
                <div className="rounded-lg border bg-muted flex items-center justify-center w-[120px] h-[120px]">
                  <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
              )}
            </div>
            <div className="space-y-3 flex-1">
              <div>
                <p className="text-sm font-medium mb-1">Step 1 — Scan QR code</p>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  Open Google Authenticator, Authy, or 1Password and scan the QR code, or enter the key manually.
                </p>
              </div>
              <div className="space-y-1.5">
                <div className="flex items-center gap-2">
                  <p className="text-xs text-muted-foreground">Manual secret key</p>
                  <button type="button" onClick={() => setShowSecret(v => !v)} className="text-xs text-indigo-400 hover:underline">
                    {showSecret ? 'hide' : 'show'}
                  </button>
                </div>
                {showSecret && (
                  <div className="flex items-center gap-2">
                    <code className="flex-1 rounded bg-background border px-3 py-1.5 text-xs font-mono tracking-widest break-all select-all">
                      {mfaSetup.secret}
                    </code>
                    <Button type="button" variant="outline" size="sm" className="h-8 px-2 shrink-0" onClick={copySecret}>
                      {copiedSecret ? <CheckCircle2 className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5" />}
                    </Button>
                  </div>
                )}
              </div>
            </div>
          </div>
          <Separator />
          <div className="space-y-2">
            <p className="text-sm font-medium">Step 2 — Verify code</p>
            <div className="flex items-center gap-3">
              <Input
                inputMode="numeric" autoComplete="one-time-code" placeholder="123 456"
                maxLength={6} className="w-40 font-mono text-base tracking-widest text-center"
                value={mfaCode} onChange={(e) => setMfaCode(e.target.value.replace(/\D/g, ''))}
                onKeyDown={(e) => e.key === 'Enter' && mfaCode.length === 6 && onVerify()}
              />
              <Button type="button" onClick={onVerify} disabled={verifyPending || mfaCode.length < 6}>
                {verifyPending ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Verifying…</>
                  : <><CheckCircle2 className="mr-2 h-4 w-4" />Verify &amp; Enable</>}
              </Button>
            </div>
          </div>
        </div>
      )}

      {mfaEnabled && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 space-y-3">
          <p className="text-sm font-medium text-destructive">Disable MFA</p>
          <p className="text-xs text-muted-foreground">Enter your current authenticator code to disable MFA.</p>
          <div className="flex items-center gap-3">
            <Input
              inputMode="numeric" autoComplete="one-time-code" placeholder="123 456"
              maxLength={6} className="w-40 font-mono text-base tracking-widest text-center"
              value={mfaCode} onChange={(e) => setMfaCode(e.target.value.replace(/\D/g, ''))}
              onKeyDown={(e) => e.key === 'Enter' && mfaCode.length === 6 && onDisable()}
            />
            <Button type="button" variant="destructive" onClick={onDisable} disabled={disablePending || mfaCode.length < 6}>
              {disablePending ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Disabling…</> : 'Disable MFA'}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

// ── API Tokens ─────────────────────────────────────────────────────────────────

function APITokensSection() {
  const qc = useQueryClient()
  const [newName, setNewName] = useState('')
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<APIToken | null>(null)

  const { data: tokens = [] } = useQuery({ queryKey: ['api-tokens'], queryFn: apiTokensApi.list })

  const createMutation = useMutation({
    mutationFn: () => apiTokensApi.create(newName),
    onSuccess: (data) => { qc.invalidateQueries({ queryKey: ['api-tokens'] }); setCreatedToken(data.token); setNewName(''); toast.success('Token created') },
    onError: () => toast.error('Failed to create token'),
  })
  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiTokensApi.delete(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['api-tokens'] }); toast.success('Token deleted'); setDeleteTarget(null) },
    onError: () => toast.error('Failed to delete token'),
  })

  return (
    <>
      {createdToken && (
        <div className="rounded-md border border-yellow-500/30 bg-yellow-500/10 p-3 space-y-2">
          <p className="text-xs font-semibold text-yellow-400 flex items-center gap-1.5">
            <AlertTriangle className="h-3.5 w-3.5" /> Copy this token now — it won't be shown again
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 text-xs font-mono bg-black/20 rounded p-2 break-all">{createdToken}</code>
            <Button size="icon" variant="ghost" className="shrink-0" onClick={() => { navigator.clipboard.writeText(createdToken); toast.success('Copied') }}>
              <Copy className="h-4 w-4" />
            </Button>
          </div>
          <Button size="sm" variant="outline" onClick={() => setCreatedToken(null)}>Dismiss</Button>
        </div>
      )}

      {tokens.length > 0 && (
        <div className="space-y-2">
          {tokens.map((t) => (
            <div key={t.id} className="flex items-center justify-between rounded-lg border p-3">
              <div className="flex items-center gap-3">
                <Key className="h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">{t.name}</p>
                  <p className="text-xs text-muted-foreground font-mono">{t.token_prefix}…</p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <div className="text-right">
                  <p className="text-xs text-muted-foreground">Created {formatDate(t.created_at)}</p>
                  <p className="text-xs text-muted-foreground flex items-center gap-1 justify-end">
                    <Clock className="h-3 w-3" />
                    {t.last_used_at ? `Used ${formatDate(t.last_used_at)}` : 'Never used'}
                  </p>
                </div>
                <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive" onClick={() => setDeleteTarget(t)}>
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          ))}
          <Separator />
        </div>
      )}

      <div className="space-y-2">
        <Label>New token</Label>
        <p className="text-xs text-muted-foreground">Give it a name, then click Generate. The token is shown once.</p>
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }} className="flex gap-2">
          <Input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="e.g. CI pipeline" required className="flex-1" />
          <Button type="submit" size="sm" disabled={createMutation.isPending || !newName.trim()}>
            {createMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Generate'}
          </Button>
        </form>
      </div>

      <ConfirmDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}
        title="Delete token" description={`Delete token "${deleteTarget?.name}"? Integrations using it will stop working.`}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)} loading={deleteMutation.isPending} />
    </>
  )
}

// ── License ───────────────────────────────────────────────────────────────────

const planColors: Record<string, string> = {
  community:    'border-border text-muted-foreground',
  starter:      'border-sky-500/50 text-sky-400',
  professional: 'border-indigo-500/50 text-indigo-400',
  business:     'border-violet-500/50 text-violet-400',
  enterprise:   'border-purple-400/50 text-purple-400',
}

const planLabel: Record<string, string> = {
  community:    'Free',
  starter:      'Starter',
  professional: 'Pro',
  business:     'Team',
  enterprise:   'Enterprise',
}

const allFeatures: { key: LicenseFeature; label: string }[] = [
  { key: 'scheduled_audits',  label: 'Scheduled audits' },
  { key: 'code_audit',        label: 'Code audit' },
  { key: 'share_links',       label: 'Share links' },
  { key: 'api_tokens',        label: 'API tokens' },
  { key: 'custom_branding',   label: 'Custom branding' },
  { key: 'team',              label: 'Team management' },
  { key: 'sso',               label: 'SSO' },
]

function UsageBar({ label, used, max }: { label: string; used: number; max: number }) {
  const pct = max > 0 ? Math.min(100, Math.round((used / max) * 100)) : 0
  const over = pct >= 90
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-sm">
        <span>{label}</span>
        <span className={`text-xs ${over ? 'text-red-400' : 'text-muted-foreground'}`}>{used} / {max < 0 ? '∞' : max}</span>
      </div>
      {max >= 0 && (
        <div className="h-1.5 rounded-full bg-muted overflow-hidden">
          <div className={`h-full rounded-full transition-all ${over ? 'bg-red-400' : 'bg-indigo-500'}`} style={{ width: `${pct}%` }} />
        </div>
      )}
    </div>
  )
}

function LicenseSection() {
  const qc = useQueryClient()
  const { user } = useAuthStore()
  const isAdmin = user?.role === 'admin' || user?.role === 'owner'
  const [keyInput, setKeyInput] = useState('')

  const { data: license, isLoading } = useQuery({ queryKey: ['license'], queryFn: licenseApi.get })
  const previewMutation = useMutation({
    mutationFn: (plan: LicensePlan) => licenseApi.setPreviewPlan(plan),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['license'] }); toast.success('Preview plan updated') },
    onError: () => toast.error('Failed to set preview plan'),
  })
  const activateMutation = useMutation({
    mutationFn: () => licenseApi.activate(keyInput.trim()),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['license'] }); setKeyInput(''); toast.success('License activated') },
    onError: (err: any) => toast.error(err?.response?.data?.error ?? 'Invalid license key'),
  })

  if (isLoading) return <div className="flex justify-center py-8"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>

  const plan = license?.plan ?? 'community'
  const features = license?.features ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">Current plan</p>
          {license?.issued_to && <p className="text-xs text-muted-foreground">Issued to: {license.issued_to}</p>}
          {license?.expires_at && <p className="text-xs text-muted-foreground">Expires: {formatDate(license.expires_at)}</p>}
        </div>
        <div className="flex items-center gap-2">
          {isAdmin && (
            <Select value={plan} onValueChange={(v) => previewMutation.mutate(v as LicensePlan)} disabled={previewMutation.isPending}>
              <SelectTrigger className="h-7 w-32 text-xs border-dashed border-muted-foreground/50"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="community">Free</SelectItem>
                <SelectItem value="starter">Starter</SelectItem>
                <SelectItem value="professional">Pro</SelectItem>
                <SelectItem value="business">Team</SelectItem>
              </SelectContent>
            </Select>
          )}
          <Badge variant="outline" className={`text-sm px-3 py-1 ${planColors[plan] ?? ''}`}>
            {planLabel[plan] ?? plan}
          </Badge>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-1.5">
        {allFeatures.map(({ key, label }) => {
          const enabled = features.includes(key)
          return (
            <div key={key} className="flex items-center gap-2 text-sm">
              {enabled ? <CheckCircle className="h-3.5 w-3.5 text-indigo-400 shrink-0" /> : <Lock className="h-3.5 w-3.5 text-muted-foreground/40 shrink-0" />}
              <span className={enabled ? 'text-foreground' : 'text-muted-foreground/50 line-through'}>{label}</span>
            </div>
          )
        })}
      </div>

      <Separator />

      <div className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Usage</p>
        {license && (
          <>
            <UsageBar label="Connections" used={license.used_connections} max={license.max_connections} />
            <UsageBar label="Audits this month" used={license.used_audits_month} max={license.max_audits_month} />
            <UsageBar label="Team members" used={license.used_users} max={license.max_users} />
          </>
        )}
      </div>

      <div className="flex items-center justify-between text-sm">
        <span className="text-muted-foreground">{plan === 'community' ? 'Upgrade from $19/mo to unlock more features.' : 'Manage your license key on the Plans page.'}</span>
        <a href="/plans" className="text-indigo-400 hover:underline text-xs font-medium flex items-center gap-1">
          View Plans <ExternalLink className="h-3 w-3" />
        </a>
      </div>

      {isAdmin && (
        <>
          <Separator />
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground flex items-center gap-1.5">
              <KeyRound className="h-3.5 w-3.5" />Activate License Key
            </p>
            <p className="text-xs text-muted-foreground">Paste a signed JWT key to unlock a plan for all users.</p>
            <form onSubmit={(e) => { e.preventDefault(); if (keyInput.trim()) activateMutation.mutate() }} className="flex gap-2">
              <Input
                value={keyInput} onChange={(e) => setKeyInput(e.target.value)}
                placeholder="eyJhbGciOiJSUzI1NiIs…"
                className="flex-1 font-mono text-xs h-8"
              />
              <Button type="submit" size="sm" className="h-8" disabled={!keyInput.trim() || activateMutation.isPending}>
                {activateMutation.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : 'Activate'}
              </Button>
            </form>
          </div>
        </>
      )}
    </div>
  )
}

// ── Team ──────────────────────────────────────────────────────────────────────

function TeamSection({ currentUserId }: { currentUserId: string }) {
  const qc = useQueryClient()
  const [inviteForm, setInviteForm] = useState({ email: '', password: '', role: 'viewer' })
  const [deleteTarget, setDeleteTarget] = useState<TeamMember | null>(null)

  const { data: members = [] } = useQuery({ queryKey: ['team'], queryFn: teamApi.list })

  const inviteMutation = useMutation({
    mutationFn: () => teamApi.invite(inviteForm),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['team'] }); setInviteForm({ email: '', password: '', role: 'viewer' }); toast.success('Member invited') },
    onError: () => toast.error('Failed to invite — email may already exist'),
  })
  const roleMutation = useMutation({
    mutationFn: ({ id, role }: { id: string; role: string }) => teamApi.updateRole(id, role),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['team'] }); toast.success('Role updated') },
    onError: () => toast.error('Failed to update role'),
  })
  const deleteMutation = useMutation({
    mutationFn: (id: string) => teamApi.delete(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['team'] }); toast.success('Member removed'); setDeleteTarget(null) },
    onError: () => toast.error('Failed to remove member'),
  })

  return (
    <>
      <div className="space-y-2">
        {members.map((m) => (
          <div key={m.id} className="flex items-center justify-between rounded-lg border p-3">
            <div className="flex items-center gap-3">
              {m.role === 'admin' || m.role === 'owner' ? <Shield className="h-4 w-4 text-indigo-400" /> : <Eye className="h-4 w-4 text-muted-foreground" />}
              <div>
                <p className="text-sm font-medium">{m.email}</p>
                <p className="text-xs text-muted-foreground">{formatDate(m.created_at)}</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {m.id !== currentUserId && m.role !== 'owner' ? (
                <Select
                  value={m.role}
                  onValueChange={(v) => roleMutation.mutate({ id: m.id, role: v })}
                  disabled={roleMutation.isPending}
                >
                  <SelectTrigger className="h-7 w-24 text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="viewer">Viewer</SelectItem>
                    <SelectItem value="admin">Admin</SelectItem>
                  </SelectContent>
                </Select>
              ) : (
                <Badge variant="outline" className="capitalize text-xs">{m.role}</Badge>
              )}
              {m.id !== currentUserId && m.role !== 'owner' && (
                <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive" onClick={() => setDeleteTarget(m)}>
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              )}
            </div>
          </div>
        ))}
      </div>

      <Separator />

      <div>
        <p className="text-sm font-medium mb-3 flex items-center gap-2"><UserPlus className="h-4 w-4" />Invite member</p>
        <form onSubmit={(e) => { e.preventDefault(); inviteMutation.mutate() }} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="invite-email">Email</Label>
              <Input id="invite-email" type="email" value={inviteForm.email} onChange={(e) => setInviteForm(p => ({ ...p, email: e.target.value }))} required placeholder="user@example.com" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="invite-role">Role</Label>
              <Select value={inviteForm.role} onValueChange={(v) => setInviteForm(p => ({ ...p, role: v }))}>
                <SelectTrigger id="invite-role"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="viewer">Viewer</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="invite-pw">Temporary password</Label>
            <Input id="invite-pw" type="password" value={inviteForm.password} onChange={(e) => setInviteForm(p => ({ ...p, password: e.target.value }))} required minLength={8} placeholder="Min 8 characters" />
          </div>
          <Button type="submit" size="sm" disabled={inviteMutation.isPending}>
            {inviteMutation.isPending ? <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" /> : <UserPlus className="mr-2 h-3.5 w-3.5" />}
            Invite
          </Button>
        </form>
      </div>

      <ConfirmDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}
        title="Remove team member" description={`Remove ${deleteTarget?.email}? They will lose access immediately.`}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)} loading={deleteMutation.isPending} />
    </>
  )
}

// ── Auditor Access Portal ─────────────────────────────────────────────────────

const PERM_OPTIONS: { value: AuditorPermission; label: string; description: string }[] = [
  { value: 'compliance', label: 'Compliance',        description: 'Framework scores & controls' },
  { value: 'evidence',   label: 'Evidence Library',  description: 'View & download evidence' },
  { value: 'policies',   label: 'Policies',          description: 'View & download policies' },
  { value: 'findings',   label: 'Findings Summary',  description: 'Severity counts only' },
]

function AuditorAccessSection() {
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [newLink, setNewLink] = useState<string | null>(null)
  const [form, setForm] = useState({
    name: '', email: '', expiry_days: 30,
    permissions: ['compliance', 'evidence', 'policies'] as AuditorPermission[],
  })

  const { data: invites = [] } = useQuery({ queryKey: ['auditor-invites'], queryFn: auditorInvitesApi.list })

  const createMutation = useMutation({
    mutationFn: () => auditorInvitesApi.create(form),
    onSuccess: (inv: AuditorInvite) => {
      qc.invalidateQueries({ queryKey: ['auditor-invites'] })
      setNewLink(inv.app_url ?? `${window.location.origin}/auditor/${inv.token}`)
      setForm({ name: '', email: '', expiry_days: 30, permissions: ['compliance', 'evidence', 'policies'] })
    },
    onError: () => toast.error('Failed to create invite'),
  })
  const deleteMutation = useMutation({
    mutationFn: (id: string) => auditorInvitesApi.delete(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auditor-invites'] }); toast.success('Invite revoked') },
    onError: () => toast.error('Revoke failed'),
  })

  const togglePerm = (perm: AuditorPermission) => setForm(f => ({
    ...f, permissions: f.permissions.includes(perm) ? f.permissions.filter(p => p !== perm) : [...f.permissions, perm],
  }))

  const copyLink = (link: string) => navigator.clipboard.writeText(link).then(() => toast.success('Link copied!'))
  const buildLink = (inv: AuditorInvite) => inv.app_url ?? `${window.location.origin}/auditor/${inv.token}`

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Share a read-only portal with external auditors. No account required — access via a secure token link.
        </p>
        <Button variant="outline" size="sm" onClick={() => { setShowCreate(true); setNewLink(null) }} className="flex items-center gap-2 shrink-0 ml-4">
          <UserPlus className="h-4 w-4" />Create Invite
        </Button>
      </div>

      {newLink && (
        <div className="rounded-lg border border-green-500/30 bg-green-500/5 p-4 space-y-2">
          <p className="text-sm font-medium text-green-400 flex items-center gap-2">
            <CheckCircle className="h-4 w-4" />Portal created! Share this link:
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 text-xs text-green-300 bg-black/20 rounded px-3 py-2 break-all font-mono">{newLink}</code>
            <Button size="sm" variant="outline" className="shrink-0" onClick={() => copyLink(newLink)}><Copy className="h-3.5 w-3.5" /></Button>
          </div>
          <p className="text-xs text-green-300/60">This link will not be shown again. Copy it now.</p>
        </div>
      )}

      {showCreate && !newLink && (
        <div className="rounded-lg border bg-muted/30 p-4 space-y-4">
          <h3 className="text-sm font-medium">New Auditor Invite</h3>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Auditor Name *</Label>
              <Input placeholder="e.g. John Smith (PwC)" value={form.name} onChange={(e) => setForm(f => ({ ...f, name: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label>Email (optional)</Label>
              <Input type="email" placeholder="auditor@firm.com" value={form.email} onChange={(e) => setForm(f => ({ ...f, email: e.target.value }))} />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>Portal Expiry</Label>
            <div className="flex gap-2 flex-wrap">
              {[{ days: 30, label: '30 days' }, { days: 60, label: '60 days' }, { days: 90, label: '90 days' }].map(({ days, label }) => (
                <button key={days} type="button" onClick={() => setForm(f => ({ ...f, expiry_days: days }))}
                  className={`px-3 py-1.5 rounded-md border text-xs font-medium transition-colors ${form.expiry_days === days ? 'border-indigo-500/50 bg-indigo-500/10 text-indigo-400' : 'border-border text-muted-foreground hover:border-foreground/30'}`}>
                  {label}
                </button>
              ))}
              <Input type="number" min={1} max={365} className="w-24 h-8 text-xs" placeholder="Custom"
                onChange={(e) => { const v = parseInt(e.target.value); if (v > 0) setForm(f => ({ ...f, expiry_days: v })) }} />
            </div>
          </div>

          <div className="space-y-2">
            <Label>Permissions</Label>
            <div className="grid grid-cols-2 gap-2">
              {PERM_OPTIONS.map((opt) => (
                <label key={opt.value} className={`flex items-start gap-2.5 rounded-lg border p-3 cursor-pointer transition-colors ${form.permissions.includes(opt.value) ? 'border-indigo-500/40 bg-indigo-500/5' : 'border-border hover:border-foreground/20'}`}>
                  <input type="checkbox" className="mt-0.5" checked={form.permissions.includes(opt.value)} onChange={() => togglePerm(opt.value)} />
                  <div>
                    <p className="text-xs font-medium">{opt.label}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">{opt.description}</p>
                  </div>
                </label>
              ))}
            </div>
          </div>

          <div className="flex gap-2">
            <Button type="button" disabled={!form.name || form.permissions.length === 0 || createMutation.isPending} onClick={() => createMutation.mutate()}>
              {createMutation.isPending ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Creating…</> : <><Link className="mr-2 h-4 w-4" />Create Portal Link</>}
            </Button>
            <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
          </div>
        </div>
      )}

      {invites.length === 0 && !showCreate ? (
        <div className="flex flex-col items-center gap-3 py-8 text-center text-muted-foreground">
          <UserCheck className="h-10 w-10 opacity-30" />
          <div>
            <p className="text-sm font-medium">No auditor portals yet</p>
            <p className="text-xs mt-1">Create a secure portal to share with external auditors.</p>
          </div>
        </div>
      ) : invites.length > 0 ? (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="pb-2 pr-4 font-medium">Auditor</th>
                <th className="pb-2 pr-4 font-medium">Permissions</th>
                <th className="pb-2 pr-4 font-medium">Expires</th>
                <th className="pb-2 pr-4 font-medium">Last seen</th>
                <th className="pb-2 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {invites.map((inv) => {
                const expired = new Date(inv.expires_at) < new Date()
                return (
                  <tr key={inv.id} className="hover:bg-muted/30 transition-colors">
                    <td className="py-3 pr-4">
                      <p className="font-medium text-sm">{inv.name}</p>
                      {inv.email && <p className="text-xs text-muted-foreground">{inv.email}</p>}
                    </td>
                    <td className="py-3 pr-4">
                      <div className="flex flex-wrap gap-1">
                        {inv.permissions.map((p) => <Badge key={p} variant="outline" className="text-xs capitalize">{p}</Badge>)}
                      </div>
                    </td>
                    <td className="py-3 pr-4">
                      <span className={expired ? 'text-red-400 text-xs font-medium' : 'text-xs text-muted-foreground'}>
                        {new Date(inv.expires_at).toLocaleDateString()}{expired && ' (expired)'}
                      </span>
                    </td>
                    <td className="py-3 pr-4 text-xs text-muted-foreground">
                      {inv.last_accessed_at ? formatDate(inv.last_accessed_at) : <span className="text-muted-foreground/50">Never</span>}
                    </td>
                    <td className="py-3">
                      <div className="flex items-center gap-1 justify-end">
                        <Button size="sm" variant="ghost" className="h-8 px-2" title="Copy link" onClick={() => copyLink(buildLink(inv))}><Copy className="h-3.5 w-3.5" /></Button>
                        <Button size="sm" variant="ghost" className="h-8 px-2" title="Open portal" onClick={() => window.open(buildLink(inv), '_blank')}><ExternalLink className="h-3.5 w-3.5" /></Button>
                        <Button size="sm" variant="ghost" className="h-8 px-2 text-destructive hover:text-destructive" title="Revoke" disabled={deleteMutation.isPending}
                          onClick={() => { if (confirm(`Revoke portal for ${inv.name}?`)) deleteMutation.mutate(inv.id) }}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  )
}

// ── Workspace Section ─────────────────────────────────────────────────────────

function WorkspaceSection({ isAdmin }: { isAdmin: boolean }) {
  const qc = useQueryClient()
  const { data: ws, isLoading } = useQuery({ queryKey: ['workspace'], queryFn: workspaceApi.get })
  const [form, setForm] = useState({ name: '', slack_webhook_url: '' })

  useEffect(() => {
    if (ws) setForm({ name: ws.name, slack_webhook_url: ws.slack_webhook_url })
  }, [ws])

  const mutation = useMutation({
    mutationFn: () => workspaceApi.update(form),
    onSuccess: (data) => { qc.setQueryData(['workspace'], data); toast.success('Workspace saved') },
    onError: () => toast.error('Failed to save workspace'),
  })

  if (isLoading) return <div className="flex justify-center py-4"><Loader2 className="h-4 w-4 animate-spin text-muted-foreground" /></div>

  return (
    <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="ws-name">Workspace name</Label>
        <Input
          id="ws-name" value={form.name}
          onChange={(e) => setForm(f => ({ ...f, name: e.target.value }))}
          placeholder="My Company" disabled={!isAdmin}
        />
        <p className="text-xs text-muted-foreground">Shown in reports and emails sent to your team.</p>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="ws-slack" className="flex items-center gap-2">
          <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" fill="currentColor"><path d="M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zM6.313 15.165a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.834 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.834 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.834zM8.834 6.313a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.528 2.528 0 0 1 0 8.834a2.528 2.528 0 0 1 2.522-2.521h6.312zM18.956 8.834a2.528 2.528 0 0 1 2.522-2.521A2.528 2.528 0 0 1 24 8.834a2.528 2.528 0 0 1-2.522 2.521h-2.522V8.834zM17.688 8.834a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.165 0a2.528 2.528 0 0 1 2.523 2.522v6.312zM15.165 18.956a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.165 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zM15.165 17.688a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z"/></svg>
          Slack Webhook URL
        </Label>
        <Input
          id="ws-slack" value={form.slack_webhook_url}
          onChange={(e) => setForm(f => ({ ...f, slack_webhook_url: e.target.value }))}
          placeholder="https://hooks.slack.com/services/T.../B.../..." disabled={!isAdmin}
        />
        <p className="text-xs text-muted-foreground">Receive audit completion notifications in your Slack channel. Leave empty to disable.</p>
      </div>
      {isAdmin && (
        <Button type="submit" size="sm" disabled={mutation.isPending || !form.name.trim()}>
          {mutation.isPending ? <><Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />Saving…</> : 'Save workspace'}
        </Button>
      )}
    </form>
  )
}

// ── Activity Log Section ──────────────────────────────────────────────────────

const actionLabels: Record<string, string> = {
  'audit.run':           'Audit started',
  'connection.created':  'Connection created',
  'license.activated':   'License activated',
  'workspace.update':    'Workspace updated',
  'team.role_changed':   'Team role changed',
}

const actionColors: Record<string, string> = {
  'audit.run':          'text-indigo-400',
  'connection.created': 'text-green-400',
  'license.activated':  'text-yellow-400',
  'workspace.update':   'text-blue-400',
  'team.role_changed':  'text-violet-400',
}

function ActivitySection() {
  const { data: entries = [], isLoading } = useQuery({
    queryKey: ['activity-log'],
    queryFn: activityApi.list,
    refetchInterval: 30_000,
  })

  if (isLoading) return <div className="flex justify-center py-6"><Loader2 className="h-4 w-4 animate-spin text-muted-foreground" /></div>

  if (entries.length === 0) return (
    <div className="flex flex-col items-center gap-3 py-8 text-center text-muted-foreground">
      <Activity className="h-10 w-10 opacity-20" />
      <div>
        <p className="text-sm font-medium">No activity recorded yet</p>
        <p className="text-xs mt-1">Events like audit runs, connections and license changes will appear here.</p>
      </div>
    </div>
  )

  return (
    <div className="space-y-1.5">
      {entries.map((e) => (
        <div key={e.id} className="flex items-center justify-between rounded-lg border px-3 py-2.5 text-sm hover:bg-muted/30 transition-colors">
          <div className="flex items-center gap-3 min-w-0">
            <Activity className={`h-3.5 w-3.5 shrink-0 ${actionColors[e.action] ?? 'text-muted-foreground'}`} />
            <div className="min-w-0">
              <span className="font-medium">{actionLabels[e.action] ?? e.action}</span>
              {e.resource_type && (
                <span className="text-muted-foreground text-xs ml-2 truncate">· {e.resource_type}{e.resource_id ? ` ${e.resource_id.slice(0, 8)}` : ''}</span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-3 text-xs text-muted-foreground shrink-0 ml-4">
            <span className="hidden sm:block truncate max-w-[140px]">{e.user_email}</span>
            {e.ip_address && <span className="font-mono hidden md:block">{e.ip_address}</span>}
            <span className="whitespace-nowrap">{formatDate(e.created_at)}</span>
          </div>
        </div>
      ))}
    </div>
  )
}

// ── Modules Section ───────────────────────────────────────────────────────────

type ModuleItem = {
  key: string
  label: string
  description: string
  icon: React.ElementType
  stub?: boolean
}

// Grouped to match sidebar structure
const navModuleGroups: { group: string; items: ModuleItem[] }[] = [
  {
    group: 'Security Center',
    items: [
      { key: 'cloud_audits', label: 'Cloud Audits',    description: 'Connect to cloud providers and run infrastructure audits',             icon: Cloud },
      { key: 'code_iac',     label: 'Code & IaC',       description: 'Scan git repos for secrets, SAST issues, Terraform misconfigurations', icon: Code2 },
      { key: 'findings',     label: 'Findings',         description: 'Aggregated findings browser across all audit jobs',                   icon: AlertTriangle },
      { key: 'remediation',  label: 'Remediation',      description: 'Kanban board for tracking and resolving security issues',             icon: Kanban },
      { key: 'monitoring',   label: 'Monitoring & SLA', description: 'Continuous monitoring scores and SLA breach tracking',               icon: Activity },
    ],
  },
  {
    group: 'Compliance',
    items: [
      { key: 'compliance',     label: 'Frameworks',      description: 'SOC 2, ISO 27001, and other compliance frameworks',             icon: ClipboardCheck },
      { key: 'evidence',       label: 'Evidence',         description: 'Upload and manage evidence for compliance controls',           icon: FolderOpen },
      { key: 'policies',       label: 'Policies',         description: 'Security policy management and approval workflows',           icon: ScrollText },
      { key: 'access_reviews', label: 'Access Reviews',   description: 'Periodic user access reviews for compliance purposes',       icon: UserCheck },
    ],
  },
  {
    group: 'Reports',
    items: [
      { key: 'reports',      label: 'Reports',     description: 'Audit job reports in HTML and DOCX formats',                    icon: FileText },
      { key: 'audit_types',  label: 'Audit Types', description: 'Manage the types of checks available for cloud auditing',      icon: Layers },
    ],
  },
]

const scannerModuleItems: ModuleItem[] = [
  { key: 'scanner_do',    label: 'DigitalOcean',          description: 'Audit Droplets, Spaces, Firewalls, Databases, and Kubernetes clusters', icon: Cloud,          stub: false },
  { key: 'scanner_aws',   label: 'Amazon Web Services',   description: 'Audit EC2, S3, IAM, security groups, and VPCs',                         icon: Cloud,          stub: true },
  { key: 'scanner_gcp',   label: 'Google Cloud Platform', description: 'Audit Compute Engine, Cloud Storage, IAM, and network policies',        icon: Cloud,          stub: true },
  { key: 'scanner_azure', label: 'Microsoft Azure',       description: 'Audit VMs, Storage accounts, IAM roles, and network configurations',    icon: Cloud,          stub: true },
  { key: 'scanner_k8s',   label: 'Kubernetes',            description: 'Audit any Kubernetes cluster for security misconfigurations',            icon: Layers,         stub: true },
]

function ModulesSection({ isAdmin }: { isAdmin: boolean }) {
  const qc = useQueryClient()

  const { data: modules, isLoading } = useQuery({
    queryKey: ['modules'],
    queryFn: modulesApi.getAll,
  })

  const saveMutation = useMutation({
    mutationFn: (data: Record<string, boolean>) => modulesApi.setAll(data),
    onSuccess: (data) => {
      qc.setQueryData(['modules'], data)
      toast.success('Module settings saved')
    },
    onError: () => toast.error('Failed to save module settings'),
  })

  const toggle = (key: string, value: boolean) => {
    if (!modules) return
    saveMutation.mutate({ ...modules, [key]: value })
  }

  const isOn = (key: string) => !modules || modules[key] !== false

  if (!isAdmin) {
    return (
      <div className="flex flex-col items-center gap-3 py-10 text-center text-muted-foreground">
        <LayoutGrid className="h-10 w-10 opacity-20" />
        <p className="text-sm">Admin access required to manage modules.</p>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-10">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Navigation groups */}
      {navModuleGroups.map(({ group, items }) => (
        <Card key={group}>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <LayoutGrid className="h-4 w-4 text-indigo-400" />
              {group}
            </CardTitle>
            <CardDescription>
              Show or hide these items in the sidebar. Disabling hides the menu entry — data is preserved.
            </CardDescription>
          </CardHeader>
          <CardContent className="divide-y divide-border/50">
            {items.map(({ key, label, description, icon: Icon }) => (
              <div key={key} className="flex items-center justify-between py-3 first:pt-0 last:pb-0">
                <div className="flex items-center gap-3">
                  <div className="flex h-8 w-8 items-center justify-center rounded-md bg-muted">
                    <Icon className="h-4 w-4 text-muted-foreground" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">{label}</p>
                    <p className="text-xs text-muted-foreground">{description}</p>
                  </div>
                </div>
                <Switch
                  checked={isOn(key)}
                  onCheckedChange={(v) => toggle(key, v)}
                  disabled={saveMutation.isPending}
                />
              </div>
            ))}
          </CardContent>
        </Card>
      ))}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <Cloud className="h-4 w-4 text-indigo-400" />
            Cloud Scanners
          </CardTitle>
          <CardDescription>
            Enable or disable cloud provider integrations. Stub scanners are under development — enabling them shows the option in connections but scanning is not yet active.
          </CardDescription>
        </CardHeader>
        <CardContent className="divide-y divide-border/50">
          {scannerModuleItems.map(({ key, label, description, icon: Icon, stub }) => (
            <div key={key} className="flex items-center justify-between py-3 first:pt-0 last:pb-0">
              <div className="flex items-center gap-3">
                <div className="flex h-8 w-8 items-center justify-center rounded-md bg-muted">
                  <Icon className="h-4 w-4 text-muted-foreground" />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium">{label}</p>
                    {stub && (
                      <span className="inline-flex items-center rounded-full border border-yellow-400/40 bg-yellow-400/10 px-1.5 py-0.5 text-[9px] font-medium text-yellow-400 leading-none">
                        coming soon
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground">{description}</p>
                </div>
              </div>
              <Switch
                checked={isOn(key)}
                onCheckedChange={(v) => toggle(key, v)}
                disabled={saveMutation.isPending}
              />
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

// ── Nav item types ────────────────────────────────────────────────────────────

type TabId = 'profile' | 'security' | 'branding' | 'team' | 'access' | 'modules' | 'activity'

const navItems: { id: TabId; label: string; icon: React.ElementType; sub: string }[] = [
  { id: 'profile',  label: 'Profile',   icon: User,        sub: 'Auditor info & notifications' },
  { id: 'security', label: 'Security',  icon: Shield,      sub: 'Password, MFA, API tokens' },
  { id: 'branding', label: 'Branding',  icon: Palette,     sub: 'Logos & report images' },
  { id: 'team',     label: 'Team',      icon: Users,       sub: 'Members & license' },
  { id: 'access',   label: 'External',  icon: Globe,       sub: 'Auditor portals' },
  { id: 'modules',  label: 'Modules',   icon: LayoutGrid,  sub: 'Enable / disable features' },
  { id: 'activity', label: 'Activity',  icon: Activity,    sub: 'Recent workspace events' },
]

// ── Styled card header helper ─────────────────────────────────────────────────

function SCardHeader({
  icon: Icon,
  iconBg,
  iconColor,
  title,
  description,
}: {
  icon: React.ElementType
  iconBg: string
  iconColor: string
  title: React.ReactNode
  description?: React.ReactNode
}) {
  return (
    <CardHeader className="pb-4">
      <div className="flex items-start gap-3">
        <div className={cn('mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', iconBg)}>
          <Icon className={cn('h-4 w-4', iconColor)} />
        </div>
        <div className="flex-1 min-w-0">
          <CardTitle className="text-sm leading-snug">{title}</CardTitle>
          {description && <CardDescription className="text-xs mt-0.5">{description}</CardDescription>}
        </div>
      </div>
    </CardHeader>
  )
}

// ── Main Settings Page ────────────────────────────────────────────────────────

export function Settings() {
  const qc = useQueryClient()
  const { setUser, user: currentUser } = useAuthStore()
  const [activeTab, setActiveTab] = useState<TabId>('profile')

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: authApi.me })

  const [profile, setProfile] = useState<Partial<UserType>>({})
  const profileData = { ...(me ?? {}), ...profile }

  const [pwForm, setPwForm] = useState({ current: '', next: '' })
  const [mfaSetup, setMfaSetup] = useState<{ secret: string; otpauth_url: string } | null>(null)
  const [mfaCode, setMfaCode] = useState('')

  const profileMutation = useMutation({
    mutationFn: () => authApi.updateSettings(profile),
    onSuccess: (user) => { setUser(user); setProfile({}); toast.success('Profile saved') },
    onError: () => toast.error('Failed to save profile'),
  })
  const pwMutation = useMutation({
    mutationFn: () => authApi.changePassword(pwForm.current, pwForm.next),
    onSuccess: () => { setPwForm({ current: '', next: '' }); toast.success('Password changed') },
    onError: () => toast.error('Current password incorrect'),
  })
  const mfaSetupMutation = useMutation({
    mutationFn: authApi.setupMfa,
    onSuccess: (data) => { setMfaSetup(data); toast.message('Scan or enter the MFA secret, then verify the code') },
    onError: () => toast.error('Failed to start MFA setup'),
  })
  const mfaVerifyMutation = useMutation({
    mutationFn: () => authApi.verifyMfa(mfaCode),
    onSuccess: () => { setMfaSetup(null); setMfaCode(''); qc.invalidateQueries({ queryKey: ['me'] }); toast.success('MFA enabled') },
    onError: () => toast.error('Invalid MFA code'),
  })
  const mfaDisableMutation = useMutation({
    mutationFn: () => authApi.disableMfa(mfaCode),
    onSuccess: () => { setMfaCode(''); qc.invalidateQueries({ queryKey: ['me'] }); toast.success('MFA disabled') },
    onError: () => toast.error('Invalid MFA code'),
  })
  const notifyMutation = useMutation({
    mutationFn: (v: boolean) => authApi.updateNotify(v),
    onSuccess: (user) => { setUser(user); toast.success('Notification preference saved') },
    onError: () => toast.error('Failed to update'),
  })

  const setP = (key: keyof UserType, value: string) => setProfile(p => ({ ...p, [key]: value }))
  const isAdmin = ['owner', 'admin'].includes(me?.role ?? '') || ['owner', 'admin'].includes(currentUser?.role ?? '')

  const profileFields = [
    { key: 'auditor_org' as const,     label: 'Organization',  placeholder: 'CloudSecGuard, Inc.' },
    { key: 'auditor_email' as const,   label: 'Contact email', placeholder: 'delivery@cloudsecguard.com' },
    { key: 'auditor_phone' as const,   label: 'Phone',         placeholder: '+1 650 4847938' },
    { key: 'auditor_website' as const, label: 'Website',       placeholder: 'cloudsecguard.com' },
    { key: 'auditor_address' as const, label: 'Address',       placeholder: '8 the Grn Ste A, Dover, DE 19901' },
    { key: 'prepared_by' as const,     label: 'Prepared by',   placeholder: 'CloudSecGuard Security Team' },
  ]

  // role badge colors
  const roleCls: Record<string, string> = {
    owner: 'border-purple-500/50 text-purple-400 bg-purple-500/5',
    admin: 'border-indigo-500/50 text-indigo-400 bg-indigo-500/5',
    viewer: 'border-border text-muted-foreground bg-muted/40',
  }
  const initials = me?.email ? me.email.slice(0, 2).toUpperCase() : '??'
  const role = me?.role ?? currentUser?.role ?? 'viewer'

  return (
    <div className="max-w-5xl">

      {/* ── Hero header ── */}
      <div className="mb-6 flex items-center gap-4 rounded-xl border bg-card p-4">
        <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-indigo-500/15 text-xl font-bold text-indigo-300 ring-1 ring-indigo-500/20">
          {initials}
        </div>
        <div className="flex-1 min-w-0">
          <p className="font-semibold truncate">{me?.email ?? '—'}</p>
          <div className="flex items-center gap-2 mt-1.5 flex-wrap">
            <span className={cn('inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold capitalize leading-none', roleCls[role] ?? roleCls.viewer)}>
              {role}
            </span>
            {me?.mfa_enabled && (
              <span className="inline-flex items-center gap-1 rounded-full border border-green-500/40 bg-green-500/5 px-2 py-0.5 text-[10px] font-semibold text-green-400 leading-none">
                <ShieldCheck className="h-2.5 w-2.5" /> MFA on
              </span>
            )}
          </div>
        </div>
        <div className="text-right shrink-0 hidden sm:block">
          <p className="text-[10px] text-muted-foreground uppercase tracking-wide mb-1">Section</p>
          <p className="text-sm font-semibold text-foreground capitalize">
            {navItems.find(n => n.id === activeTab)?.label}
          </p>
        </div>
      </div>

      {/* ── Two-column layout ── */}
      <div className="flex gap-6 items-start">

        {/* ── Vertical nav ── */}
        <nav className="w-[185px] shrink-0 space-y-0.5 sticky top-0">
          {navItems.map(({ id, label, icon: Icon, sub }) => {
            const isActive = activeTab === id
            return (
              <button
                key={id}
                onClick={() => setActiveTab(id)}
                className={cn(
                  'group relative w-full flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm text-left transition-all duration-150',
                  isActive
                    ? 'bg-indigo-500/10 text-foreground font-medium'
                    : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
                )}
              >
                {/* Active accent bar */}
                {isActive && (
                  <span className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-r-full bg-indigo-500" />
                )}
                {/* Icon */}
                <span className={cn(
                  'flex h-6 w-6 shrink-0 items-center justify-center rounded-md transition-colors',
                  isActive ? 'text-indigo-400' : 'text-muted-foreground group-hover:text-foreground'
                )}>
                  <Icon className="h-3.5 w-3.5" />
                </span>
                {/* Label + sub */}
                <div className="flex-1 min-w-0">
                  <span className="block leading-none">{label}</span>
                  <span className="block text-[10px] text-muted-foreground/50 leading-tight mt-0.5 truncate group-hover:text-muted-foreground/70">{sub}</span>
                </div>
                {isActive && <span className="h-1.5 w-1.5 rounded-full bg-indigo-500/70 shrink-0" />}
              </button>
            )
          })}
        </nav>

        {/* ── Content panel ── */}
        <div className="flex-1 min-w-0 space-y-5">

          {/* ── Profile ── */}
          {activeTab === 'profile' && (
            <>
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                {/* Auditor profile */}
                <Card>
                  <SCardHeader
                    icon={User} iconBg="bg-indigo-500/10" iconColor="text-indigo-400"
                    title="Auditor Profile"
                    description="Appears in every generated report."
                  />
                  <CardContent>
                    <form onSubmit={(e) => { e.preventDefault(); profileMutation.mutate() }} className="space-y-3">
                      <div className="grid grid-cols-2 gap-3">
                        {profileFields.map(({ key, label, placeholder }) => (
                          <div key={key} className="space-y-1.5">
                            <Label htmlFor={key} className="text-xs">{label}</Label>
                            <Input id={key} value={(profileData[key] as string | undefined) ?? ''} onChange={(e) => setP(key, e.target.value)} placeholder={placeholder} className="h-8 text-sm" />
                          </div>
                        ))}
                      </div>
                      <Button type="submit" size="sm" disabled={profileMutation.isPending} className="mt-1">
                        {profileMutation.isPending ? <><Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />Saving…</> : 'Save profile'}
                      </Button>
                    </form>
                  </CardContent>
                </Card>

                {/* Right column */}
                <div className="space-y-4">
                  {/* Notifications */}
                  <Card>
                    <SCardHeader
                      icon={Bell} iconBg="bg-amber-500/10" iconColor="text-amber-400"
                      title="Email Notifications"
                      description="Get notified when audits complete."
                    />
                    <CardContent className="space-y-3">
                      <div className="flex items-center justify-between">
                        <Label htmlFor="notify-toggle" className="text-sm">Email on audit completion</Label>
                        <Switch id="notify-toggle" checked={me?.notify_email ?? true} onCheckedChange={(v) => notifyMutation.mutate(v)} disabled={notifyMutation.isPending} />
                      </div>
                      <p className="text-xs text-muted-foreground">Sends to: {me?.auditor_email || '(set contact email above)'}</p>
                      <p className="text-xs text-muted-foreground/50">Configure SMTP via: <code className="text-[10px]">SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS</code></p>
                    </CardContent>
                  </Card>

                  {/* Account info */}
                  <Card>
                    <SCardHeader
                      icon={ShieldCheck} iconBg="bg-green-500/10" iconColor="text-green-400"
                      title="Account"
                      description="Your login and security details."
                    />
                    <CardContent className="space-y-2">
                      {[
                        { label: 'Email', value: <span className="font-mono text-xs">{me?.email}</span> },
                        { label: 'Role',  value: <Badge variant="outline" className={cn('capitalize text-xs', roleCls[role] ?? '')}>{role}</Badge> },
                        {
                          label: 'MFA',
                          value: (
                            <Badge variant="outline" className={cn('text-xs', me?.mfa_enabled ? 'border-green-500/50 text-green-400 bg-green-500/5' : '')}>
                              {me?.mfa_enabled ? 'Enabled' : 'Disabled'}
                            </Badge>
                          ),
                        },
                      ].map(({ label, value }) => (
                        <div key={label} className="flex items-center justify-between py-1">
                          <span className="text-xs text-muted-foreground">{label}</span>
                          {value}
                        </div>
                      ))}
                    </CardContent>
                  </Card>
                </div>
              </div>

              {/* Workspace — full width */}
              <Card>
                <SCardHeader
                  icon={Building2} iconBg="bg-violet-500/10" iconColor="text-violet-400"
                  title="Workspace"
                  description="Workspace name and Slack integration for notifications."
                />
                <CardContent>
                  <WorkspaceSection isAdmin={isAdmin} />
                </CardContent>
              </Card>
            </>
          )}

          {/* ── Security ── */}
          {activeTab === 'security' && (
            <>
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                {/* Password */}
                <Card>
                  <SCardHeader
                    icon={Lock} iconBg="bg-orange-500/10" iconColor="text-orange-400"
                    title="Change Password"
                    description="Use a strong, unique password."
                  />
                  <CardContent>
                    <form onSubmit={(e) => { e.preventDefault(); pwMutation.mutate() }} className="space-y-3">
                      <div className="space-y-1.5">
                        <Label htmlFor="current-pw" className="text-xs">Current password</Label>
                        <Input id="current-pw" type="password" value={pwForm.current} onChange={(e) => setPwForm(p => ({ ...p, current: e.target.value }))} required className="h-8" />
                      </div>
                      <div className="space-y-1.5">
                        <Label htmlFor="new-pw" className="text-xs">New password</Label>
                        <Input id="new-pw" type="password" value={pwForm.next} onChange={(e) => setPwForm(p => ({ ...p, next: e.target.value }))} required minLength={8} className="h-8" />
                      </div>
                      <Button type="submit" size="sm" disabled={pwMutation.isPending}>
                        {pwMutation.isPending ? <><Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />Updating…</> : 'Update password'}
                      </Button>
                    </form>
                  </CardContent>
                </Card>

                {/* MFA */}
                <Card>
                  <SCardHeader
                    icon={Shield} iconBg="bg-indigo-500/10" iconColor="text-indigo-400"
                    title="Two-Factor Authentication"
                    description="TOTP via Google Authenticator, Authy, or 1Password."
                  />
                  <CardContent>
                    <MFASection
                      mfaEnabled={me?.mfa_enabled ?? false}
                      mfaSetup={mfaSetup} mfaCode={mfaCode} setMfaCode={setMfaCode}
                      onSetup={() => mfaSetupMutation.mutate()}
                      onVerify={() => mfaVerifyMutation.mutate()}
                      onDisable={() => mfaDisableMutation.mutate()}
                      setupPending={mfaSetupMutation.isPending}
                      verifyPending={mfaVerifyMutation.isPending}
                      disablePending={mfaDisableMutation.isPending}
                    />
                  </CardContent>
                </Card>
              </div>

              {/* API Tokens */}
              <Card>
                <SCardHeader
                  icon={Key} iconBg="bg-yellow-500/10" iconColor="text-yellow-400"
                  title="Personal API Tokens"
                  description={<>Tokens for API access from scripts or CI/CD pipelines. These are <strong>not</strong> DigitalOcean tokens — those go in Connections.</>}
                />
                <CardContent className="space-y-4">
                  <APITokensSection />
                </CardContent>
              </Card>
            </>
          )}

          {/* ── Branding ── */}
          {activeTab === 'branding' && (
            <Card>
              <SCardHeader
                icon={Palette} iconBg="bg-pink-500/10" iconColor="text-pink-400"
                title="Report Assets"
                description="Custom images used in generated PDF and DOCX reports."
              />
              <CardContent className="space-y-6">
                <AssetUpload type="logo" />
                <Separator />
                <AssetUpload type="watermark" />
                <Separator />
                <AssetUpload type="footer-bg" />
              </CardContent>
            </Card>
          )}

          {/* ── Team ── */}
          {activeTab === 'team' && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
              {/* License */}
              <Card>
                <SCardHeader
                  icon={CreditCard} iconBg="bg-emerald-500/10" iconColor="text-emerald-400"
                  title="License"
                  description="Plan, features, and usage limits."
                />
                <CardContent>
                  <LicenseSection />
                </CardContent>
              </Card>

              {/* Team members */}
              <Card>
                <SCardHeader
                  icon={Users} iconBg="bg-blue-500/10" iconColor="text-blue-400"
                  title="Team Members"
                  description="Manage access roles for your workspace."
                />
                <CardContent className="space-y-4">
                  {isAdmin ? (
                    <TeamSection currentUserId={me?.id ?? ''} />
                  ) : (
                    <p className="text-sm text-muted-foreground py-4 text-center">Admin access required to manage the team.</p>
                  )}
                </CardContent>
              </Card>
            </div>
          )}

          {/* ── External Access ── */}
          {activeTab === 'access' && (
            <Card>
              <SCardHeader
                icon={UserCheck} iconBg="bg-teal-500/10" iconColor="text-teal-400"
                title="Auditor Access Portal"
                description="Token-based read-only portal for external auditors — no account required. Use for SOC 2, ISO 27001, and other compliance reviews."
              />
              <CardContent>
                <AuditorAccessSection />
              </CardContent>
            </Card>
          )}

          {/* ── Modules ── */}
          {activeTab === 'modules' && (
            <>
              <div>
                <h2 className="text-base font-semibold">Module Management</h2>
                <p className="text-sm text-muted-foreground mt-0.5">
                  Control which features and cloud scanners are visible and active in your workspace.
                </p>
              </div>
              <ModulesSection isAdmin={isAdmin} />
            </>
          )}

          {/* ── Activity ── */}
          {activeTab === 'activity' && (
            <Card>
              <SCardHeader
                icon={Activity} iconBg="bg-indigo-500/10" iconColor="text-indigo-400"
                title="Activity Log"
                description="Recent workspace events — audit runs, connections, license changes and team actions. Refreshes every 30 seconds."
              />
              <CardContent>
                <ActivitySection />
              </CardContent>
            </Card>
          )}

        </div>
      </div>
    </div>
  )
}
