package main

import "context"

func (e *commandExecutor) executeDebug(ctx context.Context, request commandRequest) (commandResult, error) {
	return e.executeRemote(ctx, request)
}
