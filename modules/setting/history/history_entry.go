// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package history

import (
	"strconv"

	version "github.com/hashicorp/go-version"
)

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
