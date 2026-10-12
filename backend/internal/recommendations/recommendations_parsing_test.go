package recommendations

import (
	"reflect"
	"strings"
	"testing"

	"backend/internal/common"
)

func movie(id int, title string) common.MovieResponse {
	return common.MovieResponse{TMDBID: id, Title: title}
}

func ids(movies []common.MovieResponse) []int {
	out := make([]int, len(movies))
	for i, m := range movies {
		out[i] = m.TMDBID
	}
	return out
}

func TestParseRecommendations(t *testing.T) {
	answer := `1. Title: The Matrix
   Reason: Hacker fights machines.
   [ID:603]
2. Title: Inception
   Reason: Dreams within dreams.
   [ID:27205]`
	got := parseRecommendations(answer)
	want := []parsedRecommendation{
		{title: "The Matrix", reason: "Hacker fights machines.", id: 603},
		{title: "Inception", reason: "Dreams within dreams.", id: 27205},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseRecommendationsSkipsMalformedEntries(t *testing.T) {
	tests := map[string]string{
		"empty":         "",
		"prose only":    "Sure! Here are some movies you may like.",
		"no reason":     "1. Title: Alien\n   [ID:1]",
		"title only":    "1. Title: Alien",
		"empty title":   "1. Title:\n   Reason: x\n",
		"no numbering":  "Title: Alien\nReason: scary",
		"reason casing": "",
	}
	delete(tests, "reason casing")
	for name, answer := range tests {
		t.Run(name, func(t *testing.T) {
			if got := parseRecommendations(answer); len(got) != 0 {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestParseRecommendationsIsCaseInsensitiveForReasonAndOptionalID(t *testing.T) {
	got := parseRecommendations("1. Title: Alien\n   REASON: Scary in space.\n")
	if len(got) != 1 || got[0].reason != "Scary in space." || got[0].id != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestCleanReason(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Plain reason.", "Plain reason."},
		{"**Bold** and *italic*", "Bold and italic"},
		{"  lots   of \t whitespace  ", "lots of whitespace"},
		{"Great film. Database ID: 123", "Great film."},
		{"Great film. The database identifier for this movie: 77", "Great film."},
		{"", ""},
	}
	for _, tt := range tests {
		if got := cleanReason(tt.in); got != tt.want {
			t.Errorf("cleanReason(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeMovieTitle(t *testing.T) {
	if got := normalizeMovieTitle("  The   MATRIX \n"); got != "the matrix" {
		t.Fatalf("got %q", got)
	}
}

func TestContainsMovie(t *testing.T) {
	movies := []common.MovieResponse{movie(1, "a"), movie(2, "b")}
	if !containsMovie(movies, 2) || containsMovie(movies, 3) || containsMovie(nil, 1) {
		t.Fatal("containsMovie returned wrong result")
	}
}

func TestNormalizeRecommendationsMatchesByTitleThenID(t *testing.T) {
	candidates := []common.MovieResponse{movie(1, "Alpha"), movie(2, "Beta"), movie(3, "Gamma"), movie(4, "Delta")}
	answer := `1. Title: gamma
   Reason: **Great** one.
   [ID:99]
2. Title: Unknown Title
   Reason: Matched through its id.
   [ID:2]
3. Title: Alpha
   Reason: First.
   [ID:1]
4. Title: Delta
   Reason: Over the limit.
   [ID:4]`
	movies, message := normalizeRecommendations(answer, candidates)
	if !reflect.DeepEqual(ids(movies), []int{3, 2, 1}) {
		t.Fatalf("ids = %v", ids(movies))
	}
	if strings.Contains(message, "*") || strings.Contains(message, "Delta") {
		t.Fatalf("message = %q", message)
	}
	if !strings.HasPrefix(message, "1. Title: Gamma\n   Reason: Great one.") {
		t.Fatalf("message = %q", message)
	}
}

func TestNormalizeRecommendationsIgnoresInventedAndDuplicateMovies(t *testing.T) {
	candidates := []common.MovieResponse{movie(1, "Alpha"), movie(2, "Beta"), movie(3, "Gamma"), movie(4, "Delta")}
	answer := `1. Title: Alpha
   Reason: One.
   [ID:1]
2. Title: Alpha
   Reason: Duplicate.
   [ID:1]
3. Title: Not A Candidate
   Reason: Invented.
   [ID:999]`
	movies, message := normalizeRecommendations(answer, candidates)
	if !reflect.DeepEqual(ids(movies), []int{1, 2, 3}) {
		t.Fatalf("ids = %v", ids(movies))
	}
	if strings.Count(message, "This movie matches the themes and tone of your request.") != 2 {
		t.Fatalf("expected two default reasons in %q", message)
	}
	if strings.Contains(message, "Invented") || strings.Contains(message, "Duplicate") {
		t.Fatalf("message = %q", message)
	}
}

func TestNormalizeRecommendationsFallsBackToCandidates(t *testing.T) {
	candidates := []common.MovieResponse{movie(1, "Alpha"), movie(2, "Beta")}
	movies, message := normalizeRecommendations("garbage answer", candidates)
	if !reflect.DeepEqual(ids(movies), []int{1, 2}) {
		t.Fatalf("ids = %v", ids(movies))
	}
	if !strings.Contains(message, "1. Title: Alpha") || !strings.Contains(message, "2. Title: Beta") {
		t.Fatalf("message = %q", message)
	}
}

func TestNormalizeRecommendationsNoCandidates(t *testing.T) {
	movies, message := normalizeRecommendations("1. Title: X\n Reason: y", nil)
	if len(movies) != 0 || message != "" {
		t.Fatalf("movies=%v message=%q", movies, message)
	}
}
