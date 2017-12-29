// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestAddReadOnlyDeployKey(t *testing.T) {
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/settings/keys")

	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 2)

	addKeyForm := auth.AddKeyForm{
		Title:   "read-only",
		Content: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
	}
	DeployKeysPost(ctx, addKeyForm)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		Name:    addKeyForm.Title,
		Content: addKeyForm.Content,
		Mode:    models.AccessModeRead,
	})
}

func TestAddReadWriteOnlyDeployKey(t *testing.T) {
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/settings/keys")

	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 2)

	addKeyForm := auth.AddKeyForm{
		Title:      "read-write",
		Content:    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
		IsWritable: true,
	}
	DeployKeysPost(ctx, addKeyForm)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		Name:    addKeyForm.Title,
		Content: addKeyForm.Content,
		Mode:    models.AccessModeWrite,
	})
}
