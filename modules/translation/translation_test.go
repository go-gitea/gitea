// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package translation

import (
	"testing"

	"code.gitea.io/gitea/modules/translation/i18n"

	"github.com/stretchr/testify/assert"
)

func TestPrettyNumber(t *testing.T) {
	// TODO: make this package friendly to testing

	i18n.ResetDefaultLocales()

	allLangMap = make(map[string]*LangType)
	allLangMap["id-ID"] = &LangType{Lang: "id-ID", Name: "Bahasa Indonesia"}

	l := NewLocale("id-ID")
	assert.EqualValues(t, "1.000.000", l.PrettyNumber(1000000))

	l = NewLocale("nosuch")
	assert.EqualValues(t, "1,000,000", l.PrettyNumber(1000000))
}
