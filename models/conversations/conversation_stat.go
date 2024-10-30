// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// ConversationStats represents conversation statistic information.
type ConversationStats struct {
	OpenCount, ClosedCount int64
	YourRepositoriesCount  int64
	AssignCount            int64
	CreateCount            int64
	MentionCount           int64
	ReviewRequestedCount   int64
	ReviewedCount          int64
}

// Filter modes.
const (
	FilterModeAll = iota
	FilterModeAssign
	FilterModeCreate
	FilterModeMention
	FilterModeYourRepositories
)

const (
	// MaxQueryParameters represents the max query parameters
	// When queries are broken down in parts because of the number
	// of parameters, attempt to break by this amount
	MaxQueryParameters = 300
)

// CountConversationsByRepo map from repoID to number of conversations matching the options
func CountConversationsByRepo(ctx context.Context, opts *ConversationsOptions) (map[int64]int64, error) {
	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`conversation`.repo_id = `repository`.id")

	applyConditions(sess, opts)

	countsSlice := make([]*struct {
		RepoID int64
		Count  int64
	}, 0, 10)
	if err := sess.GroupBy("conversation.repo_id").
		Select("conversation.repo_id AS repo_id, COUNT(*) AS count").
		Table("conversation").
		Find(&countsSlice); err != nil {
		return nil, fmt.Errorf("unable to CountConversationsByRepo: %w", err)
	}

	countMap := make(map[int64]int64, len(countsSlice))
	for _, c := range countsSlice {
		countMap[c.RepoID] = c.Count
	}
	return countMap, nil
}

// CountConversations number return of conversations by given conditions.
func CountConversations(ctx context.Context, opts *ConversationsOptions, otherConds ...builder.Cond) (int64, error) {
	sess := db.GetEngine(ctx).
		Select("COUNT(conversation.id) AS count").
		Table("conversation").
		Join("INNER", "repository", "`conversation`.repo_id = `repository`.id")
	applyConditions(sess, opts)

	for _, cond := range otherConds {
		sess.And(cond)
	}

	return sess.Count()
}

// GetConversationStats returns conversation statistic information by given conditions.
func GetConversationStats(ctx context.Context, opts *ConversationsOptions) (*ConversationStats, error) {
	if len(opts.ConversationIDs) <= MaxQueryParameters {
		return getConversationStatsChunk(ctx, opts, opts.ConversationIDs)
	}

	// If too long a list of IDs is provided, we get the statistics in
	// smaller chunks and get accumulates. Note: this could potentially
	// get us invalid results. The alternative is to insert the list of
	// ids in a temporary table and join from them.
	accum := &ConversationStats{}
	for i := 0; i < len(opts.ConversationIDs); {
		chunk := i + MaxQueryParameters
		if chunk > len(opts.ConversationIDs) {
			chunk = len(opts.ConversationIDs)
		}
		stats, err := getConversationStatsChunk(ctx, opts, opts.ConversationIDs[i:chunk])
		if err != nil {
			return nil, err
		}
		accum.OpenCount += stats.OpenCount
		accum.ClosedCount += stats.ClosedCount
		accum.YourRepositoriesCount += stats.YourRepositoriesCount
		accum.AssignCount += stats.AssignCount
		accum.CreateCount += stats.CreateCount
		accum.OpenCount += stats.MentionCount
		accum.ReviewRequestedCount += stats.ReviewRequestedCount
		accum.ReviewedCount += stats.ReviewedCount
		i = chunk
	}
	return accum, nil
}

func getConversationStatsChunk(ctx context.Context, opts *ConversationsOptions, conversationIDs []int64) (*ConversationStats, error) {
	stats := &ConversationStats{}

	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`conversation`.repo_id = `repository`.id")

	var err error
	stats.OpenCount, err = applyConversationsOptions(sess, opts, conversationIDs).
		And("conversation.is_closed = ?", false).
		Count(new(Conversation))
	if err != nil {
		return stats, err
	}
	stats.ClosedCount, err = applyConversationsOptions(sess, opts, conversationIDs).
		And("conversation.is_closed = ?", true).
		Count(new(Conversation))
	return stats, err
}

func applyConversationsOptions(sess *xorm.Session, opts *ConversationsOptions, conversationIDs []int64) *xorm.Session {
	if len(opts.RepoIDs) > 1 {
		sess.In("conversation.repo_id", opts.RepoIDs)
	} else if len(opts.RepoIDs) == 1 {
		sess.And("conversation.repo_id = ?", opts.RepoIDs[0])
	}

	if len(conversationIDs) > 0 {
		sess.In("conversation.id", conversationIDs)
	}

	if opts.PosterID > 0 {
		applyPosterCondition(sess, opts.PosterID)
	}

	if opts.MentionedID > 0 {
		applyMentionedCondition(sess, opts.MentionedID)
	}

	if opts.IsPull.Has() {
		sess.And("conversation.is_pull=?", opts.IsPull.Value())
	}

	return sess
}

// CountOrphanedConversations count conversations without a repo
func CountOrphanedConversations(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).
		Table("conversation").
		Join("LEFT", "repository", "conversation.repo_id=repository.id").
		Where(builder.IsNull{"repository.id"}).
		Select("COUNT(`conversation`.`id`)").
		Count()
}
