// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeEnvSectionKey(t *testing.T) {
	ok, section, key := decodeEnvSectionKey("SEC__KEY")
	assert.True(t, ok)
	assert.Equal(t, "sec", section)
	assert.Equal(t, "KEY", key)

	ok, section, key = decodeEnvSectionKey("sec__key")
	assert.True(t, ok)
	assert.Equal(t, "sec", section)
	assert.Equal(t, "key", key)

	ok, section, key = decodeEnvSectionKey("LOG_0x2E_CONSOLE__STDERR")
	assert.True(t, ok)
	assert.Equal(t, "log.console", section)
	assert.Equal(t, "STDERR", key)

	ok, section, key = decodeEnvSectionKey("SEC")
	assert.False(t, ok)
	assert.Equal(t, "", section)
	assert.Equal(t, "", key)
}

func TestDecodeEnvironmentKey(t *testing.T) {
	prefix := "GITEA__"
	suffix := "__FILE"

	ok, section, key, file := decodeEnvironmentKey(prefix, suffix, "SEC__KEY")
	assert.False(t, ok)
	assert.Equal(t, "", section)
	assert.Equal(t, "", key)
	assert.False(t, file)

	ok, section, key, file = decodeEnvironmentKey(prefix, suffix, "GITEA__SEC")
	assert.False(t, ok)
	assert.Equal(t, "", section)
	assert.Equal(t, "", key)
	assert.False(t, file)

	ok, section, key, file = decodeEnvironmentKey(prefix, suffix, "GITEA____KEY")
	assert.True(t, ok)
	assert.Equal(t, "", section)
	assert.Equal(t, "KEY", key)
	assert.False(t, file)

	ok, section, key, file = decodeEnvironmentKey(prefix, suffix, "GITEA__SEC__KEY")
	assert.True(t, ok)
	assert.Equal(t, "sec", section)
	assert.Equal(t, "KEY", key)
	assert.False(t, file)

	// with "__FILE" suffix, it doesn't support to write "[sec].FILE" to config (no such key FILE is used in Gitea)
	// but it could be fixed in the future by adding a new suffix like "__VALUE" (no such key VALUE is used in Gitea either)
	ok, section, key, file = decodeEnvironmentKey(prefix, suffix, "GITEA__SEC__FILE")
	assert.False(t, ok)
	assert.Equal(t, "", section)
	assert.Equal(t, "", key)
	assert.True(t, file)

	ok, section, key, file = decodeEnvironmentKey(prefix, suffix, "GITEA__SEC__KEY__FILE")
	assert.True(t, ok)
	assert.Equal(t, "sec", section)
	assert.Equal(t, "KEY", key)
	assert.True(t, file)
}

func TestEnvironmentToConfig(t *testing.T) {
	cfg, _ := NewConfigProviderFromData("")

	changed := EnvironmentToConfig(cfg, nil)
	assert.False(t, changed)

	cfg, err := NewConfigProviderFromData(`
[sec]
key = old
`)
	assert.NoError(t, err)

	changed = EnvironmentToConfig(cfg, []string{"GITEA__sec__key=new"})
	assert.True(t, changed)
	assert.Equal(t, "new", cfg.Section("sec").Key("key").String())

	changed = EnvironmentToConfig(cfg, []string{"GITEA__sec__key=new"})
	assert.False(t, changed)

	tmpFile := t.TempDir() + "/the-file"
	_ = os.WriteFile(tmpFile, []byte("value-from-file"), 0o644)
	changed = EnvironmentToConfig(cfg, []string{"GITEA__sec__key__FILE=" + tmpFile})
	assert.True(t, changed)
	assert.Equal(t, "value-from-file", cfg.Section("sec").Key("key").String())

	cfg, _ = NewConfigProviderFromData("")
	_ = os.WriteFile(tmpFile, []byte("value-from-file\n"), 0o644)
	EnvironmentToConfig(cfg, []string{"GITEA__sec__key__FILE=" + tmpFile})
	assert.Equal(t, "value-from-file", cfg.Section("sec").Key("key").String())

	cfg, _ = NewConfigProviderFromData("")
	_ = os.WriteFile(tmpFile, []byte("value-from-file\r\n"), 0o644)
	EnvironmentToConfig(cfg, []string{"GITEA__sec__key__FILE=" + tmpFile})
	assert.Equal(t, "value-from-file", cfg.Section("sec").Key("key").String())

	cfg, _ = NewConfigProviderFromData("")
	_ = os.WriteFile(tmpFile, []byte("value-from-file\n\n"), 0o644)
	EnvironmentToConfig(cfg, []string{"GITEA__sec__key__FILE=" + tmpFile})
	assert.Equal(t, "value-from-file\n", cfg.Section("sec").Key("key").String())
}
