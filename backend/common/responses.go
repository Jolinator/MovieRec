package common

import (
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	Message                    string `json:"message"`
	AuthDBAvailable            bool   `json:"authDBAvailable"`
	MoviesDBAvailable          bool   `json:"moviesDBAvailable"`
	RecommendationsDBAvailable bool   `json:"recommendationsDBAvailable"`
}

type LoginResponse struct {
	LoggedIn bool   `json:"loggedIn"`
	UserID   string `json:"userId,omitempty"`
}

type SignUpResponse struct {
	Success bool   `json:"success"`
	UserID  string `json:"userId,omitempty"`
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

func WriteJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
	}
}
