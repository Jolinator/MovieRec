package movies

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"backend/internal/common"
)

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    int
		wantErr bool
	}{
		{"missing uses fallback", "", 7, false},
		{"valid", "n=42", 42, false},
		{"negative", "n=-3", -3, false},
		{"not a number", "n=abc", 0, true},
		{"float", "n=1.5", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _ := url.ParseQuery(tt.query)
			got, err := queryInt(values, "n", 7)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if err != nil && !errors.Is(err, common.ErrInvalidMoviePagination) {
				t.Fatalf("error does not wrap ErrInvalidMoviePagination: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseMoviePagination(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
		wantGenre  *int
		wantErr    bool
	}{
		{name: "defaults", query: "", wantLimit: 20},
		{name: "custom", query: "limit=50&offset=10", wantLimit: 50, wantOffset: 10},
		{name: "max limit", query: "limit=100", wantLimit: 100},
		{name: "genre", query: "genre=28", wantLimit: 20, wantGenre: ptr(28)},
		{name: "limit zero", query: "limit=0", wantErr: true},
		{name: "limit too large", query: "limit=101", wantErr: true},
		{name: "negative offset", query: "offset=-1", wantErr: true},
		{name: "bad limit", query: "limit=x", wantErr: true},
		{name: "bad offset", query: "offset=x", wantErr: true},
		{name: "genre zero", query: "genre=0", wantErr: true},
		{name: "genre negative", query: "genre=-2", wantErr: true},
		{name: "genre not a number", query: "genre=action", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _ := url.ParseQuery(tt.query)
			got, err := parseMoviePagination(values)
			if tt.wantErr {
				if !errors.Is(err, common.ErrInvalidMoviePagination) {
					t.Fatalf("err = %v, want ErrInvalidMoviePagination", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Limit != tt.wantLimit || got.Offset != tt.wantOffset {
				t.Fatalf("got %+v", got)
			}
			if (got.GenreID == nil) != (tt.wantGenre == nil) || (got.GenreID != nil && *got.GenreID != *tt.wantGenre) {
				t.Fatalf("genre = %v, want %v", got.GenreID, tt.wantGenre)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestValidateMoviePagination(t *testing.T) {
	tests := []struct {
		limit, offset int
		wantErr       bool
	}{
		{1, 0, false}, {100, 0, false}, {20, 500, false},
		{0, 0, true}, {101, 0, true}, {-1, 0, true}, {10, -1, true},
	}
	for _, tt := range tests {
		err := validateMoviePagination(tt.limit, tt.offset)
		if (err != nil) != tt.wantErr {
			t.Errorf("validate(%d,%d) err = %v", tt.limit, tt.offset, err)
		}
	}
}

func TestParseTMDBIDs(t *testing.T) {
	many := make([]string, 101)
	for i := range many {
		many[i] = "1"
	}
	tests := []struct {
		name    string
		value   string
		want    []int
		wantErr bool
	}{
		{"single", "5", []int{5}, false},
		{"multiple", "1,2,3", []int{1, 2, 3}, false},
		{"empty", "", nil, true},
		{"zero", "0", nil, true},
		{"negative", "1,-2", nil, true},
		{"not a number", "1,a", nil, true},
		{"trailing comma", "1,", nil, true},
		{"spaces", "1, 2", nil, true},
		{"too many", strings.Join(many, ","), nil, true},
		{"exactly max", strings.Repeat("1,", 99) + "1", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTMDBIDs(tt.value)
			if tt.wantErr {
				if !errors.Is(err, common.ErrInvalidMoviePagination) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.want != nil {
				if len(got) != len(tt.want) {
					t.Fatalf("got %v", got)
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Fatalf("got %v, want %v", got, tt.want)
					}
				}
			}
		})
	}
}

func TestServiceQueryValidationHappensBeforeRepositoryAccess(t *testing.T) {
	service := NewMovieService(nil) // a nil repository would panic if reached
	ctx := context.Background()

	tests := map[string]func() error{
		"list bad limit":     func() error { _, err := service.ListFromQuery(ctx, url.Values{"limit": {"0"}}); return err },
		"list bad genre":     func() error { _, err := service.ListFromQuery(ctx, url.Values{"genre": {"-1"}}); return err },
		"list direct":        func() error { _, err := service.List(ctx, MoviePagination{Limit: 1000}); return err },
		"ids missing":        func() error { _, err := service.ListByTMDBIDsFromQuery(ctx, url.Values{}); return err },
		"ids invalid":        func() error { _, err := service.ListByTMDBIDsFromQuery(ctx, url.Values{"ids": {"x"}}); return err },
		"synopses bad limit": func() error { _, err := service.ListSynopsesFromQuery(ctx, url.Values{"limit": {"0"}}); return err },
		"synopses big limit": func() error { _, err := service.ListSynopsesFromQuery(ctx, url.Values{"limit": {"101"}}); return err },
		"synopses bad after": func() error { _, err := service.ListSynopsesFromQuery(ctx, url.Values{"after": {"x"}}); return err },
		"synopses neg after": func() error { _, err := service.ListSynopsesFromQuery(ctx, url.Values{"after": {"-1"}}); return err },
		"synopses bad limit text": func() error {
			_, err := service.ListSynopsesFromQuery(ctx, url.Values{"limit": {"x"}})
			return err
		},
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, common.ErrInvalidMoviePagination) {
				t.Fatalf("err = %v, want ErrInvalidMoviePagination", err)
			}
		})
	}
}

func TestMarkImported(t *testing.T) {
	service := NewMovieService(nil)
	if service.Imported() {
		t.Fatal("new service should not be imported")
	}
	service.MarkImported()
	if !service.Imported() {
		t.Fatal("expected imported after MarkImported")
	}
}

func TestMovieResponse(t *testing.T) {
	date := time.Date(2021, 3, 9, 15, 4, 5, 0, time.UTC)
	poster, backdrop := "/p.jpg", "/b.jpg"
	got := MovieResponse(Movie{
		TMDBID: 9, Title: "T", ReleaseDate: &date, AverageRating: 7.5,
		PosterPath: &poster, Synopsis: "S", BackdropPath: &backdrop,
	})
	if got.TMDBID != 9 || got.Title != "T" || got.AverageRating != 7.5 || got.Synopsis != "S" {
		t.Fatalf("got %+v", got)
	}
	if got.ReleaseDate == nil || *got.ReleaseDate != "2021-03-09" {
		t.Fatalf("release date = %v", got.ReleaseDate)
	}
	if got.PosterPath != &poster || got.BackdropPath != &backdrop {
		t.Fatal("paths not preserved")
	}
}

func TestMovieResponseWithoutReleaseDate(t *testing.T) {
	if got := MovieResponse(Movie{TMDBID: 1}); got.ReleaseDate != nil {
		t.Fatalf("release date = %v, want nil", got.ReleaseDate)
	}
}

func TestMovieResponses(t *testing.T) {
	if got := MovieResponses(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil input should yield empty non-nil slice, got %#v", got)
	}
	got := MovieResponses([]Movie{{TMDBID: 1}, {TMDBID: 2}})
	if len(got) != 2 || got[0].TMDBID != 1 || got[1].TMDBID != 2 {
		t.Fatalf("got %+v", got)
	}
}
