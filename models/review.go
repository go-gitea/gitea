// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
	"xorm.io/core"
)

// ReviewType defines the sort of feedback a review gives
type ReviewType int

// ReviewTypeUnknown unknown review type
const ReviewTypeUnknown ReviewType = -1

const (
	// ReviewTypePending is a review which is not published yet
	ReviewTypePending ReviewType = iota
	// ReviewTypeApprove approves changes
	ReviewTypeApprove
	// ReviewTypeComment gives general feedback
	ReviewTypeComment
	// ReviewTypeReject gives feedback blocking merge
	ReviewTypeReject
)

// Icon returns the corresponding icon for the review type
func (rt ReviewType) Icon() string {
	switch rt {
	case ReviewTypeApprove:
		return "eye"
	case ReviewTypeReject:
		return "x"
	case ReviewTypeComment, ReviewTypeUnknown:
		return "comment"
	default:
		return "comment"
	}
}

// Review represents collection of code comments giving feedback for a PR
type Review struct {
	ID         int64 `xorm:"pk autoincr"`
	Type       ReviewType
	Reviewer   *User  `xorm:"-"`
	ReviewerID int64  `xorm:"index"`
	Issue      *Issue `xorm:"-"`
	IssueID    int64  `xorm:"index"`
	Content    string

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	// CodeComments are the initial code comments of the review
	CodeComments CodeComments `xorm:"-"`
}

func (r *Review) loadCodeComments(e Engine) (err error) {
	if r.CodeComments == nil {
		r.CodeComments, err = fetchCodeCommentsByReview(e, r.Issue, nil, r)
	}
	return
}

// LoadCodeComments loads CodeComments
func (r *Review) LoadCodeComments() error {
	return r.loadCodeComments(x)
}

func (r *Review) loadIssue(e Engine) (err error) {
	r.Issue, err = getIssueByID(e, r.IssueID)
	return
}

func (r *Review) loadReviewer(e Engine) (err error) {
	if r.ReviewerID == 0 {
		return nil
	}
	r.Reviewer, err = getUserByID(e, r.ReviewerID)
	return
}

// LoadReviewer loads reviewer
func (r *Review) LoadReviewer() error {
	return r.loadReviewer(x)
}

func (r *Review) loadAttributes(e Engine) (err error) {
	if err = r.loadReviewer(e); err != nil {
		return
	}
	if err = r.loadIssue(e); err != nil {
		return
	}
	return
}

// LoadAttributes loads all attributes except CodeComments
func (r *Review) LoadAttributes() error {
	return r.loadAttributes(x)
}

func getReviewByID(e Engine, id int64) (*Review, error) {
	review := new(Review)
	if has, err := e.ID(id).Get(review); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReviewNotExist{ID: id}
	} else {
		return review, nil
	}
}

// GetReviewByID returns the review by the given ID
func GetReviewByID(id int64) (*Review, error) {
	return getReviewByID(x, id)
}

func getUniqueApprovalsByPullRequestID(e Engine, prID int64) (reviews []*Review, err error) {
	reviews = make([]*Review, 0)
	if err := e.
		Where("issue_id = ? AND type = ?", prID, ReviewTypeApprove).
		OrderBy("updated_unix").
		GroupBy("reviewer_id").
		Find(&reviews); err != nil {
		return nil, err
	}
	return
}

// GetUniqueApprovalsByPullRequestID returns all reviews submitted for a specific pull request
func GetUniqueApprovalsByPullRequestID(prID int64) ([]*Review, error) {
	return getUniqueApprovalsByPullRequestID(x, prID)
}

// FindReviewOptions represent possible filters to find reviews
type FindReviewOptions struct {
	Type       ReviewType
	IssueID    int64
	ReviewerID int64
}

func (opts *FindReviewOptions) toCond() builder.Cond {
	var cond = builder.NewCond()
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"issue_id": opts.IssueID})
	}
	if opts.ReviewerID > 0 {
		cond = cond.And(builder.Eq{"reviewer_id": opts.ReviewerID})
	}
	if opts.Type != ReviewTypeUnknown {
		cond = cond.And(builder.Eq{"type": opts.Type})
	}
	return cond
}

func findReviews(e Engine, opts FindReviewOptions) ([]*Review, error) {
	reviews := make([]*Review, 0, 10)
	sess := e.Where(opts.toCond())
	return reviews, sess.
		Asc("created_unix").
		Asc("id").
		Find(&reviews)
}

// FindReviews returns reviews passing FindReviewOptions
func FindReviews(opts FindReviewOptions) ([]*Review, error) {
	return findReviews(x, opts)
}

// CreateReviewOptions represent the options to create a review. Type, Issue and Reviewer are required.
type CreateReviewOptions struct {
	Content  string
	Type     ReviewType
	Issue    *Issue
	Reviewer *User
}

func createReview(e Engine, opts CreateReviewOptions) (*Review, error) {
	review := &Review{
		Type:       opts.Type,
		Issue:      opts.Issue,
		IssueID:    opts.Issue.ID,
		Reviewer:   opts.Reviewer,
		ReviewerID: opts.Reviewer.ID,
		Content:    opts.Content,
	}
	if _, err := e.Insert(review); err != nil {
		return nil, err
	}

	return review, nil
}

