// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package history

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	version "github.com/hashicorp/go-version"
	ini "gopkg.in/ini.v1"
)

var versionCache map[string]*version.Version // Multiple settings share the same version, so cache it instead of always creating a new version

func getVersion(stringVersion string) *version.Version {
	if _, ok := versionCache[stringVersion]; !ok {
		versionCache[stringVersion] = version.Must(version.NewVersion(stringVersion))
	}
	return versionCache[stringVersion]
}

var currentGiteaVersion = getVersion("1.19")

type setting interface {
	String() string // Returns a string representation of the given setting that can be found like that in the given settings source
	Section() string //  If a type doesn't support sectioning, use "" as the global scope
 // The un-normalized section this setting belongs to as passed in the init method below.
 // There might be settings that don't conform with the normalization (which is why they are replaced)
	Key() string // The un-normalized key of this setting
	IsNormalized() bool
	Normalize()
}

type iniSetting struct {
	section, key                     string
	normalizedSection, normalizedKey string
	isNormalized                     bool
}

var _ setting = &iniSetting{}

func (s *iniSetting) Normalize() {
	if s.IsNormalized() {
		return
	}
	s.normalizedSection = strings.ToLower(s.section)
	s.normalizedKey = strings.ToUpper(s.key)
	s.isNormalized = true
}

func (s *iniSetting) IsNormalized() bool {
	return s.isNormalized
}

func (s *iniSetting) String() string {
	s.Normalize()
	return "[" + s.normalizedSection + "]." + s.normalizedKey
}

func (s *iniSetting) Section() string {
	s.Normalize()
	return s.section
}

func (s *iniSetting) Key() string {
	s.Normalize()
	return s.key
}

type dbSetting struct {
	section, key string
	normalizedSection, normalizedKey string
	isNormalized bool
}

var _ setting = &dbSetting{}

func (s *dbSetting) Normalize() {
	if s.IsNormalized() {
		return
	}
	s.normalizedSection = strings.ToLower(s.section)
	s.normalizedKey = strings.ToLower(s.key)
	s.isNormalized = true
}

func (s *dbSetting) IsNormalized() bool {
	return s.isNormalized
}

func (s *dbSetting) String() string {
	s.Normalize()
	return s.section + "." + s.key
}

func (s *dbSetting) Section() string {
	s.Normalize()
	return s.section
}

func (s *dbSetting) Key() string {
	s.Normalize()
	return s.key
}

type eventType int

const (
	typeRemoved eventType = iota
	typeMovedFromIniToIni
	typeMovedFromIniToDB
)

type historyEntry struct {
	happensIn *version.Version // Gitea version that removed it / will remove it
	oldValue  setting
	newValue  setting // nil means removed without replacement
	event     eventType
}

func (e *historyEntry) getReplacementHint() string {
	switch e.event {
	case typeRemoved:
		return "It has no documented replacement."
	case typeMovedFromIniToIni:
		return "Please use the new value %[5]s instead."
	case typeMovedFromIniToDB:
		return "Please use the key %[5]s in the database table 'system_setting' instead. The current value will be/has been copied to it."
	default:
		panic("Unimplemented history event type: " + strconv.Itoa(int(e.event)))
	}
}

// getTemplateLogMessage returns an unformated log message for this setting.
// The returned template accepts the following commands:
// - %[1]s: old settings value
// - %[2]s: setting source
// - %[3]s: correct tense of "is"
// - %[4]s: gitea version
// -
func (e *historyEntry) getTemplateLogMessage() string {
	return "The setting %[1]s in %[2]s is no longer used since Gitea %[3]s. " + e.getReplacementHint()
}

// getTense returns the correct tense of "is" for this removed setting
func (e *historyEntry) getTense() string {
	if e.happensIn.GreaterThan(currentGiteaVersion) {
		return "will be"
	}
	return "was"
}

var removedSettings map[string][]historyEntry // ordered by section (for performance)

func removeSetting(entry *historyEntry) {
	section := entry.oldValue.Section()
	sectionList := removedSettings[section]
	sectionList = append(sectionList, *entry)
	removedSettings[section] = sectionList
}

