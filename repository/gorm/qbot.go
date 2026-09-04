package gorm

import (
	"context"
	"errors"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/traPtitech/traQ/model"
	"github.com/traPtitech/traQ/repository"
)

func (repo *Repository) GetQBotState(ctx context.Context, userID uuid.UUID) (*model.QBotState, error) {
	if userID == uuid.Nil {
		return nil, repository.ErrNilID
	}
	state := &model.QBotState{UserID: userID, ActionPayload: "{}"}
	if err := repo.db.WithContext(ctx).First(state, "user_id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return state, nil
		}
		return nil, err
	}
	return state, nil
}

func (repo *Repository) SaveQBotState(ctx context.Context, state *model.QBotState) error {
	if state == nil || state.UserID == uuid.Nil {
		return repository.ErrNilID
	}
	return repo.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		UpdateAll: true,
	}).Create(state).Error
}

func (repo *Repository) GetQBotDeletedAttachments(ctx context.Context, userID uuid.UUID) ([]*model.QBotDeletedAttachment, error) {
	if userID == uuid.Nil {
		return nil, repository.ErrNilID
	}
	var result []*model.QBotDeletedAttachment
	err := repo.db.WithContext(ctx).Where("user_id = ?", userID).Find(&result).Error
	return result, err
}

func (repo *Repository) AddQBotDeletedAttachment(ctx context.Context, attachment *model.QBotDeletedAttachment) error {
	if attachment == nil || attachment.UserID == uuid.Nil || attachment.MessageID == uuid.Nil || attachment.FileID == uuid.Nil {
		return repository.ErrNilID
	}
	return repo.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(attachment).Error
}
