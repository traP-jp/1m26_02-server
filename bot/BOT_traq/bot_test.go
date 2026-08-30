package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	traqbot "github.com/traPtitech/traq-bot"
)

type recordingPoster struct {
	channelID string
	content   string
}

func (p *recordingPoster) PostMessage(_ context.Context, channelID, content string) error {
	p.channelID = channelID
	p.content = content
	return nil
}

type postedMessage struct {
	channelID string
	content   string
}

type channelPoster chan postedMessage

func (p channelPoster) PostMessage(ctx context.Context, channelID, content string) error {
	select {
	case p <- postedMessage{channelID: channelID, content: content}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestBotHandleMention(t *testing.T) {
	t.Parallel()

	poster := &recordingPoster{}
	b := &bot{poster: poster, logger: log.New(io.Discard, "", 0)}
	b.handleMention(&traqbot.MessageCreatedPayload{
		Message: traqbot.MessagePayload{
			ID:        "message-id",
			ChannelID: "channel-id",
			PlainText: "@BOT_traq file message",
		},
	})

	if poster.channelID != "channel-id" {
		t.Errorf("channelID = %q", poster.channelID)
	}
	if poster.content != "reset BOT" {
		t.Errorf("content = %q, want %q", poster.content, "reset BOT")
	}
}

func TestEventHandlersRegistersMessageCreated(t *testing.T) {
	t.Parallel()

	b := &bot{poster: &recordingPoster{}, logger: log.New(io.Discard, "", 0)}
	handlers := newEventHandlers(b)
	if handlers[traqbot.MessageCreated] == nil {
		t.Fatal("MESSAGE_CREATED handler is not registered")
	}
}

func TestMentionEventRepliesToSourceChannel(t *testing.T) {
	t.Parallel()

	poster := make(channelPoster, 1)
	b := &bot{poster: poster, logger: log.New(io.Discard, "", 0)}
	server := traqbot.NewBotServer("verification-token", newEventHandlers(b))

	body := `{
  "eventTime": "2026-08-30T00:00:00Z",
  "message": {
    "id": "message-id",
    "channelId": "channel-id",
    "text": "@BOT_traq file message",
    "plainText": "@BOT_traq file message"
  }
}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TRAQ-BOT-EVENT", traqbot.MessageCreated)
	req.Header.Set("X-TRAQ-BOT-TOKEN", "verification-token")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}

	select {
	case posted := <-poster:
		if posted.channelID != "channel-id" {
			t.Errorf("channelID = %q", posted.channelID)
		}
		if posted.content != "reset BOT" {
			t.Errorf("content = %q, want %q", posted.content, "reset BOT")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for mention reply")
	}
}

func TestInterpretationReply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		plainText string
		want      string
	}{
		{name: "single", plainText: "@BOT_traq file message", want: "reset BOT"},
		{name: "multiple", plainText: "@BOT_traq BOT count", want: "- count BOT\n- count stamp\n- reset BOT\n- reset stamp"},
		{name: "wrong argument count", plainText: "@BOT_traq file", want: "引数を2つ指定してください。例: @BOT_traq file message"},
		{name: "no interpretation", plainText: "@BOT_traq reset message", want: "構文エラー: ナイト移動後に有効な解釈がありません。"},
		{name: "unknown word", plainText: "@BOT_traq unknown message", want: "構文エラー: unknown word \"unknown\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := interpretationReply(tt.plainText); got != tt.want {
				t.Errorf("interpretationReply() = %q, want %q", got, tt.want)
			}
		})
	}
}
