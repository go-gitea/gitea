// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestGoGet(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/blah/glah/plah?go-get=1")
	resp := MakeRequest(t, req, http.StatusOK)

	expected := fmt.Sprintf(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="%[1]s:%[2]s/blah/glah git %[3]sblah/glah.git">
		<meta name="go-source" content="%[1]s:%[2]s/blah/glah _ %[3]sblah/glah/src/branch/master{/dir} %[3]sblah/glah/src/branch/master{/dir}/{file}#L{line}">
	</head>
	<body>
		go get --insecure %[1]s:%[2]s/blah/glah
	</body>
</html>`, setting.Domain, setting.HTTPPort, setting.AppURL)

	assert.Equal(t, expected, resp.Body.String())
}

func TestGoGetSubDir(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	repoEditOption := getRepoEditOptionFromRepo(repo1)
	goModuleSubDir := "package"
	repoEditOption.GoModuleSubDir = &goModuleSubDir
	req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo1.Name), &repoEditOption).
		AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/plah?go-get=1", user2.Name, repo1.Name))
	resp := MakeRequest(t, req, http.StatusOK)

	expected := fmt.Sprintf(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="%[1]s:%[2]s/%[4]s/%[5]s git %[3]s%[4]s/%[5]s.git package">
		<meta name="go-source" content="%[1]s:%[2]s/%[4]s/%[5]s _ %[3]s%[4]s/%[5]s/src/branch/master/package{/dir} %[3]s%[4]s/%[5]s/src/branch/master/package{/dir}/{file}#L{line}">
	</head>
	<body>
		go get --insecure %[1]s:%[2]s/%[4]s/%[5]s
	</body>
</html>`, setting.Domain, setting.HTTPPort, setting.AppURL, user2.Name, repo1.Name)

	assert.Equal(t, expected, resp.Body.String())
}

func TestGoGetForSSH(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	old := setting.Repository.GoGetCloneURLProtocol
	defer func() {
		setting.Repository.GoGetCloneURLProtocol = old
	}()
	setting.Repository.GoGetCloneURLProtocol = "ssh"

	req := NewRequest(t, "GET", "/blah/glah/plah?go-get=1")
	resp := MakeRequest(t, req, http.StatusOK)

	expected := fmt.Sprintf(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="%[1]s:%[2]s/blah/glah git ssh://git@%[4]s:%[5]d/blah/glah.git">
		<meta name="go-source" content="%[1]s:%[2]s/blah/glah _ %[3]sblah/glah/src/branch/master{/dir} %[3]sblah/glah/src/branch/master{/dir}/{file}#L{line}">
	</head>
	<body>
		go get --insecure %[1]s:%[2]s/blah/glah
	</body>
</html>`, setting.Domain, setting.HTTPPort, setting.AppURL, setting.SSH.Domain, setting.SSH.Port)

	assert.Equal(t, expected, resp.Body.String())
}
