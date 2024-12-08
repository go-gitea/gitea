// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"html/template"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestSubjectBodySeparator(t *testing.T) {
	test := func(input, subject, body string) {
		loc := mailSubjectSplit.FindIndex([]byte(input))
		if loc == nil {
			assert.Empty(t, subject, "no subject found, but one expected")
			assert.Equal(t, body, input)
		} else {
			assert.Equal(t, subject, input[0:loc[0]])
			assert.Equal(t, body, input[loc[1]:])
		}
	}

	test("Simple\n---------------\nCase",
		"Simple\n",
		"\nCase")
	test("Only\nBody",
		"",
		"Only\nBody")
	test("Minimal\n---\nseparator",
		"Minimal\n",
		"\nseparator")
	test("False --- separator",
		"",
		"False --- separator")
	test("False\n--- separator",
		"",
		"False\n--- separator")
	test("False ---\nseparator",
		"",
		"False ---\nseparator")
	test("With extra spaces\n-----   \t   \nBody",
		"With extra spaces\n",
		"\nBody")
	test("With leading spaces\n   -------\nOnly body",
		"",
		"With leading spaces\n   -------\nOnly body")
	test("Multiple\n---\n-------\n---\nSeparators",
		"Multiple\n",
		"\n-------\n---\nSeparators")
	test("Insufficient\n--\nSeparators",
		"",
		"Insufficient\n--\nSeparators")
}

func TestJSEscapeSafe(t *testing.T) {
	assert.EqualValues(t, `\u0026\u003C\u003E\'\"`, jsEscapeSafe(`&<>'"`))
}

func TestSanitizeHTML(t *testing.T) {
	assert.Equal(t, template.HTML(`<a href="/" rel="nofollow">link</a> xss <div>inline</div>`), SanitizeHTML(`<a href="/">link</a> <a href="javascript:">xss</a> <div style="dangerous">inline</div>`))
}

func TestTemplateTruthy(t *testing.T) {
	tmpl := template.New("test")
	tmpl.Funcs(template.FuncMap{"Iif": iif})
	template.Must(tmpl.Parse(`{{if .Value}}true{{else}}false{{end}}:{{Iif .Value "true" "false"}}`))

	cases := []any{
		nil, false, true, "", "string", 0, 1,
		byte(0), byte(1), int64(0), int64(1), float64(0), float64(1),
		complex(0, 0), complex(1, 0),
		(chan int)(nil), make(chan int),
		(func())(nil), func() {},
		util.ToPointer(0), util.ToPointer(util.ToPointer(0)),
		util.ToPointer(1), util.ToPointer(util.ToPointer(1)),
		[0]int{},
		[1]int{0},
		[]int(nil),
		[]int{},
		[]int{0},
		map[any]any(nil),
		map[any]any{},
		map[any]any{"k": "v"},
		(*struct{})(nil),
		struct{}{},
		util.ToPointer(struct{}{}),
	}
	w := &strings.Builder{}
	truthyCount := 0
	for i, v := range cases {
		w.Reset()
		assert.NoError(t, tmpl.Execute(w, struct{ Value any }{v}), "case %d (%T) %#v fails", i, v, v)
		out := w.String()
		truthyCount += util.Iif(out == "true:true", 1, 0)
		truthyMatches := out == "true:true" || out == "false:false"
		assert.True(t, truthyMatches, "case %d (%T) %#v fail: %s", i, v, v, out)
	}
	assert.True(t, truthyCount != 0 && truthyCount != len(cases))
}

