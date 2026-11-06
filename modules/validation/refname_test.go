// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"testing"
)

func Test_GitRefNameValidation(t *testing.T) {
	expectedErrors := BindingErrors{{
		FieldNames:     []string{"BranchName"},
		Classification: ErrGitRefName,
		Message:        "GitRefName",
	}}
	for _, testCase := range []validationTestCase{
		{"Reference name contains only characters", &TestForm{BranchName: "test"}, nil},
		{"Reference name contains single slash", &TestForm{BranchName: "feature/test"}, nil},
		{"Reference name has allowed special characters", &TestForm{BranchName: "debian/1%1.6.0-2"}, nil},
		{"Reference name contains backslash", &TestForm{BranchName: "feature\\test"}, expectedErrors},
		{"Reference name starts with dot", &TestForm{BranchName: ".test"}, expectedErrors},
		{"Reference name ends with dot", &TestForm{BranchName: "test."}, expectedErrors},
		{"Reference name starts with slash", &TestForm{BranchName: "/test"}, expectedErrors},
		{"Reference name ends with slash", &TestForm{BranchName: "test/"}, expectedErrors},
		{"Reference name ends with .lock", &TestForm{BranchName: "test.lock"}, expectedErrors},
		{"Reference name contains multiple consecutive dots", &TestForm{BranchName: "te..st"}, expectedErrors},
		{"Reference name contains multiple consecutive slashes", &TestForm{BranchName: "te//st"}, expectedErrors},
		{"Reference name is single @", &TestForm{BranchName: "@"}, expectedErrors},
		{"Reference name has @{", &TestForm{BranchName: "branch@{"}, expectedErrors},
		{"Reference name has unallowed special character ~", &TestForm{BranchName: "~debian/1%1.6.0-2"}, expectedErrors},
		{"Reference name has unallowed special character *", &TestForm{BranchName: "*debian/1%1.6.0-2"}, expectedErrors},
		{"Reference name has unallowed special character ?", &TestForm{BranchName: "?debian/1%1.6.0-2"}, expectedErrors},
		{"Reference name has unallowed special character ^", &TestForm{BranchName: "^debian/1%1.6.0-2"}, expectedErrors},
		{"Reference name has unallowed special character :", &TestForm{BranchName: "debian:jessie"}, expectedErrors},
		{"Reference name has unallowed special character (whitespace)", &TestForm{BranchName: "debian jessie"}, expectedErrors},
		{"Reference name has unallowed special character [", &TestForm{BranchName: "debian[jessie"}, expectedErrors},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			performValidationTest(t, testCase)
		})
	}
}
