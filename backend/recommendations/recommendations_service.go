package recommendations

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"backend/common"
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

func NewService(catalog MovieCatalog, embeddings *EmbeddingRepository, lists *ListRepository, cache *RecommendationCacheRepository) *Service {
	return &Service{catalog: catalog, embeddings: embeddings, embedding: NewEmbeddingClient(), generation: NewGenerationClient(), lists: lists, cache: cache}
}
func (s *Service) Initialize(ctx context.Context) error {
	s.ready.Store(false)
	if err := s.syncEmbeddings(ctx); err != nil {
		return fmt.Errorf("embed movie synopses: %w", err)
	}
	if err := s.embedding.WarmUp(ctx); err != nil {
		return fmt.Errorf("embedding model warm-up failed: %w", err)
	}
	if err := s.generation.WarmUp(ctx); err != nil {
		return fmt.Errorf("generation model warm-up failed: %w", err)
	}
	s.ready.Store(true)
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
		if attempt < attempts {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("recommendation initialization failed after %d attempts: %w", attempts, lastErr)
}
func (s *Service) Ready() bool { return s.ready.Load() }

func (s *Service) RefreshForUser(ctx context.Context, userID uuid.UUID) {
	if _, loaded := s.jobs.LoadOrStore(userID, struct{}{}); loaded {
		return
	}
	defer s.jobs.Delete(userID)
	ids, err := s.lists.ListByUserID(ctx, userID)
	if err != nil {
		log.Printf("background recommendations: load list for %s: %v", userID, err)
		_ = s.cache.Save(ctx, userID, "failed", nil)
		return
	}
	if len(ids) == 0 {
		_ = s.cache.Save(ctx, userID, "empty", nil)
		return
	}
	profile, err := s.embeddings.Profile(ctx, ids)
	if err != nil {
		log.Printf("background recommendations: build profile for %s: %v", userID, err)
		_ = s.cache.Save(ctx, userID, "failed", nil)
		return
	}
	if len(profile) == 0 {
		_ = s.cache.Save(ctx, userID, "empty", nil)
		return
	}
	resultIDs, err := s.embeddings.SearchSimilarExcluding(ctx, profile, ids, 3)
	if err != nil {
		log.Printf("background recommendations: search for %s: %v", userID, err)
		_ = s.cache.Save(ctx, userID, "failed", nil)
		return
	}
	status := "ready"
	if len(resultIDs) == 0 {
		status = "empty"
	}
	if err := s.cache.Save(ctx, userID, status, resultIDs); err != nil {
		log.Printf("background recommendations: save for %s: %v", userID, err)
	}
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
	if cache.Status == "missing" ||
		(cache.RequestedAt != nil && (cache.GeneratedAt == nil || cache.GeneratedAt.Before(*cache.RequestedAt))) {
		s.RequestRefresh(ctx, userID)
	}
}

func (s *Service) Recommendations(ctx context.Context, userID uuid.UUID) (common.PersonalizedRecommendationsResponse, error) {
	cache, err := s.cache.Get(ctx, userID)
	if err != nil {
		return common.PersonalizedRecommendationsResponse{}, err
	}
	if cache.Status == "missing" || (cache.RequestedAt != nil && (cache.GeneratedAt == nil || cache.GeneratedAt.Before(*cache.RequestedAt))) {
		s.RequestRefresh(context.Background(), userID)
		return common.PersonalizedRecommendationsResponse{Status: "preparing"}, nil
	}
	movies, err := s.cache.LoadMovies(ctx, userID, s.catalog)
	if err != nil {
		return common.PersonalizedRecommendationsResponse{}, err
	}
	return common.PersonalizedRecommendationsResponse{Status: cache.Status, Movies: movies}, nil
}
func (s *Service) Chat(ctx context.Context, message string) (common.ChatResponse, error) {
	if !s.Ready() {
		return common.ChatResponse{}, common.ErrRecommendationsUnavailable
	}
	vector, err := s.embedding.Embed(ctx, message)
	if err != nil {
		return common.ChatResponse{}, err
	}
	candidateIDs, err := s.embeddings.SearchSimilar(ctx, vector, 8)
	if err != nil {
		return common.ChatResponse{}, err
	}
	candidates, err := s.catalog.ListByTMDBIDs(ctx, candidateIDs)
	if err != nil {
		return common.ChatResponse{}, err
	}
	payload, _ := json.Marshal(candidates)
	answer, err := s.generation.Complete(ctx, `You are a friendly movie recommendation assistant.
Use only the movies in the supplied JSON and choose exactly three.
Respond using exactly three numbered entries and no introduction or conclusion.
Use this format for every entry:
1. Title: exact movie title
   Reason: one concise, specific sentence explaining why it matches the user's request, based on the supplied synopsis or tone
   [ID:123]
Do not use Markdown emphasis, headings, genre labels, database labels, or any other fields.
Never invent an identifier or recommend a movie outside the supplied JSON.`, fmt.Sprintf("User request: %s\nMovie database candidates: %s", message, payload))
	if err != nil {
		return common.ChatResponse{}, err
	}
	recommendedMovies, normalizedAnswer := normalizeRecommendations(answer, candidates)
	return common.ChatResponse{Message: normalizedAnswer, Movies: recommendedMovies}, nil
}

type parsedRecommendation struct {
	title  string
	reason string
	id     int
}

func normalizeRecommendations(answer string, candidates []common.MovieResponse) ([]common.MovieResponse, string) {
	byID := make(map[int]common.MovieResponse, len(candidates))
	byTitle := make(map[string]common.MovieResponse, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.TMDBID] = candidate
		byTitle[normalizeMovieTitle(candidate.Title)] = candidate
	}

	parsed := parseRecommendations(answer)
	recommended := make([]common.MovieResponse, 0, 3)
	entries := make([]parsedRecommendation, 0, 3)
	for _, item := range parsed {
		var movie common.MovieResponse
		var ok bool
		if item.title != "" {
			movie, ok = byTitle[normalizeMovieTitle(item.title)]
		}
		if !ok && item.id != 0 {
			movie, ok = byID[item.id]
		}
		if !ok || containsMovie(recommended, movie.TMDBID) {
			continue
		}
		recommended = append(recommended, movie)
		entries = append(entries, parsedRecommendation{
			title:  movie.Title,
			reason: cleanReason(item.reason),
		})
		if len(recommended) == 3 {
			break
		}
	}

	for _, candidate := range candidates {
		if len(recommended) == 3 {
			break
		}
		if containsMovie(recommended, candidate.TMDBID) {
			continue
		}
		recommended = append(recommended, candidate)
		entries = append(entries, parsedRecommendation{
			title:  candidate.Title,
			reason: "This movie matches the themes and tone of your request.",
		})
	}

	var builder strings.Builder
	for i, entry := range entries {
		fmt.Fprintf(&builder, "%d. Title: %s\n   Reason: %s\n", i+1, entry.title, entry.reason)
	}
	return recommended, strings.TrimSpace(builder.String())
}

