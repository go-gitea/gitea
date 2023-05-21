// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

func TestDecodeEnvSectionKey(t *testing.T) {
	section, key := decodeEnvSectionKey("SEC__KEY")
	assert.Equal(t, "sec", section)
	assert.Equal(t, "KEY", key)

	section, key = decodeEnvSectionKey("sec__key")
	assert.Equal(t, "sec", section)
	assert.Equal(t, "key", key)

	section, key = decodeEnvSectionKey("LOG_0x2E_CONSOLE__STDERR")
	assert.Equal(t, "log.console", section)
	assert.Equal(t, "STDERR", key)
}

func TestDecodeEnvironmentKey(t *testing.T) {
	prefix := "GITEA__"
	suffix := "__FILE"

	ok, section, key, file := decodeEnvironmentKey(prefix, suffix, "SEC__KEY")
	assert.False(t, ok)
	assert.Equal(t, "", section)
	assert.Equal(t, "", key)
	assert.False(t, file)

	ok, section, key, file = decodeEnvironmentKey(prefix, suffix, "GITEA__SEC__KEY")
	assert.True(t, ok)
	assert.Equal(t, "sec", section)
	assert.Equal(t, "KEY", key)
	assert.False(t, file)

	ok, section, key, file = decodeEnvironmentKey(prefix, suffix, "GITEA__SEC__KEY__FILE")
	assert.True(t, ok)
	assert.Equal(t, "sec", section)
	assert.Equal(t, "KEY", key)
	assert.True(t, file)
}

func TestEnvironmentToConfig(t *testing.T) {
	cfg := ini.Empty()

	changed := EnvironmentToConfig(cfg, "GITEA__", "__FILE", nil)
	assert.False(t, changed)

	cfg, err := ini.Load([]byte(`
[sec]
key = old
`))
	assert.NoError(t, err)

	changed = EnvironmentToConfig(cfg, "GITEA__", "__FILE", []string{"GITEA__sec__key=new"})
	assert.True(t, changed)
	assert.Equal(t, "new", cfg.Section("sec").Key("key").String())

	changed = EnvironmentToConfig(cfg, "GITEA__", "__FILE", []string{"GITEA__sec__key=new"})
	assert.False(t, changed)

	tmpFile := t.TempDir() + "/the-file"
	_ = os.WriteFile(tmpFile, []byte("value-from-file"), 0o644)
	changed = EnvironmentToConfig(cfg, "GITEA__", "__FILE", []string{"GITEA__sec__key__FILE=" + tmpFile})
	assert.True(t, changed)
	assert.Equal(t, "value-from-file", cfg.Section("sec").Key("key").String())
}
