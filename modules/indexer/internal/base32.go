// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"fmt"
	"strconv"
)

func Base36(i int64) string {
	return strconv.FormatInt(i, 36)
}

func ParseBase36(s string) (int64, error) {
	i, err := strconv.ParseInt(s, 36, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid base36 integer %q: %w", s, err)
	}
	return i, nil
}
