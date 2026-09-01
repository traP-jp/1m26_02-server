package main

import (
	"context"
	"errors"
	"testing"

	traqbot "github.com/traPtitech/traq-bot"
)

type recordingRunner struct {
	requests []commandRequest
}

func (r *recordingRunner) ExecuteQBotCommand(_ context.Context, request commandRequest) (commandResult, error) {
	r.requests = append(r.requests, request)
	return commandResult{Reply: "ok"}, nil
}

func TestCommandExecutorRegistersEveryCommand(t *testing.T) {
	t.Parallel()

	executor := newCommandExecutor(nil)
	commands := []commandName{
		commandCount,
		commandList,
		commandOpen,
		commandSend,
		commandDelete,
		commandReset,
		commandDebug,
	}
	if len(executor.handlers) != len(commands) {
		t.Fatalf("handler count = %d, want %d", len(executor.handlers), len(commands))
	}
	for _, command := range commands {
		if executor.handlers[command] == nil {
			t.Errorf("handler for %q is not registered", command)
		}
		_, err := executor.execute(context.Background(), commandRequest{Command: command, Target: targetBOT})
		if !errors.Is(err, errCommandNotImplemented) {
			t.Errorf("execute(%q) error = %v, want errCommandNotImplemented", command, err)
		}
	}
}

func TestCommandExecutorDispatchesAll49Combinations(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	executor := newCommandExecutor(runner)
	commands := []commandName{commandCount, commandList, commandOpen, commandSend, commandDelete, commandReset, commandDebug}
	targets := []targetName{targetBOT, targetMessage, targetUser, targetChannel, targetStamp, targetFile, targetImage}
	for _, command := range commands {
		for _, target := range targets {
			result, err := executor.execute(context.Background(), commandRequest{Command: command, Target: target})
			if err != nil {
				t.Fatalf("execute(%s %s): %v", command, target, err)
			}
			if result.Reply != "ok" {
				t.Fatalf("execute(%s %s) reply = %q", command, target, result.Reply)
			}
		}
	}
	if len(runner.requests) != 49 {
		t.Fatalf("request count = %d, want 49", len(runner.requests))
	}
}

func TestCommandExecutorDispatchesRequest(t *testing.T) {
	t.Parallel()

	executor := newCommandExecutor(nil)
	want := commandRequest{
		Command: commandCount,
		Target:  targetFile,
		Message: traqbot.MessagePayload{ID: "message-id", ChannelID: "channel-id"},
	}
	var got commandRequest
	executor.handlers[commandCount] = func(_ context.Context, request commandRequest) (commandResult, error) {
		got = request
		return commandResult{Reply: "ok"}, nil
	}

	if _, err := executor.execute(context.Background(), want); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if got.Command != want.Command || got.Target != want.Target || got.Message.ID != want.Message.ID || got.Message.ChannelID != want.Message.ChannelID {
		t.Errorf("dispatched request = %+v, want %+v", got, want)
	}
}

func TestCommandExecutorRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	executor := newCommandExecutor(nil)
	if _, err := executor.execute(context.Background(), commandRequest{Command: "unknown", Target: targetBOT}); !errors.Is(err, errUnknownCommand) {
		t.Errorf("unknown command error = %v", err)
	}
	if _, err := executor.execute(context.Background(), commandRequest{Command: commandCount, Target: "unknown"}); !errors.Is(err, errUnknownTarget) {
		t.Errorf("unknown target error = %v", err)
	}
}
