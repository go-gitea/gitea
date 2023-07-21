// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBranchRuleMatchPriority(t *testing.T) {
	kases := []struct {
		Rules            []string
		BranchName       string
		ExpectedMatchIdx int
	}{
		{
			Rules:            []string{"release/*", "release/v1.17"},
			BranchName:       "release/v1.17",
			ExpectedMatchIdx: 1,
		},
		{
			Rules:            []string{"release/v1.17", "release/*"},
			BranchName:       "release/v1.17",
			ExpectedMatchIdx: 0,
		},
		{
			Rules:            []string{"release/**/v1.17", "release/test/v1.17"},
			BranchName:       "release/test/v1.17",
			ExpectedMatchIdx: 1,
		},
		{
			Rules:            []string{"release/test/v1.17", "release/**/v1.17"},
			BranchName:       "release/test/v1.17",
			ExpectedMatchIdx: 0,
		},
		{
			Rules:            []string{"release/**", "release/v1.0.0"},
			BranchName:       "release/v1.0.0",
			ExpectedMatchIdx: 1,
		},
		{
			Rules:            []string{"release/v1.0.0", "release/**"},
			BranchName:       "release/v1.0.0",
			ExpectedMatchIdx: 0,
		},
		{
			Rules:            []string{"release/**", "release/v1.0.0"},
			BranchName:       "release/v2.0.0",
			ExpectedMatchIdx: 0,
		},
		{
			Rules:            []string{"release/*", "release/v1.0.0"},
			BranchName:       "release/1/v2.0.0",
			ExpectedMatchIdx: -1,
		},
	}

	for _, kase := range kases {
		var pbs ProtectedBranchRules
		for _, rule := range kase.Rules {
			pbs = append(pbs, &ProtectedBranch{RuleName: rule})
		}
		pbs.sort()
		matchedPB := pbs.GetFirstMatched(kase.BranchName)
		if matchedPB == nil {
			if kase.ExpectedMatchIdx >= 0 {
				assert.Error(t, fmt.Errorf("no matched rules but expected %s[%d]", kase.Rules[kase.ExpectedMatchIdx], kase.ExpectedMatchIdx))
			}
		} else {
			assert.EqualValues(t, kase.Rules[kase.ExpectedMatchIdx], matchedPB.RuleName)
		}
	}
}
