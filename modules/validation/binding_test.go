// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	validationTestCase struct {
		description    string
		data           any
		expectedErrors BindingErrors
	}

	TestForm struct {
		BranchName   string `form:"BranchName" binding:"GitRefName"`
		URL          string `form:"ValidUrl" binding:"ValidUrl"`
		GlobPattern  string `form:"GlobPattern" binding:"GlobPattern"`
		RegexPattern string `form:"RegexPattern" binding:"RegexPattern"`
		Email        string `form:"Email" binding:"Email"`
	}
)

func performValidationTest(t *testing.T, testCase validationTestCase) {
	assert.Equal(t, testCase.expectedErrors, Binder().Validate(t.Context(), testCase.data))
}

func TestEmailValidation(t *testing.T) {
	assert.Nil(t, Binder().Validate(t.Context(), &TestForm{Email: "b@a"}))
	assert.Equal(t, BindingErrors{{FieldNames: []string{"Email"}, Classification: "EmailError", Message: "invalid email"}},
		Binder().Validate(t.Context(), &TestForm{Email: "abc"}))
}

func TestBindingTagRulesRegistered(t *testing.T) {
	fileSet := token.NewFileSet()
	for _, dir := range []string{"../../services/forms", "../../modules/structs"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		require.NotEmpty(t, files)
		for _, file := range files {
			parsed, err := parser.ParseFile(fileSet, file, nil, parser.SkipObjectResolution)
			require.NoError(t, err)
			ast.Inspect(parsed, func(node ast.Node) bool {
				field, ok := node.(*ast.Field)
				if !ok || field.Tag == nil {
					return true
				}
				tag, err := strconv.Unquote(field.Tag.Value)
				require.NoError(t, err)
				rules := reflect.StructTag(tag).Get("binding")
				if rules == "" {
					return true
				}
				structType := reflect.StructOf([]reflect.StructField{{Name: "Field", Type: reflect.TypeFor[*string](), Tag: reflect.StructTag("binding:" + strconv.Quote(rules))}})
				assert.Empty(t, Binder().Validate(t.Context(), reflect.New(structType).Interface()), fileSet.Position(field.Pos()).String())
				return true
			})
		}
	}
}