// Adds a notice that the given setting under "[section].key" has been replaced by "[replacementSection].replacementKey"
// "key" and "replacementKey" should be exactly like they are in the app.ini
func MoveSetting(version, section, key, replacementSection, replacementKey string) {
	removeSetting(&historyEntry{
		happensIn: getVersion(version),
		oldValue: &iniSetting{
			section: section,
			key:     key,
		},
		newValue: &iniSetting{
			section: replacementSection,
			key:     replacementKey,
		},
		event: typeMovedFromIniToIni,
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
		removeSetting(&historyEntry{
			happensIn: getVersion(version),
			oldValue: &iniSetting{
				section: section,
				key:     key,
			},
			event: typeRemoved,
		})
	}
}

// Marks all given setting keys in the given section as moved to the database.
// keys should be formatted exactly like they are in the app.ini
func MoveSettingsToDB(version, section string, keys ...string) {
	for _, key := range keys {
		removeSetting(&historyEntry{
			happensIn: getVersion(version),
			oldValue: &iniSetting{
				section: section,
				key:     key,
			},
			newValue: &dbSetting{
				section: section,
				key:     key,
			},
			event: typeMovedFromIniToDB,
		})
	}
}

// Adds a warning in the logs for all settings that are still present despite not being used anymore
func PrintRemovedSettings(cfg *ini.File) error {
	for sectionName, removedList := range removedSettings {
		section, err := cfg.GetSection(sectionName)
		if err != nil {
			return err
		}
		if section == nil {
			continue
		}
		for _, removed := range removedList {
			if section.HasKey(removed.oldValue.Key()) {
				log.Error(removed.getTemplateLogMessage(),removed.oldValue.String(), removed.getTense(), removed.happensIn)
			}
		}
	}
}

// Adds all previously removed settings
// It should declare all breaking configuration changes in chronological order to ensure a monotone increasing error log
func init() {
	MoveSettingInSection("1.6", "api", "ENABLE_SWAGGER_ENDPOINT", "ENABLE_SWAGGER")

	PurgeSettings("1.9", "log.database", "LEVEL", "DRIVER", "CONN")

	MoveSetting("1.12", "markup.sanitizer", "ELEMENT", "markup.sanitizer.1", "ELEMENT")
	MoveSetting("1.12", "markup.sanitizer", "ALLOW_ATTR", "markup.sanitizer.1", "ALLOW_ATTR")
	MoveSetting("1.12", "markup.sanitizer", "REGEXP", "markup.sanitizer.1", "REGEXP")

	PurgeSettings("1.14", "log", "MACARON", "REDIRECT_MACARON_LOG")

	MoveSetting("1.15", "indexer", "ISSUE_INDEXER_QUEUE_TYPE", "queue.issue_indexer", "TYPE")
	MoveSetting("1.15", "indexer", "ISSUE_INDEXER_QUEUE_DIR", "queue.issue_indexer", "DATADIR")
	MoveSetting("1.15", "indexer", "ISSUE_INDEXER_QUEUE_CONN_STR", "queue.issue_indexer", "CONN_STR")
	MoveSetting("1.15", "indexer", "ISSUE_INDEXER_QUEUE_BATCH_NUMBER", "queue.issue_indexer", "BATCH_LENGTH")
	MoveSetting("1.15", "indexer", "UPDATE_BUFFER_LEN", "queue.issue_indexer", "LENGTH")

	MoveSettingInSection("1.17", "cron.archive_cleanup", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.update_mirrors", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.repo_health_check", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.check_repo_stats", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.update_migration_poster_id", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.sync_external_users", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.deleted_branches_cleanup", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_inactive_accounts", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_repo_archives", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.git_gc_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.resync_all_sshkeys", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.resync_all_hooks", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.reinit_missing_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_missing_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_generated_repository_avatars", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_old_actions", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")

	PurgeSettings("1.18", "U2F", "APP_ID")
	MoveSettingsToDB("1.18", "picture", "ENABLE_FEDERATED_AVATAR", "DISABLE_GRAVATAR")
	MoveSettingInSection("1.18", "mailer", "HOST", "SMTP_ADDR+SMTP_PORT")
	MoveSettingInSection("1.18", "mailer", "MAILER_TYPE", "PROTOCOL")
	MoveSettingInSection("1.18", "mailer", "IS_TLS_ENABLED", "PROTOCOL")
	MoveSettingInSection("1.18", "mailer", "DISABLE_HELO", "ENABLE_HELO")
	MoveSettingInSection("1.18", "mailer", "SKIP_VERIFY", "FORCE_TRUST_SERVER_CERT")
	MoveSettingInSection("1.18", "mailer", "USE_CERTIFICATE", "USE_CLIENT_CERT")
	MoveSettingInSection("1.18", "mailer", "CERT_FILE", "CLIENT_CERT_FILE")
	MoveSettingInSection("1.18", "mailer", "KEY_FILE", "CLIENT_KEY_FILE")

	PurgeSettings("1.19", "ui", "ONLY_SHOW_RELEVANT_REPOS")
}
