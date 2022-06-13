// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

<<<<<<< HEAD
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
		Select("COUNT(`issue`.`id`)").
		Count()
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

=======
>>>>>>> main
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
