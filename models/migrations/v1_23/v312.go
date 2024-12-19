// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"xorm.io/xorm"
)

func AddActionsConcurrency(x *xorm.Engine) error {
	type ActionRun struct {
		ConcurrencyGroup  string
		ConcurrencyCancel bool
	}

	if err := x.Sync(new(ActionRun)); err != nil {
		return err
	}

	type ActionRunJob struct {
		RawConcurrencyGroup  string
		RawConcurrencyCancel string
		ConcurrencyGroup     string
		ConcurrencyCancel    bool
	}

	return x.Sync(new(ActionRunJob))
}
