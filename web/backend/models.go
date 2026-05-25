package main

import "time"

// DB row types

type User struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id,omitempty"`
	TenantName     string    `json:"tenant_name,omitempty"`
	Email          string    `json:"email"`
	PasswordHash   string    `json:"-"`
	AuditorOrg     string    `json:"auditor_org"`
	AuditorEmail   string    `json:"auditor_email"`
	AuditorPhone   string    `json:"auditor_phone"`
	AuditorWebsite string    `json:"auditor_website"`
	AuditorAddress string    `json:"auditor_address"`
	PreparedBy     string    `json:"prepared_by"`
	Role           string    `json:"role"`
	MFAEnabled     bool      `json:"mfa_enabled"`
	NotifyEmail    bool      `json:"notify_email"`
	CreatedAt      time.Time `json:"created_at"`
}

type Connection struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	UserID            string    `json:"user_id"`
	ConnType          string    `json:"conn_type"` // "do" | "code" | "ssl" | "dns" | "aws"
	Name              string    `json:"name"`
	DOToken           string    `json:"-"`
	ProjectID         string    `json:"project_id"`
	ScopeMode         string    `json:"scope_mode"`
	SpacesBuckets     string    `json:"spaces_buckets"`
	RepoSource        string    `json:"repo_source"` // "git" | "local"
	RepoURL           string    `json:"repo_url"`
	RepoToken         string    `json:"-"` // encrypted, never expose
	RepoBranch        string    `json:"repo_branch"`
	RepoLocalPath     string    `json:"repo_local_path"`
	LastStackDetected string    `json:"last_stack_detected"`
	Domains           string    `json:"domains"` // comma-separated for ssl/dns
	// AWS fields
	AWSAccessKeyID    string    `json:"-"` // encrypted
	AWSSecretKey      string    `json:"-"` // encrypted
	AWSRegion         string    `json:"aws_region,omitempty"`
	AWSAccessKeyMasked string   `json:"aws_access_key_masked,omitempty"`
	// GitHub webhook
	GitHubWebhookSecret string  `json:"-"` // encrypted
	GitHubRepoURL       string  `json:"github_repo_url,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type AuditJob struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	ConnectionID     string     `json:"connection_id"`
	ConnectionName   string     `json:"connection_name,omitempty"`
	UserID           string     `json:"user_id"`
	ConnType         string     `json:"conn_type"`
	Status           string     `json:"status"`
	ProgressMsg      string     `json:"progress_msg"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	HTMLPath         string     `json:"-"`
	DOCXPath         string     `json:"-"`
	ErrorMsg         string     `json:"error_msg,omitempty"`
	FindingsCritical int        `json:"findings_critical"`
	FindingsHigh     int        `json:"findings_high"`
	FindingsMedium   int        `json:"findings_medium"`
	FindingsLow      int        `json:"findings_low"`
	StackDetected    []string   `json:"stack_detected,omitempty"`
}

type Schedule struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	ConnectionID   string     `json:"connection_id"`
	ConnectionName string     `json:"connection_name,omitempty"`
	UserID         string     `json:"user_id"`
	Interval       string     `json:"interval"`
	Enabled        bool       `json:"enabled"`
	NextRunAt      time.Time  `json:"next_run_at"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type TeamMember struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type DashboardData struct {
	TotalConnections int                `json:"total_connections"`
	JobsThisWeek     int                `json:"jobs_this_week"`
	TotalFindings    int                `json:"total_findings"`
	RecentJobs       []AuditJob         `json:"recent_jobs"`
	FindingsTrend    []FindingsTrendDay `json:"findings_trend"`
}

type FindingsTrendDay struct {
	Date     string `json:"date"`
	Critical int    `json:"critical"`
	High     int    `json:"high"`
	Medium   int    `json:"medium"`
	Low      int    `json:"low"`
}

// Request/response types

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	MFACode  string `json:"mfa_code"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	User         User   `json:"user,omitempty"`
	MFARequired  bool   `json:"mfa_required,omitempty"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`
}

type updateSettingsRequest struct {
	AuditorOrg     string `json:"auditor_org"`
	AuditorEmail   string `json:"auditor_email"`
	AuditorPhone   string `json:"auditor_phone"`
	AuditorWebsite string `json:"auditor_website"`
	AuditorAddress string `json:"auditor_address"`
	PreparedBy     string `json:"prepared_by"`
}

