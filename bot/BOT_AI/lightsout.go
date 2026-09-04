package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const createLightsOutEventType = "CREATE_LIGHTS_OUT"

type lightsOutChannel struct {
	ID        string   `json:"id"`
	Path      string   `json:"path"`
	ParentID  string   `json:"parent_id"`
	Children  []string `json:"children"`
	StampName string   `json:"stamp_name"`
	Stamp     string   `json:"stamp"`
}

type createLightsOutEvent struct {
	RootChannelID  string             `json:"root_channel_id"`
	BoardChannelID string             `json:"board_channel_id"`
	Channels       []lightsOutChannel `json:"channels"`
}

type websocketMessage struct {
	Type string          `json:"type"`
	Body json.RawMessage `json:"body"`
}

func formatLightsOutBoard(event createLightsOutEvent) string {
	lines := make([]string, 0, len(event.Channels))
	for _, channel := range event.Channels {
		lines = append(lines, fmt.Sprintf(":%s: #%s", channel.StampName, channel.Path))
	}
	return strings.Join(lines, "\n")
}

func runLightsOutWebSocket(ctx context.Context, apiBaseURL, accessToken string, poster lightsOutBoardPoster, logger *log.Logger) {
	websocketURL, err := makeWebSocketURL(apiBaseURL)
	if err != nil {
		logger.Printf("invalid websocket URL: %v", err)
		return
	}

	for ctx.Err() == nil {
		header := http.Header{"Authorization": []string{"Bearer " + accessToken}}
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, websocketURL, header)
		if err != nil {
			logger.Printf("failed to connect websocket: %v", err)
			waitForReconnect(ctx)
			continue
		}
		logger.Printf("connected to traQ websocket")

		closed := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = conn.Close()
			case <-closed:
			}
		}()
		err = readLightsOutEvents(ctx, conn, poster, logger)
		close(closed)
		_ = conn.Close()
		if ctx.Err() == nil {
			logger.Printf("websocket disconnected: %v", err)
			waitForReconnect(ctx)
		}
	}
}

func makeWebSocketURL(apiBaseURL string) (string, error) {
	u, err := url.Parse(apiBaseURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/bots/ws"
	return u.String(), nil
}

func readLightsOutEvents(ctx context.Context, conn *websocket.Conn, poster lightsOutBoardPoster, logger *log.Logger) error {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var message websocketMessage
		if err := json.Unmarshal(data, &message); err != nil || message.Type != createLightsOutEventType {
			continue
		}
		var event createLightsOutEvent
		if err := json.Unmarshal(message.Body, &event); err != nil {
			logger.Printf("failed to decode %s event: %v", createLightsOutEventType, err)
			continue
		}

		requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = poster.PostLightsOutBoard(requestCtx, event)
		cancel()
		if err != nil {
			logger.Printf("failed to post lights out board for %s: %v", event.RootChannelID, err)
			continue
		}
		logger.Printf("posted lights out board for %s", event.RootChannelID)
	}
}

func waitForReconnect(ctx context.Context) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
