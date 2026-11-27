// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	bareRepo1 := mockRepository("repo1_bare")
	revision := []string{"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"}

	// The three commits on this revision are dated 2018-04-18, 2017-12-19 and 2017-12-19.
	for _, tc := range []struct {
		name     string
		options  CommitsCountOptions
		expected int64
	}{
		{"revision", CommitsCountOptions{Revision: revision}, 3},
		{"no filter", CommitsCountOptions{Revision: revision}, 3},
		{"since keeps newer commits", CommitsCountOptions{Revision: revision, Since: "2018-01-01"}, 1},
		{"until keeps older commits", CommitsCountOptions{Revision: revision, Until: "2018-01-01"}, 2},
		{"since and until bound the range", CommitsCountOptions{Revision: revision, Since: "2017-12-19T22:16:00-08:00", Until: "2018-01-01"}, 1},
		{"without base", CommitsCountOptions{Not: "master", Revision: []string{"branch1"}}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commitsCount, err := CommitsCount(t.Context(), bareRepo1, tc.options)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, commitsCount)
		})
	}
}
