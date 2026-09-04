package v3

import (
	"slices"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/golang/mock/gomock"

	"github.com/traPtitech/traQ/model"
	"github.com/traPtitech/traQ/service/channel/mock_channel"
)

func TestMakeCreateLightsOutEvent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	tree := mock_channel.NewMockTree(ctrl)
	rootID := uuid.Must(uuid.NewV7())
	boardID := uuid.Must(uuid.NewV7())
	childID := uuid.Must(uuid.NewV7())
	grandchildID := uuid.Must(uuid.NewV7())

	tree.EXPECT().IsChannelPresent(rootID).Return(true)
	tree.EXPECT().GetDescendantIDs(rootID).Return([]uuid.UUID{grandchildID, childID})
	tree.EXPECT().IsArchivedChannel(grandchildID).Return(false)
	tree.EXPECT().IsArchivedChannel(childID).Return(false)
	tree.EXPECT().GetChannelPath(gomock.Any()).DoAndReturn(func(id uuid.UUID) string {
		switch id {
		case rootID:
			return "random"
		case childID:
			return "random/hoge"
		default:
			return "random/hoge/dev"
		}
	}).AnyTimes()
	tree.EXPECT().GetModel(rootID).Return(&model.Channel{ID: rootID}, nil)
	tree.EXPECT().GetModel(childID).Return(&model.Channel{ID: childID, ParentID: rootID}, nil)
	tree.EXPECT().GetModel(grandchildID).Return(&model.Channel{ID: grandchildID, ParentID: childID}, nil)
	tree.EXPECT().GetChildrenIDs(rootID).Return([]uuid.UUID{childID})
	tree.EXPECT().GetChildrenIDs(childID).Return([]uuid.UUID{grandchildID})
	tree.EXPECT().GetChildrenIDs(grandchildID).Return([]uuid.UUID{})

	shuffleCalls := 0
	reverse := func(n int, swap func(int, int)) {
		shuffleCalls++
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			swap(i, j)
		}
	}
	event, err := makeCreateLightsOutEventWithShuffle(tree, rootID, boardID, reverse)
	if err != nil {
		t.Fatalf("makeCreateLightsOutEvent() error = %v", err)
	}
	if event.RootChannelID != rootID {
		t.Errorf("RootChannelID = %s, want %s", event.RootChannelID, rootID)
	}
	if event.BoardChannelID != boardID {
		t.Errorf("BoardChannelID = %s, want %s", event.BoardChannelID, boardID)
	}
	if len(event.Channels) != 3 {
		t.Fatalf("len(Channels) = %d, want 3", len(event.Channels))
	}
	if shuffleCalls != 2 {
		t.Errorf("shuffle calls = %d, want 2", shuffleCalls)
	}
	if event.Channels[0].ID != childID || event.Channels[0].ParentID != rootID || event.Channels[0].StampName != "full_moon_with_face" {
		t.Errorf("first channel = %#v", event.Channels[0])
	}
	if event.Channels[1].ID != grandchildID || event.Channels[1].ParentID != childID || event.Channels[1].StampName != "new_moon_with_face" {
		t.Errorf("second channel = %#v", event.Channels[1])
	}
	if event.Channels[2].ID != rootID || event.Channels[2].StampName != "sun_with_face" {
		t.Errorf("third channel = %#v", event.Channels[2])
	}
}

func TestMakeCreateLightsOutEventOmitsArchivedChannels(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	tree := mock_channel.NewMockTree(ctrl)
	rootID := uuid.Must(uuid.NewV7())
	boardID := uuid.Must(uuid.NewV7())
	activeID := uuid.Must(uuid.NewV7())
	archivedID := uuid.Must(uuid.NewV7())

	tree.EXPECT().IsChannelPresent(rootID).Return(true)
	tree.EXPECT().GetDescendantIDs(rootID).Return([]uuid.UUID{activeID, archivedID})
	tree.EXPECT().IsArchivedChannel(activeID).Return(false)
	tree.EXPECT().IsArchivedChannel(archivedID).Return(true)
	tree.EXPECT().GetModel(rootID).Return(&model.Channel{ID: rootID}, nil)
	tree.EXPECT().GetModel(activeID).Return(&model.Channel{ID: activeID, ParentID: rootID}, nil)
	tree.EXPECT().GetChannelPath(rootID).Return("random")
	tree.EXPECT().GetChannelPath(activeID).Return("random/new")
	tree.EXPECT().GetChildrenIDs(rootID).Return([]uuid.UUID{activeID, archivedID})
	tree.EXPECT().GetChildrenIDs(activeID).Return([]uuid.UUID{})

	event, err := makeCreateLightsOutEventWithShuffle(
		tree,
		rootID,
		boardID,
		func(_ int, _ func(int, int)) {},
	)
	if err != nil {
		t.Fatalf("makeCreateLightsOutEvent() error = %v", err)
	}
	if len(event.Channels) != 2 {
		t.Fatalf("len(Channels) = %d, want 2", len(event.Channels))
	}
	if !slices.Equal(event.Channels[0].Children, []uuid.UUID{activeID}) {
		t.Errorf("root children = %v, want only active child", event.Channels[0].Children)
	}
}

func TestMakeCreateLightsOutEventRejectsInvisibleRoot(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	tree := mock_channel.NewMockTree(ctrl)
	rootID := uuid.Must(uuid.NewV7())
	boardID := uuid.Must(uuid.NewV7())
	tree.EXPECT().IsChannelPresent(rootID).Return(false)

	if _, err := makeCreateLightsOutEvent(tree, rootID, boardID); err == nil {
		t.Fatal("makeCreateLightsOutEvent() error = nil, want error")
	}
}
