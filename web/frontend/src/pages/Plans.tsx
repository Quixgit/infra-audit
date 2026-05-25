import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Check, X, KeyRound, Loader2, Sparkles, ShieldCheck, Zap, ArrowRight,
  Server, Lock, Infinity,
} from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { licenseApi, type LicenseFeature } from '@/lib/api'
import { useAuthStore } from '@/store/useAuthStore'
import { formatDate } from '@/lib/utils'

// ── Plan definitions ──────────────────────────────────────────────────────────

type PlanDef = {
  key: string
  label: string
  tagline: string
  price: string
  priceSub: string
  priceAnnual: string | null
  highlight: string
  ring: string
  badge: string
  badgeText: string
  maxConnections: string
  maxAudits: string
  maxUsers: string
  features: string[]
}

const plans: PlanDef[] = [
  {
    key: 'community',
    label: 'Free',
    tagline: 'For solo developers & indie hackers',
    price: 'Free',
    priceSub: 'forever',
    priceAnnual: null,
    highlight: 'border-border',
    ring: '',
    badge: 'border-border text-muted-foreground bg-muted/50',
    badgeText: '',
    maxConnections: '5',
    maxAudits: '20 / month',
    maxUsers: '1',
    features: ['basic_audit', 'basic_report', 'share_links'],
  },
  {
    key: 'starter',
    label: 'Starter',
    tagline: 'For freelancers & consultants',
    price: '$19',
    priceSub: '/ month',
    priceAnnual: '$190 / year',
    highlight: 'border-border',
    ring: '',
    badge: 'border-border text-muted-foreground bg-muted/50',
    badgeText: '',
    maxConnections: '10',
    maxAudits: '30 / month',
    maxUsers: '2',
    features: [
      'basic_audit', 'basic_report',
      'scheduled_audits', 'share_links', 'pdf_reports',
    ],
  },
  {
    key: 'professional',
    label: 'Pro',
    tagline: 'For startups & small teams',
    price: '$49',
    priceSub: '/ month',
    priceAnnual: '$490 / year',
    highlight: 'border-indigo-500/40',
    ring: 'shadow-[0_0_0_2px_rgba(99,102,241,0.3)]',
    badge: 'border-indigo-500/60 text-indigo-400 bg-indigo-500/5',
    badgeText: 'Most popular',
    maxConnections: '30',
    maxAudits: '100 / month',
    maxUsers: '5',
    features: [
      'basic_audit', 'basic_report',
      'scheduled_audits', 'share_links', 'pdf_reports',
      'code_audit', 'compliance_basic', 'custom_branding',
    ],
  },
  {
    key: 'business',
    label: 'Team',
    tagline: 'For growing teams & scaleups',
    price: '$99',
    priceSub: '/ month',
    priceAnnual: '$990 / year',
    highlight: 'border-violet-500/40',
    ring: 'shadow-[0_0_0_2px_rgba(139,92,246,0.3)]',
    badge: 'border-violet-500/60 text-violet-400 bg-violet-500/5',
    badgeText: 'Best value',
    maxConnections: 'Unlimited',
    maxAudits: 'Unlimited',
    maxUsers: '15',
    features: [
      'basic_audit', 'basic_report',
      'scheduled_audits', 'share_links', 'pdf_reports',
      'code_audit', 'compliance_basic', 'custom_branding',
      'api_tokens', 'team', 'evidence', 'policies',
      'access_reviews', 'remediation', 'priority_support',
    ],
  },
]

// ── Feature rows ──────────────────────────────────────────────────────────────

type FeatureRow = {
  key: string
  label: string
  description: string
  comingSoon?: boolean
  group?: string
}

