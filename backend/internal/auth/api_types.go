package auth

import (
	"errors"
	"strings"
	"unicode/utf8"
)

type LoginPostFormat struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type SignUpPostFormat struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type SessionResponse struct {
	LoggedIn bool   `json:"loggedIn"`
	UserID   string `json:"userId,omitempty"`
	Username string `json:"username,omitempty"`
}

type LoginResponse struct {
	LoggedIn bool   `json:"loggedIn"`
	UserID   string `json:"userId,omitempty"`
}

type SignUpResponse struct {
	Success bool   `json:"success"`
	UserID  string `json:"userId,omitempty"`
}

type UserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
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
