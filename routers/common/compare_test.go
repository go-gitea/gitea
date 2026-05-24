// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"gitea.dev/modules/optional"

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
				CompareSeparator: "...",
				HeadOriRef:       "v1.1",
				HeadGroupPath:    optional.Some(""),
			},
		},
		{
			input: "main..develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				CompareSeparator: "..",
				HeadOriRef:       "develop",
				HeadGroupPath:    optional.Some(""),
			},
		},
		{
			input: "main^...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				BaseOriRefSuffix: "^",
				CompareSeparator: "...",
				HeadOriRef:       "develop",
				HeadGroupPath:    optional.Some(""),
			},
		},
		{
			input: "main^^^^^...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				BaseOriRefSuffix: "^^^^^",
				CompareSeparator: "...",
				HeadOriRef:       "develop",
				HeadGroupPath:    optional.Some(""),
			},
		},
		{
			input: "develop",
			CompareRouterReq: &CompareRouterReq{
				CompareSeparator: "...",
				HeadOriRef:       "develop",
				HeadGroupPath:    optional.Some(""),
			},
		},
		{
			input: "teabot:feature1",
			CompareRouterReq: &CompareRouterReq{
				CompareSeparator: "...",
				HeadOwner:        "teabot",
				HeadOriRef:       "feature1",
				HeadGroupPath:    optional.None[string](),
			},
		},
		{
			input: "lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				CompareSeparator: "...",
				HeadOwner:        "lunny",
				HeadRepoName:     "forked_repo",
				HeadOriRef:       "develop",
				HeadGroupPath:    optional.Some(""),
			},
		},
		{
			input: "main...lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				CompareSeparator: "...",
				HeadOwner:        "lunny",
				HeadRepoName:     "forked_repo",
				HeadOriRef:       "develop",
				HeadGroupPath:    optional.Some(""),
			},
		},
		{
			input: "main^...lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				BaseOriRefSuffix: "^",
				CompareSeparator: "...",
				HeadOwner:        "lunny",
				HeadRepoName:     "forked_repo",
				HeadOriRef:       "develop",
				HeadGroupPath:    optional.Some(""),
			},
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.CompareRouterReq, ParseCompareRouterParam(c.input), "input: %s", c.input)
	}
}
