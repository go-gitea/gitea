// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	validationTestCase struct {
		description    string
		data           any
		expectedErrors BindingErrors
	}

	TestForm struct {
		BranchName   string `form:"BranchName" binding:"GitRefName"`
		URL          string `form:"ValidUrl" binding:"ValidUrl"`
		GlobPattern  string `form:"GlobPattern" binding:"GlobPattern"`
		RegexPattern string `form:"RegexPattern" binding:"RegexPattern"`
	}
)

func performValidationTest(t *testing.T, testCase validationTestCase) {
	assert.Equal(t, testCase.expectedErrors, Binder().Validate(t.Context(), testCase.data))
}
