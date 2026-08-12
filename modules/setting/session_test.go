// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestSessionRedisSharedConnFallback(t *testing.T) {
	defer test.MockVariableValue(&SessionConfig)()
	defer test.MockVariableValue(&Redis)()

	// ProviderConfig is shadowed into a JSON blob at the end of loadSessionFrom, so it is matched
	// by substring rather than compared with field
	providerConfig := func(want, missing string) func(t *testing.T) {
		return func(t *testing.T) {
			assert.Contains(t, SessionConfig.ProviderConfig, want)
			if missing != "" {
				assert.NotContains(t, SessionConfig.ProviderConfig, missing)
			}
		}
	}

	testConfigLoad(t, []any{loadRedisFrom, loadSessionFrom}, []configTestCase{
		{
			name:  "redis provider with empty PROVIDER_CONFIG falls back to shared [redis]",
			ini:   "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[session]\nPROVIDER = redis",
			check: providerConfig("redis://127.0.0.1:6379/0", ""),
		},
		{
			name:  "session PROVIDER_CONFIG wins over shared [redis]",
			ini:   "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[session]\nPROVIDER = redis\nPROVIDER_CONFIG = redis://10.0.0.1:6379/1",
			check: providerConfig("redis://10.0.0.1:6379/1", "127.0.0.1"),
		},
		{
			name:  "no shared [redis]",
			ini:   "[session]\nPROVIDER = redis",
			check: providerConfig("", "redis://"),
		},
		{
			name:  "file provider default path is untouched by shared [redis]",
			ini:   "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[session]\nPROVIDER = file",
			check: providerConfig("sessions", "redis://"),
		},
	})
}
