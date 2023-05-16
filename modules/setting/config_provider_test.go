// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigProviderBehaviors(t *testing.T) {
	// buggy overwritten behavior
	cfg, _ := NewConfigProviderFromData(`
[foo]
key =
`)
	cfg.Section("foo.bar").Key("key").MustString("1")            // try to read a key from subsection
	assert.Equal(t, "1", cfg.Section("foo").Key("key").String()) // TODO: BUGGY! the key in [foo] is overwritten

	// subsection can see parent keys
	cfg, _ = NewConfigProviderFromData(`
[foo]
key = 123
`)
	assert.Equal(t, "123", cfg.Section("foo.bar.xxx").Key("key").String())
}

func TestConfigProviderHelper(t *testing.T) {
	cfg, _ := NewConfigProviderFromData(`
[foo]
empty =
key = 123
`)

	assert.Equal(t, "def", ConfigSectionKeyString(cfg.Section("foo"), "empty", "def"))

	assert.NotNil(t, ConfigSectionKey(cfg.Section("foo"), "key"))
	assert.Nil(t, ConfigSectionKey(cfg.Section("foo.bar"), "key"))

	assert.Equal(t, "123", ConfigSectionKeyString(cfg.Section("foo"), "key"))
	assert.Equal(t, "", ConfigSectionKeyString(cfg.Section("foo.bar"), "key"))
	assert.Equal(t, "def", ConfigSectionKeyString(cfg.Section("foo.bar"), "key", "def"))

	assert.Equal(t, "123", ConfigInheritedKeyString(cfg.Section("foo.bar"), "key"))
}
