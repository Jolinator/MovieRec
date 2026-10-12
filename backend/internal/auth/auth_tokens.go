package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"backend/internal/platform"
	"github.com/golang-jwt/jwt/v5"
)

const accessTokenLifetime = 15 * time.Minute

type TokenIssuer struct {
	privateKey *rsa.PrivateKey
	issuer     string
}

func SetupTokenIssuer() (*TokenIssuer, error) {
	keyPath := os.Getenv("JWT_PRIVATE_KEY_PATH")
	if keyPath == "" {
		keyPath = filepath.Join("..", "secrets", "jwt-private.pem")
	}

	tokenIssuer, err := NewTokenIssuer(keyPath, platform.TokenIssuerName)
	if err != nil {
		return nil, fmt.Errorf("initialize token issuer: %w", err)
	}

	return tokenIssuer, nil
}

func NewTokenIssuer(keyPath, issuer string) (*TokenIssuer, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read JWT private key: %w", err)
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse JWT private key: %w", err)
	}

	return &TokenIssuer{privateKey: privateKey, issuer: issuer}, nil
}

func (i *TokenIssuer) Issue(user User) (string, error) {
	if i == nil || i.privateKey == nil {
		return "", fmt.Errorf("token issuer is not initialized")
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		Issuer:    i.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenLifetime)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(i.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	return signedToken, nil
}

func SetAccessTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     platform.AccessTokenCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(accessTokenLifetime.Seconds()),
	})
}

func ClearAccessTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     platform.AccessTokenCookie,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
