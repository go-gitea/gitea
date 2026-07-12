// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestReleaseReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Setup setting reactions list if not set
	oldReactions := setting.UI.Reactions
	oldReactionsLookup := setting.UI.ReactionsLookup
	setting.UI.Reactions = []string{"laugh", "heart"}
	setting.UI.ReactionsLookup = make(container.Set[string])
	for _, reaction := range setting.UI.Reactions {
		setting.UI.ReactionsLookup.Add(reaction)
	}
	defer func() {
		setting.UI.Reactions = oldReactions
		setting.UI.ReactionsLookup = oldReactionsLookup
	}()

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1})
	release := unittest.AssertExistsAndLoadBean(t, &Release{ID: 1, RepoID: repo.ID})

	// Create reaction
	reaction1, err := CreateReleaseReaction(t.Context(), &ReleaseReactionOptions{
		Type:      "heart",
		DoerID:    user1.ID,
		ReleaseID: release.ID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, reaction1)
	assert.Equal(t, "heart", reaction1.Type)
	assert.Equal(t, user1.ID, reaction1.UserID)
	assert.Equal(t, release.ID, reaction1.ReleaseID)

	// Trying to create same reaction again should return ErrReleaseReactionAlreadyExist
	_, err = CreateReleaseReaction(t.Context(), &ReleaseReactionOptions{
		Type:      "heart",
		DoerID:    user1.ID,
		ReleaseID: release.ID,
	})
	assert.Error(t, err)
	assert.True(t, IsErrReleaseReactionAlreadyExist(err))

	// Creating reaction with invalid type should return ErrForbiddenReleaseReaction
	_, err = CreateReleaseReaction(t.Context(), &ReleaseReactionOptions{
		Type:      "invalid_reaction",
		DoerID:    user1.ID,
		ReleaseID: release.ID,
	})
	assert.Error(t, err)
	assert.True(t, IsErrForbiddenReleaseReaction(err))

	// Create second reaction
	reaction2, err := CreateReleaseReaction(t.Context(), &ReleaseReactionOptions{
		Type:      "laugh",
		DoerID:    user2.ID,
		ReleaseID: release.ID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, reaction2)

	// Find release reactions
	reactions, count, err := FindReleaseReactions(t.Context(), release.ID, db.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, reactions, 2)

	// Load users
	users, err := reactions.LoadUsers(t.Context(), repo)
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, user1.Name, reactions[0].User.Name)
	assert.Equal(t, user2.Name, reactions[1].User.Name)

	// HasUser
	assert.True(t, reactions.HasUser(user1.ID))
	assert.True(t, reactions.HasUser(user2.ID))
	assert.False(t, reactions.HasUser(9999))

	// GroupByType
	grouped := reactions.GroupByType()
	assert.Len(t, grouped, 2)
	assert.Len(t, grouped["heart"], 1)
	assert.Len(t, grouped["laugh"], 1)

	// GetFirstUsers and GetMoreUserCount
	setting.UI.ReactionMaxUserNum = 1
	assert.Equal(t, user1.Name, grouped["heart"].GetFirstUsers())
	assert.Equal(t, 0, grouped["heart"].GetMoreUserCount())

	// FindReactionsForReleases
	reactionsMap, err := FindReactionsForReleases(t.Context(), []*Release{release})
	assert.NoError(t, err)
	assert.Len(t, reactionsMap, 1)
	assert.Len(t, reactionsMap[release.ID], 2)

	// Delete reaction
	err = DeleteReleaseReaction(t.Context(), &ReleaseReactionOptions{
		Type:      "heart",
		DoerID:    user1.ID,
		ReleaseID: release.ID,
	})
	assert.NoError(t, err)

	reactions, count, err = FindReleaseReactions(t.Context(), release.ID, db.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.Len(t, reactions, 1)
	assert.Equal(t, "laugh", reactions[0].Type)
}
