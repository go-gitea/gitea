// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAuditFrom(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cfg      string
		expected AuditRecordOutput
	}{
		{name: "DisabledByDefault", cfg: "", expected: AuditRecordOutputDisabled},
		{name: "Database", cfg: "[audit]\nRECORD_OUTPUT = Database\n", expected: AuditRecordOutputDatabase},
		{name: "Empty", cfg: "[audit]\nRECORD_OUTPUT =\n", expected: AuditRecordOutputDisabled},
		{name: "Invalid", cfg: "[audit]\nRECORD_OUTPUT = nonsense\n", expected: AuditRecordOutputDisabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer test.MockVariableValue(&Audit)()

			cfg, err := NewConfigProviderFromData(tc.cfg)
			require.NoError(t, err)
			loadAuditFrom(cfg)

			assert.Equal(t, tc.expected, Audit.RecordOutput)
			assert.Equal(t, tc.expected != AuditRecordOutputDisabled, AuditRecordEnabled())
		})
	}
}
