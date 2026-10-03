// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"
	"time"

	"gitea.dev/modules/log"
)

type AuditRecordOutput string

const (
	AuditRecordOutputDisabled AuditRecordOutput = "disabled"
	AuditRecordOutputDatabase AuditRecordOutput = "database"
)

var Audit = struct {
	RecordOutput  AuditRecordOutput `ini:"RECORD_OUTPUT"`
	RetentionDays int64             `ini:"RETENTION_DAYS"`
}{
	RecordOutput:  AuditRecordOutputDisabled,
	RetentionDays: 30,
}

func loadAuditFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "audit", &Audit)

	Audit.RecordOutput = AuditRecordOutput(strings.ToLower(strings.TrimSpace(string(Audit.RecordOutput))))
	switch Audit.RecordOutput {
	case "":
		Audit.RecordOutput = AuditRecordOutputDisabled
	case AuditRecordOutputDisabled, AuditRecordOutputDatabase:
	default:
		log.Error("Invalid [audit].RECORD_OUTPUT %q, audit records are disabled", Audit.RecordOutput)
		Audit.RecordOutput = AuditRecordOutputDisabled
	}

	if Audit.RetentionDays < 0 {
		Audit.RetentionDays = 0 // keep forever
	}
}

// AuditRetentionPeriod is the age at which recorded events are pruned, zero meaning they are kept forever.
func AuditRetentionPeriod() time.Duration {
	return time.Duration(Audit.RetentionDays) * 24 * time.Hour
}

// AuditRecordEnabled reports whether audit events are recorded at all.
func AuditRecordEnabled() bool {
	return Audit.RecordOutput != AuditRecordOutputDisabled
}
