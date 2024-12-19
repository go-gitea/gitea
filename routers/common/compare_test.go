// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareRouters(t *testing.T) {
	kases := []struct {
		router        string
		compareRouter *CompareRouter
	}{
		{
			router: "main...develop",
			compareRouter: &CompareRouter{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				DotTimes:   3,
			},
		},
		{
			router: "main..develop",
			compareRouter: &CompareRouter{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				DotTimes:   2,
			},
		},
		{
			router: "main^...develop",
			compareRouter: &CompareRouter{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				CaretTimes: 1,
				DotTimes:   3,
			},
		},
		{
			router: "main^^^^^...develop",
			compareRouter: &CompareRouter{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				CaretTimes: 5,
				DotTimes:   3,
			},
		},
		{
			router: "develop",
			compareRouter: &CompareRouter{
				HeadOriRef: "develop",
				DotTimes:   3,
			},
		},
		{
			router: "lunny/forked_repo:develop",
			compareRouter: &CompareRouter{
				HeadOwnerName: "lunny",
				HeadRepoName:  "forked_repo",
				HeadOriRef:    "develop",
				DotTimes:      3,
			},
		},
		{
			router: "main...lunny/forked_repo:develop",
			compareRouter: &CompareRouter{
				BaseOriRef:    "main",
				HeadOwnerName: "lunny",
				HeadRepoName:  "forked_repo",
				HeadOriRef:    "develop",
				DotTimes:      3,
			},
		},
		{
			router: "main...lunny/forked_repo:develop",
			compareRouter: &CompareRouter{
				BaseOriRef:    "main",
				HeadOwnerName: "lunny",
				HeadRepoName:  "forked_repo",
				HeadOriRef:    "develop",
				DotTimes:      3,
			},
		},
		{
			router: "main^...lunny/forked_repo:develop",
			compareRouter: &CompareRouter{
				BaseOriRef:    "main",
				HeadOwnerName: "lunny",
				HeadRepoName:  "forked_repo",
				HeadOriRef:    "develop",
				DotTimes:      3,
				CaretTimes:    1,
			},
		},
		{
			router: "v1.0...v1.1",
			compareRouter: &CompareRouter{
				BaseOriRef: "v1.0",
				HeadOriRef: "v1.1",
				DotTimes:   3,
			},
		},
		{
			router: "teabot-patch-1...v0.0.1",
			compareRouter: &CompareRouter{
				BaseOriRef: "teabot-patch-1",
				HeadOriRef: "v0.0.1",
				DotTimes:   3,
			},
		},
		{
			router: "teabot:feature1",
			compareRouter: &CompareRouter{
				HeadOwnerName: "teabot",
				HeadOriRef:    "feature1",
				DotTimes:      3,
			},
		},
		{
			router: "8eb19a5ae19abae15c0666d4ab98906139a7f439...283c030497b455ecfa759d4649f9f8b45158742e",
			compareRouter: &CompareRouter{
				BaseOriRef: "8eb19a5ae19abae15c0666d4ab98906139a7f439",
				HeadOriRef: "283c030497b455ecfa759d4649f9f8b45158742e",
				DotTimes:   3,
			},
		},
	}
	for _, kase := range kases {
		t.Run(kase.router, func(t *testing.T) {
			r, err := parseCompareRouter(kase.router)
			assert.NoError(t, err)
			assert.EqualValues(t, kase.compareRouter, r)
		})
	}
}
