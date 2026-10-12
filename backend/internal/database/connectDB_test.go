package database

import (
	"strings"
	"testing"
)

func setDBEnv(t *testing.T, prefix string, overrides map[string]string) {
	t.Helper()
	values := map[string]string{
		"DB_HOST": "db.local", "DB_PORT": "6543", "DB_SSLMODE": "disable",
		"DB_NAME": "name", "DB_USER": "user", "DB_PASSWORD": "pa ss:w/rd",
	}
	for key, value := range overrides {
		values[key] = value
	}
	for key, value := range values {
		t.Setenv(prefix+key, value)
	}
}

func TestLoadDatabaseConfig(t *testing.T) {
	setDBEnv(t, "AUTH_", nil)
	config, err := loadDatabaseConfig("AUTH_")
	if err != nil {
		t.Fatal(err)
	}
	if config.host != "db.local" || config.port != 6543 || config.name != "name" ||
		config.user != "user" || config.password != "pa ss:w/rd" || config.sslMode != "disable" {
		t.Fatalf("config = %+v", config)
	}
}

func TestLoadDatabaseConfigDefaultsPort(t *testing.T) {
	setDBEnv(t, "AUTH_", map[string]string{"DB_PORT": ""})
	config, err := loadDatabaseConfig("AUTH_")
	if err != nil {
		t.Fatal(err)
	}
	if config.port != 5432 {
		t.Fatalf("port = %d", config.port)
	}
}

func TestLoadDatabaseConfigErrors(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		want      string
	}{
		{"bad port", map[string]string{"DB_PORT": "abc"}, "DB_PORT"},
		{"zero port", map[string]string{"DB_PORT": "0"}, "DB_PORT"},
		{"port too large", map[string]string{"DB_PORT": "70000"}, "DB_PORT"},
		{"no host", map[string]string{"DB_HOST": ""}, "DB_HOST"},
		{"no name", map[string]string{"DB_NAME": ""}, "DB_NAME"},
		{"no user", map[string]string{"DB_USER": ""}, "DB_USER"},
		{"no password", map[string]string{"DB_PASSWORD": ""}, "DB_PASSWORD"},
		{"no sslmode", map[string]string{"DB_SSLMODE": ""}, "DB_SSLMODE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDBEnv(t, "MOVIES_", tt.overrides)
			_, err := loadDatabaseConfig("MOVIES_")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want mention of %s", err, tt.want)
			}
		})
	}
}

func TestSetupConnectionURLEscapesCredentials(t *testing.T) {
	url := setupConnectionURL(DatabaseConfig{
		host: "h", port: 5432, name: "db", user: "us@er", password: "p@ss/word", sslMode: "require",
	})
	if url.Scheme != "postgres" || url.Host != "h:5432" || url.Path != "/db" {
		t.Fatalf("url = %s", url.String())
	}
	if url.Query().Get("sslmode") != "require" {
		t.Fatalf("sslmode = %q", url.Query().Get("sslmode"))
	}
	if password, _ := url.User.Password(); url.User.Username() != "us@er" || password != "p@ss/word" {
		t.Fatalf("credentials not preserved: %s", url.User)
	}
	if strings.Contains(url.String(), "p@ss/word") {
		t.Fatal("password must be escaped in the URL string")
	}
}

func TestSetupConnectionURLIPv6Host(t *testing.T) {
	url := setupConnectionURL(DatabaseConfig{host: "::1", port: 5432, name: "db", user: "u", password: "p", sslMode: "disable"})
	if url.Host != "[::1]:5432" {
		t.Fatalf("host = %q", url.Host)
	}
}

func TestPingWithRetryRejectsInvalidAttempts(t *testing.T) {
	if err := PingWithRetry(t.Context(), nil, 0, 0); err == nil {
		t.Fatal("expected error for zero attempts")
	}
}
