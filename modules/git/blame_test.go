// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadingBlameOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	blameReader, err := CreateBlameReader(ctx, "./tests/repos/repo5_pulls", "f32b0a9dfd09a60f616f29158f772cedd89942d2", "README.md")
	assert.NoError(t, err)
	defer blameReader.Close()

	parts := []*BlamePart{
		{
			"72866af952e98d02a73003501836074b286a78f6",
			[]string{
				"# test_repo",
				"Test repository for testing migration from github to gitea",
			},
		},
		{
			"f32b0a9dfd09a60f616f29158f772cedd89942d2",
			[]string{"", "Do not make any changes to this repo it is used for unit testing"},
		},
	}

	for _, part := range parts {
		actualPart, err := blameReader.NextPart()
		assert.NoError(t, err)
		assert.Equal(t, part, actualPart)
	}
}
