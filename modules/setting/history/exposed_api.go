// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package history

// This file contains the methods that will be exposed to other packages
import (
	"code.gitea.io/gitea/modules/log"
	ini "gopkg.in/ini.v1"
)


var removedSettings map[settingsSource]map[string][]*historyEntry // ordered by old source and then by old section (for performance)

func removeSetting(entry *historyEntry) {
	source := entry.oldValue.Source()
	section := entry.oldValue.Section()
	sections := removedSettings[source]
	entriesInSection := sections[section]
	entriesInSection = append(entriesInSection, entry)
	sections[section] = entriesInSection
}

// Adds a notice that the given setting under "[section].key" has been replaced by "[replacementSection].replacementKey" inside the ini configuration file
// Everything should be exactly like it is in the configuration file
func MoveIniSetting(version, section, key, replacementSection, replacementKey string) {
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
// Everything should be exactly like it is in the app.ini
func MoveIniSettingInSection(version, section, key, replacementKey string) {
	MoveIniSetting(version, section, key, section, replacementKey)
}

// Adds a notice that the given settings under "[section].key(s)" have been removed without any replacement
// Everything should be exactly like it is in the app.ini
func PurgeIniSettings(version, section string, keys ...string) {
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
func MoveIniSettingsToDB(version, section string, keys ...string) {
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
// Pass action to specify what will be done when an invalid setting has been found. Defaults to "log.Error(template, args)"
//
func PrintRemovedSettings(cfg *ini.File, action ...func(template string, args ...string) error) error {

	onInvalid := func(template string, args ...string) error {
		log.Error(template, args)
		return nil
	}
	if len(action) > 0 {
		onInvalid = action[0]
	}

	return printRemovedIniSettings(cfg, onInvalid) // At the moment, there are only breaking changes in the ini configurations, will probably be adapted in the future
}

func printRemovedIniSettings(cfg *ini.File, action func(template string, args ...string) error) error {
	iniChanges := removedSettings[settingsSourceINI]
	for sectionName, removedList := range iniChanges {
		section, err := cfg.GetSection(sectionName)
		if err != nil {
			return err
		}
		if section == nil {
			continue
		}
		for _, removed := range removedList {
			if section.HasKey(removed.oldValue.Key()) {
				action(removed.getTemplateLogMessage(), removed.oldValue.String(), string(removed.oldValue.Source()), removed.happensIn.String(), removed.newValue.String(), string(removed.newValue.Source()))
			}
		}
	}
	return nil
}
