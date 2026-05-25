package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func mustConnectDB() *pgxpool.Pool {
	dsn := envOr("DATABASE_URL", "")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("db config: %v", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute

	var pool *pgxpool.Pool
	for i := 0; i < 30; i++ {
		pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
		if err == nil {
			if err = pool.Ping(context.Background()); err == nil {
				break
			}
			pool.Close()
		}
		log.Printf("waiting for db (%d/30)...", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	return pool
}

func mustMigrateAndSeed(pool *pgxpool.Pool) {
	ctx := context.Background()

	_, err := pool.Exec(ctx, schema)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}

	for _, m := range schemaMigrations {
		if _, err := pool.Exec(ctx, m); err != nil {
			log.Printf("migration (non-fatal): %v", err)
		}
	}

	// Seed an admin user only on first run (no users exist yet).
	var userCount int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount)
	if userCount == 0 {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("random seed password: %v", err)
		}
		rawPwd := hex.EncodeToString(b)
		hash, err := bcrypt.GenerateFromPassword([]byte(rawPwd), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("bcrypt: %v", err)
		}
		adminEmail := envOr("ADMIN_EMAIL", "admin@localhost")
		_, err = pool.Exec(ctx, `
			INSERT INTO users (email, password_hash, role)
			VALUES ($1, $2, 'admin')
			ON CONFLICT (email) DO NOTHING`,
			adminEmail, string(hash),
		)
		if err != nil {
			log.Printf("seed admin: %v", err)
		} else {
			log.Printf("========================================")
			log.Printf("  INITIAL ADMIN CREDENTIALS")
			log.Printf("  Email:    %s", adminEmail)
			log.Printf("  Password: %s", rawPwd)
			log.Printf("  Change this password after first login!")
			log.Printf("========================================")
		}
	}
}

