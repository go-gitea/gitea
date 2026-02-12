// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"code.gitea.io/gitea/modules/optional"
)

func ParseIssueFilterStateIsClosed(state string) optional.Option[bool] {
	switch state {
	case "all":
		return optional.None[bool]()
	case "closed":
		return optional.Some(true)
	case "", "open":
		return optional.Some(false)
	default:
		return optional.Some(false) // unknown state, undefined behavior
	}
}

func ParseIssueFilterTypeIsPull(typ string) optional.Option[bool] {
	return optional.FromMapLookup(map[string]bool{"pulls": true, "issues": false}, typ)
}
