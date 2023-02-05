// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"

	"code.gitea.io/gitea/modules/log"
	ini "gopkg.in/ini.v1"
)

type settingExistType int

const (
	settingRemoved settingExistType = iota
	settingDeprecatedInIni
	settingMovedToDB
)

type removedSetting struct {
	version            string // Gitea version that removed it / will remove it
	section            string // Corresponding section in the app.ini without the parentheses
	key                string // Exact key in the app.ini, so should be uppercased
	replacementSection string
	replacementKey     string
	existType          settingExistType // In what form does this setting still exist?
}

// getReplacementHint returns a hint about how to replace this setting.
// The return value can be printed to the logs.
func (r *removedSetting) getReplacementHint() string {
	if r.existType == settingMovedToDB {
		return "This setting has been copied and moved to the database table 'sys_setting' under the key " + toDBSection(r.section, r.key)
	} else if r.replacementKey != "" && r.replacementSection != "" {
		return "Please replace this setting with " + toIniSection(r.replacementSection, r.replacementKey)
	} else {
		return "This setting has no documented replacement"
	}
}

// getTense returns the correct tense of "is" for this removed setting
func (r *removedSetting) getTense() string {
	if r.existType != settingRemoved {
		return "will be"
	}
	return "was"
}
func (r *removedSetting) validate() {
	if !strings.HasPrefix(r.version, "v") {
		r.version = "v" + r.version
	}
	r.section = strings.ToLower(r.section)
	r.replacementSection = strings.ToLower(r.replacementSection)
	if r.existType == settingMovedToDB {
		r.replacementKey = strings.ToLower(r.replacementKey)
		r.key = strings.ToLower(r.key)
	} else if r.existType == settingDeprecatedInIni {
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

func addRemovedSetting(setting *removedSetting) {
	setting.validate()
	// Append the setting at the corresponding entry
	sectionList := removedSettings[setting.section]
	sectionList = append(sectionList, *setting)
	removedSettings[setting.section] = sectionList
}

// Adds the given setting under "[section].key" to the removed settings
// "key" and "replacementKey" should be uppercased so that they are exactly like in the app.ini
func AddRemovedSetting(version, section, key, replacementSection, replacementKey string) {
	addRemovedSetting(&removedSetting{
		version:            version,
		section:            section,
		key:                key,
		replacementSection: replacementSection,
		replacementKey:     replacementKey,
	})
}

// Adds the given setting under "[section].key" to the removed settings
func AddRemovedSettingWithoutReplacement(version, section, key string) {
	addRemovedSetting(&removedSetting{
		version: version,
		section: section,
		key:     key,
	})
}

// Deprecates the given (still accepted and existing) setting under "[section].key" for removal
func AddDeprecatedSetting(version, section, key, replacementSection, replacementKey string) {
	addRemovedSetting(&removedSetting{
		version:            version,
		section:            section,
		key:                key,
		replacementSection: replacementSection,
		replacementKey:     replacementKey,
		existType:          settingDeprecatedInIni,
	})
}

// Deprecates the given (still accepted and existing) setting under "[section].key" for removal
// "key" should be uppercased so that it is exactly like in the app.ini
func AddDeprecatedSettingWithoutReplacement(version, section, key string) {
	addRemovedSetting(&removedSetting{
		version:   version,
		section:   section,
		key:       key,
		existType: settingDeprecatedInIni,
	})
}

// Marks this setting as moved to the database.
func AddDBSettingWarning(version, section, key string) {
	addRemovedSetting(&removedSetting{
		version:   version,
		section:   section,
		key:       key,
		existType: settingMovedToDB,
	})
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
				log.Error("Support for the setting %s in your config file %s removed in %s. %s.", toIniSection(removed.section, removed.key), removed.getTense(), removed.version, removed.getReplacementHint())
			}
		}
	}
}

