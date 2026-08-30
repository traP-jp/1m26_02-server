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
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	if err := client.PostMessage(context.Background(), "channel-id", mentionReply); err != nil {
		t.Fatalf("PostMessage() error = %v", err)
	}
}
