// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"strings"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDeclareManagerOptions(t *testing.T) {
	mockGatewayScopeSettings(t, "http://127.0.0.1:3000/", "", false)

	opts := validDeclareManagerOptions()
	opts.GatewayURL = "https://WorkSpace.EXAMPLE.com:443/"
	opts.GatewaySSHAddr = "WorkSpace.EXAMPLE.com:0022"
	opts.Version = " 1.0.0 "
	opts.GatewaySSHHostKeyAlgorithm = " ssh-ed25519 "
	opts.GatewaySSHHostKeyFingerprintSHA256 = " SHA256:abc "

	normalized, err := normalizeDeclareManagerOptions(opts)
	require.NoError(t, err)
	assert.Equal(t, "https://workspace.example.com", normalized.GatewayURL)
	assert.Equal(t, "workspace.example.com:22", normalized.GatewaySSHAddr)
	assert.Equal(t, "1.0.0", normalized.Version)
	assert.Equal(t, "ssh-ed25519", normalized.GatewaySSHHostKeyAlgorithm)
	assert.Equal(t, "SHA256:abc", normalized.GatewaySSHHostKeyFingerprintSHA256)
}

func TestNormalizeManagerEnvironments(t *testing.T) {
	environments, err := normalizeManagerEnvironments([]*codespacev1.EnvironmentTag{
		{Tag: "standard", Description: " General development "},
		{Tag: "gpu"},
	})
	require.NoError(t, err)
	assert.Equal(t, []ManagerEnvironmentDeclaration{
		{Tag: "standard", Description: "General development"},
		{Tag: "gpu"},
	}, environments)

	for _, tag := range []string{" Standard ", "STANDARD", "standard_gpu", "standard.gpu", strings.Repeat("a", 64)} {
		_, err = normalizeManagerEnvironments([]*codespacev1.EnvironmentTag{{Tag: tag}})
		require.Error(t, err, tag)
	}
}

func TestNormalizeDeclareManagerOptionsRejectsInvalidFields(t *testing.T) {
	mockGatewayScopeSettings(t, "http://127.0.0.1:3000/", "", false)

	for _, tc := range []struct {
		name   string
		mutate func(*DeclareManagerOptions)
	}{
		{
			name: "gateway url ip literal",
			mutate: func(opts *DeclareManagerOptions) {
				opts.GatewayURL = "https://127.0.0.1"
			},
		},
		{
			name: "gateway url trailing dot",
			mutate: func(opts *DeclareManagerOptions) {
				opts.GatewayURL = "https://workspace.example.com."
			},
		},
		{
			name: "ssh address missing port",
			mutate: func(opts *DeclareManagerOptions) {
				opts.GatewaySSHAddr = "workspace.example.com"
			},
		},
		{
			name: "invalid host key fingerprint",
			mutate: func(opts *DeclareManagerOptions) {
				opts.GatewaySSHHostKeyFingerprintSHA256 = "MD5:abc"
			},
		},
		{
			name: "version too long",
			mutate: func(opts *DeclareManagerOptions) {
				opts.Version = strings.Repeat("v", 65)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := validDeclareManagerOptions()
			tc.mutate(&opts)

			_, err := normalizeDeclareManagerOptions(opts)
			require.Error(t, err)
		})
	}
}

func TestDiagnoseGatewayCookieScope(t *testing.T) {
	for _, tc := range []struct {
		name           string
		appURL         string
		sessionDomain  string
		gatewayURL     string
		expectConflict bool
	}{
		{
			name:           "same registrable domain",
			appURL:         "https://gitea.example.com/",
			gatewayURL:     "https://workspace.example.com",
			expectConflict: true,
		},
		{
			name:           "gitea host under gateway domain",
			appURL:         "https://gitea.workspace.example.net/",
			gatewayURL:     "https://workspace.example.net",
			expectConflict: true,
		},
		{
			name:           "gitea ip literal",
			appURL:         "http://127.0.0.1:3000/",
			gatewayURL:     "https://workspace.example.com",
			expectConflict: false,
		},
		{
			name:           "session cookie domain",
			appURL:         "http://127.0.0.1:3000/",
			sessionDomain:  ".example.com",
			gatewayURL:     "https://workspace.example.com",
			expectConflict: true,
		},
		{
			name:           "separate registrable domains",
			appURL:         "https://gitea.example.org/",
			gatewayURL:     "https://workspace.example.com",
			expectConflict: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockGatewayScopeSettings(t, tc.appURL, tc.sessionDomain, false)
			opts := validDeclareManagerOptions()
			opts.GatewayURL = tc.gatewayURL

			if tc.expectConflict {
				assert.NotEmpty(t, diagnoseGatewayCookieScope(tc.gatewayURL))
			} else {
				assert.Empty(t, diagnoseGatewayCookieScope(tc.gatewayURL))
			}
			_, err := normalizeDeclareManagerOptions(opts)
			require.NoError(t, err)
		})
	}
}

