// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "github.com/go-xorm/xorm"

<<<<<<< HEAD
func addProjectsInfo(x *xorm.Engine) error {

	type Repository struct {
		NumProjects       int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedProjects int `xorm:"NOT NULL DEFAULT 0"`
		NumOpenProjects   int `xorm:"-"`
	}

	return x.Sync2(new(Repository))
=======
func addStatusCheckColumnsForProtectedBranches(x *xorm.Engine) error {
	type ProtectedBranch struct {
		EnableStatusCheck   bool     `xorm:"NOT NULL DEFAULT false"`
		StatusCheckContexts []string `xorm:"JSON TEXT"`
	}

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return err
	}

	_, err := x.Cols("enable_status_check", "status_check_contexts").Update(&ProtectedBranch{
		EnableStatusCheck:   false,
		StatusCheckContexts: []string{},
	})
	return err
>>>>>>> origin
}
