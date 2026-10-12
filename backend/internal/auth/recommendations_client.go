package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"backend/internal/common"
	"backend/internal/platform"

	"github.com/google/uuid"
)

const recommendationsCallTimeout = 5 * time.Second

// RecommendationsClient asks the recommendations service, over its internal HTTP API, to refresh a user's
// recommendations. Calls are best-effort and asynchronous so login and logout never depend on that service.
type RecommendationsClient struct {
	baseURL string
	client  *http.Client
}

func NewRecommendationsClient() *RecommendationsClient {
	defaultURL := "http://localhost:" + platform.Env("RECOMMENDATIONS_INTERNAL_PORT", "8085")
	return &RecommendationsClient{
		baseURL: strings.TrimRight(platform.Env("RECOMMENDATIONS_INTERNAL_URL", defaultURL), "/"),
		client:  &http.Client{Timeout: recommendationsCallTimeout},
	}
}

func (c *RecommendationsClient) EnsureFresh(ctx context.Context, userID uuid.UUID) {
	c.post(ctx, userID, common.InternalEnsureFreshAction)
}

func (c *RecommendationsClient) RequestRefresh(ctx context.Context, userID uuid.UUID) {
	c.post(ctx, userID, common.InternalRequestRefreshAction)
}

func (c *RecommendationsClient) post(ctx context.Context, userID uuid.UUID, action string) {
	// Detach from the caller's request, which ends as soon as the handler responds.
	ctx = context.WithoutCancel(ctx)
	go func() {
		if err := c.send(ctx, userID, action); err != nil {
			log.Printf("recommendations %s for %s: %v", action, userID, err)
		}
	}()
}

func (c *RecommendationsClient) send(ctx context.Context, userID uuid.UUID, action string) error {
	ctx, cancel := context.WithTimeout(ctx, recommendationsCallTimeout)
	defer cancel()

	url := c.baseURL + common.InternalRecommendationsUsersPath + userID.String() + "/" + action
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	return nil
}
