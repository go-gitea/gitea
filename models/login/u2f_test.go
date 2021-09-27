// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package login

import (
	"testing"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
	"github.com/tstranex/u2f"
)

func TestGetU2FRegistrationByID(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	res, err := GetU2FRegistrationByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "U2F Key", res.Name)

	_, err = GetU2FRegistrationByID(342432)
	assert.Error(t, err)
	assert.True(t, IsErrU2FRegistrationNotExist(err))
}

func TestGetU2FRegistrationsByUID(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	res, err := GetU2FRegistrationsByUID(1)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "U2F Key", res[0].Name)
}

func TestU2FRegistration_TableName(t *testing.T) {
	assert.Equal(t, "u2f_registration", U2FRegistration{}.TableName())
}

func TestU2FRegistration_UpdateCounter(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())
	reg := db.AssertExistsAndLoadBean(t, &U2FRegistration{ID: 1}).(*U2FRegistration)
	reg.Counter = 1
	assert.NoError(t, reg.UpdateCounter())
	db.AssertExistsIf(t, true, &U2FRegistration{ID: 1, Counter: 1})
}

func TestU2FRegistration_UpdateLargeCounter(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())
	reg := db.AssertExistsAndLoadBean(t, &U2FRegistration{ID: 1}).(*U2FRegistration)
	reg.Counter = 0xffffffff
	assert.NoError(t, reg.UpdateCounter())
	db.AssertExistsIf(t, true, &U2FRegistration{ID: 1, Counter: 0xffffffff})
}

func TestCreateRegistration(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	res, err := CreateRegistration(1, "U2F Created Key", &u2f.Registration{Raw: []byte("Test")})
	assert.NoError(t, err)
	assert.Equal(t, "U2F Created Key", res.Name)
	assert.Equal(t, []byte("Test"), res.Raw)

	db.AssertExistsIf(t, true, &U2FRegistration{Name: "U2F Created Key", UserID: 1})
}

func TestDeleteRegistration(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())
	reg := db.AssertExistsAndLoadBean(t, &U2FRegistration{ID: 1}).(*U2FRegistration)

	assert.NoError(t, DeleteRegistration(reg))
	db.AssertNotExistsBean(t, &U2FRegistration{ID: 1})
}
