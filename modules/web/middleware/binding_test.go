// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/translation"
	"gitea.dev/modules/validation"

	"github.com/stretchr/testify/assert"
)

type testBindingForm struct {
	FormDefaultValidator
	Hours int `binding:"Range(0,1000)"`
}

func TestBuildValidationErrorForUser(t *testing.T) {
	t.Run("Range", func(t *testing.T) {
		form := &testBindingForm{Hours: 2000}
		errs := validation.Binder().Validate(t.Context(), form)
		errorMessage, errorFieldName, fieldNames := BuildValidationErrorForUser(form, translation.MockLocale{}, errs)
		assert.Equal(t, "form.range_error:form.Hours,0,1000", errorMessage)
		assert.Equal(t, "Hours", errorFieldName)
		assert.Equal(t, []string{"Hours"}, fieldNames)
	})
	t.Run("TypeCast", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hours=abc"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		form := &testBindingForm{}
		errs := validation.Binder().Bind(req, form)
		errorMessage, errorFieldName, fieldNames := BuildValidationErrorForUser(form, translation.MockLocale{}, errs)
		assert.Equal(t, "form.field_invalid_message:form.Hours,Value could not be parsed as integer", errorMessage)
		assert.Equal(t, "Hours", errorFieldName)
		assert.Equal(t, []string{"Hours"}, fieldNames)
	})
}