const featureRows: FeatureRow[] = [
  // Core
  { key: 'basic_audit',       label: 'Cloud audit',          description: 'Audit cloud infrastructure for security misconfigurations',              group: 'Core' },
  { key: 'basic_report',      label: 'Audit reports',        description: 'View audit results in the dashboard',                                    group: 'Core' },
  { key: 'scheduled_audits',  label: 'Scheduled audits',     description: 'Run audits automatically on daily, weekly, or monthly schedules',        group: 'Core' },
  { key: 'code_audit',        label: 'Code & IaC audit',     description: 'Scan git repos for secrets, SAST issues, Terraform misconfigurations',   group: 'Core' },
  { key: 'share_links',       label: 'Share links',          description: 'Share audit reports with clients via a public link',                     group: 'Core' },
  { key: 'pdf_reports',       label: 'PDF / HTML export',    description: 'Download full reports in PDF and HTML formats',                          group: 'Core' },
  { key: 'compliance_basic',  label: 'Compliance mapping',   description: 'Map findings to SOC 2, ISO 27001, and other compliance frameworks',      group: 'Core' },
  // Business
  { key: 'custom_branding',   label: 'Custom branding',      description: 'Upload your logo and watermark to generated reports',                   group: 'Business' },
  { key: 'api_tokens',        label: 'API access',           description: 'Programmatic API access for CI/CD and automation',                      group: 'Business' },
  { key: 'team',              label: 'Team management',      description: 'Invite team members with role-based access control',                    group: 'Business' },
  { key: 'evidence',          label: 'Evidence library',     description: 'Upload and manage evidence linked to compliance controls',               group: 'Business' },
  { key: 'policies',          label: 'Policy management',    description: 'Create, approve, and track security policies',                           group: 'Business' },
  { key: 'access_reviews',    label: 'Access reviews',       description: 'Periodic user access reviews for SOC 2 and ISO 27001',                  group: 'Business' },
  { key: 'remediation',       label: 'Remediation board',    description: 'Kanban board to track and resolve security findings',                   group: 'Business' },
  { key: 'priority_support',  label: 'Priority support',     description: 'Faster response times and dedicated email support',                     group: 'Business' },
  // Enterprise
  { key: 'sso',               label: 'SSO / SAML',           description: 'Single sign-on via SAML 2.0 for enterprise identity providers',         group: 'Enterprise', comingSoon: true },
  { key: 'custom_frameworks', label: 'Custom frameworks',    description: 'Define your own compliance frameworks and control mappings',             group: 'Enterprise' },
  { key: 'white_label',       label: 'White-label reports',  description: 'Fully white-labeled reports and portal with your brand',                group: 'Enterprise' },
  { key: 'self_hosted',       label: 'Self-hosted option',   description: 'Deploy on your own infrastructure — full data sovereignty',             group: 'Enterprise', comingSoon: true },
  { key: 'dedicated_support', label: 'Dedicated support',    description: 'Dedicated account manager and SLA guarantee',                           group: 'Enterprise' },
  { key: 'human_review',      label: 'Human review add-on',  description: 'Expert security review of your audit results by the CloudSecGuard team', group: 'Enterprise', comingSoon: true },
]

const featureGroups = ['Core', 'Business', 'Enterprise'] as const

// ── Helpers ───────────────────────────────────────────────────────────────────

function FeatureCheck({ has, comingSoon }: { has: boolean; comingSoon?: boolean }) {
  if (comingSoon && has) return (
    <span className="inline-flex items-center rounded-full border border-yellow-400/40 bg-yellow-400/10 px-1.5 py-0.5 text-[9px] font-medium text-yellow-400 leading-none">
      soon
    </span>
  )
  if (has) return <Check className="h-4 w-4 text-indigo-400 shrink-0 mx-auto" />
  return <X className="h-4 w-4 text-muted-foreground/25 shrink-0 mx-auto" />
}

// ── Main Component ────────────────────────────────────────────────────────────

