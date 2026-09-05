package v3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand/v2"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/labstack/echo/v5"

	"github.com/traPtitech/traQ/model"
	"github.com/traPtitech/traQ/repository"
	"github.com/traPtitech/traQ/router/extension/herror"
	"github.com/traPtitech/traQ/service/ws"
	"github.com/traPtitech/traQ/utils/optional"
)

const (
	qBotUserName          = "BOT_MAI"
	wsEventPostQBotAssets = "POST_QBOT_ASSETS"
)

type postQBotAssetsEvent struct {
	ChannelID uuid.UUID `json:"channel_id"`
}

func (h *Handlers) publishPostQBotAssets(ctx context.Context, channelID uuid.UUID) error {
	if h.BotWS == nil || h.Repo == nil {
		return nil
	}
	botUser, err := h.Repo.GetUserByName(ctx, qBotUserName, false)
	if err != nil {
		return fmt.Errorf("get %s user: %w", qBotUserName, err)
	}
	body, err := json.Marshal(postQBotAssetsEvent{ChannelID: channelID})
	if err != nil {
		return fmt.Errorf("encode qbot assets event: %w", err)
	}
	errs, attempted := h.BotWS.WriteMessage(wsEventPostQBotAssets, uuid.Must(uuid.NewV7()), body, botUser.GetID())
	if len(errs) > 0 {
		return fmt.Errorf("send qbot assets event: %w", errors.Join(errs...))
	}
	if !attempted {
		return errors.New("BOT_MAI websocket is not connected")
	}
	return nil
}

var qBotFileURLPattern = regexp.MustCompile(`/files/([0-9a-fA-F-]{36})`)

type qBotDeletedAttachmentResponse struct {
	MessageID uuid.UUID `json:"messageId"`
	FileID    uuid.UUID `json:"fileId"`
}

type qBotStateResponse struct {
	Cleared            bool                            `json:"cleared"`
	Revision           uint64                          `json:"revision"`
	Action             string                          `json:"action"`
	ActionPayload      map[string]string               `json:"actionPayload"`
	DeletedAttachments []qBotDeletedAttachmentResponse `json:"deletedAttachments"`
}

type qBotCommandRequest struct {
	Command   string    `json:"command"`
	Target    string    `json:"target"`
	UserID    uuid.UUID `json:"userId"`
	MessageID uuid.UUID `json:"messageId"`
	ChannelID uuid.UUID `json:"channelId"`
}

type qBotCommandResponse struct {
	Reply       string `json:"reply"`
	SendContent string `json:"sendContent,omitempty"`
}

type qBotAttachment struct {
	Message *model.Message
	File    *model.FileMeta
}

func (h *Handlers) GetMyQBotState(c *echo.Context) error {
	userID := getRequestUserID(c)
	if requestedUserID := c.QueryParam("userId"); requestedUserID != "" {
		if getRequestUser(c).GetName() != qBotUserName {
			return herror.Forbidden("only BOT_MAI may inspect another user's puzzle state")
		}
		parsedUserID, err := uuid.FromString(requestedUserID)
		if err != nil {
			return herror.BadRequest("invalid userId")
		}
		userID = parsedUserID
	}
	state, err := h.qBotStateResponse(c.Request().Context(), userID)
	if err != nil {
		return herror.InternalServerError(err)
	}
	if state.Cleared {
		if err := h.postBotRecoveryForUser(c.Request().Context(), userID); err != nil {
			return herror.InternalServerError(err)
		}
	}
	return c.JSON(http.StatusOK, state)
}

func (h *Handlers) postBotRecoveryForUser(ctx context.Context, userID uuid.UUID) error {
	player, err := h.Repo.GetUser(ctx, userID, false)
	if err != nil {
		return err
	}
	general, err := h.ChannelManager.GetChannelFromPath(ctx, player.GetName()+"/general")
	if err != nil {
		return err
	}
	return h.postSystemRecovery(ctx, general.ID, botSystemRecoveredMessage)
}

