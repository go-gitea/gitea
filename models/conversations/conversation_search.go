// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// ConversationsOptions represents options of an conversation.
type ConversationsOptions struct { //nolint
	Paginator         *db.ListOptions
	RepoIDs           []int64 // overwrites RepoCond if the length is not 0
	AllPublic         bool    // include also all public repositories
	RepoCond          builder.Cond
	SortType          string
	ConversationIDs   []int64
	UpdatedAfterUnix  int64
	UpdatedBeforeUnix int64
	// prioritize conversations from this repo
	PriorityRepoID int64
	IsArchived     optional.Option[bool]
	Org            *organization.Organization // conversations permission scope
	Team           *organization.Team         // conversations permission scope
	User           *user_model.User           // conversations permission scope
}

// Copy returns a copy of the options.
// Be careful, it's not a deep copy, so `ConversationsOptions.RepoIDs = {...}` is OK while `ConversationsOptions.RepoIDs[0] = ...` is not.
func (o *ConversationsOptions) Copy(edit ...func(options *ConversationsOptions)) *ConversationsOptions {
	if o == nil {
		return nil
	}
	v := *o
	for _, e := range edit {
		e(&v)
	}
	return &v
}

// applySorts sort an conversations-related session based on the provided
// sortType string
func applySorts(sess *xorm.Session, sortType string, priorityRepoID int64) {
	switch sortType {
	case "oldest":
		sess.Asc("conversation.created_unix").Asc("conversation.id")
	case "recentupdate":
		sess.Desc("conversation.updated_unix").Desc("conversation.created_unix").Desc("conversation.id")
	case "leastupdate":
		sess.Asc("conversation.updated_unix").Asc("conversation.created_unix").Asc("conversation.id")
	case "mostcomment":
		sess.Desc("conversation.num_comments").Desc("conversation.created_unix").Desc("conversation.id")
	case "leastcomment":
		sess.Asc("conversation.num_comments").Desc("conversation.created_unix").Desc("conversation.id")
	case "priority":
		sess.Desc("conversation.priority").Desc("conversation.created_unix").Desc("conversation.id")
	case "nearduedate":
		// 253370764800 is 01/01/9999 @ 12:00am (UTC)
		sess.Join("LEFT", "milestone", "conversation.milestone_id = milestone.id").
			OrderBy("CASE " +
				"WHEN conversation.deadline_unix = 0 AND (milestone.deadline_unix = 0 OR milestone.deadline_unix IS NULL) THEN 253370764800 " +
				"WHEN milestone.deadline_unix = 0 OR milestone.deadline_unix IS NULL THEN conversation.deadline_unix " +
				"WHEN milestone.deadline_unix < conversation.deadline_unix OR conversation.deadline_unix = 0 THEN milestone.deadline_unix " +
				"ELSE conversation.deadline_unix END ASC").
			Desc("conversation.created_unix").
			Desc("conversation.id")
	case "farduedate":
		sess.Join("LEFT", "milestone", "conversation.milestone_id = milestone.id").
			OrderBy("CASE " +
				"WHEN milestone.deadline_unix IS NULL THEN conversation.deadline_unix " +
				"WHEN milestone.deadline_unix < conversation.deadline_unix OR conversation.deadline_unix = 0 THEN milestone.deadline_unix " +
				"ELSE conversation.deadline_unix END DESC").
			Desc("conversation.created_unix").
			Desc("conversation.id")
	case "priorityrepo":
		sess.OrderBy("CASE "+
			"WHEN conversation.repo_id = ? THEN 1 "+
			"ELSE 2 END ASC", priorityRepoID).
			Desc("conversation.created_unix").
			Desc("conversation.id")
	case "project-column-sorting":
		sess.Asc("project_conversation.sorting").Desc("conversation.created_unix").Desc("conversation.id")
	default:
		sess.Desc("conversation.created_unix").Desc("conversation.id")
	}
}

func applyLimit(sess *xorm.Session, opts *ConversationsOptions) {
	if opts.Paginator == nil || opts.Paginator.IsListAll() {
		return
	}

	start := 0
	if opts.Paginator.Page > 1 {
		start = (opts.Paginator.Page - 1) * opts.Paginator.PageSize
	}
	sess.Limit(opts.Paginator.PageSize, start)
}

func applyRepoConditions(sess *xorm.Session, opts *ConversationsOptions) {
	if len(opts.RepoIDs) == 1 {
		opts.RepoCond = builder.Eq{"conversation.repo_id": opts.RepoIDs[0]}
	} else if len(opts.RepoIDs) > 1 {
		opts.RepoCond = builder.In("conversation.repo_id", opts.RepoIDs)
	}
	if opts.AllPublic {
		if opts.RepoCond == nil {
			opts.RepoCond = builder.NewCond()
		}
		opts.RepoCond = opts.RepoCond.Or(builder.In("conversation.repo_id", builder.Select("id").From("repository").Where(builder.Eq{"is_private": false})))
	}
	if opts.RepoCond != nil {
		sess.And(opts.RepoCond)
	}
}

func applyConditions(sess *xorm.Session, opts *ConversationsOptions) {
	if len(opts.ConversationIDs) > 0 {
		sess.In("conversation.id", opts.ConversationIDs)
	}

	applyRepoConditions(sess, opts)

	if opts.UpdatedAfterUnix != 0 {
		sess.And(builder.Gte{"conversation.updated_unix": opts.UpdatedAfterUnix})
	}
	if opts.UpdatedBeforeUnix != 0 {
		sess.And(builder.Lte{"conversation.updated_unix": opts.UpdatedBeforeUnix})
	}

	if opts.IsArchived.Has() {
		sess.And(builder.Eq{"repository.is_archived": opts.IsArchived.Value()})
	}
}

// Conversations returns a list of conversations by given conditions.
func Conversations(ctx context.Context, opts *ConversationsOptions) (ConversationList, error) {
	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`conversation`.repo_id = `repository`.id")
	applyLimit(sess, opts)
	applyConditions(sess, opts)
	applySorts(sess, opts.SortType, opts.PriorityRepoID)

	conversations := ConversationList{}
	if err := sess.Find(&conversations); err != nil {
		return nil, fmt.Errorf("unable to query Conversations: %w", err)
	}

	if err := conversations.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("unable to LoadAttributes for Conversations: %w", err)
	}

	return conversations, nil
}

// ConversationIDs returns a list of conversation ids by given conditions.
func ConversationIDs(ctx context.Context, opts *ConversationsOptions, otherConds ...builder.Cond) ([]int64, int64, error) {
	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`conversation`.repo_id = `repository`.id")
	applyConditions(sess, opts)
	for _, cond := range otherConds {
		sess.And(cond)
	}

	applyLimit(sess, opts)
	applySorts(sess, opts.SortType, opts.PriorityRepoID)

	var res []int64
	total, err := sess.Select("`conversation`.id").Table(&Conversation{}).FindAndCount(&res)
	if err != nil {
		return nil, 0, err
	}

	return res, total, nil
}
