package main

import (
	"context"
	"errors"
	"fmt"
)

var (
	errUnknownCommand        = errors.New("unknown command")
	errUnknownTarget         = errors.New("unknown target")
	errCommandNotImplemented = errors.New("command is not implemented")
)

type commandHandler func(context.Context, commandRequest) error

type commandExecutor struct {
	handlers map[commandName]commandHandler
}

func newCommandExecutor() *commandExecutor {
	executor := &commandExecutor{}
	executor.handlers = map[commandName]commandHandler{
		commandCount:  executor.executeCount,
		commandList:   executor.executeList,
		commandOpen:   executor.executeOpen,
		commandSend:   executor.executeSend,
		commandDelete: executor.executeDelete,
		commandReset:  executor.executeReset,
		commandDebug:  executor.executeDebug,
	}
	return executor
}

func (e *commandExecutor) execute(ctx context.Context, request commandRequest) error {
	handler, ok := e.handlers[request.Command]
	if !ok {
		return fmt.Errorf("%w: %s", errUnknownCommand, request.Command)
	}
	if !request.Target.valid() {
		return fmt.Errorf("%w: %s", errUnknownTarget, request.Target)
	}
	return handler(ctx, request)
}

func notImplemented(request commandRequest) error {
	return fmt.Errorf("%w: %s %s", errCommandNotImplemented, request.Command, request.Target)
}
