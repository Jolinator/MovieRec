package gateway

import (
	"strings"
	"testing"
)

var gatewayEnvNames = []string{
	"AUTH_URL", "AUTH_INTERNAL_URL", "MOVIES_URL", "MOVIES_INTERNAL_URL",
	"RECOMMENDATIONS_URL", "RECOMMENDATIONS_INTERNAL_URL",
}

func clearGatewayEnv(t *testing.T) {
	t.Helper()
	for _, name := range gatewayEnvNames {
		t.Setenv(name, "")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	clearGatewayEnv(t)
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		config.Auth.Public.String():              "http://localhost:8082",
		config.Auth.Internal.String():            "http://localhost:8086",
		config.Movies.Public.String():            "http://localhost:8083",
		config.Movies.Internal.String():          "http://localhost:8081",
		config.Recommendations.Public.String():   "http://localhost:8084",
		config.Recommendations.Internal.String(): "http://localhost:8085",
	}
	for got, expected := range want {
		if got != expected {
			t.Errorf("got %s, want %s", got, expected)
		}
	}
}

func TestLoadConfigReadsEnvironment(t *testing.T) {
	clearGatewayEnv(t)
	t.Setenv("AUTH_URL", "http://auth:1")
	t.Setenv("AUTH_INTERNAL_URL", "http://auth:2")
	t.Setenv("MOVIES_URL", "https://movies:3")
	t.Setenv("MOVIES_INTERNAL_URL", "http://movies:4")
	t.Setenv("RECOMMENDATIONS_URL", "http://rec:5")
	t.Setenv("RECOMMENDATIONS_INTERNAL_URL", "http://rec:6")
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Auth.Public.Host != "auth:1" || config.Auth.Internal.Host != "auth:2" ||
		config.Movies.Public.Scheme != "https" || config.Movies.Public.Host != "movies:3" ||
		config.Movies.Internal.Host != "movies:4" ||
		config.Recommendations.Public.Host != "rec:5" || config.Recommendations.Internal.Host != "rec:6" {
		t.Fatalf("config = %+v", config)
	}
}

func TestLoadConfigRejectsInvalidURLs(t *testing.T) {
	for _, name := range gatewayEnvNames {
		for _, value := range []string{"no-scheme-host", "http://", "://bad", "localhost:8080"} {
			t.Run(name+"="+value, func(t *testing.T) {
				clearGatewayEnv(t)
				t.Setenv(name, value)
				_, err := LoadConfig()
				if err == nil || !strings.Contains(err.Error(), name) {
					t.Fatalf("err = %v, want mention of %s", err, name)
				}
			})
		}
	}
}

func TestParseURL(t *testing.T) {
	t.Setenv("GW_TEST_URL", "")
	parsed, err := parseURL("GW_TEST_URL", "http://fallback:1")
	if err != nil || parsed.Host != "fallback:1" {
		t.Fatalf("fallback: %v, %v", parsed, err)
	}
	t.Setenv("GW_TEST_URL", "http://set:2/base")
	parsed, err = parseURL("GW_TEST_URL", "http://fallback:1")
	if err != nil || parsed.Host != "set:2" || parsed.Path != "/base" {
		t.Fatalf("set: %v, %v", parsed, err)
	}
	if _, err := parseURL("GW_TEST_URL", "bogus"); err != nil {
		t.Fatalf("set value should win over bad fallback: %v", err)
	}
	t.Setenv("GW_TEST_URL", "")
	if _, err := parseURL("GW_TEST_URL", "bogus"); err == nil {
		t.Fatal("bad fallback should fail")
	}
}
