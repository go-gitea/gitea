// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
)

// checkForRemovedSettings checks the entire configuration for removed keys and gathers them fataling if any is present
// Arbitrary permament removal is 3 releases
func checkForRemovedSettings(rootCfg ConfigProvider) []error {
	var errs []error

	removedSettings := []struct {
		oldSection, oldKey, newSection, newKey, version string
	}{
		{"service", "EMAIL_DOMAIN_WHITELIST", "service", "EMAIL_DOMAIN_ALLOWLIST", "1.21"},
		{"mailer", "MAILER_TYPE", "mailer", "PROTOCOL", "v1.19"},
		{"mailer", "HOST", "mailer", "SMTP_ADDR", "v1.19"},
		{"mailer", "IS_TLS_ENABLED", "mailer", "PROTOCOL", "v1.19"},
		{"mailer", "DISABLE_HELO", "mailer", "ENABLE_HELO", "v1.19"},
		{"mailer", "SKIP_VERIFY", "mailer", "FORCE_TRUST_SERVER_CERT", "v1.19"},
		{"mailer", "USE_CERTIFICATE", "mailer", "USE_CLIENT_CERT", "v1.19"},
		{"mailer", "CERT_FILE", "mailer", "CLIENT_CERT_FILE", "v1.19"},
		{"mailer", "KEY_FILE", "mailer", "CLIENT_KEY_FILE", "v1.19"},
		{"task", "QUEUE_TYPE", "queue.task", "TYPE", "v1.19"},
		{"task", "QUEUE_CONN_STR", "queue.task", "CONN_STR", "v1.19"},
		{"task", "QUEUE_LENGTH", "queue.task", "LENGTH", "v1.19"},
		{"server", "ENABLE_LETSENCRYPT", "server", "ENABLE_ACME", "v1.19"},
		{"server", "LETSENCRYPT_ACCEPTTOS", "server", "ACME_ACCEPTTOS", "v1.19"},
		{"server", "LETSENCRYPT_DIRECTORY", "server", "ACME_DIRECTORY", "v1.19"},
		{"server", "LETSENCRYPT_EMAIL", "server", "ACME_EMAIL", "v1.19"},
		{"git.reflog", "ENABLED", "git.config", "core.logAllRefUpdates", "1.21"},
		{"git.reflog", "EXPIRATION", "git.config", "core.reflogExpire", "1.21"},
		{"repository", "DISABLE_MIRRORS", "mirror", "ENABLED", "v1.19"},
		{"server", "LFS_CONTENT_PATH", "lfs", "PATH", "v1.19"},
		{"log", "XORM", "log", "logger.xorm.MODE", "1.21"},
		{"log", "ENABLE_XORM_LOG", "log", "logger.xorm.MODE", "1.21"},
		{"log", "ROUTER", "log", "logger.router.MODE", "1.21"},
		{"log", "DISABLE_ROUTER_LOG", "log", "logger.router.MODE", "1.21"},
		{"log", "ACCESS", "log", "logger.access.MODE", "1.21"},
		{"log", "ENABLE_ACCESS_LOG", "log", "logger.access.MODE", "1.21"},
	}

	for _, rs := range removedSettings {
		if err := removedSetting(rootCfg, rs.oldSection, rs.oldKey, rs.newSection, rs.newKey, rs.version); err != nil {
			errs = append(errs, err)
		}
	}

	if err := removedOption(rootCfg, "mailer", "PROTOCOL", "smtp+startls", "smtp+starttls", "v1.19"); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// removedOption checks if configuration has an oldValue under key in section and returns error for user if it does
func removedOption(rootCfg ConfigProvider, section, key, oldValue, newValue, version string) error {
	if rootCfg.Section(section).Key(key).String() == oldValue {
		return fmt.Errorf("Config option `[%s].%s=%s` was removed in %s. Please use `[%s].%s=%s` instead", section, key, oldValue, version, section, key, newValue)
	}
	return nil
}

// removedSetting checks if oldKey exists in oldSection and returns an error for user if it does
func removedSetting(rootCfg ConfigProvider, oldSection, oldKey, newSection, newKey, version string) error {
	if rootCfg.Section(oldSection).HasKey(oldKey) {
		return fmt.Errorf("Config option `[%s].%s` was removed in %s. Please use `[%s].%s` instead", oldSection, oldKey, version, newSection, newKey)
	}
	return nil
}
