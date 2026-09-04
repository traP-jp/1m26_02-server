package v3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand"
	"net/http"
	"regexp"
	"slices"

	"github.com/gofrs/uuid"
	"github.com/labstack/echo/v5"

	"github.com/traPtitech/traQ/router/consts"
	"github.com/traPtitech/traQ/router/extension/herror"
	"github.com/traPtitech/traQ/service/channel"
	"github.com/traPtitech/traQ/service/ws"
)

var lightsOutBoardStampPattern = regexp.MustCompile(`(?m)^:([^:\s]+): #.+$`)

type clearLightsOutRequest struct {
	MessageID uuid.UUID `json:"messageId"`
}

// PostClearLightsOut POST /channels/:channelID/clearlightsout
func (h *Handlers) PostClearLightsOut(c *echo.Context) error {
	ctx := c.Request().Context()
	user := getRequestUser(c)
	rootID := getParamAsUUID(c, consts.ParamChannelID)
	var req clearLightsOutRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	board, err := h.ChannelManager.GetChannelFromPath(ctx, user.GetName()+"/general/1")
	if err != nil {
		return herror.InternalServerError(err)
	}
	msg, err := h.MessageManager.Get(ctx, req.MessageID)
	if err != nil || msg.GetChannelID() != board.ID {
		return herror.BadRequest("invalid lights out board")
	}
	root, err := h.ChannelManager.GetChannel(ctx, rootID)
	if err != nil || root.Name != lightsOutRootChannelName {
		return herror.BadRequest("invalid lights out root")
	}
	matches := lightsOutBoardStampPattern.FindAllStringSubmatch(msg.GetText(), -1)
	if len(matches) == 0 {
		return herror.BadRequest("invalid lights out board")
	}
	for _, match := range matches {
		stamp, err := h.Repo.GetStampByName(ctx, match[1])
		if err != nil {
			return herror.BadRequest("invalid lights out stamp")
		}
		count := 0
		for _, applied := range msg.GetStamps() {
			if applied.StampID == stamp.ID && applied.UserID == user.GetID() {
				count += applied.Count
			}
		}
		if count != 1 {
			return herror.BadRequest("lights out is not cleared")
		}
	}
	general, err := h.ChannelManager.GetChannel(ctx, board.ParentID)
	if err != nil {
		return herror.InternalServerError(err)
	}
	if err := h.postSystemRecovery(ctx, general.ID, channelSystemRecoveredMessage); err != nil {
		return herror.InternalServerError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

const (
	wsEventCreateLightsOut    = "CREATE_LIGHTS_OUT"
	wsEventDeleteLightsOut    = "DELETE_LIGHTS_OUT"
	lightsOutRootChannelName  = "random"
	lightsOutBoardChannelName = "general"
	lightsOutBoardChildName   = "1"
	lightsOutBotUserName      = "BOT_AI"
	minLightsOutChannelCount  = 10
)

var errTooManyLightsOutChannels = errors.New("too many channels for lights out stamps")

type lightsOutStamp struct {
	Name  string
	Emoji string
}

var lightsOutStamps = []lightsOutStamp{
	{Name: "thumbsup", Emoji: "👍"},
	{Name: "white_check_mark", Emoji: "✅"},
	{Name: "tada", Emoji: "🎉"},
	{Name: "smile", Emoji: "😄"},
	{Name: "joy", Emoji: "😂"},
	{Name: "wink", Emoji: "😉"},
	{Name: "sunglasses", Emoji: "😎"},
	{Name: "thinking", Emoji: "🤔"},
	{Name: "ok_hand", Emoji: "👌"},
	{Name: "clap", Emoji: "👏"},
	{Name: "wave", Emoji: "👋"},
	{Name: "pray", Emoji: "🙏"},
	{Name: "muscle", Emoji: "💪"},
	{Name: "star", Emoji: "⭐"},
	{Name: "dog", Emoji: "🐶"},
	{Name: "fire", Emoji: "🔥"},
	{Name: "bulb", Emoji: "💡"},
	{Name: "key", Emoji: "🔑"},
	{Name: "lock", Emoji: "🔒"},
	{Name: "unlock", Emoji: "🔓"},
	{Name: "cat", Emoji: "🐱"},
	{Name: "bell", Emoji: "🔔"},
	{Name: "gem", Emoji: "💎"},
	{Name: "rocket", Emoji: "🚀"},
	{Name: "ghost", Emoji: "👻"},
	{Name: "alien", Emoji: "👽"},
	{Name: "robot", Emoji: "🤖"},
	{Name: "skull", Emoji: "💀"},
	{Name: "mouse", Emoji: "🐭"},
	{Name: "blue_heart", Emoji: "💙"},
	{Name: "green_heart", Emoji: "💚"},
	{Name: "yellow_heart", Emoji: "💛"},
	{Name: "purple_heart", Emoji: "💜"},
	{Name: "orange_heart", Emoji: "🧡"},
	{Name: "black_heart", Emoji: "🖤"},
	{Name: "white_heart", Emoji: "🤍"},
	{Name: "eyes", Emoji: "👀"},
	{Name: "rabbit", Emoji: "🐰"},
	{Name: "sun_with_face", Emoji: "🌞"},
	{Name: "new_moon_with_face", Emoji: "🌚"},
	{Name: "full_moon_with_face", Emoji: "🌝"},
}

type lightsOutChannel struct {
	ID        uuid.UUID   `json:"id"`
	Path      string      `json:"path"`
	ParentID  uuid.UUID   `json:"parent_id"`
	Children  []uuid.UUID `json:"children"`
	StampName string      `json:"stamp_name"`
	Stamp     string      `json:"stamp"`
}

type createLightsOutEvent struct {
	RootChannelID  uuid.UUID          `json:"root_channel_id"`
	BoardChannelID uuid.UUID          `json:"board_channel_id"`
	Channels       []lightsOutChannel `json:"channels"`
}

type deleteLightsOutEvent struct {
	RootChannelID uuid.UUID `json:"root_channel_id"`
}

func makeCreateLightsOutEvent(tree channel.Tree, rootChannelID, boardChannelID uuid.UUID) (createLightsOutEvent, error) {
	return makeCreateLightsOutEventWithShuffle(tree, rootChannelID, boardChannelID, mathrand.Shuffle)
}

func (h *Handlers) createAndPublishLightsOut(ctx context.Context, userID, rootChannelID, boardChannelID uuid.UUID) error {
	var tree channel.Tree
	for {
		tree = h.ChannelManager.AccessibleChannelTree(ctx, userID)
		activeDescendantCount := 0
		for _, id := range tree.GetDescendantIDs(rootChannelID) {
			if !tree.IsArchivedChannel(id) {
				activeDescendantCount++
			}
		}
		if activeDescendantCount >= minLightsOutChannelCount {
			break
		}
		if activeDescendantCount > 0 {
			h.ChannelManager.DeleteLightsOutChannel(ctx, rootChannelID, userID)
		}
		h.ChannelManager.CreateLightsOutChannel(ctx, rootChannelID, userID, 3)
	}
	event, err := makeCreateLightsOutEvent(tree, rootChannelID, boardChannelID)
	if err != nil {
		return err
	}
	return h.publishCreateLightsOut(ctx, userID, event)
}

func makeCreateLightsOutEventWithShuffle(
	tree channel.Tree,
	rootChannelID, boardChannelID uuid.UUID,
	shuffle func(int, func(int, int)),
) (createLightsOutEvent, error) {
	if tree == nil || !tree.IsChannelPresent(rootChannelID) {
		return createLightsOutEvent{}, channel.ErrInvalidChannel
	}

	ids := []uuid.UUID{rootChannelID}
	for _, id := range tree.GetDescendantIDs(rootChannelID) {
		if !tree.IsArchivedChannel(id) {
			ids = append(ids, id)
		}
	}
	if len(ids) > len(lightsOutStamps) {
		return createLightsOutEvent{}, errTooManyLightsOutChannels
	}
	shuffle(len(ids), func(i, j int) {
		ids[i], ids[j] = ids[j], ids[i]
	})
	stamps := append([]lightsOutStamp(nil), lightsOutStamps...)
	shuffle(len(stamps), func(i, j int) {
		stamps[i], stamps[j] = stamps[j], stamps[i]
	})

	activeIDs := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		activeIDs[id] = struct{}{}
	}

	channels := make([]lightsOutChannel, 0, len(ids))
	for i, id := range ids {
		model, err := tree.GetModel(id)
		if err != nil {
			return createLightsOutEvent{}, err
		}
		children := slices.DeleteFunc(append([]uuid.UUID(nil), tree.GetChildrenIDs(id)...), func(childID uuid.UUID) bool {
			_, active := activeIDs[childID]
			return !active
		})
		channels = append(channels, lightsOutChannel{
			ID:        id,
			Path:      tree.GetChannelPath(id),
			ParentID:  model.ParentID,
			Children:  children,
			StampName: stamps[i].Name,
			Stamp:     stamps[i].Emoji,
		})
	}

	return createLightsOutEvent{
		RootChannelID:  rootChannelID,
		BoardChannelID: boardChannelID,
		Channels:       channels,
	}, nil
}

func (h *Handlers) publishCreateLightsOut(ctx context.Context, userID uuid.UUID, event createLightsOutEvent) error {
	if h.BotWS == nil || h.Repo == nil {
		if h.WS != nil {
			h.WS.WriteMessage(wsEventCreateLightsOut, event, ws.TargetUsers(userID))
		}
		return nil
	}

	botUser, err := h.Repo.GetUserByName(ctx, lightsOutBotUserName, false)
	if err != nil {
		return fmt.Errorf("get %s user: %w", lightsOutBotUserName, err)
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode lights out event: %w", err)
	}
	errs, attempted := h.BotWS.WriteMessage(wsEventCreateLightsOut, uuid.Must(uuid.NewV7()), body, botUser.GetID())
	if len(errs) > 0 {
		return fmt.Errorf("send lights out event to %s: %w", lightsOutBotUserName, errors.Join(errs...))
	}
	if !attempted {
		return fmt.Errorf("%s websocket is not connected", lightsOutBotUserName)
	}
	if h.WS != nil {
		h.WS.WriteMessage(wsEventCreateLightsOut, event, ws.TargetUsers(userID))
	}
	return nil
}

func (h *Handlers) publishDeleteLightsOut(userID, rootChannelID uuid.UUID) {
	if h.WS != nil {
		h.WS.WriteMessage(wsEventDeleteLightsOut, deleteLightsOutEvent{RootChannelID: rootChannelID}, ws.TargetUsers(userID))
	}
}
