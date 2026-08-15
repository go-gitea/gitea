// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
)

// NormalizeLegacyTeamAuthorize sets leftover read/write authorize values to none.
// https://github.com/go-gitea/gitea/pull/34128 made non-admin teams use team_unit (authorize=none).
// authorize>=write now means blanket access on every unit; migrating legacy read/write
// to none preserves their existing team_unit-scoped access.
func NormalizeLegacyTeamAuthorize(_ context.Context, x base.EngineMigration) error {
	// AccessModeNone=0, AccessModeRead=1, AccessModeWrite=2, AccessModeAdmin=3
	_, err := x.Exec("UPDATE team SET authorize = 0 WHERE authorize > 0 AND authorize < 3")
	return err
}
