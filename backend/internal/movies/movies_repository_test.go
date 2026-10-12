package movies

import (
	"strings"
	"testing"
)

func TestMapTMDBMovie(t *testing.T) {
	poster := "/p.jpg"
	got, err := mapTMDBMovie(TMDBMovie{
		ID: 5, Title: "  Title  ", ReleaseDate: "2020-02-29", Overview: "o",
		PosterPath: &poster, VoteAverage: 7.46, GenreIDs: []int{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TMDBID != 5 || got.Title != "Title" || got.Synopsis != "o" || got.AverageRating != 7.5 {
		t.Fatalf("got %+v", got)
	}
	if got.ReleaseDate == nil || got.ReleaseDate.Format("2006-01-02") != "2020-02-29" {
		t.Fatalf("release date = %v", got.ReleaseDate)
	}
	if got.PosterPath != &poster || len(got.GenreIDs) != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestMapTMDBMovieRatingIsRoundedAndClamped(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{0, 0}, {5.04, 5}, {5.05, 5.1}, {9.99, 10}, {10, 10}, {12, 10}, {-3, 0},
	}
	for _, tt := range tests {
		got, err := mapTMDBMovie(TMDBMovie{ID: 1, Title: "t", VoteAverage: tt.in})
		if err != nil {
			t.Fatal(err)
		}
		if got.AverageRating != tt.want {
			t.Errorf("rating(%v) = %v, want %v", tt.in, got.AverageRating, tt.want)
		}
	}
}

func TestMapTMDBMovieOptionalReleaseDate(t *testing.T) {
	for _, date := range []string{"", "   "} {
		got, err := mapTMDBMovie(TMDBMovie{ID: 1, Title: "t", ReleaseDate: date})
		if err != nil || got.ReleaseDate != nil {
			t.Fatalf("date %q: got %+v, err %v", date, got, err)
		}
	}
}

func TestMapTMDBMovieErrors(t *testing.T) {
	tests := []struct {
		name  string
		movie TMDBMovie
		want  string
	}{
		{"zero id", TMDBMovie{ID: 0, Title: "t"}, "invalid id"},
		{"negative id", TMDBMovie{ID: -1, Title: "t"}, "invalid id"},
		{"empty title", TMDBMovie{ID: 1, Title: ""}, "empty title"},
		{"blank title", TMDBMovie{ID: 1, Title: "   "}, "empty title"},
		{"bad date", TMDBMovie{ID: 1, Title: "t", ReleaseDate: "01/02/2020"}, "invalid release date"},
		{"impossible date", TMDBMovie{ID: 1, Title: "t", ReleaseDate: "2021-02-30"}, "invalid release date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mapTMDBMovie(tt.movie)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}
