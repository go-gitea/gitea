// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"sort"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// IssueLabel represents an issue-label relation.
type IssueLabel struct {
	ID      int64 `xorm:"pk autoincr"`
	IssueID int64 `xorm:"UNIQUE(s)"`
	LabelID int64 `xorm:"UNIQUE(s)"`
}

// HasIssueLabel returns true if issue has been labeled.
func HasIssueLabel(ctx context.Context, issueID, labelID int64) bool {
	has, _ := db.GetEngine(ctx).Where("issue_id = ? AND label_id = ?", issueID, labelID).Get(new(IssueLabel))
	return has
}

// newIssueLabel this function creates a new label it does not check if the label is valid for the issue
// YOU MUST CHECK THIS BEFORE THIS FUNCTION
func newIssueLabel(ctx context.Context, issue *Issue, label *Label, doer *user_model.User) (err error) {
	if err = db.Insert(ctx, &IssueLabel{
		IssueID: issue.ID,
		LabelID: label.ID,
	}); err != nil {
		return err
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}

	opts := &CreateCommentOptions{
		Type:    CommentTypeLabel,
		Doer:    doer,
		Repo:    issue.Repo,
		Issue:   issue,
		Label:   label,
		Content: "1",
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return err
	}

	issue.Labels = append(issue.Labels, label)

	return updateLabelCols(ctx, label, "num_issues", "num_closed_issue")
}

// Remove all issue labels in the given exclusive scope
func RemoveDuplicateExclusiveIssueLabels(ctx context.Context, issue *Issue, label *Label, doer *user_model.User) (err error) {
	scope := label.ExclusiveScope()
	if scope == "" {
		return nil
	}

	var toRemove []*Label
	for _, issueLabel := range issue.Labels {
		if label.ID != issueLabel.ID && issueLabel.ExclusiveScope() == scope {
			toRemove = append(toRemove, issueLabel)
		}
	}

	for _, issueLabel := range toRemove {
		if err = deleteIssueLabel(ctx, issue, issueLabel, doer); err != nil {
			return err
		}
	}

	return nil
}

// NewIssueLabel creates a new issue-label relation.
func NewIssueLabel(ctx context.Context, issue *Issue, label *Label, doer *user_model.User) (err error) {
	if HasIssueLabel(ctx, issue.ID, label.ID) {
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}

	// Do NOT add invalid labels
	if issue.RepoID != label.RepoID && issue.Repo.OwnerID != label.OrgID {
		return nil
	}

	if err = RemoveDuplicateExclusiveIssueLabels(ctx, issue, label, doer); err != nil {
		return nil
	}

	if err = newIssueLabel(ctx, issue, label, doer); err != nil {
		return err
	}

	issue.isLabelsLoaded = false
	issue.Labels = nil
	if err = issue.LoadLabels(ctx); err != nil {
		return err
	}

	return committer.Commit()
}

// newIssueLabels add labels to an issue. It will check if the labels are valid for the issue
func newIssueLabels(ctx context.Context, issue *Issue, labels []*Label, doer *user_model.User) (err error) {
	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}

	if err = issue.LoadLabels(ctx); err != nil {
		return err
	}

	for _, l := range labels {
		// Don't add already present labels and invalid labels
		if HasIssueLabel(ctx, issue.ID, l.ID) ||
			(l.RepoID != issue.RepoID && l.OrgID != issue.Repo.OwnerID) {
			continue
		}

		if err = RemoveDuplicateExclusiveIssueLabels(ctx, issue, l, doer); err != nil {
			return err
		}

		if err = newIssueLabel(ctx, issue, l, doer); err != nil {
			return fmt.Errorf("newIssueLabel: %w", err)
		}
	}

	return nil
}

