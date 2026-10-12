package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestAuthenticationMiddleware(t *testing.T) {
	key := newKey(t)
	id := uuid.New()
	valid := signToken(t, key, jwt.SigningMethodRS256, validClaims(id.String()))
	invalid := signToken(t, newKey(t), jwt.SigningMethodRS256, validClaims(id.String()))

	tests := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
		wantCalled bool
	}{
		{"no cookie", nil, http.StatusUnauthorized, false},
		{"bad token", &http.Cookie{Name: AccessTokenCookie, Value: invalid}, http.StatusUnauthorized, false},
		{"wrong cookie name", &http.Cookie{Name: "other", Value: valid}, http.StatusUnauthorized, false},
		{"valid token", &http.Cookie{Name: AccessTokenCookie, Value: valid}, http.StatusOK, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			handler := AuthenticationMiddleware(newVerifier(t, key))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				got, ok := UserIDFromContext(r.Context())
				if !ok || got != id {
					t.Errorf("context user = %s, %v", got, ok)
				}
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus || called != tt.wantCalled {
				t.Fatalf("status=%d called=%v, want %d/%v", rec.Code, called, tt.wantStatus, tt.wantCalled)
			}
		})
	}
}
