import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { ShieldCheck, Loader2, AlertCircle } from 'lucide-react'
import { toast } from 'sonner'
import { Input } from '@/components/ui/input'
import { authApi } from '@/lib/api'
import { useAuthStore } from '@/store/useAuthStore'

// Official Google G logo
function GoogleLogo() {
  return (
    <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
      <path d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844c-.209 1.125-.843 2.078-1.796 2.716v2.259h2.908c1.702-1.567 2.684-3.875 2.684-6.615z" fill="#4285F4"/>
      <path d="M9 18c2.43 0 4.467-.806 5.956-2.184l-2.909-2.258c-.806.54-1.837.86-3.047.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332C2.438 15.983 5.482 18 9 18z" fill="#34A853"/>
      <path d="M3.964 10.707c-.18-.54-.282-1.117-.282-1.707s.102-1.167.282-1.707V4.961H.957C.347 6.175 0 7.55 0 9s.348 2.825.957 4.039l3.007-2.332z" fill="#FBBC05"/>
      <path d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0 5.482 0 2.438 2.017.957 4.961L3.964 7.293C4.672 5.163 6.656 3.58 9 3.58z" fill="#EA4335"/>
    </svg>
  )
}

type Mode = 'signin' | 'register'

export function Login() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { setAuth } = useAuthStore()
  const [mode, setMode] = useState<Mode>('signin')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [tenantName, setTenantName] = useState('')
  const [mfaCode, setMfaCode] = useState('')
  const [mfaRequired, setMfaRequired] = useState(false)
  const [loading, setLoading] = useState(false)
  const [googleEnabled, setGoogleEnabled] = useState<boolean | null>(null)

  useEffect(() => {
    authApi.providers()
      .then((p) => setGoogleEnabled(p.google))
      .catch(() => setGoogleEnabled(false))
  }, [])

  useEffect(() => {
    const oauthCode = params.get('oauth_code')
    const error = params.get('error')
    if (error) {
      const messages: Record<string, string> = {
        google_state:  'Google sign-in failed: state mismatch',
        google_token:  'Google sign-in failed: could not exchange token',
        google_user:   'Google sign-in failed: could not fetch user info',
        google_create: 'Google sign-in failed: could not create account',
        token:         'Google sign-in failed: token error',
      }
      toast.error(messages[error] ?? `Sign-in error: ${error}`)
      navigate('/login', { replace: true })
      return
    }
    if (!oauthCode) return
    // Exchange the short-lived code for real tokens (tokens never touch the URL)
    authApi.exchangeOAuthCode(oauthCode)
      .then(({ access_token, refresh_token }) => {
        localStorage.setItem('access_token', access_token)
        localStorage.setItem('refresh_token', refresh_token)
        return authApi.me().then((user) => {
          setAuth(user, access_token, refresh_token)
          navigate('/', { replace: true })
        })
      })
      .catch(() => toast.error('Google sign-in failed: could not exchange code'))
  }, [params, navigate, setAuth])

  const handleGoogleSignIn = () => {
    if (!googleEnabled) return
    window.location.href = authApi.googleStartUrl()
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      if (mode === 'register') {
        const { user, access_token, refresh_token } = await authApi.register({
          email, password,
          tenant_name: tenantName || email,
          prepared_by: email,
        })
        setAuth(user, access_token, refresh_token)
        navigate('/')
        return
      }
      const res = await authApi.login(email, password, mfaCode)
      if (res.mfa_required) {
        setMfaRequired(true)
        toast.message('Enter your 6-digit authenticator code')
        return
      }
      if (!res.user || !res.access_token || !res.refresh_token) throw new Error('missing auth response')
      setAuth(res.user, res.access_token, res.refresh_token)
      navigate('/')
    } catch {
      toast.error(mode === 'register' ? 'Registration failed' : 'Invalid credentials or MFA code')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden px-4 py-12"
      style={{
        background: 'radial-gradient(ellipse 90% 60% at 50% 0%, #e0e7ff 0%, #f5f3ff 35%, #ffffff 70%)',
      }}
    >
      {/* Decorative rings */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute left-1/2 top-0 -translate-x-1/2 -translate-y-1/2 h-[600px] w-[600px] rounded-full border border-indigo-200/40" />
        <div className="absolute left-1/2 top-0 -translate-x-1/2 -translate-y-1/2 h-[900px] w-[900px] rounded-full border border-indigo-100/25" />
        <div className="absolute left-1/2 top-0 -translate-x-1/2 -translate-y-1/2 h-[1200px] w-[1200px] rounded-full border border-indigo-100/15" />
      </div>

      {/* ── Card ── */}
      <div className="relative z-10 w-full max-w-[390px]">
        <div className="rounded-2xl border border-white/80 bg-white/90 backdrop-blur-sm shadow-2xl shadow-indigo-200/40 px-8 py-8">

          {/* Brand */}
          <div className="flex items-center gap-3 mb-8">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-indigo-500 shadow-md shadow-indigo-500/30">
              <ShieldCheck className="h-5 w-5 text-white" strokeWidth={2} />
            </div>
            <div>
              <h1 className="text-[18px] font-bold leading-tight tracking-tight text-slate-900">
                CloudSec<span className="text-indigo-500">Guard</span>
              </h1>
              <p className="text-[11px] text-slate-400 leading-tight">Cloud Security Audit Platform</p>
            </div>
          </div>

          {/* Heading */}
          <div className="mb-6">
            <h2 className="text-xl font-semibold text-slate-800">
              {mode === 'signin' ? 'Welcome back' : 'Create your account'}
            </h2>
            <p className="text-[13px] text-slate-400 mt-0.5">
              {mode === 'signin' ? 'Sign in to your workspace' : 'Start securing your cloud infrastructure'}
            </p>
          </div>

          {/* Google button — shown when enabled */}
          {googleEnabled === true && (
            <>
              <button
                type="button"
                onClick={handleGoogleSignIn}
                className="group flex w-full items-center gap-3 rounded-lg border border-slate-200 bg-white px-4 py-2.5 text-sm font-medium text-slate-700 shadow-sm transition-all hover:shadow-md hover:border-slate-300 active:scale-[0.99]"
              >
                <GoogleLogo />
                <span className="flex-1 text-center">Sign in with Google</span>
              </button>

              <div className="relative my-5">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t border-slate-200" />
                </div>
                <div className="relative flex justify-center">
                  <span className="bg-white px-3 text-[11px] uppercase tracking-wider text-slate-400">or continue with email</span>
                </div>
              </div>
            </>
          )}

          {/* Form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            {mode === 'register' && (
              <div className="space-y-1.5">
                <label className="text-[13px] font-medium text-slate-700" htmlFor="tenant">
                  Company / workspace
                </label>
                <Input
                  id="tenant"
                  placeholder="Acme Inc."
                  value={tenantName}
                  onChange={(e) => setTenantName(e.target.value)}
                  className="h-10 border-slate-200 bg-slate-50/60 focus:bg-white focus-visible:ring-indigo-400/50"
                />
              </div>
            )}

            <div className="space-y-1.5">
              <label className="text-[13px] font-medium text-slate-700" htmlFor="email">
                Email address
              </label>
              <Input
                id="email"
                type="email"
                placeholder="you@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoFocus={googleEnabled !== null}
                className="h-10 border-slate-200 bg-slate-50/60 focus:bg-white focus-visible:ring-indigo-400/50"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-[13px] font-medium text-slate-700" htmlFor="password">
                Password
              </label>
              <Input
                id="password"
                type="password"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="h-10 border-slate-200 bg-slate-50/60 focus:bg-white focus-visible:ring-indigo-400/50"
              />
            </div>

            {mfaRequired && (
              <div className="space-y-1.5">
                <label className="text-[13px] font-medium text-slate-700" htmlFor="mfa">
                  Authenticator code
                </label>
                <Input
                  id="mfa"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  placeholder="123 456"
                  maxLength={6}
                  value={mfaCode}
                  onChange={(e) => setMfaCode(e.target.value.replace(/\D/g, ''))}
                  autoFocus
                  required
                  className="h-10 text-center tracking-[0.5em] text-lg font-mono border-slate-200"
                />
                <p className="flex items-center gap-1.5 text-[12px] text-slate-500">
                  <AlertCircle className="h-3 w-3 shrink-0" />
                  Open your authenticator app and enter the 6-digit code
                </p>
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="mt-1 flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-500 py-2.5 text-sm font-semibold text-white shadow-md shadow-indigo-500/25 transition-all hover:bg-indigo-600 hover:shadow-lg hover:shadow-indigo-500/30 active:scale-[0.99] disabled:opacity-60"
            >
              {loading
                ? <><Loader2 className="h-4 w-4 animate-spin" />Please wait…</>
                : mode === 'signin' ? 'Sign in' : 'Create account'}
            </button>
          </form>

          {/* Switch mode */}
          <p className="mt-5 text-center text-[13px] text-slate-500">
            {mode === 'signin' ? (
              <>Don't have an account?{' '}
                <button type="button" onClick={() => { setMode('register'); setMfaRequired(false) }}
                  className="font-medium text-indigo-500 hover:text-indigo-600 hover:underline">
                  Create one
                </button>
              </>
            ) : (
              <>Already have an account?{' '}
                <button type="button" onClick={() => { setMode('signin'); setMfaRequired(false) }}
                  className="font-medium text-indigo-500 hover:text-indigo-600 hover:underline">
                  Sign in
                </button>
              </>
            )}
          </p>

          {googleEnabled === false && (
            <p className="mt-4 text-center text-[11px] text-slate-400">
              Set <code className="font-mono text-slate-500">GOOGLE_CLIENT_ID</code> env vars to enable Google sign-in.
            </p>
          )}
        </div>

        {/* Below card */}
        <div className="mt-5 flex items-center justify-center gap-4 text-[11px] text-slate-400">
          <span className="flex items-center gap-1">
            <ShieldCheck className="h-3 w-3 text-indigo-400" />
            SOC 2 ready
          </span>
          <span className="h-3 w-px bg-slate-300/70" />
          <span>ISO 27001</span>
          <span className="h-3 w-px bg-slate-300/70" />
          <span>End-to-end encrypted</span>
        </div>

        <p className="mt-3 text-center text-[11px] text-slate-400">
          By continuing you agree to our{' '}
          <a href="/privacy" className="underline hover:text-slate-600 transition-colors">Privacy Policy</a>
        </p>
      </div>
    </div>
  )
}
