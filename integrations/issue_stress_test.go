// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build stress

package integrations

import (
	"testing"

	"code.gitea.io/gitea/models"
)

// TestStressCreateIssue do something
func TestStressCreateIssue(t *testing.T) {
	// TODO: refactor this to avoid including StressIssueNoDupIndex() in production
	models.StressIssueNoDupIndex(t)
}
