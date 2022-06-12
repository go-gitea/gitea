// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"strconv"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
	gofff_format "lab.forgefriends.org/friendlyforgeformat/gofff/format"
)

func TestGiteaUploadRemapLocalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	uploader := NewGiteaLocalUploader(context.Background(), doer, doer.Name, base.MigrateOptions{})
	// call remapLocalUser
	uploader.sameApp = true

	externalID := int64(1234567)
	externalName := "username"
	source := gofff_format.Release{
		PublisherID:   externalID,
		PublisherName: externalName,
	}

	//
	// The externalID does not match any existing user, everything
	// belongs to the doer
	//
	target := models.Release{}
	uploader.userMap = make(map[int64]int64)
	err := uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// The externalID matches a known user but the name does not match,
	// everything belongs to the doer
	//
	source.PublisherID = user.ID
	target = models.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// The externalID and externalName match an existing user, everything
	// belongs to the existing user
	//
	source.PublisherName = user.Name
	target = models.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, user.ID, target.GetUserID())
}

func TestGiteaUploadRemapExternalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	uploader := NewGiteaLocalUploader(context.Background(), doer, doer.Name, base.MigrateOptions{})
	uploader.gitServiceType = structs.GiteaService
	// call remapExternalUser
	uploader.sameApp = false

	externalID := int64(1234567)
	externalName := "username"
	source := gofff_format.Release{
		PublisherID:   externalID,
		PublisherName: externalName,
	}

	//
	// When there is no user linked to the external ID, the migrated data is authored
	// by the doer
	//
	uploader.userMap = make(map[int64]int64)
	target := models.Release{}
	err := uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// Link the external ID to an existing user
	//
	linkedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	externalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    strconv.FormatInt(externalID, 10),
		UserID:        linkedUser.ID,
		LoginSourceID: 0,
		Provider:      structs.GiteaService.Name(),
	}
	err = user_model.LinkExternalToUser(linkedUser, externalLoginUser)
	assert.NoError(t, err)

	//
	// When a user is linked to the external ID, it becomes the author of
	// the migrated data
	//
	uploader.userMap = make(map[int64]int64)
	target = models.Release{}
	err = uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, linkedUser.ID, target.GetUserID())
}
