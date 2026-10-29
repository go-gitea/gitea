// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
		{"name", "1.0.0q", "", "", "0", true},
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
	for _, test := range []struct {
		user, channel, revision string
		method                  func(*RecipeReference) string
		expected                string
	}{
		{"", "", "", (*RecipeReference).RevisionOrDefault, DefaultRevision},
		{"", "", DefaultRevision, (*RecipeReference).RevisionOrDefault, DefaultRevision},
		{"", "", "Az09", (*RecipeReference).RevisionOrDefault, "Az09"},
		{"", "", "", (*RecipeReference).String, "name/1.0"},
		{"user", "channel", "", (*RecipeReference).String, "name/1.0@user/channel"},
		{"user", "channel", "Az09", (*RecipeReference).String, "name/1.0@user/channel#Az09"},
		{"", "", "", (*RecipeReference).LinkName, "name/1.0/_/_/0"},
		{"user", "channel", "", (*RecipeReference).LinkName, "name/1.0/user/channel/0"},
		{"user", "channel", "Az09", (*RecipeReference).LinkName, "name/1.0/user/channel/Az09"},
	} {
		rref, err := NewRecipeReference("name", "1.0", test.user, test.channel, test.revision)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, test.method(rref))
	}
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

	for _, test := range []struct {
		revision string
		method   func(*PackageReference) string
		expected string
	}{
		{"", (*PackageReference).RevisionOrDefault, DefaultRevision},
		{DefaultRevision, (*PackageReference).RevisionOrDefault, DefaultRevision},
		{"Az09", (*PackageReference).RevisionOrDefault, "Az09"},
		{"", (*PackageReference).LinkName, "ref/0"},
		{"Az09", (*PackageReference).LinkName, "ref/Az09"},
	} {
		pref, err := NewPackageReference(rref, "ref", test.revision)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, test.method(pref))
	}
}