func TestQueryBuild(t *testing.T) {
	t.Run("construct", func(t *testing.T) {
		assert.Equal(t, "", string(queryBuild()))
		assert.Equal(t, "a=1&b=true", string(queryBuild("a", 1, "b", "true")))
		assert.Equal(t, "?k=1", string(queryBuild("?", "k", 1)))
		assert.Equal(t, "?a=b&k=1", string(queryBuild("?a=b", "k", 1)))
		assert.Equal(t, "&k=1", string(queryBuild("&", "k", 1)))
		assert.Equal(t, "&a=b&k=1", string(queryBuild("&a=b", "k", 1)))
	})
	t.Run("replace", func(t *testing.T) {
		assert.Equal(t, "a=1&c=d&e=f", string(queryBuild(QueryString("a=b&c=d&e=f"), "a", 1)))
		assert.Equal(t, "a=b&c=1&e=f", string(queryBuild(QueryString("a=b&c=d&e=f"), "c", 1)))
		assert.Equal(t, "a=b&c=d&e=1", string(queryBuild(QueryString("a=b&c=d&e=f"), "e", 1)))
		assert.Equal(t, "a=b&c=d&e=f&k=1", string(queryBuild(QueryString("a=b&c=d&e=f"), "k", 1)))
	})
	t.Run("replace-?", func(t *testing.T) {
		assert.Equal(t, "?a=1&c=d&e=f", string(queryBuild(QueryString("?a=b&c=d&e=f"), "a", 1)))
		assert.Equal(t, "?a=b&c=1&e=f", string(queryBuild(QueryString("?a=b&c=d&e=f"), "c", 1)))
		assert.Equal(t, "?a=b&c=d&e=1", string(queryBuild(QueryString("?a=b&c=d&e=f"), "e", 1)))
		assert.Equal(t, "?a=b&c=d&e=f&k=1", string(queryBuild(QueryString("?a=b&c=d&e=f"), "k", 1)))
	})
	t.Run("replace-&", func(t *testing.T) {
		assert.Equal(t, "&a=1&c=d&e=f", string(queryBuild(QueryString("&a=b&c=d&e=f"), "a", 1)))
		assert.Equal(t, "&a=b&c=1&e=f", string(queryBuild(QueryString("&a=b&c=d&e=f"), "c", 1)))
		assert.Equal(t, "&a=b&c=d&e=1", string(queryBuild(QueryString("&a=b&c=d&e=f"), "e", 1)))
		assert.Equal(t, "&a=b&c=d&e=f&k=1", string(queryBuild(QueryString("&a=b&c=d&e=f"), "k", 1)))
	})
	t.Run("delete", func(t *testing.T) {
		assert.Equal(t, "c=d&e=f", string(queryBuild(QueryString("a=b&c=d&e=f"), "a", "")))
		assert.Equal(t, "a=b&e=f", string(queryBuild(QueryString("a=b&c=d&e=f"), "c", "")))
		assert.Equal(t, "a=b&c=d", string(queryBuild(QueryString("a=b&c=d&e=f"), "e", "")))
		assert.Equal(t, "a=b&c=d&e=f", string(queryBuild(QueryString("a=b&c=d&e=f"), "k", "")))
	})
	t.Run("delete-?", func(t *testing.T) {
		assert.Equal(t, "?c=d&e=f", string(queryBuild(QueryString("?a=b&c=d&e=f"), "a", "")))
		assert.Equal(t, "?a=b&e=f", string(queryBuild(QueryString("?a=b&c=d&e=f"), "c", "")))
		assert.Equal(t, "?a=b&c=d", string(queryBuild(QueryString("?a=b&c=d&e=f"), "e", "")))
		assert.Equal(t, "?a=b&c=d&e=f", string(queryBuild(QueryString("?a=b&c=d&e=f"), "k", "")))
	})
	t.Run("delete-&", func(t *testing.T) {
		assert.Equal(t, "&c=d&e=f", string(queryBuild(QueryString("&a=b&c=d&e=f"), "a", "")))
		assert.Equal(t, "&a=b&e=f", string(queryBuild(QueryString("&a=b&c=d&e=f"), "c", "")))
		assert.Equal(t, "&a=b&c=d", string(queryBuild(QueryString("&a=b&c=d&e=f"), "e", "")))
		assert.Equal(t, "&a=b&c=d&e=f", string(queryBuild(QueryString("&a=b&c=d&e=f"), "k", "")))
	})
}
