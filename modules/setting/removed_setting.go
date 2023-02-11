// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	ini "gopkg.in/ini.v1"
)

type settingExistType int

const (
	settingRemoved settingExistType = iota
	settingReplaced
	settingMovedToDB
)

type whenChanged int

const (
	pastVersion whenChanged = iota
	nextVersion
)

type removedSetting struct {
	version            string // Gitea version that removed it / will remove it
	section            string // Corresponding section in the app.ini without the parentheses
	key                string // Exact key in the app.ini, so should be uppercased
	replacementSection string
	replacementKey     string
	existType          settingExistType  // In what form does this setting still exist?
	when               whenChanged // When did/will this change happen?
}

// getTemplateLogMessage returns an unformated log message for this setting.
// The returned template accepts the following commands:
// - %[1]s: old [section].key
// - %[2]s: correct tense of "is"
// - %[3]s: gitea version
// -
func (r *removedSetting) getTemplateLogMessage() string {
	switch r.existType {
	case settingMovedToDB:
		return "The setting %[1]s in your config file has been copied and moved to the database table 'sys_setting' under the key "
	case settingReplaced:
		return "The setting %[1]s in your config file %[2]s removed in Gitea %[3]s. %s."
	case settingRemoved:
		return "The setting %[1]s in your config file is no longer used since Gitea %[3]s. It has no documented replacement."
	default:
		panic("Missing setting replacement type: " + strconv.Itoa(int(r.existType)) + " cannot be converted to a log message.")
	}
}

// getTense returns the correct tense of "is" for this removed setting
func (r *removedSetting) getTense() string {
	switch r.when {
	case nextVersion:
		return "will be"
	case pastVersion:
		return "was"
	default:
		panic("Unknown setting changed time: " + strconv.Itoa(int(r.when)))
	}
}

func (r *removedSetting) validate() {
	if r.existType == settingMovedToDB {
		r.replacementKey = strings.ToLower(r.replacementKey)
		r.key = strings.ToLower(r.key)
	} else {
		r.replacementKey = strings.ToUpper(r.replacementKey)
		r.key = strings.ToUpper(r.key)
	}
}

func toIniSection(section, key string) string {
	return "[" + section + "]." + strings.ToUpper(key)
}

func toDBSection(section, key string) string {
	return section + "." + key
}

var removedSettings map[string][]removedSetting // ordered by section (for performance)

func removeSetting(setting *removedSetting) {
	setting.validate()
	// Append the setting at the corresponding entry
	sectionList := removedSettings[setting.section]
	sectionList = append(sectionList, *setting)
	removedSettings[setting.section] = sectionList
}

// Adds a notice that the given setting under "[section].key" has been replaced by "[replacementSection].replacementKey"
// "key" and "replacementKey" should be exactly like they are in the app.ini
func MoveSetting(version, section, key, replacementSection, replacementKey string) {
	removeSetting(&removedSetting{
		version:            version,
		section:            section,
		key:                key,
		replacementSection: replacementSection,
		replacementKey:     replacementKey,
		when:               past,
	})
}

// Adds a notice that the given setting under "[section].key" has been replaced by "[section].replacementKey"
// "key" and "replacementKey" should be exactly like they are in the app.ini
func MoveSettingInSection(version, section, key, replacementKey string) {
	MoveSetting(version, section, key, section, replacementKey)
}

// Adds a notice that the given settings under "[section].key(s)" have been removed without any replacement
// "key"s should be exactly like they are in the app.ini
func PurgeSettings(version, section string, keys ...string) {
	for _, key := range keys {
		removeSetting(&removedSetting{
			version: version,
			section: section,
			key:     key,
		})
	}
}

// Adds a notice that the given setting under "[section].key" has been deprecated and should be replaced with "[replacementSection].replacementKey" soon
func DeprecateSetting(version, section, key, replacementSection, replacementKey string) {
	removeSetting(&removedSetting{
		version:            version,
		section:            section,
		key:                key,
		replacementSection: replacementSection,
		replacementKey:     replacementKey,
		existType:          settingToBeRemoved,
	})
}

// Adds a notice that the given setting under "[section].key" has been deprecated and should be replaced with "[section].replacementKey" soon
func DeprecateSettingSameSection(version, section, key, replacementKey string) {
	DeprecateSetting(version, section, key, section, replacementKey)
}

