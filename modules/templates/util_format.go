// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"

	"code.gitea.io/gitea/modules/util"
)

func timeEstimateString(timeSec any) string {
	v, _ := util.ToInt64(timeSec)
	if v == 0 {
		return ""
	}
	return util.TimeEstimateString(v)
}

func countFmt(data any) string {
	// legacy code, not ideal, still used in some places
	num, err := util.ToInt64(data)
	if err != nil {
		return ""
	}
	if num < 1000 {
		return fmt.Sprintf("%d", num)
	} else if num < 1_000_000 {
		num2 := float32(num) / 1000.0
		return fmt.Sprintf("%.1fk", num2)
	} else if num < 1_000_000_000 {
		num2 := float32(num) / 1_000_000.0
		return fmt.Sprintf("%.1fM", num2)
	}
	num2 := float32(num) / 1_000_000_000.0
	return fmt.Sprintf("%.1fG", num2)
}
