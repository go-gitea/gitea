// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_AddConfidentialClientColumnToOAuth2ApplicationTable(t *testing.T) {
	// premigration
	type oauth2Application struct {
		ID int64
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(oauth2Application))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := AddConfidentialClientColumnToOAuth2ApplicationTable(x); err != nil {
		assert.NoError(t, err)
		return
	}

	// postmigration
	type ExpectedOAuth2Application struct {
		ID                 int64
		ConfidentialClient bool
	}

	got := []ExpectedOAuth2Application{}
	if err := x.Table("oauth2_application").Select("id, confidential_client").Find(&got); !assert.NoError(t, err) {
		return
	}

	assert.NotEmpty(t, got)
	for _, e := range got {
		assert.True(t, e.ConfidentialClient)
	}
}
