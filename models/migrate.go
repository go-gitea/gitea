// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "github.com/go-xorm/xorm"

// InsertIssue insert one issue to database
func InsertIssue(issue *Issue, labelIDs []int64) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := insertIssue(sess, issue, labelIDs); err != nil {
		return err
	}
	return sess.Commit()
}

func insertIssue(sess *xorm.Session, issue *Issue, labelIDs []int64) error {
	if _, err := sess.Insert(issue); err != nil {
		return err
	}
	var issueLabels = make([]IssueLabel, 0, len(labelIDs))
	for _, labelID := range labelIDs {
		issueLabels = append(issueLabels, IssueLabel{
			IssueID: issue.ID,
			LabelID: labelID,
		})
	}
	if _, err := sess.Insert(issueLabels); err != nil {
		return err
	}

	return nil
}

// InsertComment inserted a comment
func InsertComment(comment *Comment) error {
	_, err := x.Insert(comment)
	return err
}

// InsertPullRequest inserted a pull request
func InsertPullRequest(pr *PullRequest, labelIDs []int64) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := insertIssue(sess, pr.Issue, labelIDs); err != nil {
		return err
	}
	pr.IssueID = pr.Issue.ID
	if _, err := sess.Insert(pr); err != nil {
		return err
	}
	return sess.Commit()
}
