import { NavLink, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  LayoutDashboard, Cloud, Code2, AlertTriangle, FileText,
  FolderOpen, Layers, Settings, ShieldCheck, Sparkles, Shield,
  Kanban, ScrollText, Activity, UserCheck, ClipboardCheck,
  LogOut, ChevronRight, Zap,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { modulesApi, licenseApi } from '@/lib/api'
import { useAuthStore } from '@/store/useAuthStore'
import { authApi } from '@/lib/api'
import { toast } from 'sonner'

// ── Nav item ──────────────────────────────────────────────────────────────────

function NavItem({
  to,
  label,
  icon: Icon,
  exact,
}: {
  to: string
  label: string
  icon: React.ElementType
  exact?: boolean
}) {
  return (
    <NavLink
      to={to}
      end={exact}
      className={({ isActive }) =>
        cn(
          'group relative flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-all duration-150',
          isActive
            ? 'bg-indigo-500/10 text-foreground font-medium'
            : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
        )
      }
    >
      {({ isActive }) => (
        <>
          {/* Left accent bar */}
          {isActive && (
            <span className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-r-full bg-indigo-500" />
          )}
          {/* Icon container */}
          <span className={cn(
            'flex h-6 w-6 shrink-0 items-center justify-center rounded-md transition-colors',
            isActive ? 'text-indigo-400' : 'text-muted-foreground group-hover:text-foreground'
          )}>
            <Icon className="h-3.5 w-3.5" />
          </span>
          <span className="flex-1 leading-none">{label}</span>
          {isActive && (
            <span className="h-1.5 w-1.5 rounded-full bg-indigo-500/70 shrink-0" />
          )}
        </>
      )}
    </NavLink>
  )
}

// ── Section label ─────────────────────────────────────────────────────────────

function SectionLabel({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-2 mt-5 mb-1 px-3">
      <span className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/40">
        {label}
      </span>
      <div className="flex-1 h-px bg-border/50" />
    </div>
  )
}

// ── Plan badge ─────────────────────────────────────────────────────────────────

const planMeta: Record<string, { label: string; cls: string }> = {
  community:    { label: 'Free',       cls: 'text-muted-foreground border-border' },
  starter:      { label: 'Starter',    cls: 'text-sky-400 border-sky-500/40' },
  professional: { label: 'Pro',        cls: 'text-indigo-400 border-indigo-500/40' },
  business:     { label: 'Team',       cls: 'text-violet-400 border-violet-500/40' },
  enterprise:   { label: 'Enterprise', cls: 'text-purple-400 border-purple-400/40' },
}

// ── Sidebar ────────────────────────────────────────────────────────────────────

export function Sidebar() {
  const navigate = useNavigate()
  const { user, logout, refreshToken } = useAuthStore()

  const { data: modules } = useQuery({
    queryKey: ['modules'],
    queryFn: modulesApi.getAll,
    staleTime: 60_000,
  })
  const { data: license } = useQuery({
    queryKey: ['license'],
    queryFn: licenseApi.get,
    staleTime: 120_000,
  })

  const on = (key: string) => !modules || modules[key] !== false

  const plan = license?.plan ?? 'community'
  const isFree = plan === 'community'
  const meta = planMeta[plan] ?? planMeta.community

  const handleLogout = async () => {
    try { await authApi.logout(refreshToken ?? undefined) } catch {}
    logout()
    toast.success('Logged out')
    navigate('/login')
  }

  // User avatar initials
  const initials = user?.email
    ? user.email.slice(0, 2).toUpperCase()
    : '??'

  return (
    <aside className="flex h-full w-[220px] flex-col sidebar-bg border-r">

      {/* ── Logo ── */}
      <div className="flex h-[60px] items-center gap-2.5 border-b px-5 shrink-0">
        <div className="relative flex h-8 w-8 items-center justify-center rounded-xl bg-indigo-500/15 ring-1 ring-indigo-500/30">
          <ShieldCheck className="h-4.5 w-4.5 text-indigo-400" strokeWidth={2} />
          <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-green-500 ring-1 ring-sidebar-bg" />
        </div>
        <div className="leading-none">
          <span className="text-sm font-bold tracking-tight">CloudSec</span>
          <span className="text-sm font-bold tracking-tight text-indigo-400">Guard</span>
        </div>
      </div>

      {/* ── Navigation ── */}
      <nav className="flex-1 overflow-y-auto py-3 px-2 space-y-0.5">

        <NavItem to="/" label="Overview" icon={LayoutDashboard} exact />

        {/* Security Center */}
        <SectionLabel label="Security" />
        {on('cloud_audits') && <NavItem to="/cloud-audits" label="Cloud Audits"  icon={Cloud} />}
        {on('code_iac')     && <NavItem to="/code-iac"     label="Code & IaC"    icon={Code2} />}
        {on('findings')     && <NavItem to="/findings"     label="Findings"      icon={AlertTriangle} />}
        {on('remediation')  && <NavItem to="/remediation"  label="Remediation"   icon={Kanban} />}
        {on('monitoring')   && <NavItem to="/monitoring"   label="Monitoring"    icon={Activity} />}

        {/* Compliance */}
        <SectionLabel label="Compliance" />
        {on('compliance')     && <NavItem to="/compliance"    label="Frameworks"    icon={ClipboardCheck} />}
        {on('evidence')       && <NavItem to="/evidence"      label="Evidence"      icon={FolderOpen} />}
        {on('policies')       && <NavItem to="/policies"      label="Policies"      icon={ScrollText} />}
        {on('access_reviews') && <NavItem to="/access-reviews" label="Access Reviews" icon={UserCheck} />}

        {/* Reports */}
        <SectionLabel label="Reports" />
        {on('reports')     && <NavItem to="/reports"     label="Reports"     icon={FileText} />}
        {on('audit_types') && <NavItem to="/audit-types" label="Audit Types" icon={Layers} />}

        {/* Account */}
        <SectionLabel label="Account" />
        <NavItem to="/plans"    label="Plans"    icon={Sparkles} />
        <NavItem to="/settings" label="Settings" icon={Settings} />
        <NavItem to="/privacy"  label="Privacy"  icon={Shield} />

        {/* Free plan upgrade nudge */}
        {isFree && (
          <div className="mt-3 mx-1">
            <button
              onClick={() => navigate('/plans')}
              className="w-full flex items-center gap-2 rounded-lg border border-indigo-500/25 bg-indigo-500/8 px-3 py-2.5 text-left text-xs transition-colors hover:bg-indigo-500/15"
            >
              <Zap className="h-3.5 w-3.5 text-indigo-400 shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="font-semibold text-indigo-300 leading-none mb-0.5">Upgrade to Pro</p>
                <p className="text-muted-foreground/60 leading-none truncate">Unlock all features →</p>
              </div>
            </button>
          </div>
        )}
      </nav>

      {/* ── User profile ── */}
      <div className="shrink-0 border-t p-3">
        <div className="flex items-center gap-2.5 rounded-lg px-2 py-2">
          {/* Avatar */}
          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-indigo-500/20 text-[11px] font-bold text-indigo-300 ring-1 ring-indigo-500/30">
            {initials}
          </div>
          {/* Info */}
          <div className="flex-1 min-w-0">
            <p className="text-xs font-medium truncate leading-none mb-0.5">{user?.email ?? '—'}</p>
            <div className="flex items-center gap-1.5">
              <span className={cn('inline-flex items-center rounded-full border px-1.5 py-0.5 text-[9px] font-semibold leading-none', meta.cls)}>
                {meta.label}
              </span>
              <span className="text-[10px] text-muted-foreground/50 capitalize">{user?.role}</span>
            </div>
          </div>
          {/* Logout */}
          <button
            onClick={handleLogout}
            title="Sign out"
            className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-muted-foreground/40 transition-colors hover:bg-muted hover:text-foreground"
          >
            <LogOut className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </aside>
  )
}
