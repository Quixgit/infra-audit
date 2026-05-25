package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type TenantMembership struct {
	TenantID   string
	TenantName string
	UserID     string
	Role       string
}

func (srv *server) getDefaultTenantMembership(ctx context.Context, userID string) (TenantMembership, error) {
	var m TenantMembership
	err := srv.db.QueryRow(ctx, `
		SELECT tm.tenant_id, t.name, tm.user_id, tm.role
		FROM tenant_members tm
		JOIN tenants t ON t.id = tm.tenant_id
		WHERE tm.user_id=$1
		ORDER BY CASE tm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'member' THEN 2 ELSE 3 END, tm.created_at
		LIMIT 1`, userID).Scan(&m.TenantID, &m.TenantName, &m.UserID, &m.Role)
	return m, err
}

func (srv *server) attachDefaultTenant(ctx context.Context, user User) (User, error) {
	m, err := srv.getDefaultTenantMembership(ctx, user.ID)
	if err != nil {
		return user, err
	}
	user.TenantID = m.TenantID
	user.TenantName = m.TenantName
	user.Role = m.Role
	return user, nil
}

func (srv *server) createUserWithTenant(ctx context.Context, email, passwordHash, tenantName, preparedBy, googleSub string) (User, error) {
	if tenantName == "" {
		tenantName = email
	}
	var u User
	tx, err := srv.db.Begin(ctx)
	if err != nil {
		return u, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		INSERT INTO users(email,password_hash,auditor_org,auditor_email,prepared_by,role,google_sub)
		VALUES($1,$2,$3,$1,$4,'admin',$5)
		RETURNING id,email,password_hash,auditor_org,auditor_email,auditor_phone,auditor_website,auditor_address,prepared_by,role,mfa_enabled,notify_email,created_at`,
		email, passwordHash, tenantName, preparedBy, googleSub,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AuditorOrg, &u.AuditorEmail, &u.AuditorPhone, &u.AuditorWebsite, &u.AuditorAddress, &u.PreparedBy, &u.Role, &u.MFAEnabled, &u.NotifyEmail, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	var tenantID string
	if err := tx.QueryRow(ctx, `INSERT INTO tenants(name,created_by) VALUES($1,$2) RETURNING id`, tenantName, u.ID).Scan(&tenantID); err != nil {
		return u, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tenant_members(tenant_id,user_id,role) VALUES($1,$2,'owner')`, tenantID, u.ID); err != nil {
		return u, err
	}
	if err := tx.Commit(ctx); err != nil {
		return u, err
	}
	u.TenantID = tenantID
	u.TenantName = tenantName
	u.Role = "owner"
	return u, nil
}

func (srv *server) getUserAuthByEmail(ctx context.Context, email string) (User, string, bool, error) {
	var u User
	var mfaSecret string
	err := srv.db.QueryRow(ctx, `
		SELECT id,email,password_hash,auditor_org,auditor_email,auditor_phone,
		       auditor_website,auditor_address,prepared_by,role,mfa_secret,mfa_enabled,notify_email,created_at
		FROM users WHERE email=$1`, strings.ToLower(strings.TrimSpace(email)),
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AuditorOrg, &u.AuditorEmail,
		&u.AuditorPhone, &u.AuditorWebsite, &u.AuditorAddress, &u.PreparedBy,
		&u.Role, &mfaSecret, &u.MFAEnabled, &u.NotifyEmail, &u.CreatedAt)
	if err != nil {
		return u, "", false, err
	}
	u, err = srv.attachDefaultTenant(ctx, u)
	return u, mfaSecret, u.MFAEnabled, err
}

func (srv *server) getUserByGoogleSubOrEmail(ctx context.Context, googleSub, email string) (User, error) {
	var u User
	err := srv.db.QueryRow(ctx, `
		SELECT id,email,password_hash,auditor_org,auditor_email,auditor_phone,
		       auditor_website,auditor_address,prepared_by,role,mfa_enabled,notify_email,created_at
		FROM users WHERE google_sub=$1 OR email=$2 LIMIT 1`, googleSub, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AuditorOrg, &u.AuditorEmail,
		&u.AuditorPhone, &u.AuditorWebsite, &u.AuditorAddress, &u.PreparedBy,
		&u.Role, &u.MFAEnabled, &u.NotifyEmail, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	_, _ = srv.db.Exec(ctx, `UPDATE users SET google_sub=$2 WHERE id=$1 AND google_sub=''`, u.ID, googleSub)
	return srv.attachDefaultTenant(ctx, u)
}

func generateTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), "="), nil
}

func verifyTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	for offset := int64(-1); offset <= 1; offset++ {
		if totpCode(secret, now.Add(time.Duration(offset)*30*time.Second)) == code {
			return true
		}
	}
	return false
}

func totpCode(secret string, t time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return ""
	}
	counter := uint64(t.Unix() / 30)
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg)
	sum := mac.Sum(nil)
	o := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[o])&0x7f)<<24 | (uint32(sum[o+1])&0xff)<<16 | (uint32(sum[o+2])&0xff)<<8 | (uint32(sum[o+3]) & 0xff)
	return fmt.Sprintf("%06d", bin%1000000)
}

func otpauthURL(issuer, account, secret string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + v.Encode()
}
