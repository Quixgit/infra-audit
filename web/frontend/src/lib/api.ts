import axios from 'axios'

const BASE = '/api'

const api = axios.create({ baseURL: BASE })

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

let refreshing = false
api.interceptors.response.use(
  (r) => r,
  async (error) => {
    const original = error.config
    if (error.response?.status === 401 && !original._retry && !refreshing) {
      original._retry = true
      refreshing = true
      try {
        const rt = localStorage.getItem('refresh_token')
        if (!rt) throw new Error('no refresh token')
        const res = await axios.post(`${BASE}/auth/refresh`, { refresh_token: rt })
        localStorage.setItem('access_token', res.data.access_token)
        original.headers.Authorization = `Bearer ${res.data.access_token}`
        return api(original)
      } catch {
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
        window.location.href = '/login'
      } finally {
        refreshing = false
      }
    }
    return Promise.reject(error)
  }
)

// ── Auth ──────────────────────────────────────────────────────────────────────

export type User = {
  id: string
  email: string
  tenant_id?: string
  tenant_name?: string
  auditor_org: string
  auditor_email: string
  auditor_phone: string
  auditor_website: string
  auditor_address: string
  prepared_by: string
  role: string
  mfa_enabled: boolean
  notify_email: boolean
  created_at: string
}

