// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	conversations_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

func Test_GetConversationIDsByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ids, err := conversations_model.GetConversationIDsByRepoID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.Len(t, ids, 5)
}

func TestConversationAPIURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	conversation := unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{ID: 1})
	err := conversation.LoadAttributes(db.DefaultContext)

	assert.NoError(t, err)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/conversations/1", conversation.APIURL(db.DefaultContext))
}

func TestGetConversationsByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(expectedConversationIDs, nonExistentConversationIDs []int64) {
		conversations, err := conversations_model.GetConversationsByIDs(db.DefaultContext, append(expectedConversationIDs, nonExistentConversationIDs...), true)
		assert.NoError(t, err)
		actualConversationIDs := make([]int64, len(conversations))
		for i, conversation := range conversations {
			actualConversationIDs[i] = conversation.ID
		}
		assert.Equal(t, expectedConversationIDs, actualConversationIDs)
	}
	testSuccess([]int64{1, 2, 3}, []int64{})
	testSuccess([]int64{1, 2, 3}, []int64{unittest.NonexistentID})
	testSuccess([]int64{3, 2, 1}, []int64{})
}

func TestUpdateConversationCols(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	conversation := unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{})

	now := time.Now().Unix()
	assert.NoError(t, conversations_model.UpdateConversationCols(db.DefaultContext, conversation, "name"))
	then := time.Now().Unix()

	updatedConversation := unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{ID: conversation.ID})
	unittest.AssertInt64InRange(t, now, then, int64(updatedConversation.UpdatedUnix))
}

func TestConversations(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	for _, test := range []struct {
		Opts                    conversations_model.ConversationsOptions
		ExpectedConversationIDs []int64
	}{
		{
			conversations_model.ConversationsOptions{
				AssigneeID: 1,
				SortType:   "oldest",
			},
			[]int64{1, 6},
		},
		{
			conversations_model.ConversationsOptions{
				RepoCond: builder.In("repo_id", 1, 3),
				SortType: "oldest",
				Paginator: &db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
			},
			[]int64{1, 2, 3, 5},
		},
		{
			conversations_model.ConversationsOptions{
				LabelIDs: []int64{1},
				Paginator: &db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
			},
			[]int64{2, 1},
		},
		{
			conversations_model.ConversationsOptions{
				LabelIDs: []int64{1, 2},
				Paginator: &db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
			},
			[]int64{}, // conversations with **both** label 1 and 2, none of these conversations matches, TODO: add more tests
		},
		{
			conversations_model.ConversationsOptions{
				MilestoneIDs: []int64{1},
			},
			[]int64{2},
		},
	} {
		conversations, err := conversations_model.Conversations(db.DefaultContext, &test.Opts)
		assert.NoError(t, err)
		if assert.Len(t, conversations, len(test.ExpectedConversationIDs)) {
			for i, conversation := range conversations {
				assert.EqualValues(t, test.ExpectedConversationIDs[i], conversation.ID)
			}
		}
	}
}

func TestConversation_InsertConversation(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// there are 5 conversations and max index is 5 on repository 1, so this one should 6
	conversation := testInsertConversation(t, "my conversation1", "special conversation's comments?", 6)
	_, err := db.DeleteByID[conversations_model.Conversation](db.DefaultContext, conversation.ID)
	assert.NoError(t, err)

	conversation = testInsertConversation(t, `my conversation2, this is my son's love \n \r \ `, "special conversation's '' comments?", 7)
	_, err = db.DeleteByID[conversations_model.Conversation](db.DefaultContext, conversation.ID)
	assert.NoError(t, err)
}

func TestResourceIndex(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			testInsertConversation(t, fmt.Sprintf("conversation %d", i+1), "my conversation", 0)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestCorrectConversationStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Because the condition is to have chunked database look-ups,
	// We have to more conversations than `maxQueryParameters`, we will insert.
	// maxQueryParameters + 10 conversations into the testDatabase.
	// Each new conversations will have a constant description "Bugs are nasty"
	// Which will be used later on.

	conversationAmount := conversations_model.MaxQueryParameters + 10

	var wg sync.WaitGroup
	for i := 0; i < conversationAmount; i++ {
		wg.Add(1)
		go func(i int) {
			testInsertConversation(t, fmt.Sprintf("Conversation %d", i+1), "Bugs are nasty", 0)
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Now we will get all conversationID's that match the "Bugs are nasty" query.
	conversations, err := conversations_model.Conversations(context.TODO(), &conversations_model.ConversationsOptions{
		Paginator: &db.ListOptions{
			PageSize: conversationAmount,
		},
		RepoIDs: []int64{1},
	})
	total := int64(len(conversations))
	var ids []int64
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}

	// Just to be sure.
	assert.NoError(t, err)
	assert.EqualValues(t, conversationAmount, total)

	// Now we will call the GetConversationStats with these IDs and if working,
	// get the correct stats back.
	conversationStats, err := conversations_model.GetConversationStats(db.DefaultContext, &conversations_model.ConversationsOptions{
		RepoIDs:         []int64{1},
		ConversationIDs: ids,
	})

	// Now check the values.
	assert.NoError(t, err)
	assert.EqualValues(t, conversationStats.OpenCount, conversationAmount)
}

func TestCountConversations(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	count, err := conversations_model.CountConversations(db.DefaultContext, &conversations_model.ConversationsOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, 22, count)
}

func TestConversationLoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.Service.EnableTimetracking = true

	conversationList := conversations_model.ConversationList{
		unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{ID: 4}),
	}

	for _, conversation := range conversationList {
		assert.NoError(t, conversation.LoadAttributes(db.DefaultContext))
		assert.EqualValues(t, conversation.RepoID, conversation.Repo.ID)
		for _, comment := range conversation.Comments {
			assert.EqualValues(t, conversation.ID, comment.ConversationID)
		}
	}
}

func assertCreateConversations(t *testing.T, isPull bool) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reponame := "repo1"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame})

	conversationID := int64(99)
	is := &conversations_model.Conversation{
		RepoID: repo.ID,
		Repo:   repo,
		ID:     conversationID,
	}
	err := conversations_model.InsertConversations(db.DefaultContext, is)
	assert.NoError(t, err)

	unittest.AssertExistsAndLoadBean(t, &conversations_model.Conversation{RepoID: repo.ID, ID: conversationID})
}

func testInsertConversation(t *testing.T, title, content string, expectIndex int64) *conversations_model.Conversation {
	var newConversation conversations_model.Conversation
	t.Run(title, func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

		conversation := conversations_model.Conversation{
			RepoID: repo.ID,
		}
		err := conversations_model.NewConversation(db.DefaultContext, repo, &conversation, nil)
		assert.NoError(t, err)

		has, err := db.GetEngine(db.DefaultContext).ID(conversation.ID).Get(&newConversation)
		assert.NoError(t, err)
		assert.True(t, has)
		if expectIndex > 0 {
			assert.EqualValues(t, expectIndex, newConversation.Index)
		}
	})
	return &newConversation
}
