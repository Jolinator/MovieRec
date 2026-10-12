package recommendations

import (
	"strings"
	"testing"
)

func TestRecommendationChatRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short", "a", true},
		{"minimum", "ab", false},
		{"typical", "something funny with robots", false},
		{"maximum", strings.Repeat("a", 1000), false},
		{"too long", strings.Repeat("a", 1001), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RecommendationChatRequest{Message: tt.message}.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAddMovieToListValidate(t *testing.T) {
	for id, wantErr := range map[int]bool{-1: true, 0: true, 1: false, 603: false} {
		if err := (AddMovieToListPostFormat{TMDBID: id}).Validate(); (err != nil) != wantErr {
			t.Errorf("id %d: err = %v", id, err)
		}
	}
}

func TestParseTMDBID(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"1", 1, false}, {"603", 603, false},
		{"0", 0, true}, {"-4", 0, true}, {"", 0, true}, {"abc", 0, true}, {"1.5", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseTMDBID(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseTMDBID(%q) = %d, %v", tt.in, got, err)
		}
	}
}
