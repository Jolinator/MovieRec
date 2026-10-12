package gateway

import (
	"fmt"
	"net/url"

	"backend/internal/platform"
)

// Upstream is a backend service: Public serves its /movieAPI routes and Internal its /internal API.
type Upstream struct {
	Public   *url.URL
	Internal *url.URL
}

type Config struct {
	Auth            Upstream
	Movies          Upstream
	Recommendations Upstream
}

// LoadConfig reads upstream addresses from the environment; the defaults suit running every service on localhost.
func LoadConfig() (Config, error) {
	var config Config
	for _, upstream := range []struct {
		target                   *Upstream
		name                     string
		publicPort, internalPort string
	}{
		{&config.Auth, "AUTH", "8082", "8086"},
		{&config.Movies, "MOVIES", "8083", "8081"},
		{&config.Recommendations, "RECOMMENDATIONS", "8084", "8085"},
	} {
		var err error
		if upstream.target.Public, err = parseURL(upstream.name+"_URL", "http://localhost:"+upstream.publicPort); err != nil {
			return Config{}, err
		}
		if upstream.target.Internal, err = parseURL(upstream.name+"_INTERNAL_URL", "http://localhost:"+upstream.internalPort); err != nil {
			return Config{}, err
		}
	}
	return config, nil
}

func parseURL(name, fallback string) (*url.URL, error) {
	value := platform.Env(name, fallback)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid %s %q", name, value)
	}
	return parsed, nil
}
