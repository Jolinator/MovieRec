package platform

import (
	"net/http"
)

func AuthenticationMiddleware(verifier *TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(AccessTokenCookie)
			if err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			userID, err := verifier.Verify(cookie.Value)
			if err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), userID)))
		})
	}
}
