// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	admin_model "code.gitea.io/gitea/models/admin"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// CountOrphanedLabels return count of labels witch are broken and not accessible via ui anymore
func CountOrphanedLabels() (int64, error) {
	noref, err := db.GetEngine(db.DefaultContext).Table("label").Where("repo_id=? AND org_id=?", 0, 0).Count("label.id")
	if err != nil {
		return 0, err
	}

	norepo, err := db.GetEngine(db.DefaultContext).Table("label").
		Where(builder.And(
			builder.Gt{"repo_id": 0},
			builder.NotIn("repo_id", builder.Select("id").From("repository")),
		)).
		Count()
	if err != nil {
		return 0, err
	}

	noorg, err := db.GetEngine(db.DefaultContext).Table("label").
		Where(builder.And(
			builder.Gt{"org_id": 0},
			builder.NotIn("org_id", builder.Select("id").From("user")),
		)).
		Count()
	if err != nil {
		return 0, err
	}

	return noref + norepo + noorg, nil
}

// DeleteOrphanedLabels delete labels witch are broken and not accessible via ui anymore
func DeleteOrphanedLabels() error {
	// delete labels with no reference
	if _, err := db.GetEngine(db.DefaultContext).Table("label").Where("repo_id=? AND org_id=?", 0, 0).Delete(new(Label)); err != nil {
		return err
	}

	// delete labels with none existing repos
	if _, err := db.GetEngine(db.DefaultContext).
		Where(builder.And(
			builder.Gt{"repo_id": 0},
			builder.NotIn("repo_id", builder.Select("id").From("repository")),
		)).
		Delete(Label{}); err != nil {
		return err
	}

	// delete labels with none existing orgs
	if _, err := db.GetEngine(db.DefaultContext).
		Where(builder.And(
			builder.Gt{"org_id": 0},
			builder.NotIn("org_id", builder.Select("id").From("user")),
		)).
		Delete(Label{}); err != nil {
		return err
	}

	return nil
}

// CountOrphanedIssueLabels return count of IssueLabels witch have no label behind anymore
func CountOrphanedIssueLabels() (int64, error) {
	return db.GetEngine(db.DefaultContext).Table("issue_label").
		NotIn("label_id", builder.Select("id").From("label")).
		Count()
}

// DeleteOrphanedIssueLabels delete IssueLabels witch have no label behind anymore
func DeleteOrphanedIssueLabels() error {
	_, err := db.GetEngine(db.DefaultContext).
		NotIn("label_id", builder.Select("id").From("label")).
		Delete(IssueLabel{})

	return err
}

// CountOrphanedIssues count issues without a repo
func CountOrphanedIssues() (int64, error) {
	return db.GetEngine(db.DefaultContext).Table("issue").
		Join("LEFT", "repository", "issue.repo_id=repository.id").
		Where(builder.IsNull{"repository.id"}).
		Count("id")
}

// DeleteOrphanedIssues delete issues without a repo
func DeleteOrphanedIssues() error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	var ids []int64

	if err := db.GetEngine(ctx).Table("issue").Distinct("issue.repo_id").
		Join("LEFT", "repository", "issue.repo_id=repository.id").
		Where(builder.IsNull{"repository.id"}).GroupBy("issue.repo_id").
		Find(&ids); err != nil {
		return err
	}

	var attachmentPaths []string
	for i := range ids {
		paths, err := deleteIssuesByRepoID(ctx, ids[i])
		if err != nil {
			return err
		}
		attachmentPaths = append(attachmentPaths, paths...)
	}

	if err := committer.Commit(); err != nil {
		return err
	}
	committer.Close()

	// Remove issue attachment files.
	for i := range attachmentPaths {
		admin_model.RemoveAllWithNotice(db.DefaultContext, "Delete issue attachment", attachmentPaths[i])
	}
	return nil
}

// CountOrphanedObjects count subjects with have no existing refobject anymore
func CountOrphanedObjects(subject, refobject, joinCond string) (int64, error) {
	return db.GetEngine(db.DefaultContext).Table("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"}).
		Count("id")
}

