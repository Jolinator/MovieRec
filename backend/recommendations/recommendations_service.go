package recommendations

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
Use only the movies in the supplied JSON and choose exactly three.
Respond using exactly three numbered entries and no introduction or conclusion.
Use this format for every entry:
1. Title: exact movie title
   Reason: one concise, specific sentence explaining why it matches the user's request, based on the supplied synopsis or tone
   [ID:123]
Do not use Markdown emphasis, headings, genre labels, database labels, or any other fields.
Never invent an identifier or recommend a movie outside the supplied JSON.`, fmt.Sprintf("User request: %s\nMovie database candidates: %s", message, payload))
	if err != nil {
		return ChatResponse{}, err
	}
	recommendedMovies, normalizedAnswer := normalizeRecommendations(answer, candidates)
	return ChatResponse{Message: normalizedAnswer, Movies: movieResponses(recommendedMovies)}, nil
}

type parsedRecommendation struct {
	title  string
	reason string
	id     int
}

func normalizeRecommendations(answer string, candidates []movies.Movie) ([]movies.Movie, string) {
	byID := make(map[int]movies.Movie, len(candidates))
	byTitle := make(map[string]movies.Movie, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.TMDBID] = candidate
		byTitle[normalizeMovieTitle(candidate.Title)] = candidate
	}

	parsed := parseRecommendations(answer)
	recommended := make([]movies.Movie, 0, 3)
	entries := make([]parsedRecommendation, 0, 3)
	for _, item := range parsed {
		var movie movies.Movie
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

func containsMovie(movies []movies.Movie, tmdbID int) bool {
	for _, movie := range movies {
		if movie.TMDBID == tmdbID {
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
