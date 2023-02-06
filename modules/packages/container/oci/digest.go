// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oci

import (
	"regexp"
	"strings"
)

var digestPattern = regexp.MustCompile(`\Asha256:[a-f0-9]{64}\z`)

type Digest string

// Validate checks if the digest has a valid SHA256 signature
func (d Digest) Validate() bool {
	return digestPattern.MatchString(string(d))
}

func (d Digest) Hash() string {
	p := strings.SplitN(string(d), ":", 2)
	if len(p) != 2 {
		return ""
	}
	return p[1]
}
