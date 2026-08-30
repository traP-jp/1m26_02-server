package main

import traqbot "github.com/traPtitech/traq-bot"

type commandName string

const (
	commandCount  commandName = "count"
	commandList   commandName = "list"
	commandOpen   commandName = "open"
	commandSend   commandName = "send"
	commandDelete commandName = "delete"
	commandReset  commandName = "reset"
	commandDebug  commandName = "debug"
)

type targetName string

const (
	targetBOT     targetName = "BOT"
	targetMessage targetName = "message"
	targetUser    targetName = "user"
	targetChannel targetName = "channel"
	targetStamp   targetName = "stamp"
	targetFile    targetName = "file"
	targetImage   targetName = "image"
)

type commandRequest struct {
	Command commandName
	Target  targetName
	Message traqbot.MessagePayload
}

func (t targetName) valid() bool {
	switch t {
	case targetBOT, targetMessage, targetUser, targetChannel, targetStamp, targetFile, targetImage:
		return true
	default:
		return false
	}
}
