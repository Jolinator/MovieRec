package recommendations

import (
	"reflect"
	"testing"
)

func TestVectorLiteral(t *testing.T) {
	tests := []struct {
		in   []float32
		want string
	}{
		{nil, "[]"},
		{[]float32{}, "[]"},
		{[]float32{1}, "[1]"},
		{[]float32{0.5, -2, 3.25}, "[0.5,-2,3.25]"},
	}
	for _, tt := range tests {
		if got := vectorLiteral(tt.in); got != tt.want {
			t.Errorf("vectorLiteral(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseVectorLiteral(t *testing.T) {
	tests := []struct {
		in      string
		want    []float32
		wantErr bool
	}{
		{"[]", []float32{}, false},
		{"", []float32{}, false},
		{"[1,2,3]", []float32{1, 2, 3}, false},
		{" [0.5, -2 ,3.25] ", []float32{0.5, -2, 3.25}, false},
		{"[1,abc]", nil, true},
		{"[1,,2]", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseVectorLiteral(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVectorLiteralRoundTrip(t *testing.T) {
	in := []float32{0.125, -0.5, 1e-3, 42}
	out, err := parseVectorLiteral(vectorLiteral(in))
	if err != nil || !reflect.DeepEqual(in, out) {
		t.Fatalf("out = %v, err = %v", out, err)
	}
}
