package httpapi

import (
	"net/http"

	"backend/auth"
)

func AuthenticationMiddleware(verifier *auth.TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("access_token")
			if err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			userID, err := verifier.Verify(cookie.Value)
			if err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithUserID(r.Context(), userID)))
		})
	}
}
