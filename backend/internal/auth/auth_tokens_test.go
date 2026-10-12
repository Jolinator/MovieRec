package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"backend/internal/platform"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func writeKeyPair(t *testing.T) (privatePath, publicPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privatePath = filepath.Join(dir, "private.pem")
	publicPath = filepath.Join(dir, "public.pem")
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	if err := os.WriteFile(privatePath, privatePEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, publicPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return privatePath, publicPath
}

func TestNewTokenIssuerErrors(t *testing.T) {
	if _, err := NewTokenIssuer(filepath.Join(t.TempDir(), "missing.pem"), "iss"); err == nil {
		t.Error("missing key file should fail")
	}
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTokenIssuer(bad, "iss"); err == nil {
		t.Error("invalid key should fail")
	}
}

func TestSetupTokenIssuerUsesEnvironment(t *testing.T) {
	private, _ := writeKeyPair(t)
	t.Setenv("JWT_PRIVATE_KEY_PATH", private)
	if issuer, err := SetupTokenIssuer(); err != nil || issuer == nil {
		t.Fatalf("issuer = %v, err = %v", issuer, err)
	}
	t.Setenv("JWT_PRIVATE_KEY_PATH", filepath.Join(t.TempDir(), "missing.pem"))
	if _, err := SetupTokenIssuer(); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestIssueOnUninitializedIssuer(t *testing.T) {
	var nilIssuer *TokenIssuer
	if _, err := nilIssuer.Issue(User{ID: uuid.New()}); err == nil {
		t.Error("nil issuer should fail")
	}
	if _, err := (&TokenIssuer{}).Issue(User{ID: uuid.New()}); err == nil {
		t.Error("issuer without key should fail")
	}
}

func TestIssuedTokenVerifiesWithPlatformVerifier(t *testing.T) {
	private, public := writeKeyPair(t)
	issuer, err := NewTokenIssuer(private, platform.TokenIssuerName)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := platform.NewTokenVerifier(public, platform.TokenIssuerName)
	if err != nil {
		t.Fatal(err)
	}
	user := User{ID: uuid.New(), Username: "alice"}
	token, err := issuer.Issue(user)
	if err != nil {
		t.Fatal(err)
	}
	got, err := verifier.Verify(token)
	if err != nil || got != user.ID {
		t.Fatalf("got %v, err = %v", got, err)
	}
}

func TestIssuedTokenClaims(t *testing.T) {
	private, _ := writeKeyPair(t)
	issuer, err := NewTokenIssuer(private, "my-issuer")
	if err != nil {
		t.Fatal(err)
	}
	user := User{ID: uuid.New()}
	signed, err := issuer.Issue(user)
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.RegisteredClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(signed, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Subject != user.ID.String() || claims.Issuer != "my-issuer" {
		t.Fatalf("claims = %+v", claims)
	}
	lifetime := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	if lifetime != accessTokenLifetime {
		t.Fatalf("lifetime = %v, want %v", lifetime, accessTokenLifetime)
	}
	if time.Until(claims.ExpiresAt.Time) <= 0 {
		t.Fatal("token already expired")
	}
}

func TestAccessTokenCookies(t *testing.T) {
	rec := httptest.NewRecorder()
	SetAccessTokenCookie(rec, "tok")
	cookie := rec.Result().Cookies()[0]
	if cookie.Name != platform.AccessTokenCookie || cookie.Value != "tok" || cookie.Path != "/" ||
		!cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode ||
		cookie.MaxAge != int(accessTokenLifetime.Seconds()) {
		t.Fatalf("cookie = %+v", cookie)
	}

	rec = httptest.NewRecorder()
	ClearAccessTokenCookie(rec)
	cleared := rec.Result().Cookies()[0]
	if cleared.Name != platform.AccessTokenCookie || cleared.Value != "" || cleared.MaxAge >= 0 ||
		!cleared.HttpOnly || !cleared.Secure || cleared.Path != "/" {
		t.Fatalf("cleared cookie = %+v", cleared)
	}
}