func TestNormalizeDeclareManagerOptionsGatewayRequiresHTTPS(t *testing.T) {
	mockGatewayScopeSettings(t, "http://127.0.0.1:3000/", "", true)
	opts := validDeclareManagerOptions()
	opts.GatewayURL = "http://workspace.example.com"

	_, err := normalizeDeclareManagerOptions(opts)
	require.ErrorContains(t, err, "gateway url must use https")
}

func TestWarnManagerGatewayAddressConflicts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	mockGatewayScopeSettings(t, "https://gitea.example.org/", "", false)
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, true))

	manager := insertServiceManager(t)
	setServiceManagerGatewayURL(t, manager, "https://workspace.example.com")
	invalidManager := insertServiceManager(t)
	invalidManager.GatewayURL = "http://127.0.0.1:18081"
	_, err := db.GetEngine(t.Context()).ID(invalidManager.ID).Cols("gateway_url").Update(invalidManager)
	require.NoError(t, err)

	require.NoError(t, WarnManagerGatewayAddressConflicts(t.Context()))

	t.Cleanup(test.MockVariableValue(&setting.AppURL, "https://gitea.example.com/"))
	require.NoError(t, WarnManagerGatewayAddressConflicts(t.Context()))
}

func TestCodespaceInitSkipsGatewayAddressValidationWhenDisabled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	mockGatewayScopeSettings(t, "https://gitea.example.com/", "", false)
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	manager := insertServiceManager(t)
	setServiceManagerGatewayURL(t, manager, "https://workspace.example.com")

	require.NoError(t, Init(t.Context()))
}

func TestCodespaceInitAllowsHTTPWhenSSHCloneDisabled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	mockGatewayScopeSettings(t, "https://gitea.example.com/", "", false)
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, true))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitProtocol, codespace_model.GitProtocolHTTP))
	t.Cleanup(test.MockVariableValue(&setting.Repository.DisableHTTPGit, false))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Disabled, true))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, false))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, []string(nil)))

	require.NoError(t, Init(t.Context()))
}

func TestCodespaceInitRequiresSSHForSSHPreferred(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	mockGatewayScopeSettings(t, "https://gitea.example.com/", "", false)
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, true))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitProtocol, codespace_model.GitProtocolSSH))
	t.Cleanup(test.MockVariableValue(&setting.Repository.DisableHTTPGit, false))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Disabled, true))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, false))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, []string(nil)))

	err := Init(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "[server] DISABLE_SSH=true")
}

func TestValidateCodespaceConfig(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&setting.Codespace.OperationMaxDuration, time.Minute))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.LogMaxSize, int64(32*1024*1024)))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopDefaultTimeout, 25*time.Minute))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopMinTimeout, 3*time.Minute))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopMaxTimeout, 24*time.Hour))

	require.NoError(t, ValidateCodespaceConfig())
	for _, duration := range []time.Duration{0, -time.Second, 1500 * time.Millisecond} {
		setting.Codespace.OperationMaxDuration = duration
		require.ErrorContains(t, ValidateCodespaceConfig(), "OPERATION_MAX_DURATION")
	}
}

func TestDeclareManagerRejectsDeletedManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	require.NoError(t, DeclareManager(t.Context(), manager, validDeclareManagerOptions()))
	assert.NotEmpty(t, loadServiceManager(t, manager.ID).GatewayURL)

	require.NoError(t, DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeSite,
		ManagerID: manager.ID,
		Confirm:   true,
	}))

	err := DeclareManager(t.Context(), manager, validDeclareManagerOptions())
	require.ErrorIs(t, err, ErrManagerUnregistered)
	assertServiceNotExists(t, new(codespace_model.Manager), "id = ?", manager.ID)
}

func validDeclareManagerOptions() DeclareManagerOptions {
	return DeclareManagerOptions{
		GatewayURL:                         "https://workspace.example.com",
		GatewaySSHAddr:                     "workspace.example.com:22",
		Environments:                       []*codespacev1.EnvironmentTag{{Tag: "default"}},
		Version:                            "1.0.0",
		RuntimeState:                       codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE,
		GatewaySSHHostKeyAlgorithm:         "ssh-ed25519",
		GatewaySSHHostKeyFingerprintSHA256: "SHA256:abc",
	}
}

func mockGatewayScopeSettings(t *testing.T, appURL, sessionDomain string, requireHTTPS bool) {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.AppURL, appURL))
	t.Cleanup(test.MockVariableValue(&setting.SessionConfig.Domain, sessionDomain))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GatewayRequireHTTPS, requireHTTPS))
}
