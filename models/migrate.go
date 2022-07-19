// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/structs"
)

// InsertMilestones creates milestones of repository.
func InsertMilestones(ms ...*issues_model.Milestone) (err error) {
	if len(ms) == 0 {
		return nil
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	// to return the id, so we should not use batch insert
	for _, m := range ms {
		if _, err = sess.NoAutoTime().Insert(m); err != nil {
			return err
		}
	}

	if _, err = db.Exec(ctx, "UPDATE `repository` SET num_milestones = num_milestones + ? WHERE id = ?", len(ms), ms[0].RepoID); err != nil {
		return err
	}
	return committer.Commit()
}

// InsertIssues insert issues to database
func InsertIssues(issues ...*issues_model.Issue) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, issue := range issues {
		if err := insertIssue(ctx, issue); err != nil {
			return err
		}
	}
	return committer.Commit()
}

func insertIssue(ctx context.Context, issue *issues_model.Issue) error {
	sess := db.GetEngine(ctx)
	if _, err := sess.NoAutoTime().Insert(issue); err != nil {
		return err
	}
	issueLabels := make([]issues_model.IssueLabel, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		issueLabels = append(issueLabels, issues_model.IssueLabel{
			IssueID: issue.ID,
			LabelID: label.ID,
		})
	}
	if len(issueLabels) > 0 {
		if _, err := sess.Insert(issueLabels); err != nil {
			return err
		}
	}

	for _, reaction := range issue.Reactions {
		reaction.IssueID = issue.ID
	}

	if len(issue.Reactions) > 0 {
		if _, err := sess.Insert(issue.Reactions); err != nil {
			return err
		}
	}

	if issue.ForeignReference != nil {
		issue.ForeignReference.LocalIndex = issue.Index
		if _, err := sess.Insert(issue.ForeignReference); err != nil {
			return err
		}
	}

	return nil
}

// InsertIssueComments inserts many comments of issues.
func InsertIssueComments(comments []*issues_model.Comment) error {
	if len(comments) == 0 {
		return nil
	}

	issueIDs := make(map[int64]bool)
	for _, comment := range comments {
		issueIDs[comment.IssueID] = true
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	for _, comment := range comments {
		if _, err := db.GetEngine(ctx).NoAutoTime().Insert(comment); err != nil {
			return err
		}

		for _, reaction := range comment.Reactions {
			reaction.IssueID = comment.IssueID
			reaction.CommentID = comment.ID
		}
		if len(comment.Reactions) > 0 {
			if err := db.Insert(ctx, comment.Reactions); err != nil {
				return err
			}
		}
	}

	for issueID := range issueIDs {
		if _, err := db.Exec(ctx, "UPDATE issue set num_comments = (SELECT count(*) FROM comment WHERE issue_id = ? AND `type`=?) WHERE id = ?",
			issueID, issues_model.CommentTypeComment, issueID); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// InsertPullRequests inserted pull requests
func InsertPullRequests(prs ...*issues_model.PullRequest) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)
	for _, pr := range prs {
		if err := insertIssue(ctx, pr.Issue); err != nil {
			return err
		}
		pr.IssueID = pr.Issue.ID
		if _, err := sess.NoAutoTime().Insert(pr); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// InsertReleases migrates release
func InsertReleases(rels ...*Release) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	for _, rel := range rels {
		if _, err := sess.NoAutoTime().Insert(rel); err != nil {
			return err
		}

		if len(rel.Attachments) > 0 {
			for i := range rel.Attachments {
				rel.Attachments[i].ReleaseID = rel.ID
			}

			if _, err := sess.NoAutoTime().Insert(rel.Attachments); err != nil {
				return err
			}
		}
	}

	return committer.Commit()
}

// UpdateMigrationsByType updates all migrated repositories' posterid from gitServiceType to replace originalAuthorID to posterID
func UpdateMigrationsByType(tp structs.GitServiceType, externalUserID string, userID int64) error {
	if err := issues_model.UpdateIssuesMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := issues_model.UpdateCommentsMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := UpdateReleasesMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := issues_model.UpdateReactionsMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}
	return issues_model.UpdateReviewsMigrationsByType(tp, externalUserID, userID)
}
