package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Validatable interface {
	Validate() error
}

func decodeAndValidate[T Validatable](r *http.Request) (T, error) {
	var body T

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return body, fmt.Errorf("invalid JSON: %w", err)
	}

	if err := body.Validate(); err != nil {
		return body, err
	}

	return body, nil
}

func ValidateRequest[T Validatable](r *http.Request, w http.ResponseWriter) (T, bool) {
	request, err := decodeAndValidate[T](r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return request, false
	}
	return request, true
}
