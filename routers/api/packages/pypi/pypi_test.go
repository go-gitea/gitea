// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pypi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidNameAndVersion(t *testing.T) {
	// The test cases below were created from the following Python PEPs:
	// https://peps.python.org/pep-0426/#name
	// https://peps.python.org/pep-0440/#appendix-b-parsing-version-strings-with-regular-expressions

	// Valid Cases
	assert.True(t, isValidNameAndVersion("A", "1.0.1"))
	assert.True(t, isValidNameAndVersion("Test.Name.1234", "1.0.1"))
	assert.True(t, isValidNameAndVersion("test_name", "1.0.1"))
	assert.True(t, isValidNameAndVersion("test-name", "1.0.1"))
	assert.True(t, isValidNameAndVersion("test-name", "v1.0.1"))
	assert.True(t, isValidNameAndVersion("test-name", "2012.4"))
	assert.True(t, isValidNameAndVersion("test-name", "1.0.1-alpha"))
	assert.True(t, isValidNameAndVersion("test-name", "1.0.1a1"))
	assert.True(t, isValidNameAndVersion("test-name", "1.0b2.r345.dev456"))
	assert.True(t, isValidNameAndVersion("test-name", "1!1.0.1"))
	assert.True(t, isValidNameAndVersion("test-name", "1.0.1+local.1"))

	// Invalid Cases
	assert.False(t, isValidNameAndVersion(".test-name", "1.0.1"))
	assert.False(t, isValidNameAndVersion("test!name", "1.0.1"))
	assert.False(t, isValidNameAndVersion("-test-name", "1.0.1"))
	assert.False(t, isValidNameAndVersion("test-name-", "1.0.1"))
	assert.False(t, isValidNameAndVersion("test-name", "a1.0.1"))
	assert.False(t, isValidNameAndVersion("test-name", "1.0.1aa"))
	assert.False(t, isValidNameAndVersion("test-name", "1.0.0-alpha.beta"))
}

func TestNormalizeLabel(t *testing.T) {
	// Cases fetched from https://packaging.python.org/en/latest/specifications/well-known-project-urls/#label-normalization.
	assert.Equal(t, "homepage", normalizeLabel("Homepage"))
	assert.Equal(t, "homepage", normalizeLabel("Home-page"))
	assert.Equal(t, "homepage", normalizeLabel("Home page"))
	assert.Equal(t, "changelog", normalizeLabel("Change_Log"))
	assert.Equal(t, "whatsnew", normalizeLabel("What's New?"))
	assert.Equal(t, "github", normalizeLabel("github"))
}
