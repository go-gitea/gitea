// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/optional"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// ConversationsOptions represents options of an conversation.
type ConversationsOptions struct { //nolint
	Paginator          *db.ListOptions
	RepoIDs            []int64 // overwrites RepoCond if the length is not 0
	AllPublic          bool    // include also all public repositories
	RepoCond           builder.Cond
	AssigneeID         int64
	PosterID           int64
	MentionedID        int64
	ReviewRequestedID  int64
	ReviewedID         int64
	SubscriberID       int64
	MilestoneIDs       []int64
	ProjectID          int64
	ProjectColumnID    int64
	IsClosed           optional.Option[bool]
	IsPull             optional.Option[bool]
	LabelIDs           []int64
	IncludedLabelNames []string
	ExcludedLabelNames []string
	IncludeMilestones  []string
	SortType           string
	ConversationIDs    []int64
	UpdatedAfterUnix   int64
	UpdatedBeforeUnix  int64
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

func applyLabelsCondition(sess *xorm.Session, opts *ConversationsOptions) {
	if len(opts.LabelIDs) > 0 {
		if opts.LabelIDs[0] == 0 {
			sess.Where("conversation.id NOT IN (SELECT conversation_id FROM conversation_label)")
		} else {
			// deduplicate the label IDs for inclusion and exclusion
			includedLabelIDs := make(container.Set[int64])
			excludedLabelIDs := make(container.Set[int64])
			for _, labelID := range opts.LabelIDs {
				if labelID > 0 {
					includedLabelIDs.Add(labelID)
				} else if labelID < 0 { // 0 is not supported here, so just ignore it
					excludedLabelIDs.Add(-labelID)
				}
			}
			// ... and use them in a subquery of the form :
			//  where (select count(*) from conversation_label where conversation_id=conversation.id and label_id in (2, 4, 6)) = 3
			// This equality is guaranteed thanks to unique index (conversation_id,label_id) on table conversation_label.
			if len(includedLabelIDs) > 0 {
				subQuery := builder.Select("count(*)").From("conversation_label").Where(builder.Expr("conversation_id = conversation.id")).
					And(builder.In("label_id", includedLabelIDs.Values()))
				sess.Where(builder.Eq{strconv.Itoa(len(includedLabelIDs)): subQuery})
			}
			// or (select count(*)...) = 0 for excluded labels
			if len(excludedLabelIDs) > 0 {
				subQuery := builder.Select("count(*)").From("conversation_label").Where(builder.Expr("conversation_id = conversation.id")).
					And(builder.In("label_id", excludedLabelIDs.Values()))
				sess.Where(builder.Eq{"0": subQuery})
			}
		}
	}
}

func applyMilestoneCondition(sess *xorm.Session, opts *ConversationsOptions) {
	if len(opts.MilestoneIDs) == 1 && opts.MilestoneIDs[0] == db.NoConditionID {
		sess.And("conversation.milestone_id = 0")
	} else if len(opts.MilestoneIDs) > 0 {
		sess.In("conversation.milestone_id", opts.MilestoneIDs)
	}

	if len(opts.IncludeMilestones) > 0 {
		sess.In("conversation.milestone_id",
			builder.Select("id").
				From("milestone").
				Where(builder.In("name", opts.IncludeMilestones)))
	}
}

func applyProjectCondition(sess *xorm.Session, opts *ConversationsOptions) {
	if opts.ProjectID > 0 { // specific project
		sess.Join("INNER", "project_conversation", "conversation.id = project_conversation.conversation_id").
			And("project_conversation.project_id=?", opts.ProjectID)
	} else if opts.ProjectID == db.NoConditionID { // show those that are in no project
		sess.And(builder.NotIn("conversation.id", builder.Select("conversation_id").From("project_conversation").And(builder.Neq{"project_id": 0})))
	}
	// opts.ProjectID == 0 means all projects,
	// do not need to apply any condition
}

func applyProjectColumnCondition(sess *xorm.Session, opts *ConversationsOptions) {
	// opts.ProjectColumnID == 0 means all project columns,
	// do not need to apply any condition
	if opts.ProjectColumnID > 0 {
		sess.In("conversation.id", builder.Select("conversation_id").From("project_conversation").Where(builder.Eq{"project_board_id": opts.ProjectColumnID}))
	} else if opts.ProjectColumnID == db.NoConditionID {
		sess.In("conversation.id", builder.Select("conversation_id").From("project_conversation").Where(builder.Eq{"project_board_id": 0}))
	}
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

	if opts.IsClosed.Has() {
		sess.And("conversation.is_closed=?", opts.IsClosed.Value())
	}

	if opts.PosterID > 0 {
		applyPosterCondition(sess, opts.PosterID)
	}

	if opts.MentionedID > 0 {
		applyMentionedCondition(sess, opts.MentionedID)
	}

	if opts.SubscriberID > 0 {
		applySubscribedCondition(sess, opts.SubscriberID)
	}

	applyMilestoneCondition(sess, opts)

	if opts.UpdatedAfterUnix != 0 {
		sess.And(builder.Gte{"conversation.updated_unix": opts.UpdatedAfterUnix})
	}
	if opts.UpdatedBeforeUnix != 0 {
		sess.And(builder.Lte{"conversation.updated_unix": opts.UpdatedBeforeUnix})
	}

	applyProjectCondition(sess, opts)

	applyProjectColumnCondition(sess, opts)

	if opts.IsPull.Has() {
		sess.And("conversation.is_pull=?", opts.IsPull.Value())
	}

	if opts.IsArchived.Has() {
		sess.And(builder.Eq{"repository.is_archived": opts.IsArchived.Value()})
	}

	applyLabelsCondition(sess, opts)
}

// teamUnitsRepoCond returns query condition for those repo id in the special org team with special units access
func teamUnitsRepoCond(id string, userID, orgID, teamID int64, units ...unit.Type) builder.Cond {
	return builder.In(id,
		builder.Select("repo_id").From("team_repo").Where(
			builder.Eq{
				"team_id": teamID,
			}.And(
				builder.Or(
					// Check if the user is member of the team.
					builder.In(
						"team_id", builder.Select("team_id").From("team_user").Where(
							builder.Eq{
								"uid": userID,
							},
						),
					),
					// Check if the user is in the owner team of the organisation.
					builder.Exists(builder.Select("team_id").From("team_user").
						Where(builder.Eq{
							"org_id": orgID,
							"team_id": builder.Select("id").From("team").Where(
								builder.Eq{
									"org_id":     orgID,
									"lower_name": strings.ToLower(organization.OwnerTeamName),
								}),
							"uid": userID,
						}),
					),
				)).And(
				builder.In(
					"team_id", builder.Select("team_id").From("team_unit").Where(
						builder.Eq{
							"`team_unit`.org_id": orgID,
						}.And(
							builder.In("`team_unit`.type", units),
						),
					),
				),
			),
		))
}

func applyPosterCondition(sess *xorm.Session, posterID int64) {
	sess.And("conversation.poster_id=?", posterID)
}

func applyMentionedCondition(sess *xorm.Session, mentionedID int64) {
	sess.Join("INNER", "conversation_user", "conversation.id = conversation_user.conversation_id").
		And("conversation_user.is_mentioned = ?", true).
		And("conversation_user.uid = ?", mentionedID)
}

func applySubscribedCondition(sess *xorm.Session, subscriberID int64) {
	sess.And(
		builder.
			NotIn("conversation.id",
				builder.Select("conversation_id").
					From("conversation_watch").
					Where(builder.Eq{"is_watching": false, "user_id": subscriberID}),
			),
	).And(
		builder.Or(
			builder.In("conversation.id", builder.
				Select("conversation_id").
				From("conversation_watch").
				Where(builder.Eq{"is_watching": true, "user_id": subscriberID}),
			),
			builder.In("conversation.id", builder.
				Select("conversation_id").
				From("comment").
				Where(builder.Eq{"poster_id": subscriberID}),
			),
			builder.Eq{"conversation.poster_id": subscriberID},
			builder.In("conversation.repo_id", builder.
				Select("id").
				From("watch").
				Where(builder.And(builder.Eq{"user_id": subscriberID},
					builder.In("mode", repo_model.WatchModeNormal, repo_model.WatchModeAuto))),
			),
		),
	)
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
