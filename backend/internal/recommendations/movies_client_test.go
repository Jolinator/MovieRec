package recommendations

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/common"
)

func moviesClientFor(t *testing.T, handler http.HandlerFunc) *MoviesClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &MoviesClient{baseURL: server.URL, client: server.Client()}
}

func TestNewMoviesClient(t *testing.T) {
	t.Setenv("MOVIES_INTERNAL_URL", "http://movies:9/")
	if got := NewMoviesClient().baseURL; got != "http://movies:9" {
		t.Fatalf("baseURL = %q", got)
	}
	t.Setenv("MOVIES_INTERNAL_URL", "")
	t.Setenv("MOVIES_INTERNAL_PORT", "")
	if got := NewMoviesClient().baseURL; got != "http://localhost:8081" {
		t.Fatalf("default baseURL = %q", got)
	}
	t.Setenv("MOVIES_INTERNAL_PORT", "1234")
	if got := NewMoviesClient().baseURL; got != "http://localhost:1234" {
		t.Fatalf("port baseURL = %q", got)
	}
}

func TestJoinIDs(t *testing.T) {
	if got := joinIDs([]int{1, 22, 333}); got != "1,22,333" {
		t.Fatalf("got %q", got)
	}
	if got := joinIDs(nil); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestMoviesClientReady(t *testing.T) {
	for _, want := range []bool{true, false} {
		client := moviesClientFor(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/internal/movies/ready" {
				t.Errorf("path = %s", r.URL.Path)
			}
			fmt.Fprintf(w, `{"ready":%v}`, want)
		})
		got, err := client.Ready(context.Background())
		if err != nil || got != want {
			t.Fatalf("got %v, %v; want %v", got, err, want)
		}
	}
}

func TestMoviesClientListSynopsesBuildsQuery(t *testing.T) {
	client := moviesClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/movies/synopses" || r.URL.Query().Get("after") != "7" || r.URL.Query().Get("limit") != "50" {
			t.Errorf("url = %s", r.URL)
		}
		_, _ = w.Write([]byte(`[{"tmdbId":8,"synopsis":"s"}]`))
	})
	got, err := client.ListSynopses(context.Background(), 7, 50)
	if err != nil || len(got) != 1 || got[0].TMDBID != 8 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestMoviesClientListByTMDBIDsChunksRequests(t *testing.T) {
	var requests []int
	client := moviesClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		requests = append(requests, len(ids))
		items := make([]string, len(ids))
		for i, id := range ids {
			items[i] = fmt.Sprintf(`{"tmdbId":%s}`, id)
		}
		fmt.Fprintf(w, "[%s]", strings.Join(items, ","))
	})
	ids := make([]int, 250)
	for i := range ids {
		ids[i] = i + 1
	}
	got, err := client.ListByTMDBIDs(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 3 || requests[0] != 100 || requests[1] != 100 || requests[2] != 50 {
		t.Fatalf("chunks = %v", requests)
	}
	if len(got) != 250 || got[0].TMDBID != 1 || got[249].TMDBID != 250 {
		t.Fatalf("got %d movies", len(got))
	}
}

func TestMoviesClientListByTMDBIDsEmptyMakesNoRequest(t *testing.T) {
	client := moviesClientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") })
	got, err := client.ListByTMDBIDs(context.Background(), nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestMoviesClientErrorMapping(t *testing.T) {
	tests := []struct {
		status  int
		wantIs  error
		wantNot error
	}{
		{http.StatusNotFound, common.ErrMovieNotFound, nil},
		{http.StatusServiceUnavailable, common.ErrMoviesDatabaseUnavailable, nil},
		{http.StatusBadRequest, nil, common.ErrMoviesDatabaseUnavailable},
		{http.StatusInternalServerError, nil, common.ErrMoviesDatabaseUnavailable},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			client := moviesClientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status) })
			_, err := client.ListByTMDBIDs(context.Background(), []int{1})
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("err = %v, want %v", err, tt.wantIs)
			}
			if tt.wantNot != nil && errors.Is(err, tt.wantNot) {
				t.Fatalf("err = %v must not be %v", err, tt.wantNot)
			}
		})
	}
}

func TestMoviesClientTransportFailureIsDatabaseUnavailable(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	client := &MoviesClient{baseURL: url, client: http.DefaultClient}
	if _, err := client.Ready(context.Background()); !errors.Is(err, common.ErrMoviesDatabaseUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestMoviesErrorNil(t *testing.T) {
	if moviesError(nil) != nil {
		t.Fatal("nil must map to nil")
	}
}
