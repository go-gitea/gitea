// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pypi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasInvalidMetadata(t *testing.T) {
	// The test cases below were created from the following Python PEPs:
	// https://peps.python.org/pep-0426/#name
	// https://peps.python.org/pep-0440/#appendix-b-parsing-version-strings-with-regular-expressions

	// Valid Cases
	assert.False(t, hasInvalidMetadata("A", "1.0.1"))
	assert.False(t, hasInvalidMetadata("Test.Name.1234", "1.0.1"))
	assert.False(t, hasInvalidMetadata("test_name", "1.0.1"))
	assert.False(t, hasInvalidMetadata("test-name", "1.0.1"))
	assert.False(t, hasInvalidMetadata("test-name", "v1.0.1"))
	assert.False(t, hasInvalidMetadata("test-name", "2012.4"))
	assert.False(t, hasInvalidMetadata("test-name", "1.0.1-alpha"))
	assert.False(t, hasInvalidMetadata("test-name", "1.0.1a1"))
	assert.False(t, hasInvalidMetadata("test-name", "1.0b2.r345.dev456"))
	assert.False(t, hasInvalidMetadata("test-name", "1!1.0.1"))
	assert.False(t, hasInvalidMetadata("test-name", "1.0.1+local.1"))

	// Invalid Cases
	assert.True(t, hasInvalidMetadata(".test-name", "1.0.1"))
	assert.True(t, hasInvalidMetadata("test!name", "1.0.1"))
	assert.True(t, hasInvalidMetadata("test-name", "a1.0.1"))
	assert.True(t, hasInvalidMetadata("test-name", "1.0.1aa"))
	assert.True(t, hasInvalidMetadata("test-name", "1.0.0-alpha.beta"))
}
