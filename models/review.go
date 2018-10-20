// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"github.com/go-xorm/xorm"

	"github.com/go-xorm/builder"
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
