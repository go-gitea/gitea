// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/modules/translation"

	"gitea.com/go-chi/binding"
	"github.com/stretchr/testify/assert"
)

type testRangeForm struct {
	FormDefaultValidator
	Hours int `binding:"Range(0,1000)"`
}

func TestBuildValidationErrorForUser(t *testing.T) {
	// an out-of-range value must reach its own message instead of the panicking "default" branch
	form := &testRangeForm{Hours: 2000}
	errs := binding.Validate(httptest.NewRequest(http.MethodPost, "/", nil), form)
	errorMessage, errorFieldName, fieldNames := BuildValidationErrorForUser(form, translation.MockLocale{}, errs)
	assert.Equal(t, "form.range_error:form.Hours,0,1000", errorMessage)
	assert.Equal(t, "Hours", errorFieldName)
	assert.Equal(t, []string{"Hours"}, fieldNames)
}
