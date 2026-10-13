package recommendations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"backend/internal/common"
	"backend/internal/observability"
)

type EmbeddingClient struct {
	url, model string
	client     *http.Client
}

type GenerationClient struct {
	url, model string
	client     *http.Client
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

type generationResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewEmbeddingClient() *EmbeddingClient {
	return &EmbeddingClient{url: strings.TrimRight(env("EMBEDDING_URL", "http://llama-embedding:8080"), "/"), model: env("EMBEDDING_MODEL", "bge-small-en-v1.5"), client: &http.Client{Timeout: 45 * time.Second}}
}
func NewGenerationClient() *GenerationClient {
	return &GenerationClient{url: strings.TrimRight(env("GENERATION_URL", "http://llama-generation:8080"), "/"), model: env("GENERATION_MODEL", "Qwen3-0.6B-Instruct"), client: &http.Client{Timeout: 60 * time.Second}}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func (c *EmbeddingClient) Embed(ctx context.Context, input string) (vector []float32, err error) {
	started := time.Now()
	result := "success"
	defer func() {
		if err != nil {
			result = "error"
		}
		observability.ObserveModelRequest("embedding", c.model, "embed", result, started)
	}()

	body := marshalEmbeddingRequest(c.model, input)
	req, err := newInferenceRequest(ctx, c.url, "/v1/embeddings", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var response embeddingResponse
	if err := doJSON(c.client, req, &response); err != nil {
		return nil, fmt.Errorf("%w: embedding request failed: %v", common.ErrInferenceUnavailable, err)
	}
	if len(response.Data) == 0 || len(response.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("%w: embedding endpoint returned no vector", common.ErrInferenceUnavailable)
	}
	return response.Data[0].Embedding, nil
}

func (c *EmbeddingClient) WarmUp(ctx context.Context) error {
	_, err := c.Embed(ctx, "Movie recommendation model warm-up.")
	return err
}

func (c *GenerationClient) Complete(ctx context.Context, system, user string) (string, error) {
	return c.complete(ctx, system, user, 300, true)
}

func (c *GenerationClient) WarmUp(ctx context.Context) error {
	_, err := c.complete(ctx, "Reply with one short word.", "Say ready.", 8, true)
	return err
}

func (c *GenerationClient) complete(ctx context.Context, system, user string, maxTokens int, requireContent bool) (answer string, err error) {
	started := time.Now()
	result := "success"
	defer func() {
		if err != nil {
			result = "error"
		}
		observability.ObserveModelRequest("generation", c.model, "complete", result, started)
	}()

	body := marshalGenerationRequest(c.model, system, user, maxTokens)
	req, err := newInferenceRequest(ctx, c.url, "/v1/chat/completions", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	var response generationResponse
	if err := doJSON(c.client, req, &response); err != nil {
		return "", fmt.Errorf("%w: generation request failed: %v", common.ErrInferenceUnavailable, err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("%w: generation endpoint returned no choices", common.ErrInferenceUnavailable)
	}
	answer = strings.TrimSpace(response.Choices[0].Message.Content)
	if answer == "" && requireContent {
		return "", fmt.Errorf("%w: generation endpoint returned an empty answer", common.ErrInferenceUnavailable)
	}
	return answer, nil
}

func newInferenceRequest(ctx context.Context, baseURL, path string, body []byte) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(body))
}

func marshalEmbeddingRequest(model, input string) []byte {
	body, _ := json.Marshal(map[string]any{"model": model, "input": input})
	return body
}

func marshalGenerationRequest(model, system, user string, maxTokens int) []byte {
	body, _ := json.Marshal(map[string]any{
		"model":                model,
		"temperature":          0.7,
		"max_tokens":           maxTokens,
		"chat_template_kwargs": map[string]bool{"enable_thinking": false},
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	return body
}

type httpStatusError struct {
	host string
	code int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s returned HTTP %d", e.host, e.code)
}

func doJSON(client *http.Client, req *http.Request, target any) error {
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &httpStatusError{host: req.URL.Host, code: response.StatusCode}
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response from %s: %w", req.URL.Host, err)
	}
	return nil
}