// Deprecates the given (still accepted and existing) settings under "[section].key" for removal
// keys should be formatted exactly like they are in the app.ini
func DeprecateSettingsForRemoval(version, section string, keys ...string) {
	for _, key := range keys {
		removeSetting(&removedSetting{
			version:   version,
			section:   section,
			key:       key,
			existType: settingToBeRemoved,
		})
	}
}

// Marks all given setting keys in the given section as moved to the database.
// keys should be formatted exactly like they are in the app.ini
func MoveSettingsToDB(version, section string, keys ...string) {
	for _, key := range keys {
		removeSetting(&removedSetting{
			version:   version,
			section:   section,
			key:       key,
			existType: settingMovedToDB,
		})
	}
}

// Adds a warning in the logs for all settings that are still present despite not being used anymore
func PrintRemovedSettings(cfg *ini.File) {
	for sectionName, removedList := range removedSettings {
		section, err := cfg.GetSection(sectionName)
		if section == nil || err != nil {
			continue
		}
		for _, removed := range removedList {
			if section.HasKey(removed.key) {
				log.Error(removed.getLogTemplate(), toIniSection(removed.section, removed.key), removed.getTense(), removed.version)
			}
		}
	}
}

// Adds all previously removed settings
func init() {
	MoveSettingInSection("6", "api", "ENABLE_SWAGGER_ENDPOINT", "ENABLE_SWAGGER")

	PurgeSettings("9", "log.database", "LEVEL", "DRIVER", "CONN")

	MoveSetting("12", "markup.sanitizer", "ELEMENT", "markup.sanitizer.1", "ELEMENT")
	MoveSetting("12", "markup.sanitizer", "ALLOW_ATTR", "markup.sanitizer.1", "ALLOW_ATTR")
	MoveSetting("12", "markup.sanitizer", "REGEXP", "markup.sanitizer.1", "REGEXP")

	PurgeSettings("14", "log", "MACARON", "REDIRECT_MACARON_LOG")

	MoveSetting("15", "indexer", "ISSUE_INDEXER_QUEUE_TYPE", "queue.issue_indexer", "TYPE")
	MoveSetting("15", "indexer", "ISSUE_INDEXER_QUEUE_DIR", "queue.issue_indexer", "DATADIR")
	MoveSetting("15", "indexer", "ISSUE_INDEXER_QUEUE_CONN_STR", "queue.issue_indexer", "CONN_STR")
	MoveSetting("15", "indexer", "ISSUE_INDEXER_QUEUE_BATCH_NUMBER", "queue.issue_indexer", "BATCH_LENGTH")
	MoveSetting("15", "indexer", "UPDATE_BUFFER_LEN", "queue.issue_indexer", "LENGTH")

	MoveSettingInSection("17", "cron.archive_cleanup", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.update_mirrors", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.repo_health_check", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.check_repo_stats", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.update_migration_poster_id", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.sync_external_users", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.deleted_branches_cleanup", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.delete_inactive_accounts", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.delete_repo_archives", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.git_gc_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.resync_all_sshkeys", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.resync_all_hooks", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.reinit_missing_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.delete_missing_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.delete_generated_repository_avatars", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("17", "cron.delete_old_actions", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")

	DeprecateSettingsForRemoval("18", "U2F", "APP_ID")
	MoveSettingsToDB("18", "picture", "ENABLE_FEDERATED_AVATAR", "DISABLE_GRAVATAR")
	DeprecateSettingSameSection("18", "mailer", "HOST","SMTP_ADDR+SMTP_PORT")
	DeprecateSettingSameSection("18", "mailer", "MAILER_TYPE","PROTOCOL")
	DeprecateSettingSameSection("18", "mailer", "IS_TLS_ENABLED","PROTOCOL")
	DeprecateSettingSameSection("18", "mailer", "DISABLE_HELO","ENABLE_HELO")
	DeprecateSettingSameSection("18", "mailer", "SKIP_VERIFY","FORCE_TRUST_SERVER_CERT")
	DeprecateSettingSameSection("18", "mailer", "USE_CERTIFICATE","USE_CLIENT_CERT")
	DeprecateSettingSameSection("18", "mailer", "CERT_FILE","CLIENT_CERT_FILE")
	DeprecateSettingSameSection("18", "mailer", "KEY_FILE","CLIENT_KEY_FILE")

	DeprecateSettingsForRemoval("19", "ui", "ONLY_SHOW_RELEVANT_REPOS")
}
