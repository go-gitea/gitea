// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// IssuesOptions represents options of an issue.
type IssuesOptions struct { //nolint
	db.ListOptions
	RepoIDs            []int64 // overwrites RepoCond if the length is not 0
	RepoCond           builder.Cond
	AssigneeID         int64
	PosterID           int64
	MentionedID        int64
	ReviewRequestedID  int64
	ReviewedID         int64
	SubscriberID       int64
	MilestoneIDs       []int64
	ProjectID          int64
	ProjectBoardID     int64
	IsClosed           util.OptionalBool
	IsPull             util.OptionalBool
	LabelIDs           []int64
	IncludedLabelNames []string
	ExcludedLabelNames []string
	IncludeMilestones  []string
	SortType           string
	IssueIDs           []int64
	UpdatedAfterUnix   int64
	UpdatedBeforeUnix  int64
	// prioritize issues from this repo
	PriorityRepoID int64
	IsArchived     util.OptionalBool
	Org            *organization.Organization // issues permission scope
	Team           *organization.Team         // issues permission scope
	User           *user_model.User           // issues permission scope
}

// applySorts sort an issues-related session based on the provided
// sortType string
func applySorts(sess *xorm.Session, sortType string, priorityRepoID int64) {
	switch sortType {
	case "oldest":
		sess.Asc("issue.created_unix").Asc("issue.id")
	case "recentupdate":
		sess.Desc("issue.updated_unix").Desc("issue.created_unix").Desc("issue.id")
	case "leastupdate":
		sess.Asc("issue.updated_unix").Asc("issue.created_unix").Asc("issue.id")
	case "mostcomment":
		sess.Desc("issue.num_comments").Desc("issue.created_unix").Desc("issue.id")
	case "leastcomment":
		sess.Asc("issue.num_comments").Desc("issue.created_unix").Desc("issue.id")
	case "priority":
		sess.Desc("issue.priority").Desc("issue.created_unix").Desc("issue.id")
	case "nearduedate":
		// 253370764800 is 01/01/9999 @ 12:00am (UTC)
		sess.Join("LEFT", "milestone", "issue.milestone_id = milestone.id").
			OrderBy("CASE " +
				"WHEN issue.deadline_unix = 0 AND (milestone.deadline_unix = 0 OR milestone.deadline_unix IS NULL) THEN 253370764800 " +
				"WHEN milestone.deadline_unix = 0 OR milestone.deadline_unix IS NULL THEN issue.deadline_unix " +
				"WHEN milestone.deadline_unix < issue.deadline_unix OR issue.deadline_unix = 0 THEN milestone.deadline_unix " +
				"ELSE issue.deadline_unix END ASC").
			Desc("issue.created_unix").
			Desc("issue.id")
	case "farduedate":
		sess.Join("LEFT", "milestone", "issue.milestone_id = milestone.id").
			OrderBy("CASE " +
				"WHEN milestone.deadline_unix IS NULL THEN issue.deadline_unix " +
				"WHEN milestone.deadline_unix < issue.deadline_unix OR issue.deadline_unix = 0 THEN milestone.deadline_unix " +
				"ELSE issue.deadline_unix END DESC").
			Desc("issue.created_unix").
			Desc("issue.id")
	case "priorityrepo":
		sess.OrderBy("CASE "+
			"WHEN issue.repo_id = ? THEN 1 "+
			"ELSE 2 END ASC", priorityRepoID).
			Desc("issue.created_unix").
			Desc("issue.id")
	case "project-column-sorting":
		sess.Asc("project_issue.sorting").Desc("issue.created_unix").Desc("issue.id")
	default:
		sess.Desc("issue.created_unix").Desc("issue.id")
	}
}

func applyLimit(sess *xorm.Session, opts *IssuesOptions) *xorm.Session {
	if opts.Page >= 0 && opts.PageSize > 0 {
		var start int
		if opts.Page == 0 {
			start = 0
		} else {
			start = (opts.Page - 1) * opts.PageSize
		}
		sess.Limit(opts.PageSize, start)
	}
	return sess
}

