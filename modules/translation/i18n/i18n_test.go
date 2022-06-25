// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Tr(t *testing.T) {
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
	assert.NoError(t, ls.AddLocaleByIni("lang1", "Lang1", testData1))
	assert.NoError(t, ls.AddLocaleByIni("lang2", "Lang2", testData2))
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
	assert.Equal(t, []string{"lang1", "lang2"}, langs)
	assert.Equal(t, []string{"Lang1", "Lang2"}, descs)
}
