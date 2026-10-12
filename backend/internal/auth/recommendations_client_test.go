package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"backend/internal/common"

	"github.com/google/uuid"
)

func TestRecommendationsClientCallsInternalEndpoints(t *testing.T) {
	userID := uuid.New()
	var mutex sync.Mutex
	var seen []string
	called := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mutex.Unlock()
		w.WriteHeader(http.StatusNoContent)
		called <- struct{}{}
	}))
	defer server.Close()
	t.Setenv("RECOMMENDATIONS_INTERNAL_URL", server.URL)

	client := NewRecommendationsClient()
	// A cancelled caller context must not abort the call: the handler's request is over by then.
	ctx, cancel := context.WithCancel(context.Background())
	client.EnsureFresh(ctx, userID)
	client.RequestRefresh(ctx, userID)
	cancel()

	for range 2 {
		select {
		case <-called:
		case <-time.After(5 * time.Second):
			t.Fatal("recommendations service was not called")
		}
	}
	mutex.Lock()
	defer mutex.Unlock()
	base := "POST " + common.InternalRecommendationsUsersPath + userID.String() + "/"
	want := map[string]bool{base + common.InternalEnsureFreshAction: true, base + common.InternalRequestRefreshAction: true}
	for _, call := range seen {
		if !want[call] {
			t.Fatalf("unexpected call %q", call)
		}
	}
}

func TestRecommendationsClientToleratesUnavailableService(t *testing.T) {
	server := httptest.NewServer(nil)
	t.Setenv("RECOMMENDATIONS_INTERNAL_URL", server.URL)
	server.Close()

	// Must neither panic nor block the caller.
	done := make(chan struct{})
	go func() {
		client := NewRecommendationsClient()
		client.EnsureFresh(context.Background(), uuid.New())
		client.RequestRefresh(context.Background(), uuid.New())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("client blocked on an unavailable service")
	}
}
