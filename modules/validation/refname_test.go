// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"testing"
)

func Test_GitRefNameValidation(t *testing.T) {
	gitRefNameValidationTestCases := []validationTestCase{
		{
			description: "Reference name contains only characters",
			data: &TestForm{
				BranchName: "test",
			},
		},
		{
			description: "Reference name contains single slash",
			data: &TestForm{
				BranchName: "feature/test",
			},
		},
		{
			description: "Reference name has allowed special characters",
			data: &TestForm{
				BranchName: "debian/1%1.6.0-2",
			},
		},
		{
			description: "Reference name contains backslash",
			data: &TestForm{
				BranchName: "feature\\test",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name starts with dot",
			data: &TestForm{
				BranchName: ".test",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name ends with dot",
			data: &TestForm{
				BranchName: "test.",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name starts with slash",
			data: &TestForm{
				BranchName: "/test",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name ends with slash",
			data: &TestForm{
				BranchName: "test/",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name ends with .lock",
			data: &TestForm{
				BranchName: "test.lock",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name contains multiple consecutive dots",
			data: &TestForm{
				BranchName: "te..st",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name contains multiple consecutive slashes",
			data: &TestForm{
				BranchName: "te//st",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name is single @",
			data: &TestForm{
				BranchName: "@",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name has @{",
			data: &TestForm{
				BranchName: "branch@{",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name has unallowed special character ~",
			data: &TestForm{
				BranchName: "~debian/1%1.6.0-2",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name has unallowed special character *",
			data: &TestForm{
				BranchName: "*debian/1%1.6.0-2",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name has unallowed special character ?",
			data: &TestForm{
				BranchName: "?debian/1%1.6.0-2",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name has unallowed special character ^",
			data: &TestForm{
				BranchName: "^debian/1%1.6.0-2",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name has unallowed special character :",
			data: &TestForm{
				BranchName: "debian:jessie",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name has unallowed special character (whitespace)",
			data: &TestForm{
				BranchName: "debian jessie",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
		{
			description: "Reference name has unallowed special character [",
			data: &TestForm{
				BranchName: "debian[jessie",
			},
			expectedErrors: BindingErrors{
				BindingError{
					FieldNames:     []string{"BranchName"},
					Classification: ErrGitRefName,
					Message:        "GitRefName",
				},
			},
		},
	}

	for _, testCase := range gitRefNameValidationTestCases {
		t.Run(testCase.description, func(t *testing.T) {
			performValidationTest(t, testCase)
		})
	}
}