// DeleteOrphanedObjects delete subjects with have no existing refobject anymore
func DeleteOrphanedObjects(subject, refobject, joinCond string) error {
	subQuery := builder.Select("`"+subject+"`.id").
		From("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"})
	sql, args, err := builder.Delete(builder.In("id", subQuery)).From("`" + subject + "`").ToSQL()
	if err != nil {
		return err
	}
	_, err = db.GetEngine(db.DefaultContext).Exec(append([]interface{}{sql}, args...)...)
	return err
}

// CountNullArchivedRepository counts the number of repositories with is_archived is null
func CountNullArchivedRepository() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.IsNull{"is_archived"}).Count(new(repo_model.Repository))
}

// FixNullArchivedRepository sets is_archived to false where it is null
func FixNullArchivedRepository() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.IsNull{"is_archived"}).Cols("is_archived").NoAutoTime().Update(&repo_model.Repository{
		IsArchived: false,
	})
}

// CountWrongUserType count OrgUser who have wrong type
func CountWrongUserType() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Eq{"type": 0}.And(builder.Neq{"num_teams": 0})).Count(new(user_model.User))
}

// FixWrongUserType fix OrgUser who have wrong type
func FixWrongUserType() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Eq{"type": 0}.And(builder.Neq{"num_teams": 0})).Cols("type").NoAutoTime().Update(&user_model.User{Type: 1})
}

// CountCommentTypeLabelWithEmptyLabel count label comments with empty label
func CountCommentTypeLabelWithEmptyLabel() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Eq{"type": CommentTypeLabel, "label_id": 0}).Count(new(Comment))
}

// FixCommentTypeLabelWithEmptyLabel count label comments with empty label
func FixCommentTypeLabelWithEmptyLabel() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Eq{"type": CommentTypeLabel, "label_id": 0}).Delete(new(Comment))
}

// CountCommentTypeLabelWithOutsideLabels count label comments with outside label
func CountCommentTypeLabelWithOutsideLabels() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("comment.type = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id))", CommentTypeLabel).
		Table("comment").
		Join("inner", "label", "label.id = comment.label_id").
		Join("inner", "issue", "issue.id = comment.issue_id ").
		Join("inner", "repository", "issue.repo_id = repository.id").
		Count(new(Comment))
}

// FixCommentTypeLabelWithOutsideLabels count label comments with outside label
func FixCommentTypeLabelWithOutsideLabels() (int64, error) {
	res, err := db.GetEngine(db.DefaultContext).Exec(`DELETE FROM comment WHERE comment.id IN (
		SELECT il_too.id FROM (
			SELECT com.id
				FROM comment AS com
					INNER JOIN label ON com.label_id = label.id
					INNER JOIN issue on issue.id = com.issue_id
					INNER JOIN repository ON issue.repo_id = repository.id
				WHERE
					com.type = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id))
	) AS il_too)`, CommentTypeLabel)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// CountIssueLabelWithOutsideLabels count label comments with outside label
func CountIssueLabelWithOutsideLabels() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Expr("(label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id)")).
		Table("issue_label").
		Join("inner", "label", "issue_label.label_id = label.id ").
		Join("inner", "issue", "issue.id = issue_label.issue_id ").
		Join("inner", "repository", "issue.repo_id = repository.id").
		Count(new(IssueLabel))
}

// FixIssueLabelWithOutsideLabels fix label comments with outside label
func FixIssueLabelWithOutsideLabels() (int64, error) {
	res, err := db.GetEngine(db.DefaultContext).Exec(`DELETE FROM issue_label WHERE issue_label.id IN (
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

// CountActionCreatedUnixString count actions where created_unix is an empty string
func CountActionCreatedUnixString() (int64, error) {
	if setting.Database.UseSQLite3 {
		return db.GetEngine(db.DefaultContext).Where(`created_unix = ""`).Count(new(Action))
	}
	return 0, nil
}

// FixActionCreatedUnixString set created_unix to zero if it is an empty string
func FixActionCreatedUnixString() (int64, error) {
	if setting.Database.UseSQLite3 {
		res, err := db.GetEngine(db.DefaultContext).Exec(`UPDATE action SET created_unix = 0 WHERE created_unix = ""`)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	return 0, nil
}
