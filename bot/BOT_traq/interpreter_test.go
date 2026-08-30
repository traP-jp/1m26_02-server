package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestKnightDestinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		word     string
		commands []string
		targets  []string
	}{
		{word: "count", commands: []string{"delete"}, targets: []string{"BOT", "stamp"}},
		{word: "list", commands: []string{"open"}, targets: []string{"user", "channel"}},
		{word: "debug", commands: []string{"send"}, targets: []string{"stamp"}},
		{word: "open", commands: []string{"list"}, targets: []string{"stamp", "image"}},
		{word: "send", commands: []string{"debug"}, targets: []string{"channel", "message"}},
		{word: "reset", commands: []string{}, targets: []string{"BOT", "image", "file"}},
		{word: "delete", commands: []string{"count"}, targets: []string{"user", "message"}},
		{word: "BOT", commands: []string{"count", "reset"}, targets: []string{"message"}},
		{word: "user", commands: []string{"list", "delete"}, targets: []string{"file"}},
		{word: "stamp", commands: []string{"count", "debug", "open"}, targets: []string{}},
		{word: "channel", commands: []string{"list", "send"}, targets: []string{"image"}},
		{word: "image", commands: []string{"open", "reset"}, targets: []string{"channel"}},
		{word: "message", commands: []string{"send", "delete"}, targets: []string{"BOT"}},
		{word: "file", commands: []string{"reset"}, targets: []string{"user"}},
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			commands, err := knightDestinations(tt.word, commandWord)
			if err != nil {
				t.Fatalf("command destinations error = %v", err)
			}
			if !reflect.DeepEqual(commands, tt.commands) {
				t.Errorf("command destinations = %v, want %v", commands, tt.commands)
			}

			targets, err := knightDestinations(tt.word, targetWord)
			if err != nil {
				t.Fatalf("target destinations error = %v", err)
			}
			if !reflect.DeepEqual(targets, tt.targets) {
				t.Errorf("target destinations = %v, want %v", targets, tt.targets)
			}
		})
	}
}

func TestInterpret(t *testing.T) {
	t.Parallel()

	t.Run("single interpretation", func(t *testing.T) {
		got, err := interpret("file", "message")
		if err != nil {
			t.Fatalf("interpret() error = %v", err)
		}
		want := []string{"reset BOT"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("interpretations = %v, want %v", got, want)
		}
	})

	t.Run("multiple interpretations", func(t *testing.T) {
		got, err := interpret("BOT", "count")
		if err != nil {
			t.Fatalf("interpret() error = %v", err)
		}
		want := []string{"count BOT", "count stamp", "reset BOT", "reset stamp"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("interpretations = %v, want %v", got, want)
		}
	})

	t.Run("no command destination", func(t *testing.T) {
		_, err := interpret("reset", "message")
		if !errors.Is(err, errNoInterpretation) {
			t.Errorf("error = %v, want errNoInterpretation", err)
		}
	})

	t.Run("no target destination", func(t *testing.T) {
		_, err := interpret("file", "stamp")
		if !errors.Is(err, errNoInterpretation) {
			t.Errorf("error = %v, want errNoInterpretation", err)
		}
	})

	t.Run("unknown word", func(t *testing.T) {
		if _, err := interpret("unknown", "message"); err == nil {
			t.Fatal("interpret() error = nil, want error")
		}
	})
}

func TestInterpretMessage(t *testing.T) {
	t.Parallel()

	got, err := interpretMessage("@BOT_traq file message")
	if err != nil {
		t.Fatalf("interpretMessage() error = %v", err)
	}
	if want := []string{"reset BOT"}; !reflect.DeepEqual(got, want) {
		t.Errorf("interpretations = %v, want %v", got, want)
	}

	if _, err := interpretMessage("@BOT_traq file"); !errors.Is(err, errInvalidArgumentCount) {
		t.Errorf("error = %v, want errInvalidArgumentCount", err)
	}
	if _, err := interpretMessage("@BOT_traq file message extra"); !errors.Is(err, errInvalidArgumentCount) {
		t.Errorf("error = %v, want errInvalidArgumentCount", err)
	}
}
