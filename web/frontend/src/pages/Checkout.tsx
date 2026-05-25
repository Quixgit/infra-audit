import { useSearchParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, CreditCard, ShieldCheck, Lock, Check, Infinity } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { useAuthStore } from '@/store/useAuthStore'

// ── Plan definitions ──────────────────────────────────────────────────────────

type PlanInfo = {
  label: string
  priceMonthly: number
  color: string
  borderCls: string
  bgCls: string
  features: string[]
}

const planInfo: Record<string, PlanInfo> = {
  professional: {
    label: 'Professional',
    priceMonthly: 99,
    color: 'text-indigo-400',
    borderCls: 'border-indigo-500/30',
    bgCls: 'bg-indigo-500/5',
    features: [
      'Up to 20 connections',
      '100 audits / month',
      'Up to 5 users',
      'Scheduled audits',
      'Code & IaC scanning',
      'Share links',
      'PDF / HTML reports',
      'Compliance mapping',
    ],
  },
  business: {
    label: 'Business',
    priceMonthly: 299,
    color: 'text-violet-400',
    borderCls: 'border-violet-500/30',
    bgCls: 'bg-violet-500/5',
    features: [
      'Up to 100 connections',
      '500 audits / month',
      'Up to 15 users',
      'All Professional features',
      'Custom branding',
      'API access & tokens',
      'Team management',
      'Evidence library, Policies, Access reviews',
      'Remediation board',
      'Priority support',
    ],
  },
  enterprise: {
    label: 'Enterprise',
    priceMonthly: 799,
    color: 'text-purple-400',
    borderCls: 'border-purple-400/30',
    bgCls: 'bg-purple-400/5',
    features: [
      'Unlimited connections & audits',
      'Unlimited users',
      'All Business features',
      'SSO / SAML',
      'Custom compliance frameworks',
      'White-label reports',
      'Self-hosted option',
      'Dedicated support & SLA',
      'Human review add-on',
    ],
  },
}

// ── Checkout Page ─────────────────────────────────────────────────────────────

