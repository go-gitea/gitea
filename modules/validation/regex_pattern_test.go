// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"regexp"
	"testing"
)

func getRegexPatternErrorString(pattern string) string {
	if _, err := regexp.Compile(pattern); err != nil {
		return err.Error()
	}
	return ""
}

func Test_RegexPatternValidation(t *testing.T) {
	regexValidationTestCases := []validationTestCase{
		{
			description: "Empty regex pattern",
			data: &TestForm{
				RegexPattern: "",
			},
		},
		{
			description: "Valid regex",
			data: &TestForm{
				RegexPattern: `(\d{1,3})+`,
			},
		},

		{
			description: "Invalid regex",
			data: &TestForm{
				RegexPattern: "[a-",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"RegexPattern"},
					Classification: ErrRegexPattern,
					Message:        getRegexPatternErrorString("[a-"),
				},
			},
		},
	}

	for _, testCase := range regexValidationTestCases {
		t.Run(testCase.description, func(t *testing.T) {
			performValidationTest(t, testCase)
		})
	}
}
