package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type messagePoster interface {
	PostMessage(ctx context.Context, channelID, content string) error
}

type traQClient struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

func newTraQClient(baseURL, accessToken string) *traQClient {
	return &traQClient{
		baseURL:     baseURL,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (c *traQClient) PostMessage(ctx context.Context, channelID, content string) error {
	if strings.HasPrefix(content, "/") {
		content = strings.TrimSuffix(c.baseURL, "/api/v3") + content
	}
	body, err := json.Marshal(struct {
		Content string `json:"content"`
		Embed   bool   `json:"embed"`
	}{Content: content, Embed: true})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	endpoint := c.baseURL + "/channels/" + url.PathEscape(channelID) + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("post message: status=%d body=%q", res.StatusCode, string(responseBody))
	}
	return nil
}

func (c *traQClient) ExecuteQBotCommand(ctx context.Context, request commandRequest) (commandResult, error) {
	body, err := json.Marshal(struct {
		Command   commandName `json:"command"`
		Target    targetName  `json:"target"`
		UserID    string      `json:"userId"`
		MessageID string      `json:"messageId"`
		ChannelID string      `json:"channelId"`
	}{
		Command: request.Command, Target: request.Target,
		UserID: request.Message.User.ID, MessageID: request.Message.ID, ChannelID: request.Message.ChannelID,
	})
	if err != nil {
		return commandResult{}, fmt.Errorf("encode q_bot command: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/qbot/commands", bytes.NewReader(body))
	if err != nil {
		return commandResult{}, fmt.Errorf("create q_bot command request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return commandResult{}, fmt.Errorf("execute q_bot command: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return commandResult{}, fmt.Errorf("execute q_bot command: status=%d body=%q", res.StatusCode, string(responseBody))
	}
	var result commandResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return commandResult{}, fmt.Errorf("decode q_bot command: %w", err)
	}
	return result, nil
}

func (c *traQClient) GetQBotState(ctx context.Context, userID string) (qBotState, error) {
	endpoint := c.baseURL + "/qbot/state?userId=" + url.QueryEscape(userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return qBotState{}, fmt.Errorf("create q_bot state request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return qBotState{}, fmt.Errorf("get q_bot state: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return qBotState{}, fmt.Errorf("get q_bot state: status=%d body=%q", res.StatusCode, string(responseBody))
	}
	var state qBotState
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		return qBotState{}, fmt.Errorf("decode q_bot state: %w", err)
	}
	return state, nil
}
