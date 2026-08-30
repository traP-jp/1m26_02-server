package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	body, err := json.Marshal(struct {
		Content string `json:"content"`
	}{Content: content})
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
