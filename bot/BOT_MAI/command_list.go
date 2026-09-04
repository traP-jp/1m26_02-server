package main

import "context"

func (e *commandExecutor) executeList(ctx context.Context, request commandRequest) (commandResult, error) {
	return e.executeRemote(ctx, request)
}
