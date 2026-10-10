package recommendations

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	"backend/common"
	"backend/movies"
)

type Service struct {
	repository *movies.MovieRepository
	embedding  *EmbeddingClient
	generation *GenerationClient
	ready      atomic.Bool
}

func NewService(repository *movies.MovieRepository) *Service {
	return &Service{repository: repository, embedding: NewEmbeddingClient(), generation: NewGenerationClient()}
}
func (s *Service) Initialize(ctx context.Context) error {
	if err := s.repository.EmbedSynopses(ctx, s.embedding); err != nil {
		return err
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
	return fmt.Errorf("embedding initialization failed after %d attempts: %w", attempts, lastErr)
}
func (s *Service) Ready() bool { return s.ready.Load() }
func (s *Service) Chat(ctx context.Context, message string) (ChatResponse, error) {
	if !s.Ready() {
		return ChatResponse{}, common.ErrRecommendationsUnavailable
	}
	vector, err := s.embedding.Embed(ctx, message)
	if err != nil {
		return ChatResponse{}, err
	}
	candidates, err := s.repository.SearchSimilar(ctx, vector, 8)
	if err != nil {
		return ChatResponse{}, err
	}
	payload, _ := json.Marshal(candidates)
	answer, err := s.generation.Complete(ctx, `You are a friendly movie recommendation assistant.
Use only the movies in the supplied JSON.
Choose exactly three movies that best match the user's request.
Write a short, natural explanation with one specific reason for each choice, based on its title, synopsis, genre, or tone.
Do not write a generic heading such as "Recommended movies".
At the end of each movie's explanation, include its database identifier exactly as [ID:123].
Never invent an identifier and do not recommend any movie outside the supplied JSON.`, fmt.Sprintf("User request: %s\nMovie database candidates: %s", message, payload))
	if err != nil {
		return ChatResponse{}, err
	}
	valid := make(map[int]bool)
	for _, movie := range candidates {
		valid[movie.TMDBID] = true
	}
	ids := make([]int, 0)
	re := regexp.MustCompile(`\[ID:(\d+)\]`)
	for _, match := range re.FindAllStringSubmatch(answer, -1) {
		id, _ := strconv.Atoi(match[1])
		if valid[id] && !containsID(ids, id) && len(ids) < 3 {
			ids = append(ids, id)
		}
	}
	recommendedMovies := candidates
	if len(ids) > 0 {
		byID := make(map[int]movies.Movie, len(candidates))
		for _, movie := range candidates {
			byID[movie.TMDBID] = movie
		}
		recommendedMovies = make([]movies.Movie, 0, len(ids))
		for _, id := range ids {
			recommendedMovies = append(recommendedMovies, byID[id])
		}
		for _, candidate := range candidates {
			if len(recommendedMovies) == 3 {
				break
			}
			if !containsID(ids, candidate.TMDBID) {
				recommendedMovies = append(recommendedMovies, candidate)
			}
		}
	}
	if len(recommendedMovies) > 3 {
		recommendedMovies = recommendedMovies[:3]
	}
	answer = re.ReplaceAllString(answer, "")
	answer = regexp.MustCompile(`(?im)[ \t]*[-*]?[ \t]*the database identifier for this movie is[ \t]*[.:]?`).ReplaceAllString(answer, "")
	return ChatResponse{Message: answer, Movies: movieResponses(recommendedMovies)}, nil
}

func containsID(ids []int, candidate int) bool {
	for _, id := range ids {
		if id == candidate {
			return true
		}
	}
	return false
}

func movieResponses(input []movies.Movie) []common.MovieResponse {
	response := make([]common.MovieResponse, 0, len(input))
	for _, movie := range input {
		var releaseDate *string
		if movie.ReleaseDate != nil {
			formatted := movie.ReleaseDate.Format("2006-01-02")
			releaseDate = &formatted
		}
		response = append(response, common.MovieResponse{
			TMDBID: movie.TMDBID, Title: movie.Title, ReleaseDate: releaseDate,
			AverageRating: movie.AverageRating, PosterPath: movie.PosterPath,
			Synopsis: movie.Synopsis, BackdropPath: movie.BackdropPath,
		})
	}
	return response
}

type ChatResponse struct {
	Message string                 `json:"message"`
	Movies  []common.MovieResponse `json:"movies"`
}
