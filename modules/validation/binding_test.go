// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.com/go-chi/binding"
	chi "github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

const (
	testRoute = "/test"
)

type (
	validationTestCase struct {
		description    string
		data           any
		expectedErrors binding.Errors
	}

	TestForm struct {
		BranchName   string `form:"BranchName" binding:"GitRefName"`
		URL          string `form:"ValidUrl" binding:"ValidUrl"`
		URLs         string `form:"ValidUrls" binding:"ValidUrlList"`
		GlobPattern  string `form:"GlobPattern" binding:"GlobPattern"`
		RegexPattern string `form:"RegexPattern" binding:"RegexPattern"`
	}
)

func performValidationTest(t *testing.T, testCase validationTestCase) {
	httpRecorder := httptest.NewRecorder()
	m := chi.NewRouter()

	m.Post(testRoute, func(resp http.ResponseWriter, req *http.Request) {
		actual := binding.Validate(req, testCase.data)
		// see https://github.com/stretchr/testify/issues/435
		if actual == nil {
			actual = binding.Errors{}
		}

		assert.Equal(t, testCase.expectedErrors, actual)
	})

	req, err := http.NewRequest("POST", testRoute, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "x-www-form-urlencoded")
	m.ServeHTTP(httpRecorder, req)

	switch httpRecorder.Code {
	case http.StatusNotFound:
		panic("Routing is messed up in test fixture (got 404): check methods and paths")
	case http.StatusInternalServerError:
		panic("Something bad happened on '" + testCase.description + "'")
	}
}
