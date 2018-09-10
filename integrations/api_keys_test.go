// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"
)

func TestViewDeployKeysNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/keys")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestCreateDeployKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/keys", api.CreateKeyOption{
		Title: "title",
		Key:   "key",
	})
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestGetDeployKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestDeleteDeployKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "DELETE", "/api/v1/repos/user2/repo1/keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestCreateReadOnlyDeployKey(t *testing.T) {
	prepareTestEnv(t)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{Name: "repo1"}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/keys?token=%s", repoOwner.Name, repo.Name, token)
	rawKeyBody := api.CreateKeyOption{
		Title:    "read-only",
		Key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
		ReadOnly: true,
	}
	req := NewRequestWithJSON(t, "POST", keysURL, rawKeyBody)
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var newDeployKey api.DeployKey
	DecodeJSON(t, resp, &newDeployKey)
	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		ID:      newDeployKey.ID,
		Name:    rawKeyBody.Title,
		Content: rawKeyBody.Key,
		Mode:    models.AccessModeRead,
	})
}

func TestCreateReadWriteDeployKey(t *testing.T) {
	prepareTestEnv(t)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{Name: "repo1"}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/keys?token=%s", repoOwner.Name, repo.Name, token)
	rawKeyBody := api.CreateKeyOption{
		Title: "read-write",
		Key:   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDsufOCrDDlT8DLkodnnJtbq7uGflcPae7euTfM+Laq4So+v4WeSV362Rg0O/+Sje1UthrhN6lQkfRkdWIlCRQEXg+LMqr6RhvDfZquE2Xwqv/itlz7LjbdAUdYoO1iH7rMSmYvQh4WEnC/DAacKGbhdGIM/ZBz0z6tHm7bPgbI9ykEKekTmPwQFP1Qebvf5NYOFMWqQ2sCEAI9dBMVLoojsIpV+KADf+BotiIi8yNfTG2rzmzpxBpW9fYjd1Sy1yd4NSUpoPbEJJYJ1TrjiSWlYOVq9Ar8xW1O87i6gBjL/3zN7ANeoYhaAXupdOS6YL22YOK/yC0tJtXwwdh/eSrh",
	}
	req := NewRequestWithJSON(t, "POST", keysURL, rawKeyBody)
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var newDeployKey api.DeployKey
	DecodeJSON(t, resp, &newDeployKey)
	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		ID:      newDeployKey.ID,
		Name:    rawKeyBody.Title,
		Content: rawKeyBody.Key,
		Mode:    models.AccessModeWrite,
	})
}
