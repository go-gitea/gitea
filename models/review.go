// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "code.gitea.io/gitea/modules/util"

type ReviewType int

const (
	// Approving changes
	ReviewTypeApprove ReviewType = iota
	// General feedback
	ReviewTypeComment
	// Feedback blocking merge
	ReviewTypeReject
)

// Review represents collection of code comments giving feedback for a PR
type Review struct {
	ID              int64 `xorm:"pk autoincr"`
	Type            ReviewType
	Pending         bool
	Reviewer        *User    `xorm:"-"`
	ReviewerID      int64    `xorm:"index"`
	Issue           *Issue   `xorm:"-"`
	IssueID         int64    `xorm:"index"`
	ReviewCommentID int64    `xorm:"index"`
	ReviewComment   *Comment `xorm:"-"`

	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`

	// CodeComments are the initial code comments of the review
	CodeComments []*Comment `xorm:"-"`
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

func GetReviewByID(id int64) (*Review, error) {
	return getReviewByID(x, id)
}
