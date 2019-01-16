// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	api "code.gitea.io/sdk/gitea"

	"github.com/go-xorm/builder"
	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
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

	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`

	// CodeComments are the initial code comments of the review
	CodeComments CodeComments `xorm:"-"`
}

func (r *Review) loadCodeComments(e Engine) (err error) {
	r.CodeComments, err = fetchCodeCommentsByReview(e, r.Issue, nil, r)
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

// Publish will send notifications / actions to participants for all code comments; parts are concurrent
func (r *Review) Publish() error {
	return r.publish(x)
}

func (r *Review) publish(e *xorm.Engine) error {
	if r.Type == ReviewTypePending || r.Type == ReviewTypeUnknown {
		return fmt.Errorf("review cannot be published if type is pending or unknown")
	}
	if r.Issue == nil {
		if err := r.loadIssue(e); err != nil {
			return err
		}
	}
	if err := r.Issue.loadRepo(e); err != nil {
		return err
	}
	if len(r.CodeComments) == 0 {
		if err := r.loadCodeComments(e); err != nil {
			return err
		}
	}
	for _, lines := range r.CodeComments {
		for _, comments := range lines {
			for _, comment := range comments {
				go func(en *xorm.Engine, review *Review, comm *Comment) {
					sess := en.NewSession()
					defer sess.Close()
					if err := sendCreateCommentAction(sess, &CreateCommentOptions{
						Doer:    comm.Poster,
						Issue:   review.Issue,
						Repo:    review.Issue.Repo,
						Type:    comm.Type,
						Content: comm.Content,
					}, comm); err != nil {
						log.Warn("sendCreateCommentAction: %v", err)
					}
				}(e, r, comment)
			}
		}
	}
	return nil
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

	var reviewHookType HookEventType

	switch opts.Type {
	case ReviewTypeApprove:
		reviewHookType = HookEventPullRequestApproved
	case ReviewTypeComment:
		reviewHookType = HookEventPullRequestComment
	case ReviewTypeReject:
		reviewHookType = HookEventPullRequestRejected
	default:
		// unsupported review webhook type here
		return review, nil
	}

	pr := opts.Issue.PullRequest

	if err := pr.LoadIssue(); err != nil {
		return nil, err
	}

	mode, err := AccessLevel(opts.Issue.Poster, opts.Issue.Repo)
	if err != nil {
		return nil, err
	}

	if err := PrepareWebhooks(opts.Issue.Repo, reviewHookType, &api.PullRequestPayload{
		Action:      api.HookIssueSynchronized,
		Index:       opts.Issue.Index,
		PullRequest: pr.APIFormat(),
		Repository:  opts.Issue.Repo.APIFormat(mode),
		Sender:      opts.Reviewer.APIFormat(),
	}); err != nil {
		return nil, err
	}
	go HookQueue.Add(opts.Issue.Repo.ID)

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

// GetCurrentReview returns the current pending review of reviewer for given issue
func GetCurrentReview(reviewer *User, issue *Issue) (*Review, error) {
	return getCurrentReview(x, reviewer, issue)
}

// UpdateReview will update all cols of the given review in db
func UpdateReview(r *Review) error {
	if _, err := x.ID(r.ID).AllCols().Update(r); err != nil {
		return err
	}
	return nil
}

// PullReviewersWithType represents the type used to display a review overview
type PullReviewersWithType struct {
	User              `xorm:"extends"`
	Type              ReviewType
	ReviewUpdatedUnix util.TimeStamp `xorm:"review_updated_unix"`
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
