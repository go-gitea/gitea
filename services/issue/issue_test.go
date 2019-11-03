// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestGetRefEndNamesAndURLs(t *testing.T) {
	issues := []*models.Issue{
		{ID: 1, Ref: "refs/heads/branch1"},
		{ID: 2, Ref: "refs/tags/tag1"},
	}
	repoLink := "/foo/bar"

	endNames, urls := GetRefEndNamesAndURLs(issues, repoLink)
	assert.EqualValues(t, map[int64]string{1: "branch1", 2: "tag1"}, endNames)
	assert.EqualValues(t, map[int64]string{1: repoLink + "/src/branch/branch1", 2: repoLink + "/src/tag/tag1"}, urls)
}
