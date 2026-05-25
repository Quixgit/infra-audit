package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"log"
)

type server struct {
	db  *pgxpool.Pool
	hub *hub
}

func main() {
	pool := mustConnectDB()
	defer pool.Close()
	mustMigrateAndSeed(pool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := &server{db: pool, hub: newHub()}

	go srv.startScheduler(ctx)
	go srv.startSLAChecker(ctx)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(90 * time.Second))
	// Limit request body size to 20 MB for all non-file-upload endpoints
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > 20<<20 {
				writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	origins := strings.Split(envOr("CORS_ORIGINS", "http://localhost:3000,http://frontend:3000"), ",")
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Static assets
	assetsDir := envOr("ASSETS_DIR", "/app/assets")
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "db unhealthy")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "ok"})
	})

	// Public routes
	r.Get("/api/auth/providers", srv.handleGetAuthProviders)
	r.Post("/api/auth/login", srv.handleLogin)
	r.Post("/api/auth/register", srv.handleRegister)
	r.Get("/api/auth/google/start", srv.handleGoogleStart)
	r.Get("/api/auth/google/callback", srv.handleGoogleCallback)
	r.Post("/api/auth/oauth/exchange", srv.handleOAuthExchange)
	r.Post("/api/auth/logout", srv.handleLogout)
	r.Post("/api/auth/refresh", srv.handleRefresh)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(srv.authMiddleware)

		r.Get("/api/dashboard", srv.handleGetDashboard)

		r.Get("/api/me", srv.handleGetMe)
		r.Put("/api/me/settings", srv.handleUpdateSettings)
		r.Post("/api/me/password", srv.handleChangePassword)
		r.Put("/api/me/notify", srv.handleUpdateNotify)
		r.Post("/api/me/assets/{assetType}", srv.handleUploadAsset)
		r.Post("/api/me/mfa/setup", srv.handleMFASetup)
		r.Post("/api/me/mfa/verify", srv.handleMFAVerify)
		r.Post("/api/me/mfa/disable", srv.handleMFADisable)

		r.Get("/api/connections", srv.handleListConnections)
		r.Post("/api/connections", srv.handleCreateConnection)
		r.Put("/api/connections/{id}", srv.handleUpdateConnection)
		r.Delete("/api/connections/{id}", srv.handleDeleteConnection)
		r.Post("/api/connections/{id}/test", srv.handleTestConnection)
		r.Post("/api/connections/test-git", srv.handleTestGit)
		r.Post("/api/connections/test-local", srv.handleTestLocal)

		r.Post("/api/audit/run/{connectionId}", srv.handleRunAudit)
		r.Get("/api/audit/jobs", srv.handleListJobs)
		r.Get("/api/audit/jobs/{id}", srv.handleGetJob)
		r.Delete("/api/audit/jobs/{id}", srv.handleDeleteJob)
		r.Get("/api/audit/jobs/{id}/download/html", srv.handleDownloadHTML)
		r.Get("/api/audit/jobs/{id}/download/docx", srv.handleDownloadDOCX)
		r.Get("/api/audit/jobs/{id}/findings", srv.handleGetFindings)
		r.Get("/api/audit/jobs/{id}/code-findings", srv.handleGetCodeFindings)
		r.Get("/api/audit/jobs/{id}/tf-findings", srv.handleGetTFFindings)
		r.Get("/api/audit/jobs/{id}/compare", srv.handleCompareJob)
		r.Post("/api/audit/jobs/{id}/share", srv.handleCreateShare)
		r.Post("/api/audit/run-bulk", srv.handleBulkRun)

		r.Get("/api/connections/{id}/history", srv.handleGetConnectionHistory)

		r.Get("/api/schedules", srv.handleListSchedules)
		r.Post("/api/schedules", srv.handleCreateSchedule)
		r.Put("/api/schedules/{id}", srv.handleUpdateSchedule)
		r.Delete("/api/schedules/{id}", srv.handleDeleteSchedule)

		r.Get("/api/tokens", srv.handleListAPITokens)
		r.Post("/api/tokens", srv.handleCreateAPIToken)
		r.Delete("/api/tokens/{id}", srv.handleDeleteAPIToken)

		r.Get("/api/findings", srv.handleGetAggregatedFindings)
		r.Put("/api/findings/override", srv.handleSetFindingOverride)

		r.Get("/api/compliance/frameworks", srv.handleGetComplianceFrameworks)
		r.Get("/api/compliance/frameworks/{slug}", srv.handleGetComplianceFramework)

		r.Get("/api/evidence", srv.handleListEvidence)
		r.Post("/api/evidence/upload", srv.handleUploadEvidence)
		r.Get("/api/evidence/counts", srv.handleGetEvidenceCounts)
		r.Get("/api/evidence/by-control/{framework}/{ctrl_id}", srv.handleGetEvidenceByControl)
		r.Get("/api/evidence/{id}/download", srv.handleDownloadEvidence)
		r.Delete("/api/evidence/{id}", srv.handleDeleteEvidence)
		r.Put("/api/evidence/{id}/mappings", srv.handleSetEvidenceMappings)

		r.Get("/api/license", srv.handleGetLicense)
		r.Post("/api/license/activate", srv.handleActivateLicense)

		// Monitoring / SLA
		r.Get("/api/monitoring/overview", srv.handleGetMonitoringOverview)
		r.Get("/api/monitoring/scores", srv.handleGetConnectionScoresList)
		r.Get("/api/monitoring/scores/:connectionId", srv.handleGetConnectionScores)
		r.Get("/api/monitoring/sla-breaches", srv.handleGetSLABreaches)
		r.Get("/api/sla-rules", srv.handleGetSLARules)
		r.Put("/api/sla-rules", srv.handleUpdateSLARules)

		// Policies
		r.Get("/api/policies/templates", srv.handleListPolicyTemplates)
		r.Get("/api/policies/stats", srv.handleGetPolicyStats)
		r.Get("/api/policies", srv.handleListPolicies)
		r.Post("/api/policies", srv.handleCreatePolicy)
		r.Put("/api/policies/{id}", srv.handleUpdatePolicy)
		r.Delete("/api/policies/{id}", srv.handleDeletePolicy)
		r.Post("/api/policies/{id}/approve", srv.handleApprovePolicy)
		r.Post("/api/policies/{id}/review", srv.handleReviewPolicy)
		r.Post("/api/policies/upload", srv.handleUploadPolicy)
		r.Get("/api/policies/{id}/download", srv.handleDownloadPolicy)

		// Auditor Invites management
		r.Get("/api/auditor-invites", srv.handleListAuditorInvites)
		r.Post("/api/auditor-invites", srv.handleCreateAuditorInvite)
		r.Delete("/api/auditor-invites/{id}", srv.handleDeleteAuditorInvite)
		r.Get("/api/auditor-invites/{id}/comments", srv.handleListInviteComments)

		// Access Reviews
		r.Get("/api/access-reviews/stats", srv.handleGetAccessReviewStats)
		r.Get("/api/access-reviews", srv.handleListAccessReviews)
		r.Post("/api/access-reviews", srv.handleCreateAccessReview)
		r.Put("/api/access-reviews/{id}", srv.handleUpdateAccessReview)
		r.Delete("/api/access-reviews/{id}", srv.handleDeleteAccessReview)
		r.Post("/api/access-reviews/{id}/complete", srv.handleCompleteAccessReview)
		r.Get("/api/access-reviews/{id}/items", srv.handleGetReviewItems)
		r.Post("/api/access-reviews/{id}/items", srv.handleAddReviewItem)
		r.Put("/api/access-reviews/{id}/items/{itemId}", srv.handleUpdateReviewItem)
		r.Post("/api/access-reviews/{id}/import-do", srv.handleImportDOTeam)

		r.Get("/api/remediation/tasks", srv.handleListRemediationTasks)
		r.Post("/api/remediation/tasks", srv.handleCreateRemediationTask)
		r.Put("/api/remediation/tasks/{id}", srv.handleUpdateRemediationTask)
		r.Delete("/api/remediation/tasks/{id}", srv.handleDeleteRemediationTask)
		r.Get("/api/remediation/tasks/{id}/comments", srv.handleListRemediationComments)
		r.Post("/api/remediation/tasks/{id}/comments", srv.handleAddRemediationComment)
		r.Post("/api/remediation/tasks/{id}/verify", srv.handleVerifyFix)
		r.Get("/api/remediation/tasks/{id}/verify-result", srv.handleVerifyResult)

		// Workspace (tenant) settings
		r.Get("/api/workspace", srv.handleGetWorkspace)
		r.Put("/api/workspace", srv.handleUpdateWorkspace)

		// Branding asset preview
		r.Get("/api/me/assets/{assetType}", srv.handleGetAsset)

		// Activity log
		r.Get("/api/activity-log", srv.handleGetActivityLog)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(srv.adminOnly)
			// Team management
			r.Get("/api/team", srv.handleListTeam)
			r.Post("/api/team/invite", srv.handleInviteTeamMember)
			r.Patch("/api/team/{id}", srv.handleUpdateTeamMember)
			r.Delete("/api/team/{id}", srv.handleDeleteTeamMember)
			// License preview (admin only — not for regular users)
			r.Post("/api/license/preview-plan", srv.handleSetAdminPreviewPlan)
			// Module toggles (admin only)
			r.Get("/api/admin/modules", srv.handleGetModules)
			r.Put("/api/admin/modules", srv.handleSetModules)
		})
	})

	// Auditor Portal (public, token-based)
	r.Get("/api/auditor/{token}/verify", srv.handleVerifyAuditorToken)
	r.Get("/api/auditor/{token}/compliance", srv.handleAuditorCompliance)
	r.Get("/api/auditor/{token}/evidence", srv.handleAuditorEvidence)
	r.Get("/api/auditor/{token}/policies", srv.handleAuditorPolicies)
	r.Get("/api/auditor/{token}/findings/summary", srv.handleAuditorFindingsSummary)
	r.Get("/api/auditor/{token}/evidence/{id}/download", srv.handleAuditorDownloadEvidence)
	r.Get("/api/auditor/{token}/policies/{id}/download", srv.handleAuditorDownloadPolicy)
	r.Post("/api/auditor/{token}/comments", srv.handleAuditorAddComment)
	r.Get("/api/auditor/{token}/comments", srv.handleAuditorListComments)

	// Public share endpoint
	r.Get("/api/share/{token}", srv.handleGetShare)
	r.Post("/api/notify-me", srv.handleNotifyMe)

	// WebSocket — auth handled inside handler via query param
	r.Get("/api/ws/jobs/{id}", srv.handleJobWS)

	port := envOr("PORT", "8080")
	log.Printf("CloudSecGuard backend listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
