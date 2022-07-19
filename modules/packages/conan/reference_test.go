// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package conan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRecipeReference(t *testing.T) {
	cases := []struct {
		Name     string
		Version  string
		User     string
		Channel  string
		Revision string
		IsValid  bool
	}{
		{"", "", "", "", "", false},
		{"name", "", "", "", "", false},
		{"", "1.0", "", "", "", false},
		{"", "", "user", "", "", false},
		{"", "", "", "channel", "", false},
		{"", "", "", "", "0", false},
		{"name", "1.0", "", "", "", true},
		{"name", "1.0", "user", "", "", false},
		{"name", "1.0", "", "channel", "", false},
		{"name", "1.0", "user", "channel", "", true},
		{"name", "1.0", "_", "", "", true},
		{"name", "1.0", "", "_", "", true},
		{"name", "1.0", "_", "_", "", true},
		{"name", "1.0", "_", "_", "0", true},
		{"name", "1.0", "", "", "0", true},
		{"name", "1.0", "", "", "000000000000000000000000000000000000000000000000000000000000", false},
	}

	for i, c := range cases {
		rref, err := NewRecipeReference(c.Name, c.Version, c.User, c.Channel, c.Revision)
		if c.IsValid {
			assert.NoError(t, err, "case %d, should be invalid", i)
			assert.NotNil(t, rref, "case %d, should not be nil", i)
		} else {
			assert.Error(t, err, "case %d, should be valid", i)
		}
	}
}

func TestRecipeReferenceRevisionOrDefault(t *testing.T) {
	rref, err := NewRecipeReference("name", "1.0", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, DefaultRevision, rref.RevisionOrDefault())

	rref, err = NewRecipeReference("name", "1.0", "", "", DefaultRevision)
	assert.NoError(t, err)
	assert.Equal(t, DefaultRevision, rref.RevisionOrDefault())

	rref, err = NewRecipeReference("name", "1.0", "", "", "Az09")
	assert.NoError(t, err)
	assert.Equal(t, "Az09", rref.RevisionOrDefault())
}

func TestRecipeReferenceString(t *testing.T) {
	rref, err := NewRecipeReference("name", "1.0", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, "name/1.0", rref.String())

	rref, err = NewRecipeReference("name", "1.0", "user", "channel", "")
	assert.NoError(t, err)
	assert.Equal(t, "name/1.0@user/channel", rref.String())

	rref, err = NewRecipeReference("name", "1.0", "user", "channel", "Az09")
	assert.NoError(t, err)
	assert.Equal(t, "name/1.0@user/channel#Az09", rref.String())
}

func TestRecipeReferenceLinkName(t *testing.T) {
	rref, err := NewRecipeReference("name", "1.0", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, "name/1.0/_/_/0", rref.LinkName())

	rref, err = NewRecipeReference("name", "1.0", "user", "channel", "")
	assert.NoError(t, err)
	assert.Equal(t, "name/1.0/user/channel/0", rref.LinkName())

	rref, err = NewRecipeReference("name", "1.0", "user", "channel", "Az09")
	assert.NoError(t, err)
	assert.Equal(t, "name/1.0/user/channel/Az09", rref.LinkName())
}

func TestNewPackageReference(t *testing.T) {
	rref, _ := NewRecipeReference("name", "1.0", "", "", "")

	cases := []struct {
		Recipe    *RecipeReference
		Reference string
		Revision  string
		IsValid   bool
	}{
		{nil, "", "", false},
		{rref, "", "", false},
		{nil, "aZ09", "", false},
		{rref, "aZ09", "", true},
		{rref, "", "Az09", false},
		{rref, "aZ09", "Az09", true},
	}

	for i, c := range cases {
		pref, err := NewPackageReference(c.Recipe, c.Reference, c.Revision)
		if c.IsValid {
			assert.NoError(t, err, "case %d, should be invalid", i)
			assert.NotNil(t, pref, "case %d, should not be nil", i)
		} else {
			assert.Error(t, err, "case %d, should be valid", i)
		}
	}
}

func TestPackageReferenceRevisionOrDefault(t *testing.T) {
	rref, _ := NewRecipeReference("name", "1.0", "", "", "")

	pref, err := NewPackageReference(rref, "ref", "")
	assert.NoError(t, err)
	assert.Equal(t, DefaultRevision, pref.RevisionOrDefault())

	pref, err = NewPackageReference(rref, "ref", DefaultRevision)
	assert.NoError(t, err)
	assert.Equal(t, DefaultRevision, pref.RevisionOrDefault())

	pref, err = NewPackageReference(rref, "ref", "Az09")
	assert.NoError(t, err)
	assert.Equal(t, "Az09", pref.RevisionOrDefault())
}

func TestPackageReferenceLinkName(t *testing.T) {
	rref, _ := NewRecipeReference("name", "1.0", "", "", "")

	pref, err := NewPackageReference(rref, "ref", "")
	assert.NoError(t, err)
	assert.Equal(t, "ref/0", pref.LinkName())

	pref, err = NewPackageReference(rref, "ref", "Az09")
	assert.NoError(t, err)
	assert.Equal(t, "ref/Az09", pref.LinkName())
}
