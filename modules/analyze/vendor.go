// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"github.com/go-enry/go-enry/v2"
)

// IsVendor returns whether or not path is a vendor path.
func IsVendor(path string) bool {
	return enry.IsVendor(path)
}
