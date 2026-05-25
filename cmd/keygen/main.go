package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LicenseClaims must stay in sync with web/backend/license.go
type LicenseClaims struct {
	Plan           string   `json:"plan"`
	IssuedTo       string   `json:"issued_to"`
	MaxConnections int      `json:"max_connections"`
	MaxUsers       int      `json:"max_users"`
	MaxAuditsMonth int      `json:"max_audits_month"`
	Features       []string `json:"features"`
	jwt.RegisteredClaims
}

var planFeatures = map[string][]string{
	"community":    {},
	"professional": {"scheduled_audits", "code_audit", "share_links", "api_tokens", "custom_branding"},
	"enterprise":   {"scheduled_audits", "code_audit", "share_links", "api_tokens", "custom_branding", "team", "sso"},
}

func main() {
	plan := flag.String("plan", "professional", "Plan: community|professional|enterprise")
	email := flag.String("email", "", "Issued-to email (required)")
	maxConns := flag.Int("max-connections", -1, "Max connections (-1 = unlimited)")
	maxUsers := flag.Int("max-users", -1, "Max users (-1 = unlimited)")
	maxAudits := flag.Int("max-audits-month", -1, "Max audits/month (-1 = unlimited)")
	expires := flag.String("expires", "2027-01-01", "Expiry date YYYY-MM-DD")
	privKeyPath := flag.String("private-key", "keys/private.pem", "RSA private key path")
	genKeys := flag.Bool("gen-keys", false, "Generate new RSA key pair and exit")
	flag.Parse()

	if *genKeys {
		if err := generateKeyPair(*privKeyPath); err != nil {
			log.Fatalf("keygen: %v", err)
		}
		return
	}

	if *email == "" {
		log.Fatal("--email is required")
	}

	if _, ok := planFeatures[*plan]; !ok {
		log.Fatalf("unknown plan %q; choose community|professional|enterprise", *plan)
	}

	// Generate keys if missing
	if _, err := os.Stat(*privKeyPath); os.IsNotExist(err) {
		log.Printf("private key not found, generating new key pair...")
		if err := generateKeyPair(*privKeyPath); err != nil {
			log.Fatalf("keygen: %v", err)
		}
	}

	privKeyBytes, err := os.ReadFile(*privKeyPath)
	if err != nil {
		log.Fatalf("read private key: %v", err)
	}
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privKeyBytes)
	if err != nil {
		log.Fatalf("parse private key: %v", err)
	}

	expiresAt, err := time.Parse("2006-01-02", *expires)
	if err != nil {
		log.Fatalf("invalid expiry date %q: %v", *expires, err)
	}
	expiresAt = time.Date(expiresAt.Year(), expiresAt.Month(), expiresAt.Day(), 23, 59, 59, 0, time.UTC)

	features := planFeatures[*plan]

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, LicenseClaims{
		Plan:           *plan,
		IssuedTo:       *email,
		MaxConnections: *maxConns,
		MaxUsers:       *maxUsers,
		MaxAuditsMonth: *maxAudits,
		Features:       features,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "infrajump",
			Subject:   *email,
		},
	})

	signed, err := tok.SignedString(privKey)
	if err != nil {
		log.Fatalf("sign token: %v", err)
	}

	fmt.Println(signed)

	log.Printf("License issued:")
	log.Printf("  plan:            %s", *plan)
	log.Printf("  issued_to:       %s", *email)
	log.Printf("  expires:         %s", expiresAt.Format("2006-01-02"))
	log.Printf("  max_connections: %s", limitStr(*maxConns))
	log.Printf("  max_users:       %s", limitStr(*maxUsers))
	log.Printf("  max_audits/mo:   %s", limitStr(*maxAudits))
	log.Printf("  features:        %s", strings.Join(features, ", "))
}

func limitStr(n int) string {
	if n < 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", n)
}

func generateKeyPair(privPath string) error {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate RSA key: %w", err)
	}

	dir := "keys"
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	privFile, err := os.OpenFile(privPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create private key file: %w", err)
	}
	defer privFile.Close()
	if err := pem.Encode(privFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}); err != nil {
		return fmt.Errorf("encode private key: %w", err)
	}

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal public key: %w", err)
	}
	pubFile, err := os.OpenFile("keys/public.pem", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create public key file: %w", err)
	}
	defer pubFile.Close()
	if err := pem.Encode(pubFile, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}); err != nil {
		return fmt.Errorf("encode public key: %w", err)
	}

	log.Printf("Generated: %s", privPath)
	log.Printf("Generated: keys/public.pem")
	log.Printf("Copy keys/public.pem → web/backend/keys/public.pem and rebuild the backend")
	return nil
}