export function Plans() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const { user } = useAuthStore()
  const isAdmin = user?.role === 'admin' || user?.role === 'owner'
  const [keyInput, setKeyInput] = useState('')

  const { data: license, isLoading } = useQuery({
    queryKey: ['license'],
    queryFn: licenseApi.get,
  })

  const activateMutation = useMutation({
    mutationFn: () => licenseApi.activate(keyInput.trim()),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['license'] })
      setKeyInput('')
      toast.success('License activated successfully')
    },
    onError: (err: any) => {
      const msg = err?.response?.data?.error ?? 'Invalid license key'
      toast.error(msg)
    },
  })

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const activePlan = license?.plan ?? 'community'
  const activeFeatures: LicenseFeature[] = license?.features ?? []
  const isCommunity = activePlan === 'community'

  return (
    <div className="space-y-8 max-w-6xl">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold">Plans & Pricing</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Simple pricing for solo developers, freelancers and small teams. No hidden fees, cancel anytime.
        </p>
      </div>

      {/* Active plan banner */}
      {!isCommunity && license && (
        <div className="flex items-center gap-3 rounded-lg border border-indigo-500/20 bg-indigo-500/5 px-4 py-3">
          <ShieldCheck className="h-4 w-4 text-indigo-400 shrink-0" />
          <div className="flex-1 text-sm">
            <span className="font-medium capitalize">{activePlan} plan active</span>
            {license.issued_to && <span className="text-muted-foreground ml-2">— issued to {license.issued_to}</span>}
            {license.expires_at && <span className="text-muted-foreground ml-2">· expires {formatDate(license.expires_at)}</span>}
          </div>
        </div>
      )}

      {isCommunity && (
        <div className="flex items-start gap-3 rounded-lg border border-yellow-500/20 bg-yellow-500/5 px-4 py-3">
          <Zap className="h-4 w-4 text-yellow-400 mt-0.5 shrink-0" />
          <div className="text-sm">
            <span className="font-medium text-yellow-400">You're on the Free plan.</span>
            <span className="text-muted-foreground ml-2">
              Upgrade from $19/mo to unlock scheduled audits, share links, PDF exports, code scanning and more.
            </span>
          </div>
        </div>
      )}

      {/* ── Plan cards ── */}
      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4 pt-4">
        {plans.map((plan) => {
          const isCurrent = activePlan === plan.key
          const hasBadge = isCurrent || !!plan.badgeText
          return (
            <div key={plan.key} className="relative flex flex-col">
              {hasBadge && (
                <div className="absolute -top-3 left-1/2 -translate-x-1/2 z-20">
                  {isCurrent ? (
                    <Badge className={`${plan.badge} bg-card border text-xs px-3 py-0.5 font-medium whitespace-nowrap`}>
                      ✓ Current plan
                    </Badge>
                  ) : (
                    <Badge className={`bg-card border ${plan.badge} text-xs px-3 py-0.5 whitespace-nowrap font-medium`}>
                      {plan.badgeText}
                    </Badge>
                  )}
                </div>
              )}

              <div className={`flex flex-col flex-1 rounded-xl border bg-card transition-all ${
                isCurrent
                  ? `${plan.highlight} ${plan.ring} border-2`
                  : plan.key === 'community'
                  ? 'border-border opacity-80 hover:opacity-100'
                  : 'border-border hover:border-foreground/20'
              }`}>
                <div className="p-5 space-y-3 flex-1">
                  {/* Header */}
                  <div>
                    <div className="flex items-center justify-between mb-0.5">
                      <span className="font-semibold text-base">{plan.label}</span>
                      {plan.key === 'enterprise' && <Sparkles className="h-4 w-4 text-purple-400" />}
                    </div>
                    <p className="text-xs text-muted-foreground">{plan.tagline}</p>
                    <div className="mt-3">
                      <span className="text-2xl font-bold">{plan.price}</span>
                      <span className="text-sm text-muted-foreground ml-1">{plan.priceSub}</span>
                      {plan.priceAnnual && (
                        <p className="text-[11px] text-muted-foreground/60 mt-0.5">or {plan.priceAnnual}</p>
                      )}
                    </div>
                  </div>

                  <Separator />

                  {/* Limits */}
                  <div className="space-y-1.5 text-xs">
                    {[
                      { label: 'Connections', value: plan.maxConnections },
                      { label: 'Audits', value: plan.maxAudits },
                      { label: 'Users', value: plan.maxUsers },
                    ].map(({ label, value }) => (
                      <div key={label} className="flex justify-between items-center">
                        <span className="text-muted-foreground">{label}</span>
                        <span className={`font-semibold ${value === 'Unlimited' ? 'text-indigo-400' : ''}`}>{value}</span>
                      </div>
                    ))}
                  </div>

                  <Separator />

                  {/* Key features (condensed) */}
                  <div className="space-y-1.5">
                    {featureRows.filter(r => plan.features.includes(r.key)).slice(0, 7).map((row) => (
                      <div key={row.key} className="flex items-center gap-2 text-xs">
                        <Check className={`h-3.5 w-3.5 shrink-0 ${
                          plan.key === 'enterprise' ? 'text-purple-400'
                          : plan.key === 'business' ? 'text-violet-400'
                          : plan.key === 'professional' ? 'text-indigo-400'
                          : 'text-muted-foreground/50'
                        }`} />
                        <span className="text-foreground/75">{row.label}</span>
                      </div>
                    ))}
                    {plan.features.length > 7 && (
                      <p className="text-[11px] text-muted-foreground/50 pl-5">
                        +{plan.features.length - 7} more features
                      </p>
                    )}
                  </div>
                </div>

                {/* CTA */}
                <div className="px-4 pb-4">
                  {isCurrent ? (
                    <div className="w-full text-center text-xs text-muted-foreground py-2 rounded-md border bg-muted/30">
                      Active
                    </div>
                  ) : plan.key === 'community' ? (
                    <div className="w-full text-center text-xs text-muted-foreground py-2 rounded-md border border-border">
                      Free forever
                    </div>
                  ) : plan.key === 'enterprise' ? (
                    <a
                      href="mailto:sales@infrajump.io?subject=Enterprise%20Plan%20Inquiry"
                      className="flex items-center justify-center gap-1.5 w-full text-xs font-medium py-2 rounded-md border border-purple-500/40 text-purple-400 hover:bg-purple-500/10 transition-colors"
                    >
                      Contact Sales <ArrowRight className="h-3 w-3" />
                    </a>
                  ) : (
                    <button
                      onClick={() => navigate(`/checkout?plan=${plan.key}`)}
                      className={`flex items-center justify-center gap-1.5 w-full text-xs font-medium py-2 rounded-md border transition-colors ${
                        plan.key === 'business'
                          ? 'border-violet-500/50 text-violet-400 hover:bg-violet-500/10'
                          : plan.key === 'professional'
                          ? 'border-indigo-500/40 text-indigo-400 hover:bg-indigo-500/10'
                          : 'border-border text-muted-foreground hover:bg-muted/30'
                      }`}
                    >
                      Get started <ArrowRight className="h-3 w-3" />
                    </button>
                  )}
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {/* ── Feature comparison table ── */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Full feature comparison</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="py-3 pr-4 text-left font-medium text-muted-foreground w-[38%]">Feature</th>
                  {plans.map((p) => (
                    <th key={p.key} className="py-3 px-2 text-center font-medium min-w-[90px]">
                      <div className="flex flex-col items-center gap-0.5">
                        <span className={activePlan === p.key ? 'text-foreground font-semibold' : 'text-muted-foreground'}>
                          {p.label}
                        </span>
                        {activePlan === p.key && <span className="text-[10px] text-indigo-400">✓ active</span>}
                        <span className="text-[10px] text-muted-foreground/50 font-normal">
                          {p.key === 'community' ? 'Free' : p.price}
                        </span>
                      </div>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {/* Limits rows */}
                {[
                  { label: 'Connections',    values: ['5', '10', '30', 'Unlimited'] },
                  { label: 'Audits / month', values: ['20', '30', '100', 'Unlimited'] },
                  { label: 'Users',          values: ['1', '2', '5', '15'] },
                ].map((row) => (
                  <tr key={row.label} className="border-b border-border/30 hover:bg-muted/20 transition-colors">
                    <td className="py-2.5 pr-4 text-sm text-muted-foreground font-medium">{row.label}</td>
                    {row.values.map((v, i) => (
                      <td key={i} className="py-2.5 px-2 text-center">
                        <span className={`text-xs font-semibold ${v === 'Unlimited' ? 'text-indigo-400' : ''}`}>{v}</span>
                      </td>
                    ))}
                  </tr>
                ))}

                {/* Feature rows grouped */}
                {featureGroups.map((group) => {
                  const rows = featureRows.filter(r => r.group === group)
                  return (
                    <>
                      <tr key={`group-${group}`}>
                        <td colSpan={5} className="pt-5 pb-1.5">
                          <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/40">
                            {group}
                          </span>
                        </td>
                      </tr>
                      {rows.map((row) => (
                        <tr key={row.key} className="border-b border-border/20 hover:bg-muted/20 transition-colors group">
                          <td className="py-2 pr-4">
                            <div>
                              <p className="text-sm font-medium leading-snug">{row.label}</p>
                              <p className="text-xs text-muted-foreground/60 leading-relaxed hidden group-hover:block mt-0.5">
                                {row.description}
                              </p>
                            </div>
                          </td>
                          {plans.map((plan) => {
                            const has = plan.features.includes(row.key)
                            return (
                              <td key={plan.key} className="py-2 px-2 text-center align-middle">
                                <FeatureCheck has={has} comingSoon={row.comingSoon} />
                              </td>
                            )
                          })}
                        </tr>
                      ))}
                    </>
                  )
                })}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* Active features summary */}
      {!isCommunity && activeFeatures.length > 0 && (
        <Card className="border-indigo-500/20 bg-indigo-500/5">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-indigo-400 flex items-center gap-2">
              <ShieldCheck className="h-4 w-4" />Features active on your license
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {featureRows.filter(r => !r.comingSoon).map((row) => {
                const has = activeFeatures.includes(row.key as LicenseFeature)
                return (
                  <div key={row.key} className={`flex items-center gap-2 text-xs ${has ? 'text-foreground' : 'text-muted-foreground/40'}`}>
                    {has
                      ? <Check className="h-3.5 w-3.5 text-indigo-400 shrink-0" />
                      : <X className="h-3.5 w-3.5 shrink-0" />}
                    {row.label}
                  </div>
                )
              })}
            </div>
          </CardContent>
        </Card>
      )}

      {/* License key activation */}
      {isAdmin ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <KeyRound className="h-4 w-4" />Activate License Key
            </CardTitle>
            <p className="text-sm text-muted-foreground">
              Paste your license key to unlock features for all users in this installation.
              Keys are cryptographically signed JWT tokens — contact{' '}
              <a href="mailto:sales@cloudsecguard.com" className="text-indigo-400 hover:underline">sales@cloudsecguard.com</a> to purchase.
            </p>
          </CardHeader>
          <CardContent className="space-y-4">
            {!isCommunity && (
              <div className="text-sm rounded-lg border border-border bg-muted/20 px-4 py-3 text-muted-foreground">
                <span className="font-medium capitalize text-foreground">{activePlan}</span> license is currently active.
                Paste a new key below to replace it.
              </div>
            )}
            <form onSubmit={(e) => { e.preventDefault(); if (keyInput.trim()) activateMutation.mutate() }} className="space-y-3">
              <div className="space-y-1.5">
                <Label htmlFor="license-key">License key</Label>
                <div className="flex gap-2">
                  <Input
                    id="license-key" value={keyInput} onChange={(e) => setKeyInput(e.target.value)}
                    placeholder="eyJhbGciOiJSUzI1NiIsIn..."
                    className="flex-1 font-mono text-xs"
                  />
                  <Button type="submit" disabled={!keyInput.trim() || activateMutation.isPending}>
                    {activateMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Activate'}
                  </Button>
                </div>
              </div>
            </form>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="py-6">
            <div className="flex items-center gap-3 text-sm text-muted-foreground">
              <KeyRound className="h-4 w-4 shrink-0" />
              <p>To upgrade, ask your administrator to activate a license key.</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* How license keys work */}
      <Card className="border-border/40 bg-muted/10">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">How license keys work</CardTitle>
        </CardHeader>
        <CardContent className="text-xs text-muted-foreground space-y-1.5">
          <p>• License keys are cryptographically signed JWT tokens issued by CloudSecGuard.</p>
          <p>• One key activates a plan for the entire installation — all users benefit from the same plan.</p>
          <p>• Monthly subscribers receive one key — it's automatically renewed each billing cycle (no new key).</p>
          <p>• Annual subscribers save ~17% and receive a key valid for 365 days.</p>
          <p>• After expiry the installation reverts to Community. Contact <a href="mailto:sales@cloudsecguard.com" className="text-indigo-400 hover:underline">sales@cloudsecguard.com</a> to purchase or renew.</p>
        </CardContent>
      </Card>

      {/* ── Self-hosted section ── */}
      <div className="relative overflow-hidden rounded-2xl border border-border/60 bg-gradient-to-br from-card to-muted/20 p-8">
        <div className="pointer-events-none absolute inset-0 opacity-5">
          <div className="absolute -right-16 -top-16 h-64 w-64 rounded-full bg-indigo-400 blur-3xl" />
          <div className="absolute -bottom-16 -left-8 h-48 w-48 rounded-full bg-purple-400 blur-3xl" />
        </div>
        <div className="relative flex flex-col sm:flex-row items-start sm:items-center gap-6">
          <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl border border-border/60 bg-card shadow-sm">
            <Server className="h-6 w-6 text-indigo-400" />
          </div>
          <div className="flex-1 space-y-1">
            <div className="flex items-center gap-3 flex-wrap">
              <h3 className="text-lg font-bold">Self-Hosted Edition</h3>
              <span className="inline-flex items-center rounded-full border border-yellow-400/40 bg-yellow-400/10 px-2.5 py-1 text-[10px] font-semibold text-yellow-400">
                In development
              </span>
            </div>
            <p className="text-sm text-muted-foreground max-w-xl">
              Deploy CloudSecGuard entirely on your own infrastructure. Full data sovereignty,
              air-gapped deployments, unlimited users — no cloud dependency. Private Docker image included.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-4">
              {[
                { icon: Lock,     label: 'Full data sovereignty',  sub: 'All data stays on your servers' },
                { icon: Infinity, label: 'Unlimited everything',   sub: 'No caps on users or connections' },
                { icon: Server,   label: 'Air-gapped deployments', sub: 'Works in isolated networks' },
              ].map(({ icon: Icon, label, sub }) => (
                <div key={label} className="flex items-start gap-2.5 rounded-lg border border-border/40 bg-card/60 px-3 py-2.5">
                  <Icon className="h-4 w-4 text-indigo-400 mt-0.5 shrink-0" />
                  <div>
                    <p className="text-xs font-semibold">{label}</p>
                    <p className="text-[11px] text-muted-foreground">{sub}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className="shrink-0">
            <a
              href="mailto:sales@cloudsecguard.com?subject=Self-Hosted%20Edition%20Interest"
              className="inline-flex items-center gap-2 rounded-lg border border-indigo-500/40 bg-indigo-500/5 px-4 py-2.5 text-sm font-medium text-indigo-400 transition-colors hover:bg-indigo-500/10"
            >
              Get notified <ArrowRight className="h-4 w-4" />
            </a>
          </div>
        </div>
      </div>
    </div>
  )
}
