import { useEffect, Component, type ReactNode } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { AppLayout } from '@/components/AppLayout'
import { Login } from '@/pages/Login'
import { Dashboard } from '@/pages/Dashboard'
import { CloudAudits } from '@/pages/CloudAudits'
import { CodeIaC } from '@/pages/CodeIaC'
import { ConnectionForm } from '@/pages/ConnectionForm'
import { AuditTypes } from '@/pages/AuditTypes'
import { Jobs } from '@/pages/Jobs'
import { JobDetail } from '@/pages/JobDetail'
import { Findings } from '@/pages/Findings'
import { Compliance } from '@/pages/Compliance'
import { Remediation } from '@/pages/Remediation'
import { Privacy } from '@/pages/Privacy'
import { Reports } from '@/pages/Reports'
import { Evidence } from '@/pages/Evidence'
import { Settings } from '@/pages/Settings'
import { Plans } from '@/pages/Plans'
import { ShareView } from '@/pages/ShareView'
import { Policies } from '@/pages/Policies'
import { Monitoring } from '@/pages/Monitoring'
import { AccessReviews } from '@/pages/AccessReviews'
import { AccessReviewDetail } from '@/pages/AccessReviewDetail'
import { AuditorPortal } from '@/pages/AuditorPortal'
import { Checkout } from '@/pages/Checkout'
import { useAuthStore } from '@/store/useAuthStore'
import { authApi } from '@/lib/api'

// ── Error Boundary ─────────────────────────────────────────────────────────────
class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  constructor(props: { children: ReactNode }) {
    super(props)
    this.state = { error: null }
  }
  static getDerivedStateFromError(error: Error) {
    return { error }
  }
  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-screen items-center justify-center bg-background p-8">
          <div className="max-w-md text-center space-y-4">
            <div className="text-4xl">⚠️</div>
            <h1 className="text-xl font-bold">Something went wrong</h1>
            <p className="text-sm text-muted-foreground">{this.state.error.message}</p>
            <button
              onClick={() => { this.setState({ error: null }); window.location.reload() }}
              className="px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium"
            >
              Reload page
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
    },
  },
})

function RequireAuth({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.accessToken)
  const { setUser } = useAuthStore()

  useEffect(() => {
    if (token) {
      authApi.me().then(setUser).catch(() => {})
    }
  }, [token])

  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <ErrorBoundary>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/share/:token" element={<ShareView />} />
          <Route path="/auditor/:token" element={<AuditorPortal />} />
          <Route path="/checkout" element={<Checkout />} />
          <Route
            element={
              <RequireAuth>
                <AppLayout />
              </RequireAuth>
            }
          >
            <Route path="/" element={<Dashboard />} />
            <Route path="/cloud-audits" element={<CloudAudits />} />
            <Route path="/code-iac" element={<CodeIaC />} />
            <Route path="/compliance" element={<Compliance />} />
            <Route path="/findings" element={<Findings />} />
            <Route path="/remediation" element={<Remediation />} />
            <Route path="/reports" element={<Reports />} />
            <Route path="/evidence" element={<Evidence />} />
            {/* keep /connections working for ConnectionForm edit links */}
            <Route path="/connections" element={<Navigate to="/cloud-audits" replace />} />
            <Route path="/connections/new" element={<ConnectionForm />} />
            <Route path="/connections/:id/edit" element={<ConnectionForm />} />
            <Route path="/audit-types" element={<AuditTypes />} />
            <Route path="/jobs" element={<Jobs />} />
            <Route path="/jobs/:id" element={<JobDetail />} />
            <Route path="/plans" element={<Plans />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="/policies" element={<Policies />} />
            <Route path="/monitoring" element={<Monitoring />} />
            <Route path="/access-reviews" element={<AccessReviews />} />
            <Route path="/access-reviews/:id" element={<AccessReviewDetail />} />
            <Route path="/privacy" element={<Privacy />} />
          </Route>
        </Routes>
      </BrowserRouter>
      <Toaster position="bottom-right" richColors />
    </QueryClientProvider>
    </ErrorBoundary>
  )
}
