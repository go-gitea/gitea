// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"
)

func TestIsAuthenticationError(t *testing.T) {
	errDummy := errors.New("dummy")
	cases := []struct {
		name string
		want bool
		err  error
	}{
		{"git authentication failed", true, gitcmd.NewRunStdError(errDummy, "fatal: Authentication failed for 'https://host/repo.git/'")},
		{"git could not read username", true, fmt.Errorf("%w", gitcmd.NewRunStdError(errDummy, "fatal: could not read Username for 'https://host'"))},
		{"github unauthorized", true, util.SanitizeErrorCredentialURLs(&github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusUnauthorized}})},
		{"github other", false, &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusNotFound}}},
		{"github nil response", false, &github.ErrorResponse{}},
		{"unrelated error", false, errDummy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, IsAuthenticationError(c.err))
		})
	}
}

func TestMigrateWhiteBlocklist(t *testing.T) {
	adminUser := &user_model.User{IsAdmin: true}
	nonAdminUser := &user_model.User{}

	defer test.MockVariableValue(&setting.Migrations.AllowedHostList, "external")()
	defer test.MockVariableValue(&setting.Migrations.BlockedHostList, "8.8.4.4")()
	assert.NoError(t, IsMigrateURLAllowed("https://8.8.8.8/go-gitea/gitea.git", nonAdminUser))
	assert.Error(t, IsMigrateURLAllowed("https://8.8.4.4/go-gitea/gitea.git", nonAdminUser))
	assert.Error(t, IsMigrateURLAllowed("https://[64:ff9b::a9fe:a9fe]/go-gitea/gitea.git", nonAdminUser))

	old := setting.ImportLocalPaths
	setting.ImportLocalPaths = false

	assert.Error(t, IsMigrateURLAllowed("/home/foo/bar/goo", adminUser))

	setting.ImportLocalPaths = true
	abs, err := filepath.Abs(".")
	assert.NoError(t, err)

	assert.NoError(t, IsMigrateURLAllowed(abs, adminUser))
	assert.Error(t, IsMigrateURLAllowed(abs, nonAdminUser))

	nonAdminUser.AllowImportLocal = true
	assert.NoError(t, IsMigrateURLAllowed(abs, nonAdminUser))

	setting.ImportLocalPaths = old
}
