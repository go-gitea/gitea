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

	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	packageName := "admin-test-package"
	packageVersion := "1.0.0"

	url := fmt.Sprintf("/api/packages/%s/generic/%s/%s/file.bin", user4.Name, packageName, packageVersion)
	req := NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{})).AddBasicAuth(user4.Name)
	MakeRequest(t, req, http.StatusCreated)

	adminToken := getUserToken(t, "user1", auth_model.AccessTokenScopeReadAdmin)
	req = NewRequest(t, "GET", "/api/v1/admin/packages").AddTokenAuth(adminToken)
	resp := MakeRequest(t, req, http.StatusOK)

	apiPackages := DecodeJSON(t, resp, []*api.Package{})
	actual := map[string]any{}
	for _, p := range apiPackages {
		actual[p.Name] = map[string]any{"ownerName": p.Owner.UserName, "type": p.Type, "version": p.Version}
	}
	assert.Equal(t, map[string]any{
		packageName: map[string]any{"ownerName": user4.Name, "type": "generic", "version": packageVersion},
	}, actual)
}
