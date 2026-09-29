// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"testing"

	"gitea.dev/modules/translation"
	"gitea.dev/modules/validation"

	"github.com/stretchr/testify/assert"
)

type testRangeForm struct {
	FormDefaultValidator
	Hours int `binding:"Range(0,1000)"`
}

func TestBuildValidationErrorForUser(t *testing.T) {
	// an out-of-range value must reach its own message instead of the panicking "default" branch
	form := &testRangeForm{Hours: 2000}
	errs := validation.Binder().Validate(t.Context(), form)
	errorMessage, errorFieldName, fieldNames := BuildValidationErrorForUser(form, translation.MockLocale{}, errs)
	assert.Equal(t, "form.range_error:form.Hours,0,1000", errorMessage)
	assert.Equal(t, "Hours", errorFieldName)
	assert.Equal(t, []string{"Hours"}, fieldNames)
}