func parseRecommendations(answer string) []parsedRecommendation {
	marker := regexp.MustCompile(`(?m)^\s*\d+\.\s*Title:\s*`)
	locations := marker.FindAllStringIndex(answer, -1)
	recommendations := make([]parsedRecommendation, 0, len(locations))
	idPattern := regexp.MustCompile(`\[ID:(\d+)\]`)
	for i, location := range locations {
		end := len(answer)
		if i+1 < len(locations) {
			end = locations[i+1][0]
		}
		block := answer[location[1]:end]
		lines := strings.Split(block, "\n")
		if len(lines) < 2 {
			continue
		}
		item := parsedRecommendation{title: strings.TrimSpace(lines[0])}
		for _, line := range lines[1:] {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(trimmed), "reason:") {
				item.reason = strings.TrimSpace(trimmed[len("Reason:"):])
				continue
			}
			if match := idPattern.FindStringSubmatch(trimmed); len(match) == 2 {
				item.id, _ = strconv.Atoi(match[1])
			}
		}
		if item.title != "" && item.reason != "" {
			recommendations = append(recommendations, item)
		}
	}
	return recommendations
}

func cleanReason(reason string) string {
	reason = regexp.MustCompile(`\*`).ReplaceAllString(reason, "")
	reason = regexp.MustCompile(`(?i)(?:the )?database (?:id|identifier)(?: for this movie)?\s*:\s*\d*`).ReplaceAllString(reason, "")
	return strings.TrimSpace(strings.Join(strings.Fields(reason), " "))
}

func normalizeMovieTitle(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(title)), " "))
}

func containsMovie(candidates []common.MovieResponse, tmdbID int) bool {
	for _, movie := range candidates {
		if movie.TMDBID == tmdbID {
			return true
		}
	}
	return false
}