func applyLabelsCondition(sess *xorm.Session, opts *IssuesOptions) *xorm.Session {
	if len(opts.LabelIDs) > 0 {
		if opts.LabelIDs[0] == 0 {
			sess.Where("issue.id NOT IN (SELECT issue_id FROM issue_label)")
		} else {
			for i, labelID := range opts.LabelIDs {
				if labelID > 0 {
					sess.Join("INNER", fmt.Sprintf("issue_label il%d", i),
						fmt.Sprintf("issue.id = il%[1]d.issue_id AND il%[1]d.label_id = %[2]d", i, labelID))
				} else if labelID < 0 { // 0 is not supported here, so just ignore it
					sess.Where("issue.id not in (select issue_id from issue_label where label_id = ?)", -labelID)
				}
			}
		}
	}

	if len(opts.IncludedLabelNames) > 0 {
		sess.In("issue.id", BuildLabelNamesIssueIDsCondition(opts.IncludedLabelNames))
	}

	if len(opts.ExcludedLabelNames) > 0 {
		sess.And(builder.NotIn("issue.id", BuildLabelNamesIssueIDsCondition(opts.ExcludedLabelNames)))
	}

	return sess
}

func applyMilestoneCondition(sess *xorm.Session, opts *IssuesOptions) *xorm.Session {
	if len(opts.MilestoneIDs) == 1 && opts.MilestoneIDs[0] == db.NoConditionID {
		sess.And("issue.milestone_id = 0")
	} else if len(opts.MilestoneIDs) > 0 {
		sess.In("issue.milestone_id", opts.MilestoneIDs)
	}

	if len(opts.IncludeMilestones) > 0 {
		sess.In("issue.milestone_id",
			builder.Select("id").
				From("milestone").
				Where(builder.In("name", opts.IncludeMilestones)))
	}

	return sess
}

func applyRepoConditions(sess *xorm.Session, opts *IssuesOptions) *xorm.Session {
	if len(opts.RepoIDs) == 1 {
		opts.RepoCond = builder.Eq{"issue.repo_id": opts.RepoIDs[0]}
	} else if len(opts.RepoIDs) > 1 {
		opts.RepoCond = builder.In("issue.repo_id", opts.RepoIDs)
	}
	if opts.RepoCond != nil {
		sess.And(opts.RepoCond)
	}
	return sess
}

