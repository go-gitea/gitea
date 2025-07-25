// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"xorm.io/xorm"
)

func AddActionsConcurrency(x *xorm.Engine) error {
	type ActionRun struct {
		RepoID            int64  `xorm:"index unique(repo_index) index(repo_concurrency)"`
		ConcurrencyGroup  string `xorm:"index(repo_concurrency)"`
		ConcurrencyCancel bool
	}

	if err := x.Sync(new(ActionRun)); err != nil {
		return err
	}

	type ActionRunJob struct {
		RepoID                 int64 `xorm:"index index(repo_concurrency)"`
		RawConcurrencyGroup    string
		RawConcurrencyCancel   string
		IsConcurrencyEvaluated bool
		ConcurrencyGroup       string `xorm:"index(repo_concurrency)"`
		ConcurrencyCancel      bool
	}

	return x.Sync(new(ActionRunJob))
}
