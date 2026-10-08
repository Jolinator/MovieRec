package auth

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, contextKey{}, userID)
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(contextKey{}).(uuid.UUID)
	return userID, ok
}
