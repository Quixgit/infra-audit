import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Cloud, Code2, Lock, Globe, Server, Layers, Hexagon,
  Bell, Lock as LockIcon,
} from 'lucide-react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import { licenseApi, notifyApi } from '@/lib/api'
import { useAuthStore } from '@/store/useAuthStore'

type AuditTypeStatus = 'available' | 'coming-soon'

interface AuditTypeCard {
  id: string
  name: string
  description: string
  checks: string[]
  status: AuditTypeStatus
  icon: React.ElementType
  iconBg: string
  iconColor: string
  connectPath?: string
  requiresFeature?: string
}

const auditTypes: AuditTypeCard[] = [
  {
    id: 'do',
    name: 'DigitalOcean',
    description: 'Scan your DigitalOcean account for security misconfigurations across firewalls, databases, apps, secrets and more.',
    checks: ['Firewall rules', 'Database exposure', 'App Platform', 'Secrets & tokens', 'Reserved IPs', 'Spaces buckets'],
    status: 'available',
    icon: Cloud,
    iconBg: 'bg-indigo-500/10',
    iconColor: 'text-indigo-400',
    connectPath: '/connections/new?type=do',
  },
  {
    id: 'ssl',
    name: 'SSL / TLS',
    description: 'Check your domains for certificate expiry, deprecated TLS versions, weak cipher suites and missing HSTS headers.',
    checks: ['Certificate expiry', 'TLS 1.0 / 1.1', 'Weak ciphers', 'HSTS header', 'HTTP→HTTPS redirect', 'Invalid certs'],
    status: 'available',
    icon: Lock,
    iconBg: 'bg-blue-500/10',
    iconColor: 'text-blue-400',
    connectPath: '/connections/new?type=ssl',
  },
  {
    id: 'dns',
    name: 'DNS Security',
    description: 'Audit your DNS configuration for email spoofing risks, zone transfer vulnerabilities, open resolvers and more.',
    checks: ['SPF record', 'DMARC policy', 'DKIM keys', 'Zone transfer (AXFR)', 'Open resolver', 'CAA records'],
    status: 'available',
    icon: Globe,
    iconBg: 'bg-orange-500/10',
    iconColor: 'text-orange-400',
    connectPath: '/connections/new?type=dns',
  },
  {
    id: 'code',
    name: 'Code & IaC',
    description: 'Static analysis of your codebase — secrets in code, vulnerable dependencies, Dockerfile security, GitHub Actions, Terraform.',
    checks: ['Secret scanning', 'SAST', 'Dependency CVEs', 'Dockerfile', 'Terraform IaC', 'GitHub Actions'],
    status: 'available',
    icon: Code2,
    iconBg: 'bg-purple-500/10',
    iconColor: 'text-purple-400',
    connectPath: '/connections/new?type=code',
    requiresFeature: 'code_audit',
  },
  {
    id: 'aws',
    name: 'AWS',
    description: 'Scan AWS account for IAM misconfigurations, open S3 buckets, unencrypted RDS, exposed EC2 security groups.',
    checks: ['IAM policies', 'S3 buckets', 'RDS encryption', 'EC2 / SGs', 'CloudTrail', 'Root account MFA'],
    status: 'coming-soon',
    icon: Server,
    iconBg: 'bg-yellow-500/10',
    iconColor: 'text-yellow-400',
  },
  {
    id: 'gcp',
    name: 'Google Cloud',
    description: 'Audit GCP resources — GKE clusters, Cloud SQL, Storage buckets, IAM roles and VPC firewall rules.',
    checks: ['GKE security', 'Cloud SQL', 'GCS buckets', 'IAM roles', 'VPC rules', 'Logging'],
    status: 'coming-soon',
    icon: Layers,
    iconBg: 'bg-sky-500/10',
    iconColor: 'text-sky-400',
  },
  {
    id: 'azure',
    name: 'Azure',
    description: 'Scan Azure subscriptions — VM exposure, storage accounts, RBAC, Key Vault and network security groups.',
    checks: ['VMs & NSGs', 'Storage accounts', 'RBAC', 'Key Vault', 'Defender', 'Activity logs'],
    status: 'coming-soon',
    icon: Layers,
    iconBg: 'bg-blue-600/10',
    iconColor: 'text-blue-500',
  },
  {
    id: 'k8s',
    name: 'Kubernetes',
    description: 'Audit Kubernetes clusters — RBAC, pod security, network policies, exposed dashboards and secrets in env vars.',
    checks: ['RBAC config', 'Pod security', 'Network policies', 'Exposed dashboards', 'Secrets in env', 'CIS Benchmark'],
    status: 'coming-soon',
    icon: Hexagon,
    iconBg: 'bg-violet-500/10',
    iconColor: 'text-violet-400',
  },
]

