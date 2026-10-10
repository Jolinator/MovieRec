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

	"backend/common"
)

type EmbeddingClient struct {
	url, model string
	client     *http.Client
}

type GenerationClient struct {
	url, model string
	client     *http.Client
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

func (c *EmbeddingClient) Embed(ctx context.Context, input string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{"model": c.model, "input": input})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := doJSON(c.client, req, &response); err != nil {
		return nil, fmt.Errorf("%w: embedding request failed: %v", common.ErrInferenceUnavailable, err)
	}
	if len(response.Data) == 0 || len(response.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("%w: embedding endpoint returned no vector", common.ErrInferenceUnavailable)
	}
	return response.Data[0].Embedding, nil
}

func (c *GenerationClient) Complete(ctx context.Context, system, user string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":                c.model,
		"temperature":          0.7,
		"max_tokens":           300,
		"chat_template_kwargs": map[string]bool{"enable_thinking": false},
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := doJSON(c.client, req, &response); err != nil {
		return "", fmt.Errorf("%w: generation request failed: %v", common.ErrInferenceUnavailable, err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("%w: generation endpoint returned no choices", common.ErrInferenceUnavailable)
	}
	answer := strings.TrimSpace(response.Choices[0].Message.Content)
	if answer == "" {
		return "", fmt.Errorf("%w: generation endpoint returned an empty answer", common.ErrInferenceUnavailable)
	}
	return answer, nil
}

func doJSON(client *http.Client, req *http.Request, target any) error {
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("llama.cpp returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode llama.cpp response: %w", err)
	}
	return nil
}
