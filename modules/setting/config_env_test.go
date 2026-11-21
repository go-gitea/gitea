// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeEnvSectionKey(t *testing.T) {
	for _, tc := range []struct {
		input, section, key string
	}{
		{"SEC__KEY", "sec", "KEY"},
		{"sec__key", "sec", "key"},
		{"LOG_0x2E_CONSOLE__STDERR", "log.console", "STDERR"},
	} {
		ok, section, key := decodeEnvSectionKey(tc.input)
		assert.True(t, ok)
		assert.Equal(t, tc.section, section)
		assert.Equal(t, tc.key, key)
	}

	ok, section, key := decodeEnvSectionKey("SEC")
	assert.False(t, ok)
	assert.Empty(t, section)
	assert.Empty(t, key)
}

func TestDecodeEnvironmentKey(t *testing.T) {
	prefix := "GITEA__"
	suffix := "__FILE"

	for _, input := range []string{"SEC__KEY", "GITEA__SEC"} {
		ok, section, key, file := decodeEnvironmentKey(prefix, suffix, input)
		assert.False(t, ok)
		assert.Empty(t, section)
		assert.Empty(t, key)
		assert.False(t, file)
	}

	ok, section, key, file := decodeEnvironmentKey(prefix, suffix, "GITEA____KEY")
	assert.True(t, ok)
	assert.Empty(t, section)
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
	assert.Empty(t, section)
	assert.Empty(t, key)
	assert.True(t, file)

	ok, section, key, file = decodeEnvironmentKey(prefix, suffix, "GITEA__SEC__KEY__FILE")
	assert.True(t, ok)
	assert.Equal(t, "sec", section)
	assert.Equal(t, "KEY", key)
	assert.True(t, file)

	ok, _, _, _ = decodeEnvironmentKey("PREFIX__", "", "PREFIX__SEC__KEY")
	assert.True(t, ok)
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

	for _, tc := range []struct {
		content, want string
	}{
		{"value-from-file\n", "value-from-file"},
		{"value-from-file\r\n", "value-from-file"},
		{"value-from-file\n\n", "value-from-file\n"},
	} {
		cfg, _ = NewConfigProviderFromData("")
		_ = os.WriteFile(tmpFile, []byte(tc.content), 0o644)
		EnvironmentToConfig(cfg, []string{"GITEA__sec__key__FILE=" + tmpFile})
		assert.Equal(t, tc.want, cfg.Section("sec").Key("key").String())
	}
}

func TestEnvironmentToConfigSubSecKey(t *testing.T) {
	// the INI package has a quirk: by default, the keys are inherited.
	// when maintaining the keys, the newly added sub key should not be affected by the parent key.
	cfg, err := NewConfigProviderFromData(`
[sec]
key = some
`)
	assert.NoError(t, err)

	changed := EnvironmentToConfig(cfg, []string{"GITEA__sec_0X2E_sub__key=some"})
	assert.True(t, changed)

	tmpFile := t.TempDir() + "/test-sub-sec-key.ini"
	defer os.Remove(tmpFile)
	err = cfg.SaveTo(tmpFile)
	assert.NoError(t, err)
	bs, err := os.ReadFile(tmpFile)
	assert.NoError(t, err)
	assert.Equal(t, `[sec]
key = some

[sec.sub]
key = some
`, string(bs))
}
