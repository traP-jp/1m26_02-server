package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	devBotName        = "AI"
	devBotUserName    = "BOT_" + devBotName
	devBotDisplayName = "アイ"
	devBotDescription = "アイ development bot"
	mentionEvent      = "MENTION_MESSAGE_CREATED"
)

type provisionedBot struct {
	ID                string
	AccessToken       string
	VerificationToken string
}

type devProvisioner struct {
	baseURL         string
	botEndpoint     string
	devUserName     string
	devUserPassword string
	httpClient      *http.Client
}

type apiUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiBot struct {
	ID              string   `json:"id"`
	BotUserID       string   `json:"botUserId"`
	DisplayName     string   `json:"displayName"`
	Endpoint        string   `json:"endpoint"`
	SubscribeEvents []string `json:"subscribeEvents"`
	Tokens          struct {
		VerificationToken string `json:"verificationToken"`
		AccessToken       string `json:"accessToken"`
	} `json:"tokens"`
}

func newDevProvisioner(cfg config) (*devProvisioner, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &devProvisioner{
		baseURL:         cfg.APIBaseURL,
		botEndpoint:     cfg.BotEndpoint,
		devUserName:     cfg.DevUserName,
		devUserPassword: cfg.DevUserPassword,
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 5 * time.Second,
		},
	}, nil
}

func provisionDevBot(ctx context.Context, cfg config, logger *log.Logger) (*devProvisioner, provisionedBot, error) {
	var lastErr error
	for {
		provisioner, err := newDevProvisioner(cfg)
		if err == nil {
			var bot provisionedBot
			bot, err = provisioner.provision(ctx)
			if err == nil {
				return provisioner, bot, nil
			}
		}
		lastErr = err
		logger.Printf("waiting for traQ dev setup: %v", err)

		select {
		case <-ctx.Done():
			return nil, provisionedBot{}, fmt.Errorf("provision development bot: %w (last error: %v)", ctx.Err(), lastErr)
		case <-time.After(2 * time.Second):
		}
	}
}

func (p *devProvisioner) provision(ctx context.Context) (provisionedBot, error) {
	if err := p.ensureDevUser(ctx); err != nil {
		return provisionedBot{}, err
	}
	if err := p.login(ctx); err != nil {
		return provisionedBot{}, err
	}

	bot, created, err := p.findOrCreateBot(ctx)
	if err != nil {
		return provisionedBot{}, err
	}
	if !created {
		var detail apiBot
		if err := p.doJSON(ctx, http.MethodGet, "/bots/"+url.PathEscape(bot.ID)+"?detail=true", nil, []int{http.StatusOK}, &detail); err != nil {
			return provisionedBot{}, fmt.Errorf("get existing bot detail: %w", err)
		}
		bot = detail
	}
	if err := p.ensureConfiguration(ctx, &bot); err != nil {
		return provisionedBot{}, err
	}
	return bot.provisioned(), nil
}

func (p *devProvisioner) ensureDevUser(ctx context.Context) error {
	body := map[string]string{
		"name":     p.devUserName,
		"password": p.devUserPassword,
	}
	if err := p.doJSON(ctx, http.MethodPost, "/users", body, []int{http.StatusCreated, http.StatusConflict}, nil); err != nil {
		return fmt.Errorf("create development user: %w", err)
	}
	return nil
}

func (p *devProvisioner) login(ctx context.Context) error {
	body := map[string]string{
		"name":     p.devUserName,
		"password": p.devUserPassword,
	}
	if err := p.doJSON(ctx, http.MethodPost, "/login", body, []int{http.StatusNoContent}, nil); err != nil {
		return fmt.Errorf("login development user: %w", err)
	}
	return nil
}

func (p *devProvisioner) findOrCreateBot(ctx context.Context) (apiBot, bool, error) {
	var users []apiUser
	path := "/users?name=" + url.QueryEscape(devBotUserName)
	if err := p.doJSON(ctx, http.MethodGet, path, nil, []int{http.StatusOK}, &users); err != nil {
		return apiBot{}, false, fmt.Errorf("find bot user: %w", err)
	}

	if len(users) == 0 {
		var bot apiBot
		body := map[string]string{
			"name":        devBotName,
			"displayName": devBotDisplayName,
			"description": devBotDescription,
			"mode":        "HTTP",
			"endpoint":    p.botEndpoint,
		}
		if err := p.doJSON(ctx, http.MethodPost, "/bots", body, []int{http.StatusCreated}, &bot); err != nil {
			return apiBot{}, false, fmt.Errorf("create bot: %w", err)
		}
		return bot, true, nil
	}
	if len(users) != 1 {
		return apiBot{}, false, fmt.Errorf("found %d users named %s", len(users), devBotUserName)
	}

	var bots []apiBot
	if err := p.doJSON(ctx, http.MethodGet, "/bots", nil, []int{http.StatusOK}, &bots); err != nil {
		return apiBot{}, false, fmt.Errorf("list owned bots: %w", err)
	}
	for _, bot := range bots {
		if bot.BotUserID == users[0].ID {
			return bot, false, nil
		}
	}
	return apiBot{}, false, fmt.Errorf("%s exists but is not owned by development user %s", devBotUserName, p.devUserName)
}

func (p *devProvisioner) ensureConfiguration(ctx context.Context, bot *apiBot) error {
	body := make(map[string]any)
	if bot.DisplayName != devBotDisplayName {
		body["displayName"] = devBotDisplayName
		bot.DisplayName = devBotDisplayName
	}
	hasMentionEvent := false
	for _, event := range bot.SubscribeEvents {
		if event == mentionEvent {
			hasMentionEvent = true
			break
		}
	}
	if !hasMentionEvent {
		events := append(append([]string(nil), bot.SubscribeEvents...), mentionEvent)
		sort.Strings(events)
		body["subscribeEvents"] = events
		bot.SubscribeEvents = events
	}
	if bot.Endpoint != p.botEndpoint {
		body["endpoint"] = p.botEndpoint
		bot.Endpoint = p.botEndpoint
	}
	if len(body) == 0 {
		return nil
	}
	if err := p.doJSON(ctx, http.MethodPatch, "/bots/"+url.PathEscape(bot.ID), body, []int{http.StatusNoContent}, nil); err != nil {
		return fmt.Errorf("configure bot: %w", err)
	}
	return nil
}

func (p *devProvisioner) activate(ctx context.Context, botID string) error {
	path := "/bots/" + url.PathEscape(botID) + "/actions/activate"
	if err := p.doJSON(ctx, http.MethodPost, path, nil, []int{http.StatusAccepted}, nil); err != nil {
		return fmt.Errorf("activate bot: %w", err)
	}
	return nil
}

func (p *devProvisioner) doJSON(ctx context.Context, method, path string, body any, accepted []int, result any) error {
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, requestBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	for _, status := range accepted {
		if res.StatusCode == status {
			if result == nil || res.StatusCode == http.StatusNoContent {
				return nil
			}
			if err := json.NewDecoder(res.Body).Decode(result); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			return nil
		}
	}
	responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	return fmt.Errorf("%s %s: status=%d body=%q", method, path, res.StatusCode, strings.TrimSpace(string(responseBody)))
}

func (b apiBot) provisioned() provisionedBot {
	return provisionedBot{
		ID:                b.ID,
		AccessToken:       b.Tokens.AccessToken,
		VerificationToken: b.Tokens.VerificationToken,
	}
}
