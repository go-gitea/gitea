// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package pull

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

// ViewedState stores for a file in which state it is currently viewed
type ViewedState uint8

const (
	Unviewed   ViewedState = iota
	HasChanged             // cannot be set from the UI/ API, only internally
	Viewed
)

func (viewedState ViewedState) String() string {
	switch viewedState {
	case Unviewed:
		return "unviewed"
	case HasChanged:
		return "has-changed"
	case Viewed:
		return "viewed"
	default:
		return fmt.Sprintf("unknown(value=%d)", viewedState)
	}
}

// ReviewState stores for a user-PR-commit combination which files the user has already viewed
type ReviewState struct {
	ID           int64                  `xorm:"pk autoincr"`
	UserID       int64                  `xorm:"NOT NULL UNIQUE(pull_commit_user)"`
	PullID       int64                  `xorm:"NOT NULL INDEX UNIQUE(pull_commit_user) DEFAULT 0"` // Which PR was the review on?
	CommitSHA    string                 `xorm:"NOT NULL VARCHAR(40) UNIQUE(pull_commit_user)"`     // Which commit was the head commit for the review?
	UpdatedFiles map[string]ViewedState `xorm:"NOT NULL LONGTEXT JSON"`                            // Stores for each of the changed files of a PR whether they have been viewed, changed since last viewed, or not viewed
	UpdatedUnix  timeutil.TimeStamp     `xorm:"updated"`                                           // Is an accurate indicator of the order of commits as we do not expect it to be possible to make reviews on previous commits
}

func init() {
	db.RegisterModel(new(ReviewState))
}

// GetReviewState returns the ReviewState with all given values prefilled, whether or not it exists in the database.
// If the review didn't exist before in the database, it won't afterwards either.
// The returned boolean shows whether the review exists in the database
func GetReviewState(ctx context.Context, userID, pullID int64, commitSHA string) (*ReviewState, bool, error) {
	review := &ReviewState{UserID: userID, PullID: pullID, CommitSHA: commitSHA}
	has, err := db.GetEngine(ctx).Get(review)
	return review, has, err
}

// UpdateReviewState updates the given review inside the database, regardless of whether it existed before or not
// The given map of files with their viewed state will be merged with the previous review, if present
func UpdateReviewState(ctx context.Context, userID, pullID int64, commitSHA string, updatedFiles map[string]ViewedState) error {
	log.Trace("Updating review for user %d, repo %d, commit %s with the updated files %v.", userID, pullID, commitSHA, updatedFiles)

	review, exists, err := GetReviewState(ctx, userID, pullID, commitSHA)
	if err != nil {
		return err
	}

	if exists {
		review.UpdatedFiles = mergeFiles(review.UpdatedFiles, updatedFiles)
	} else if previousReview, err := getNewestReviewStateApartFrom(ctx, userID, pullID, commitSHA); err != nil {
		return err

		// Overwrite the viewed files of the previous review if present
	} else if previousReview != nil {
		review.UpdatedFiles = mergeFiles(previousReview.UpdatedFiles, updatedFiles)
	} else {
		review.UpdatedFiles = updatedFiles
	}

	// Insert or Update review
	engine := db.GetEngine(ctx)
	if !exists {
		log.Trace("Inserting new review for user %d, repo %d, commit %s with the updated files %v.", userID, pullID, commitSHA, review.UpdatedFiles)
		_, err := engine.Insert(review)
		return err
	}
	log.Trace("Updating already existing review with ID %d (user %d, repo %d, commit %s) with the updated files %v.", review.ID, userID, pullID, commitSHA, review.UpdatedFiles)
	_, err = engine.ID(review.ID).Update(&ReviewState{UpdatedFiles: review.UpdatedFiles})
	return err
}

// mergeFiles merges the given maps of files with their viewing state into one map.
// Values from oldFiles will be overridden with values from newFiles
func mergeFiles(oldFiles, newFiles map[string]ViewedState) map[string]ViewedState {
	if oldFiles == nil {
		return newFiles
	} else if newFiles == nil {
		return oldFiles
	}

	for file, viewed := range newFiles {
		oldFiles[file] = viewed
	}
	return oldFiles
}

// GetNewestReviewState gets the newest review of the current user in the current PR.
// The returned PR Review will be nil if the user has not yet reviewed this PR.
func GetNewestReviewState(ctx context.Context, userID, pullID int64) (*ReviewState, error) {
	var review ReviewState
	has, err := db.GetEngine(ctx).Where("user_id = ?", userID).And("pull_id = ?", pullID).OrderBy("updated_unix DESC").Get(&review)
	if err != nil || !has {
		return nil, err
	}
	return &review, err
}

// getNewestReviewStateApartFrom is like GetNewestReview, except that the second newest review will be returned if the newest review points at the given commit.
// The returned PR Review will be nil if the user has not yet reviewed this PR.
func getNewestReviewStateApartFrom(ctx context.Context, userID, pullID int64, commitSHA string) (*ReviewState, error) {
	var reviews []ReviewState
	err := db.GetEngine(ctx).Where("user_id = ?", userID).And("pull_id = ?", pullID).OrderBy("updated_unix DESC").Limit(2).Find(&reviews)
	// It would also be possible to use ".And("commit_sha != ?", commitSHA)" instead of the error handling below
	// However, benchmarks show drastically improved performance by not doing that

	// Error cases in which no review should be returned
	if err != nil || len(reviews) == 0 || (len(reviews) == 1 && reviews[0].CommitSHA == commitSHA) {
		return nil, err

		// The first review points at the commit to exclude, hence skip to the second review
	} else if len(reviews) >= 2 && reviews[0].CommitSHA == commitSHA {
		return &reviews[1], nil
	}

	// As we have no error cases left, the result must be the first element in the list
	return &reviews[0], nil
}
