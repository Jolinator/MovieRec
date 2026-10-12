package common

// Types shared across service boundaries; service-specific API types live in their own package.

type HealthResponse struct {
	Message                    string `json:"message"`
	BackendAvailable           bool   `json:"backendAvailable"`
	ImportedMoviesAvailable    bool   `json:"importedMoviesAvailable"`
	AuthDBAvailable            bool   `json:"authDBAvailable"`
	MoviesDBAvailable          bool   `json:"moviesDBAvailable"`
	RecommendationsDBAvailable bool   `json:"recommendationsDBAvailable"`
	RecommendationsReady       bool   `json:"recommendationsReady"`
}

type ReadyResponse struct {
	Ready bool `json:"ready"`
}

type MovieResponse struct {
	TMDBID        int     `json:"tmdbId"`
	Title         string  `json:"title"`
	ReleaseDate   *string `json:"releaseDate"`
	AverageRating float64 `json:"averageRating"`
	PosterPath    *string `json:"posterPath"`
	Synopsis      string  `json:"synopsis"`
	BackdropPath  *string `json:"backdropPath"`
}

type MovieSynopsis struct {
	TMDBID   int    `json:"tmdbId"`
	Synopsis string `json:"synopsis"`
}