export const authApi = {
  providers: () =>
    api.get<{ google: boolean }>('/auth/providers').then((r) => r.data),

  login: (email: string, password: string, mfaCode?: string) =>
    api.post<{ access_token?: string; refresh_token?: string; user?: User; mfa_required?: boolean }>(
      '/auth/login', { email, password, mfa_code: mfaCode ?? '' }
    ).then((r) => r.data),

  register: (data: { email: string; password: string; tenant_name?: string; prepared_by?: string }) =>
    api.post<{ access_token: string; refresh_token: string; user: User }>(
      '/auth/register', data
    ).then((r) => r.data),

  googleStartUrl: () => `${BASE}/auth/google/start`,

  exchangeOAuthCode: (code: string) =>
    api.post<{ access_token: string; refresh_token: string }>(
      '/auth/oauth/exchange', { code }
    ).then((r) => r.data),

  logout: (refreshToken?: string) =>
    api.post('/auth/logout', { refresh_token: refreshToken }).catch(() => {}),

  me: () => api.get<User>('/me').then((r) => r.data),

  updateSettings: (settings: Partial<User>) =>
    api.put<User>('/me/settings', settings).then((r) => r.data),

  changePassword: (currentPassword: string, newPassword: string) =>
    api.post('/me/password', { current_password: currentPassword, new_password: newPassword }),

  updateNotify: (notifyEmail: boolean) =>
    api.put<User>('/me/notify', { notify_email: notifyEmail }).then((r) => r.data),

  setupMfa: () => api.post<{ secret: string; otpauth_url: string }>('/me/mfa/setup').then((r) => r.data),

  verifyMfa: (code: string) => api.post<{ mfa_enabled: boolean }>('/me/mfa/verify', { code }).then((r) => r.data),

  disableMfa: (code: string) => api.post<{ mfa_enabled: boolean }>('/me/mfa/disable', { code }).then((r) => r.data),

  uploadAsset: (type: 'logo' | 'watermark' | 'footer-bg', file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post(`/me/assets/${type}`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
}

// ── Connections ───────────────────────────────────────────────────────────────

export type Connection = {
  id: string
  user_id: string
  conn_type: 'do' | 'code' | 'ssl' | 'dns' | 'aws'
  name: string
  do_token_masked?: string
  project_id: string
  scope_mode: string
  spaces_buckets: string
  repo_source?: 'git' | 'local'
  repo_url?: string
  repo_branch?: string
  repo_local_path?: string
  last_stack_detected?: string
  domains?: string // comma-separated for ssl/dns
  // AWS fields
  aws_access_key_masked?: string
  aws_region?: string
  // GitHub webhook
  github_repo_url?: string
  created_at: string
}

export type ConnectionFormData = {
  conn_type: 'do' | 'code' | 'ssl' | 'dns' | 'aws'
  name: string
  // DO fields
  do_token: string
  project_id: string
  scope_mode: string
  spaces_buckets: string
  // Code fields
  repo_source: 'git' | 'local'
  repo_url: string
  repo_token: string
  repo_branch: string
  repo_local_path: string
  // SSL / DNS fields
  domains: string
  // AWS fields
  aws_access_key_id?: string
  aws_secret_key?: string
  aws_region?: string
  // GitHub webhook
  github_webhook_secret?: string
  github_repo_url?: string
}

export const connectionsApi = {
  list: () => api.get<Connection[]>('/connections').then((r) => r.data),

  create: (data: ConnectionFormData) =>
    api.post<Connection>('/connections', data).then((r) => r.data),

  update: (id: string, data: ConnectionFormData) =>
    api.put<Connection>(`/connections/${id}`, data).then((r) => r.data),

  delete: (id: string) => api.delete(`/connections/${id}`),

  test: (id: string) =>
    api.post<{ status: string; projects: unknown[] }>(`/connections/${id}/test`).then((r) => r.data),

  testGit: (data: { repo_url: string; repo_token: string; repo_branch: string }) =>
    api.post<{ ok: boolean; message: string }>('/connections/test-git', data).then((r) => r.data),

  testLocal: (data: { repo_local_path: string }) =>
    api.post<{ ok: boolean; message: string }>('/connections/test-local', data).then((r) => r.data),
}

// ── Jobs ──────────────────────────────────────────────────────────────────────

export type AuditJob = {
  id: string
  connection_id: string
  connection_name: string
  user_id: string
  conn_type: 'do' | 'code' | 'ssl' | 'dns'
  status: 'pending' | 'running' | 'done' | 'failed'
  progress_msg: string
  started_at: string
  finished_at?: string
  error_msg?: string
  findings_critical: number
  findings_high: number
  findings_medium: number
  findings_low: number
  stack_detected?: string[]
}

export const jobsApi = {
  run: (connectionId: string) =>
    api.post<AuditJob>(`/audit/run/${connectionId}`).then((r) => r.data),

  list: () => api.get<AuditJob[]>('/audit/jobs').then((r) => r.data),

  get: (id: string) => api.get<AuditJob>(`/audit/jobs/${id}`).then((r) => r.data),

  delete: (id: string) => api.delete(`/audit/jobs/${id}`),

  downloadHtmlUrl: (id: string) => `${BASE}/audit/jobs/${id}/download/html`,
  downloadDocxUrl: (id: string) => `${BASE}/audit/jobs/${id}/download/docx`,
}

// ── Findings ──────────────────────────────────────────────────────────────────

export type Finding = {
  id: string
  title: string
  severity: string
  status: string
  category: string
  resource_type: string
  resource_name: string
  resource_id: string
  affected_components: string[]
  standard: string
  control_mapping: string[]
  risk: string
  business_impact: string
  evidence: string
  recommendation: string
  remediation: string
  validation: string
  priority: string
  timeline: string
}

export const findingsApi = {
  get: (jobId: string) =>
    api.get<Finding[]>(`/audit/jobs/${jobId}/findings`).then((r) => r.data),
}

// ── Code findings ─────────────────────────────────────────────────────────────

export type CodeFinding = {
  severity: string
  rule_id: string
  title: string
  description: string
  file: string
  line: number
  tool: string
  remediation?: string
  cve?: string
  package?: string
  version?: string
}

export const codeFindingsApi = {
  get: (jobId: string) =>
    api.get<CodeFinding[]>(`/audit/jobs/${jobId}/code-findings`).then((r) => r.data),
}

export const tfFindingsApi = {
  get: (jobId: string) =>
    api.get<CodeFinding[]>(`/audit/jobs/${jobId}/tf-findings`).then((r) => r.data),
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export type FindingsTrendDay = {
  date: string
  critical: number
  high: number
  medium: number
  low: number
}

export type DashboardData = {
  total_connections: number
  jobs_this_week: number
  total_findings: number
  recent_jobs: AuditJob[]
  findings_trend: FindingsTrendDay[]
}

export const dashboardApi = {
  get: () => api.get<DashboardData>('/dashboard').then((r) => r.data),
}

// ── Schedules ─────────────────────────────────────────────────────────────────

export type Schedule = {
  id: string
  connection_id: string
  connection_name?: string
  user_id: string
  interval: string
  enabled: boolean
  next_run_at: string
  last_run_at?: string
  created_at: string
}

export const schedulesApi = {
  list: () => api.get<Schedule[]>('/schedules').then((r) => r.data),

  create: (data: { connection_id: string; interval: string; enabled: boolean }) =>
    api.post<Schedule>('/schedules', data).then((r) => r.data),

  update: (id: string, data: { interval: string; enabled: boolean }) =>
    api.put<Schedule>(`/schedules/${id}`, data).then((r) => r.data),

  delete: (id: string) => api.delete(`/schedules/${id}`),
}

// ── Team ──────────────────────────────────────────────────────────────────────

export type TeamMember = {
  id: string
  email: string
  role: string
  created_at: string
}

export const teamApi = {
  list: () => api.get<TeamMember[]>('/team').then((r) => r.data),

  invite: (data: { email: string; password: string; role: string }) =>
    api.post<TeamMember>('/team/invite', data).then((r) => r.data),

  updateRole: (id: string, role: string) =>
    api.patch<{ role: string }>(`/team/${id}`, { role }).then((r) => r.data),

  delete: (id: string) => api.delete(`/team/${id}`),
}

// ── Share links ───────────────────────────────────────────────────────────────

export type ShareLink = {
  id: string
  job_id: string
  token: string
  created_at: string
}

export type ShareData = {
  job: AuditJob
  findings: Finding[]
}

export const shareApi = {
  create: (jobId: string) =>
    api.post<ShareLink>(`/audit/jobs/${jobId}/share`).then((r) => r.data),

  get: (token: string) =>
    axios.get<ShareData>(`${BASE}/share/${token}`).then((r) => r.data),
}

// ── Compare ───────────────────────────────────────────────────────────────────

export type CompareResult = {
  new_findings: Finding[]
  fixed_findings: Finding[]
  prev_job_id: string
}

export const compareApi = {
  get: (jobId: string) =>
    api.get<CompareResult>(`/audit/jobs/${jobId}/compare`).then((r) => r.data),
}

// ── Connection history ────────────────────────────────────────────────────────

export const connectionHistoryApi = {
  get: (connectionId: string) =>
    api.get<AuditJob[]>(`/connections/${connectionId}/history`).then((r) => r.data),
}

// ── API tokens ────────────────────────────────────────────────────────────────

export type APIToken = {
  id: string
  user_id: string
  name: string
  token_prefix: string
  created_at: string
  last_used_at?: string
}

export const apiTokensApi = {
  list: () => api.get<APIToken[]>('/tokens').then((r) => r.data),

  create: (name: string) =>
    api.post<APIToken & { token: string }>('/tokens', { name }).then((r) => r.data),

  delete: (id: string) => api.delete(`/tokens/${id}`),
}

// ── License ───────────────────────────────────────────────────────────────────

export type LicensePlan = 'community' | 'starter' | 'professional' | 'business' | 'enterprise'

export type LicenseFeature =
  | 'scheduled_audits'
  | 'code_audit'
  | 'share_links'
  | 'api_tokens'
  | 'custom_branding'
  | 'team'
  | 'sso'
  | 'aws_audit'
  | 'basic_audit'
  | 'basic_report'
  | 'pdf_reports'
  | 'compliance_basic'
  | 'evidence'
  | 'policies'
  | 'access_reviews'
  | 'remediation'
  | 'priority_support'
  | 'custom_frameworks'
  | 'white_label'
  | 'self_hosted'
  | 'dedicated_support'
  | 'human_review'

export type LicenseInfo = {
  plan: LicensePlan
  issued_to?: string
  expires_at?: string
  max_connections: number
  max_users: number
  max_audits_month: number
  features: LicenseFeature[]
  used_connections: number
  used_audits_month: number
  used_users: number
}

export const licenseApi = {
  get: () => api.get<LicenseInfo>('/license').then((r) => r.data),

  activate: (key: string) =>
    api.post<LicenseInfo>('/license/activate', { key }).then((r) => r.data),

  setPreviewPlan: (plan: LicensePlan) =>
    api.post<{ plan: string }>('/license/preview-plan', { plan }).then((r) => r.data),
}

// ── Workspace ─────────────────────────────────────────────────────────────────

export type WorkspaceInfo = {
  id: string
  name: string
  slack_webhook_url: string
}

export const workspaceApi = {
  get: () => api.get<WorkspaceInfo>('/workspace').then((r) => r.data),
  update: (data: { name: string; slack_webhook_url: string }) =>
    api.put<WorkspaceInfo>('/workspace', data).then((r) => r.data),
}

// ── Activity log ──────────────────────────────────────────────────────────────

export type ActivityLogEntry = {
  id: string
  user_email: string
  action: string
  resource_type: string
  resource_id: string
  ip_address: string
  created_at: string
}

export const activityApi = {
  list: () => api.get<ActivityLogEntry[]>('/activity-log').then((r) => r.data),
}

// ── Aggregated findings ───────────────────────────────────────────────────────

export type FindingStatus = 'open' | 'in_progress' | 'fixed' | 'accepted_risk' | 'false_positive'

export type AggregatedFinding = {
  job_id: string
  source: 'findings' | 'tf_findings'
  finding_index: number
  job_date: string
  connection_id: string
  connection_name: string
  conn_type: 'do' | 'code'
  severity: string
  title: string
  category?: string
  resource_type?: string
  resource_name?: string
  evidence?: string
  tool?: string
  file?: string
  line?: number
  rule_id?: string
  package?: string
  cve?: string
  recommendation?: string
  remediation?: string
  status: FindingStatus
  note: string
}

export const aggregatedFindingsApi = {
  list: (params?: {
    severity?: string
    status?: string
    connection_id?: string
    tool?: string
    search?: string
  }) => {
    const q = new URLSearchParams()
    if (params?.severity) q.set('severity', params.severity)
    if (params?.status) q.set('status', params.status)
    if (params?.connection_id) q.set('connection_id', params.connection_id)
    if (params?.tool) q.set('tool', params.tool)
    if (params?.search) q.set('search', params.search)
    const qs = q.toString()
    return api.get<AggregatedFinding[]>(`/findings${qs ? '?' + qs : ''}`).then((r) => r.data)
  },

  setOverride: (data: {
    job_id: string
    source: string
    finding_index: number
    status: FindingStatus
    note: string
  }) => api.put('/findings/override', data).then((r) => r.data),
}

// ── Compliance ────────────────────────────────────────────────────────────────

export type ComplianceStatus = 'met' | 'partial' | 'not_met' | 'not_assessed'

export type ComplianceControl = {
  ctrl_id: string
  name: string
  description: string
  status: ComplianceStatus
  finding_count: number
  open_count: number
  findings?: AggregatedFinding[]
}

export type ComplianceFramework = {
  slug: string
  name: string
  version: string
  description: string
  score: number
  met_count: number
  total_count: number
  controls?: ComplianceControl[]
}

export const complianceApi = {
  listFrameworks: () =>
    api.get<ComplianceFramework[]>('/compliance/frameworks').then((r) => r.data),

  getFramework: (slug: string) =>
    api.get<ComplianceFramework>(`/compliance/frameworks/${slug}`).then((r) => r.data),
}

// ── Evidence ──────────────────────────────────────────────────────────────────

export type EvidenceStatus = 'fresh' | 'stale' | 'expired'

export type EvidenceMapping = {
  id: string
  evidence_id: string
  framework_slug: string
  ctrl_id: string
  created_at: string
}

export type EvidenceItem = {
  id: string
  job_id?: string
  job_name?: string
  source: 'auto' | 'manual'
  evidence_type: string
  name: string
  description: string
  content_type: string
  size: number
  expires_at: string
  created_at: string
  status: EvidenceStatus
  mappings: EvidenceMapping[]
}

export const evidenceApi = {
  list: () => api.get<EvidenceItem[]>('/evidence').then((r) => r.data),

  upload: (formData: FormData) =>
    api.post<EvidenceItem>('/evidence/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then((r) => r.data),

  downloadUrl: (id: string) => `${BASE}/evidence/${id}/download`,

  delete: (id: string) => api.delete(`/evidence/${id}`),

  setMappings: (id: string, mappings: { framework_slug: string; ctrl_id: string }[]) =>
    api.put<EvidenceMapping[]>(`/evidence/${id}/mappings`, { mappings }).then((r) => r.data),

  byControl: (framework: string, ctrlId: string) =>
    api.get<EvidenceItem[]>(`/evidence/by-control/${framework}/${encodeURIComponent(ctrlId)}`).then((r) => r.data),

  counts: () => api.get<Record<string, number>>('/evidence/counts').then((r) => r.data),
}

// ── Monitoring / SLA ──────────────────────────────────────────────────────────

export type SLARule = {
  id: string
  tenant_id: string
  severity: string
  max_days_open: number
  notify_email: boolean
  notify_slack: boolean
  created_at: string
}

export type SLABreach = {
  id: string
  job_id: string
  connection_id: string
  connection_name: string
  finding_index: number
  source: string
  title: string
  severity: string
  opened_at: string
  breached_at: string
  notified_at?: string
  days_overdue: number
  status: string
}

export type SecurityScore = {
  id: string
  connection_id: string
  connection_name?: string
  job_id: string
  score: number
  critical_count: number
  high_count: number
  medium_count: number
  low_count: number
  calculated_at: string
}

export type ScoreTrendDay = {
  date: string
  avg_score: number
}

export type FindingChange = {
  job_id: string
  connection_id: string
  connection_name: string
  title: string
  severity: string
  change_type: 'new' | 'regression' | 'resolved'
  occurred_at: string
}

export type SLABySeverity = {
  severity: string
  count: number
  oldest_days: number
}

export type MonitoringOverview = {
  avg_score: number
  sla_breach_count: number
  new_findings_this_week: number
  regressions_findings_count: number
  sla_breaches_by_severity: SLABySeverity[]
  score_trend: ScoreTrendDay[]
  recent_changes: FindingChange[]
}

export const monitoringApi = {
  overview: () =>
    api.get<MonitoringOverview>('/monitoring/overview').then((r) => r.data),

  scores: () =>
    api.get<SecurityScore[]>('/monitoring/scores').then((r) => r.data),

  connectionScores: (connectionId: string) =>
    api.get<SecurityScore[]>(`/monitoring/scores/${connectionId}`).then((r) => r.data),

  slaBreaches: () =>
    api.get<SLABreach[]>('/monitoring/sla-breaches').then((r) => r.data),
}

export const slaApi = {
  list: () => api.get<SLARule[]>('/sla-rules').then((r) => r.data),

  update: (rules: Pick<SLARule, 'severity' | 'max_days_open' | 'notify_email' | 'notify_slack'>[]) =>
    api.put<SLARule[]>('/sla-rules', rules).then((r) => r.data),
}

// ── Policies ──────────────────────────────────────────────────────────────────

export type PolicyStatus = 'Draft' | 'Under Review' | 'Approved' | 'Expired'

export type PolicyControlMapping = {
  policy_id: string
  framework_slug: string
  control_code: string
}

export type Policy = {
  id: string
  tenant_id?: string
  name: string
  category: string
  template_slug: string
  content_html?: string
  file_name: string
  status: PolicyStatus
  version: number
  approved_by_user_id?: string
  approved_by_email?: string
  approved_at?: string
  review_date?: string
  last_reviewed_at?: string
  controls: PolicyControlMapping[]
  created_at: string
  updated_at: string
}

export type PolicyTemplate = {
  slug: string
  name: string
  category: string
  description: string
}

export type PolicyStats = {
  total: number
  approved: number
  draft: number
  expired: number
  review_due: number
}

export const policiesApi = {
  listTemplates: () =>
    api.get<PolicyTemplate[]>('/policies/templates').then((r) => r.data),

  stats: () =>
    api.get<PolicyStats>('/policies/stats').then((r) => r.data),

  list: () =>
    api.get<Policy[]>('/policies').then((r) => r.data),

  create: (data: {
    name?: string
    category?: string
    template_slug: string
    content_html?: string
    review_date?: string
    controls?: PolicyControlMapping[]
    // template fill
    company_name?: string
    effective_date?: string
    owner_name?: string
    owner_title?: string
  }) => api.post<Policy>('/policies', data).then((r) => r.data),

  update: (id: string, data: {
    name?: string
    category?: string
    content_html?: string
    file_name?: string
    status?: string
    review_date?: string
    controls?: PolicyControlMapping[]
  }) => api.put<Policy>(`/policies/${id}`, data).then((r) => r.data),

  delete: (id: string) => api.delete(`/policies/${id}`),

  approve: (id: string) =>
    api.post<Policy>(`/policies/${id}/approve`, {}).then((r) => r.data),

  markReviewed: (id: string, reviewDate: string) =>
    api.post<Policy>(`/policies/${id}/review`, { review_date: reviewDate }).then((r) => r.data),

  upload: (formData: FormData) =>
    api.post<Policy>('/policies/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then((r) => r.data),

  downloadUrl: (id: string) => `${BASE}/policies/${id}/download`,
}

// ── Auditor Portal ────────────────────────────────────────────────────────────

export type AuditorPermission = 'compliance' | 'evidence' | 'policies' | 'findings'

export type AuditorInvite = {
  id: string
  name: string
  email: string
  token: string
  permissions: AuditorPermission[]
  expires_at: string
  last_accessed_at?: string
  created_at: string
  app_url?: string
}

export type AuditorComment = {
  id: string
  invite_id: string
  auditor_name: string
  section: string
  item_id: string
  body: string
  created_at: string
}

export type AuditorPortalInfo = {
  name: string
  company: string
  expires_at: string
  permissions: AuditorPermission[]
  app_url: string
}

export type FindingsSummary = {
  critical: number
  high: number
  medium: number
  low: number
  total: number
  by_connection: {
    connection_id: string
    connection_name: string
    critical: number
    high: number
    medium: number
    low: number
  }[]
}

export const auditorInvitesApi = {
  list: () =>
    api.get<AuditorInvite[]>('/auditor-invites').then((r) => r.data),

  create: (data: {
    name: string
    email?: string
    expiry_days?: number
    permissions: AuditorPermission[]
  }) => api.post<AuditorInvite>('/auditor-invites', data).then((r) => r.data),

  delete: (id: string) => api.delete(`/auditor-invites/${id}`),

  listComments: (id: string) =>
    api.get<AuditorComment[]>(`/auditor-invites/${id}/comments`).then((r) => r.data),
}

// Public auditor portal API — no auth header, token in URL
const auditorBase = axios.create({ baseURL: '/api' })

export const auditorPortalApi = {
  verify: (token: string) =>
    auditorBase.get<AuditorPortalInfo>(`/auditor/${token}/verify`).then((r) => r.data),

  compliance: (token: string) =>
    auditorBase.get<ComplianceFramework[]>(`/auditor/${token}/compliance`).then((r) => r.data),

  evidence: (token: string) =>
    auditorBase.get<EvidenceItem[]>(`/auditor/${token}/evidence`).then((r) => r.data),

  policies: (token: string) =>
    auditorBase.get<Policy[]>(`/auditor/${token}/policies`).then((r) => r.data),

  findingsSummary: (token: string) =>
    auditorBase.get<FindingsSummary>(`/auditor/${token}/findings/summary`).then((r) => r.data),

  downloadEvidenceUrl: (token: string, id: string) =>
    `/api/auditor/${token}/evidence/${id}/download`,

  downloadPolicyUrl: (token: string, id: string) =>
    `/api/auditor/${token}/policies/${id}/download`,

  addComment: (token: string, data: { section: string; item_id?: string; body: string }) =>
    auditorBase.post<AuditorComment>(`/auditor/${token}/comments`, data).then((r) => r.data),

  listComments: (token: string) =>
    auditorBase.get<AuditorComment[]>(`/auditor/${token}/comments`).then((r) => r.data),
}

// ── Access Reviews ────────────────────────────────────────────────────────────

export type AccessReviewDecision = 'pending' | 'approved' | 'revoked' | 'needs_followup'

export type AccessReviewItem = {
  id: string
  review_id: string
  subject_name: string
  subject_email: string
  subject_role: string
  access_level: string
  last_active_at?: string
  decision: AccessReviewDecision
  decided_by_user_id?: string
  decided_by_email?: string
  decided_at?: string
  notes: string
}

export type AccessReview = {
  id: string
  tenant_id?: string
  name: string
  description: string
  review_type: 'manual' | 'do_team' | 'github_org'
  connection_id?: string
  status: 'in_progress' | 'completed' | 'overdue'
  due_date?: string
  completed_at?: string
  item_count: number
  reviewed_count: number
  created_at: string
}

export type AccessReviewStats = {
  total: number
  in_progress: number
  overdue: number
  completed: number
  due_this_month: number
}

export const accessReviewsApi = {
  stats: () =>
    api.get<AccessReviewStats>('/access-reviews/stats').then((r) => r.data),

  list: () =>
    api.get<AccessReview[]>('/access-reviews').then((r) => r.data),

  create: (data: {
    name: string
    description?: string
    review_type: string
    connection_id?: string | null
    due_date?: string | null
    github_org?: string
    github_token?: string
  }) => api.post<AccessReview>('/access-reviews', data).then((r) => r.data),

  update: (id: string, data: { name?: string; description?: string; due_date?: string | null }) =>
    api.put<AccessReview>(`/access-reviews/${id}`, data).then((r) => r.data),

  delete: (id: string) => api.delete(`/access-reviews/${id}`),

  complete: (id: string) =>
    api.post<AccessReview>(`/access-reviews/${id}/complete`, {}).then((r) => r.data),

  listItems: (id: string) =>
    api.get<AccessReviewItem[]>(`/access-reviews/${id}/items`).then((r) => r.data),

  addItem: (id: string, item: Partial<AccessReviewItem>) =>
    api.post<AccessReviewItem>(`/access-reviews/${id}/items`, item).then((r) => r.data),

  updateItem: (reviewId: string, itemId: string, data: { decision: AccessReviewDecision; notes?: string }) =>
    api.put<AccessReviewItem>(`/access-reviews/${reviewId}/items/${itemId}`, data).then((r) => r.data),

  importDO: (reviewId: string, connectionId: string) =>
    api.post<{ imported: number; review: AccessReview }>(`/access-reviews/${reviewId}/import-do`, {
      connection_id: connectionId,
    }).then((r) => r.data),
}

// ── Notify me ─────────────────────────────────────────────────────────────────

export const notifyApi = {
  request: (type: string, email: string) =>
    api.post('/notify-me', { type, email }).then((r) => r.data),
}

// ── Bulk run ──────────────────────────────────────────────────────────────────

export const bulkApi = {
  run: (connectionIds: string[]) =>
    api.post<AuditJob[]>('/audit/run-bulk', { connection_ids: connectionIds }).then((r) => r.data),
}

// ── Modules ───────────────────────────────────────────────────────────────────

export const modulesApi = {
  getAll: () => api.get<Record<string, boolean>>('/admin/modules').then((r) => r.data),
  setAll: (modules: Record<string, boolean>) =>
    api.put<Record<string, boolean>>('/admin/modules', modules).then((r) => r.data),
}

// ── Remediation ───────────────────────────────────────────────────────────────

export type RemediationTask = {
  id: string
  tenant_id: string
  job_id?: string
  source: string
  finding_index: number
  connection_id?: string
  connection_name: string
  title: string
  severity: string
  resource_name: string
  description: string
  remediation_text: string
  risk_text: string
  assigned_to?: string
  assigned_email: string
  lane: 'immediate' | 'this_week' | 'this_month' | 'backlog' | 'done'
  due_date?: string
  verify_job_id?: string
  verify_status: '' | 'pending' | 'still_present' | 'not_found'
  verified_at?: string
  comment_count: number
  created_at: string
  updated_at: string
}

export type RemediationComment = {
  id: string
  task_id: string
  user_id: string
  user_email: string
  body: string
  created_at: string
}

export const remediationApi = {
  listTasks: () =>
    api.get<RemediationTask[]>('/remediation/tasks').then((r) => r.data),

  createTask: (data: Partial<RemediationTask>) =>
    api.post<RemediationTask>('/remediation/tasks', data).then((r) => r.data),

  updateTask: (id: string, data: { lane?: string; assigned_to?: string | null; due_date?: string | null }) =>
    api.put<RemediationTask>(`/remediation/tasks/${id}`, data).then((r) => r.data),

  deleteTask: (id: string) =>
    api.delete(`/remediation/tasks/${id}`),

  listComments: (id: string) =>
    api.get<RemediationComment[]>(`/remediation/tasks/${id}/comments`).then((r) => r.data),

  addComment: (id: string, body: string) =>
    api.post<RemediationComment>(`/remediation/tasks/${id}/comments`, { body }).then((r) => r.data),

  verify: (id: string) =>
    api.post<{ verify_job_id: string }>(`/remediation/tasks/${id}/verify`).then((r) => r.data),

  verifyResult: (id: string) =>
    api.get<{ verify_status: string }>(`/remediation/tasks/${id}/verify-result`).then((r) => r.data),

  getAISuggestion: (id: string) =>
    api.get<AISuggestion>(`/remediation/tasks/${id}/ai-suggest`).then((r) => r.data),
}

// ── AI suggestions ────────────────────────────────────────────────────────────

export type AISuggestion = {
  commands: string[]
  explanation: string
  doc_links: string[]
  difficulty: 'easy' | 'medium' | 'hard'
  est_time: string
  error?: string
  fallback?: string
}

// ── Custom Compliance Frameworks ──────────────────────────────────────────────

export type CustomControl = {
  id: string
  framework_id: string
  tenant_id: string
  ctrl_id: string
  name: string
  description: string
  category: string
  created_at: string
}

export type CustomFramework = {
  id: string
  tenant_id: string
  name: string
  slug: string
  version: string
  description: string
  controls?: CustomControl[]
  created_at: string
  updated_at: string
}

export const customFrameworksApi = {
  list: () =>
    api.get<CustomFramework[]>('/custom-frameworks').then((r) => r.data),

  create: (data: { name: string; slug?: string; version?: string; description?: string }) =>
    api.post<CustomFramework>('/custom-frameworks', data).then((r) => r.data),

  get: (id: string) =>
    api.get<CustomFramework>(`/custom-frameworks/${id}`).then((r) => r.data),

  update: (id: string, data: { name?: string; version?: string; description?: string }) =>
    api.put<CustomFramework>(`/custom-frameworks/${id}`, data).then((r) => r.data),

  delete: (id: string) =>
    api.delete(`/custom-frameworks/${id}`),

  listControls: (frameworkId: string) =>
    api.get<CustomControl[]>(`/custom-frameworks/${frameworkId}/controls`).then((r) => r.data),

  createControl: (frameworkId: string, data: {
    ctrl_id: string; name: string; description?: string; category?: string
  }) =>
    api.post<CustomControl>(`/custom-frameworks/${frameworkId}/controls`, data).then((r) => r.data),

  updateControl: (frameworkId: string, controlId: string, data: {
    name?: string; description?: string; category?: string
  }) =>
    api.put<CustomControl>(`/custom-frameworks/${frameworkId}/controls/${controlId}`, data).then((r) => r.data),

  deleteControl: (frameworkId: string, controlId: string) =>
    api.delete(`/custom-frameworks/${frameworkId}/controls/${controlId}`),

  importControls: (frameworkId: string, controls: Array<{
    ctrl_id: string; name: string; description?: string; category?: string
  }>) =>
    api.post<{ imported: number; total: number }>(
      `/custom-frameworks/${frameworkId}/controls/import`,
      { controls }
    ).then((r) => r.data),
}

export default api
