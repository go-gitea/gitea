// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"path"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

// IsVendor returns whether the path is a vendor path.
// It uses go-enry's IsVendor function but overrides its detection for certain
// special cases that shouldn't be marked as vendored in the diff view.
func IsVendor(treePath string) bool {
	if !enry.IsVendor(treePath) {
		return false
	}

	// Override detection for single files
	basename := path.Base(treePath)
	switch basename {
	case ".gitignore", ".gitattributes", ".gitmodules":
		return false
	}
	if strings.HasPrefix(treePath, ".github/") || strings.HasPrefix(treePath, ".gitea/") {
		return false
	}
	return true
}
