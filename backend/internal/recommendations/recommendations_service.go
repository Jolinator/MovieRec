package recommendations

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"backend/internal/common"
	"backend/internal/observability"
	"github.com/google/uuid"
)

type Service struct {
	catalog    MovieCatalog
	embeddings *EmbeddingRepository
	embedding  *EmbeddingClient
	generation *GenerationClient
	lists      *ListRepository
	cache      *RecommendationCacheRepository
	jobs       sync.Map
	ready      atomic.Bool
}

const chatSystemPrompt = `You are a friendly movie recommendation assistant.
Use only the movies in the supplied JSON and choose exactly three.
Respond using exactly three numbered entries and no introduction or conclusion.
Use this format for every entry:
1. Title: exact movie title
   Reason: one concise, specific sentence explaining why it matches the user's request, based on the supplied synopsis or tone
   [ID:123]
Do not use Markdown emphasis, headings, genre labels, database labels, or any other fields.
Never invent an identifier or recommend a movie outside the supplied JSON.`

func NewService(catalog MovieCatalog, embeddings *EmbeddingRepository, lists *ListRepository, cache *RecommendationCacheRepository) *Service {
	embedding := NewEmbeddingClient()
	generation := NewGenerationClient()
	observability.ObserveModelReady("embedding", embedding.model, false)
	observability.ObserveModelReady("generation", generation.model, false)
	return &Service{catalog: catalog, embeddings: embeddings, embedding: embedding, generation: generation, lists: lists, cache: cache}
}
func (s *Service) Initialize(ctx context.Context) error {
	s.ready.Store(false)
	if err := s.syncEmbeddings(ctx); err != nil {
		return fmt.Errorf("embed movie synopses: %w", err)
	}
	if err := s.initializeEmbeddingModel(ctx); err != nil {
		return err
	}
	if err := s.initializeGenerationModel(ctx); err != nil {
		return err
	}
	s.ready.Store(true)
	return nil
}

func (s *Service) initializeEmbeddingModel(ctx context.Context) error {
	if err := s.embedding.WarmUp(ctx); err != nil {
		return fmt.Errorf("embedding model warm-up failed: %w", err)
	}
	observability.ObserveModelReady("embedding", s.embedding.model, true)
	return nil
}

func (s *Service) initializeGenerationModel(ctx context.Context) error {
	if err := s.generation.WarmUp(ctx); err != nil {
		return fmt.Errorf("generation model warm-up failed: %w", err)
	}
	observability.ObserveModelReady("generation", s.generation.model, true)
	return nil
}