type updateNotifyRequest struct {
	NotifyEmail bool `json:"notify_email"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type registerRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantName string `json:"tenant_name"`
	PreparedBy string `json:"prepared_by"`
}

type createConnectionRequest struct {
	ConnType      string `json:"conn_type"`
	Name          string `json:"name"`
	DOToken       string `json:"do_token"`
	ProjectID     string `json:"project_id"`
	ScopeMode     string `json:"scope_mode"`
	SpacesBuckets string `json:"spaces_buckets"`
	RepoSource    string `json:"repo_source"`
	RepoURL       string `json:"repo_url"`
	RepoToken     string `json:"repo_token"`
	RepoBranch    string `json:"repo_branch"`
	RepoLocalPath string `json:"repo_local_path"`
	Domains       string `json:"domains"` // comma-separated for ssl/dns
	// AWS
	AWSAccessKeyID  string `json:"aws_access_key_id"`
	AWSSecretKey    string `json:"aws_secret_key"`
	AWSRegion       string `json:"aws_region"`
	// GitHub webhook
	GitHubWebhookSecret string `json:"github_webhook_secret"`
	GitHubRepoURL       string `json:"github_repo_url"`
}

type connectionResponse struct {
	Connection
	DOTokenMasked string `json:"do_token_masked,omitempty"`
	DOToken       string `json:"-"`
	RepoToken     string `json:"-"`
}

type wsMessage struct {
	JobID       string     `json:"job_id"`
	Status      string     `json:"status"`
	ProgressMsg string     `json:"progress_msg"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	Findings    *struct {
		Critical int `json:"critical"`
		High     int `json:"high"`
		Medium   int `json:"medium"`
		Low      int `json:"low"`
	} `json:"findings,omitempty"`
}

type ShareLink struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	JobID     string    `json:"job_id"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

type APIToken struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

type ConnectionHistory struct {
	Jobs []AuditJob `json:"jobs"`
}

type CompareResult struct {
	NewFindings   []map[string]interface{} `json:"new_findings"`
	FixedFindings []map[string]interface{} `json:"fixed_findings"`
	PrevJobID     string                   `json:"prev_job_id"`
}

type inviteRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type createScheduleRequest struct {
	ConnectionID string `json:"connection_id"`
	Interval     string `json:"interval"`
	Enabled      bool   `json:"enabled"`
}

type updateScheduleRequest struct {
	Interval string `json:"interval"`
	Enabled  bool   `json:"enabled"`
}

type createAPITokenRequest struct {
	Name string `json:"name"`
}

type LicenseInfo struct {
	Plan            string     `json:"plan"`
	IssuedTo        string     `json:"issued_to,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	MaxConnections  int        `json:"max_connections"`
	MaxUsers        int        `json:"max_users"`
	MaxAuditsMonth  int        `json:"max_audits_month"`
	Features        []string   `json:"features"`
	UsedConnections int        `json:"used_connections"`
	UsedAuditsMonth int        `json:"used_audits_month"`
	UsedUsers       int        `json:"used_users"`
}

type Tenant struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	SlackWebhookURL string `json:"slack_webhook_url"`
}

type ActivityLogEntry struct {
	ID           string    `json:"id"`
	UserEmail    string    `json:"user_email"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	IPAddress    string    `json:"ip_address"`
	CreatedAt    time.Time `json:"created_at"`
}

func maskToken(tok string) string {
	if len(tok) <= 12 {
		return "***"
	}
	return tok[:8] + "..." + tok[len(tok)-4:]
}

func connToResponse(c Connection) connectionResponse {
	r := connectionResponse{Connection: c}
	if c.ConnType == "do" || c.ConnType == "" {
		r.DOTokenMasked = maskToken(c.DOToken)
	}
	if c.ConnType == "aws" && c.AWSAccessKeyID != "" {
		r.AWSAccessKeyMasked = maskToken(c.AWSAccessKeyID)
	}
	// Never expose plaintext secrets in response
	r.DOToken = ""
	r.RepoToken = ""
	r.AWSAccessKeyID = ""
	r.AWSSecretKey = ""
	r.GitHubWebhookSecret = ""
	return r
}
