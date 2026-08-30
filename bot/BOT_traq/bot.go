package main

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	traqbot "github.com/traPtitech/traq-bot"
)

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

	reply := interpretationReply(payload.Message.PlainText)
	if err := b.poster.PostMessage(ctx, payload.Message.ChannelID, reply); err != nil {
		b.logger.Printf("failed to reply to mention message %s: %v", payload.Message.ID, err)
		return
	}
	b.logger.Printf("replied to mention message %s", payload.Message.ID)
}

func interpretationReply(plainText string) string {
	interpretations, err := interpretMessage(plainText)
	if err == nil {
		if len(interpretations) == 1 {
			return interpretations[0]
		}
		return "- " + strings.Join(interpretations, "\n- ")
	}
	if errors.Is(err, errInvalidArgumentCount) {
		return "引数を2つ指定してください。例: @BOT_traq file message"
	}
	if errors.Is(err, errNoInterpretation) {
		return "構文エラー: ナイト移動後に有効な解釈がありません。"
	}
	return "構文エラー: " + err.Error()
}
