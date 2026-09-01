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
	logger   *log.Logger
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

	interpretations, err := interpretMessage(payload.Message.PlainText)
	if err != nil {
		if postErr := b.poster.PostMessage(ctx, payload.Message.ChannelID, interpretationErrorReply(err)); postErr != nil {
			b.logger.Printf("failed to reply to mention message %s: %v", payload.Message.ID, postErr)
		}
		return
	}
	selected := selectInterpretation(interpretations)
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
		if err := b.poster.PostMessage(ctx, payload.Message.ChannelID, result.SendContent); err != nil {
			b.logger.Printf("failed to send command result for message %s: %v", payload.Message.ID, err)
			return
		}
	}
	b.logger.Printf("replied to mention message %s", payload.Message.ID)
}

func interpretationErrorReply(err error) string {
	if errors.Is(err, errInvalidArgumentCount) {
		return "引数を2つ指定してください。例: @BOT_traq file message"
	}
	if errors.Is(err, errNoInterpretation) {
		return "構文エラー: ナイト移動後に有効な解釈がありません。"
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
