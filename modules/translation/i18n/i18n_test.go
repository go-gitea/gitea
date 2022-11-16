// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package i18n

import (
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
	assert.Equal(t, []string{"lang1", "lang2"}, langs)
	assert.Equal(t, []string{"Lang1", "Lang2"}, descs)

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
