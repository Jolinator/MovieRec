package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testBody struct {
	Name string `json:"name"`
}

func (b testBody) Validate() error {
	if b.Name == "" {
		return http.ErrMissingFile
	}
	return nil
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, map[string]int{"a": 1})

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q", got)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"a":1}` {
		t.Fatalf("body = %q", got)
	}
}

func TestWriteJSONEncodeError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, make(chan int))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		wantOK bool
		want   int
	}{
		{"valid", `{"name":"x"}`, true, http.StatusOK},
		{"invalid json", `{`, false, http.StatusBadRequest},
		{"fails validation", `{"name":""}`, false, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			_, ok := ValidateRequest[testBody](req, rec)
			if ok != tt.wantOK || rec.Code != tt.want {
				t.Fatalf("ok=%v code=%d, want ok=%v code=%d", ok, rec.Code, tt.wantOK, tt.want)
			}
		})
	}
}
