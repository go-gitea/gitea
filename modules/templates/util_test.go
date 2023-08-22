// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDict(t *testing.T) {
	type M map[string]any
	cases := []struct {
		args []any
		want map[string]any
	}{
		{[]any{"a", 1, "b", 2}, M{"a": 1, "b": 2}},
		{[]any{".", M{"base": 1}, "b", 2}, M{"base": 1, "b": 2}},
		{[]any{"a", 1, ".", M{"extra": 2}}, M{"a": 1, "extra": 2}},
		{[]any{"a", 1, ".", map[string]int{"int": 2}}, M{"a": 1, "int": 2}},
		{[]any{".", nil, "b", 2}, M{"b": 2}},
	}

	for _, c := range cases {
		got, err := dict(c.args...)
		if assert.NoError(t, err) {
			assert.EqualValues(t, c.want, got)
		}
	}

	bads := []struct {
		args []any
	}{
		{[]any{"a", 1, "b"}},
		{[]any{1}},
		{[]any{struct{}{}}},
	}
	for _, c := range bads {
		_, err := dict(c.args...)
		assert.Error(t, err)
	}
}

func TestUtils(t *testing.T) {
	execTmpl := func(code string, data any) string {
		tmpl := template.New("test")
		tmpl.Funcs(template.FuncMap{"SliceUtils": NewSliceUtils, "StringUtils": NewStringUtils})
		template.Must(tmpl.Parse(code))
		w := &strings.Builder{}
		assert.NoError(t, tmpl.Execute(w, data))
		return w.String()
	}

	actual := execTmpl("{{SliceUtils.Contains .Slice .Value}}", map[string]any{"Slice": []string{"a", "b"}, "Value": "a"})
	assert.Equal(t, "true", actual)

	actual = execTmpl("{{SliceUtils.Contains .Slice .Value}}", map[string]any{"Slice": []string{"a", "b"}, "Value": "x"})
	assert.Equal(t, "false", actual)

	actual = execTmpl("{{SliceUtils.Contains .Slice .Value}}", map[string]any{"Slice": []int64{1, 2}, "Value": int64(2)})
	assert.Equal(t, "true", actual)

	actual = execTmpl("{{StringUtils.Contains .String .Value}}", map[string]any{"String": "abc", "Value": "b"})
	assert.Equal(t, "true", actual)

	actual = execTmpl("{{StringUtils.Contains .String .Value}}", map[string]any{"String": "abc", "Value": "x"})
	assert.Equal(t, "false", actual)

	tmpl := template.New("test")
	tmpl.Funcs(template.FuncMap{"SliceUtils": NewSliceUtils, "StringUtils": NewStringUtils})
	template.Must(tmpl.Parse("{{SliceUtils.Contains .Slice .Value}}"))
	// error is like this: `template: test:1:12: executing "test" at <SliceUtils.Contains>: error calling Contains: ...`
	err := tmpl.Execute(io.Discard, map[string]any{"Slice": struct{}{}})
	assert.ErrorContains(t, err, "invalid type, expected slice or array")
}
