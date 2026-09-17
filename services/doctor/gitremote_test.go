// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSelfOverwritingFetchRefSpec(t *testing.T) {
	cases := map[string]bool{
		"+refs/*:refs/*":                       true,
		"+refs/heads/*:refs/heads/*":           true,
		"refs/heads/main:refs/heads/main":      true,
		"+refs/heads/*:refs/remotes/origin/*":  false,
		"refs/heads/*:refs/remotes/upstream/*": false,
		"refs/heads/main":                      false,
		"":                                     false,
	}
	for refSpec, expected := range cases {
		assert.Equal(t, expected, isSelfOverwritingFetchRefSpec(refSpec), refSpec)
	}
}
