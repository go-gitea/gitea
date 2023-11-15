// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigProviderBehaviors(t *testing.T) {
	t.Run("BuggyKeyOverwritten", func(t *testing.T) {
		cfg, _ := NewConfigProviderFromData(`
[foo]
key =
`)
		sec := cfg.Section("foo")
		secSub := cfg.Section("foo.bar")
		secSub.Key("key").MustString("1")             // try to read a key from subsection
		assert.Equal(t, "1", sec.Key("key").String()) // TODO: BUGGY! the key in [foo] is overwritten
	})

	t.Run("SubsectionSeeParentKeys", func(t *testing.T) {
		cfg, _ := NewConfigProviderFromData(`
[foo]
key = 123
`)
		secSub := cfg.Section("foo.bar.xxx")
		assert.Equal(t, "123", secSub.Key("key").String())
	})
	t.Run("TrailingSlash", func(t *testing.T) {
		cfg, _ := NewConfigProviderFromData(`
[foo]
key = E:\
xxx = yyy
`)
		sec := cfg.Section("foo")
		assert.Equal(t, "E:\\", sec.Key("key").String())
		assert.Equal(t, "yyy", sec.Key("xxx").String())
	})
}

func TestConfigProviderHelper(t *testing.T) {
	cfg, _ := NewConfigProviderFromData(`
[foo]
empty =
key = 123
`)

	sec := cfg.Section("foo")
	secSub := cfg.Section("foo.bar")

	// test empty key
	assert.Equal(t, "def", ConfigSectionKeyString(sec, "empty", "def"))
	assert.Equal(t, "xyz", ConfigSectionKeyString(secSub, "empty", "xyz"))

	// test non-inherited key, only see the keys in current section
	assert.NotNil(t, ConfigSectionKey(sec, "key"))
	assert.Nil(t, ConfigSectionKey(secSub, "key"))

	// test default behavior
	assert.Equal(t, "123", ConfigSectionKeyString(sec, "key"))
	assert.Equal(t, "", ConfigSectionKeyString(secSub, "key"))
	assert.Equal(t, "def", ConfigSectionKeyString(secSub, "key", "def"))

	assert.Equal(t, "123", ConfigInheritedKeyString(secSub, "key"))

	// Workaround for ini package's BuggyKeyOverwritten behavior
	assert.Equal(t, "", ConfigSectionKeyString(sec, "empty"))
	assert.Equal(t, "", ConfigSectionKeyString(secSub, "empty"))
	assert.Equal(t, "def", ConfigInheritedKey(secSub, "empty").MustString("def"))
	assert.Equal(t, "def", ConfigInheritedKey(secSub, "empty").MustString("xyz"))
	assert.Equal(t, "", ConfigSectionKeyString(sec, "empty"))
	assert.Equal(t, "def", ConfigSectionKeyString(secSub, "empty"))
}

func TestNewConfigProviderFromFile(t *testing.T) {
	cfg, err := NewConfigProviderFromFile("no-such.ini")
	assert.NoError(t, err)
	assert.True(t, cfg.IsLoadedFromEmpty())

	// load non-existing file and save
	testFile := t.TempDir() + "/test.ini"
	testFile1 := t.TempDir() + "/test1.ini"
	cfg, err = NewConfigProviderFromFile(testFile)
	assert.NoError(t, err)

	sec, _ := cfg.NewSection("foo")
	_, _ = sec.NewKey("k1", "a")
	assert.NoError(t, cfg.Save())
	_, _ = sec.NewKey("k2", "b")
	assert.NoError(t, cfg.SaveTo(testFile1))

	bs, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "[foo]\nk1 = a\n", string(bs))

	bs, err = os.ReadFile(testFile1)
	assert.NoError(t, err)
	assert.Equal(t, "[foo]\nk1 = a\nk2 = b\n", string(bs))

	// load existing file and save
	cfg, err = NewConfigProviderFromFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "a", cfg.Section("foo").Key("k1").String())
	sec, _ = cfg.NewSection("bar")
	_, _ = sec.NewKey("k1", "b")
	assert.NoError(t, cfg.Save())
	bs, err = os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "[foo]\nk1 = a\n\n[bar]\nk1 = b\n", string(bs))
}

func TestNewConfigProviderForLocale(t *testing.T) {
	// load locale from file
	localeFile := t.TempDir() + "/locale.ini"
	_ = os.WriteFile(localeFile, []byte(`k1=a`), 0o644)
	cfg, err := NewConfigProviderForLocale(localeFile)
	assert.NoError(t, err)
	assert.Equal(t, "a", cfg.Section("").Key("k1").String())

	// load locale from bytes
	cfg, err = NewConfigProviderForLocale([]byte("k1=foo\nk2=bar"))
	assert.NoError(t, err)
	assert.Equal(t, "foo", cfg.Section("").Key("k1").String())
	cfg, err = NewConfigProviderForLocale([]byte("k1=foo\nk2=bar"), []byte("k2=xxx"))
	assert.NoError(t, err)
	assert.Equal(t, "foo", cfg.Section("").Key("k1").String())
	assert.Equal(t, "xxx", cfg.Section("").Key("k2").String())
}

func TestDisableSaving(t *testing.T) {
	testFile := t.TempDir() + "/test.ini"
	_ = os.WriteFile(testFile, []byte("k1=a\nk2=b"), 0o644)
	cfg, err := NewConfigProviderFromFile(testFile)
	assert.NoError(t, err)

	cfg.DisableSaving()
	err = cfg.Save()
	assert.ErrorIs(t, err, errDisableSaving)

	saveCfg, err := cfg.PrepareSaving()
	assert.NoError(t, err)

	saveCfg.Section("").Key("k1").MustString("x")
	saveCfg.Section("").Key("k2").SetValue("y")
	saveCfg.Section("").Key("k3").SetValue("z")
	err = saveCfg.Save()
	assert.NoError(t, err)

	bs, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "k1 = a\nk2 = y\nk3 = z\n", string(bs))
}