const schema = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,
    auditor_org     TEXT NOT NULL DEFAULT '',
    auditor_email   TEXT NOT NULL DEFAULT '',
    auditor_phone   TEXT NOT NULL DEFAULT '',
    auditor_website TEXT NOT NULL DEFAULT '',
    auditor_address TEXT NOT NULL DEFAULT '',
    prepared_by     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS connections (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    do_token       TEXT NOT NULL,
    project_id     TEXT NOT NULL DEFAULT '',
    scope_mode     TEXT NOT NULL DEFAULT 'project',
    spaces_buckets TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_jobs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id     UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status            TEXT NOT NULL DEFAULT 'pending',
    progress_msg      TEXT NOT NULL DEFAULT '',
    started_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at       TIMESTAMPTZ,
    html_path         TEXT NOT NULL DEFAULT '',
    docx_path         TEXT NOT NULL DEFAULT '',
    error_msg         TEXT NOT NULL DEFAULT '',
    findings_critical INT NOT NULL DEFAULT 0,
    findings_high     INT NOT NULL DEFAULT 0,
    findings_medium   INT NOT NULL DEFAULT 0,
    findings_low      INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

var schemaMigrations = []string{
	`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'admin'`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS conn_type TEXT NOT NULL DEFAULT 'do'`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS repo_source TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS repo_url TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS repo_token TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS repo_branch TEXT NOT NULL DEFAULT 'main'`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS repo_local_path TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS last_stack_detected TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE audit_jobs ADD COLUMN IF NOT EXISTS conn_type TEXT NOT NULL DEFAULT 'do'`,
	`ALTER TABLE audit_jobs ADD COLUMN IF NOT EXISTS stack_detected TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS notify_email BOOLEAN NOT NULL DEFAULT TRUE`,
	`CREATE TABLE IF NOT EXISTS schedules (
		id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		connection_id UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
		user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		interval      TEXT NOT NULL DEFAULT 'daily',
		enabled       BOOLEAN NOT NULL DEFAULT TRUE,
		next_run_at   TIMESTAMPTZ NOT NULL,
		last_run_at   TIMESTAMPTZ,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS share_links (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		job_id     UUID NOT NULL REFERENCES audit_jobs(id) ON DELETE CASCADE,
		token      TEXT UNIQUE NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS api_tokens (
		id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name         TEXT NOT NULL,
		token_hash   TEXT UNIQUE NOT NULL,
		token_prefix TEXT NOT NULL,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_used_at TIMESTAMPTZ
	)`,
	`CREATE TABLE IF NOT EXISTS notify_requests (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		type       TEXT NOT NULL,
		email      TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS finding_overrides (
		id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		job_id        UUID NOT NULL REFERENCES audit_jobs(id) ON DELETE CASCADE,
		source        TEXT NOT NULL DEFAULT 'findings',
		finding_index INT NOT NULL,
		status        TEXT NOT NULL DEFAULT 'open',
		note          TEXT NOT NULL DEFAULT '',
		updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(job_id, source, finding_index)
	)`,
	`CREATE TABLE IF NOT EXISTS evidence_items (
		id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		job_id        UUID REFERENCES audit_jobs(id) ON DELETE SET NULL,
		source        TEXT NOT NULL DEFAULT 'manual',
		evidence_type TEXT NOT NULL DEFAULT 'other',
		name          TEXT NOT NULL,
		description   TEXT NOT NULL DEFAULT '',
		content_type  TEXT NOT NULL DEFAULT 'application/octet-stream',
		size          BIGINT NOT NULL DEFAULT 0,
		data          BYTEA,
		file_path     TEXT NOT NULL DEFAULT '',
		expires_at    TIMESTAMPTZ NOT NULL,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS evidence_mappings (
		id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		evidence_id    UUID NOT NULL REFERENCES evidence_items(id) ON DELETE CASCADE,
		framework_slug TEXT NOT NULL,
		ctrl_id        TEXT NOT NULL,
		created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(evidence_id, framework_slug, ctrl_id)
	)`,
	`CREATE TABLE IF NOT EXISTS tenants (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name       TEXT NOT NULL,
		created_by UUID REFERENCES users(id) ON DELETE SET NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS tenant_members (
		tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		role       TEXT NOT NULL DEFAULT 'viewer',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY(tenant_id, user_id)
	)`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS google_sub TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_secret TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE`,
	`ALTER TABLE audit_jobs ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE`,
	`ALTER TABLE schedules ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE`,
	`ALTER TABLE api_tokens ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE`,
	`ALTER TABLE share_links ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE`,
	`ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE`,
	`ALTER TABLE finding_overrides ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE`,
	`INSERT INTO tenants(name, created_by)
	 SELECT COALESCE(NULLIF(u.auditor_org,''), u.email), u.id
	 FROM users u
	 WHERE NOT EXISTS (SELECT 1 FROM tenant_members tm WHERE tm.user_id = u.id)`,
	`INSERT INTO tenant_members(tenant_id, user_id, role)
	 SELECT t.id, u.id, CASE WHEN COALESCE(NULLIF(u.role,''), 'admin') = 'viewer' THEN 'viewer' ELSE 'owner' END
	 FROM users u
	 JOIN tenants t ON t.created_by = u.id
	 WHERE NOT EXISTS (SELECT 1 FROM tenant_members tm WHERE tm.user_id = u.id)`,
	`UPDATE connections c SET tenant_id = tm.tenant_id FROM tenant_members tm WHERE c.user_id = tm.user_id AND c.tenant_id IS NULL`,
	`UPDATE audit_jobs j SET tenant_id = c.tenant_id FROM connections c WHERE j.connection_id = c.id AND j.tenant_id IS NULL`,
	`UPDATE audit_jobs j SET tenant_id = tm.tenant_id FROM tenant_members tm WHERE j.user_id = tm.user_id AND j.tenant_id IS NULL`,
	`UPDATE schedules s SET tenant_id = tm.tenant_id FROM tenant_members tm WHERE s.user_id = tm.user_id AND s.tenant_id IS NULL`,
	`UPDATE api_tokens t SET tenant_id = tm.tenant_id FROM tenant_members tm WHERE t.user_id = tm.user_id AND t.tenant_id IS NULL`,
	`UPDATE evidence_items e SET tenant_id = tm.tenant_id FROM tenant_members tm WHERE e.user_id = tm.user_id AND e.tenant_id IS NULL`,
	`UPDATE finding_overrides f SET tenant_id = tm.tenant_id FROM tenant_members tm WHERE f.user_id = tm.user_id AND f.tenant_id IS NULL`,
	`UPDATE share_links s SET tenant_id = j.tenant_id FROM audit_jobs j WHERE s.job_id = j.id AND s.tenant_id IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_connections_tenant_id ON connections(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_jobs_tenant_id ON audit_jobs(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_evidence_items_tenant_id ON evidence_items(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_schedules_tenant_id ON schedules(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_api_tokens_tenant_id ON api_tokens(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_share_links_tenant_id ON share_links(tenant_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_sub ON users(google_sub) WHERE google_sub <> ''`,
	`CREATE TABLE IF NOT EXISTS remediation_tasks (
		id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id        UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		job_id           UUID REFERENCES audit_jobs(id) ON DELETE SET NULL,
		source           TEXT NOT NULL DEFAULT 'findings',
		finding_index    INT  NOT NULL DEFAULT -1,
		connection_id    UUID REFERENCES connections(id) ON DELETE SET NULL,
		connection_name  TEXT NOT NULL DEFAULT '',
		title            TEXT NOT NULL,
		severity         TEXT NOT NULL DEFAULT 'medium',
		resource_name    TEXT NOT NULL DEFAULT '',
		description      TEXT NOT NULL DEFAULT '',
		remediation_text TEXT NOT NULL DEFAULT '',
		risk_text        TEXT NOT NULL DEFAULT '',
		assigned_to      UUID REFERENCES users(id) ON DELETE SET NULL,
		lane             TEXT NOT NULL DEFAULT 'backlog',
		due_date         DATE,
		verify_job_id    UUID REFERENCES audit_jobs(id) ON DELETE SET NULL,
		verify_status    TEXT NOT NULL DEFAULT '',
		verified_at      TIMESTAMPTZ,
		created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_remediation_tasks_finding
		ON remediation_tasks(tenant_id, job_id, source, finding_index)
		WHERE job_id IS NOT NULL`,
	`CREATE TABLE IF NOT EXISTS remediation_comments (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		task_id    UUID NOT NULL REFERENCES remediation_tasks(id) ON DELETE CASCADE,
		user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		user_email TEXT NOT NULL DEFAULT '',
		body       TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_remediation_comments_task_id ON remediation_comments(task_id)`,
	`CREATE TABLE IF NOT EXISTS policies (
		id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name                TEXT NOT NULL,
		category            TEXT NOT NULL DEFAULT '',
		template_slug       TEXT NOT NULL DEFAULT '',
		content_html        TEXT NOT NULL DEFAULT '',
		file_path           TEXT NOT NULL DEFAULT '',
		file_name           TEXT NOT NULL DEFAULT '',
		status              TEXT NOT NULL DEFAULT 'Draft',
		version             INT  NOT NULL DEFAULT 1,
		approved_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
		approved_at         TIMESTAMPTZ,
		review_date         DATE,
		last_reviewed_at    TIMESTAMPTZ,
		created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS policy_control_mappings (
		policy_id      UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
		framework_slug TEXT NOT NULL,
		control_code   TEXT NOT NULL,
		PRIMARY KEY(policy_id, framework_slug, control_code)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_policies_tenant_id ON policies(tenant_id)`,
	// Monitoring / SLA
	`ALTER TABLE audit_jobs ADD COLUMN IF NOT EXISTS new_findings_count INT NOT NULL DEFAULT 0`,
	`ALTER TABLE audit_jobs ADD COLUMN IF NOT EXISTS regression_findings_count INT NOT NULL DEFAULT 0`,
	`CREATE TABLE IF NOT EXISTS sla_rules (
		id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		severity      TEXT NOT NULL,
		max_days_open INT  NOT NULL DEFAULT 7,
		notify_email  BOOL NOT NULL DEFAULT TRUE,
		notify_slack  BOOL NOT NULL DEFAULT FALSE,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(tenant_id, severity)
	)`,
	`CREATE TABLE IF NOT EXISTS finding_sla_breaches (
		id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		job_id         UUID NOT NULL REFERENCES audit_jobs(id) ON DELETE CASCADE,
		connection_id  UUID REFERENCES connections(id) ON DELETE SET NULL,
		source         TEXT NOT NULL DEFAULT 'findings',
		finding_index  INT  NOT NULL,
		title          TEXT NOT NULL DEFAULT '',
		severity       TEXT NOT NULL DEFAULT '',
		opened_at      TIMESTAMPTZ NOT NULL,
		breached_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		notified_at    TIMESTAMPTZ,
		UNIQUE(tenant_id, job_id, source, finding_index)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_sla_breaches_tenant ON finding_sla_breaches(tenant_id)`,
	`CREATE TABLE IF NOT EXISTS security_scores (
		id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		connection_id  UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
		job_id         UUID NOT NULL REFERENCES audit_jobs(id) ON DELETE CASCADE,
		score          INT  NOT NULL DEFAULT 100,
		critical_count INT  NOT NULL DEFAULT 0,
		high_count     INT  NOT NULL DEFAULT 0,
		medium_count   INT  NOT NULL DEFAULT 0,
		low_count      INT  NOT NULL DEFAULT 0,
		calculated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(job_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_security_scores_tenant ON security_scores(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_security_scores_conn ON security_scores(connection_id)`,
	`CREATE TABLE IF NOT EXISTS finding_changes (
		id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		job_id         UUID NOT NULL REFERENCES audit_jobs(id) ON DELETE CASCADE,
		connection_id  UUID REFERENCES connections(id) ON DELETE SET NULL,
		title          TEXT NOT NULL DEFAULT '',
		severity       TEXT NOT NULL DEFAULT '',
		change_type    TEXT NOT NULL DEFAULT 'new',
		occurred_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_finding_changes_tenant ON finding_changes(tenant_id)`,
	// Access Reviews
	`CREATE TABLE IF NOT EXISTS access_reviews (
		id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name          TEXT NOT NULL,
		description   TEXT NOT NULL DEFAULT '',
		review_type   TEXT NOT NULL DEFAULT 'manual',
		connection_id UUID REFERENCES connections(id) ON DELETE SET NULL,
		status        TEXT NOT NULL DEFAULT 'in_progress',
		due_date      DATE,
		completed_at  TIMESTAMPTZ,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS access_review_items (
		id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		review_id           UUID NOT NULL REFERENCES access_reviews(id) ON DELETE CASCADE,
		subject_name        TEXT NOT NULL DEFAULT '',
		subject_email       TEXT NOT NULL DEFAULT '',
		subject_role        TEXT NOT NULL DEFAULT '',
		access_level        TEXT NOT NULL DEFAULT '',
		last_active_at      TIMESTAMPTZ,
		decision            TEXT NOT NULL DEFAULT 'pending',
		decided_by_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
		decided_at          TIMESTAMPTZ,
		notes               TEXT NOT NULL DEFAULT '',
		UNIQUE(review_id, subject_email)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_access_reviews_tenant ON access_reviews(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_access_review_items_review ON access_review_items(review_id)`,
	// Auditor Portal
	`CREATE TABLE IF NOT EXISTS auditor_invites (
		id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id        UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name             TEXT NOT NULL,
		email            TEXT NOT NULL DEFAULT '',
		token            TEXT UNIQUE NOT NULL,
		permissions      TEXT NOT NULL DEFAULT '[]',
		expires_at       TIMESTAMPTZ NOT NULL,
		last_accessed_at TIMESTAMPTZ,
		created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS auditor_comments (
		id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		invite_id    UUID NOT NULL REFERENCES auditor_invites(id) ON DELETE CASCADE,
		tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		auditor_name TEXT NOT NULL DEFAULT '',
		section      TEXT NOT NULL DEFAULT '',
		item_id      TEXT NOT NULL DEFAULT '',
		body         TEXT NOT NULL,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_auditor_invites_tenant ON auditor_invites(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_auditor_invites_token ON auditor_invites(token)`,
	`CREATE INDEX IF NOT EXISTS idx_auditor_comments_invite ON auditor_comments(invite_id)`,
	// Per-tenant Slack webhook
	`ALTER TABLE tenants ADD COLUMN IF NOT EXISTS slack_webhook_url TEXT NOT NULL DEFAULT ''`,
	// Activity log
	`CREATE TABLE IF NOT EXISTS activity_log (
		id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		user_id       UUID REFERENCES users(id) ON DELETE SET NULL,
		user_email    TEXT NOT NULL DEFAULT '',
		action        TEXT NOT NULL,
		resource_type TEXT NOT NULL DEFAULT '',
		resource_id   TEXT NOT NULL DEFAULT '',
		meta          JSONB NOT NULL DEFAULT '{}',
		ip_address    TEXT NOT NULL DEFAULT '',
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_activity_log_tenant ON activity_log(tenant_id, created_at DESC)`,
	// SSL / DNS connection domains field
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS domains TEXT NOT NULL DEFAULT ''`,
	// Performance indexes
	`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens(token_hash)`,
	// ── AWS Scanner fields ─────────────────────────────────────────────────────
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS aws_access_key_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS aws_secret_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS aws_region TEXT NOT NULL DEFAULT ''`,
	// ── GitHub webhook fields ──────────────────────────────────────────────────
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS github_webhook_secret TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE connections ADD COLUMN IF NOT EXISTS github_repo_url TEXT NOT NULL DEFAULT ''`,
	// ── Custom compliance frameworks ───────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS custom_frameworks (
		id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		slug        TEXT NOT NULL,
		version     TEXT NOT NULL DEFAULT '1.0',
		description TEXT NOT NULL DEFAULT '',
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(tenant_id, slug)
	)`,
	`CREATE TABLE IF NOT EXISTS custom_controls (
		id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		framework_id UUID NOT NULL REFERENCES custom_frameworks(id) ON DELETE CASCADE,
		tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		ctrl_id      TEXT NOT NULL,
		name         TEXT NOT NULL,
		description  TEXT NOT NULL DEFAULT '',
		category     TEXT NOT NULL DEFAULT '',
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(framework_id, ctrl_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_custom_frameworks_tenant ON custom_frameworks(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_custom_controls_framework ON custom_controls(framework_id)`,
	// ── Weekly digest tracking ─────────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS digest_log (
		id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		sent_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_digest_log_tenant ON digest_log(tenant_id, sent_at DESC)`,
	// ── GitHub webhook events ──────────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS github_webhook_events (
		id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		connection_id   UUID REFERENCES connections(id) ON DELETE CASCADE,
		event_type      TEXT NOT NULL DEFAULT '',
		pr_number       INT  NOT NULL DEFAULT 0,
		pr_title        TEXT NOT NULL DEFAULT '',
		pr_head_sha     TEXT NOT NULL DEFAULT '',
		pr_repo_url     TEXT NOT NULL DEFAULT '',
		job_id          UUID REFERENCES audit_jobs(id) ON DELETE SET NULL,
		status          TEXT NOT NULL DEFAULT 'pending',
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_github_events_tenant ON github_webhook_events(tenant_id, created_at DESC)`,
}

// ── User queries ──────────────────────────────────────────────────────────────

func (srv *server) getUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := srv.db.QueryRow(ctx,
		`SELECT id,email,password_hash,auditor_org,auditor_email,auditor_phone,
		        auditor_website,auditor_address,prepared_by,role,mfa_enabled,notify_email,created_at
		 FROM users WHERE email=$1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AuditorOrg, &u.AuditorEmail,
		&u.AuditorPhone, &u.AuditorWebsite, &u.AuditorAddress, &u.PreparedBy,
		&u.Role, &u.MFAEnabled, &u.NotifyEmail, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	return srv.attachDefaultTenant(ctx, u)
}

func (srv *server) getUser(ctx context.Context, id string) (User, error) {
	var u User
	err := srv.db.QueryRow(ctx,
		`SELECT id,email,password_hash,auditor_org,auditor_email,auditor_phone,
		        auditor_website,auditor_address,prepared_by,role,mfa_enabled,notify_email,created_at
		 FROM users WHERE id=$1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AuditorOrg, &u.AuditorEmail,
		&u.AuditorPhone, &u.AuditorWebsite, &u.AuditorAddress, &u.PreparedBy,
		&u.Role, &u.MFAEnabled, &u.NotifyEmail, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	return srv.attachDefaultTenant(ctx, u)
}

func (srv *server) updateUserSettings(ctx context.Context, id string, req updateSettingsRequest) error {
	_, err := srv.db.Exec(ctx,
		`UPDATE users SET auditor_org=$2,auditor_email=$3,auditor_phone=$4,
		                  auditor_website=$5,auditor_address=$6,prepared_by=$7
		 WHERE id=$1`,
		id, req.AuditorOrg, req.AuditorEmail, req.AuditorPhone,
		req.AuditorWebsite, req.AuditorAddress, req.PreparedBy,
	)
	return err
}

func (srv *server) updateUserPassword(ctx context.Context, id, hash string) error {
	_, err := srv.db.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, id, hash)
	return err
}

func (srv *server) updateUserNotify(ctx context.Context, id string, notifyEmail bool) error {
	_, err := srv.db.Exec(ctx, `UPDATE users SET notify_email=$2 WHERE id=$1`, id, notifyEmail)
	return err
}

func (srv *server) getUserByRefreshToken(ctx context.Context, tokenHash string) (User, error) {
	var u User
	err := srv.db.QueryRow(ctx,
		`SELECT u.id,u.email,u.password_hash,u.auditor_org,u.auditor_email,u.auditor_phone,
		        u.auditor_website,u.auditor_address,u.prepared_by,u.role,u.notify_email,u.created_at
		 FROM refresh_tokens rt
		 JOIN users u ON u.id=rt.user_id
		 WHERE rt.token_hash=$1 AND rt.expires_at > NOW()`, tokenHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AuditorOrg, &u.AuditorEmail,
		&u.AuditorPhone, &u.AuditorWebsite, &u.AuditorAddress, &u.PreparedBy,
		&u.Role, &u.MFAEnabled, &u.NotifyEmail, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	return srv.attachDefaultTenant(ctx, u)
}

// ── Team queries ──────────────────────────────────────────────────────────────

func (srv *server) listTeamMembers(ctx context.Context, tenantID string) ([]TeamMember, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT u.id, tm.tenant_id, u.email, tm.role, tm.created_at
		FROM tenant_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.tenant_id=$1
		ORDER BY tm.created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.ID, &m.TenantID, &m.Email, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (srv *server) createTeamMember(ctx context.Context, tenantID, email, passwordHash, role string) (TeamMember, error) {
	var m TeamMember
	tx, err := srv.db.Begin(ctx)
	if err != nil {
		return m, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx,
		`INSERT INTO users(email,password_hash,role) VALUES($1,$2,$3)
		 ON CONFLICT(email) DO UPDATE SET email=EXCLUDED.email
		 RETURNING id,email,created_at`, email, passwordHash, role,
	).Scan(&m.ID, &m.Email, &m.CreatedAt)
	if err != nil {
		return m, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO tenant_members(tenant_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT(tenant_id,user_id) DO UPDATE SET role=EXCLUDED.role`, tenantID, m.ID, role)
	if err != nil {
		return m, err
	}
	if err := tx.Commit(ctx); err != nil {
		return m, err
	}
	m.TenantID = tenantID
	m.Role = role
	return m, nil
}

func (srv *server) deleteTeamMember(ctx context.Context, tenantID, id string) error {
	tag, err := srv.db.Exec(ctx, `DELETE FROM tenant_members WHERE tenant_id=$1 AND user_id=$2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// ── Connection queries ────────────────────────────────────────────────────────

const connCols = `id,tenant_id,user_id,conn_type,name,do_token,project_id,scope_mode,spaces_buckets,
repo_source,repo_url,repo_token,repo_branch,repo_local_path,last_stack_detected,domains,
aws_access_key_id,aws_secret_key,aws_region,github_webhook_secret,github_repo_url,created_at`

func scanConn(row interface {
	Scan(...interface{}) error
}) (Connection, error) {
	var c Connection
	err := row.Scan(
		&c.ID, &c.TenantID, &c.UserID, &c.ConnType, &c.Name, &c.DOToken,
		&c.ProjectID, &c.ScopeMode, &c.SpacesBuckets,
		&c.RepoSource, &c.RepoURL, &c.RepoToken, &c.RepoBranch, &c.RepoLocalPath,
		&c.LastStackDetected, &c.Domains,
		&c.AWSAccessKeyID, &c.AWSSecretKey, &c.AWSRegion,
		&c.GitHubWebhookSecret, &c.GitHubRepoURL, &c.CreatedAt,
	)
	if err != nil {
		return c, err
	}
	// Decrypt tokens — falls back to plaintext for legacy unencrypted values
	c.DOToken, _ = decryptToken(c.DOToken)
	c.RepoToken, _ = decryptToken(c.RepoToken)
	c.AWSAccessKeyID, _ = decryptToken(c.AWSAccessKeyID)
	c.AWSSecretKey, _ = decryptToken(c.AWSSecretKey)
	c.GitHubWebhookSecret, _ = decryptToken(c.GitHubWebhookSecret)
	return c, nil
}

func (srv *server) listConnections(ctx context.Context, tenantID string) ([]Connection, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT `+connCols+` FROM connections WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Connection
	for rows.Next() {
		c, err := scanConn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (srv *server) getConnection(ctx context.Context, id string) (Connection, error) {
	row := srv.db.QueryRow(ctx,
		`SELECT `+connCols+` FROM connections WHERE id=$1`, id)
	return scanConn(row)
}

func (srv *server) createConnection(ctx context.Context, tenantID, userID string, req createConnectionRequest) (Connection, error) {
	connType := req.ConnType
	if connType == "" {
		connType = "do"
	}
	branch := req.RepoBranch
	encRepoTok, err := encryptToken(req.RepoToken)
	if err != nil {
		return Connection{}, err
	}
	encDOTok, err := encryptToken(req.DOToken)
	if err != nil {
		return Connection{}, err
	}
	encAWSKey, err := encryptToken(req.AWSAccessKeyID)
	if err != nil {
		return Connection{}, err
	}
	encAWSSecret, err := encryptToken(req.AWSSecretKey)
	if err != nil {
		return Connection{}, err
	}
	encGHSecret, err := encryptToken(req.GitHubWebhookSecret)
	if err != nil {
		return Connection{}, err
	}
	row := srv.db.QueryRow(ctx,
		`INSERT INTO connections(user_id,tenant_id,conn_type,name,do_token,project_id,scope_mode,spaces_buckets,
		  repo_source,repo_url,repo_token,repo_branch,repo_local_path,domains,
		  aws_access_key_id,aws_secret_key,aws_region,github_webhook_secret,github_repo_url)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		 RETURNING `+connCols,
		userID, tenantID, connType, req.Name, encDOTok, req.ProjectID, req.ScopeMode, req.SpacesBuckets,
		req.RepoSource, req.RepoURL, encRepoTok, branch, req.RepoLocalPath, req.Domains,
		encAWSKey, encAWSSecret, req.AWSRegion, encGHSecret, req.GitHubRepoURL,
	)
	return scanConn(row)
}

func (srv *server) updateConnection(ctx context.Context, id, tenantID string, req createConnectionRequest) (Connection, error) {
	branch := req.RepoBranch

	// Preserve existing encrypted token if new one is blank
	var encTok string
	if req.RepoToken != "" {
		var err error
		encTok, err = encryptToken(req.RepoToken)
		if err != nil {
			return Connection{}, err
		}
	} else {
		// keep existing
		existing, err := srv.getConnection(ctx, id)
		if err == nil {
			encTok = existing.RepoToken
		}
	}

	// Preserve existing encrypted DO token if new one is blank
	var encDOTok string
	if req.DOToken != "" {
		var err2 error
		encDOTok, err2 = encryptToken(req.DOToken)
		if err2 != nil {
			return Connection{}, err2
		}
	} else {
		existing, err2 := srv.getConnection(ctx, id)
		if err2 == nil {
			// re-encrypt the already-decrypted value from scanConn
			encDOTok, _ = encryptToken(existing.DOToken)
		}
	}

	// AWS credentials
	var encAWSKey, encAWSSecret string
	if req.AWSAccessKeyID != "" {
		encAWSKey, _ = encryptToken(req.AWSAccessKeyID)
	} else {
		existing3, err3 := srv.getConnection(ctx, id)
		if err3 == nil {
			encAWSKey, _ = encryptToken(existing3.AWSAccessKeyID)
		}
	}
	if req.AWSSecretKey != "" {
		encAWSSecret, _ = encryptToken(req.AWSSecretKey)
	} else {
		existing4, err4 := srv.getConnection(ctx, id)
		if err4 == nil {
			encAWSSecret, _ = encryptToken(existing4.AWSSecretKey)
		}
	}

	row := srv.db.QueryRow(ctx,
		`UPDATE connections SET name=$3,do_token=$4,project_id=$5,scope_mode=$6,spaces_buckets=$7,
		  repo_source=$8,repo_url=$9,repo_token=$10,repo_branch=$11,repo_local_path=$12,domains=$13,
		  aws_access_key_id=$14,aws_secret_key=$15,aws_region=$16,github_repo_url=$17
		 WHERE id=$1 AND tenant_id=$2
		 RETURNING `+connCols,
		id, tenantID, req.Name, encDOTok, req.ProjectID, req.ScopeMode, req.SpacesBuckets,
		req.RepoSource, req.RepoURL, encTok, branch, req.RepoLocalPath, req.Domains,
		encAWSKey, encAWSSecret, req.AWSRegion, req.GitHubRepoURL,
	)
	return scanConn(row)
}

func (srv *server) updateConnectionStack(ctx context.Context, connID, stack string) {
	_, _ = srv.db.Exec(ctx,
		`UPDATE connections SET last_stack_detected=$2 WHERE id=$1`, connID, stack)
}

func (srv *server) deleteConnection(ctx context.Context, id, tenantID string) error {
	tag, err := srv.db.Exec(ctx, `DELETE FROM connections WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// ── Job queries ───────────────────────────────────────────────────────────────

func (srv *server) createJob(ctx context.Context, connectionID, userID, tenantID, connType string) (AuditJob, error) {
	if connType == "" {
		connType = "do"
	}
	var j AuditJob
	err := srv.db.QueryRow(ctx,
		`INSERT INTO audit_jobs(connection_id,user_id,tenant_id,conn_type,status,progress_msg)
		 VALUES($1,$2,$3,$4,'pending','Queued...')
		 RETURNING id,tenant_id,connection_id,user_id,conn_type,status,progress_msg,started_at,
		           finished_at,html_path,docx_path,error_msg,
		           findings_critical,findings_high,findings_medium,findings_low,stack_detected`,
		connectionID, userID, tenantID, connType,
	).Scan(&j.ID, &j.TenantID, &j.ConnectionID, &j.UserID, &j.ConnType, &j.Status, &j.ProgressMsg,
		&j.StartedAt, &j.FinishedAt, &j.HTMLPath, &j.DOCXPath, &j.ErrorMsg,
		&j.FindingsCritical, &j.FindingsHigh, &j.FindingsMedium, &j.FindingsLow,
		new(string)) // stack_detected ignored on create
	return j, err
}

const jobCols = `j.id,j.tenant_id,j.connection_id,c.name,j.user_id,j.conn_type,j.status,j.progress_msg,j.started_at,
j.finished_at,j.html_path,j.docx_path,j.error_msg,
j.findings_critical,j.findings_high,j.findings_medium,j.findings_low,j.stack_detected`

func scanJob(row interface{ Scan(...interface{}) error }) (AuditJob, error) {
	var j AuditJob
	var stackJSON string
	err := row.Scan(&j.ID, &j.TenantID, &j.ConnectionID, &j.ConnectionName, &j.UserID,
		&j.ConnType, &j.Status, &j.ProgressMsg, &j.StartedAt, &j.FinishedAt,
		&j.HTMLPath, &j.DOCXPath, &j.ErrorMsg,
		&j.FindingsCritical, &j.FindingsHigh, &j.FindingsMedium, &j.FindingsLow,
		&stackJSON)
	if err == nil && stackJSON != "" {
		_ = json.Unmarshal([]byte(stackJSON), &j.StackDetected)
	}
	return j, err
}

func (srv *server) getJob(ctx context.Context, id string) (AuditJob, error) {
	row := srv.db.QueryRow(ctx,
		`SELECT `+jobCols+`
		 FROM audit_jobs j
		 JOIN connections c ON c.id=j.connection_id
		 WHERE j.id=$1`, id)
	return scanJob(row)
}

func (srv *server) listJobs(ctx context.Context, tenantID string, limit, offset int) ([]AuditJob, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := srv.db.Query(ctx,
		`SELECT `+jobCols+`
		 FROM audit_jobs j
		 JOIN connections c ON c.id=j.connection_id
		 WHERE j.tenant_id=$1
		 ORDER BY j.started_at DESC
		 LIMIT $2 OFFSET $3`, tenantID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (srv *server) deleteJob(ctx context.Context, id, tenantID string) error {
	tag, err := srv.db.Exec(ctx, `DELETE FROM audit_jobs WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (srv *server) updateJobProgress(ctx context.Context, jobID, status, msg string) {
	_, err := srv.db.Exec(ctx,
		`UPDATE audit_jobs SET status=$2,progress_msg=$3 WHERE id=$1`, jobID, status, msg)
	if err != nil {
		log.Printf("updateJobProgress: %v", err)
	}
	srv.hub.broadcast(jobID, wsMessage{JobID: jobID, Status: status, ProgressMsg: msg})
}

func (srv *server) updateJobFailed(ctx context.Context, jobID, errMsg string) {
	now := time.Now().UTC()
	_, err := srv.db.Exec(ctx,
		`UPDATE audit_jobs SET status='failed',error_msg=$2,finished_at=$3 WHERE id=$1`,
		jobID, errMsg, now)
	if err != nil {
		log.Printf("updateJobFailed: %v", err)
	}
	srv.hub.broadcast(jobID, wsMessage{
		JobID: jobID, Status: "failed", ErrorMsg: errMsg, FinishedAt: &now,
	})
}

func (srv *server) updateJobDone(ctx context.Context, jobID, htmlPath, docxPath string, critical, high, medium, low int) {
	now := time.Now().UTC()
	_, err := srv.db.Exec(ctx,
		`UPDATE audit_jobs SET status='done',progress_msg='Complete',
		        finished_at=$2,html_path=$3,docx_path=$4,
		        findings_critical=$5,findings_high=$6,findings_medium=$7,findings_low=$8
		 WHERE id=$1`,
		jobID, now, htmlPath, docxPath, critical, high, medium, low)
	if err != nil {
		log.Printf("updateJobDone: %v", err)
	}
	srv.hub.broadcast(jobID, wsMessage{
		JobID: jobID, Status: "done", ProgressMsg: "Complete", FinishedAt: &now,
		Findings: &struct {
			Critical int `json:"critical"`
			High     int `json:"high"`
			Medium   int `json:"medium"`
			Low      int `json:"low"`
		}{critical, high, medium, low},
	})
}

// ── Refresh token queries ─────────────────────────────────────────────────────

func (srv *server) storeRefreshToken(ctx context.Context, tenantID, tokenHash string, expiresAt time.Time) error {
	_, err := srv.db.Exec(ctx,
		`INSERT INTO refresh_tokens(user_id,token_hash,expires_at) VALUES($1,$2,$3)`,
		tenantID, tokenHash, expiresAt)
	return err
}

func (srv *server) deleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := srv.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash=$1`, tokenHash)
	return err
}

func (srv *server) cleanExpiredTokens(ctx context.Context) {
	_, _ = srv.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < NOW()`)
}

// ── Dashboard queries ─────────────────────────────────────────────────────────

func (srv *server) getDashboardData(ctx context.Context, tenantID string) (DashboardData, error) {
	var d DashboardData

	_ = srv.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM connections WHERE tenant_id=$1`, tenantID,
	).Scan(&d.TotalConnections)

	_ = srv.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_jobs WHERE tenant_id=$1 AND started_at > NOW() - INTERVAL '7 days'`, tenantID,
	).Scan(&d.JobsThisWeek)

	_ = srv.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(findings_critical+findings_high+findings_medium+findings_low),0)
		 FROM audit_jobs WHERE tenant_id=$1 AND status='done'`, tenantID,
	).Scan(&d.TotalFindings)

	rows, err := srv.db.Query(ctx,
		`SELECT `+jobCols+`
		 FROM audit_jobs j
		 JOIN connections c ON c.id=j.connection_id
		 WHERE j.tenant_id=$1
		 ORDER BY j.started_at DESC LIMIT 5`, tenantID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			if j, err := scanJob(rows); err == nil {
				d.RecentJobs = append(d.RecentJobs, j)
			}
		}
	}
	if d.RecentJobs == nil {
		d.RecentJobs = []AuditJob{}
	}

	trendRows, err := srv.db.Query(ctx, `
		SELECT
		    gs.dt::date::text,
		    COALESCE(SUM(j.findings_critical),0)::int,
		    COALESCE(SUM(j.findings_high),0)::int,
		    COALESCE(SUM(j.findings_medium),0)::int,
		    COALESCE(SUM(j.findings_low),0)::int
		FROM generate_series(NOW()-INTERVAL '29 days', NOW(), INTERVAL '1 day') gs(dt)
		LEFT JOIN audit_jobs j
		    ON j.finished_at::date = gs.dt::date
		    AND j.tenant_id = $1
		    AND j.status = 'done'
		GROUP BY gs.dt::date
		ORDER BY gs.dt::date`, tenantID)
	if err == nil {
		defer trendRows.Close()
		for trendRows.Next() {
			var t FindingsTrendDay
			if err := trendRows.Scan(&t.Date, &t.Critical, &t.High, &t.Medium, &t.Low); err == nil {
				d.FindingsTrend = append(d.FindingsTrend, t)
			}
		}
	}
	if d.FindingsTrend == nil {
		d.FindingsTrend = []FindingsTrendDay{}
	}

	return d, nil
}

// ── Schedule queries ──────────────────────────────────────────────────────────

func nextRunTime(interval string) time.Time {
	if interval == "weekly" {
		return time.Now().Add(7 * 24 * time.Hour)
	}
	return time.Now().Add(24 * time.Hour)
}

func (srv *server) listSchedules(ctx context.Context, tenantID string) ([]Schedule, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT s.id,s.tenant_id,s.connection_id,c.name,s.user_id,s.interval,s.enabled,s.next_run_at,s.last_run_at,s.created_at
		 FROM schedules s
		 JOIN connections c ON c.id=s.connection_id
		 WHERE s.tenant_id=$1
		 ORDER BY s.created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ConnectionID, &s.ConnectionName, &s.UserID,
			&s.Interval, &s.Enabled, &s.NextRunAt, &s.LastRunAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (srv *server) createSchedule(ctx context.Context, tenantID, userID string, req createScheduleRequest) (Schedule, error) {
	nextRun := nextRunTime(req.Interval)
	var s Schedule
	err := srv.db.QueryRow(ctx,
		`INSERT INTO schedules(connection_id,user_id,tenant_id,interval,enabled,next_run_at)
		 VALUES($1,$2,$3,$4,$5,$6)
		 RETURNING id,tenant_id,connection_id,user_id,interval,enabled,next_run_at,last_run_at,created_at`,
		req.ConnectionID, userID, tenantID, req.Interval, req.Enabled, nextRun,
	).Scan(&s.ID, &s.TenantID, &s.ConnectionID, &s.UserID, &s.Interval, &s.Enabled, &s.NextRunAt, &s.LastRunAt, &s.CreatedAt)
	return s, err
}

func (srv *server) updateSchedule(ctx context.Context, id, tenantID string, req updateScheduleRequest) (Schedule, error) {
	var s Schedule
	err := srv.db.QueryRow(ctx,
		`UPDATE schedules SET interval=$3,enabled=$4 WHERE id=$1 AND tenant_id=$2
		 RETURNING id,tenant_id,connection_id,user_id,interval,enabled,next_run_at,last_run_at,created_at`,
		id, tenantID, req.Interval, req.Enabled,
	).Scan(&s.ID, &s.TenantID, &s.ConnectionID, &s.UserID, &s.Interval, &s.Enabled, &s.NextRunAt, &s.LastRunAt, &s.CreatedAt)
	return s, err
}

func (srv *server) deleteSchedule(ctx context.Context, id, tenantID string) error {
	tag, err := srv.db.Exec(ctx, `DELETE FROM schedules WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (srv *server) getDueSchedules(ctx context.Context) ([]Schedule, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT id,tenant_id,connection_id,user_id,interval,enabled,next_run_at,last_run_at,created_at
		 FROM schedules WHERE enabled=true AND next_run_at <= NOW()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ConnectionID, &s.UserID, &s.Interval, &s.Enabled,
			&s.NextRunAt, &s.LastRunAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (srv *server) advanceSchedule(ctx context.Context, id, interval string) error {
	nextRun := nextRunTime(interval)
	_, err := srv.db.Exec(ctx,
		`UPDATE schedules SET last_run_at=NOW(), next_run_at=$2 WHERE id=$1`,
		id, nextRun)
	return err
}

// ── Share link queries ────────────────────────────────────────────────────────

func (srv *server) createShareLink(ctx context.Context, jobID, token string) (ShareLink, error) {
	var s ShareLink
	err := srv.db.QueryRow(ctx,
		`INSERT INTO share_links(job_id,token) VALUES($1,$2)
		 RETURNING id,job_id,token,created_at`,
		jobID, token,
	).Scan(&s.ID, &s.JobID, &s.Token, &s.CreatedAt)
	return s, err
}

func (srv *server) getShareLinkByToken(ctx context.Context, token string) (ShareLink, error) {
	var s ShareLink
	err := srv.db.QueryRow(ctx,
		`SELECT id,job_id,token,created_at FROM share_links WHERE token=$1`, token,
	).Scan(&s.ID, &s.JobID, &s.Token, &s.CreatedAt)
	return s, err
}

// ── API token queries ─────────────────────────────────────────────────────────

func (srv *server) listAPITokens(ctx context.Context, tenantID string) ([]APIToken, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT id,tenant_id,user_id,name,token_prefix,created_at,last_used_at
		 FROM api_tokens WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIToken
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.TenantID, &t.UserID, &t.Name, &t.TokenPrefix, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (srv *server) createAPIToken(ctx context.Context, tenantID, userID, name, tokenHash, tokenPrefix string) (APIToken, error) {
	var t APIToken
	err := srv.db.QueryRow(ctx,
		`INSERT INTO api_tokens(tenant_id,user_id,name,token_hash,token_prefix) VALUES($1,$2,$3,$4,$5)
		 RETURNING id,tenant_id,user_id,name,token_prefix,created_at,last_used_at`,
		tenantID, userID, name, tokenHash, tokenPrefix,
	).Scan(&t.ID, &t.TenantID, &t.UserID, &t.Name, &t.TokenPrefix, &t.CreatedAt, &t.LastUsedAt)
	return t, err
}

func (srv *server) deleteAPIToken(ctx context.Context, id, tenantID string) error {
	tag, err := srv.db.Exec(ctx, `DELETE FROM api_tokens WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (srv *server) getUserByAPIToken(ctx context.Context, tokenHash string) (User, error) {
	var u User
	err := srv.db.QueryRow(ctx,
		`SELECT u.id,u.email,u.password_hash,u.auditor_org,u.auditor_email,u.auditor_phone,
		        u.auditor_website,u.auditor_address,u.prepared_by,u.role,u.notify_email,u.created_at
		 FROM api_tokens t
		 JOIN users u ON u.id=t.user_id
		 WHERE t.token_hash=$1`, tokenHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AuditorOrg, &u.AuditorEmail,
		&u.AuditorPhone, &u.AuditorWebsite, &u.AuditorAddress, &u.PreparedBy,
		&u.Role, &u.MFAEnabled, &u.NotifyEmail, &u.CreatedAt)
	if err == nil {
		_, _ = srv.db.Exec(ctx, `UPDATE api_tokens SET last_used_at=NOW() WHERE token_hash=$1`, tokenHash)
	}
	if err != nil {
		return u, err
	}
	return srv.attachDefaultTenant(ctx, u)
}

// ── Connection history queries ─────────────────────────────────────────────────

func (srv *server) getConnectionHistory(ctx context.Context, connectionID, tenantID string) ([]AuditJob, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT `+jobCols+`
		 FROM audit_jobs j
		 JOIN connections c ON c.id=j.connection_id
		 WHERE j.connection_id=$1 AND j.tenant_id=$2
		 ORDER BY j.started_at DESC`, connectionID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (srv *server) getPreviousJob(ctx context.Context, jobID, connectionID, tenantID string) (AuditJob, error) {
	row := srv.db.QueryRow(ctx,
		`SELECT `+jobCols+`
		 FROM audit_jobs j
		 JOIN connections c ON c.id=j.connection_id
		 WHERE j.connection_id=$2 AND j.tenant_id=$3 AND j.id != $1 AND j.status='done'
		 ORDER BY j.started_at DESC LIMIT 1`, jobID, connectionID, tenantID)
	return scanJob(row)
}

func (srv *server) updateJobStack(ctx context.Context, jobID, stackJSON string) {
	_, _ = srv.db.Exec(ctx, `UPDATE audit_jobs SET stack_detected=$2 WHERE id=$1`, jobID, stackJSON)
}

// ── Settings queries ──────────────────────────────────────────────────────────

func (srv *server) getSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := srv.db.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&value)
	return value, err
}

func (srv *server) setSetting(ctx context.Context, key, value string) error {
	_, err := srv.db.Exec(ctx,
		`INSERT INTO settings(key,value) VALUES($1,$2)
		 ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`,
		key, value)
	return err
}

// ── License queries ───────────────────────────────────────────────────────────

func (srv *server) getLicenseUsage(ctx context.Context, tenantID string) (int, int, error) {
	var conns, audits int
	_ = srv.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM connections WHERE tenant_id=$1`, tenantID,
	).Scan(&conns)
	_ = srv.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_jobs WHERE tenant_id=$1 AND started_at > date_trunc('month', NOW())`, tenantID,
	).Scan(&audits)
	return conns, audits, nil
}

func (srv *server) getUserCount(ctx context.Context) (int, error) {
	var n int
	err := srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// ── Asset helpers ─────────────────────────────────────────────────────────────

func userAssetPath(tenantID, filename string) string {
	return fmt.Sprintf("%s/users/%s/assets/%s",
		envOr("DATA_DIR", "/app/data"), tenantID, filename)
}

func defaultAssetPath(filename string) string {
	return fmt.Sprintf("%s/%s", envOr("ASSETS_DIR", "/app/assets"), filename)
}

// ── Finding override queries ──────────────────────────────────────────────────

func (srv *server) listFindingOverrides(ctx context.Context, tenantID string) ([]FindingOverride, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT id,user_id,job_id,source,finding_index,status,note,updated_at
		 FROM finding_overrides WHERE tenant_id=$1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FindingOverride
	for rows.Next() {
		var o FindingOverride
		if err := rows.Scan(&o.ID, &o.UserID, &o.JobID, &o.Source, &o.FindingIndex,
			&o.Status, &o.Note, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (srv *server) upsertFindingOverride(ctx context.Context, userID, tenantID, jobID, source string, idx int, status, note string) (FindingOverride, error) {
	var o FindingOverride
	err := srv.db.QueryRow(ctx,
		`INSERT INTO finding_overrides(user_id,tenant_id,job_id,source,finding_index,status,note)
		 VALUES($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT(job_id,source,finding_index) DO UPDATE
		   SET status=EXCLUDED.status, note=EXCLUDED.note, updated_at=NOW()
		 RETURNING id,user_id,job_id,source,finding_index,status,note,updated_at`,
		userID, tenantID, jobID, source, idx, status, note,
	).Scan(&o.ID, &o.UserID, &o.JobID, &o.Source, &o.FindingIndex, &o.Status, &o.Note, &o.UpdatedAt)
	return o, err
}

// ── Workspace (tenant) queries ────────────────────────────────────────────────

func (srv *server) getTenant(ctx context.Context, tenantID string) (Tenant, error) {
	var t Tenant
	err := srv.db.QueryRow(ctx,
		`SELECT id, name, slack_webhook_url FROM tenants WHERE id=$1`, tenantID,
	).Scan(&t.ID, &t.Name, &t.SlackWebhookURL)
	return t, err
}

func (srv *server) updateTenant(ctx context.Context, tenantID, name, slackURL string) (Tenant, error) {
	var t Tenant
	err := srv.db.QueryRow(ctx,
		`UPDATE tenants SET name=$2, slack_webhook_url=$3 WHERE id=$1
		 RETURNING id, name, slack_webhook_url`,
		tenantID, name, slackURL,
	).Scan(&t.ID, &t.Name, &t.SlackWebhookURL)
	return t, err
}

// ── Team role update ──────────────────────────────────────────────────────────

func (srv *server) updateTeamMemberRole(ctx context.Context, tenantID, userID, role string) error {
	tag, err := srv.db.Exec(ctx,
		`UPDATE tenant_members SET role=$3 WHERE tenant_id=$1 AND user_id=$2`,
		tenantID, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// ── Activity log queries ──────────────────────────────────────────────────────

func (srv *server) logActivity(ctx context.Context, tenantID, userID, userEmail, action, resourceType, resourceID, ipAddress string) {
	_, _ = srv.db.Exec(ctx,
		`INSERT INTO activity_log(tenant_id, user_id, user_email, action, resource_type, resource_id, ip_address)
		 VALUES($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7)`,
		tenantID, userID, userEmail, action, resourceType, resourceID, ipAddress)
}

func (srv *server) listActivityLog(ctx context.Context, tenantID string, limit int) ([]ActivityLogEntry, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT id, user_email, action, resource_type, resource_id, ip_address, created_at
		 FROM activity_log
		 WHERE tenant_id=$1
		 ORDER BY created_at DESC
		 LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActivityLogEntry
	for rows.Next() {
		var e ActivityLogEntry
		if err := rows.Scan(&e.ID, &e.UserEmail, &e.Action, &e.ResourceType, &e.ResourceID, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ── Asset helpers ─────────────────────────────────────────────────────────────

func resolveAsset(tenantID, filename, destDir string) string {
	userPath := userAssetPath(tenantID, filename)
	if _, err := os.Stat(userPath); err == nil {
		dest := destDir + "/" + filename
		if err := copyFile(userPath, dest); err == nil {
			return "assets/" + filename
		}
	}
	defPath := defaultAssetPath(filename)
	if _, err := os.Stat(defPath); err == nil {
		dest := destDir + "/" + filename
		if err := copyFile(defPath, dest); err == nil {
			return "assets/" + filename
		}
	}
	return ""
}
