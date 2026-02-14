// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"fmt"

	"xorm.io/xorm"
)

func FixClosedMilestoneCompleteness(x *xorm.Engine) error {
	// Update all milestones to recalculate completeness with the new logic:
	// - Closed milestones with 0 issues should show 100%
	// - All other milestones should calculate based on closed/total ratio
	_, err := x.Exec("UPDATE `milestone` SET completeness=(CASE WHEN is_closed = ? AND num_issues = 0 THEN 100 ELSE 100*num_closed_issues/(CASE WHEN num_issues > 0 THEN num_issues ELSE 1 END) END)",
		true,
	)
	if err != nil {
		return fmt.Errorf("error updating milestone completeness: %w", err)
	}

	return nil
}
