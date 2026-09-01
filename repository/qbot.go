package repository

import (
	"context"

	"github.com/gofrs/uuid"

	"github.com/traPtitech/traQ/model"
)

type QBotRepository interface {
	GetQBotState(ctx context.Context, userID uuid.UUID) (*model.QBotState, error)
	SaveQBotState(ctx context.Context, state *model.QBotState) error
	GetQBotDeletedAttachments(ctx context.Context, userID uuid.UUID) ([]*model.QBotDeletedAttachment, error)
	AddQBotDeletedAttachment(ctx context.Context, attachment *model.QBotDeletedAttachment) error
}
