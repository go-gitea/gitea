// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_addConfidentialClientColumnToOAuth2ApplicationTable(t *testing.T) {
	// premigration
	type OAuth2Application struct {
		ID int64
	}

	// Prepare and load the testing database
	x, deferable := prepareTestEnv(t, 0, new(OAuth2Application))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := addConfidentialClientColumnToOAuth2ApplicationTable(x); err != nil {
		assert.NoError(t, err)
		return
	}

	// postmigration
	type ExpectedOAuth2Application struct {
		ID                 int64
		ConfidentialClient bool
	}

	got := []ExpectedOAuth2Application{}
	if err := x.Table("o_auth2_application").Select("id, confidential_client").Find(&got); !assert.NoError(t, err) {
		return
	}

	assert.NotEmpty(t, got)
	for _, e := range got {
		assert.True(t, e.ConfidentialClient)
	}
}
