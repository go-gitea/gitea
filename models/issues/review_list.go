// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"slices"
	"sort"

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

// GetReviewsByIssueID gets the latest review of each reviewer for a pull request
// The first returned parameter is the latest review of each individual reviewer or team
// The second returned parameter is the latest review of each original author which is migrated from other systems
// The reviews are sorted by updated time
func GetReviewsByIssueID(ctx context.Context, issueID int64) (latestReviews, migratedOriginalReviews ReviewList, err error) {
	reviews := make([]*Review, 0, 10)

	// Get all reviews for the issue id
	if err := db.GetEngine(ctx).Where("issue_id=?", issueID).OrderBy("updated_unix ASC").Find(&reviews); err != nil {
		return nil, nil, err
	}

	// filter them in memory to get the latest review of each reviewer
	// Since the reviews should not be too many for one issue, less than 100 commonly, it's acceptable to do this in memory
	// And since there are too less indexes in review table, it will be very slow to filter in the database
	reviewersMap := make(map[int64][]*Review)         // key is reviewer id
	originalReviewersMap := make(map[int64][]*Review) // key is original author id
	reviewTeamsMap := make(map[int64][]*Review)       // key is reviewer team id
	countedReivewTypes := []ReviewType{ReviewTypeApprove, ReviewTypeReject, ReviewTypeRequest}
	for _, review := range reviews {
		if review.ReviewerTeamID == 0 && slices.Contains(countedReivewTypes, review.Type) && !review.Dismissed {
			if review.OriginalAuthorID != 0 {
				originalReviewersMap[review.OriginalAuthorID] = append(originalReviewersMap[review.OriginalAuthorID], review)
			} else {
				reviewersMap[review.ReviewerID] = append(reviewersMap[review.ReviewerID], review)
			}
		} else if review.ReviewerTeamID != 0 && review.OriginalAuthorID == 0 {
			reviewTeamsMap[review.ReviewerTeamID] = append(reviewTeamsMap[review.ReviewerTeamID], review)
		}
	}

	individualReviews := make([]*Review, 0, 10)
	for _, reviews := range reviewersMap {
		individualReviews = append(individualReviews, reviews[len(reviews)-1])
	}
	sort.Slice(individualReviews, func(i, j int) bool {
		return individualReviews[i].UpdatedUnix < individualReviews[j].UpdatedUnix
	})

	originalReviews := make([]*Review, 0, 10)
	for _, reviews := range originalReviewersMap {
		originalReviews = append(originalReviews, reviews[len(reviews)-1])
	}
	sort.Slice(originalReviews, func(i, j int) bool {
		return originalReviews[i].UpdatedUnix < originalReviews[j].UpdatedUnix
	})

	teamReviewRequests := make([]*Review, 0, 5)
	for _, reviews := range reviewTeamsMap {
		teamReviewRequests = append(teamReviewRequests, reviews[len(reviews)-1])
	}
	sort.Slice(teamReviewRequests, func(i, j int) bool {
		return teamReviewRequests[i].UpdatedUnix < teamReviewRequests[j].UpdatedUnix
	})

	return append(individualReviews, teamReviewRequests...), originalReviews, nil
}
