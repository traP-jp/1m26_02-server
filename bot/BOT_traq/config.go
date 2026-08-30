package main

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const defaultPort = "8080"

type config struct {
	Port              string
	APIBaseURL        string
	AccessToken       string
	VerificationToken string
	AutoRegister      bool
	BotEndpoint       string
	DevUserName       string
	DevUserPassword   string
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		Port:              strings.TrimSpace(getenv("PORT")),
		APIBaseURL:        strings.TrimRight(strings.TrimSpace(getenv("TRAQ_API_BASE_URL")), "/"),
		AccessToken:       strings.TrimSpace(getenv("TRAQ_BOT_ACCESS_TOKEN")),
		VerificationToken: strings.TrimSpace(getenv("TRAQ_BOT_VERIFICATION_TOKEN")),
		BotEndpoint:       strings.TrimRight(strings.TrimSpace(getenv("TRAQ_BOT_ENDPOINT")), "/"),
		DevUserName:       strings.TrimSpace(getenv("TRAQ_DEV_USER_NAME")),
		DevUserPassword:   strings.TrimSpace(getenv("TRAQ_DEV_USER_PASSWORD")),
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	autoRegister := strings.TrimSpace(getenv("TRAQ_BOT_AUTO_REGISTER"))
	if autoRegister != "" {
		var err error
		cfg.AutoRegister, err = strconv.ParseBool(autoRegister)
		if err != nil {
			return config{}, errors.New("TRAQ_BOT_AUTO_REGISTER must be a boolean")
		}
	}

	var missing []string
	if cfg.APIBaseURL == "" {
		missing = append(missing, "TRAQ_API_BASE_URL")
	}
	if cfg.AutoRegister {
		if cfg.BotEndpoint == "" {
			missing = append(missing, "TRAQ_BOT_ENDPOINT")
		}
		if cfg.DevUserName == "" {
			missing = append(missing, "TRAQ_DEV_USER_NAME")
		}
		if cfg.DevUserPassword == "" {
			missing = append(missing, "TRAQ_DEV_USER_PASSWORD")
		}
	} else {
		if cfg.AccessToken == "" {
			missing = append(missing, "TRAQ_BOT_ACCESS_TOKEN")
		}
		if cfg.VerificationToken == "" {
			missing = append(missing, "TRAQ_BOT_VERIFICATION_TOKEN")
		}
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	u, err := url.Parse(cfg.APIBaseURL)
	if err != nil || u.Host == "" || u.Path == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return config{}, errors.New("TRAQ_API_BASE_URL must be an http(s) URL including /api/v3")
	}
	if !strings.HasSuffix(u.Path, "/api/v3") {
		return config{}, errors.New("TRAQ_API_BASE_URL must end with /api/v3")
	}
	if cfg.AutoRegister {
		endpoint, err := url.Parse(cfg.BotEndpoint)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return config{}, errors.New("TRAQ_BOT_ENDPOINT must be an http(s) URL")
		}
	}

	return cfg, nil
}
