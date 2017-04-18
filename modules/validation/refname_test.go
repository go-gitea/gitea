// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package validation

import (
	"testing"

	"github.com/go-macaron/binding"
)

var gitRefNameValidationTestCases = []validationTestCase{
	{
		description: "Referece contains only characters",
		data: TestForm{
			BranchName: "test",
		},
		expectedErrors: binding.Errors{},
	},
	{
		description: "Reference name contains single slash",
		data: TestForm{
			BranchName: "feature/test",
		},
		expectedErrors: binding.Errors{},
	},
	{
		description: "Reference name contains backslash",
		data: TestForm{
			BranchName: "feature\\test",
		},
		expectedErrors: binding.Errors{
			binding.Error{
				FieldNames:     []string{"BranchName"},
				Classification: ErrGitRefName,
				Message:        "GitRefName",
			},
		},
	},
	{
		description: "Reference name starts with dot",
		data: TestForm{
			BranchName: ".test",
		},
		expectedErrors: binding.Errors{
			binding.Error{
				FieldNames:     []string{"BranchName"},
				Classification: ErrGitRefName,
				Message:        "GitRefName",
			},
		},
	},
	{
		description: "Reference name ends with dot",
		data: TestForm{
			BranchName: "test.",
		},
		expectedErrors: binding.Errors{
			binding.Error{
				FieldNames:     []string{"BranchName"},
				Classification: ErrGitRefName,
				Message:        "GitRefName",
			},
		},
	},
	{
		description: "Reference name starts with slash",
		data: TestForm{
			BranchName: "/test",
		},
		expectedErrors: binding.Errors{
			binding.Error{
				FieldNames:     []string{"BranchName"},
				Classification: ErrGitRefName,
				Message:        "GitRefName",
			},
		},
	},
	{
		description: "Reference name ends with slash",
		data: TestForm{
			BranchName: "test/",
		},
		expectedErrors: binding.Errors{
			binding.Error{
				FieldNames:     []string{"BranchName"},
				Classification: ErrGitRefName,
				Message:        "GitRefName",
			},
		},
	},
	{
		description: "Reference name ends with .lock",
		data: TestForm{
			BranchName: "test.lock",
		},
		expectedErrors: binding.Errors{
			binding.Error{
				FieldNames:     []string{"BranchName"},
				Classification: ErrGitRefName,
				Message:        "GitRefName",
			},
		},
	},
	{
		description: "Reference name contains multiple consecutive dots",
		data: TestForm{
			BranchName: "te..st",
		},
		expectedErrors: binding.Errors{
			binding.Error{
				FieldNames:     []string{"BranchName"},
				Classification: ErrGitRefName,
				Message:        "GitRefName",
			},
		},
	},
	{
		description: "Reference name contains multiple consecutive slashes",
		data: TestForm{
			BranchName: "te//st",
		},
		expectedErrors: binding.Errors{
			binding.Error{
				FieldNames:     []string{"BranchName"},
				Classification: ErrGitRefName,
				Message:        "GitRefName",
			},
		},
	},
}

func Test_GitRefNameValidation(t *testing.T) {
	AddBindingRules()

	for _, testCase := range gitRefNameValidationTestCases {
		t.Run(testCase.description, func(t *testing.T) {
			performValidationTest(t, testCase)
		})
	}
}
