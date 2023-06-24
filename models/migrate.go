// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/structs"
)

// InsertMilestones creates milestones of repository.
func InsertMilestones(ms ...*issues_model.Milestone) (err error) {
	if len(ms) == 0 {
		return nil
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
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

	if _, err = sess.Exec("UPDATE `repository` SET num_milestones = num_milestones + ? WHERE id = ?", len(ms), ms[0].RepoID); err != nil {
		return err
	}
	return committer.Commit()
}

// UpdateMilestones updates milestones of repository.
func UpdateMilestones(ms ...*issues_model.Milestone) (err error) {
	if len(ms) == 0 {
		return nil
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	// get existing milestones
	existingMilestones := make([]*issues_model.Milestone, 0)
	if err = sess.Where("repo_id = ?", ms[0].RepoID).Find(&existingMilestones); err != nil {
		return err
	}

	milestonesToAdd := make([]*issues_model.Milestone, 0)
	milestonesToUpdate := make([]*issues_model.Milestone, 0)
	milestonesToDelete := make([]*issues_model.Milestone, 0)
	foundMap := make(map[int64]bool)

	openCount := 0
	closedCount := 0

	for _, m := range ms {
		var foundMilestone *issues_model.Milestone
		for _, existingMilestone := range existingMilestones {
			if existingMilestone.OriginalID == m.OriginalID {
				foundMilestone = existingMilestone
				foundMap[existingMilestone.ID] = true
				break
			}
		}

		if foundMilestone == nil {
			milestonesToAdd = append(milestonesToAdd, m)
		} else if foundMilestone.OriginalID != m.OriginalID {
			m.ID = foundMilestone.ID
			milestonesToUpdate = append(milestonesToUpdate, m)
		}

		if m.IsClosed {
			closedCount++
		} else {
			openCount++
		}
	}

	for _, existingMilestone := range existingMilestones {
		if _, exist := foundMap[existingMilestone.ID]; !exist {
			milestonesToDelete = append(milestonesToDelete, existingMilestone)
		}
	}

	if len(milestonesToAdd) > 0 {
		if _, err = sess.Insert(milestonesToAdd); err != nil {
			return err
		}
	}

	for _, m := range milestonesToUpdate {
		if _, err = sess.ID(m.ID).AllCols().Update(m); err != nil {
			return err
		}
	}

	for _, m := range milestonesToDelete {
		if _, err = sess.ID(m.ID).Delete(m); err != nil {
			return err
		}
	}

	// TODO: is this correct?
	if _, err = sess.ID(ms[0].RepoID).Update(&repo_model.Repository{
		NumMilestones:       len(ms),
		NumOpenMilestones:   openCount,
		NumClosedMilestones: closedCount,
	}); err != nil {
		return err
	}

	return committer.Commit()
}

// InsertIssues insert issues to database
func InsertIssues(issues ...*issues_model.Issue) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
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

func resolveIssueLabels(issueID int64, labels []*issues_model.Label) []issues_model.IssueLabel {
	issueLabels := make([]issues_model.IssueLabel, 0, len(labels))
	for _, label := range labels {
		issueLabels = append(issueLabels, issues_model.IssueLabel{
			IssueID: issueID,
			LabelID: label.ID,
		})
	}
	return issueLabels
}

func insertIssue(ctx context.Context, issue *issues_model.Issue) error {
	sess := db.GetEngine(ctx)
	if _, err := sess.NoAutoTime().Insert(issue); err != nil {
		return err
	}
	issueLabels := resolveIssueLabels(issue.ID, issue.Labels)
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

	return nil
}

// UpsertIssues creates new issues and updates existing issues in database
func UpsertIssues(issues ...*issues_model.Issue) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, issue := range issues {
		if _, err := upsertIssue(ctx, issue); err != nil {
			return err
		}
	}
	return committer.Commit()
}

func updateIssue(ctx context.Context, issue *issues_model.Issue) error {
	sess := db.GetEngine(ctx)
	if _, err := sess.NoAutoTime().ID(issue.ID).Update(issue); err != nil {
		return err
	}
	issueLabels := resolveIssueLabels(issue.ID, issue.Labels)
	if len(issueLabels) > 0 {
		// delete old labels
		if _, err := sess.Table("issue_label").Where("issue_id = ?", issue.ID).Delete(); err != nil {
			return err
		}
		// insert new labels
		if _, err := sess.Insert(issueLabels); err != nil {
			return err
		}
	}

	for _, reaction := range issue.Reactions {
		reaction.IssueID = issue.ID
	}

	if len(issue.Reactions) > 0 {
		// update existing reactions and insert new ones
		for _, reaction := range issue.Reactions {
			exists, err := sess.Exist(&issues_model.Reaction{ID: reaction.ID})
			if err != nil {
				return err
			}
			if exists {
				if _, err := sess.ID(reaction.ID).Update(&reaction); err != nil {
					return err
				}
			} else {
				if _, err := sess.Insert(&reaction); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func upsertIssue(ctx context.Context, issue *issues_model.Issue) (isInsert bool, err error) {
	sess := db.GetEngine(ctx)

	var issueIDs []int64
	err = sess.Table("issue").Where("repo_id = ? AND `index` = ?", issue.RepoID, issue.Index).Cols("id").Find(&issueIDs)
	if err != nil {
		return false, err
	}

	if len(issueIDs) == 0 {
		return true, insertIssue(ctx, issue)
	}

	issue.ID = issueIDs[0]
	return false, updateIssue(ctx, issue)
}

// InsertIssueComments inserts many comments of issues.
func InsertIssueComments(comments []*issues_model.Comment) error {
	if len(comments) == 0 {
		return nil
	}

	issueIDs := make(container.Set[int64])
	for _, comment := range comments {
		issueIDs.Add(comment.IssueID)
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
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

// UpsertIssueComments inserts many comments of issues.
func UpsertIssueComments(comments []*issues_model.Comment) error {
	if len(comments) == 0 {
		return nil
	}

	issueIDs := make(map[int64]bool)
	for _, comment := range comments {
		issueIDs[comment.IssueID] = true
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)
	for _, comment := range comments {
		exists, err := sess.Exist(&issues_model.Comment{
			IssueID:     comment.IssueID,
			CreatedUnix: comment.CreatedUnix,
		})
		if err != nil {
			return err
		}
		if !exists {
			if _, err := sess.NoAutoTime().Insert(comment); err != nil {
				return err
			}
		} else {
			if _, err := sess.NoAutoTime().Where(
				"issue_id = ? AND created_unix = ?", comment.IssueID, comment.CreatedUnix,
			).Update(comment); err != nil {
				return err
			}
		}

		for _, reaction := range comment.Reactions {
			reaction.IssueID = comment.IssueID
			reaction.CommentID = comment.ID
		}
		if len(comment.Reactions) > 0 {
			for _, reaction := range comment.Reactions {
				// issue is uniquely identified by issue_id, comment_id and type
				exists, err := sess.Exist(&issues_model.Reaction{
					IssueID:   reaction.IssueID,
					CommentID: reaction.CommentID,
					Type:      reaction.Type,
				})
				if err != nil {
					return err
				}
				if exists {
					if _, err := sess.Where(
						"issue_id = ? AND comment_id = ? AND type = ?",
						reaction.IssueID, reaction.CommentID, reaction.Type,
					).Update(&reaction); err != nil {
						return err
					}
				} else {
					if _, err := sess.Insert(&reaction); err != nil {
						return err
					}
				}
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
	ctx, committer, err := db.TxContext(db.DefaultContext)
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

// UpsertPullRequests inserts new pull requests and updates existing pull requests in database
func UpsertPullRequests(prs ...*issues_model.PullRequest) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)
	for _, pr := range prs {
		isInsert, err := upsertIssue(ctx, pr.Issue)
		if err != nil {
			return err
		}
		pr.IssueID = pr.Issue.ID

		if isInsert {
			if _, err := sess.NoAutoTime().Insert(pr); err != nil {
				return err
			}
		} else {
			if _, err := sess.NoAutoTime().ID(pr.ID).Update(pr); err != nil {
				return err
			}
		}
	}
	return committer.Commit()
}

// InsertReleases migrates release
func InsertReleases(rels ...*repo_model.Release) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
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

// UpsertReleases inserts new releases and updates existing releases
func UpsertReleases(rels ...*repo_model.Release) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	for _, rel := range rels {
		exists, err := sess.Where("repo_id = ? AND tag_name = ?", rel.RepoID, rel.TagName).Exist(&repo_model.Release{})
		if err != nil {
			return err
		}

		if !exists {
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
		} else {
			if _, err := sess.NoAutoTime().Where("repo_id = ? AND tag_name = ?", rel.RepoID, rel.TagName).Update(rel); err != nil {
				return err
			}

			if len(rel.Attachments) > 0 {
				for i := range rel.Attachments {
					rel.Attachments[i].ReleaseID = rel.ID
				}

				var existingReleases []*repo_model.Attachment
				err := sess.Where("release_id = ?", rel.ID).Find(&existingReleases)
				if err != nil {
					return err
				}

				if _, err := sess.NoAutoTime().Insert(rel.Attachments); err != nil {
					return err
				}

				var ids []int64
				for _, existingRelease := range existingReleases {
					// TODO: file operations are not atomic, so errors should be handled
					err = storage.Attachments.Delete(existingRelease.RelativePath())
					if err != nil {
						return err
					}

					ids = append(ids, existingRelease.ID)
				}
				if _, err := sess.NoAutoTime().In("id", ids).Delete(&repo_model.Attachment{}); err != nil {
					return err
				}
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

	if err := repo_model.UpdateReleasesMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := issues_model.UpdateReactionsMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}
	return issues_model.UpdateReviewsMigrationsByType(tp, externalUserID, userID)
}
