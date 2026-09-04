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

type commandHandler func(context.Context, commandRequest) (commandResult, error)

type commandRunner interface {
	ExecuteQBotCommand(context.Context, commandRequest) (commandResult, error)
}

type commandExecutor struct {
	handlers map[commandName]commandHandler
	runner   commandRunner
}

func newCommandExecutor(runner commandRunner) *commandExecutor {
	executor := &commandExecutor{runner: runner}
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

func (e *commandExecutor) execute(ctx context.Context, request commandRequest) (commandResult, error) {
	handler, ok := e.handlers[request.Command]
	if !ok {
		return commandResult{}, fmt.Errorf("%w: %s", errUnknownCommand, request.Command)
	}
	if !request.Target.valid() {
		return commandResult{}, fmt.Errorf("%w: %s", errUnknownTarget, request.Target)
	}
	return handler(ctx, request)
}

func (e *commandExecutor) executeRemote(ctx context.Context, request commandRequest) (commandResult, error) {
	if e.runner == nil {
		return commandResult{}, fmt.Errorf("%w: %s %s", errCommandNotImplemented, request.Command, request.Target)
	}
	return e.runner.ExecuteQBotCommand(ctx, request)
}
