package platform

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"backend/internal/common"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func writePublicKey(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "public.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newVerifier(t *testing.T, key *rsa.PrivateKey) *TokenVerifier {
	t.Helper()
	verifier, err := NewTokenVerifier(writePublicKey(t, key), TokenIssuerName)
	if err != nil {
		t.Fatal(err)
	}
	return verifier
}

func signToken(t *testing.T, key *rsa.PrivateKey, method jwt.SigningMethod, claims jwt.RegisteredClaims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func validClaims(subject string) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    TokenIssuerName,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
}

func TestNewTokenVerifierErrors(t *testing.T) {
	if _, err := NewTokenVerifier(filepath.Join(t.TempDir(), "missing.pem"), "x"); err == nil {
		t.Error("expected error for missing key file")
	}
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTokenVerifier(bad, "x"); err == nil {
		t.Error("expected error for malformed key")
	}
}

func TestSetupTokenVerifierUsesEnvPath(t *testing.T) {
	key := newKey(t)
	t.Setenv("JWT_PUBLIC_KEY_PATH", writePublicKey(t, key))
	if _, err := SetupTokenVerifier(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("JWT_PUBLIC_KEY_PATH", filepath.Join(t.TempDir(), "nope.pem"))
	if _, err := SetupTokenVerifier(); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestVerifyValidToken(t *testing.T) {
	key := newKey(t)
	id := uuid.New()
	got, err := newVerifier(t, key).Verify(signToken(t, key, jwt.SigningMethodRS256, validClaims(id.String())))
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("user id = %s, want %s", got, id)
	}
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	key := newKey(t)
	otherKey := newKey(t)
	verifier := newVerifier(t, key)
	id := uuid.NewString()

	expired := validClaims(id)
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	wrongIssuer := validClaims(id)
	wrongIssuer.Issuer = "someone-else"
	noExpiry := validClaims(id)
	noExpiry.ExpiresAt = nil
	noSubject := validClaims("")
	badSubject := validClaims("not-a-uuid")

	tests := map[string]string{
		"garbage":       "not.a.token",
		"empty":         "",
		"expired":       signToken(t, key, jwt.SigningMethodRS256, expired),
		"wrong issuer":  signToken(t, key, jwt.SigningMethodRS256, wrongIssuer),
		"no expiration": signToken(t, key, jwt.SigningMethodRS256, noExpiry),
		"no subject":    signToken(t, key, jwt.SigningMethodRS256, noSubject),
		"bad subject":   signToken(t, key, jwt.SigningMethodRS256, badSubject),
		"wrong key":     signToken(t, otherKey, jwt.SigningMethodRS256, validClaims(id)),
		"wrong alg":     signToken(t, key, jwt.SigningMethodRS512, validClaims(id)),
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := verifier.Verify(token)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, common.ErrInvalidToken) {
				t.Fatalf("error %v does not wrap ErrInvalidToken", err)
			}
			if got != uuid.Nil {
				t.Fatalf("user id = %s, want nil", got)
			}
		})
	}
}

func TestVerifyUninitialized(t *testing.T) {
	var nilVerifier *TokenVerifier
	if _, err := nilVerifier.Verify("x"); err == nil {
		t.Error("nil verifier should fail")
	}
	if _, err := (&TokenVerifier{}).Verify("x"); err == nil {
		t.Error("verifier without key should fail")
	}
}
