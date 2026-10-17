package movies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	tmdbDiscoverURL       = "https://api.themoviedb.org/3/discover/movie"
	defaultTMDBMovieLimit = 5000
)

type TMDBMovie struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	ReleaseDate  string  `json:"release_date"`
	Overview     string  `json:"overview"`
	PosterPath   *string `json:"poster_path"`
	BackdropPath *string `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	GenreIDs     []int   `json:"genre_ids"`
}

type tmdbDiscoverResponse struct {
	Page       int         `json:"page"`
	TotalPages int         `json:"total_pages"`
	Results    []TMDBMovie `json:"results"`
}

type TMDBClient struct {
	readToken  string
	httpClient *http.Client
	baseURL    string
}

func ImportLimit() (int, error) {
	value := os.Getenv("TMDB_IMPORT_LIMIT")
	if value == "" {
		return defaultTMDBMovieLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf("invalid TMDB_IMPORT_LIMIT %q", value)
	}
	return limit, nil
}

func NewTMDBClient(httpClient *http.Client) (*TMDBClient, error) {
	readToken := os.Getenv("TMDB_READ_ACCESS_TOKEN")
	if readToken == "" {
		return nil, errors.New("TMDB_READ_ACCESS_TOKEN must be set")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &TMDBClient{
		readToken:  readToken,
		httpClient: httpClient,
		baseURL:    tmdbDiscoverURL,
	}, nil
}

func (c *TMDBClient) DiscoverPopularMovies(ctx context.Context, limit int) ([]TMDBMovie, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("TMDB import limit must be positive")
	}

	movies := make([]TMDBMovie, 0, limit)
	for page := 1; len(movies) < limit; page++ {
		response, err := c.fetchDiscoverPage(ctx, page)
		if err != nil {
			return nil, err
		}
		if len(response.Results) == 0 {
			break
		}
		movies = append(movies, response.Results...)
		if page >= response.TotalPages {
			break
		}
	}
	if len(movies) > limit {
		movies = movies[:limit]
	}
	return movies, nil
}

func (c *TMDBClient) fetchDiscoverPage(ctx context.Context, page int) (tmdbDiscoverResponse, error) {
	query := url.Values{
		"language": {"en-US"},
		"sort_by":  {"popularity.desc"},
		"page":     {strconv.Itoa(page)},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return tmdbDiscoverResponse{}, fmt.Errorf("create TMDB request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.readToken)
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return tmdbDiscoverResponse{}, fmt.Errorf("request TMDB discover page %d: %w", page, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return tmdbDiscoverResponse{}, fmt.Errorf("TMDB discover page %d returned HTTP %s", page, response.Status)
	}

	var result tmdbDiscoverResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return tmdbDiscoverResponse{}, fmt.Errorf("decode TMDB discover page %d: %w", page, err)
	}
	return result, nil
}
