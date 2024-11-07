// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/indexer/conversations/internal"
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestDBSearchConversations(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.Indexer.ConversationType = "db"
	InitConversationIndexer(true)

	t.Run("search conversations with keyword", searchConversationWithKeyword)
	t.Run("search conversations by index", searchConversationByIndex)
	t.Run("search conversations in repo", searchConversationInRepo)
	t.Run("search conversations by ID", searchConversationByID)
	t.Run("search conversations with order", searchConversationWithOrder)
	t.Run("search conversations with paginator", searchConversationWithPaginator)
}

func searchConversationWithKeyword(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				Keyword: "conversationcomment2",
				RepoIDs: []int64{1},
			},
			[]int64{2},
		},
		{
			SearchOptions{
				Keyword: "first",
				RepoIDs: []int64{1},
			},
			[]int64{1},
		},
		{
			SearchOptions{
				Keyword: "good",
				RepoIDs: []int64{1},
			},
			[]int64{1},
		},
		{
			SearchOptions{
				Keyword: "comment",
			},
			[]int64{2, 4},
		},
	}

	for _, test := range tests {
		conversationIDs, _, err := SearchConversations(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, conversationIDs)
	}
}

func searchConversationByIndex(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				Keyword: "1000",
				RepoIDs: []int64{1},
			},
			[]int64{},
		},
		{
			SearchOptions{
				Keyword: "2",
				RepoIDs: []int64{1, 2, 3, 32},
			},
			[]int64{17, 12, 7, 2},
		},
		{
			SearchOptions{
				Keyword: "1",
				RepoIDs: []int64{58},
			},
			[]int64{19},
		},
	}

	for _, test := range tests {
		conversationIDs, _, err := SearchConversations(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, conversationIDs)
	}
}

func searchConversationInRepo(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				RepoIDs: []int64{1},
			},
			[]int64{11, 5, 3, 2, 1},
		},
		{
			SearchOptions{
				RepoIDs: []int64{2},
			},
			[]int64{7, 4},
		},
		{
			SearchOptions{
				RepoIDs: []int64{3},
			},
			[]int64{12, 6},
		},
		{
			SearchOptions{
				RepoIDs: []int64{4},
			},
			[]int64{},
		},
		{
			SearchOptions{
				RepoIDs: []int64{5},
			},
			[]int64{15},
		},
	}

	for _, test := range tests {
		conversationIDs, _, err := SearchConversations(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, conversationIDs)
	}
}

func searchConversationByID(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			// NOTE: This tests no filtering
			opts:        *ToSearchOptions("", &conversations.ConversationsOptions{}),
			expectedIDs: []int64{22, 21, 17, 16, 15, 14, 13, 12, 11, 20, 6, 5, 19, 18, 10, 7, 4, 9, 8, 3, 2, 1},
		},
	}

	for _, test := range tests {
		conversationIDs, _, err := SearchConversations(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, conversationIDs)
	}
}

func searchConversationWithOrder(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				SortBy: internal.SortByCreatedAsc,
			},
			[]int64{1, 2, 3, 8, 9, 4, 7, 10, 18, 19, 5, 6, 20, 11, 12, 13, 14, 15, 16, 17, 21, 22},
		},
	}
	for _, test := range tests {
		conversationIDs, _, err := SearchConversations(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, conversationIDs)
	}
}

func searchConversationWithPaginator(t *testing.T) {
	tests := []struct {
		opts          SearchOptions
		expectedIDs   []int64
		expectedTotal int64
	}{
		{
			SearchOptions{
				Paginator: &db.ListOptions{
					PageSize: 5,
				},
			},
			[]int64{22, 21, 17, 16, 15},
			22,
		},
	}
	for _, test := range tests {
		conversationIDs, total, err := SearchConversations(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, conversationIDs)
		assert.Equal(t, test.expectedTotal, total)
	}
}
