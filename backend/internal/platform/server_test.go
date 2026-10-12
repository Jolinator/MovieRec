package platform

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestEnv(t *testing.T) {
	t.Setenv("PLATFORM_TEST_VAR", "")
	if got := Env("PLATFORM_TEST_VAR", "fallback"); got != "fallback" {
		t.Errorf("empty var = %q", got)
	}
	t.Setenv("PLATFORM_TEST_VAR", "value")
	if got := Env("PLATFORM_TEST_VAR", "fallback"); got != "value" {
		t.Errorf("set var = %q", got)
	}
}

func TestPortFromEnv(t *testing.T) {
	tests := []struct {
		value, fallback, want string
	}{
		{"", "8080", "8080"},
		{"9000", "8080", "9000"},
		{"65535", "8080", "65535"},
		{"0", "8080", ""},
		{"65536", "8080", ""},
		{"abc", "8080", ""},
		{"-1", "8080", ""},
		{"", "oops", ""},
	}
	for _, tt := range tests {
		t.Run(tt.value+"/"+tt.fallback, func(t *testing.T) {
			t.Setenv("PLATFORM_TEST_PORT", tt.value)
			if got := PortFromEnv("PLATFORM_TEST_PORT", tt.fallback); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewHTTPServer(t *testing.T) {
	handler := http.NewServeMux()
	server := NewHTTPServer("8123", handler)
	if server.Addr != ":8123" || server.Handler != handler {
		t.Fatalf("addr=%q", server.Addr)
	}
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatal("timeouts must be set")
	}
}

func TestHandleErrorsReturnsOnServerError(t *testing.T) {
	errs := make(chan error, 1)
	errs <- errors.New("boom")
	done := make(chan struct{})
	go func() {
		HandleErrors(errs, context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleErrors did not return")
	}
}

func TestHandleErrorsShutsDownOnContextCancel(t *testing.T) {
	server := NewHTTPServer("0", http.NewServeMux())
	server.Addr = "127.0.0.1:0"
	errs := RunServers(server)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		HandleErrors(errs, ctx, server)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("HandleErrors did not return after cancel")
	}
	if err := <-errs; !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("server error = %v", err)
	}
}
