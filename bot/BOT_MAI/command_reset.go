package main

import "context"

func (e *commandExecutor) executeReset(ctx context.Context, request commandRequest) (commandResult, error) {
	return e.executeRemote(ctx, request)
}
