// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package translation

import (
	"testing"

	"gitea.dev/modules/translation/i18n"

	"github.com/stretchr/testify/assert"
)

func TestPrettyNumber(t *testing.T) {
	// TODO: make this package friendly to testing

	i18n.ResetDefaultLocales()

	allLangMap = make(map[string]*LangType)
	allLangMap["id-ID"] = &LangType{Lang: "id-ID", Name: "Bahasa Indonesia"}

	l := NewLocale("id-ID")
	assert.Equal(t, "1.000.000", l.PrettyNumber(1000000))
	assert.Equal(t, "1.000.000,1", l.PrettyNumber(1000000.1))
	assert.Equal(t, "1.000.000", l.PrettyNumber("1000000"))
	assert.Equal(t, "1.000.000", l.PrettyNumber("1000000.0"))
	assert.Equal(t, "1.000.000,1", l.PrettyNumber("1000000.1"))

	l = NewLocale("nosuch")
	assert.Equal(t, "1,000,000", l.PrettyNumber(1000000))
	assert.Equal(t, "1,000,000.1", l.PrettyNumber(1000000.1))
}

func TestTrPluralLegacyCompat(t *testing.T) {
	i18n.ResetDefaultLocales()
	allLangMap = map[string]*LangType{
		"en-US": {Lang: "en-US", Name: "English"},
		"ru-RU": {Lang: "ru-RU", Name: "Russian"},
		"de-DE": {Lang: "de-DE", Name: "German"},
	}
	en := `{"x.count": {"one": "%d item", "other": "%d items"}, "x.cmt": {"one": "%d comment", "other": "%d comments"}}`
	// ru is not migrated: old "_1"/"_n" strings, and the irregular legacy names for "x.cmt"
	ru := `{"x.count_1": "%d штука", "x.count_n": "%d штук", "x.cmt_1": "%d комментарий", "x.cmt": "%d комментариев"}`
	assert.NoError(t, i18n.DefaultLocales.AddLocaleByJSON("en-US", "English", []byte(en), nil))
	assert.NoError(t, i18n.DefaultLocales.AddLocaleByJSON("ru-RU", "Russian", []byte(ru), nil))
	assert.NoError(t, i18n.DefaultLocales.AddLocaleByJSON("de-DE", "German", []byte(`{}`), nil))
	i18n.DefaultLocales.SetDefaultLang("en-US")
	legacyPluralKeys["x.cmt"] = [2]string{"x.cmt_1", "x.cmt"}
	defer delete(legacyPluralKeys, "x.cmt")

	en1 := NewLocale("en-US")
	assert.Equal(t, "1 item", en1.TrPluralString("x.count", 1, 1))
	assert.Equal(t, "2 items", en1.TrPluralString("x.count", int64(2), 2))
	assert.EqualValues(t, "3 items", en1.TrPlural("x.count", uint8(3), 3))

	ru1 := NewLocale("ru-RU")
	assert.False(t, ru1.HasPluralKey("x.count"))
	assert.Equal(t, "21 штука", ru1.TrPluralString("x.count", 21, 21))
	assert.Equal(t, "5 штук", ru1.TrPluralString("x.count", 5, 5))
	assert.EqualValues(t, "1 комментарий", ru1.TrPlural("x.cmt", 1, 1))
	assert.EqualValues(t, "5 комментариев", ru1.TrPlural("x.cmt", 5, 5))

	// a locale without any translation falls back to the default locale
	assert.Equal(t, "5 items", NewLocale("de-DE").TrPluralString("x.count", 5, 5))

	assert.JSONEq(t, `{"one":"%d item","other":"%d items"}`, en1.TrPluralForms("x.count"))
	assert.JSONEq(t, `{"one":"%d штука","other":"%d штук"}`, ru1.TrPluralForms("x.count"))
	assert.JSONEq(t, `{"one":"%d item","other":"%d items"}`, NewLocale("de-DE").TrPluralForms("x.count"))
}
