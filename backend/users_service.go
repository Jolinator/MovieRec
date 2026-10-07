package main

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repository *UserRepository
}

func NewUserService(repository *UserRepository) *UserService {
	return &UserService{repository: repository}
}

func (s *UserService) Register(ctx context.Context, username, password string) (User, error) {
	passwordHash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}

	user, err := s.repository.Create(ctx, strings.TrimSpace(username), passwordHash)
	if err != nil {
		return User{}, fmt.Errorf("register user: %w", err)
	}

	return user, nil
}

func hashPassword(password string) (string, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(passwordHash), nil
}
