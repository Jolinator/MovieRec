package recommendations

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/common"
)

type fakeCatalog struct {
	ready    bool
	readyErr error
	listErr  error
	byIDs    []common.MovieResponse
	pages    [][]common.MovieSynopsis
}

func (c *fakeCatalog) Ready(context.Context) (bool, error) { return c.ready, c.readyErr }

func (c *fakeCatalog) ListByTMDBIDs(context.Context, []int) ([]common.MovieResponse, error) {
	return c.byIDs, c.listErr
}

func (c *fakeCatalog) ListSynopses(context.Context, int, int) ([]common.MovieSynopsis, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	if len(c.pages) == 0 {
		return nil, nil
	}
	page := c.pages[0]
	c.pages = c.pages[1:]
	return page, nil
}

func inferenceServer(t *testing.T, embeddingOK, generationOK bool) (*EmbeddingClient, *GenerationClient) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/embeddings" && embeddingOK:
			_, _ = w.Write([]byte(`{"data":[{"embedding":[1]}]}`))
		case r.URL.Path == "/v1/chat/completions" && generationOK:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ready"}}]}`))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(server.Close)
	return &EmbeddingClient{url: server.URL, model: "m", client: server.Client()},
		&GenerationClient{url: server.URL, model: "m", client: server.Client()}
}

func TestRequireCatalogReady(t *testing.T) {
	ctx := context.Background()
	if err := (&Service{catalog: &fakeCatalog{ready: true}}).requireCatalogReady(ctx); err != nil {
		t.Fatalf("ready catalogue: %v", err)
	}
	if err := (&Service{catalog: &fakeCatalog{}}).requireCatalogReady(ctx); !errors.Is(err, errCatalogNotReady) {
		t.Fatalf("not ready: %v", err)
	}
	boom := errors.New("boom")
	if err := (&Service{catalog: &fakeCatalog{readyErr: boom}}).requireCatalogReady(ctx); !errors.Is(err, boom) {
		t.Fatalf("catalogue error: %v", err)
	}
}

func TestSyncEmbeddingsWithEmptyCatalogue(t *testing.T) {
	service := &Service{catalog: &fakeCatalog{ready: true}}
	if err := service.syncEmbeddings(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSyncEmbeddingsPropagatesCatalogueErrors(t *testing.T) {
	boom := errors.New("boom")
	if err := (&Service{catalog: &fakeCatalog{}}).syncEmbeddings(context.Background()); !errors.Is(err, errCatalogNotReady) {
		t.Fatalf("not ready: %v", err)
	}
	if err := (&Service{catalog: &fakeCatalog{ready: true, listErr: boom}}).syncEmbeddings(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("list error: %v", err)
	}
}

func TestInitializeBecomesReady(t *testing.T) {
	embedding, generation := inferenceServer(t, true, true)
	service := &Service{catalog: &fakeCatalog{ready: true}, embedding: embedding, generation: generation}
	if service.Ready() {
		t.Fatal("must not be ready before Initialize")
	}
	if err := service.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !service.Ready() {
		t.Fatal("expected ready after Initialize")
	}
}

func TestInitializeFailuresLeaveServiceNotReady(t *testing.T) {
	tests := map[string]func(t *testing.T) *Service{
		"catalogue not ready": func(t *testing.T) *Service {
			e, g := inferenceServer(t, true, true)
			return &Service{catalog: &fakeCatalog{}, embedding: e, generation: g}
		},
		"embedding down": func(t *testing.T) *Service {
			e, g := inferenceServer(t, false, true)
			return &Service{catalog: &fakeCatalog{ready: true}, embedding: e, generation: g}
		},
		"generation down": func(t *testing.T) *Service {
			e, g := inferenceServer(t, true, false)
			return &Service{catalog: &fakeCatalog{ready: true}, embedding: e, generation: g}
		},
	}
	for name, build := range tests {
		t.Run(name, func(t *testing.T) {
			service := build(t)
			service.ready.Store(true) // Initialize must reset readiness
			if err := service.Initialize(context.Background()); err == nil {
				t.Fatal("expected error")
			}
			if service.Ready() {
				t.Fatal("service must not be ready after a failed Initialize")
			}
		})
	}
}

func TestInitializeWithRetry(t *testing.T) {
	embedding, generation := inferenceServer(t, true, true)
	service := &Service{catalog: &fakeCatalog{ready: true}, embedding: embedding, generation: generation}

	if err := service.InitializeWithRetry(context.Background(), 0, 0); err == nil {
		t.Fatal("attempts < 1 should fail")
	}
	if err := service.InitializeWithRetry(context.Background(), 2, time.Millisecond); err != nil || !service.Ready() {
		t.Fatalf("err = %v, ready = %v", err, service.Ready())
	}
}

func TestInitializeWithRetryGivesUp(t *testing.T) {
	embedding, generation := inferenceServer(t, true, true)
	service := &Service{catalog: &fakeCatalog{}, embedding: embedding, generation: generation}
	err := service.InitializeWithRetry(context.Background(), 3, time.Millisecond)
	if !errors.Is(err, errCatalogNotReady) {
		t.Fatalf("err = %v", err)
	}
}

func TestInitializeWithRetryStopsWhenContextCancelled(t *testing.T) {
	service := &Service{catalog: &fakeCatalog{}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := service.InitializeWithRetry(ctx, 100, time.Minute)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("retry did not stop on cancellation")
	}
}

func TestChatRequiresReadiness(t *testing.T) {
	_, err := (&Service{}).Chat(context.Background(), "hi")
	if !errors.Is(err, common.ErrRecommendationsUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestChatReportsEmbeddingFailure(t *testing.T) {
	embedding, generation := inferenceServer(t, false, true)
	service := &Service{embedding: embedding, generation: generation}
	service.ready.Store(true)
	if _, err := service.Chat(context.Background(), "hi"); !errors.Is(err, common.ErrInferenceUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestListServiceAddFailsWhenCatalogueFails(t *testing.T) {
	service := NewListService(nil, &fakeCatalog{listErr: common.ErrMovieNotFound})
	if _, err := service.Add(context.Background(), [16]byte{}, 5); !errors.Is(err, common.ErrMovieNotFound) {
		t.Fatalf("err = %v", err)
	}
}
