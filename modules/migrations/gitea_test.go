// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"github.com/stretchr/testify/assert"
)

func TestGiteaUploadRepo(t *testing.T) {
	if uploadToken == nil || *uploadToken == "" {
		t.Skipf("Gitea token is not provided and the test ignored")
		return
	}

	var user = &models.User{
		ID: 1,
	}

	err := MigrateRepository(user, "xorm", MigrateOptions{
		RemoteURL: "https://github.com/go-xorm/builder",
		Name:      "builder-" + time.Now().Format("2006-01-02-15-04-05"),
	})
	assert.NoError(t, err)
}
