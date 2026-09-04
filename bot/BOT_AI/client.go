package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type lightsOutBoardPoster interface {
	PostLightsOutBoard(ctx context.Context, event createLightsOutEvent) error
}

type traQClient struct {
	baseURL                  string
	accessToken              string
	httpClient               *http.Client
	lightsOutBoardMessageIDs map[string]string
}

func newTraQClient(baseURL, accessToken string) *traQClient {
	return &traQClient{
		baseURL:     baseURL,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		lightsOutBoardMessageIDs: make(map[string]string),
	}
}

func (c *traQClient) PostMessage(ctx context.Context, channelID, content string) error {
	_, err := c.postMessage(ctx, channelID, content)
	return err
}

func (c *traQClient) postMessage(ctx context.Context, channelID, content string) (string, error) {
	body, err := json.Marshal(struct {
		Content string `json:"content"`
	}{Content: content})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	endpoint := c.baseURL + "/channels/" + url.PathEscape(channelID) + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return "", fmt.Errorf("post message: status=%d body=%q", res.StatusCode, string(responseBody))
	}
	var message struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&message); err != nil {
		return "", fmt.Errorf("decode posted message: %w", err)
	}
	if message.ID == "" {
		return "", errors.New("posted message response has no id")
	}
	return message.ID, nil
}

func (c *traQClient) PostLightsOutBoard(ctx context.Context, event createLightsOutEvent) error {
	stampIDs, err := c.getUnicodeStampIDs(ctx)
	if err != nil {
		return err
	}
	resolvedStampIDs := make([]string, 0, len(event.Channels))
	for _, channel := range event.Channels {
		stampID, ok := stampIDs[channel.StampName]
		if !ok {
			return fmt.Errorf("unicode stamp %q not found", channel.StampName)
		}
		resolvedStampIDs = append(resolvedStampIDs, stampID)
	}

	messageID, err := c.postMessage(ctx, event.BoardChannelID, formatLightsOutBoard(event))
	if err != nil {
		return err
	}
	for _, stampID := range resolvedStampIDs {
		if err := c.addMessageStamp(ctx, messageID, stampID); err != nil {
			return err
		}
	}
	previousMessageID := c.lightsOutBoardMessageIDs[event.RootChannelID]
	c.lightsOutBoardMessageIDs[event.RootChannelID] = messageID
	if previousMessageID != "" {
		if err := c.deleteMessage(ctx, previousMessageID); err != nil {
			return fmt.Errorf("delete previous lights out board: %w", err)
		}
	}
	return nil
}

func (c *traQClient) deleteMessage(ctx context.Context, messageID string) error {
	endpoint := c.baseURL + "/messages/" + url.PathEscape(messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create delete message request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("delete message: status=%d body=%q", res.StatusCode, string(responseBody))
	}
	return nil
}

func (c *traQClient) getUnicodeStampIDs(ctx context.Context) (map[string]string, error) {
	endpoint := c.baseURL + "/stamps?type=unicode"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create stamps request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get unicode stamps: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return nil, fmt.Errorf("get unicode stamps: status=%d body=%q", res.StatusCode, string(responseBody))
	}

	var stamps []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&stamps); err != nil {
		return nil, fmt.Errorf("decode unicode stamps: %w", err)
	}
	ids := make(map[string]string, len(stamps))
	for _, stamp := range stamps {
		ids[stamp.Name] = stamp.ID
	}
	return ids, nil
}

func (c *traQClient) addMessageStamp(ctx context.Context, messageID, stampID string) error {
	endpoint := c.baseURL + "/messages/" + url.PathEscape(messageID) + "/stamps/" + url.PathEscape(stampID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(`{"count":1}`))
	if err != nil {
		return fmt.Errorf("create add stamp request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("add message stamp: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("add message stamp: status=%d body=%q", res.StatusCode, string(responseBody))
	}
	return nil
}
