// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePushPorcelain(t *testing.T) {
	out := "To https://example.com/r.git\n" +
		"*\trefs/heads/new:refs/heads/new\t[new branch]\n" +
		" \trefs/heads/ff:refs/heads/ff\t1111..2222\n" +
		"+\trefs/heads/forced:refs/heads/forced\t1111...2222 (forced update)\n" +
		"-\t:refs/tags/gone\t[deleted]\n" +
		"=\trefs/heads/same:refs/heads/same\t[up to date]\n" +
		"!\trefs/heads/bad:refs/heads/bad\t[remote rejected] (pre-receive hook declined)\n" +
		"Done\n"
	results := ParsePushPorcelain(out)
	assert.Len(t, results, 6)
	assert.Equal(t, "refs/tags/gone", results[3].Ref)
	assert.True(t, results[3].Deleted)
	assert.True(t, results[4].UpToDate)
	assert.True(t, results[5].Failed)
	assert.Equal(t, "[remote rejected] (pre-receive hook declined)", results[5].Summary)
	assert.False(t, results[0].Failed)
}
