// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package i18n

import (
	"html/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocaleStore(t *testing.T) {
	testData1 := []byte(`
{
".dot.name": "Dot Name",
"fmt": "%[1]s %[2]s",
"only_in_default": "",

"section.sub": "Sub String",
"section.mixed": "test value; <span style=\"color: red; background: none;\">%s</span>"
}`)

	testData2 := []byte(`
{
	"fmt": "%[2]s %[1]s",
	"section.sub": "Changed Sub String"
}`)

	ls := NewLocaleStore()
	assert.NoError(t, ls.AddLocaleByJSON("lang1", "Lang1", testData1, nil))
	assert.NoError(t, ls.AddLocaleByJSON("lang2", "Lang2", testData2, nil))
	ls.SetDefaultLang("lang1")

	lang1, _ := ls.Locale("lang1")
	lang2, _ := ls.Locale("lang2")

	result := lang1.TrString("fmt", "a", "b")
	assert.Equal(t, "a b", result)

	result = lang2.TrString("fmt", "a", "b")
	assert.Equal(t, "b a", result)

	result = lang1.TrString("section.sub")
	assert.Equal(t, "Sub String", result)

	result = lang2.TrString("section.sub")
	assert.Equal(t, "Changed Sub String", result)

	langNone, _ := ls.Locale("none")
	result = langNone.TrString(".dot.name")
	assert.Equal(t, "Dot Name", result)

	result2 := lang2.TrHTML("section.mixed", "a&b")
	assert.EqualValues(t, `test value; <span style="color: red; background: none;">a&amp;b</span>`, result2)

	langs, descs := ls.ListLangNameDesc()
	assert.ElementsMatch(t, []string{"lang1", "lang2"}, langs)
	assert.ElementsMatch(t, []string{"Lang1", "Lang2"}, descs)

	found := lang1.HasKey("no-such")
	assert.False(t, found)
	found = lang2.HasKey("only_in_default")
	assert.True(t, found)

	assert.NoError(t, ls.Close())

	res := lang1.TrHTML("<no-such>")
	assert.Equal(t, "&lt;no-such&gt;", string(res))
}

func TestLocaleStoreMoreSource(t *testing.T) {
	testData1 := []byte(`
{
	"a": "11",
	"b": "12"
}
`)

	testData2 := []byte(`
{
	"b": "21",
	"c": "22"
}
`)

	ls := NewLocaleStore()
	assert.NoError(t, ls.AddLocaleByJSON("lang1", "Lang1", testData1, testData2))
	lang1, _ := ls.Locale("lang1")
	assert.Equal(t, "11", lang1.TrString("a"))
	assert.Equal(t, "21", lang1.TrString("b"))
	assert.Equal(t, "22", lang1.TrString("c"))
}

type stringerPointerReceiver struct {
	s string
}

func (s *stringerPointerReceiver) String() string {
	return s.s
}

type stringerStructReceiver struct {
	s string
}

func (s stringerStructReceiver) String() string {
	return s.s
}

type errorStructReceiver struct {
	s string
}

func (e errorStructReceiver) Error() string {
	return e.s
}

type errorPointerReceiver struct {
	s string
}

func (e *errorPointerReceiver) Error() string {
	return e.s
}

