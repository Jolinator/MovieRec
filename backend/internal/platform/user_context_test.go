package platform

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestUserIDContext(t *testing.T) {
	if _, ok := UserIDFromContext(context.Background()); ok {
		t.Fatal("empty context should have no user")
	}
	id := uuid.New()
	got, ok := UserIDFromContext(WithUserID(context.Background(), id))
	if !ok || got != id {
		t.Fatalf("got %s, %v", got, ok)
	}
}
