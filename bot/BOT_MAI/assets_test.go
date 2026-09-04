package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestPostQBotAssets(t *testing.T) {
	t.Parallel()
	client := newTraQClient("https://q.example.com/api/v3", "access-token")
	uploads, messages := 0, 0
	fileIDs := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/files":
			uploads++
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if r.FormValue("channelId") != "channel-id" {
				t.Fatalf("channelId = %q", r.FormValue("channelId"))
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			_ = file.Close()
			if header.Filename == "" || header.Size == 0 {
				t.Fatalf("invalid uploaded file: %#v", header)
			}
			return testResponse(http.StatusCreated, fmt.Sprintf(`{"id":"%s"}`, fileIDs[uploads-1])), nil
		case "/api/v3/channels/channel-id/messages":
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			want := "https://q.example.com/files/" + fileIDs[messages]
			if body.Content != want {
				t.Fatalf("content = %q, want %q", body.Content, want)
			}
			messages++
			return testResponse(http.StatusCreated, `{}`), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
			return nil, nil
		}
	})}

	if err := client.PostQBotAssets(context.Background(), "channel-id"); err != nil {
		t.Fatal(err)
	}
	if uploads != 2 || messages != 2 {
		t.Fatalf("uploads=%d messages=%d, want 2 each", uploads, messages)
	}
}
