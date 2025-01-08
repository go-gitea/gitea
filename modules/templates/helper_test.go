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

func TestTemplateIif(t *testing.T) {
	tmpl := template.New("test")
	tmpl.Funcs(template.FuncMap{"Iif": iif})
	template.Must(tmpl.Parse(`{{if .Value}}true{{else}}false{{end}}:{{Iif .Value "true" "false"}}`))

	cases := []any{nil, false, true, "", "string", 0, 1}
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

func TestTemplateEscape(t *testing.T) {
	execTmpl := func(code string) string {
		tmpl := template.New("test")
		tmpl.Funcs(template.FuncMap{"QueryBuild": QueryBuild, "HTMLFormat": htmlFormat})
		template.Must(tmpl.Parse(code))
		w := &strings.Builder{}
		assert.NoError(t, tmpl.Execute(w, nil))
		return w.String()
	}

	t.Run("Golang URL Escape", func(t *testing.T) {
		// Golang template considers "href", "*src*", "*uri*", "*url*" (and more) ... attributes as contentTypeURL and does auto-escaping
		actual := execTmpl(`<a href="?a={{"%"}}"></a>`)
		assert.Equal(t, `<a href="?a=%25"></a>`, actual)
		actual = execTmpl(`<a data-xxx-url="?a={{"%"}}"></a>`)
		assert.Equal(t, `<a data-xxx-url="?a=%25"></a>`, actual)
	})
	t.Run("Golang URL No-escape", func(t *testing.T) {
		// non-URL content isn't auto-escaped
		actual := execTmpl(`<a data-link="?a={{"%"}}"></a>`)
		assert.Equal(t, `<a data-link="?a=%"></a>`, actual)
	})
	t.Run("QueryBuild", func(t *testing.T) {
		actual := execTmpl(`<a href="{{QueryBuild "?" "a" "%"}}"></a>`)
		assert.Equal(t, `<a href="?a=%25"></a>`, actual)
		actual = execTmpl(`<a href="?{{QueryBuild "a" "%"}}"></a>`)
		assert.Equal(t, `<a href="?a=%25"></a>`, actual)
	})
	t.Run("HTMLFormat", func(t *testing.T) {
		actual := execTmpl("{{HTMLFormat `<a k=\"%s\">%s</a>` `\"` `<>`}}")
		assert.Equal(t, `<a k="&#34;">&lt;&gt;</a>`, actual)
	})
}

func TestQueryBuild(t *testing.T) {
	t.Run("construct", func(t *testing.T) {
		assert.Equal(t, "", string(QueryBuild()))
		assert.Equal(t, "", string(QueryBuild("a", nil, "b", false, "c", 0, "d", "")))
		assert.Equal(t, "a=1&b=true", string(QueryBuild("a", 1, "b", "true")))

		// path with query parameters
		assert.Equal(t, "/?k=1", string(QueryBuild("/", "k", 1)))
		assert.Equal(t, "/", string(QueryBuild("/?k=a", "k", 0)))

		// no path but question mark with query parameters
		assert.Equal(t, "?k=1", string(QueryBuild("?", "k", 1)))
		assert.Equal(t, "?", string(QueryBuild("?", "k", 0)))
		assert.Equal(t, "path?k=1", string(QueryBuild("path?", "k", 1)))
		assert.Equal(t, "path", string(QueryBuild("path?", "k", 0)))

		// only query parameters
		assert.Equal(t, "&k=1", string(QueryBuild("&", "k", 1)))
		assert.Equal(t, "", string(QueryBuild("&", "k", 0)))
		assert.Equal(t, "", string(QueryBuild("&k=a", "k", 0)))
		assert.Equal(t, "", string(QueryBuild("k=a&", "k", 0)))
		assert.Equal(t, "a=1&b=2", string(QueryBuild("a=1", "b", 2)))
		assert.Equal(t, "&a=1&b=2", string(QueryBuild("&a=1", "b", 2)))
		assert.Equal(t, "a=1&b=2&", string(QueryBuild("a=1&", "b", 2)))
	})

	t.Run("replace", func(t *testing.T) {
		assert.Equal(t, "a=1&c=d&e=f", string(QueryBuild("a=b&c=d&e=f", "a", 1)))
		assert.Equal(t, "a=b&c=1&e=f", string(QueryBuild("a=b&c=d&e=f", "c", 1)))
		assert.Equal(t, "a=b&c=d&e=1", string(QueryBuild("a=b&c=d&e=f", "e", 1)))
		assert.Equal(t, "a=b&c=d&e=f&k=1", string(QueryBuild("a=b&c=d&e=f", "k", 1)))
	})

	t.Run("replace-&", func(t *testing.T) {
		assert.Equal(t, "&a=1&c=d&e=f", string(QueryBuild("&a=b&c=d&e=f", "a", 1)))
		assert.Equal(t, "&a=b&c=1&e=f", string(QueryBuild("&a=b&c=d&e=f", "c", 1)))
		assert.Equal(t, "&a=b&c=d&e=1", string(QueryBuild("&a=b&c=d&e=f", "e", 1)))
		assert.Equal(t, "&a=b&c=d&e=f&k=1", string(QueryBuild("&a=b&c=d&e=f", "k", 1)))
	})

	t.Run("delete", func(t *testing.T) {
		assert.Equal(t, "c=d&e=f", string(QueryBuild("a=b&c=d&e=f", "a", "")))
		assert.Equal(t, "a=b&e=f", string(QueryBuild("a=b&c=d&e=f", "c", "")))
		assert.Equal(t, "a=b&c=d", string(QueryBuild("a=b&c=d&e=f", "e", "")))
		assert.Equal(t, "a=b&c=d&e=f", string(QueryBuild("a=b&c=d&e=f", "k", "")))
	})

	t.Run("delete-&", func(t *testing.T) {
		assert.Equal(t, "&c=d&e=f", string(QueryBuild("&a=b&c=d&e=f", "a", "")))
		assert.Equal(t, "&a=b&e=f", string(QueryBuild("&a=b&c=d&e=f", "c", "")))
		assert.Equal(t, "&a=b&c=d", string(QueryBuild("&a=b&c=d&e=f", "e", "")))
		assert.Equal(t, "&a=b&c=d&e=f", string(QueryBuild("&a=b&c=d&e=f", "k", "")))
	})
}
