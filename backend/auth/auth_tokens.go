package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"backend/common"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	accessTokenLifetime = 15 * time.Minute
	accessTokenCookie   = "access_token"
)

type TokenIssuer struct {
	privateKey *rsa.PrivateKey
	issuer     string
}

type TokenVerifier struct {
	publicKey *rsa.PublicKey
	issuer    string
}

func SetupTokenIssuer() (*TokenIssuer, error) {
	keyPath := os.Getenv("JWT_PRIVATE_KEY_PATH")
	if keyPath == "" {
		keyPath = filepath.Join("..", "secrets", "jwt-private.pem")
	}

	tokenIssuer, err := NewTokenIssuer(keyPath, "movierec")
	if err != nil {
		return nil, fmt.Errorf("initialize token issuer: %w", err)
	}

	return tokenIssuer, nil
}

func SetupTokenVerifier() (*TokenVerifier, error) {
	keyPath := os.Getenv("JWT_PUBLIC_KEY_PATH")
	if keyPath == "" {
		keyPath = filepath.Join("..", "secrets", "jwt-public.pem")
	}

	tokenVerifier, err := NewTokenVerifier(keyPath, "movierec")
	if err != nil {
		return nil, fmt.Errorf("initialize token verifier: %w", err)
	}

	return tokenVerifier, nil
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

func NewTokenVerifier(keyPath, issuer string) (*TokenVerifier, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read JWT public key: %w", err)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse JWT public key: %w", err)
	}

	return &TokenVerifier{publicKey: publicKey, issuer: issuer}, nil
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

func (v *TokenVerifier) Verify(tokenString string) (uuid.UUID, error) {
	if v == nil || v.publicKey == nil {
		return uuid.Nil, fmt.Errorf("token verifier is not initialized")
	}

	var claims jwt.RegisteredClaims
	token, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (any, error) {
			return v.publicKey, nil
		},
		jwt.WithIssuer(v.issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %v", common.ErrInvalidToken, err)
	}
	if !token.Valid {
		return uuid.Nil, common.ErrInvalidToken
	}
	if claims.ExpiresAt == nil {
		return uuid.Nil, fmt.Errorf("%w: missing expiration claim", common.ErrInvalidToken)
	}
	if claims.Issuer == "" {
		return uuid.Nil, fmt.Errorf("%w: missing issuer claim", common.ErrInvalidToken)
	}
	if claims.Subject == "" {
		return uuid.Nil, fmt.Errorf("%w: missing subject claim", common.ErrInvalidToken)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid subject: %v", common.ErrInvalidToken, err)
	}

	return userID, nil
}

func SetAccessTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookie,
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
		Name:     accessTokenCookie,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
