package main

import (
	"context"
	"errors"
	"testing"

	traqbot "github.com/traPtitech/traq-bot"
)

func TestCommandExecutorRegistersEveryCommand(t *testing.T) {
	t.Parallel()

	executor := newCommandExecutor()
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
		err := executor.execute(context.Background(), commandRequest{Command: command, Target: targetBOT})
		if !errors.Is(err, errCommandNotImplemented) {
			t.Errorf("execute(%q) error = %v, want errCommandNotImplemented", command, err)
		}
	}
}

func TestCommandExecutorDispatchesRequest(t *testing.T) {
	t.Parallel()

	executor := newCommandExecutor()
	want := commandRequest{
		Command: commandCount,
		Target:  targetFile,
		Message: traqbot.MessagePayload{ID: "message-id", ChannelID: "channel-id"},
	}
	var got commandRequest
	executor.handlers[commandCount] = func(_ context.Context, request commandRequest) error {
		got = request
		return nil
	}

	if err := executor.execute(context.Background(), want); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if got.Command != want.Command || got.Target != want.Target || got.Message.ID != want.Message.ID || got.Message.ChannelID != want.Message.ChannelID {
		t.Errorf("dispatched request = %+v, want %+v", got, want)
	}
}

func TestCommandExecutorRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	executor := newCommandExecutor()
	if err := executor.execute(context.Background(), commandRequest{Command: "unknown", Target: targetBOT}); !errors.Is(err, errUnknownCommand) {
		t.Errorf("unknown command error = %v", err)
	}
	if err := executor.execute(context.Background(), commandRequest{Command: commandCount, Target: "unknown"}); !errors.Is(err, errUnknownTarget) {
		t.Errorf("unknown target error = %v", err)
	}
}
