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
