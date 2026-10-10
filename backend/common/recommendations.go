package common

import "errors"

type RecommendationChatRequest struct {
	Message string `json:"message"`
}

func (r RecommendationChatRequest) Validate() error {
	if len(r.Message) < 2 || len(r.Message) > 1000 {
		return errors.New("message must be between 2 and 1000 characters")
	}
	return nil
}
