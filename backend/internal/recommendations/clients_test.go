package recommendations

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/common"
)

func TestEnv(t *testing.T) {
	t.Setenv("REC_TEST_ENV", "")
	if env("REC_TEST_ENV", "fb") != "fb" {
		t.Error("empty should use fallback")
	}
	t.Setenv("REC_TEST_ENV", "v")
	if env("REC_TEST_ENV", "fb") != "v" {
		t.Error("set value should win")
	}
}

func TestNewClientsReadEnvironment(t *testing.T) {
	t.Setenv("EMBEDDING_URL", "http://emb:1/")
	t.Setenv("EMBEDDING_MODEL", "em")
	t.Setenv("GENERATION_URL", "http://gen:2///")
	t.Setenv("GENERATION_MODEL", "gm")
	e, g := NewEmbeddingClient(), NewGenerationClient()
	if e.url != "http://emb:1" || e.model != "em" || e.client.Timeout <= 0 {
		t.Fatalf("embedding client = %+v", e)
	}
	if g.url != "http://gen:2" || g.model != "gm" || g.client.Timeout <= 0 {
		t.Fatalf("generation client = %+v", g)
	}
}

func TestNewClientsDefaults(t *testing.T) {
	for _, name := range []string{"EMBEDDING_URL", "EMBEDDING_MODEL", "GENERATION_URL", "GENERATION_MODEL"} {
		t.Setenv(name, "")
	}
	if e := NewEmbeddingClient(); e.url == "" || e.model == "" {
		t.Errorf("embedding defaults empty: %+v", e)
	}
	if g := NewGenerationClient(); g.url == "" || g.model == "" {
		t.Errorf("generation defaults empty: %+v", g)
	}
}

func embeddingClient(t *testing.T, handler http.HandlerFunc) *EmbeddingClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &EmbeddingClient{url: server.URL, model: "m", client: server.Client()}
}

func generationClient(t *testing.T, handler http.HandlerFunc) *GenerationClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &GenerationClient{url: server.URL, model: "m", client: server.Client()}
}

func TestEmbedSuccessAndRequestShape(t *testing.T) {
	client := embeddingClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/embeddings" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("bad request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "m" || body["input"] != "hello" {
			t.Errorf("body = %v", body)
		}
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2]}]}`))
	})
	got, err := client.Embed(context.Background(), "hello")
	if err != nil || len(got) != 2 || got[0] != 0.1 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestEmbedErrorsWrapInferenceUnavailable(t *testing.T) {
	tests := map[string]string{
		"no data":      `{"data":[]}`,
		"empty vector": `{"data":[{"embedding":[]}]}`,
		"bad json":     `{`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			client := embeddingClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
			if _, err := client.Embed(context.Background(), "x"); !errors.Is(err, common.ErrInferenceUnavailable) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	client := embeddingClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	if _, err := client.Embed(context.Background(), "x"); !errors.Is(err, common.ErrInferenceUnavailable) {
		t.Fatalf("status error = %v", err)
	}
}

func TestEmbedTransportFailureAndBadURL(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	client := &EmbeddingClient{url: url, model: "m", client: http.DefaultClient}
	if _, err := client.Embed(context.Background(), "x"); !errors.Is(err, common.ErrInferenceUnavailable) {
		t.Fatalf("err = %v", err)
	}
	client = &EmbeddingClient{url: "http://bad host", model: "m", client: http.DefaultClient}
	if _, err := client.Embed(context.Background(), "x"); err == nil {
		t.Fatal("expected request creation error")
	}
}

func TestEmbeddingWarmUp(t *testing.T) {
	client := embeddingClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"embedding":[1]}]}`))
	})
	if err := client.WarmUp(context.Background()); err != nil {
		t.Fatal(err)
	}
	failing := embeddingClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	if err := failing.WarmUp(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCompleteSuccessTrimsAnswerAndSendsMessages(t *testing.T) {
	client := generationClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body struct {
			Model     string              `json:"model"`
			MaxTokens int                 `json:"max_tokens"`
			Messages  []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "m" || body.MaxTokens != 300 || len(body.Messages) != 2 ||
			body.Messages[0]["role"] != "system" || body.Messages[0]["content"] != "sys" ||
			body.Messages[1]["role"] != "user" || body.Messages[1]["content"] != "usr" {
			t.Errorf("body = %+v", body)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  answer \n"}}]}`))
	})
	got, err := client.Complete(context.Background(), "sys", "usr")
	if err != nil || got != "answer" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCompleteErrors(t *testing.T) {
	tests := map[string]string{
		"no choices":   `{"choices":[]}`,
		"empty answer": `{"choices":[{"message":{"content":"   "}}]}`,
		"bad json":     `nope`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			client := generationClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
			if _, err := client.Complete(context.Background(), "s", "u"); !errors.Is(err, common.ErrInferenceUnavailable) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	client := generationClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	if _, err := client.Complete(context.Background(), "s", "u"); !errors.Is(err, common.ErrInferenceUnavailable) {
		t.Fatalf("status error = %v", err)
	}
}

func TestCompleteAllowsEmptyAnswerWhenNotRequired(t *testing.T) {
	client := generationClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	})
	got, err := client.complete(context.Background(), "s", "u", 8, false)
	if err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestGenerationWarmUp(t *testing.T) {
	client := generationClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.MaxTokens != 8 {
			t.Errorf("max_tokens = %d", body.MaxTokens)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ready"}}]}`))
	})
	if err := client.WarmUp(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPStatusErrorMessage(t *testing.T) {
	err := &httpStatusError{host: "svc:80", code: 502}
	if got := err.Error(); !strings.Contains(got, "svc:80") || !strings.Contains(got, "502") {
		t.Fatalf("message = %q", got)
	}
}

func TestDoJSONAcceptsAny2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"a":1}`))
	}))
	defer server.Close()
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	var target map[string]int
	if err := doJSON(server.Client(), req, &target); err != nil || target["a"] != 1 {
		t.Fatalf("target = %v, err = %v", target, err)
	}
}
