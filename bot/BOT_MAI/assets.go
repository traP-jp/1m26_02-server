package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const postQBotAssetsEventType = "POST_QBOT_ASSETS"

//go:embed BOTコマンド仕様書.png
var commandGuidePNG []byte

//go:embed BOTコマンド仕様書.pdf
var commandGuidePDF []byte

type qBotAssetsEvent struct {
	ChannelID string `json:"channel_id"`
}

type botWebSocketMessage struct {
	Type string          `json:"type"`
	Body json.RawMessage `json:"body"`
}

func runQBotWebSocket(ctx context.Context, client *traQClient, logger *log.Logger) {
	u, err := url.Parse(client.baseURL)
	if err != nil {
		logger.Printf("invalid websocket URL: %v", err)
		return
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else {
		u.Scheme = "wss"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/bots/ws"
	for ctx.Err() == nil {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), http.Header{"Authorization": {"Bearer " + client.accessToken}})
		if err != nil {
			logger.Printf("failed to connect websocket: %v", err)
			time.Sleep(time.Second)
			continue
		}
		logger.Printf("connected to traQ websocket")
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var message botWebSocketMessage
			if json.Unmarshal(data, &message) != nil || message.Type != postQBotAssetsEventType {
				continue
			}
			var event qBotAssetsEvent
			if err := json.Unmarshal(message.Body, &event); err != nil {
				logger.Printf("failed to decode asset event: %v", err)
				continue
			}
			requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err = client.PostQBotAssets(requestCtx, event.ChannelID)
			cancel()
			if err != nil {
				logger.Printf("failed to post qbot assets: %v", err)
			}
		}
		_ = conn.Close()
	}
}

func (c *traQClient) PostQBotAssets(ctx context.Context, channelID string) error {
	for _, asset := range []struct {
		name, contentType string
		data              []byte
	}{
		{"BOTコマンド仕様書.png", "image/png", commandGuidePNG},
		{"BOTコマンド仕様書.pdf", "application/pdf", commandGuidePDF},
	} {
		fileID, err := c.uploadFile(ctx, channelID, asset.name, asset.contentType, asset.data)
		if err != nil {
			return err
		}
		content := c.publicOrigin + "/files/" + fileID
		if err := c.PostMessage(ctx, channelID, content); err != nil {
			return err
		}
	}
	return nil
}

func (c *traQClient) uploadFile(ctx context.Context, channelID, name, contentType string, data []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, name))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := writer.WriteField("channelId", channelID); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/files", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return "", fmt.Errorf("upload %s: status=%d body=%q", name, res.StatusCode, responseBody)
	}
	var uploaded struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&uploaded); err != nil {
		return "", err
	}
	return uploaded.ID, nil
}
