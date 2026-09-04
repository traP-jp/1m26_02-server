package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTraQClientPostMessage(t *testing.T) {
	t.Parallel()

	client := newTraQClient("https://q.example.com/api/v3", "access-token")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/api/v3/channels/channel-id/messages" {
				t.Errorf("path = %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
				t.Errorf("Authorization = %q", got)
			}

			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body.Content != mentionReply {
				t.Errorf("content = %q, want %q", body.Content, mentionReply)
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{"id":"message-id"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	if err := client.PostMessage(context.Background(), "channel-id", mentionReply); err != nil {
		t.Fatalf("PostMessage() error = %v", err)
	}
}

func TestTraQClientPostLightsOutBoard(t *testing.T) {
	t.Parallel()

	var requests []string
	client := newTraQClient("https://q.example.com/api/v3", "access-token")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests = append(requests, r.Method+" "+r.URL.RequestURI())
			switch r.URL.Path {
			case "/api/v3/channels/general-id/messages":
				var body struct {
					Content string `json:"content"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode message request: %v", err)
				}
				if body.Content != ":thumbsup: #random\n:white_check_mark: #random/hoge" {
					t.Errorf("content = %q", body.Content)
				}
				return response(http.StatusCreated, `{"id":"message-id"}`), nil
			case "/api/v3/stamps":
				return response(http.StatusOK, `[{"id":"stamp-1","name":"thumbsup"},{"id":"stamp-2","name":"white_check_mark"}]`), nil
			case "/api/v3/messages/message-id/stamps/stamp-1", "/api/v3/messages/message-id/stamps/stamp-2":
				return response(http.StatusNoContent, ""), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		}),
	}
	event := createLightsOutEvent{
		RootChannelID:  "root-id",
		BoardChannelID: "general-id",
		Channels: []lightsOutChannel{
			{Path: "random", StampName: "thumbsup", Stamp: "👍"},
			{Path: "random/hoge", StampName: "white_check_mark", Stamp: "✅"},
		},
	}

	if err := client.PostLightsOutBoard(context.Background(), event); err != nil {
		t.Fatalf("PostLightsOutBoard() error = %v", err)
	}
	if len(requests) != 4 {
		t.Fatalf("requests = %v", requests)
	}
}

func TestTraQClientPostLightsOutBoardValidatesStampsBeforePosting(t *testing.T) {
	t.Parallel()

	client := newTraQClient("https://q.example.com/api/v3", "access-token")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/v3/stamps" {
				t.Fatalf("unexpected request before stamp validation: %s %s", r.Method, r.URL.String())
			}
			return response(http.StatusOK, `[{"id":"stamp-1","name":"thumbsup"}]`), nil
		}),
	}
	event := createLightsOutEvent{
		RootChannelID:  "root-id",
		BoardChannelID: "general-id",
		Channels: []lightsOutChannel{
			{Path: "random", StampName: "missing", Stamp: "❓"},
		},
	}

	err := client.PostLightsOutBoard(context.Background(), event)
	if err == nil || !strings.Contains(err.Error(), `unicode stamp "missing" not found`) {
		t.Fatalf("PostLightsOutBoard() error = %v", err)
	}
}

func TestTraQClientPostLightsOutBoardDeletesPreviousBoard(t *testing.T) {
	t.Parallel()

	var deletedMessageID string
	client := newTraQClient("https://q.example.com/api/v3", "access-token")
	client.lightsOutBoardMessageIDs["root-id"] = "old-message-id"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.Method + " " + r.URL.Path {
			case "GET /api/v3/stamps":
				return response(http.StatusOK, `[{"id":"stamp-1","name":"thumbsup"}]`), nil
			case "POST /api/v3/channels/general-id/messages":
				return response(http.StatusCreated, `{"id":"new-message-id"}`), nil
			case "POST /api/v3/messages/new-message-id/stamps/stamp-1":
				return response(http.StatusNoContent, ""), nil
			case "DELETE /api/v3/messages/old-message-id":
				deletedMessageID = "old-message-id"
				return response(http.StatusNoContent, ""), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		}),
	}

	err := client.PostLightsOutBoard(context.Background(), createLightsOutEvent{
		RootChannelID:  "root-id",
		BoardChannelID: "general-id",
		Channels: []lightsOutChannel{
			{Path: "random", StampName: "thumbsup", Stamp: "👍"},
		},
	})
	if err != nil {
		t.Fatalf("PostLightsOutBoard() error = %v", err)
	}
	if deletedMessageID != "old-message-id" {
		t.Errorf("deleted message = %q", deletedMessageID)
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