// CreateReview creates a new review based on opts
func CreateReview(opts CreateReviewOptions) (*Review, error) {
	return createReview(x, opts)
}

func getCurrentReview(e Engine, reviewer *User, issue *Issue) (*Review, error) {
	if reviewer == nil {
		return nil, nil
	}
	reviews, err := findReviews(e, FindReviewOptions{
		Type:       ReviewTypePending,
		IssueID:    issue.ID,
		ReviewerID: reviewer.ID,
	})
	if err != nil {
		return nil, err
	}
	if len(reviews) == 0 {
		return nil, ErrReviewNotExist{}
	}
	reviews[0].Reviewer = reviewer
	reviews[0].Issue = issue
	return reviews[0], nil
}

// ReviewExists returns whether a review exists for a particular line of code in the PR
func ReviewExists(issue *Issue, treePath string, line int64) (bool, error) {
	return x.Cols("id").Exist(&Comment{IssueID: issue.ID, TreePath: treePath, Line: line, Type: CommentTypeCode})
}

// GetCurrentReview returns the current pending review of reviewer for given issue
func GetCurrentReview(reviewer *User, issue *Issue) (*Review, error) {
	return getCurrentReview(x, reviewer, issue)
}

// ContentEmptyErr represents an content empty error
type ContentEmptyErr struct {
}

func (ContentEmptyErr) Error() string {
	return "Review content is empty"
}

// IsContentEmptyErr returns true if err is a ContentEmptyErr
func IsContentEmptyErr(err error) bool {
	_, ok := err.(ContentEmptyErr)
	return ok
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(doer *User, issue *Issue, reviewType ReviewType, content string) (*Review, *Comment, error) {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, nil, err
	}

	review, err := getCurrentReview(sess, doer, issue)
	if err != nil {
		if !IsErrReviewNotExist(err) {
			return nil, nil, err
		}

		if reviewType != ReviewTypeApprove && len(strings.TrimSpace(content)) == 0 {
			return nil, nil, ContentEmptyErr{}
		}

		// No current review. Create a new one!
		review, err = createReview(sess, CreateReviewOptions{
			Type:     reviewType,
			Issue:    issue,
			Reviewer: doer,
			Content:  content,
		})
		if err != nil {
			return nil, nil, err
		}
	} else {
		if err := review.loadCodeComments(sess); err != nil {
			return nil, nil, err
		}
		if reviewType != ReviewTypeApprove && len(review.CodeComments) == 0 && len(strings.TrimSpace(content)) == 0 {
			return nil, nil, ContentEmptyErr{}
		}

		review.Issue = issue
		review.Content = content
		review.Type = reviewType
		if _, err := sess.ID(review.ID).Cols("content, type").Update(review); err != nil {
			return nil, nil, err
		}
	}

	comm, err := createCommentWithNoAction(sess, &CreateCommentOptions{
		Type:     CommentTypeReview,
		Doer:     doer,
		Content:  review.Content,
		Issue:    issue,
		Repo:     issue.Repo,
		ReviewID: review.ID,
	})
	if err != nil || comm == nil {
		return nil, nil, err
	}

	comm.Review = review
	return review, comm, sess.Commit()
}

// PullReviewersWithType represents the type used to display a review overview
type PullReviewersWithType struct {
	User              `xorm:"extends"`
	Type              ReviewType
	ReviewUpdatedUnix timeutil.TimeStamp `xorm:"review_updated_unix"`
}

// GetReviewersByPullID gets all reviewers for a pull request with the statuses
func GetReviewersByPullID(pullID int64) (issueReviewers []*PullReviewersWithType, err error) {
	irs := []*PullReviewersWithType{}
	if x.Dialect().DBType() == core.MSSQL {
		err = x.SQL(`SELECT [user].*, review.type, review.review_updated_unix FROM
(SELECT review.id, review.type, review.reviewer_id, max(review.updated_unix) as review_updated_unix
FROM review WHERE review.issue_id=? AND (review.type = ? OR review.type = ?)
GROUP BY review.id, review.type, review.reviewer_id) as review
INNER JOIN [user] ON review.reviewer_id = [user].id ORDER BY review_updated_unix DESC`,
			pullID, ReviewTypeApprove, ReviewTypeReject).
			Find(&irs)
	} else {
		err = x.Select("`user`.*, review.type, max(review.updated_unix) as review_updated_unix").
			Table("review").
			Join("INNER", "`user`", "review.reviewer_id = `user`.id").
			Where("review.issue_id = ? AND (review.type = ? OR review.type = ?)",
				pullID, ReviewTypeApprove, ReviewTypeReject).
			GroupBy("`user`.id, review.type").
			OrderBy("review_updated_unix DESC").
			Find(&irs)
	}

	// We need to group our results by user id _and_ review type, otherwise the query fails when using postgresql.
	// But becaus we're doing this, we need to manually filter out multiple reviews of different types by the
	// same person because we only want to show the newest review grouped by user. Thats why we're using a map here.
	issueReviewers = []*PullReviewersWithType{}
	usersInArray := make(map[int64]bool)
	for _, ir := range irs {
		if !usersInArray[ir.ID] {
			issueReviewers = append(issueReviewers, ir)
			usersInArray[ir.ID] = true
		}
	}

	return
}
