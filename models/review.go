// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "code.gitea.io/gitea/modules/util"

// ReviewType defines the sort of feedback a review gives
type ReviewType int

const (
	// ReviewTypeApprove approves changes
	ReviewTypeApprove ReviewType = iota
	// ReviewTypeComment gives general feedback
	ReviewTypeComment
	// ReviewTypeReject gives feedback blocking merge
	ReviewTypeReject
	// ReviewTypePending is a review which is not published yet
	ReviewTypePending
)

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
	CodeComments []*Comment `xorm:"-"`
}

func (r *Review) loadCodeComments(e Engine) (err error) {
	r.CodeComments, err = findComments(e, FindCommentsOptions{IssueID: r.IssueID, ReviewID: r.ID})
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

func getPendingReviewByReviewerID(e Engine, reviewer *User, issue *Issue) (review *Review, err error) {
	var exists bool
	if exists, err = e.Table("review").Where("reviewer_id = ? and issue_id = ? and type = ?", reviewer.ID, issue.ID, ReviewTypePending).
		Get(review); !exists && err == nil {
		return nil, nil
	}
	return
}

// GetPendingReviewByReviewer returns the latest pending review of reviewer at PR issue
func GetPendingReviewByReviewer(reviewer *User, issue *Issue) (*Review, error) {
	return getPendingReviewByReviewerID(x, reviewer, issue)
}

func createPendingReview(e Engine, reviewer *User, issue *Issue) (*Review, error) {
	review := &Review{
		Type:       ReviewTypePending,
		Issue:      issue,
		IssueID:    issue.ID,
		Reviewer:   reviewer,
		ReviewerID: reviewer.ID,
	}
	_, err := e.Insert(review)
	return review, err
}

// CreatePendingReview creates an empty pending review
func CreatePendingReview(reviewer *User, issue *Issue) (*Review, error) {
	return createPendingReview(x, reviewer, issue)
}
