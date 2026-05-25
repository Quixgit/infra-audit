import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  Shield, Lock, Database, Key, Trash2, FileDown, AlertTriangle,
  CheckCircle2, Clock, ServerCrash, ShieldAlert,
} from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { authApi } from '@/lib/api'
import { useAuthStore } from '@/store/useAuthStore'

function SectionTitle({ icon: Icon, title }: { icon: React.ElementType; title: string }) {
  return (
    <div className="flex items-center gap-2 mb-4">
      <Icon className="h-4 w-4 text-indigo-400" />
      <h2 className="text-base font-semibold">{title}</h2>
    </div>
  )
}

function DataRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between py-2.5 border-b border-border/50 last:border-0">
      <span className="text-sm text-muted-foreground w-48 shrink-0">{label}</span>
      <span className="text-sm text-right">{value}</span>
    </div>
  )
}

const securityPractices = [
  {
    icon: Lock,
    title: 'Credentials encrypted at rest',
    description: 'DigitalOcean API tokens are AES-256 encrypted before storage. They are never logged or returned in plaintext after initial submission.',
  },
  {
    icon: Key,
    title: 'Passwords hashed with bcrypt',
    description: 'User passwords are stored only as bcrypt hashes (cost 10). CloudSecGuard never stores your plaintext password.',
  },
  {
    icon: Database,
    title: 'Audit data stays on your server',
    description: 'All audit findings, HTML/DOCX reports, and collected cloud inventory are stored only on your self-hosted instance. Nothing is transmitted to CloudSecGuard servers.',
  },
  {
    icon: Shield,
    title: 'API tokens hashed, never stored whole',
    description: 'Personal API tokens are shown once at creation and stored only as SHA-256 hashes. Lost tokens cannot be recovered — create a new one.',
  },
  {
    icon: ServerCrash,
    title: 'Evidence files are sensitive',
    description: 'Raw cloud inventory JSON exports may contain sensitive configuration values. They are stored in the data directory and should be protected with appropriate filesystem permissions.',
  },
  {
    icon: ShieldAlert,
    title: 'Share links are public by design',
    description: 'When you create a share link for a report, the linked report is accessible to anyone with the URL — no authentication required. Revoke links that are no longer needed.',
  },
]

export function Privacy() {
  const navigate = useNavigate()
  const { user } = useAuthStore()
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const { data: me } = useQuery({
    queryKey: ['me'],
    queryFn: authApi.me,
  })

  return (
    <div className="space-y-6 max-w-3xl">
      <div>
        <h1 className="text-2xl font-bold">Privacy & Security</h1>
        <p className="text-sm text-muted-foreground mt-1">
          How CloudSecGuard handles your data and how to keep your account secure.
        </p>
      </div>

      {/* Account security */}
      <Card>
        <CardHeader className="pb-3">
          <SectionTitle icon={Lock} title="Account Security" />
        </CardHeader>
        <CardContent className="space-y-0">
          <DataRow
            label="Email address"
            value={<span className="font-mono text-xs">{me?.email ?? '—'}</span>}
          />
          <DataRow
            label="Account role"
            value={
              <Badge variant="secondary" className="text-xs capitalize">
                {me?.role ?? '—'}
              </Badge>
            }
          />
          <DataRow
            label="Password"
            value={
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs"
                onClick={() => navigate('/settings')}
              >
                Change password →
              </Button>
            }
          />
          <DataRow
            label="Two-factor authentication"
            value={
              <div className="flex items-center gap-2">
                <Badge variant="secondary" className="text-xs">Coming Soon</Badge>
                <span className="text-xs text-muted-foreground">Not available</span>
              </div>
            }
          />
          <DataRow
            label="API tokens"
            value={
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs"
                onClick={() => navigate('/settings')}
              >
                Manage tokens →
              </Button>
            }
          />
        </CardContent>
      </Card>

      {/* What we store */}
      <Card>
        <CardHeader className="pb-3">
          <SectionTitle icon={Database} title="What CloudSecGuard Stores" />
        </CardHeader>
        <CardContent className="space-y-0">
          <DataRow label="Account data" value="Email, bcrypt password hash, auditor profile fields" />
          <DataRow label="Connections" value="Name, encrypted API token, repository URL (no plaintext secrets)" />
          <DataRow label="Audit jobs" value="Status, timestamps, finding counts, paths to report files on disk" />
          <DataRow label="Finding overrides" value="Status changes you make (Open → Fixed etc.) stored in PostgreSQL" />
          <DataRow label="Report files" value="HTML and DOCX reports, findings JSON — stored in the data directory on your host" />
          <DataRow label="Refresh tokens" value="SHA-256 hash only, expires after 30 days" />
          <DataRow label="Share links" value="Random token linked to a job — no expiry unless manually deleted" />
          <DataRow label="Schedules" value="Interval, enabled state, next run time" />
        </CardContent>
      </Card>

      {/* Security practices */}
      <Card>
        <CardHeader className="pb-3">
          <SectionTitle icon={Shield} title="Security Practices" />
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {securityPractices.map((p) => (
              <div key={p.title} className="flex gap-3">
                <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted">
                  <p.icon className="h-3.5 w-3.5 text-indigo-400" />
                </div>
                <div>
                  <p className="text-sm font-medium">{p.title}</p>
                  <p className="text-xs text-muted-foreground mt-0.5">{p.description}</p>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Data retention */}
      <Card>
        <CardHeader className="pb-3">
          <SectionTitle icon={Clock} title="Data Retention" />
        </CardHeader>
        <CardContent className="space-y-0">
          <DataRow label="Audit jobs" value="Stored indefinitely — no automatic deletion" />
          <DataRow label="Report files" value="Deleted when the job record is deleted" />
          <DataRow label="Refresh tokens" value="Auto-deleted after 30 days" />
          <DataRow label="Finding overrides" value="Deleted when the parent job is deleted" />
          <DataRow label="Share links" value="Deleted when the parent job is deleted" />
        </CardContent>
      </Card>

      {/* Compliance note */}
      <Card className="border-border/50 bg-muted/20">
        <CardContent className="py-4">
          <div className="flex gap-3">
            <CheckCircle2 className="h-4 w-4 text-indigo-400 shrink-0 mt-0.5" />
            <div className="text-xs text-muted-foreground space-y-1">
              <p>CloudSecGuard is a <span className="font-medium text-foreground">self-hosted</span> tool. Your audit data never leaves your infrastructure — all processing happens on the server you control.</p>
              <p>We recommend restricting filesystem access to the data directory, using TLS in front of CloudSecGuard, and rotating API tokens periodically.</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Danger zone */}
      <Card className="border-destructive/30">
        <CardHeader className="pb-3">
          <div className="flex items-center gap-2">
            <AlertTriangle className="h-4 w-4 text-destructive" />
            <CardTitle className="text-base text-destructive">Danger Zone</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          {!showDeleteConfirm ? (
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Delete my account</p>
                <p className="text-xs text-muted-foreground mt-0.5">
                  Permanently removes your account, all connections, jobs, and reports.
                </p>
              </div>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setShowDeleteConfirm(true)}
              >
                <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                Delete account
              </Button>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-destructive font-medium">
                Are you sure? This cannot be undone. All data will be permanently deleted.
              </p>
              <div className="flex gap-2">
                <Button
                  variant="destructive"
                  size="sm"
                  disabled
                >
                  Coming Soon
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowDeleteConfirm(false)}
                >
                  Cancel
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
