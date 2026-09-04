package main

import "testing"

func TestMakeWebSocketURL(t *testing.T) {
	t.Parallel()

	got, err := makeWebSocketURL("https://q.example.com/api/v3")
	if err != nil {
		t.Fatalf("makeWebSocketURL() error = %v", err)
	}
	if got != "wss://q.example.com/api/v3/bots/ws" {
		t.Errorf("makeWebSocketURL() = %q", got)
	}
}

func TestFormatLightsOutBoard(t *testing.T) {
	t.Parallel()

	event := createLightsOutEvent{Channels: []lightsOutChannel{
		{Path: "random", StampName: "thumbsup", Stamp: "👍"},
		{Path: "random/hoge", StampName: "white_check_mark", Stamp: "✅"},
	}}
	if got := formatLightsOutBoard(event); got != ":thumbsup: #random\n:white_check_mark: #random/hoge" {
		t.Errorf("formatLightsOutBoard() = %q", got)
	}
}
