package main

import "testing"

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"TRAQ_API_BASE_URL":           "https://q.trap.jp/api/v3/",
		"TRAQ_BOT_ACCESS_TOKEN":       "access-token",
		"TRAQ_BOT_VERIFICATION_TOKEN": "verification-token",
	}
	cfg, err := loadConfig(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Port != defaultPort {
		t.Errorf("Port = %q, want %q", cfg.Port, defaultPort)
	}
	if cfg.APIBaseURL != "https://q.trap.jp/api/v3" {
		t.Errorf("APIBaseURL = %q", cfg.APIBaseURL)
	}
}

func TestLoadConfigRequiresSecrets(t *testing.T) {
	t.Parallel()

	if _, err := loadConfig(func(string) string { return "" }); err == nil {
		t.Fatal("loadConfig() error = nil, want error")
	}
}

func TestLoadConfigForAutoRegistration(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"TRAQ_API_BASE_URL":      "http://backend/api/v3",
		"TRAQ_BOT_AUTO_REGISTER": "true",
		"TRAQ_BOT_ENDPOINT":      "http://bot-traq:8080/",
		"TRAQ_DEV_USER_NAME":     "qbot_dev",
		"TRAQ_DEV_USER_PASSWORD": "qbot-dev-password",
	}
	cfg, err := loadConfig(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if !cfg.AutoRegister {
		t.Fatal("AutoRegister = false, want true")
	}
	if cfg.BotEndpoint != "http://bot-traq:8080" {
		t.Errorf("BotEndpoint = %q", cfg.BotEndpoint)
	}
}

func TestLoadConfigRejectsInvalidAutoRegister(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"TRAQ_API_BASE_URL":      "http://backend/api/v3",
		"TRAQ_BOT_AUTO_REGISTER": "sometimes",
	}
	if _, err := loadConfig(func(key string) string { return env[key] }); err == nil {
		t.Fatal("loadConfig() error = nil, want error")
	}
}
