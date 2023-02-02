// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oci

import (
	"regexp"
)

var referencePattern = regexp.MustCompile(`\A[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}\z`)

type Reference string

func (r Reference) Validate() bool {
	return referencePattern.MatchString(string(r))
}
