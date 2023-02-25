// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package history

// This file contains the methods that will be exposed to other packages
import (
	"code.gitea.io/gitea/modules/log"
	ini "gopkg.in/ini.v1"
)

var removedSettings map[string][]historyEntry // ordered by section (for performance)

func removeSetting(entry *historyEntry) {
	section := entry.oldValue.Section()
	sectionList := removedSettings[section]
	sectionList = append(sectionList, *entry)
	removedSettings[section] = sectionList
}

// Adds a notice that the given setting under "[section].key" has been replaced by "[replacementSection].replacementKey"
// everything should be exactly like it is in the app.ini
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
				log.Error(removed.getTemplateLogMessage(), removed.oldValue.String(), removed.getTense(), removed.happensIn)
			}
		}
	}
	return nil
}

