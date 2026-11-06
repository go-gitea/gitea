// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"testing"
)

func Test_ValidURLValidation(t *testing.T) {
	expectedErrors := BindingErrors{{
		FieldNames:     []string{"URL"},
		Classification: "UrlError",
		Message:        "Url",
	}}
	for _, testCase := range []validationTestCase{
		{"Empty URL", &TestForm{URL: ""}, nil},
		{"URL without port", &TestForm{URL: "http://test.lan/"}, nil},
		{"URL with port", &TestForm{URL: "http://test.lan:3000/"}, nil},
		{"URL with IPv6 address without port", &TestForm{URL: "http://[::1]/"}, nil},
		{"URL with IPv6 address with port", &TestForm{URL: "http://[::1]:3000/"}, nil},
		{"Invalid URL", &TestForm{URL: "http//test.lan/"}, expectedErrors},
		{"Invalid schema", &TestForm{URL: "ftp://test.lan/"}, expectedErrors},
		{"Invalid port", &TestForm{URL: "http://test.lan:3x4/"}, expectedErrors},
		{"Invalid port with IPv6 address", &TestForm{URL: "http://[::1]:3x4/"}, expectedErrors},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			performValidationTest(t, testCase)
		})
	}
}
