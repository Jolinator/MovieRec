package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
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

type LoginPostFormat struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type SignUpPostFormat struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type AddMovieToListPostFormat struct {
	TMDBID int `json:"tmdbId"`
}

func (p AddMovieToListPostFormat) Validate() error {
	if p.TMDBID <= 0 {
		return errors.New("tmdbId must be a positive integer")
	}
	return nil
}

func (p LoginPostFormat) Validate() error {
	if strings.TrimSpace(p.UserName) == "" {
		return errors.New("userName is required")
	}
	if p.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

func (p SignUpPostFormat) Validate() error {
	username := strings.TrimSpace(p.UserName)
	if username == "" {
		return errors.New("userName is required")
	}
	if utf8.RuneCountInString(username) > 100 {
		return errors.New("userName must be at most 100 characters")
	}
	if p.Password == "" {
		return errors.New("password is required")
	}
	if utf8.RuneCountInString(p.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len([]byte(p.Password)) > 72 {
		return errors.New("password must be at most 72 bytes")
	}
	return nil
}
