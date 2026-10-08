// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"testing"

	"gitea.dev/modules/glob"
)

func getGlobPatternErrorString(pattern string) string {
	// It would be unwise to rely on that glob
	// compilation errors don't ever change.
	if _, err := glob.Compile(pattern); err != nil {
		return err.Error()
	}
	return ""
}

func Test_GlobPatternValidation(t *testing.T) {
	globValidationTestCases := []validationTestCase{
		{
			description: "Empty glob pattern",
			data: &TestForm{
				GlobPattern: "",
			},
		},
		{
			description: "Valid glob",
			data: &TestForm{
				GlobPattern: "{master,release*}",
			},
		},

		{
			description: "Invalid glob",
			data: &TestForm{
				GlobPattern: "[a-",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"GlobPattern"},
					Classification: ErrGlobPattern,
					Message:        getGlobPatternErrorString("[a-"),
				},
			},
		},
	}

	for _, testCase := range globValidationTestCases {
		t.Run(testCase.description, func(t *testing.T) {
			performValidationTest(t, testCase)
		})
	}
}
