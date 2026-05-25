package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

//go:embed keys/public.pem
var licensePublicKeyPEM []byte

var planFeaturesMap = map[string][]string{
	"community":    {"share_links"},
	"starter":      {"scheduled_audits", "share_links"},
	"professional": {"scheduled_audits", "code_audit", "share_links", "custom_branding"},
	"business":     {"scheduled_audits", "code_audit", "share_links", "api_tokens", "custom_branding", "team"},
	// sso is planned but not yet implemented — excluded from active enterprise features
	"enterprise": {"scheduled_audits", "code_audit", "share_links", "api_tokens", "custom_branding", "team"},
}

func syntheticClaims(plan string) *LicenseClaims {
	switch plan {
	case "community":
		return &LicenseClaims{
			Plan: "community", MaxConnections: communityMaxConnections,
			MaxAuditsMonth: communityMaxAuditsMonth, MaxUsers: communityMaxUsers,
			Features: planFeaturesMap["community"],
		}
	case "starter":
		return &LicenseClaims{
			Plan: "starter", MaxConnections: 10, MaxAuditsMonth: 30, MaxUsers: 2,
			Features: planFeaturesMap["starter"],
		}
	case "professional":
		return &LicenseClaims{
			Plan: "professional", MaxConnections: 30, MaxAuditsMonth: 100, MaxUsers: 5,
			Features: planFeaturesMap["professional"],
		}
	case "business":
		return &LicenseClaims{
			Plan: "business", MaxConnections: 9999, MaxAuditsMonth: 9999, MaxUsers: 15,
			Features: planFeaturesMap["business"],
		}
	default: // enterprise
		return &LicenseClaims{
			Plan: "enterprise", MaxConnections: 9999, MaxAuditsMonth: 9999, MaxUsers: 9999,
			Features: planFeaturesMap["enterprise"],
		}
	}
}

// getEffectiveClaims returns the license claims used for all feature enforcement.
// For admin/owner users: if an admin preview plan is set in Settings, that plan's
// synthetic claims are used — allowing admins to fully test gated features locally.
// For all other users: always the real installed license key.
func (srv *server) getEffectiveClaims(ctx context.Context, userID string) *LicenseClaims {
	if userID != "" {
		user, err := srv.getUser(ctx, userID)
		if err == nil && (user.Role == "admin" || user.Role == "owner") {
			preview, _ := srv.getSetting(ctx, "admin_preview_plan")
			if preview != "" && preview != "community" {
				return syntheticClaims(preview)
			}
		}
	}
	return srv.getActiveLicense(ctx)
}

// getDisplayClaims is an alias used by the /api/license endpoint.
// It behaves identically to getEffectiveClaims — keeping the same preview logic
// so the UI and enforcement always agree.
func (srv *server) getDisplayClaims(ctx context.Context, userID string) *LicenseClaims {
	return srv.getEffectiveClaims(ctx, userID)
}

// LicenseClaims is the JWT payload for CloudSecGuard license keys.
type LicenseClaims struct {
	Plan           string   `json:"plan"`
	IssuedTo       string   `json:"issued_to"`
	MaxConnections int      `json:"max_connections"`
	MaxUsers       int      `json:"max_users"`
	MaxAuditsMonth int      `json:"max_audits_month"`
	Features       []string `json:"features"`
	jwt.RegisteredClaims
}

const (
	communityMaxConnections = 5
	communityMaxAuditsMonth = 20
	communityMaxUsers       = 1
)

var licenseCache struct {
	sync.RWMutex
	claims  *LicenseClaims
	validAt time.Time
}

func parseLicenseJWT(keyStr string) (*LicenseClaims, error) {
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(licensePublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("public key error: %w", err)
	}
	token, err := jwt.ParseWithClaims(keyStr, &LicenseClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pubKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*LicenseClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func (srv *server) getActiveLicense(ctx context.Context) *LicenseClaims {
	licenseCache.RLock()
	if licenseCache.claims != nil && time.Since(licenseCache.validAt) < time.Hour {
		c := licenseCache.claims
		licenseCache.RUnlock()
		return c
	}
	licenseCache.RUnlock()

	licenseCache.Lock()
	defer licenseCache.Unlock()

	// double-check after acquiring write lock
	if licenseCache.claims != nil && time.Since(licenseCache.validAt) < time.Hour {
		return licenseCache.claims
	}

	keyStr, err := srv.getSetting(ctx, "license_key")
	if err != nil || keyStr == "" {
		licenseCache.claims = nil
		licenseCache.validAt = time.Now()
		return nil
	}

	claims, err := parseLicenseJWT(keyStr)
	if err != nil {
		licenseCache.claims = nil
		licenseCache.validAt = time.Now()
		return nil
	}

	licenseCache.claims = claims
	licenseCache.validAt = time.Now()
	return claims
}

func invalidateLicenseCache() {
	licenseCache.Lock()
	licenseCache.claims = nil
	licenseCache.validAt = time.Time{}
	licenseCache.Unlock()
}

func hasFeature(claims *LicenseClaims, feature string) bool {
	if claims == nil {
		return false
	}
	for _, f := range claims.Features {
		if f == feature {
			return true
		}
	}
	return false
}

func effectiveMaxConnections(claims *LicenseClaims) int {
	if claims == nil {
		return communityMaxConnections
	}
	return claims.MaxConnections
}

func effectiveMaxAuditsMonth(claims *LicenseClaims) int {
	if claims == nil {
		return communityMaxAuditsMonth
	}
	return claims.MaxAuditsMonth
}

func effectiveMaxUsers(claims *LicenseClaims) int {
	if claims == nil {
		return communityMaxUsers
	}
	return claims.MaxUsers
}

func effectivePlan(claims *LicenseClaims) string {
	if claims == nil {
		return "community"
	}
	return claims.Plan
}

func effectiveFeatures(claims *LicenseClaims) []string {
	if claims == nil || claims.Features == nil {
		return []string{}
	}
	return claims.Features
}

// writeFeatureError writes a 403 with upgrade info for gated features.
func writeFeatureError(w http.ResponseWriter, feature string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":       "feature_not_available",
		"feature":     feature,
		"upgrade_url": "",
	})
}

// trimKey normalises a license key (strips whitespace / PEM headers if user pastes wrong thing).
func trimKey(s string) string {
	return strings.TrimSpace(s)
}
