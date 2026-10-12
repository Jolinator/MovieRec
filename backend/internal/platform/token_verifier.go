package platform

import (
	"crypto/rsa"
	"fmt"
	"os"
	"path/filepath"

	"backend/internal/common"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenIssuerName   = "movierec"
	AccessTokenCookie = "access_token"
)

type TokenVerifier struct {
	publicKey *rsa.PublicKey
	issuer    string
}

func SetupTokenVerifier() (*TokenVerifier, error) {
	keyPath := os.Getenv("JWT_PUBLIC_KEY_PATH")
	if keyPath == "" {
		keyPath = filepath.Join("..", "secrets", "jwt-public.pem")
	}

	tokenVerifier, err := NewTokenVerifier(keyPath, TokenIssuerName)
	if err != nil {
		return nil, fmt.Errorf("initialize token verifier: %w", err)
	}

	return tokenVerifier, nil
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
