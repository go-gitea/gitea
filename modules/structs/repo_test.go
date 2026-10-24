// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitServiceTypeVariants(t *testing.T) {
	assert.Equal(t, []GitServiceType{BitbucketService, BitbucketServerService}, BitbucketServerService.Variants())
	assert.Equal(t, BitbucketServerService.Variants(), BitbucketService.Variants())
	assert.Nil(t, GithubService.Variants())

	assert.Equal(t, "Bitbucket", BitbucketService.ProviderTitle())
	assert.Equal(t, "Bitbucket", BitbucketServerService.ProviderTitle())
	assert.Equal(t, "GitHub", GithubService.ProviderTitle())
	assert.True(t, BitbucketService.TokenAuth())
}
