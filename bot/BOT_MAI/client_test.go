package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	traqbot "github.com/traPtitech/traq-bot"
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
				Embed   bool   `json:"embed"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body.Content != "reset BOT" {
				t.Errorf("content = %q, want %q", body.Content, "reset BOT")
			}
			if !body.Embed {
				t.Error("embed = false, want true for plain content")
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	if err := client.PostMessage(context.Background(), "channel-id", "reset BOT"); err != nil {
		t.Fatalf("PostMessage() error = %v", err)
	}
}

func TestTraQClientPostMessageDoesNotReembedStructuredContent(t *testing.T) {
	t.Parallel()

	const content = `!{"type":"channel","raw":"#general","id":"01a06ce5-834e-7e96-a3a7-af9c14580f32"}`
	client := newTraQClient("https://q.example.com/api/v3", "access-token")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Content string `json:"content"`
			Embed   bool   `json:"embed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Content != content {
			t.Errorf("content = %q, want %q", body.Content, content)
		}
		if body.Embed {
			t.Error("embed = true, want false for structured content")
		}
		return testResponse(http.StatusCreated, ""), nil
	})}

	if err := client.PostMessage(context.Background(), "channel-id", content); err != nil {
		t.Fatalf("PostMessage() error = %v", err)
	}
}

func TestExpandInternalLinks(t *testing.T) {
	t.Parallel()
	got := expandInternalLinks("/messages/first\n/messages/second\nplain", "https://q.example.com")
	want := "https://q.example.com/messages/first\nhttps://q.example.com/messages/second\nplain"
	if got != want {
		t.Fatalf("expandInternalLinks() = %q, want %q", got, want)
	}
}

func TestShouldEmbedContent(t *testing.T) {
	t.Parallel()
	if !shouldEmbedContent("plain #general") {
		t.Error("plain content should use server-side embedding")
	}
	if shouldEmbedContent(`| channel |\n| !{"type":"channel","raw":"#general","id":"id"} |`) {
		t.Error("content containing a structured embed must not be embedded again")
	}
}

func TestTraQClientExecuteQBotCommand(t *testing.T) {
	t.Parallel()
	client := newTraQClient("https://q.example.com/api/v3", "access-token")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v3/qbot/commands" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["command"] != "open" || body["target"] != "user" || body["userId"] != "user-id" || body["messageId"] != "message-id" || body["channelId"] != "channel-id" {
			t.Fatalf("body = %#v", body)
		}
		return testResponse(http.StatusOK, `{"reply":"対象を開いています…","sendContent":""}`), nil
	})}

	result, err := client.ExecuteQBotCommand(context.Background(), commandRequest{
		Command: commandOpen,
		Target:  targetUser,
		Message: traqbot.MessagePayload{ID: "message-id", ChannelID: "channel-id", User: traqbot.UserPayload{ID: "user-id"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reply != "対象を開いています…" {
		t.Fatalf("result = %#v", result)
	}
}

func TestTraQClientGetQBotStateForUser(t *testing.T) {
	t.Parallel()
	client := newTraQClient("https://q.example.com/api/v3", "access-token")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v3/qbot/state" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("userId"); got != "user-id" {
			t.Fatalf("userId = %q, want user-id", got)
		}
		return testResponse(http.StatusOK, `{"cleared":true}`), nil
	})}

	state, err := client.GetQBotState(context.Background(), "user-id")
	if err != nil {
		t.Fatal(err)
	}
	if !state.Cleared {
		t.Fatal("Cleared = false, want true")
	}
}
