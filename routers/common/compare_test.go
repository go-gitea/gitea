// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestCompareRouterReq(t *testing.T) {
	unittest.PrepareTestEnv(t)

	kases := []struct {
		router           string
		CompareRouterReq *CompareRouterReq
	}{
		{
			router: "",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef: "",
				HeadOriRef: "",
				DotTimes:   0,
			},
		},
		{
			router: "main...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				DotTimes:   3,
			},
		},
		{
			router: "main..develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				DotTimes:   2,
			},
		},
		{
			router: "main^...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				CaretTimes: 1,
				DotTimes:   3,
			},
		},
		{
			router: "main^^^^^...develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				CaretTimes: 5,
				DotTimes:   3,
			},
		},
		{
			router: "develop",
			CompareRouterReq: &CompareRouterReq{
				HeadOriRef: "develop",
				DotTimes:   3,
			},
		},
		{
			router: "lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				HeadOwner:    "lunny",
				HeadRepoName: "forked_repo",
				HeadOriRef:   "develop",
				DotTimes:     3,
			},
		},
		{
			router: "main...lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:   "main",
				HeadOwner:    "lunny",
				HeadRepoName: "forked_repo",
				HeadOriRef:   "develop",
				DotTimes:     3,
			},
		},
		{
			router: "main...lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:   "main",
				HeadOwner:    "lunny",
				HeadRepoName: "forked_repo",
				HeadOriRef:   "develop",
				DotTimes:     3,
			},
		},
		{
			router: "main^...lunny/forked_repo:develop",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef:   "main",
				HeadOwner:    "lunny",
				HeadRepoName: "forked_repo",
				HeadOriRef:   "develop",
				DotTimes:     3,
				CaretTimes:   1,
			},
		},
		{
			router: "v1.0...v1.1",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef: "v1.0",
				HeadOriRef: "v1.1",
				DotTimes:   3,
			},
		},
		{
			router: "teabot-patch-1...v0.0.1",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef: "teabot-patch-1",
				HeadOriRef: "v0.0.1",
				DotTimes:   3,
			},
		},
		{
			router: "teabot:feature1",
			CompareRouterReq: &CompareRouterReq{
				HeadOwner:  "teabot",
				HeadOriRef: "feature1",
				DotTimes:   3,
			},
		},
		{
			router: "8eb19a5ae19abae15c0666d4ab98906139a7f439...283c030497b455ecfa759d4649f9f8b45158742e",
			CompareRouterReq: &CompareRouterReq{
				BaseOriRef: "8eb19a5ae19abae15c0666d4ab98906139a7f439",
				HeadOriRef: "283c030497b455ecfa759d4649f9f8b45158742e",
				DotTimes:   3,
			},
		},
	}

	for _, kase := range kases {
		t.Run(kase.router, func(t *testing.T) {
			r, err := ParseCompareRouterParam(kase.router)
			assert.NoError(t, err)
			assert.Equal(t, kase.CompareRouterReq, r)
		})
	}
}
