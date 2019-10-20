// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func removeOrganizationWatchRepo(x *xorm.Engine) error {
	// UserType defines the user type
	type UserType int

	const (
		// UserTypeIndividual defines an individual user
		UserTypeIndividual UserType = iota // Historic reason to make it starts at 0.

		// UserTypeOrganization defines an organization
		UserTypeOrganization
	)

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if _, err := sess.Exec("DELETE FROM `watch` WHERE `user_id` IN (SELECT `id` FROM `user` WHERE `type` = ?)", UserTypeOrganization); err != nil {
		return err
	}
	if _, err := sess.Exec("UPDATE `repository` SET num_watches = (SELECT count(*) FROM watch WHERE `repository`.`id` = watch.repo_id)"); err != nil {
		return err
	}

	return sess.Commit()
}
