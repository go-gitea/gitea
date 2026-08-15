// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gitea.dev/modules/translation"

	"gitea.com/go-chi/binding"
	"github.com/stretchr/testify/assert"
)

type testRangeForm struct {
	FormDefaultValidator
	Hours int `binding:"Range(0,1000)"`
}

type testTrimSpaceForm struct {
	FormDefaultValidator
	Addr    string `binding:"TrimSpace"`
	Name    string `binding:"TrimSpace;Required"`
	Content string
}

func TestTrimFormValues(t *testing.T) {
	body := url.Values{"addr": {" 127.0.0.1 "}, "name": {"  "}, "content": {" keep "}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	form := &testTrimSpaceForm{}
	TrimFormValues(req, form)
	errs := binding.Bind(req, form)

	assert.Equal(t, "127.0.0.1", form.Addr)
	assert.Equal(t, " keep ", form.Content)
	// trimming must happen before validation, so a whitespace-only value fails "Required"
	assert.True(t, errs.Has(binding.ERR_REQUIRED))
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