export function Checkout() {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const { user } = useAuthStore()

  const planKey = params.get('plan') ?? 'professional'
  const billing = params.get('billing') ?? 'monthly'
  const plan = planInfo[planKey] ?? planInfo.professional

  const monthlyPrice = plan.priceMonthly
  // Enterprise has custom annual pricing ($7,500) — others get 2 months free (×10)
  const annualPrice = planKey === 'enterprise' ? 7500 : Math.round(monthlyPrice * 10)
  const displayPrice = billing === 'annual' ? annualPrice : monthlyPrice
  const savings = billing === 'annual' && planKey !== 'enterprise'
    ? Math.round(monthlyPrice * 12 - annualPrice)
    : 0

  const setBilling = (v: 'monthly' | 'annual') => {
    setParams({ plan: planKey, billing: v })
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-6"
      style={{ background: 'radial-gradient(ellipse 80% 50% at 50% 0%, #e0e7ff 0%, #f5f3ff 30%, #ffffff 60%)' }}
    >
      <div className="w-full max-w-4xl">
        {/* Back nav */}
        <button
          onClick={() => navigate('/plans')}
          className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-8 transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to plans
        </button>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-start">
          {/* ── Left: Order summary ── */}
          <div className="space-y-5">
            <div>
              <h1 className="text-2xl font-bold">Complete your order</h1>
              <p className="text-sm text-muted-foreground mt-1">
                Your license key will be sent to{' '}
                <span className="font-medium text-foreground">{user?.email ?? 'your email'}</span> immediately after payment.
              </p>
            </div>

            {/* Plan card */}
            <Card className={`border-2 ${plan.borderCls}`}>
              <CardHeader className={`${plan.bgCls} rounded-t-xl pb-3`}>
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base">{plan.label}</CardTitle>
                  <Badge className={`${plan.borderCls} ${plan.color} ${plan.bgCls} border text-xs`}>
                    Selected
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className="pt-4 space-y-4">
                {/* Billing toggle */}
                <div className="flex gap-2">
                  <button
                    onClick={() => setBilling('monthly')}
                    className={`flex-1 py-2 text-xs rounded-md border transition-colors ${
                      billing !== 'annual'
                        ? 'border-indigo-500/40 bg-indigo-500/5 text-indigo-400 font-medium'
                        : 'border-border text-muted-foreground hover:border-foreground/20'
                    }`}
                  >
                    Monthly
                  </button>
                  <button
                    onClick={() => setBilling('annual')}
                    className={`flex-1 py-2 text-xs rounded-md border transition-colors relative ${
                      billing === 'annual'
                        ? 'border-indigo-500/40 bg-indigo-500/5 text-indigo-400 font-medium'
                        : 'border-border text-muted-foreground hover:border-foreground/20'
                    }`}
                  >
                    Annual
                    <span className="absolute -top-2.5 -right-1 text-[9px] bg-green-500 text-white rounded-full px-1.5 py-0.5 font-semibold leading-none">
                      −17%
                    </span>
                  </button>
                </div>

                {/* Price */}
                <div>
                  <div className="flex items-end gap-1.5">
                    <span className="text-3xl font-bold">${displayPrice}</span>
                    <span className="text-sm text-muted-foreground mb-1">
                      / {billing === 'annual' ? 'year' : 'month'}
                    </span>
                  </div>
                  {savings > 0 && (
                    <p className="text-xs text-green-500 mt-0.5">You save ${savings} compared to monthly billing</p>
                  )}
                </div>

                <Separator />

                {/* Features */}
                <div className="space-y-1.5">
                  {plan.features.map((f) => (
                    <div key={f} className="flex items-center gap-2 text-xs">
                      <Check className={`h-3.5 w-3.5 shrink-0 ${plan.color}`} />
                      <span className="text-foreground/80">{f}</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Trust indicators */}
            <div className="flex items-center flex-wrap gap-4 text-xs text-muted-foreground">
              <div className="flex items-center gap-1.5">
                <Lock className="h-3.5 w-3.5" />
                SSL encrypted
              </div>
              <div className="flex items-center gap-1.5">
                <ShieldCheck className="h-3.5 w-3.5" />
                Secure payment
              </div>
              <div className="flex items-center gap-1.5">
                <Infinity className="h-3.5 w-3.5" />
                Cancel anytime
              </div>
            </div>
          </div>

          {/* ── Right: Payment form ── */}
          <div>
            <Card className="shadow-xl shadow-indigo-100/40">
              <CardHeader className="pb-3">
                <CardTitle className="text-base flex items-center gap-2">
                  <CreditCard className="h-4 w-4 text-indigo-400" />
                  Payment details
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="checkout-email">Email for license delivery</Label>
                  <Input
                    id="checkout-email"
                    type="email"
                    defaultValue={user?.email ?? ''}
                    placeholder="you@company.com"
                  />
                </div>

                {/* Stripe placeholder */}
                <div className="rounded-xl border border-dashed border-indigo-200 bg-indigo-50/50 p-6 text-center space-y-3">
                  <div className="flex justify-center">
                    <div className="rounded-full bg-indigo-100 p-3">
                      <CreditCard className="h-6 w-6 text-indigo-400" />
                    </div>
                  </div>
                  <div>
                    <p className="text-sm font-semibold">Stripe Checkout</p>
                    <p className="text-xs text-muted-foreground mt-1 leading-relaxed">
                      Online payment is coming soon.
                      Reach out directly to purchase a license key.
                    </p>
                  </div>
                  <a
                    href={`mailto:sales@cloudsecguard.com?subject=License%20purchase%20—%20${plan.label}%20(${billing})&body=Hi%2C%20I'd%20like%20to%20purchase%20the%20${plan.label}%20plan%20(${billing}%20billing)%20for%20%24${displayPrice}.%0A%0AEmail%3A%20${encodeURIComponent(user?.email ?? '')}`}
                    className="inline-flex items-center gap-1.5 text-xs text-indigo-500 hover:text-indigo-400 transition-colors font-medium"
                  >
                    Contact sales team →
                  </a>
                </div>

                <Button
                  className="w-full bg-indigo-500 hover:bg-indigo-600 text-white shadow-md shadow-indigo-500/25 opacity-50 cursor-not-allowed"
                  disabled
                >
                  <Lock className="mr-2 h-4 w-4" />
                  Proceed to payment — coming soon
                </Button>

                <p className="text-[11px] text-muted-foreground text-center leading-relaxed">
                  By purchasing you agree to our Terms of Service.
                  Your license key is delivered instantly after payment confirmation.
                  Subscriptions renew automatically — cancel at any time.
                </p>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  )
}