func applyConditions(sess *xorm.Session, opts *IssuesOptions) *xorm.Session {
	if len(opts.IssueIDs) > 0 {
		sess.In("issue.id", opts.IssueIDs)
	}

	applyRepoConditions(sess, opts)

	if !opts.IsClosed.IsNone() {
		sess.And("issue.is_closed=?", opts.IsClosed.IsTrue())
	}

	if opts.AssigneeID > 0 {
		applyAssigneeCondition(sess, opts.AssigneeID)
	} else if opts.AssigneeID == db.NoConditionID {
		sess.Where("issue.id NOT IN (SELECT issue_id FROM issue_assignees)")
	}

	if opts.PosterID > 0 {
		applyPosterCondition(sess, opts.PosterID)
	}

	if opts.MentionedID > 0 {
		applyMentionedCondition(sess, opts.MentionedID)
	}

	if opts.ReviewRequestedID > 0 {
		applyReviewRequestedCondition(sess, opts.ReviewRequestedID)
	}

	if opts.ReviewedID > 0 {
		applyReviewedCondition(sess, opts.ReviewedID)
	}

	if opts.SubscriberID > 0 {
		applySubscribedCondition(sess, opts.SubscriberID)
	}

	applyMilestoneCondition(sess, opts)

	if opts.UpdatedAfterUnix != 0 {
		sess.And(builder.Gte{"issue.updated_unix": opts.UpdatedAfterUnix})
	}
	if opts.UpdatedBeforeUnix != 0 {
		sess.And(builder.Lte{"issue.updated_unix": opts.UpdatedBeforeUnix})
	}

	if opts.ProjectID > 0 {
		sess.Join("INNER", "project_issue", "issue.id = project_issue.issue_id").
			And("project_issue.project_id=?", opts.ProjectID)
	} else if opts.ProjectID == db.NoConditionID { // show those that are in no project
		sess.And(builder.NotIn("issue.id", builder.Select("issue_id").From("project_issue")))
	}

	if opts.ProjectBoardID != 0 {
		if opts.ProjectBoardID > 0 {
			sess.In("issue.id", builder.Select("issue_id").From("project_issue").Where(builder.Eq{"project_board_id": opts.ProjectBoardID}))
		} else {
			sess.In("issue.id", builder.Select("issue_id").From("project_issue").Where(builder.Eq{"project_board_id": 0}))
		}
	}

	switch opts.IsPull {
	case util.OptionalBoolTrue:
		sess.And("issue.is_pull=?", true)
	case util.OptionalBoolFalse:
		sess.And("issue.is_pull=?", false)
	}

	if opts.IsArchived != util.OptionalBoolNone {
		sess.And(builder.Eq{"repository.is_archived": opts.IsArchived.IsTrue()})
	}

	applyLabelsCondition(sess, opts)

	if opts.User != nil {
		sess.And(issuePullAccessibleRepoCond("issue.repo_id", opts.User.ID, opts.Org, opts.Team, opts.IsPull.IsTrue()))
	}

	return sess
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

// issuePullAccessibleRepoCond userID must not be zero, this condition require join repository table
func issuePullAccessibleRepoCond(repoIDstr string, userID int64, org *organization.Organization, team *organization.Team, isPull bool) builder.Cond {
	cond := builder.NewCond()
	unitType := unit.TypeIssues
	if isPull {
		unitType = unit.TypePullRequests
	}
	if org != nil {
		if team != nil {
			cond = cond.And(teamUnitsRepoCond(repoIDstr, userID, org.ID, team.ID, unitType)) // special team member repos
		} else {
			cond = cond.And(
				builder.Or(
					repo_model.UserOrgUnitRepoCond(repoIDstr, userID, org.ID, unitType), // team member repos
					repo_model.UserOrgPublicUnitRepoCond(userID, org.ID),                // user org public non-member repos, TODO: check repo has issues
				),
			)
		}
	} else {
		cond = cond.And(
			builder.Or(
				repo_model.UserOwnedRepoCond(userID),                          // owned repos
				repo_model.UserAccessRepoCond(repoIDstr, userID),              // user can access repo in a unit independent way
				repo_model.UserAssignedRepoCond(repoIDstr, userID),            // user has been assigned accessible public repos
				repo_model.UserMentionedRepoCond(repoIDstr, userID),           // user has been mentioned accessible public repos
				repo_model.UserCreateIssueRepoCond(repoIDstr, userID, isPull), // user has created issue/pr accessible public repos
			),
		)
	}
	return cond
}

func applyAssigneeCondition(sess *xorm.Session, assigneeID int64) *xorm.Session {
	return sess.Join("INNER", "issue_assignees", "issue.id = issue_assignees.issue_id").
		And("issue_assignees.assignee_id = ?", assigneeID)
}

func applyPosterCondition(sess *xorm.Session, posterID int64) *xorm.Session {
	return sess.And("issue.poster_id=?", posterID)
}

func applyMentionedCondition(sess *xorm.Session, mentionedID int64) *xorm.Session {
	return sess.Join("INNER", "issue_user", "issue.id = issue_user.issue_id").
		And("issue_user.is_mentioned = ?", true).
		And("issue_user.uid = ?", mentionedID)
}

func applyReviewRequestedCondition(sess *xorm.Session, reviewRequestedID int64) *xorm.Session {
	return sess.Join("INNER", []string{"review", "r"}, "issue.id = r.issue_id").
		And("issue.poster_id <> ?", reviewRequestedID).
		And("r.type = ?", ReviewTypeRequest).
		And("r.reviewer_id = ? and r.id in (select max(id) from review where issue_id = r.issue_id and reviewer_id = r.reviewer_id and type in (?, ?, ?))"+
			" or r.reviewer_team_id in (select team_id from team_user where uid = ?)",
			reviewRequestedID, ReviewTypeApprove, ReviewTypeReject, ReviewTypeRequest, reviewRequestedID)
}

func applyReviewedCondition(sess *xorm.Session, reviewedID int64) *xorm.Session {
	// Query for pull requests where you are a reviewer or commenter, excluding
	// any pull requests already returned by the the review requested filter.
	notPoster := builder.Neq{"issue.poster_id": reviewedID}
	reviewed := builder.In("issue.id", builder.
		Select("issue_id").
		From("review").
		Where(builder.And(
			builder.Neq{"type": ReviewTypeRequest},
			builder.Or(
				builder.Eq{"reviewer_id": reviewedID},
				builder.In("reviewer_team_id", builder.
					Select("team_id").
					From("team_user").
					Where(builder.Eq{"uid": reviewedID}),
				),
			),
		)),
	)
	commented := builder.In("issue.id", builder.
		Select("issue_id").
		From("comment").
		Where(builder.And(
			builder.Eq{"poster_id": reviewedID},
			builder.In("type", CommentTypeComment, CommentTypeCode, CommentTypeReview),
		)),
	)
	return sess.And(notPoster, builder.Or(reviewed, commented))
}

func applySubscribedCondition(sess *xorm.Session, subscriberID int64) *xorm.Session {
	return sess.And(
		builder.
			NotIn("issue.id",
				builder.Select("issue_id").
					From("issue_watch").
					Where(builder.Eq{"is_watching": false, "user_id": subscriberID}),
			),
	).And(
		builder.Or(
			builder.In("issue.id", builder.
				Select("issue_id").
				From("issue_watch").
				Where(builder.Eq{"is_watching": true, "user_id": subscriberID}),
			),
			builder.In("issue.id", builder.
				Select("issue_id").
				From("comment").
				Where(builder.Eq{"poster_id": subscriberID}),
			),
			builder.Eq{"issue.poster_id": subscriberID},
			builder.In("issue.repo_id", builder.
				Select("id").
				From("watch").
				Where(builder.And(builder.Eq{"user_id": subscriberID},
					builder.In("mode", repo_model.WatchModeNormal, repo_model.WatchModeAuto))),
			),
		),
	)
}

// GetRepoIDsForIssuesOptions find all repo ids for the given options
func GetRepoIDsForIssuesOptions(opts *IssuesOptions, user *user_model.User) ([]int64, error) {
	repoIDs := make([]int64, 0, 5)
	e := db.GetEngine(db.DefaultContext)

	sess := e.Join("INNER", "repository", "`issue`.repo_id = `repository`.id")

	applyConditions(sess, opts)

	accessCond := repo_model.AccessibleRepositoryCondition(user, unit.TypeInvalid)
	if err := sess.Where(accessCond).
		Distinct("issue.repo_id").
		Table("issue").
		Find(&repoIDs); err != nil {
		return nil, fmt.Errorf("unable to GetRepoIDsForIssuesOptions: %w", err)
	}

	return repoIDs, nil
}

// Issues returns a list of issues by given conditions.
func Issues(ctx context.Context, opts *IssuesOptions) ([]*Issue, error) {
	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`issue`.repo_id = `repository`.id")
	applyLimit(sess, opts)
	applyConditions(sess, opts)
	applySorts(sess, opts.SortType, opts.PriorityRepoID)

	issues := make(IssueList, 0, opts.ListOptions.PageSize)
	if err := sess.Find(&issues); err != nil {
		return nil, fmt.Errorf("unable to query Issues: %w", err)
	}

	if err := issues.LoadAttributes(); err != nil {
		return nil, fmt.Errorf("unable to LoadAttributes for Issues: %w", err)
	}

	return issues, nil
}

// SearchIssueIDsByKeyword search issues on database
func SearchIssueIDsByKeyword(ctx context.Context, kw string, repoIDs []int64, limit, start int) (int64, []int64, error) {
	repoCond := builder.In("repo_id", repoIDs)
	subQuery := builder.Select("id").From("issue").Where(repoCond)
	cond := builder.And(
		repoCond,
		builder.Or(
			db.BuildCaseInsensitiveLike("name", kw),
			db.BuildCaseInsensitiveLike("content", kw),
			builder.In("id", builder.Select("issue_id").
				From("comment").
				Where(builder.And(
					builder.Eq{"type": CommentTypeComment},
					builder.In("issue_id", subQuery),
					db.BuildCaseInsensitiveLike("content", kw),
				)),
			),
		),
	)

	ids := make([]int64, 0, limit)
	res := make([]struct {
		ID          int64
		UpdatedUnix int64
	}, 0, limit)
	err := db.GetEngine(ctx).Distinct("id", "updated_unix").Table("issue").Where(cond).
		OrderBy("`updated_unix` DESC").Limit(limit, start).
		Find(&res)
	if err != nil {
		return 0, nil, err
	}
	for _, r := range res {
		ids = append(ids, r.ID)
	}

	total, err := db.GetEngine(ctx).Distinct("id").Table("issue").Where(cond).Count()
	if err != nil {
		return 0, nil, err
	}

	return total, ids, nil
}
