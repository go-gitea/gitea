// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2/data"
)

// IsGenerated returns whether or not path is a generated path.
func IsGenerated(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := data.GeneratedCodeExtensions[ext]; ok {
		return true
	}

	for _, m := range data.GeneratedCodeNameMatchers {
		if m(path) {
			return true
		}
	}

	return false
}
