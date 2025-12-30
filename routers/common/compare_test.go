// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareRouterReq(t *testing.T) {
	cases := []struct {
		input            string
		CompareRouterReq *CompareRouterReq
	}{
		{
			input:            "",
			CompareRouterReq: &CompareRouterReq{},
		},
		{
			input: "v1.0...v1.1",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "v1.0",
				HeadOriRef:       "v1.1",
				CompareSeparator: "...",
			},
		},
		{
			input: "main..develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				HeadOriRef:       "develop",
				CompareSeparator: "..",
			},
		},
		{
			input: "main^...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				HeadOriRef:       "develop",
				CompareSeparator: "^...",
			},
		},
		{
			input: "main^^^^^...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				HeadOriRef:       "develop",
				CompareSeparator: "^^^^^...",
			},
		},
		{
			input: "develop",
			CompareRouterReq: &CompareRouterReq{
				HeadOriRef:       "develop",
				CompareSeparator: "...",
			},
		},
		{
			input: "teabot:feature1",
			CompareRouterReq: &CompareRouterReq{
				HeadOwner:        "teabot",
				HeadOriRef:       "feature1",
				CompareSeparator: "...",
			},
		},
		{
			input: "lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				HeadOwner:        "lunny",
				HeadRepoName:     "forked_repo",
				HeadOriRef:       "develop",
				CompareSeparator: "...",
			},
		},
		{
			input: "main...lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				HeadOwner:        "lunny",
				HeadRepoName:     "forked_repo",
				HeadOriRef:       "develop",
				CompareSeparator: "...",
			},
		},
		{
			input: "main^...lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				HeadOwner:        "lunny",
				HeadRepoName:     "forked_repo",
				HeadOriRef:       "develop",
				CompareSeparator: "^...",
			},
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.CompareRouterReq, ParseCompareRouterParam(c.input), "input: %s", c.input)
	}
}
