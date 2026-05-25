import { Outlet, useLocation } from 'react-router-dom'
import { Moon, Sun, Bell } from 'lucide-react'
import { Sidebar } from './Sidebar'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/store/useAuthStore'
import { useThemeStore } from '@/store/useThemeStore'

// Route → display title map
const routeTitles: Record<string, string> = {
  '/':               'Overview',
  '/cloud-audits':   'Cloud Audits',
  '/code-iac':       'Code & IaC',
  '/findings':       'Findings',
  '/remediation':    'Remediation',
  '/monitoring':     'Monitoring',
  '/compliance':     'Compliance',
  '/evidence':       'Evidence',
  '/policies':       'Policies',
  '/access-reviews': 'Access Reviews',
  '/reports':        'Reports',
  '/audit-types':    'Audit Types',
  '/plans':          'Plans & Pricing',
  '/settings':       'Settings',
  '/privacy':        'Privacy & Security',
  '/connections':    'Connections',
  '/jobs':           'Audit Jobs',
}

function getTitle(pathname: string): string {
  if (routeTitles[pathname]) return routeTitles[pathname]
  // Match prefixes
  for (const [prefix, label] of Object.entries(routeTitles)) {
    if (prefix !== '/' && pathname.startsWith(prefix)) return label
  }
  return 'CloudSecGuard'
}

export function AppLayout() {
  const { user } = useAuthStore()
  const { theme, toggle } = useThemeStore()
  const { pathname } = useLocation()

  const title = getTitle(pathname)
  const initials = user?.email ? user.email.slice(0, 2).toUpperCase() : '??'

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <Sidebar />

      <div className="flex flex-1 flex-col overflow-hidden min-w-0">

        {/* ── Top bar ── */}
        <header className="flex h-[60px] shrink-0 items-center justify-between border-b bg-card/80 backdrop-blur px-5 gap-4">
          {/* Left — page title */}
          <div className="flex items-center gap-2 min-w-0">
            <h2 className="text-sm font-semibold text-foreground truncate">{title}</h2>
          </div>

          {/* Right — controls */}
          <div className="flex items-center gap-1 shrink-0">
            {/* Bell (decorative, ready for future notifications) */}
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              title="Notifications"
            >
              <Bell className="h-4 w-4" />
            </Button>

            {/* Theme toggle */}
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              onClick={toggle}
              title="Toggle theme"
            >
              {theme === 'dark'
                ? <Sun className="h-4 w-4" />
                : <Moon className="h-4 w-4" />
              }
            </Button>

            {/* User chip */}
            <div className="ml-1 flex items-center gap-2 rounded-lg border bg-muted/30 px-2.5 py-1.5">
              <div className="flex h-5 w-5 items-center justify-center rounded-full bg-indigo-500/20 text-[9px] font-bold text-indigo-300">
                {initials}
              </div>
              <span className="text-xs text-muted-foreground max-w-[140px] truncate hidden sm:block">
                {user?.email}
              </span>
            </div>
          </div>
        </header>

        {/* ── Page content ── */}
        <main className="flex-1 overflow-y-auto">
          <div className="p-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