// NewIssueLabels creates a list of issue-label relations.
func NewIssueLabels(ctx context.Context, issue *Issue, labels []*Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = newIssueLabels(ctx, issue, labels, doer); err != nil {
		return err
	}

	// reload all labels
	issue.isLabelsLoaded = false
	issue.Labels = nil
	if err = issue.LoadLabels(ctx); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteIssueLabel(ctx context.Context, issue *Issue, label *Label, doer *user_model.User) (err error) {
	if count, err := db.DeleteByBean(ctx, &IssueLabel{
		IssueID: issue.ID,
		LabelID: label.ID,
	}); err != nil {
		return err
	} else if count == 0 {
		return nil
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}

	opts := &CreateCommentOptions{
		Type:  CommentTypeLabel,
		Doer:  doer,
		Repo:  issue.Repo,
		Issue: issue,
		Label: label,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return err
	}

	return updateLabelCols(ctx, label, "num_issues", "num_closed_issue")
}

// DeleteIssueLabel deletes issue-label relation.
func DeleteIssueLabel(ctx context.Context, issue *Issue, label *Label, doer *user_model.User) error {
	if err := deleteIssueLabel(ctx, issue, label, doer); err != nil {
		return err
	}

	issue.Labels = nil
	return issue.LoadLabels(ctx)
}

// DeleteLabelsByRepoID  deletes labels of some repository
func DeleteLabelsByRepoID(ctx context.Context, repoID int64) error {
	deleteCond := builder.Select("id").From("label").Where(builder.Eq{"label.repo_id": repoID})

	if _, err := db.GetEngine(ctx).In("label_id", deleteCond).
		Delete(&IssueLabel{}); err != nil {
		return err
	}

	_, err := db.DeleteByBean(ctx, &Label{RepoID: repoID})
	return err
}

// CountOrphanedLabels return count of labels witch are broken and not accessible via ui anymore
func CountOrphanedLabels(ctx context.Context) (int64, error) {
	noref, err := db.GetEngine(ctx).Table("label").Where("repo_id=? AND org_id=?", 0, 0).Count()
	if err != nil {
		return 0, err
	}

	norepo, err := db.GetEngine(ctx).Table("label").
		Where(builder.And(
			builder.Gt{"repo_id": 0},
			builder.NotIn("repo_id", builder.Select("id").From("`repository`")),
		)).
		Count()
	if err != nil {
		return 0, err
	}

	noorg, err := db.GetEngine(ctx).Table("label").
		Where(builder.And(
			builder.Gt{"org_id": 0},
			builder.NotIn("org_id", builder.Select("id").From("`user`")),
		)).
		Count()
	if err != nil {
		return 0, err
	}

	return noref + norepo + noorg, nil
}

// DeleteOrphanedLabels delete labels witch are broken and not accessible via ui anymore
func DeleteOrphanedLabels(ctx context.Context) error {
	// delete labels with no reference
	if _, err := db.GetEngine(ctx).Table("label").Where("repo_id=? AND org_id=?", 0, 0).Delete(new(Label)); err != nil {
		return err
	}

	// delete labels with none existing repos
	if _, err := db.GetEngine(ctx).
		Where(builder.And(
			builder.Gt{"repo_id": 0},
			builder.NotIn("repo_id", builder.Select("id").From("`repository`")),
		)).
		Delete(Label{}); err != nil {
		return err
	}

	// delete labels with none existing orgs
	if _, err := db.GetEngine(ctx).
		Where(builder.And(
			builder.Gt{"org_id": 0},
			builder.NotIn("org_id", builder.Select("id").From("`user`")),
		)).
		Delete(Label{}); err != nil {
		return err
	}

	return nil
}

// CountOrphanedIssueLabels return count of IssueLabels witch have no label behind anymore
func CountOrphanedIssueLabels(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Table("issue_label").
		NotIn("label_id", builder.Select("id").From("label")).
		Count()
}

// DeleteOrphanedIssueLabels delete IssueLabels witch have no label behind anymore
func DeleteOrphanedIssueLabels(ctx context.Context) error {
	_, err := db.GetEngine(ctx).
		NotIn("label_id", builder.Select("id").From("label")).
		Delete(IssueLabel{})
	return err
}

// CountIssueLabelWithOutsideLabels count label comments with outside label
func CountIssueLabelWithOutsideLabels(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Where(builder.Expr("(label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id)")).
		Table("issue_label").
		Join("inner", "label", "issue_label.label_id = label.id ").
		Join("inner", "issue", "issue.id = issue_label.issue_id ").
		Join("inner", "repository", "issue.repo_id = repository.id").
		Count(new(IssueLabel))
}

// FixIssueLabelWithOutsideLabels fix label comments with outside label
func FixIssueLabelWithOutsideLabels(ctx context.Context) (int64, error) {
	res, err := db.GetEngine(ctx).Exec(`DELETE FROM issue_label WHERE issue_label.id IN (
		SELECT il_too.id FROM (
			SELECT il_too_too.id
				FROM issue_label AS il_too_too
					INNER JOIN label ON il_too_too.label_id = label.id
					INNER JOIN issue on issue.id = il_too_too.issue_id
					INNER JOIN repository on repository.id = issue.repo_id
				WHERE
					(label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id)
	) AS il_too )`)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// LoadLabels loads labels
func (issue *Issue) LoadLabels(ctx context.Context) (err error) {
	if !issue.isLabelsLoaded && issue.Labels == nil && issue.ID != 0 {
		issue.Labels, err = GetLabelsByIssueID(ctx, issue.ID)
		if err != nil {
			return fmt.Errorf("getLabelsByIssueID [%d]: %w", issue.ID, err)
		}
		issue.isLabelsLoaded = true
	}
	return nil
}

// GetLabelsByIssueID returns all labels that belong to given issue by ID.
func GetLabelsByIssueID(ctx context.Context, issueID int64) ([]*Label, error) {
	var labels []*Label
	return labels, db.GetEngine(ctx).Where("issue_label.issue_id = ?", issueID).
		Join("LEFT", "issue_label", "issue_label.label_id = label.id").
		Asc("label.name").
		Find(&labels)
}

func clearIssueLabels(ctx context.Context, issue *Issue, doer *user_model.User) (err error) {
	if err = issue.LoadLabels(ctx); err != nil {
		return fmt.Errorf("getLabels: %w", err)
	}

	for i := range issue.Labels {
		if err = deleteIssueLabel(ctx, issue, issue.Labels[i], doer); err != nil {
			return fmt.Errorf("removeLabel: %w", err)
		}
	}

	return nil
}

// ClearIssueLabels removes all issue labels as the given user.
// Triggers appropriate WebHooks, if any.
func ClearIssueLabels(ctx context.Context, issue *Issue, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	} else if err = issue.LoadPullRequest(ctx); err != nil {
		return err
	}

	perm, err := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWriteIssuesOrPulls(issue.IsPull) {
		return ErrRepoLabelNotExist{}
	}

	if err = clearIssueLabels(ctx, issue, doer); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}

	return nil
}

