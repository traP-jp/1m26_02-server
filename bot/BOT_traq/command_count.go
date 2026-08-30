package main

import "context"

func (e *commandExecutor) executeCount(_ context.Context, request commandRequest) error {
	return notImplemented(request)
}
