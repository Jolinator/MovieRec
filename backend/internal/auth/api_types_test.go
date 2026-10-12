package auth

import (
	"strings"
	"testing"
)

func TestLoginValidate(t *testing.T) {
	tests := []struct {
		name, user, pass string
		wantErr          bool
	}{
		{"valid", "alice", "x", false},
		{"empty user", "", "pw", true},
		{"blank user", "   ", "pw", true},
		{"empty password", "alice", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := (LoginPostFormat{UserName: tt.user, Password: tt.pass}).Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestSignUpValidate(t *testing.T) {
	tests := []struct {
		name, user, pass string
		wantErr          bool
	}{
		{"valid", "alice", "password1", false},
		{"blank user", "  ", "password1", true},
		{"username 100 runes", strings.Repeat("é", 100), "password1", false},
		{"username 101 runes", strings.Repeat("é", 101), "password1", true},
		{"padded username counts trimmed", "  " + strings.Repeat("a", 100) + "  ", "password1", false},
		{"empty password", "alice", "", true},
		{"password 7 chars", "alice", "1234567", true},
		{"password 8 chars", "alice", "12345678", false},
		{"password 72 bytes", "alice", strings.Repeat("a", 72), false},
		{"password 73 bytes", "alice", strings.Repeat("a", 73), true},
		{"multibyte password over 72 bytes", "alice", strings.Repeat("é", 37), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := (SignUpPostFormat{UserName: tt.user, Password: tt.pass}).Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