func (s *Service) InitializeWithRetry(ctx context.Context, attempts int, delay time.Duration) error {
	if attempts < 1 {
		return fmt.Errorf("embedding initialization attempts must be at least 1")
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := s.Initialize(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == attempts {
			break
		}
		if err := waitForRetry(ctx, delay); err != nil {
			return err
		}
	}
	return fmt.Errorf("recommendation initialization failed after %d attempts: %w", attempts, lastErr)
}

// InitializeUntilReady keeps retrying until initialization succeeds or the service context ends.
func (s *Service) InitializeUntilReady(ctx context.Context, delay time.Duration) error {
	for attempt := 1; ; attempt++ {
		if err := s.Initialize(ctx); err == nil {
			return nil
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("recommendation initialization attempt %d failed: %v; retrying in %s", attempt, err, delay)
		}
		if err := waitForRetry(ctx, delay); err != nil {
			return err
		}
	}
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (s *Service) Ready() bool { return s.ready.Load() }

func (s *Service) RefreshForUser(ctx context.Context, userID uuid.UUID) {
	started := time.Now()
	result := "ready"
	defer func() {
		observability.ObserveRecommendationRequest("refresh", result, started)
	}()

	if !s.startRefreshJob(userID) {
		result = "skipped"
		return
	}
	defer s.jobs.Delete(userID)
	result, movieIDs := s.generateRecommendations(ctx, userID)
	if err := s.cache.Save(ctx, userID, result, movieIDs); err != nil {
		result = "failed"
		log.Printf("background recommendations: save for %s: %v", userID, err)
	}
}

func (s *Service) startRefreshJob(userID uuid.UUID) bool {
	_, loaded := s.jobs.LoadOrStore(userID, struct{}{})
	return !loaded
}

func (s *Service) generateRecommendations(ctx context.Context, userID uuid.UUID) (string, []int) {
	ids, err := s.lists.ListByUserID(ctx, userID)
	if err != nil {
		return s.refreshFailure(userID, "load list", err)
	}
	if len(ids) == 0 {
		return "empty", nil
	}

	profile, err := s.embeddings.Profile(ctx, ids)
	if err != nil {
		return s.refreshFailure(userID, "build profile", err)
	}
	if len(profile) == 0 {
		return "empty", nil
	}

	resultIDs, err := s.embeddings.SearchSimilarExcluding(ctx, profile, ids, 3)
	if err != nil {
		return s.refreshFailure(userID, "search", err)
	}
	if len(resultIDs) == 0 {
		return "empty", resultIDs
	}
	return "ready", resultIDs
}

func (s *Service) refreshFailure(userID uuid.UUID, operation string, err error) (string, []int) {
	log.Printf("background recommendations: %s for %s: %v", operation, userID, err)
	return "failed", nil
}

func (s *Service) RequestRefresh(ctx context.Context, userID uuid.UUID) {
	if err := s.cache.Request(ctx, userID); err != nil {
		log.Printf("background recommendations: request refresh for %s: %v", userID, err)
		return
	}
	go s.RefreshForUser(context.Background(), userID)
}

func (s *Service) EnsureFresh(ctx context.Context, userID uuid.UUID) {
	cache, err := s.cache.Get(ctx, userID)
	if err != nil {
		log.Printf("background recommendations: inspect cache for %s: %v", userID, err)
		return
	}
	if cacheNeedsRefresh(cache) {
		s.RequestRefresh(ctx, userID)
	}
}

func (s *Service) Recommendations(ctx context.Context, userID uuid.UUID) (response PersonalizedRecommendationsResponse, err error) {
	started := time.Now()
	result := "ready"
	defer func() {
		if err != nil {
			result = "error"
		} else if response.Status != "" {
			result = response.Status
		}
		observability.ObserveRecommendationRequest("recommendations", result, started)
	}()

	cache, err := s.cache.Get(ctx, userID)
	if err != nil {
		return PersonalizedRecommendationsResponse{}, err
	}
	if cacheNeedsRefresh(cache) {
		s.RequestRefresh(context.Background(), userID)
		return preparingRecommendations(), nil
	}
	movies, err := s.cache.LoadMovies(ctx, userID, s.catalog)
	if err != nil {
		return PersonalizedRecommendationsResponse{}, err
	}
	return PersonalizedRecommendationsResponse{Status: cache.Status, Movies: movies}, nil
}

func cacheNeedsRefresh(cache recommendationCache) bool {
	return cache.Status == "missing" ||
		(cache.RequestedAt != nil && (cache.GeneratedAt == nil || cache.GeneratedAt.Before(*cache.RequestedAt)))
}

func preparingRecommendations() PersonalizedRecommendationsResponse {
	return PersonalizedRecommendationsResponse{Status: "preparing"}
}

func (s *Service) Chat(ctx context.Context, message string) (response ChatResponse, err error) {
	started := time.Now()
	result := "success"
	defer func() {
		if err != nil {
			result = "error"
		}
		observability.ObserveRecommendationRequest("chat", result, started)
	}()

	if !s.Ready() {
		return ChatResponse{}, common.ErrRecommendationsUnavailable
	}
	candidates, err := s.chatCandidates(ctx, message)
	if err != nil {
		return ChatResponse{}, err
	}
	answer, err := s.generateChatAnswer(ctx, message, candidates)
	if err != nil {
		return ChatResponse{}, err
	}
	return buildChatResponse(answer, candidates), nil
}

func (s *Service) chatCandidates(ctx context.Context, message string) ([]common.MovieResponse, error) {
	vector, err := s.embedding.Embed(ctx, message)
	if err != nil {
		return nil, err
	}
	candidateIDs, err := s.embeddings.SearchSimilar(ctx, vector, 8)
	if err != nil {
		return nil, err
	}
	return s.catalog.ListByTMDBIDs(ctx, candidateIDs)
}

func (s *Service) generateChatAnswer(ctx context.Context, message string, candidates []common.MovieResponse) (string, error) {
	payload, _ := json.Marshal(candidates)
	userPrompt := fmt.Sprintf("User request: %s\nMovie database candidates: %s", message, payload)
	return s.generation.Complete(ctx, chatSystemPrompt, userPrompt)
}

func buildChatResponse(answer string, candidates []common.MovieResponse) ChatResponse {
	movies, message := normalizeRecommendations(answer, candidates)
	return ChatResponse{Message: message, Movies: movies}
}
