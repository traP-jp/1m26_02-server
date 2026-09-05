package v3

import (
	"context"
	"strings"
	"testing"

	"github.com/gofrs/uuid"

	"github.com/traPtitech/traQ/model"
)

func TestQBotCommandMatrix(t *testing.T) {
	commands := []string{"count", "list", "open", "send", "delete", "reset", "debug"}
	targets := []string{"BOT", "message", "user", "channel", "stamp", "file", "image"}

	combinations := 0
	for _, command := range commands {
		if !validQBotCommand(command) {
			t.Fatalf("valid command %q was rejected", command)
		}
		for _, target := range targets {
			if !validQBotTarget(target) {
				t.Fatalf("valid target %q was rejected", target)
			}
			combinations++
		}
	}
	if combinations != 49 {
		t.Fatalf("command matrix has %d combinations, want 49", combinations)
	}

	for _, command := range []string{"", "COUNT", "unknown"} {
		if validQBotCommand(command) {
			t.Errorf("invalid command %q was accepted", command)
		}
	}
	for _, target := range []string{"", "bot", "unknown"} {
		if validQBotTarget(target) {
			t.Errorf("invalid target %q was accepted", target)
		}
	}
}

func TestQBotFormatting(t *testing.T) {
	if got := qBotCell("a|b\nc"); got != `a\|b c` {
		t.Errorf("qBotCell() = %q", got)
	}
	if got := qBotOneLine("  one\n two   three  ", 7); got != "one two…" {
		t.Errorf("qBotOneLine() = %q", got)
	}
	if got := qBotPathWithinUserRoot("alice/general/2", "alice"); got != "general/2" {
		t.Errorf("qBotPathWithinUserRoot() = %q", got)
	}
	channelID := uuid.Must(uuid.FromString("01a06ce5-834e-7e96-a3a7-af9c14580f32"))
	if got := qBotChannelEmbed("#general", channelID); got != `!{"type":"channel","raw":"#general","id":"01a06ce5-834e-7e96-a3a7-af9c14580f32"}` {
		t.Errorf("qBotChannelEmbed() = %q", got)
	}
}

func TestQBotChannelsUnderRootExcludesRootAndArchivedChannels(t *testing.T) {
	rootID := uuid.Must(uuid.NewV7())
	generalID := uuid.Must(uuid.NewV7())
	channels := []*model.Channel{
		{ID: rootID, Name: "alice", IsVisible: true},
		{ID: generalID, ParentID: rootID, Name: "general", IsVisible: true},
		{ID: uuid.Must(uuid.NewV7()), ParentID: generalID, Name: "1", IsVisible: true},
		{ID: uuid.Must(uuid.NewV7()), ParentID: rootID, Name: "archived", IsVisible: false},
		{ID: uuid.Must(uuid.NewV7()), Name: "someone-else", IsVisible: true},
	}

	got := qBotChannelsUnderRoot(channels, "alice")
	if len(got) != 2 {
		t.Fatalf("qBotChannelsUnderRoot() returned %d channels, want 2", len(got))
	}
	if got[0].path != "general" || got[1].path != "general/1" {
		t.Fatalf("paths = [%q, %q], want [general, general/1]", got[0].path, got[1].path)
	}
}

func TestExecuteQBotDebugOmitsParserAndType(t *testing.T) {
	h := &Handlers{}
	response, err := h.executeQBotDebug(
		context.Background(),
		qBotCommandRequest{Target: "message"},
		&model.Message{Text: "test message"},
	)
	if err != nil {
		t.Fatalf("executeQBotDebug() returned an error: %v", err)
	}
	for _, field := range []string{`"parser"`, `"type"`} {
		if strings.Contains(response.Reply, field) {
			t.Errorf("executeQBotDebug() unexpectedly included %s: %s", field, response.Reply)
		}
	}
	if !strings.Contains(response.Reply, `"length": 12`) {
		t.Errorf("executeQBotDebug() omitted target-specific data: %s", response.Reply)
	}
}

func TestExecuteQBotDebugEmptyData(t *testing.T) {
	h := &Handlers{}
	response, err := h.executeQBotDebug(
		context.Background(),
		qBotCommandRequest{},
		&model.Message{},
	)
	if err != nil {
		t.Fatalf("executeQBotDebug() returned an error: %v", err)
	}
	if response.Reply != "```json\n{}\n```" {
		t.Errorf("executeQBotDebug() = %q, want an empty JSON object", response.Reply)
	}
}
