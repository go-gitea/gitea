// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

// IsVendor returns whether or not path is a vendor path.
// It uses go-enry's IsVendor function but overrides its detection for certain
// special cases that shouldn't be marked as vendored in the diff view.
// See https://github.com/go-gitea/gitea/issues/22618
func IsVendor(path string) bool {
	if !enry.IsVendor(path) {
		return false
	}

	// go-enry marks certain files as "vendored" for language statistics purposes,
	// but these shouldn't show as "Vendored" in Gitea's diff view.
	// Override detection for these special cases.
	basename := filepath.Base(path)
	switch basename {
	case ".gitignore", ".gitattributes", ".gitmodules":
		return false
	}

	// Files in .github/ or .gitea/ directories shouldn't be marked as vendored
	if strings.HasPrefix(path, ".github/") || strings.HasPrefix(path, ".gitea/") {
		return false
	}

	return true
}
