package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"backend/common"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repository  *UserRepository
	tokenIssuer *TokenIssuer
}

func NewUserService(repository *UserRepository, tokenIssuer *TokenIssuer) *UserService {
	return &UserService{repository: repository, tokenIssuer: tokenIssuer}
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

func (s *UserService) Authenticate(ctx context.Context, username, password string) (User, string, error) {
	credentials, err := s.repository.FindByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, common.ErrUserNotFound) {
			return User{}, "", common.ErrInvalidCredentials
		}
		return User{}, "", fmt.Errorf("find user for login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(password)); err != nil {
		return User{}, "", common.ErrInvalidCredentials
	}

	token, err := s.tokenIssuer.Issue(credentials.User)
	if err != nil {
		return User{}, "", fmt.Errorf("issue login token: %w", err)
	}

	return credentials.User, token, nil
}

func hashPassword(password string) (string, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(passwordHash), nil
}
