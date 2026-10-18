// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strconv"

	"gitea.dev/modules/util"
)

type deprecatedKey struct {
	oldSection, oldKey, newSection, newKey, removeIn string
	convert                                          func(old ConfigKey) string
}

func invertBool(old ConfigKey) string {
	return strconv.FormatBool(!old.MustBool())
}

// keep rows after removeIn, users upgrading from older versions still need the warning
var deprecatedKeys = []deprecatedKey{
	{"server", "LFS_CONTENT_PATH", "lfs", "PATH", "v1.19.0", nil},
	{"server", "ENABLE_LETSENCRYPT", "server", "ENABLE_ACME", "v1.19.0", nil},
	{"server", "LETSENCRYPT_ACCEPTTOS", "server", "ACME_ACCEPTTOS", "v1.19.0", nil},
	{"server", "LETSENCRYPT_DIRECTORY", "server", "ACME_DIRECTORY", "v1.19.0", nil},
	{"server", "LETSENCRYPT_EMAIL", "server", "ACME_EMAIL", "v1.19.0", nil},
	{"mailer", "MAILER_TYPE", "mailer", "PROTOCOL", "v1.19.0", func(old ConfigKey) string { return old.In("", []string{"sendmail"}) }},
	{"mailer", "IS_TLS_ENABLED", "mailer", "PROTOCOL", "v1.19.0", func(old ConfigKey) string { return util.Iif(old.MustBool(), "smtps", "smtp+starttls") }},
	{"mailer", "DISABLE_HELO", "mailer", "ENABLE_HELO", "v1.19.0", invertBool},
	{"mailer", "SKIP_VERIFY", "mailer", "FORCE_TRUST_SERVER_CERT", "v1.19.0", nil},
	{"mailer", "USE_CERTIFICATE", "mailer", "USE_CLIENT_CERT", "v1.19.0", nil},
	{"mailer", "CERT_FILE", "mailer", "CLIENT_CERT_FILE", "v1.19.0", nil},
	{"mailer", "KEY_FILE", "mailer", "CLIENT_KEY_FILE", "v1.19.0", nil},
	{"mailer", "ENABLE_HTML_ALTERNATIVE", "mailer", "SEND_AS_PLAIN_TEXT", "v1.19.0", invertBool},
	{"repository", "DISABLE_MIRRORS", "mirror", "DISABLE_NEW_PULL", "v1.19.0", nil},
	{"git.reflog", "ENABLED", "git.config", "core.logAllRefUpdates", "1.21", func(old ConfigKey) string { return old.In("true", []string{"true", "false"}) }},
	{"git.reflog", "EXPIRATION", "git.config", "gc.reflogExpire", "1.21", func(old ConfigKey) string { return old.String() + ".days" }},
	{"oauth2", "ENABLE", "oauth2", "ENABLED", "v1.23.0", nil},
	{"cors", "X_FRAME_OPTIONS", "security", "X_FRAME_OPTIONS", "v1.26.0", nil},
	{"repository.pull-request", "POPULATE_SQUASH_COMMENT_WITH_COMMIT_MESSAGES", "repository.pull-request", "DEFAULT_SQUASH_MERGE_MESSAGE", "v30.0.0", func(old ConfigKey) string { return util.Iif(old.MustBool(), "COMMIT_MESSAGES", "PR_BODY") }},
}

// applyDeprecatedKeys copies each deprecated key of the table to its replacement unless the replacement is present
func applyDeprecatedKeys(rootCfg ConfigProvider) {
	for _, deprecated := range deprecatedKeys {
		oldSec, _ := rootCfg.GetSection(deprecated.oldSection)
		if oldSec == nil || !oldSec.HasKey(deprecated.oldKey) {
			continue
		}
		deprecatedSetting(rootCfg, deprecated.oldSection, deprecated.oldKey, deprecated.newSection, deprecated.newKey, deprecated.removeIn)
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