function NotifyDialog({
  open, onClose, auditType, userEmail,
}: {
  open: boolean
  onClose: () => void
  auditType: AuditTypeCard | null
  userEmail: string
}) {
  const [email, setEmail] = useState(userEmail)
  const mutation = useMutation({
    mutationFn: () => notifyApi.request(auditType?.id ?? '', email),
    onSuccess: () => { toast.success("We'll notify you when it's available!"); onClose() },
    onError: () => toast.error('Failed to save notification request'),
  })
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Notify me — {auditType?.name}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <p className="text-sm text-muted-foreground">
            We'll email you when <span className="font-medium text-foreground">{auditType?.name}</span> is ready.
          </p>
          <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="your@email.com" />
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onClose}>Cancel</Button>
          <Button size="sm" onClick={() => mutation.mutate()} disabled={!email || mutation.isPending}>
            <Bell className="mr-2 h-3.5 w-3.5" />Notify Me
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function AuditTypes() {
  const navigate = useNavigate()
  const { user } = useAuthStore()
  const [notifyTarget, setNotifyTarget] = useState<AuditTypeCard | null>(null)

  const { data: license } = useQuery({ queryKey: ['license'], queryFn: licenseApi.get })
  const hasCodeAudit = license?.features?.includes('code_audit') ?? false

  const available = auditTypes.filter((t) => t.status === 'available')
  const comingSoon = auditTypes.filter((t) => t.status === 'coming-soon')

  return (
    <div className="space-y-6 max-w-5xl">
      <div>
        <h1 className="text-2xl font-bold">Audit Types</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Choose the type of security audit to run on your infrastructure
        </p>
      </div>

      {/* Available now */}
      <div>
        <h2 className="text-base font-semibold mb-3 flex items-center gap-2">
          <Cloud className="h-4 w-4 text-indigo-400" />
          Available now
        </h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {available.map((type) => {
            const Icon = type.icon
            const locked = type.requiresFeature === 'code_audit' && !hasCodeAudit
            return (
              <Card key={type.id} className="hover:border-indigo-500/40 transition-colors">
                <CardContent className="p-4">
                  {/* Top row: icon + name + button */}
                  <div className="flex items-start gap-3">
                    <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${type.iconBg} ${type.iconColor}`}>
                      <Icon className="h-5 w-5" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-semibold leading-tight">{type.name}</p>
                      <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{type.description}</p>
                    </div>
                    {locked ? (
                      <Button
                        size="sm"
                        variant="outline"
                        className="shrink-0 h-7 text-xs opacity-60"
                        disabled
                        title="Requires Professional plan"
                      >
                        <LockIcon className="mr-1 h-3 w-3" />Pro
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        variant="outline"
                        className="shrink-0 h-7 text-xs"
                        onClick={() => navigate(type.connectPath!)}
                      >
                        Start audit
                      </Button>
                    )}
                  </div>
                  {/* Checks */}
                  <div className="mt-3 flex flex-wrap gap-1.5 pl-12">
                    {type.checks.map((check) => (
                      <span
                        key={check}
                        className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                      >
                        {check}
                      </span>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      </div>

      {/* Coming soon */}
      <div>
        <h2 className="text-base font-semibold mb-3 flex items-center gap-2">
          <Bell className="h-4 w-4 text-muted-foreground" />
          Coming soon
        </h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {comingSoon.map((type) => {
            const Icon = type.icon
            return (
              <Card key={type.id} className="opacity-70">
                <CardContent className="p-4">
                  {/* Top row: icon + name + button */}
                  <div className="flex items-start gap-3">
                    <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${type.iconBg} ${type.iconColor}`}>
                      <Icon className="h-5 w-5" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-semibold leading-tight">{type.name}</p>
                      <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{type.description}</p>
                    </div>
                    <Button
                      size="sm"
                      variant="outline"
                      className="shrink-0 h-7 text-xs"
                      onClick={() => setNotifyTarget(type)}
                    >
                      <Bell className="mr-1 h-3 w-3" />Notify me
                    </Button>
                  </div>
                  {/* Checks */}
                  <div className="mt-3 flex flex-wrap gap-1.5 pl-12">
                    {type.checks.map((check) => (
                      <span
                        key={check}
                        className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                      >
                        {check}
                      </span>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      </div>

      <NotifyDialog
        open={!!notifyTarget}
        onClose={() => setNotifyTarget(null)}
        auditType={notifyTarget}
        userEmail={user?.email ?? ''}
      />
    </div>
  )
}
