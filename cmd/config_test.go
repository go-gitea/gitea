// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigEdit(t *testing.T) {
	tmpDir := t.TempDir()
	configOld := tmpDir + "/app-old.ini"
	configTemplate := tmpDir + "/app-template.ini"
	_ = os.WriteFile(configOld, []byte(`
[sec]
k1=v1
k2=v2
`), os.ModePerm)

	_ = os.WriteFile(configTemplate, []byte(`
[sec]
k1=in-template

[sec2]
k3=v3
`), os.ModePerm)

	t.Setenv("GITEA__EnV__KeY", "val")

	t.Run("OutputToNewWithEnv", func(t *testing.T) {
		configNew := tmpDir + "/app-new.ini"
		err := NewMainApp(AppVersion{}).Run(t.Context(), []string{
			"./gitea", "--config", configOld,
			"config", "edit-ini",
			"--apply-env",
			"--config-keep-keys", configTemplate,
			"--out", configNew,
		})
		require.NoError(t, err)

		// "k1" old value is kept because its key is in the template
		// "k2" is removed because it isn't in the template
		// "k3" isn't in new config because it isn't in the old config
		// [env] is applied from environment variable
		data, _ := os.ReadFile(configNew)
		require.Equal(t, `[sec]
k1 = v1

[env]
KeY = val
`, string(data))
	})

	t.Run("OutputToExisting(environment-to-ini)", func(t *testing.T) {
		// the legacy "environment-to-ini" (now a wrapper script) behavior:
		// if no "--out", then "--in-place" must be used to overwrite the existing "--config" file
		err := NewMainApp(AppVersion{}).Run(t.Context(), []string{
			"./gitea", "config", "edit-ini",
			"--apply-env",
			"--config", configOld,
		})
		require.ErrorContains(t, err, "either --in-place or --out must be specified")

		// simulate the "environment-to-ini" behavior with "--in-place"
		err = NewMainApp(AppVersion{}).Run(t.Context(), []string{
			"./gitea", "config", "edit-ini",
			"--in-place",
			"--apply-env",
			"--config", configOld,
		})
		require.NoError(t, err)

		data, _ := os.ReadFile(configOld)
		require.Equal(t, `[sec]
k1 = v1
k2 = v2

[env]
KeY = val
`, string(data))
	})
}
