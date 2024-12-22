// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
	organization_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/optional"

	"xorm.io/builder"
)

type ReviewList []*Review

// LoadReviewers loads reviewers
func (reviews ReviewList) LoadReviewers(ctx context.Context) error {
	reviewerIDs := make([]int64, len(reviews))
	for i := 0; i < len(reviews); i++ {
		reviewerIDs[i] = reviews[i].ReviewerID
	}
	reviewers, err := user_model.GetPossibleUserByIDs(ctx, reviewerIDs)
	if err != nil {
		return err
	}

	userMap := make(map[int64]*user_model.User, len(reviewers))
	for _, reviewer := range reviewers {
		userMap[reviewer.ID] = reviewer
	}
	for _, review := range reviews {
		review.Reviewer = userMap[review.ReviewerID]
	}
	return nil
}

// LoadReviewersTeams loads reviewers teams
func (reviews ReviewList) LoadReviewersTeams(ctx context.Context) error {
	reviewersTeamsIDs := make([]int64, 0)
	for _, review := range reviews {
		if review.ReviewerTeamID != 0 {
			reviewersTeamsIDs = append(reviewersTeamsIDs, review.ReviewerTeamID)
		}
	}

	teamsMap, err := organization_model.GetTeamsByIDs(ctx, reviewersTeamsIDs)
	if err != nil {
		return err
	}

	for _, review := range reviews {
		if review.ReviewerTeamID != 0 {
			review.ReviewerTeam = teamsMap[review.ReviewerTeamID]
		}
	}

	return nil
}

func (reviews ReviewList) LoadIssues(ctx context.Context) error {
	issueIDs := container.FilterSlice(reviews, func(review *Review) (int64, bool) {
		return review.IssueID, true
	})

	issues, err := GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		return err
	}
	if _, err := issues.LoadRepositories(ctx); err != nil {
		return err
	}
	issueMap := make(map[int64]*Issue, len(issues))
	for _, issue := range issues {
		issueMap[issue.ID] = issue
	}

	for _, review := range reviews {
		review.Issue = issueMap[review.IssueID]
	}
	return nil
}

// FindReviewOptions represent possible filters to find reviews
type FindReviewOptions struct {
	db.ListOptions
	Types        []ReviewType
	IssueID      int64
	ReviewerID   int64
	OfficialOnly bool
	Dismissed    optional.Option[bool]
}

func (opts *FindReviewOptions) toCond() builder.Cond {
	cond := builder.NewCond()
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"issue_id": opts.IssueID})
	}
	if opts.ReviewerID > 0 {
		cond = cond.And(builder.Eq{"reviewer_id": opts.ReviewerID})
	}
	if len(opts.Types) > 0 {
		cond = cond.And(builder.In("type", opts.Types))
	}
	if opts.OfficialOnly {
		cond = cond.And(builder.Eq{"official": true})
	}
	if opts.Dismissed.Has() {
		cond = cond.And(builder.Eq{"dismissed": opts.Dismissed.Value()})
	}
	return cond
}

// FindReviews returns reviews passing FindReviewOptions
func FindReviews(ctx context.Context, opts FindReviewOptions) (ReviewList, error) {
	reviews := make([]*Review, 0, 10)
	sess := db.GetEngine(ctx).Where(opts.toCond())
	if opts.Page > 0 && !opts.IsListAll() {
		sess = db.SetSessionPagination(sess, &opts)
	}
	return reviews, sess.
		Asc("created_unix").
		Asc("id").
		Find(&reviews)
}

// FindLatestReviews returns only latest reviews per user, passing FindReviewOptions
func FindLatestReviews(ctx context.Context, opts FindReviewOptions) (ReviewList, error) {
	reviews := make([]*Review, 0, 10)
	cond := opts.toCond()
	sess := db.GetEngine(ctx).Where(cond)
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, &opts)
	}

	sess.In("id", builder.
		Select("max(id)").
		From("review").
		Where(cond).
		GroupBy("reviewer_id"))

	return reviews, sess.
		Asc("created_unix").
		Asc("id").
		Find(&reviews)
}

// CountReviews returns count of reviews passing FindReviewOptions
func CountReviews(ctx context.Context, opts FindReviewOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toCond()).Count(&Review{})
}

// GetReviewersFromOriginalAuthorsByIssueID gets the latest review of each original authors for a pull request
func GetReviewersFromOriginalAuthorsByIssueID(ctx context.Context, issueID int64) (ReviewList, error) {
	reviews := make([]*Review, 0, 10)

	// Get latest review of each reviewer, sorted in order they were made
	if err := db.GetEngine(ctx).SQL("SELECT * FROM review WHERE id IN (SELECT max(id) as id FROM review WHERE issue_id = ? AND reviewer_team_id = 0 AND type in (?, ?, ?) AND original_author_id <> 0 GROUP BY issue_id, original_author_id) ORDER BY review.updated_unix ASC",
		issueID, ReviewTypeApprove, ReviewTypeReject, ReviewTypeRequest).
		Find(&reviews); err != nil {
		return nil, err
	}

	return reviews, nil
}

// GetReviewsByIssueID gets the latest review of each reviewer for a pull request
func GetReviewsByIssueID(ctx context.Context, issueID int64) (ReviewList, error) {
	reviews := make([]*Review, 0, 10)

	sess := db.GetEngine(ctx)

	// Get latest review of each reviewer, sorted in order they were made
	if err := sess.SQL("SELECT * FROM review WHERE id IN (SELECT max(id) as id FROM review WHERE issue_id = ? AND reviewer_team_id = 0 AND type in (?, ?, ?) AND dismissed = ? AND original_author_id = 0 GROUP BY issue_id, reviewer_id) ORDER BY review.updated_unix ASC",
		issueID, ReviewTypeApprove, ReviewTypeReject, ReviewTypeRequest, false).
		Find(&reviews); err != nil {
		return nil, err
	}

	teamReviewRequests := make([]*Review, 0, 5)
	if err := sess.SQL("SELECT * FROM review WHERE id IN (SELECT max(id) as id FROM review WHERE issue_id = ? AND reviewer_team_id <> 0 AND original_author_id = 0 GROUP BY issue_id, reviewer_team_id) ORDER BY review.updated_unix ASC",
		issueID).
		Find(&teamReviewRequests); err != nil {
		return nil, err
	}

	if len(teamReviewRequests) > 0 {
		reviews = append(reviews, teamReviewRequests...)
	}

	return reviews, nil
}
