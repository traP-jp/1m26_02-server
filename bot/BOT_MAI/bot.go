package main

import (
	"context"
	"errors"
	"log"
	mathrand "math/rand/v2"
	"strings"
	"time"

	traqbot "github.com/traPtitech/traq-bot"
)

type bot struct {
	poster   messagePoster
	executor *commandExecutor
	state    qBotStateReader
	logger   *log.Logger
}

type qBotStateReader interface {
	GetQBotState(context.Context, string) (qBotState, error)
}

func newEventHandlers(b *bot) traqbot.EventHandlers {
	handlers := traqbot.EventHandlers{}
	handlers.SetMessageCreatedHandler(func(payload *traqbot.MessageCreatedPayload) {
		// traQへのイベント応答を待たせないよう、メッセージ投稿は非同期で行う。
		go b.handleMention(payload)
	})
	return handlers
}

func (b *bot) handleMention(payload *traqbot.MessageCreatedPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cleared := false
	if b.state != nil {
		state, err := b.state.GetQBotState(ctx, payload.Message.User.ID)
		if err != nil {
			b.logger.Printf("failed to get q_bot state for user %s: %v", payload.Message.User.ID, err)
			_ = b.poster.PostMessage(ctx, payload.Message.ChannelID, "処理中にエラーが発生しました。")
			return
		}
		cleared = state.Cleared
	}
	arguments := messageArguments(payload.Message.PlainText)
	if len(arguments) != 2 {
		if postErr := b.poster.PostMessage(ctx, payload.Message.ChannelID, interpretationErrorReply(errInvalidArgumentCount)); postErr != nil {
			b.logger.Printf("failed to reply to mention message %s: %v", payload.Message.ID, postErr)
		}
		return
	}
	selected := strings.Join(arguments, " ")
	if !cleared {
		interpretations, err := interpret(arguments[0], arguments[1])
		if err != nil {
			if postErr := b.poster.PostMessage(ctx, payload.Message.ChannelID, interpretationErrorReply(err)); postErr != nil {
				b.logger.Printf("failed to reply to mention message %s: %v", payload.Message.ID, postErr)
			}
			return
		}
		selected = selectInterpretation(interpretations)
	}
	parts := strings.SplitN(selected, " ", 2)
	result, err := b.executor.execute(ctx, commandRequest{Command: commandName(parts[0]), Target: targetName(parts[1]), Message: payload.Message})
	if err != nil {
		b.logger.Printf("failed to execute %s for message %s: %v", selected, payload.Message.ID, err)
		_ = b.poster.PostMessage(ctx, payload.Message.ChannelID, "処理中にエラーが発生しました。")
		return
	}
	if err := b.poster.PostMessage(ctx, payload.Message.ChannelID, result.Reply); err != nil {
		b.logger.Printf("failed to reply to mention message %s: %v", payload.Message.ID, err)
		return
	}
	if result.SendContent != "" {
		// Keep the progress reply observably ahead of the generated content even
		// when two message events are delivered almost at the same time.
		time.Sleep(100 * time.Millisecond)
		if err := b.poster.PostMessage(ctx, payload.Message.ChannelID, result.SendContent); err != nil {
			b.logger.Printf("failed to send command result for message %s: %v", payload.Message.ID, err)
			return
		}
	}
	b.logger.Printf("replied to mention message %s", payload.Message.ID)
}

func interpretationErrorReply(err error) string {
	if errors.Is(err, errInvalidArgumentCount) {
		return "引数を2つ指定してください。例: @BOT_MAI reset BOT"
	}
	if errors.Is(err, errNoInterpretation) {
		return "構文エラー"
	}
	return "構文エラー: " + err.Error()
}

func selectInterpretation(interpretations []string) string {
	candidates := interpretations
	if len(candidates) > 1 {
		withoutReset := make([]string, 0, len(candidates)-1)
		for _, candidate := range candidates {
			if candidate != "reset BOT" {
				withoutReset = append(withoutReset, candidate)
			}
		}
		if len(withoutReset) > 0 {
			candidates = withoutReset
		}
	}
	return candidates[mathrand.IntN(len(candidates))]
}
