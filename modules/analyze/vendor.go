// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"strings"
)

// vendorPatterns is a list of directory path prefixes/components that indicate
// the file is vendored third-party code. All patterns are lowercase for
// case-insensitive matching.
var vendorPatterns = []string{
	"vendor/",
	"vendors/",
	"node_modules/",
	"bower_components/",
	"godeps/",
	"third_party/",
	"3rdparty/",
	"external/",
	"externals/",
}

// IsVendor returns whether or not path is a vendor path.
// This is a more restrictive check than go-enry's IsVendor, which marks many files
// as "vendored" for the purpose of language statistics (e.g., .gitignore, .github/,
// testdata/, minified files). For Gitea's diff view, we only want to show "Vendored"
// for files that are truly third-party dependencies in typical vendor directories.
// See https://github.com/go-gitea/gitea/issues/22618
func IsVendor(path string) bool {
	pathLower := strings.ToLower(path)
	for _, pattern := range vendorPatterns {
		if strings.HasPrefix(pathLower, pattern) || strings.Contains(pathLower, "/"+pattern) {
			return true
		}
	}
	return false
}
