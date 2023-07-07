// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareRouters(t *testing.T) {
	kases := []struct {
		router        string
		compareRouter compareRouter
	}{
		{
			router: "main...develop",
			compareRouter: compareRouter{
				BaseBranch: "main",
				HeadBranch: "develop",
				DotTimes:   3,
			},
		},
		{
			router: "main^...develop",
			compareRouter: compareRouter{
				BaseBranch: "main",
				HeadBranch: "develop",
				CaretTimes: 1,
				DotTimes:   3,
			},
		},
		{
			router: "main^^^^^...develop",
			compareRouter: compareRouter{
				BaseBranch: "main",
				HeadBranch: "develop",
				CaretTimes: 5,
				DotTimes:   3,
			},
		},
		{
			router: "develop",
			compareRouter: compareRouter{
				HeadBranch: "develop",
			},
		},
		{
			router: "lunny/forked_repo:develop",
			compareRouter: compareRouter{
				HeadOwner:    "lunny",
				HeadRepoName: "forked_repo",
				HeadBranch:   "develop",
			},
		},
		{
			router: "main...lunny/forked_repo:develop",
			compareRouter: compareRouter{
				BaseBranch:   "main",
				HeadOwner:    "lunny",
				HeadRepoName: "forked_repo",
				HeadBranch:   "develop",
				DotTimes:     3,
			},
		},
	}
	for _, kase := range kases {
		r := parseCompareRouters(kase.router)
		assert.EqualValues(t, kase.compareRouter, r)
	}
}
