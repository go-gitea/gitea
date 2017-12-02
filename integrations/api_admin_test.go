// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"
)

func TestAPIAdminCreateSSHKey(t *testing.T) {
	prepareTestEnv(t)

	// user1 is an admin user
	session := loginUser(t, "user1")
	req := NewRequestWithValues(t, "POST", "/api/v1/admin/users/user2/keys", map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
		"title": "test-key",
	})
	resp := session.MakeRequest(t, req, http.StatusOK)

	keyOwner := models.AssertExistsAndLoadBean(t, &models.User{LoginName: "user2"}).(*models.User)
	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)
	models.AssertExistsAndLoadBean(t, &models.PublicKey{
		ID:          newPublicKey.ID,
		Name:        newPublicKey.Title,
		Content:     newPublicKey.Key,
		Fingerprint: newPublicKey.Fingerprint,
		OwnerID:     keyOwner.ID,
	})
}
