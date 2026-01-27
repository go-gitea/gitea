// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"testing"

	"gitea.com/go-chi/binding"
)

func Test_ValidURLListValidation(t *testing.T) {
	AddBindingRules()

	// This is a copy of all the URL tests cases, plus additional ones to
	// account for multiple URLs
	urlListValidationTestCases := []validationTestCase{
		{
			description: "Empty URL",
			data: TestForm{
				URLs: "",
			},
			expectedErrors: binding.Errors{},
		},
		{
			description: "URL without port",
			data: TestForm{
				URLs: "http://test.lan/",
			},
			expectedErrors: binding.Errors{},
		},
		{
			description: "URL with port",
			data: TestForm{
				URLs: "http://test.lan:3000/",
			},
			expectedErrors: binding.Errors{},
		},
		{
			description: "URL with IPv6 address without port",
			data: TestForm{
				URLs: "http://[::1]/",
			},
			expectedErrors: binding.Errors{},
		},
		{
			description: "URL with IPv6 address with port",
			data: TestForm{
				URLs: "http://[::1]:3000/",
			},
			expectedErrors: binding.Errors{},
		},
		{
			description: "Invalid URL",
			data: TestForm{
				URLs: "http//test.lan/",
			},
			expectedErrors: binding.Errors{
				binding.Error{
					FieldNames:     []string{"URLs"},
					Classification: binding.ERR_URL,
					Message:        "http//test.lan/",
				},
			},
		},
		{
			description: "Invalid schema",
			data: TestForm{
				URLs: "ftp://test.lan/",
			},
			expectedErrors: binding.Errors{
				binding.Error{
					FieldNames:     []string{"URLs"},
					Classification: binding.ERR_URL,
					Message:        "ftp://test.lan/",
				},
			},
		},
		{
			description: "Invalid port",
			data: TestForm{
				URLs: "http://test.lan:3x4/",
			},
			expectedErrors: binding.Errors{
				binding.Error{
					FieldNames:     []string{"URLs"},
					Classification: binding.ERR_URL,
					Message:        "http://test.lan:3x4/",
				},
			},
		},
		{
			description: "Invalid port with IPv6 address",
			data: TestForm{
				URLs: "http://[::1]:3x4/",
			},
			expectedErrors: binding.Errors{
				binding.Error{
					FieldNames:     []string{"URLs"},
					Classification: binding.ERR_URL,
					Message:        "http://[::1]:3x4/",
				},
			},
		},
		{
			description: "Multi URLs",
			data: TestForm{
				URLs: "http://test.lan:3000/\nhttp://test.local/",
			},
			expectedErrors: binding.Errors{},
		},
		{
			description: "Multi URLs with newline",
			data: TestForm{
				URLs: "http://test.lan:3000/\nhttp://test.local/\n",
			},
			expectedErrors: binding.Errors{},
		},
		{
			description: "List with invalid entry",
			data: TestForm{
				URLs: "http://test.lan:3000/\nhttp://[::1]:3x4/",
			},
			expectedErrors: binding.Errors{
				binding.Error{
					FieldNames:     []string{"URLs"},
					Classification: binding.ERR_URL,
					Message:        "http://[::1]:3x4/",
				},
			},
		},
		{
			description: "List with two invalid entries",
			data: TestForm{
				URLs: "ftp://test.lan:3000/\nhttp://[::1]:3x4/\n",
			},
			expectedErrors: binding.Errors{
				binding.Error{
					FieldNames:     []string{"URLs"},
					Classification: binding.ERR_URL,
					Message:        "ftp://test.lan:3000/",
				},
				binding.Error{
					FieldNames:     []string{"URLs"},
					Classification: binding.ERR_URL,
					Message:        "http://[::1]:3x4/",
				},
			},
		},
	}

	for _, testCase := range urlListValidationTestCases {
		t.Run(testCase.description, func(t *testing.T) {
			performValidationTest(t, testCase)
		})
	}
}
