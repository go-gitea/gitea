// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"slices"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsistencyCheck(t *testing.T) {
	checks := prepareDBConsistencyChecks()
	idx := slices.IndexFunc(checks, func(check consistencyCheck) bool {
		return check.Name == "Orphaned OAuth2Application without existing User"
	})
	require.NotEqual(t, -1, idx)

	_ = db.TruncateBeans(db.DefaultContext, &auth.OAuth2Application{}, &user.User{})
	_ = db.TruncateBeans(db.DefaultContext, &auth.OAuth2Application{}, &auth.OAuth2Application{})

	err := db.Insert(db.DefaultContext, &user.User{ID: 1})
	assert.NoError(t, err)
	err = db.Insert(db.DefaultContext, &auth.OAuth2Application{Name: "test-oauth2-app-1", ClientID: "client-id-1"})
	assert.NoError(t, err)
	err = db.Insert(db.DefaultContext, &auth.OAuth2Application{Name: "test-oauth2-app-2", ClientID: "client-id-2", UID: 1})
	assert.NoError(t, err)
	err = db.Insert(db.DefaultContext, &auth.OAuth2Application{Name: "test-oauth2-app-3", ClientID: "client-id-3", UID: 99999999})
	assert.NoError(t, err)

	unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ClientID: "client-id-1"})
	unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ClientID: "client-id-2"})
	unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ClientID: "client-id-3"})

	oauth2AppCheck := checks[idx]
	err = oauth2AppCheck.Run(db.DefaultContext, log.GetManager().GetLogger(log.DEFAULT), true)
	assert.NoError(t, err)

	unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ClientID: "client-id-1"})
	unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ClientID: "client-id-2"})
	unittest.AssertNotExistsBean(t, &auth.OAuth2Application{ClientID: "client-id-3"})
}
