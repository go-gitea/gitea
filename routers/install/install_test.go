// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package install

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newInstallTestConfig(t *testing.T, content string) setting.ConfigProvider {
	t.Helper()
	cfg, err := setting.NewConfigProviderFromData(content)
	require.NoError(t, err)
	return cfg
}

func configValue(cfg setting.ConfigProvider, section, key string) (string, bool) {
	configKey := setting.ConfigSectionKey(cfg.Section(section), key)
	if configKey == nil {
		return "", false
	}
	return configKey.String(), true
}

func TestEnsureInstallJWTSecret(t *testing.T) {
	t.Parallel()

	t.Run("generates a missing secret", func(t *testing.T) {
		t.Parallel()
		cfg := newInstallTestConfig(t, "")

		ensureInstallJWTSecret(cfg, "oauth2", "JWT_SECRET", "JWT_SECRET_URI")

		secret, ok := configValue(cfg, "oauth2", "JWT_SECRET")
		assert.True(t, ok)
		assert.NotEmpty(t, secret)
		_, uriExists := configValue(cfg, "oauth2", "JWT_SECRET_URI")
		assert.False(t, uriExists)
	})

	t.Run("preserves an existing inline secret", func(t *testing.T) {
		t.Parallel()
		cfg := newInstallTestConfig(t, "[oauth2]\nJWT_SECRET = existing\n")

		ensureInstallJWTSecret(cfg, "oauth2", "JWT_SECRET", "JWT_SECRET_URI")

		secret, ok := configValue(cfg, "oauth2", "JWT_SECRET")
		assert.True(t, ok)
		assert.Equal(t, "existing", secret)
	})

	t.Run("preserves an existing URI", func(t *testing.T) {
		t.Parallel()
		cfg := newInstallTestConfig(t, "[oauth2]\nJWT_SECRET_URI = file:/run/secrets/jwt\n")

		ensureInstallJWTSecret(cfg, "oauth2", "JWT_SECRET", "JWT_SECRET_URI")

		_, secretExists := configValue(cfg, "oauth2", "JWT_SECRET")
		assert.False(t, secretExists)
		uri, ok := configValue(cfg, "oauth2", "JWT_SECRET_URI")
		assert.True(t, ok)
		assert.Equal(t, "file:/run/secrets/jwt", uri)
	})

	t.Run("does not overwrite an existing LFS secret", func(t *testing.T) {
		t.Parallel()
		cfg := newInstallTestConfig(t, "[server]\nLFS_JWT_SECRET = existing-lfs\n")

		ensureInstallJWTSecret(cfg, "server", "LFS_JWT_SECRET", "LFS_JWT_SECRET_URI")

		secret, ok := configValue(cfg, "server", "LFS_JWT_SECRET")
		assert.True(t, ok)
		assert.Equal(t, "existing-lfs", secret)
	})
}

func TestInstallJWTSecretRespectsEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("URI environment variable prevents inline generation", func(t *testing.T) {
		t.Parallel()
		cfg := newInstallTestConfig(t, "")
		envs := []string{"GITEA__OAUTH2__JWT_SECRET_URI=file:/run/secrets/oauth-jwt"}

		setting.EnvironmentToConfig(cfg, envs)
		ensureInstallJWTSecret(cfg, "oauth2", "JWT_SECRET", "JWT_SECRET_URI")
		// The installer applies the same snapshot again after form values are written.
		setting.EnvironmentToConfig(cfg, envs)

		_, secretExists := configValue(cfg, "oauth2", "JWT_SECRET")
		assert.False(t, secretExists)
		uri, ok := configValue(cfg, "oauth2", "JWT_SECRET_URI")
		assert.True(t, ok)
		assert.Equal(t, "file:/run/secrets/oauth-jwt", uri)
	})

	t.Run("file environment variable supplies the inline secret", func(t *testing.T) {
		t.Parallel()
		secretPath := filepath.Join(t.TempDir(), "jwt-secret")
		require.NoError(t, os.WriteFile(secretPath, []byte("external-secret\n"), 0o600))
		cfg := newInstallTestConfig(t, "")
		envs := []string{"GITEA__OAUTH2__JWT_SECRET__FILE=" + secretPath}

		setting.EnvironmentToConfig(cfg, envs)
		ensureInstallJWTSecret(cfg, "oauth2", "JWT_SECRET", "JWT_SECRET_URI")

		secret, ok := configValue(cfg, "oauth2", "JWT_SECRET")
		assert.True(t, ok)
		assert.Equal(t, "external-secret", secret)
		_, uriExists := configValue(cfg, "oauth2", "JWT_SECRET_URI")
		assert.False(t, uriExists)
	})

	t.Run("LFS URI environment variable prevents inline generation", func(t *testing.T) {
		t.Parallel()
		cfg := newInstallTestConfig(t, "")
		envs := []string{"GITEA__SERVER__LFS_JWT_SECRET_URI=file:/run/secrets/lfs-jwt"}

		setting.EnvironmentToConfig(cfg, envs)
		ensureInstallJWTSecret(cfg, "server", "LFS_JWT_SECRET", "LFS_JWT_SECRET_URI")

		_, secretExists := configValue(cfg, "server", "LFS_JWT_SECRET")
		assert.False(t, secretExists)
		uri, ok := configValue(cfg, "server", "LFS_JWT_SECRET_URI")
		assert.True(t, ok)
		assert.Equal(t, "file:/run/secrets/lfs-jwt", uri)
	})
}
