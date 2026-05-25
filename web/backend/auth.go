package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type ctxKey string

const ctxUserID ctxKey = "userID"
const ctxTenantID ctxKey = "tenantID"
const ctxUserRole ctxKey = "userRole"

var jwtSecret []byte

func init() {
	secret := envOr("JWT_SECRET", "")
	if secret == "" {
		secret = "cloudsecguard-dev-insecure-change-in-production"
		log.Println("WARNING: JWT_SECRET is not set — using an insecure default. " +
			"Set a strong random secret via JWT_SECRET env var before deploying to production.")
	}
	jwtSecret = []byte(secret)
}

// ── IP-based rate limiter ─────────────────────────────────────────────────────

type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipRateEntry
}

type ipRateEntry struct {
	count    int
	windowAt time.Time
}

var loginLimiter = &ipRateLimiter{entries: make(map[string]*ipRateEntry)}

func init() {
	loginLimiter.startCleanup(5 * time.Minute)
}

func (rl *ipRateLimiter) startCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			rl.mu.Lock()
			now := time.Now()
			for key, e := range rl.entries {
				if now.Sub(e.windowAt) > 10*time.Minute {
					delete(rl.entries, key)
				}
			}
			rl.mu.Unlock()
		}
	}()
}

func (rl *ipRateLimiter) allow(key string, maxPerWindow int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	e, ok := rl.entries[key]
	if !ok || now.Sub(e.windowAt) > window {
		rl.entries[key] = &ipRateEntry{count: 1, windowAt: now}
		return true
	}
	e.count++
	return e.count <= maxPerWindow
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.Index(fwd, ","); i > 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		return ip[:i]
	}
	return ip
}

type claims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func generateAccessToken(user User) (string, error) {
	c := claims{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(jwtSecret)
}

func generateRefreshToken() (string, string, error) {
	raw := uuid.New().String() + uuid.New().String()
	h := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(h[:])
	return raw, hash, nil
}

func validateAccessToken(tokenStr string) (*claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := tok.Claims.(*claims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return c, nil
}

func (srv *server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing token")
			return
		}
		raw := strings.TrimPrefix(auth, "Bearer ")

		// Try JWT first
		if c, err := validateAccessToken(raw); err == nil {
			ctx := context.WithValue(r.Context(), ctxUserID, c.UserID)
			ctx = context.WithValue(ctx, ctxTenantID, c.TenantID)
			ctx = context.WithValue(ctx, ctxUserRole, c.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Fall back to API token
		h := sha256.Sum256([]byte(raw))
		user, err := srv.getUserByAPIToken(r.Context(), hex.EncodeToString(h[:]))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, user.ID)
		ctx = context.WithValue(ctx, ctxTenantID, user.TenantID)
		ctx = context.WithValue(ctx, ctxUserRole, user.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (srv *server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(ctxUserRole).(string)
		if role != "owner" && role != "admin" {
			writeError(w, http.StatusForbidden, "admin only")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (srv *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Rate limit: 10 login attempts per minute per IP
	if !loginLimiter.allow(clientIP(r), 10, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	user, mfaSecret, mfaEnabled, err := srv.getUserAuthByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if mfaEnabled {
		if req.MFACode == "" {
			writeJSON(w, http.StatusOK, loginResponse{MFARequired: true})
			return
		}
		if !verifyTOTP(mfaSecret, req.MFACode, time.Now()) {
			writeError(w, http.StatusUnauthorized, "invalid MFA code")
			return
		}
	}

	accessToken, err := generateAccessToken(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	rawRT, hashRT, err := generateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := srv.storeRefreshToken(r.Context(), user.ID, hashRT, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "token store error")
		return
	}

	srv.cleanExpiredTokens(r.Context())

	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRT,
		User:         user,
	})
}

func (srv *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.RefreshToken != "" {
		h := sha256.Sum256([]byte(req.RefreshToken))
		_ = srv.deleteRefreshToken(r.Context(), hex.EncodeToString(h[:]))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "missing refresh_token")
		return
	}

	h := sha256.Sum256([]byte(req.RefreshToken))
	hashHex := hex.EncodeToString(h[:])

	user, err := srv.getUserByRefreshToken(r.Context(), hashHex)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	accessToken, err := generateAccessToken(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	writeJSON(w, http.StatusOK, refreshResponse{AccessToken: accessToken})
}

func (srv *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Rate limit: 5 registrations per minute per IP
	if !loginLimiter.allow("reg:"+clientIP(r), 5, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "too many registration attempts, try again later")
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "email and password of at least 8 characters are required")
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash error")
		return
	}
	user, err := srv.createUserWithTenant(r.Context(), req.Email, hash, req.TenantName, req.PreparedBy, "")
	if err != nil {
		writeError(w, http.StatusConflict, "account already exists or registration failed")
		return
	}
	accessToken, err := generateAccessToken(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}
	rawRT, hashRT, err := generateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}
	if err := srv.storeRefreshToken(r.Context(), user.ID, hashRT, time.Now().Add(7*24*time.Hour)); err != nil {
		writeError(w, http.StatusInternalServerError, "token store error")
		return
	}
	user.PasswordHash = ""
	writeJSON(w, http.StatusCreated, loginResponse{AccessToken: accessToken, RefreshToken: rawRT, User: user})
}

func hashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func checkPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (srv *server) handleGetAuthProviders(w http.ResponseWriter, r *http.Request) {
	googleEnabled := envOr("GOOGLE_CLIENT_ID", "") != "" &&
		envOr("GOOGLE_CLIENT_SECRET", "") != "" &&
		envOr("GOOGLE_REDIRECT_URL", "") != ""
	writeJSON(w, http.StatusOK, map[string]bool{
		"google": googleEnabled,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
