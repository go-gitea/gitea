// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package login

import (
	"encoding/hex"
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

const validU2FRegistrationResponseHex = "0504b174bc49c7ca254b70d2e5c207cee9cf174820ebd77ea3c65508c26da51b657c1cc6b952f8621697936482da0a6d3d3826a59095daf6cd7c03e2e60385d2f6d9402a552dfdb7477ed65fd84133f86196010b2215b57da75d315b7b9e8fe2e3925a6019551bab61d16591659cbaf00b4950f7abfe6660e2e006f76868b772d70c253082013c3081e4a003020102020a47901280001155957352300a06082a8648ce3d0403023017311530130603550403130c476e756262792050696c6f74301e170d3132303831343138323933325a170d3133303831343138323933325a3031312f302d0603550403132650696c6f74476e756262792d302e342e312d34373930313238303030313135353935373335323059301306072a8648ce3d020106082a8648ce3d030107034200048d617e65c9508e64bcc5673ac82a6799da3c1446682c258c463fffdf58dfd2fa3e6c378b53d795c4a4dffb4199edd7862f23abaf0203b4b8911ba0569994e101300a06082a8648ce3d0403020347003044022060cdb6061e9c22262d1aac1d96d8c70829b2366531dda268832cb836bcd30dfa0220631b1459f09e6330055722c8d89b7f48883b9089b88d60d1d9795902b30410df304502201471899bcc3987e62e8202c9b39c33c19033f7340352dba80fcab017db9230e402210082677d673d891933ade6f617e5dbde2e247e70423fd5ad7804a6d3d3961ef871"

func TestToRegistrations_SkipInvalidItemsWithoutCrashing(t *testing.T) {
	regKeyRaw, _ := hex.DecodeString(validU2FRegistrationResponseHex)
	regs := U2FRegistrationList{
		&U2FRegistration{ID: 1},
		&U2FRegistration{ID: 2, Name: "U2F Key", UserID: 2, Counter: 0, Raw: regKeyRaw, CreatedUnix: 946684800, UpdatedUnix: 946684800},
	}

	actual := regs.ToRegistrations()
	assert.Len(t, actual, 1)
}

func TestToRegistrations(t *testing.T) {
	regKeyRaw, _ := hex.DecodeString(validU2FRegistrationResponseHex)
	regs := U2FRegistrationList{
		&U2FRegistration{ID: 1, Name: "U2F Key", UserID: 1, Counter: 0, Raw: regKeyRaw, CreatedUnix: 946684800, UpdatedUnix: 946684800},
		&U2FRegistration{ID: 2, Name: "U2F Key", UserID: 2, Counter: 0, Raw: regKeyRaw, CreatedUnix: 946684800, UpdatedUnix: 946684800},
	}

	actual := regs.ToRegistrations()
	assert.Len(t, actual, 2)
}
