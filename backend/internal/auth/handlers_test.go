package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"backend/internal/platform"

	"github.com/google/uuid"
)

type fakeRefresher struct {
	mu        sync.Mutex
	fresh     []uuid.UUID
	refreshed []uuid.UUID
}

func (f *fakeRefresher) EnsureFresh(_ context.Context, id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fresh = append(f.fresh, id)
}

func (f *fakeRefresher) RequestRefresh(_ context.Context, id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshed = append(f.refreshed, id)
}

func passthrough(next http.Handler) http.Handler { return next }

func newTestMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, passthrough)
	return mux
}

func serve(mux *http.ServeMux, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestLoginAndSignUpRejectInvalidInput(t *testing.T) {
	mux := newTestMux(NewHandler(nil, nil, &fakeRefresher{}))
	tests := []struct{ path, body string }{
		{"/movieAPI/login", ``},
		{"/movieAPI/login", `{`},
		{"/movieAPI/login", `{"userName":"","password":"x"}`},
		{"/movieAPI/login", `{"userName":"a","password":""}`},
		{"/movieAPI/signup", `{`},
		{"/movieAPI/signup", `{"userName":"alice","password":"short"}`},
		{"/movieAPI/signup", `{"userName":" ","password":"longenough"}`},
		{"/movieAPI/signup", `{"userName":"alice","password":"` + strings.Repeat("a", 73) + `"}`},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("POST", tt.path, strings.NewReader(tt.body))
		if rec := serve(mux, req); rec.Code != http.StatusBadRequest {
			t.Errorf("%s %.30q: status = %d, want 400", tt.path, tt.body, rec.Code)
		}
	}
}

func TestSessionHandlerRequiresUser(t *testing.T) {
	mux := newTestMux(NewHandler(nil, nil, &fakeRefresher{}))
	if rec := serve(mux, httptest.NewRequest("GET", "/movieAPI/session", nil)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUsersHandlerEmptySearchReturnsEmptyArray(t *testing.T) {
	mux := newTestMux(NewHandler(&UserService{}, nil, &fakeRefresher{}))
	for _, target := range []string{"/movieAPI/users", "/movieAPI/users?search=", "/movieAPI/users?search=%20%20"} {
		rec := serve(mux, httptest.NewRequest("GET", target, nil))
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Errorf("%s: status = %d body = %q", target, rec.Code, rec.Body.String())
		}
	}
}

func TestSearchUsersValidation(t *testing.T) {
	service := &UserService{}
	users, err := service.SearchUsers(context.Background(), "   ", 5)
	if err != nil || users == nil || len(users) != 0 {
		t.Fatalf("users = %#v, err = %v", users, err)
	}
	if _, err := service.SearchUsers(context.Background(), "bob", 0); err == nil {
		t.Fatal("non-positive limit should fail")
	}
}

func TestHashPassword(t *testing.T) {
	hash, err := hashPassword("password123")
	if err != nil || hash == "" || hash == "password123" {
		t.Fatalf("hash = %q, err = %v", hash, err)
	}
	other, _ := hashPassword("password123")
	if hash == other {
		t.Fatal("hashes must be salted")
	}
	if _, err := hashPassword(strings.Repeat("a", 100)); err == nil {
		t.Fatal("bcrypt should reject passwords over 72 bytes")
	}
}

func TestLogoutClearsCookieAndRefreshesForContextUser(t *testing.T) {
	refresher := &fakeRefresher{}
	mux := newTestMux(NewHandler(nil, nil, refresher))
	id := uuid.New()
	req := httptest.NewRequest("POST", "/movieAPI/logout", nil)
	req = req.WithContext(platform.WithUserID(req.Context(), id))
	rec := serve(mux, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"success":true}` {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("cookies = %+v", cookies)
	}
	if len(refresher.refreshed) != 1 || refresher.refreshed[0] != id {
		t.Fatalf("refreshed = %v", refresher.refreshed)
	}
}

func TestLogoutWithoutIdentityStillSucceeds(t *testing.T) {
	refresher := &fakeRefresher{}
	mux := newTestMux(NewHandler(nil, nil, refresher))
	rec := serve(mux, httptest.NewRequest("POST", "/movieAPI/logout", nil))
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("status = %d cookies = %v", rec.Code, rec.Result().Cookies())
	}
	if len(refresher.refreshed) != 0 {
		t.Fatalf("refreshed = %v", refresher.refreshed)
	}
}

func TestLogoutUsesCookieTokenOnlyWhenValid(t *testing.T) {
	private, public := writeKeyPair(t)
	issuer, err := NewTokenIssuer(private, platform.TokenIssuerName)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := platform.NewTokenVerifier(public, platform.TokenIssuerName)
	if err != nil {
		t.Fatal(err)
	}
	user := User{ID: uuid.New()}
	token, err := issuer.Issue(user)
	if err != nil {
		t.Fatal(err)
	}

	refresher := &fakeRefresher{}
	mux := newTestMux(NewHandler(nil, verifier, refresher))

	valid := httptest.NewRequest("POST", "/movieAPI/logout", nil)
	valid.AddCookie(&http.Cookie{Name: platform.AccessTokenCookie, Value: token})
	if rec := serve(mux, valid); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(refresher.refreshed) != 1 || refresher.refreshed[0] != user.ID {
		t.Fatalf("refreshed = %v", refresher.refreshed)
	}

	invalid := httptest.NewRequest("POST", "/movieAPI/logout", nil)
	invalid.AddCookie(&http.Cookie{Name: platform.AccessTokenCookie, Value: "garbage"})
	rec := serve(mux, invalid)
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(refresher.refreshed) != 1 {
		t.Fatalf("invalid token must not trigger refresh: %v", refresher.refreshed)
	}
}

func TestRoutesRejectWrongMethods(t *testing.T) {
	mux := newTestMux(NewHandler(nil, nil, &fakeRefresher{}))
	for _, tt := range [][2]string{{"GET", "/movieAPI/login"}, {"GET", "/movieAPI/signup"}, {"GET", "/movieAPI/logout"}, {"POST", "/movieAPI/session"}} {
		if rec := serve(mux, httptest.NewRequest(tt[0], tt[1], nil)); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", tt[0], tt[1], rec.Code)
		}
	}
}
