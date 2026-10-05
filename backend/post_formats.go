package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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



type LoginPostFormat struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type SignUpPostFormat struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
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
	if strings.TrimSpace(p.UserName) == "" {
		return errors.New("userName is required")
	}
	if p.Password == "" {
		return errors.New("password is required")
	}
	return nil
}