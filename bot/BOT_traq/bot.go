package main

import (
	"context"
	"log"
	"time"

	traqbot "github.com/traPtitech/traq-bot"
)

const mentionReply = "呼びましたか？"

type bot struct {
	poster messagePoster
	logger *log.Logger
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := b.poster.PostMessage(ctx, payload.Message.ChannelID, mentionReply); err != nil {
		b.logger.Printf("failed to reply to mention message %s: %v", payload.Message.ID, err)
		return
	}
	b.logger.Printf("replied to mention message %s", payload.Message.ID)
}
