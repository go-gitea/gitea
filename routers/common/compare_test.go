// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"gitea.dev/modules/util"

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
			},
		},
		{
			input: "main..develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				CompareSeparator: "..",
				HeadOriRef:       "develop",
			},
		},
		{
			input: "main^...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				BaseOriRefSuffix: "^",
				CompareSeparator: "...",
				HeadOriRef:       "develop",
			},
		},
		{
			input: "main^^^^^...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				BaseOriRefSuffix: "^^^^^",
				CompareSeparator: "...",
				HeadOriRef:       "develop",
			},
		},
		{
			input: "develop",
			CompareRouterReq: &CompareRouterReq{
				CompareSeparator: "...",
				HeadOriRef:       "develop",
			},
		},
		{
			input: "teabot:feature1",
			CompareRouterReq: &CompareRouterReq{
				CompareSeparator: "...",
				HeadOwner:        "teabot",
				HeadOriRef:       "feature1",
			},
		},
		{
			input: "lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				CompareSeparator: "...",
				HeadOwner:        "lunny",
				HeadRepoName:     "forked_repo",
				HeadOriRef:       "develop",
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
			},
		},
		{
			input: "main...develop^",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				CompareSeparator: "...",
				HeadOriRef:       "develop",
				HeadOriRefSuffix: "^",
			},
		},
		{
			input: "main~2...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				BaseOriRefSuffix: "~2",
				CompareSeparator: "...",
				HeadOriRef:       "develop",
			},
		},
		{
			input: "main...lunny/forked_repo:develop~3",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:       "main",
				CompareSeparator: "...",
				HeadOwner:        "lunny",
				HeadRepoName:     "forked_repo",
				HeadOriRef:       "develop",
				HeadOriRefSuffix: "~3",
			},
		},
		{
			input: "develop^",
			CompareRouterReq: &CompareRouterReq{
				CompareSeparator: "...",
				HeadOriRef:       "develop",
				HeadOriRefSuffix: "^",
			},
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.CompareRouterReq, ParseCompareRouterParam(c.input), "input: %s", c.input)
	}
}

func TestResolveRefWithSuffix(t *testing.T) {
	// The ^{...}, @{...} and :path forms address non-commit objects or reflog state, so they are
	// rejected before any repository access and a nil repo is fine here.
	for _, refSuffix := range []string{"^{/Add}", "^{commit}", "@{upstream}", "~1:path"} {
		ref, err := ResolveRefWithSuffix(t.Context(), nil, "branch", refSuffix)
		assert.ErrorIs(t, err, util.ErrInvalidArgument, "suffix %q", refSuffix)
		assert.Empty(t, ref, "suffix %q", refSuffix)
	}
}