func (h *Handlers) ExecuteQBotCommand(c *echo.Context) error {
	ctx := c.Request().Context()
	if getRequestUser(c).GetName() != qBotUserName {
		return herror.Forbidden("only BOT_MAI may execute puzzle commands")
	}

	var req qBotCommandRequest
	if err := c.Bind(&req); err != nil {
		return herror.BadRequest(err)
	}
	if !validQBotCommand(req.Command) || !validQBotTarget(req.Target) || req.UserID == uuid.Nil || req.MessageID == uuid.Nil || req.ChannelID == uuid.Nil {
		return herror.BadRequest("invalid q_bot command request")
	}

	invocation, err := h.Repo.GetMessageByID(ctx, req.MessageID)
	if err != nil || invocation.UserID != req.UserID || invocation.ChannelID != req.ChannelID {
		return herror.BadRequest("the invocation message does not match the player and channel")
	}

	result, err := h.executeQBotCommand(ctx, req, invocation)
	if err != nil {
		return herror.InternalServerError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func validQBotCommand(command string) bool {
	switch command {
	case "count", "list", "open", "send", "delete", "reset", "debug":
		return true
	default:
		return false
	}
}

func validQBotTarget(target string) bool {
	switch target {
	case "BOT", "message", "user", "channel", "stamp", "file", "image":
		return true
	default:
		return false
	}
}

func (h *Handlers) executeQBotCommand(ctx context.Context, req qBotCommandRequest, invocation *model.Message) (qBotCommandResponse, error) {
	switch req.Command {
	case "count":
		return h.executeQBotCount(ctx, req)
	case "list":
		return h.executeQBotList(ctx, req)
	case "open":
		return h.executeQBotOpen(ctx, req)
	case "send":
		return h.executeQBotSend(ctx, req)
	case "delete":
		return h.executeQBotDelete(ctx, req, invocation)
	case "reset":
		return h.executeQBotReset(ctx, req)
	case "debug":
		return h.executeQBotDebug(ctx, req, invocation)
	default:
		return qBotCommandResponse{}, fmt.Errorf("unknown q_bot command %q", req.Command)
	}
}

func (h *Handlers) executeQBotCount(ctx context.Context, req qBotCommandRequest) (qBotCommandResponse, error) {
	var count int64
	switch req.Target {
	case "BOT":
		users, err := h.Repo.GetUsers(ctx, repository.UsersQuery{IsBot: optional.From(true)})
		if err != nil {
			return qBotCommandResponse{}, err
		}
		for _, user := range users {
			if strings.HasPrefix(user.GetName(), "BOT_") {
				count++
			}
		}
	case "user":
		count = 1
	case "message":
		stats, err := h.Repo.GetChannelStats(ctx, req.ChannelID, true)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		count = stats.TotalMessageCount
	case "channel":
		player, err := h.Repo.GetUser(ctx, req.UserID, false)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		channels, err := h.qBotChannelsForUser(ctx, player.GetName())
		if err != nil {
			return qBotCommandResponse{}, err
		}
		count = int64(len(channels))
	case "stamp":
		entity, err := h.Repo.GetAllStampsWithThumbnail(ctx, repository.StampTypeAll)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		count = int64(len(entity.Value()))
	case "file", "image":
		attachments, err := h.qBotAttachments(ctx, req.UserID, req.ChannelID, req.Target == "image")
		if err != nil {
			return qBotCommandResponse{}, err
		}
		count = int64(len(attachments))
	}
	return qBotCommandResponse{Reply: fmt.Sprintf("集計しています…\n\n# %d", count)}, nil
}

func (h *Handlers) executeQBotList(ctx context.Context, req qBotCommandRequest) (qBotCommandResponse, error) {
	const limit = 8
	lines := []string{"取得した項目:"}
	found := false
	switch req.Target {
	case "BOT":
		bots, err := h.Repo.GetBots(ctx, repository.BotsQuery{})
		if err != nil {
			return qBotCommandResponse{}, err
		}
		lines = append(lines, "| 名前 | ID |", "|---|---|")
		for _, bot := range bots[:min(len(bots), limit)] {
			found = true
			user, _ := h.Repo.GetUser(ctx, bot.BotUserID, false)
			name := bot.BotUserID.String()
			if user != nil {
				name = user.GetName()
			}
			lines = append(lines, fmt.Sprintf("| %s | `%s` |", qBotCell(name), bot.ID))
		}
	case "message":
		messages, _, err := h.Repo.GetMessages(ctx, repository.MessagesQuery{Channel: req.ChannelID, Limit: 4})
		if err != nil {
			return qBotCommandResponse{}, err
		}
		for _, message := range messages {
			if message.ID == req.MessageID {
				continue
			}
			found = true
			lines = append(lines, "/messages/"+message.ID.String())
			if len(lines) == 4 {
				break
			}
		}
		if found {
			return qBotCommandResponse{Reply: strings.Join(lines[1:], "\n")}, nil
		}
	case "user":
		user, err := h.Repo.GetUser(ctx, req.UserID, true)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		found = true
		lines = append(lines, "| 名前 | ID | 種別 |", "|---|---|---|", fmt.Sprintf("| %s | `%s` | user |", qBotCell(user.GetName()), user.GetID()))
	case "channel":
		player, err := h.Repo.GetUser(ctx, req.UserID, false)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		channels, err := h.qBotChannelsForUser(ctx, player.GetName())
		if err != nil {
			return qBotCommandResponse{}, err
		}
		lines = append(lines, "| パス | トピック |", "|---|---|")
		for _, channel := range channels[:min(len(channels), limit)] {
			found = true
			lines = append(lines, fmt.Sprintf("| %s | %s |", qBotChannelEmbed("#"+channel.path, channel.channel.ID), qBotCell(channel.channel.Topic)))
		}
	case "stamp":
		entity, err := h.Repo.GetAllStampsWithThumbnail(ctx, repository.StampTypeAll)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		stamps := entity.Value()
		lines = append(lines, "| スタンプ | ID | 種別 |", "|---|---|---|")
		for _, stamp := range stamps[:min(len(stamps), limit)] {
			found = true
			kind := "traQ"
			if stamp.IsUnicode {
				kind = "Unicode"
			}
			lines = append(lines, fmt.Sprintf("| :%s: | `%s` | %s |", stamp.Name, stamp.ID, kind))
		}
	case "file", "image":
		attachments, err := h.qBotAttachments(ctx, req.UserID, req.ChannelID, req.Target == "image")
		if err != nil {
			return qBotCommandResponse{}, err
		}
		lines = append(lines, "| ファイル名 | 投稿日時 | サイズ |", "|---|---|---|")
		for _, attachment := range attachments[:min(len(attachments), limit)] {
			found = true
			lines = append(lines, fmt.Sprintf("| %s | %s | %d bytes |", qBotCell(attachment.File.Name), attachment.Message.CreatedAt.Format(time.DateTime), attachment.File.Size))
		}
	}
	if !found {
		lines = append(lines, "見つかりませんでした。")
	}
	return qBotCommandResponse{Reply: strings.Join(lines, "\n")}, nil
}

type qBotChannelPath struct {
	channel *model.Channel
	path    string
}

func (h *Handlers) qBotChannelsForUser(ctx context.Context, name string) ([]qBotChannelPath, error) {
	channels, err := h.Repo.GetPublicChannels(ctx)
	if err != nil {
		return nil, err
	}
	return qBotChannelsUnderRoot(channels, name), nil
}

func qBotChannelsUnderRoot(channels []*model.Channel, name string) []qBotChannelPath {
	byID := make(map[uuid.UUID]*model.Channel, len(channels))
	for _, channel := range channels {
		byID[channel.ID] = channel
	}
	var pathOf func(*model.Channel) string
	pathOf = func(channel *model.Channel) string {
		if parent, ok := byID[channel.ParentID]; ok {
			return pathOf(parent) + "/" + channel.Name
		}
		return channel.Name
	}
	result := make([]qBotChannelPath, 0)
	rootPrefix := name + "/"
	for _, channel := range channels {
		path := pathOf(channel)
		if !channel.IsArchived() && strings.HasPrefix(path, rootPrefix) {
			result = append(result, qBotChannelPath{channel: channel, path: strings.TrimPrefix(path, rootPrefix)})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].path < result[j].path })
	return result
}

func (h *Handlers) executeQBotOpen(ctx context.Context, req qBotCommandRequest) (qBotCommandResponse, error) {
	payload := map[string]string{}
	switch req.Target {
	case "BOT":
		payload["userName"] = qBotUserName
	case "message":
		payload["messageId"] = req.MessageID.String()
	case "user":
		user, err := h.Repo.GetUser(ctx, req.UserID, false)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		payload["userName"] = user.GetName()
	case "channel":
		payload["channelId"] = req.ChannelID.String()
	case "stamp":
		// The client opens the normal stamp picker; no target ID is required.
	case "file", "image":
		attachments, err := h.qBotAttachments(ctx, req.UserID, req.ChannelID, req.Target == "image")
		if err != nil {
			return qBotCommandResponse{}, err
		}
		if len(attachments) == 0 {
			return qBotCommandResponse{Reply: "見つかりませんでした。"}, nil
		}
		payload["messageId"] = attachments[0].Message.ID.String()
		payload["fileId"] = attachments[0].File.ID.String()
	}
	if err := h.saveAndPublishQBotAction(ctx, req.UserID, "open_"+strings.ToLower(req.Target), payload, false); err != nil {
		return qBotCommandResponse{}, err
	}
	return qBotCommandResponse{Reply: "対象を開いています…"}, nil
}

func (h *Handlers) executeQBotSend(ctx context.Context, req qBotCommandRequest) (qBotCommandResponse, error) {
	content := ""
	switch req.Target {
	case "BOT":
		content = "@" + qBotUserName
	case "message":
		content = "/messages/" + req.MessageID.String()
	case "user":
		user, err := h.Repo.GetUser(ctx, req.UserID, false)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		content = "@" + user.GetName()
	case "channel":
		player, err := h.Repo.GetUser(ctx, req.UserID, false)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		general, err := h.ChannelManager.GetChannelFromPath(ctx, player.GetName()+"/general")
		if err != nil {
			return qBotCommandResponse{}, err
		}
		content = qBotChannelEmbed("#general", general.ID)
	case "stamp":
		entity, err := h.Repo.GetAllStampsWithThumbnail(ctx, repository.StampTypeAll)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		stamps := entity.Value()
		if len(stamps) == 0 {
			return qBotCommandResponse{Reply: "見つかりませんでした。"}, nil
		}
		content = ":" + stamps[mathrand.IntN(len(stamps))].Name + ":"
	case "file", "image":
		attachments, err := h.qBotAttachments(ctx, req.UserID, req.ChannelID, req.Target == "image")
		if err != nil {
			return qBotCommandResponse{}, err
		}
		if len(attachments) == 0 {
			return qBotCommandResponse{Reply: "見つかりませんでした。"}, nil
		}
		content = "/files/" + attachments[0].File.ID.String()
	}
	return qBotCommandResponse{Reply: "↗ 送信しています…", SendContent: content}, nil
}

func (h *Handlers) executeQBotDelete(ctx context.Context, req qBotCommandRequest, invocation *model.Message) (qBotCommandResponse, error) {
	switch req.Target {
	case "BOT", "user", "channel":
		return qBotCommandResponse{Reply: "権限がありません。"}, nil
	case "message":
		if err := h.MessageManager.Delete(ctx, req.MessageID); err != nil {
			return qBotCommandResponse{}, err
		}
		return qBotCommandResponse{Reply: "削除が完了しました。"}, nil
	case "stamp":
		if len(invocation.Stamps) == 0 {
			return qBotCommandResponse{Reply: "権限がありません。"}, nil
		}
		stamp := invocation.Stamps[0]
		if err := h.MessageManager.RemoveStamps(ctx, req.MessageID, stamp.StampID, stamp.UserID); err != nil {
			return qBotCommandResponse{}, err
		}
		return qBotCommandResponse{Reply: "削除が完了しました。"}, nil
	case "file", "image":
		attachments, err := h.qBotAttachments(ctx, req.UserID, req.ChannelID, req.Target == "image")
		if err != nil {
			return qBotCommandResponse{}, err
		}
		if len(attachments) == 0 {
			return qBotCommandResponse{Reply: "見つかりませんでした。"}, nil
		}
		target := attachments[0]
		if target.Message.UserID != req.UserID {
			return qBotCommandResponse{Reply: "権限がありません。"}, nil
		}
		if err := h.Repo.AddQBotDeletedAttachment(ctx, &model.QBotDeletedAttachment{UserID: req.UserID, MessageID: target.Message.ID, FileID: target.File.ID}); err != nil {
			return qBotCommandResponse{}, err
		}
		payload := map[string]string{"messageId": target.Message.ID.String(), "fileId": target.File.ID.String()}
		if err := h.saveAndPublishQBotAction(ctx, req.UserID, "delete_attachment", payload, false); err != nil {
			return qBotCommandResponse{}, err
		}
		return qBotCommandResponse{Reply: "削除が完了しました。"}, nil
	}
	return qBotCommandResponse{}, nil
}

func (h *Handlers) executeQBotReset(ctx context.Context, req qBotCommandRequest) (qBotCommandResponse, error) {
	if req.Target != "BOT" {
		return qBotCommandResponse{Reply: "権限がありません。"}, nil
	}
	if err := h.saveAndPublishQBotAction(ctx, req.UserID, "reset_bot", map[string]string{}, true); err != nil {
		return qBotCommandResponse{}, err
	}
	if err := h.postBotRecoveryForUser(ctx, req.UserID); err != nil {
		return qBotCommandResponse{}, err
	}
	return qBotCommandResponse{Reply: "BOTの復旧が完了しました。"}, nil
}

func (h *Handlers) executeQBotDebug(ctx context.Context, req qBotCommandRequest, invocation *model.Message) (qBotCommandResponse, error) {
	data := map[string]any{}
	switch req.Target {
	case "BOT":
		botUser, err := h.Repo.GetUserByName(ctx, qBotUserName, false)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		bots, err := h.Repo.GetBots(ctx, repository.BotsQuery{}.BotUserID(botUser.GetID()))
		if err != nil {
			return qBotCommandResponse{}, err
		}
		data["version"], data["revision"] = h.Version, h.Revision
		if len(bots) > 0 {
			data["id"] = bots[0].ID
			data["subscriptions"] = bots[0].SubscribeEvents.Array()
			data["state"] = bots[0].State
		}
	case "message":
		data["id"], data["length"], data["stampCount"] = invocation.ID, len([]rune(invocation.Text)), len(invocation.Stamps)
		data["attachmentCount"] = len(qBotFileURLPattern.FindAllStringSubmatch(invocation.Text, -1))
	case "user":
		user, err := h.Repo.GetUser(ctx, req.UserID, false)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		data["id"], data["name"], data["bot"], data["state"], data["permission"] = user.GetID(), user.GetName(), user.IsBot(), user.GetState().Int(), user.GetRole()
	case "channel":
		player, err := h.Repo.GetUser(ctx, req.UserID, false)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		channel, err := h.Repo.GetChannel(ctx, req.ChannelID)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		path, err := h.qBotChannelPath(ctx, req.ChannelID)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		data["id"], data["parentId"], data["name"], data["path"], data["visible"], data["canView"], data["canPost"] = channel.ID, channel.ParentID, channel.Name, qBotPathWithinUserRoot(path, player.GetName()), channel.IsVisible, true, !channel.IsArchived()
	case "stamp":
		entity, err := h.Repo.GetAllStampsWithThumbnail(ctx, repository.StampTypeAll)
		if err != nil {
			return qBotCommandResponse{}, err
		}
		stamps := entity.Value()
		if len(stamps) > 0 {
			stamp := stamps[0]
			stats, _ := h.Repo.GetStampStats(ctx, stamp.ID)
			data["id"], data["name"], data["fileId"] = stamp.ID, stamp.Name, stamp.FileID
			if file, fileErr := h.Repo.GetFileMeta(ctx, stamp.FileID); fileErr == nil && len(file.Thumbnails) > 0 {
				data["width"], data["height"] = file.Thumbnails[0].Width, file.Thumbnails[0].Height
			}
			if stats != nil {
				data["count"] = stats.TotalCount
			}
		}
	case "file", "image":
		attachments, err := h.qBotAttachments(ctx, req.UserID, req.ChannelID, req.Target == "image")
		if err != nil {
			return qBotCommandResponse{}, err
		}
		if len(attachments) > 0 {
			file := attachments[0].File
			data["id"], data["mime"], data["name"], data["bytes"], data["md5"] = file.ID, file.Mime, file.Name, file.Size, file.Hash
			if len(file.Thumbnails) > 0 {
				data["width"], data["height"] = file.Thumbnails[0].Width, file.Thumbnails[0].Height
			}
		}
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return qBotCommandResponse{}, err
	}
	return qBotCommandResponse{Reply: "```json\n" + string(encoded) + "\n```"}, nil
}

func (h *Handlers) qBotChannelPath(ctx context.Context, channelID uuid.UUID) (string, error) {
	channels, err := h.Repo.GetPublicChannels(ctx)
	if err != nil {
		return "", err
	}
	byID := make(map[uuid.UUID]*model.Channel, len(channels))
	for _, channel := range channels {
		byID[channel.ID] = channel
	}
	channel, ok := byID[channelID]
	if !ok {
		return "", repository.ErrNotFound
	}
	parts := []string{channel.Name}
	for channel.ParentID != uuid.Nil {
		parent, exists := byID[channel.ParentID]
		if !exists {
			break
		}
		parts = append(parts, parent.Name)
		channel = parent
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return strings.Join(parts, "/"), nil
}

func (h *Handlers) qBotAttachments(ctx context.Context, userID, channelID uuid.UUID, images bool) ([]qBotAttachment, error) {
	deleted, err := h.Repo.GetQBotDeletedAttachments(ctx, userID)
	if err != nil {
		return nil, err
	}
	deletedSet := make(map[string]struct{}, len(deleted))
	for _, item := range deleted {
		deletedSet[item.MessageID.String()+":"+item.FileID.String()] = struct{}{}
	}

	messages, _, err := h.Repo.GetMessages(ctx, repository.MessagesQuery{Channel: channelID})
	if err != nil {
		return nil, err
	}
	result := make([]qBotAttachment, 0)
	for _, message := range messages {
		for _, match := range qBotFileURLPattern.FindAllStringSubmatch(message.Text, -1) {
			fileID, err := uuid.FromString(match[1])
			if err != nil {
				continue
			}
			if _, hidden := deletedSet[message.ID.String()+":"+fileID.String()]; hidden {
				continue
			}
			file, err := h.Repo.GetFileMeta(ctx, fileID)
			if err != nil {
				continue
			}
			isImage := strings.HasPrefix(strings.ToLower(file.Mime), "image/")
			if isImage == images {
				result = append(result, qBotAttachment{Message: message, File: file})
			}
		}
	}
	return result, nil
}

func (h *Handlers) saveAndPublishQBotAction(ctx context.Context, userID uuid.UUID, action string, payload map[string]string, cleared bool) error {
	state, err := h.Repo.GetQBotState(ctx, userID)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	state.Revision++
	state.Action = action
	state.ActionPayload = string(encoded)
	state.Cleared = state.Cleared || cleared
	if err := h.Repo.SaveQBotState(ctx, state); err != nil {
		return err
	}
	response, err := h.qBotStateResponse(ctx, userID)
	if err != nil {
		return err
	}
	h.WS.WriteMessage("QBOT_ACTION", response, ws.TargetUsers(userID))
	return nil
}

func (h *Handlers) qBotStateResponse(ctx context.Context, userID uuid.UUID) (qBotStateResponse, error) {
	state, err := h.Repo.GetQBotState(ctx, userID)
	if err != nil {
		return qBotStateResponse{}, err
	}
	payload := map[string]string{}
	if state.ActionPayload != "" {
		_ = json.Unmarshal([]byte(state.ActionPayload), &payload)
	}
	deleted, err := h.Repo.GetQBotDeletedAttachments(ctx, userID)
	if err != nil {
		return qBotStateResponse{}, err
	}
	result := qBotStateResponse{Cleared: state.Cleared, Revision: state.Revision, Action: state.Action, ActionPayload: payload, DeletedAttachments: make([]qBotDeletedAttachmentResponse, 0, len(deleted))}
	for _, attachment := range deleted {
		result.DeletedAttachments = append(result.DeletedAttachments, qBotDeletedAttachmentResponse{MessageID: attachment.MessageID, FileID: attachment.FileID})
	}
	return result, nil
}

func qBotCell(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\n", " "), "|", "\\|")
}

func qBotOneLine(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return value
}

func qBotPathWithinUserRoot(path, userName string) string {
	return strings.TrimPrefix(path, userName+"/")
}

func qBotChannelEmbed(raw string, channelID uuid.UUID) string {
	return fmt.Sprintf(`!{"type":"channel","raw":"%s","id":"%s"}`, raw, channelID)
}