func TestLocaleWithTemplate(t *testing.T) {
	ls := NewLocaleStore()
	assert.NoError(t, ls.AddLocaleByJSON("lang1", "Lang1", []byte(`{"key":"<a>%s</a>"}`), nil))
	lang1, _ := ls.Locale("lang1")

	tmpl := template.New("test").Funcs(template.FuncMap{"tr": lang1.TrHTML})
	tmpl = template.Must(tmpl.Parse(`{{tr "key" .var}}`))

	cases := []struct {
		in   any
		want string
	}{
		{"<str>", "<a>&lt;str&gt;</a>"},
		{[]byte("<bytes>"), "<a>[60 98 121 116 101 115 62]</a>"},
		{template.HTML("<html>"), "<a><html></a>"},
		{stringerPointerReceiver{"<stringerPointerReceiver>"}, "<a>{&lt;stringerPointerReceiver&gt;}</a>"},
		{&stringerPointerReceiver{"<stringerPointerReceiver ptr>"}, "<a>&lt;stringerPointerReceiver ptr&gt;</a>"},
		{stringerStructReceiver{"<stringerStructReceiver>"}, "<a>&lt;stringerStructReceiver&gt;</a>"},
		{&stringerStructReceiver{"<stringerStructReceiver ptr>"}, "<a>&lt;stringerStructReceiver ptr&gt;</a>"},
		{errorStructReceiver{"<errorStructReceiver>"}, "<a>&lt;errorStructReceiver&gt;</a>"},
		{&errorStructReceiver{"<errorStructReceiver ptr>"}, "<a>&lt;errorStructReceiver ptr&gt;</a>"},
		{errorPointerReceiver{"<errorPointerReceiver>"}, "<a>{&lt;errorPointerReceiver&gt;}</a>"},
		{&errorPointerReceiver{"<errorPointerReceiver ptr>"}, "<a>&lt;errorPointerReceiver ptr&gt;</a>"},
	}

	buf := &strings.Builder{}
	for _, c := range cases {
		buf.Reset()
		assert.NoError(t, tmpl.Execute(buf, map[string]any{"var": c.in}))
		assert.Equal(t, c.want, buf.String())
	}
}

func TestLocaleStorePlural(t *testing.T) {
	en := []byte(`{
"days": {"one": "%d day", "other": "%d days"},
"legacy": "%d things",
"legacy_html": "1 %s",
"only_en": {"one": "%d en-one", "other": "%d en-other"}
}`)
	ru := []byte(`{
"days": {"one": "%d день", "few": "%d дня", "many": "%d дней", "other": "%d дня"},
"legacy": "%d штук"
}`)
	pl := []byte(`{"days": {"one": "%d dzień", "few": "%d dni", "many": "%d dni", "other": "%d dnia"}}`)
	ja := []byte(`{"days": {"other": "%d 日"}}`)

	ls := NewLocaleStore()
	assert.NoError(t, ls.AddLocaleByJSON("en-US", "English", en, nil))
	assert.NoError(t, ls.AddLocaleByJSON("ru-RU", "Russian", ru, nil))
	assert.NoError(t, ls.AddLocaleByJSON("pl-PL", "Polish", pl, nil))
	assert.NoError(t, ls.AddLocaleByJSON("ja-JP", "Japanese", ja, nil))
	ls.SetDefaultLang("en-US")

	test := func(lang, key string, cnt int64, expected string) {
		l, _ := ls.Locale(lang)
		assert.Equal(t, expected, l.TrPluralString(key, cnt, cnt), "%s %s %d", lang, key, cnt)
	}

	test("en-US", "days", 1, "1 day")
	test("en-US", "days", 0, "0 days")
	test("en-US", "days", 2, "2 days")

	for cnt, expected := range map[int64]string{
		1: "1 день", 2: "2 дня", 4: "4 дня", 5: "5 дней", 11: "11 дней", 12: "12 дней", 14: "14 дней",
		21: "21 день", 22: "22 дня", 25: "25 дней", 101: "101 день", 111: "111 дней", 0: "0 дней",
	} {
		test("ru-RU", "days", cnt, expected)
	}
	test("ru-RU", "days", -2, "-2 дня")

	test("pl-PL", "days", 1, "1 dzień")
	test("pl-PL", "days", 3, "3 dni")
	test("pl-PL", "days", 5, "5 dni")
	test("ja-JP", "days", 1, "1 日")

	// legacy plain-string values are used for every count
	test("ru-RU", "legacy", 1, "1 штук")
	test("ru-RU", "legacy", 5, "5 штук")
	test("en-US", "legacy", 1, "1 things")

	// missing in the locale: fall back to the default locale with its own plural rules
	test("ru-RU", "only_en", 1, "1 en-one")
	test("ru-RU", "only_en", 5, "5 en-other")
	// unknown key
	test("ru-RU", "no_such_key", 5, "no_such_key%!(EXTRA int64=5)")

	ru1, _ := ls.Locale("ru-RU")
	assert.True(t, ru1.HasPluralKey("days"))
	assert.False(t, ru1.HasPluralKey("legacy"))
	assert.False(t, ru1.HasPluralKey("only_en"))

	en1, _ := ls.Locale("en-US")
	assert.Equal(t, template.HTML("1 &lt;b&gt;"), en1.TrPluralHTML("legacy_html", 1, "<b>"))
}
