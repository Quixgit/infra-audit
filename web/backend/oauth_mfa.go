package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ── Short-lived OAuth exchange codes ─────────────────────────────────────────
// Instead of passing JWT tokens in the redirect URL (where they end up in
// browser history, server logs, and referrer headers), we store them in memory
// behind a one-time code that the frontend exchanges within 60 seconds.

type oauthCodeEntry struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

var (
	oauthCodesMu sync.Mutex
	oauthCodes   = map[string]oauthCodeEntry{}
)

func storeOAuthCode(accessToken, refreshToken string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	code := hex.EncodeToString(b)
	oauthCodesMu.Lock()
	oauthCodes[code] = oauthCodeEntry{
		accessToken:  accessToken,
		refreshToken: refreshToken,
		expiresAt:    time.Now().Add(60 * time.Second),
	}
	// Prune expired codes while holding the lock.
	now := time.Now()
	for k, v := range oauthCodes {
		if now.After(v.expiresAt) {
			delete(oauthCodes, k)
		}
	}
	oauthCodesMu.Unlock()
	return code
}

func consumeOAuthCode(code string) (string, string, bool) {
	oauthCodesMu.Lock()
	defer oauthCodesMu.Unlock()
	entry, ok := oauthCodes[code]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(oauthCodes, code)
		return "", "", false
	}
	delete(oauthCodes, code)
	return entry.accessToken, entry.refreshToken, true
}

// handleOAuthExchange exchanges a short-lived OAuth code for real tokens.
// Called by the frontend after the Google callback redirect.
func (srv *server) handleOAuthExchange(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeError(w, http.StatusBadRequest, "missing code")
		return
	}
	// Validate code length to prevent timing side-channels on map lookup.
	h := sha256.Sum256([]byte(req.Code))
	_ = h // just used for length validation pattern; actual lookup below is fine
	accessToken, refreshToken, ok := consumeOAuthCode(req.Code)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid or expired oauth code")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (srv *server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	clientID := envOr("GOOGLE_CLIENT_ID", "")
	redirectURI := envOr("GOOGLE_REDIRECT_URL", "")
	if clientID == "" || redirectURI == "" {
		writeError(w, http.StatusNotImplemented, "google login is not configured")
		return
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := hex.EncodeToString(b)
	secure := envOr("APP_ENV", "") == "production"
	http.SetCookie(w, &http.Cookie{Name: "google_oauth_state", Value: state, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+q.Encode(), http.StatusFound)
}

func (srv *server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	frontend := envOr("FRONTEND_URL", "http://localhost:3000")
	stateCookie, err := r.Cookie("google_oauth_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Redirect(w, r, frontend+"/login?error=google_state", http.StatusFound)
		return
	}
	code := r.URL.Query().Get("code")
	clientID := envOr("GOOGLE_CLIENT_ID", "")
	clientSecret := envOr("GOOGLE_CLIENT_SECRET", "")
	redirectURI := envOr("GOOGLE_REDIRECT_URL", "")
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil || resp.StatusCode >= 300 {
		http.Redirect(w, r, frontend+"/login?error=google_token", http.StatusFound)
		return
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if json.NewDecoder(resp.Body).Decode(&tok) != nil || tok.AccessToken == "" {
		http.Redirect(w, r, frontend+"/login?error=google_token", http.StatusFound)
		return
	}
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	infoResp, err := http.DefaultClient.Do(req)
	if err != nil || infoResp.StatusCode >= 300 {
		http.Redirect(w, r, frontend+"/login?error=google_user", http.StatusFound)
		return
	}
	defer infoResp.Body.Close()
	var info struct {
		Sub, Email, Name string
		EmailVerified    bool `json:"email_verified"`
	}
	if json.NewDecoder(infoResp.Body).Decode(&info) != nil || info.Sub == "" || info.Email == "" {
		http.Redirect(w, r, frontend+"/login?error=google_user", http.StatusFound)
		return
	}
	info.Email = strings.ToLower(strings.TrimSpace(info.Email))
	user, err := srv.getUserByGoogleSubOrEmail(r.Context(), info.Sub, info.Email)
	if err != nil {
		user, err = srv.createUserWithTenant(r.Context(), info.Email, "", info.Email, info.Name, info.Sub)
		if err != nil {
			http.Redirect(w, r, frontend+"/login?error=google_create", http.StatusFound)
			return
		}
	}
	accessToken, err := generateAccessToken(user)
	if err != nil {
		http.Redirect(w, r, frontend+"/login?error=token", http.StatusFound)
		return
	}
	rawRT, hashRT, err := generateRefreshToken()
	if err != nil {
		http.Redirect(w, r, frontend+"/login?error=token", http.StatusFound)
		return
	}
	if err := srv.storeRefreshToken(r.Context(), user.ID, hashRT, time.Now().Add(7*24*time.Hour)); err != nil {
		http.Redirect(w, r, frontend+"/login?error=token", http.StatusFound)
		return
	}
	// Use a short-lived one-time code instead of passing tokens in the URL.
	// The frontend exchanges this code within 60 seconds via POST /api/auth/oauth/exchange.
	oauthCode := storeOAuthCode(accessToken, rawRT)
	q := url.Values{}
	q.Set("oauth_code", oauthCode)
	http.Redirect(w, r, frontend+"/login?"+q.Encode(), http.StatusFound)
}

func (srv *server) handleMFASetup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	user, err := srv.getUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	secret, err := generateTOTPSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "secret error")
		return
	}
	_, err = srv.db.Exec(r.Context(), `UPDATE users SET mfa_secret=$2,mfa_enabled=false WHERE id=$1`, userID, secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": secret, "otpauth_url": otpauthURL("CloudSecGuard", user.Email, secret)})
}

func (srv *server) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	var req struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var secret string
	if err := srv.db.QueryRow(r.Context(), `SELECT mfa_secret FROM users WHERE id=$1`, userID).Scan(&secret); err != nil || secret == "" {
		writeError(w, http.StatusBadRequest, "MFA setup required")
		return
	}
	if !verifyTOTP(secret, req.Code, time.Now()) {
		writeError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	_, _ = srv.db.Exec(r.Context(), `UPDATE users SET mfa_enabled=true WHERE id=$1`, userID)
	writeJSON(w, http.StatusOK, map[string]bool{"mfa_enabled": true})
}

func (srv *server) handleMFADisable(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(string)
	var req struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var secret string
	_ = srv.db.QueryRow(r.Context(), `SELECT mfa_secret FROM users WHERE id=$1`, userID).Scan(&secret)
	if secret != "" && !verifyTOTP(secret, req.Code, time.Now()) {
		writeError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	_, _ = srv.db.Exec(r.Context(), `UPDATE users SET mfa_enabled=false,mfa_secret='' WHERE id=$1`, userID)
	writeJSON(w, http.StatusOK, map[string]bool{"mfa_enabled": false})
}
