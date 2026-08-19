// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIAdminListPackages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	packageName := "admin-test-package"
	packageVersion := "1.0.0"

	url := fmt.Sprintf("/api/packages/%s/generic/%s/%s/file.bin", user.Name, packageName, packageVersion)
	req := NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{})).
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusCreated)

	adminToken := getUserToken(t, "user1", auth_model.AccessTokenScopeReadAdmin)
	req = NewRequest(t, "GET", "/api/v1/admin/packages").
		AddTokenAuth(adminToken)
	resp := MakeRequest(t, req, http.StatusOK)

	apiPackages := DecodeJSON(t, resp, []*api.Package{})
	found := false
	for _, p := range apiPackages {
		if p.Owner != nil &&
			p.Owner.UserName == user.Name &&
			p.Type == "generic" &&
			p.Name == packageName &&
			p.Version == packageVersion {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestAPIAdminListPackagesForbidden(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadAdmin)
	req := NewRequest(t, "GET", "/api/v1/admin/packages").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIAdminListPackagesNotLoggedIn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/admin/packages")
	MakeRequest(t, req, http.StatusUnauthorized)
}
