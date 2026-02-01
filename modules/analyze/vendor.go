// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"strings"
)

// vendorDirs is a set of directory names that indicate vendored third-party code.
// All names are lowercase for case-insensitive matching.
var vendorDirs = map[string]bool{
	"vendor":           true,
	"vendors":          true,
	"node_modules":     true,
	"bower_components": true,
	"godeps":           true,
	"third_party":      true,
	"3rdparty":         true,
	"external":         true,
	"externals":        true,
}

// IsVendor returns whether or not path is a vendor path.
// This is a more restrictive check than go-enry's IsVendor, which marks many files
// as "vendored" for the purpose of language statistics (e.g., .gitignore, .github/,
// testdata/, minified files). For Gitea's diff view, we only want to show "Vendored"
// for files that are truly third-party dependencies in typical vendor directories.
// See https://github.com/go-gitea/gitea/issues/22618
func IsVendor(path string) bool {
	pathLower := strings.ToLower(path)
	for _, component := range strings.Split(pathLower, "/") {
		if vendorDirs[component] {
			return true
		}
	}
	return false
}
