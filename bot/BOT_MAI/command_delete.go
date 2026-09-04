package main

import "context"

func (e *commandExecutor) executeDelete(ctx context.Context, request commandRequest) (commandResult, error) {
	return e.executeRemote(ctx, request)
}
