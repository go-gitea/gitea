// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaginatedListsOrderTiesByID(t *testing.T) {
	require.NoError(t, unittest.LoadFixtures())
	repoOpts := repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 50},
		Actor:       unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}),
		OwnerID:     2,
		Private:     true,
		OrderBy:     db.SearchOrderByRecentUpdated,
	}
	searched, _, err := repo_model.SearchRepository(t.Context(), repoOpts)
	require.NoError(t, err)
	assert.IsDecreasing(t, searched.IDs())
	owned, _, err := repo_model.GetUserRepositories(t.Context(), repoOpts)
	require.NoError(t, err)
	assert.IsDecreasing(t, owned.IDs())

	users, _, err := user_model.SearchUsers(t.Context(), user_model.SearchUserOptions{
		ListOptions: db.ListOptions{Page: 1, PageSize: 50},
		Types:       []user_model.UserType{user_model.UserTypeIndividual},
		OrderBy:     "`user`.updated_unix DESC",
	})
	require.NoError(t, err)
	assert.IsDecreasing(t, user_model.UserList(users).GetUserIDs())

	topics, _, err := db.FindAndCount[repo_model.Topic](t.Context(), &repo_model.FindTopicOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 50}})
	require.NoError(t, err)
	topicIDs := make([]int64, 0, len(topics))
	for _, topic := range topics {
		topicIDs = append(topicIDs, topic.ID)
	}
	assert.Equal(t, []int64{6, 1, 5, 4, 3, 2}, topicIDs)
}
