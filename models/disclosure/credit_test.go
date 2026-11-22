// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package disclosure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreditListJSONRoundTrip(t *testing.T) {
	list := CreditList{
		{Role: CreditRoleReporter, UserID: 2, Name: "User Two"},
		{Role: CreditRoleFinder, Name: "Jane Doe", URL: "https://jane.example.com"},
	}
	bs, err := list.ToDB()
	require.NoError(t, err)
	var back CreditList
	require.NoError(t, back.FromDB(bs))
	assert.Equal(t, list, back)

	var empty CreditList
	bs, err = empty.ToDB()
	require.NoError(t, err)
	assert.Equal(t, "[]", string(bs))
	require.NoError(t, empty.FromDB(nil))
	assert.Nil(t, empty)
}

func TestParseCreditRole(t *testing.T) {
	for _, r := range AllCreditRoles {
		parsed, err := ParseCreditRole(string(r))
		assert.NoError(t, err)
		assert.Equal(t, r, parsed)
	}
	_, err := ParseCreditRole("bogus")
	assert.Error(t, err)
}

func TestCreditValidate(t *testing.T) {
	// local user credit
	c := &Credit{Role: CreditRoleAnalyst, UserID: 2, Name: "User Two"}
	assert.NoError(t, c.Validate())
	assert.Empty(t, c.URL) // URL is dropped for local users

	// external with valid URL
	c = &Credit{Role: CreditRoleFinder, Name: "Jane Doe", URL: "https://jane.example.com"}
	assert.NoError(t, c.Validate())

	// external without URL
	c = &Credit{Role: CreditRoleSponsor, Name: "Jane Doe"}
	assert.NoError(t, c.Validate())

	// missing name
	assert.Error(t, (&Credit{Role: CreditRoleOther}).Validate())

	// bad role
	assert.Error(t, (&Credit{Role: "bogus", Name: "x"}).Validate())

	// invalid URLs
	assert.Error(t, (&Credit{Role: CreditRoleOther, Name: "x", URL: "notaurl"}).Validate())
	assert.Error(t, (&Credit{Role: CreditRoleOther, Name: "x", URL: "ftp://example.com"}).Validate())

	// too long name
	assert.Error(t, (&Credit{Role: CreditRoleOther, Name: string(make([]byte, 300))}).Validate())
}

func TestValidateCredits(t *testing.T) {
	assert.NoError(t, ValidateCredits(nil))
	assert.NoError(t, ValidateCredits(CreditList{{Role: CreditRoleReporter, UserID: 2, Name: "u"}}))
	tooMany := make(CreditList, CreditsMaxCount+1)
	for i := range tooMany {
		tooMany[i] = &Credit{Role: CreditRoleOther, Name: "x"}
	}
	assert.Error(t, ValidateCredits(tooMany))
}
