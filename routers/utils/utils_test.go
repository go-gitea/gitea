// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestRemoveUsernameParameterSuffix(t *testing.T) {
	assert.Equal(t, "foobar", RemoveUsernameParameterSuffix("foobar (Foo Bar)"))
	assert.Equal(t, "foobar", RemoveUsernameParameterSuffix("foobar"))
	assert.Equal(t, "", RemoveUsernameParameterSuffix(""))
}

var allRepoOrderByTypes = []models.RepoOrderByType{
	models.RepoOrderByAlphabetically,
	models.RepoOrderByAlphabeticallyReverse,
	models.RepoOrderByLeastUpdated,
	models.RepoOrderByRecentUpdated,
	models.RepoOrderByOldest,
	models.RepoOrderByNewest,
	models.RepoOrderByID,
}

func TestParseOrderByType(t *testing.T) {
	for _, orderByType := range allRepoOrderByTypes {
		s := ToQueryString(orderByType)
		assert.EqualValues(t, orderByType, ParseRepoOrderByType(s))
	}
	assert.EqualValues(t, models.RepoOrderByRecentUpdated,
		ParseRepoOrderByType(""))
	assert.EqualValues(t, models.RepoOrderByRecentUpdated,
		ParseRepoOrderByType("I don't match anything!"))
}
