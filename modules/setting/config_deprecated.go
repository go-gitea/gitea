// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strconv"

	"gitea.dev/modules/util"
)

type deprecatedKey struct {
	oldSection, oldKey, newSection, newKey string
	convert                                func(old ConfigKey) string
}

func invertBool(old ConfigKey) string {
	return strconv.FormatBool(!old.MustBool())
}

// keep rows, users upgrading from older versions still need them
var deprecatedKeys = []deprecatedKey{
	{"server", "LFS_CONTENT_PATH", "lfs", "PATH", nil},
	{"server", "ENABLE_LETSENCRYPT", "server", "ENABLE_ACME", nil},
	{"server", "LETSENCRYPT_ACCEPTTOS", "server", "ACME_ACCEPTTOS", nil},
	{"server", "LETSENCRYPT_DIRECTORY", "server", "ACME_DIRECTORY", nil},
	{"server", "LETSENCRYPT_EMAIL", "server", "ACME_EMAIL", nil},
	{"mailer", "MAILER_TYPE", "mailer", "PROTOCOL", func(old ConfigKey) string { return old.In("", []string{"sendmail"}) }},
	{"mailer", "IS_TLS_ENABLED", "mailer", "PROTOCOL", func(old ConfigKey) string { return util.Iif(old.MustBool(), "smtps", "smtp+starttls") }},
	{"mailer", "DISABLE_HELO", "mailer", "ENABLE_HELO", invertBool},
	{"mailer", "SKIP_VERIFY", "mailer", "FORCE_TRUST_SERVER_CERT", nil},
	{"mailer", "USE_CERTIFICATE", "mailer", "USE_CLIENT_CERT", nil},
	{"mailer", "CERT_FILE", "mailer", "CLIENT_CERT_FILE", nil},
	{"mailer", "KEY_FILE", "mailer", "CLIENT_KEY_FILE", nil},
	{"mailer", "ENABLE_HTML_ALTERNATIVE", "mailer", "SEND_AS_PLAIN_TEXT", invertBool},
	{"repository", "DISABLE_MIRRORS", "mirror", "DISABLE_NEW_PULL", nil},
	{"git.reflog", "ENABLED", "git.config", "core.logAllRefUpdates", func(old ConfigKey) string { return old.In("true", []string{"true", "false"}) }},
	{"git.reflog", "EXPIRATION", "git.config", "gc.reflogExpire", func(old ConfigKey) string { return old.String() + ".days" }},
	{"oauth2", "ENABLE", "oauth2", "ENABLED", nil},
	{"cors", "X_FRAME_OPTIONS", "security", "X_FRAME_OPTIONS", nil},
	{"repository.pull-request", "POPULATE_SQUASH_COMMENT_WITH_COMMIT_MESSAGES", "repository.pull-request", "DEFAULT_SQUASH_MERGE_MESSAGE", func(old ConfigKey) string { return util.Iif(old.MustBool(), "COMMIT_MESSAGES", "PR_BODY") }},
}

// applyDeprecatedKeys copies each deprecated key of the table to its replacement unless the replacement is present
func applyDeprecatedKeys(rootCfg ConfigProvider) {
	for _, deprecated := range deprecatedKeys {
		oldSec, _ := rootCfg.GetSection(deprecated.oldSection)
		if oldSec == nil || !oldSec.HasKey(deprecated.oldKey) {
			continue
		}
		deprecatedSetting(rootCfg, deprecated.oldSection, deprecated.oldKey, deprecated.newSection, deprecated.newKey)
		if newSec, _ := rootCfg.GetSection(deprecated.newSection); newSec != nil && newSec.HasKey(deprecated.newKey) {
			continue
		}
		oldKey := oldSec.Key(deprecated.oldKey)
		value := oldKey.String()
		if deprecated.convert != nil {
			value = deprecated.convert(oldKey)
		}
		if value != "" {
			_, _ = rootCfg.Section(deprecated.newSection).NewKey(deprecated.newKey, value)
		}
	}
}
