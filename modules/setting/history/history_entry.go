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
		return "Please use the new value %[4]s in the %[5]s instead."
	case typeMovedFromIniToDB:
		return "Please use the key %[4]s in the %[5]s instead. The current value will be/has been copied to it."
	default:
		panic("Unimplemented history event type: " + strconv.Itoa(int(e.event)))
	}
}

// getTemplateLogMessage returns an unformatted log message for this history entry.
// The returned template accepts the following commands:
// - %[1]s: old settings value ([section].key)
// - %[2]s: old setting source (ini, db, …)
// - %[3]s: gitea version of the change (1.19.0)
// - %[4]s: new settings value
// - %[5]s: new setting source
func (e *historyEntry) getTemplateLogMessage() string {
	return "The setting %[1]s in %[2]s is no longer used since Gitea %[3]s. " + e.getReplacementHint()
}
