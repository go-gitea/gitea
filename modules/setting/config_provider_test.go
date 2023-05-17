// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
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
