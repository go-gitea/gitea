// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package validation

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-macaron/binding"
	"github.com/stretchr/testify/assert"
	"gopkg.in/macaron.v1"
)

const (
	testRoute = "/test"
)

type (
	validationTestCase struct {
		description    string
		data           interface{}
		expectedErrors binding.Errors
	}

	handlerFunc func(interface{}, ...interface{}) macaron.Handler

	modeler interface {
		Model() string
	}

	TestForm struct {
		BranchName string `form:"BranchName" binding:"GitRefName"`
		URL        string `form:"ValidUrl" binding:"ValidUrl"`
	}
)

func performValidationTest(t *testing.T, testCase validationTestCase) {
	httpRecorder := httptest.NewRecorder()
	m := macaron.Classic()

	m.Post(testRoute, binding.Validate(testCase.data), func(actual binding.Errors) {
		assert.Equal(t, fmt.Sprintf("%+v", testCase.expectedErrors), fmt.Sprintf("%+v", actual))
	})

	req, err := http.NewRequest("POST", testRoute, nil)
	if err != nil {
		panic(err)
	}

	m.ServeHTTP(httpRecorder, req)

	switch httpRecorder.Code {
	case http.StatusNotFound:
		panic("Routing is messed up in test fixture (got 404): check methods and paths")
	case http.StatusInternalServerError:
		panic("Something bad happened on '" + testCase.description + "'")
	}
}
