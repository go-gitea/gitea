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

func TestPrettyNumberArg(t *testing.T) {
	i18n.ResetDefaultLocales()

	allLangMap = make(map[string]*LangType)
	allLangMap["id-ID"] = &LangType{Lang: "id-ID", Name: "Bahasa Indonesia"}

	assert.NoError(t, i18n.DefaultLocales.AddLocaleByJSON("id-ID", "Bahasa Indonesia", []byte(`{
		"item_1": "%d item",
		"item_n": "%d items",
		"message": "%d <span>%s</span>"
	}`), nil))
	i18n.DefaultLocales.SetDefaultLang("id-ID")

	l := NewLocale("id-ID")
	assert.EqualValues(t, "1.000 items", l.TrN(1000, "item_1", "item_n", l.PrettyNumberArg(1000)))
	assert.EqualValues(t, "1.000 <span>a&amp;b</span>", l.Tr("message", l.PrettyNumberArg(1000), "a&b"))
}
