// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25 //nolint

import (
	"xorm.io/xorm"
)

func AddActionsConcurrency(x *xorm.Engine) error {
	type ActionRun struct {
		ConcurrencyGroup  string `xorm:"index"`
		ConcurrencyCancel bool
	}

	if err := x.Sync(new(ActionRun)); err != nil {
		return err
	}

	type ActionRunJob struct {
		RawConcurrencyGroup    string
		RawConcurrencyCancel   string
		IsConcurrencyEvaluated bool
		ConcurrencyGroup       string `xorm:"index"`
		ConcurrencyCancel      bool
	}

	return x.Sync(new(ActionRunJob))
}
