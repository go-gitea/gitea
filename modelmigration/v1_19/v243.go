// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import "gitea.dev/modelmigration/base"

func AddExclusiveLabel(x base.EngineMigration) error {
	type Label struct {
		Exclusive bool
	}

	return x.Sync(new(Label))
}