type labelSorter []*Label

func (ts labelSorter) Len() int {
	return len([]*Label(ts))
}

func (ts labelSorter) Less(i, j int) bool {
	return []*Label(ts)[i].ID < []*Label(ts)[j].ID
}

func (ts labelSorter) Swap(i, j int) {
	[]*Label(ts)[i], []*Label(ts)[j] = []*Label(ts)[j], []*Label(ts)[i]
}

// Ensure only one label of a given scope exists, with labels at the end of the
// array getting preference over earlier ones.
func RemoveDuplicateExclusiveLabels(labels []*Label) []*Label {
	validLabels := make([]*Label, 0, len(labels))

	for i, label := range labels {
		scope := label.ExclusiveScope()
		if scope != "" {
			foundOther := false
			for _, otherLabel := range labels[i+1:] {
				if otherLabel.ExclusiveScope() == scope {
					foundOther = true
					break
				}
			}
			if foundOther {
				continue
			}
		}
		validLabels = append(validLabels, label)
	}

	return validLabels
}

// ReplaceIssueLabels removes all current labels and add new labels to the issue.
// Triggers appropriate WebHooks, if any.
func ReplaceIssueLabels(ctx context.Context, issue *Issue, labels []*Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}

	if err = issue.LoadLabels(ctx); err != nil {
		return err
	}

	labels = RemoveDuplicateExclusiveLabels(labels)

	sort.Sort(labelSorter(labels))
	sort.Sort(labelSorter(issue.Labels))

	var toAdd, toRemove []*Label

	addIndex, removeIndex := 0, 0
	for addIndex < len(labels) && removeIndex < len(issue.Labels) {
		addLabel := labels[addIndex]
		removeLabel := issue.Labels[removeIndex]
		if addLabel.ID == removeLabel.ID {
			// Silently drop invalid labels
			if removeLabel.RepoID != issue.RepoID && removeLabel.OrgID != issue.Repo.OwnerID {
				toRemove = append(toRemove, removeLabel)
			}

			addIndex++
			removeIndex++
		} else if addLabel.ID < removeLabel.ID {
			// Only add if the label is valid
			if addLabel.RepoID == issue.RepoID || addLabel.OrgID == issue.Repo.OwnerID {
				toAdd = append(toAdd, addLabel)
			}
			addIndex++
		} else {
			toRemove = append(toRemove, removeLabel)
			removeIndex++
		}
	}
	toAdd = append(toAdd, labels[addIndex:]...)
	toRemove = append(toRemove, issue.Labels[removeIndex:]...)

	if len(toAdd) > 0 {
		if err = newIssueLabels(ctx, issue, toAdd, doer); err != nil {
			return fmt.Errorf("addLabels: %w", err)
		}
	}

	for _, l := range toRemove {
		if err = deleteIssueLabel(ctx, issue, l, doer); err != nil {
			return fmt.Errorf("removeLabel: %w", err)
		}
	}

	issue.Labels = nil
	if err = issue.LoadLabels(ctx); err != nil {
		return err
	}

	return committer.Commit()
}
