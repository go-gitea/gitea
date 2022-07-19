// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oci

import (
	"regexp"
)

var referencePattern = regexp.MustCompile(`\A[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}\z`)

type Reference string

func (r Reference) Validate() bool {
	return referencePattern.MatchString(string(r))
}
