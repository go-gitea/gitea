// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocaleStore(t *testing.T) {
	testData1 := []byte(`
.dot.name = Dot Name
fmt = %[1]s %[2]s

[section]
sub = Sub String
mixed = test value; <span style="color: red\; background: none;">more text</span>
`)

	testData2 := []byte(`
fmt = %[2]s %[1]s

[section]
sub = Changed Sub String
`)

	ls := NewLocaleStore()
	assert.NoError(t, ls.AddLocaleByIni("lang1", "Lang1", testData1, nil))
	assert.NoError(t, ls.AddLocaleByIni("lang2", "Lang2", testData2, nil))
	ls.SetDefaultLang("lang1")

	result := ls.Tr("lang1", "fmt", "a", "b")
	assert.Equal(t, "a b", result)

	result = ls.Tr("lang2", "fmt", "a", "b")
	assert.Equal(t, "b a", result)

	result = ls.Tr("lang1", "section.sub")
	assert.Equal(t, "Sub String", result)

	result = ls.Tr("lang2", "section.sub")
	assert.Equal(t, "Changed Sub String", result)

	result = ls.Tr("", ".dot.name")
	assert.Equal(t, "Dot Name", result)

	result = ls.Tr("lang2", "section.mixed")
	assert.Equal(t, `test value; <span style="color: red; background: none;">more text</span>`, result)

	langs, descs := ls.ListLangNameDesc()
	assert.ElementsMatch(t, []string{"lang1", "lang2"}, langs)
	assert.ElementsMatch(t, []string{"Lang1", "Lang2"}, descs)

	found := ls.Has("lang1", "no-such")
	assert.False(t, found)
	assert.NoError(t, ls.Close())
}

func TestLocaleStoreMoreSource(t *testing.T) {
	testData1 := []byte(`
a=11
b=12
`)

	testData2 := []byte(`
b=21
c=22
`)

	ls := NewLocaleStore()
	assert.NoError(t, ls.AddLocaleByIni("lang1", "Lang1", testData1, testData2))
	assert.Equal(t, "11", ls.Tr("lang1", "a"))
	assert.Equal(t, "21", ls.Tr("lang1", "b"))
	assert.Equal(t, "22", ls.Tr("lang1", "c"))
}

func TestLocaleStoreQuirks(t *testing.T) {
	const nl = "\n"
	q := func(q1, s string, q2 ...string) string {
		return q1 + s + strings.Join(q2, "")
	}
	testDataList := []struct {
		in   string
		out  string
		hint string
	}{
		{` xx`, `xx`, "simple, no quote"},
		{`" xx"`, ` xx`, "simple, double-quote"},
		{`' xx'`, ` xx`, "simple, single-quote"},
		{"` xx`", ` xx`, "simple, back-quote"},

		{`x\"y`, `x\"y`, "no unescape, simple"},
		{q(`"`, `x\"y`, `"`), `"x\"y"`, "unescape, double-quote"},
		{q(`'`, `x\"y`, `'`), `x\"y`, "no unescape, single-quote"},
		{q("`", `x\"y`, "`"), `x\"y`, "no unescape, back-quote"},

		{q(`"`, `x\"y`) + nl + "b=", `"x\"y`, "half open, double-quote"},
		{q(`'`, `x\"y`) + nl + "b=", `'x\"y`, "half open, single-quote"},
		{q("`", `x\"y`) + nl + "b=`", `x\"y` + nl + "b=", "half open, back-quote, multi-line"},

		{`x ; y`, `x ; y`, "inline comment (;)"},
		{`x # y`, `x # y`, "inline comment (#)"},
		{`x \; y`, `x ; y`, `inline comment (\;)`},
		{`x \# y`, `x # y`, `inline comment (\#)`},
	}

	for _, testData := range testDataList {
		ls := NewLocaleStore()
		err := ls.AddLocaleByIni("lang1", "Lang1", []byte("a="+testData.in), nil)
		assert.NoError(t, err, testData.hint)
		assert.Equal(t, testData.out, ls.Tr("lang1", "a"), testData.hint)
		assert.NoError(t, ls.Close())
	}

	// TODO: Crowdin needs the strings to be quoted correctly and doesn't like incomplete quotes
	//       and Crowdin always outputs quoted strings if there are quotes in the strings.
	//       So, Gitea's `key="quoted" unquoted` content shouldn't be used on Crowdin directly,
	//       it should be converted to `key="\"quoted\" unquoted"` first.
	// TODO: We can not use UnescapeValueDoubleQuotes=true, because there are a lot of back-quotes in en-US.ini,
	//       then Crowdin will output:
	//       > key = "`x \" y`"
	//       Then Gitea will read a string with back-quotes, which is incorrect.
	// TODO: Crowdin might generate multi-line strings, quoted by double-quote, it's not supported by LocaleStore
	//       LocaleStore uses back-quote for multi-line strings, it's not supported by Crowdin.
	// TODO: Crowdin doesn't support back-quote as string quoter, it mainly uses double-quote
	//       so, the following line will be parsed as: value="`first", comment="second`" on Crowdin
	//       > a = `first; second`
}
