package recommendations

import (
	"errors"

	"backend/internal/common"
)

type RecommendationChatRequest struct {
	Message string `json:"message"`
}

type AddMovieToListPostFormat struct {
	TMDBID int `json:"tmdbId"`
}

type ChatResponse struct {
	Message string                 `json:"message"`
	Movies  []common.MovieResponse `json:"movies"`
}

type PersonalizedRecommendationsResponse struct {
	Status string                 `json:"status"`
	Movies []common.MovieResponse `json:"movies"`
}

func (r RecommendationChatRequest) Validate() error {
	if len(r.Message) < 2 || len(r.Message) > 1000 {
		return errors.New("message must be between 2 and 1000 characters")
	}
	return nil
}

func (p AddMovieToListPostFormat) Validate() error {
	if p.TMDBID <= 0 {
		return errors.New("tmdbId must be a positive integer")
	}
	return nil
}
