// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"testing"
)

func Test_ValidURLValidation(t *testing.T) {
	urlValidationTestCases := []validationTestCase{
		{
			description: "Empty URL",
			data: &TestForm{
				URL: "",
			},
		},
		{
			description: "URL without port",
			data: &TestForm{
				URL: "http://test.lan/",
			},
		},
		{
			description: "URL with port",
			data: &TestForm{
				URL: "http://test.lan:3000/",
			},
		},
		{
			description: "URL with IPv6 address without port",
			data: &TestForm{
				URL: "http://[::1]/",
			},
		},
		{
			description: "URL with IPv6 address with port",
			data: &TestForm{
				URL: "http://[::1]:3000/",
			},
		},
		{
			description: "Invalid URL",
			data: &TestForm{
				URL: "http//test.lan/",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"URL"},
					Classification: "UrlError",
					Message:        "Url",
				},
			},
		},
		{
			description: "Invalid schema",
			data: &TestForm{
				URL: "ftp://test.lan/",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"URL"},
					Classification: "UrlError",
					Message:        "Url",
				},
			},
		},
		{
			description: "Invalid port",
			data: &TestForm{
				URL: "http://test.lan:3x4/",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"URL"},
					Classification: "UrlError",
					Message:        "Url",
				},
			},
		},
		{
			description: "Invalid port with IPv6 address",
			data: &TestForm{
				URL: "http://[::1]:3x4/",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"URL"},
					Classification: "UrlError",
					Message:        "Url",
				},
			},
		},
	}

	for _, testCase := range urlValidationTestCases {
		t.Run(testCase.description, func(t *testing.T) {
			performValidationTest(t, testCase)
		})
	}
}
