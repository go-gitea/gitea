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
		{Tag: " Standard ", Description: " General development "},
		{Tag: "gpu"},
	})
	require.NoError(t, err)
	assert.Equal(t, []ManagerEnvironmentDeclaration{
		{Tag: "standard", Description: "General development"},
		{Tag: "gpu"},
	}, environments)

	_, err = normalizeManagerEnvironments([]*codespacev1.EnvironmentTag{{Tag: "standard"}, {Tag: "STANDARD"}})
	require.Error(t, err)
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
	insertServiceManagerGatewayAddress(t, manager, "https://workspace.example.com")
	invalidManager := insertServiceManager(t)
	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerAddress{
		ManagerID: invalidManager.ID,
		Kind:      codespace_model.ManagerAddressGateway,
		Address:   "http://127.0.0.1:18081",
	}))

	require.NoError(t, WarnManagerGatewayAddressConflicts(t.Context()))

	t.Cleanup(test.MockVariableValue(&setting.AppURL, "https://gitea.example.com/"))
	require.NoError(t, WarnManagerGatewayAddressConflicts(t.Context()))
}

func TestBindRuntimeIdentityAssignsManagerRuntimeUUID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		Status:            codespace_model.StatusCreating,
		OperationRVersion: 3,
		OperationType:     codespace_model.OperationCreate,
		OperationStatus:   codespace_model.OperationStatusRunning,
	})
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(t.Context()).Where("manager_id = ? AND uuid = ?", manager.ID, "").Get(codespace)
	require.NoError(t, err)
	require.True(t, has)

	runtimeUUID := "34343434-3434-4434-8434-343434343434"
	boundUUID, err := BindRuntimeIdentity(t.Context(), manager, BindRuntimeIdentityOptions{
		CodespaceID:       codespace.ID,
		OperationRVersion: 3,
		RuntimeUUID:       runtimeUUID,
	})
	require.NoError(t, err)
	assert.Equal(t, runtimeUUID, boundUUID)
	assert.Equal(t, runtimeUUID, loadServiceCodespace(t, runtimeUUID).UUID)

	boundUUID, err = BindRuntimeIdentity(t.Context(), manager, BindRuntimeIdentityOptions{
		CodespaceID:       codespace.ID,
		OperationRVersion: 3,
		RuntimeUUID:       runtimeUUID,
	})
	require.NoError(t, err)
	assert.Equal(t, runtimeUUID, boundUUID)

	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		Status:            codespace_model.StatusCreating,
		OperationRVersion: 4,
		OperationType:     codespace_model.OperationCreate,
		OperationStatus:   codespace_model.OperationStatusRunning,
	})
	otherCodespace := new(codespace_model.Codespace)
	has, err = db.GetEngine(t.Context()).Where("manager_id = ? AND uuid = ? AND id <> ?", manager.ID, "", codespace.ID).Get(otherCodespace)
	require.NoError(t, err)
	require.True(t, has)
	_, err = BindRuntimeIdentity(t.Context(), manager, BindRuntimeIdentityOptions{
		CodespaceID:       otherCodespace.ID,
		OperationRVersion: 4,
		RuntimeUUID:       runtimeUUID,
	})
	require.ErrorIs(t, err, ErrBindRuntimeIdentityConflict)
}

func TestBindRuntimeIdentityRejectsInvalidOperationState(t *testing.T) {
	runtimeUUID := "45454545-4545-4454-8454-454545454545"

	for _, tc := range []struct {
		name     string
		setup    func(*testing.T, *codespace_model.Manager) (*codespace_model.Codespace, *codespace_model.Manager, int64)
		expected error
	}{
		{
			name: "wrong manager",
			setup: func(t *testing.T, manager *codespace_model.Manager) (*codespace_model.Codespace, *codespace_model.Manager, int64) {
				otherManager := insertServiceManager(t)
				codespace := insertRuntimeIdentityTarget(t, manager, "")
				return codespace, otherManager, codespace.OperationRVersion
			},
			expected: ErrBindRuntimeIdentityNotFound,
		},
		{
			name: "wrong operation version",
			setup: func(t *testing.T, manager *codespace_model.Manager) (*codespace_model.Codespace, *codespace_model.Manager, int64) {
				codespace := insertRuntimeIdentityTarget(t, manager, "")
				return codespace, manager, codespace.OperationRVersion + 1
			},
			expected: ErrBindRuntimeIdentityNotFound,
		},
		{
			name: "queued operation",
			setup: func(t *testing.T, manager *codespace_model.Manager) (*codespace_model.Codespace, *codespace_model.Manager, int64) {
				codespace := insertRuntimeIdentityTarget(t, manager, "")
				codespace.OperationStatus = codespace_model.OperationStatusQueued
				_, err := db.GetEngine(t.Context()).ID(codespace.ID).Cols("operation_status").Update(codespace)
				require.NoError(t, err)
				return codespace, manager, codespace.OperationRVersion
			},
			expected: ErrBindRuntimeIdentityNotFound,
		},
		{
			name: "stop operation",
			setup: func(t *testing.T, manager *codespace_model.Manager) (*codespace_model.Codespace, *codespace_model.Manager, int64) {
				codespace := insertRuntimeIdentityTarget(t, manager, "")
				codespace.OperationType = codespace_model.OperationStop
				_, err := db.GetEngine(t.Context()).ID(codespace.ID).Cols("operation_type").Update(codespace)
				require.NoError(t, err)
				return codespace, manager, codespace.OperationRVersion
			},
			expected: ErrBindRuntimeIdentityNotFound,
		},
		{
			name: "already bound",
			setup: func(t *testing.T, manager *codespace_model.Manager) (*codespace_model.Codespace, *codespace_model.Manager, int64) {
				codespace := insertRuntimeIdentityTarget(t, manager, "56565656-5656-4656-8656-565656565656")
				return codespace, manager, codespace.OperationRVersion
			},
			expected: ErrBindRuntimeIdentityStateConflict,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			manager := insertServiceManager(t)
			codespace, bindManager, operationRVersion := tc.setup(t, manager)

			_, err := BindRuntimeIdentity(t.Context(), bindManager, BindRuntimeIdentityOptions{
				CodespaceID:       codespace.ID,
				OperationRVersion: operationRVersion,
				RuntimeUUID:       runtimeUUID,
			})
			require.ErrorIs(t, err, tc.expected)
		})
	}
}

func insertRuntimeIdentityTarget(t *testing.T, manager *codespace_model.Manager, runtimeUUID string) *codespace_model.Codespace {
	t.Helper()

	codespace := &codespace_model.Codespace{
		UUID:              runtimeUUID,
		Status:            codespace_model.StatusCreating,
		OperationRVersion: 3,
		OperationType:     codespace_model.OperationCreate,
		OperationStatus:   codespace_model.OperationStatusRunning,
	}
	insertServiceCodespace(t, manager.ID, codespace)
	return codespace
}

func TestCodespaceInitSkipsGatewayAddressValidationWhenDisabled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	mockGatewayScopeSettings(t, "https://gitea.example.com/", "", false)
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	manager := insertServiceManager(t)
	insertServiceManagerGatewayAddress(t, manager, "https://workspace.example.com")

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

func TestValidateCodespaceConfigAndTimings(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&setting.Codespace.ControlPlaneTimeout, 10*time.Second))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.ManagerOfflineTimeout, 80*time.Second))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.OperationLeaseTimeout, 1500*time.Millisecond))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.OperationMaxDuration, 3*time.Hour))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.QueueTimeout, 7*time.Minute))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.LogMaxSize, int64(32*1024*1024)))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopDefaultTimeout, 25*time.Minute))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopMinTimeout, 3*time.Minute))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopMaxTimeout, 24*time.Hour))

	require.NoError(t, ValidateCodespaceConfig())
	heartbeatMillis, metadataRefreshMillis, maxMessageBytes, _ := ManagerServiceTimings()
	assert.EqualValues(t, 20_000, heartbeatMillis)
	assert.EqualValues(t, 40_000, metadataRefreshMillis)
	assert.EqualValues(t, 32*1024*1024, maxMessageBytes)

	t.Cleanup(test.MockVariableValue(&setting.Codespace.ControlPlaneTimeout, 21*time.Second))
	require.ErrorContains(t, ValidateCodespaceConfig(), "CONTROL_PLANE_TIMEOUT")
}

func TestDeclareManagerRejectsDeletedManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	require.NoError(t, DeclareManager(t.Context(), manager, validDeclareManagerOptions()))
	assertServiceExists(t, new(codespace_model.ManagerAddress), "manager_id = ?", manager.ID)

	require.NoError(t, DeleteManager(t.Context(), DeleteManagerOptions{
		Scope:     ManagerSettingsScopeSite,
		ManagerID: manager.ID,
		Confirm:   true,
	}))

	err := DeclareManager(t.Context(), manager, validDeclareManagerOptions())
	require.ErrorIs(t, err, ErrManagerUnregistered)
	assertServiceNotExists(t, new(codespace_model.ManagerAddress), "manager_id = ?", manager.ID)
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
		GatewaySSHHostKeyUpdatedUnix:       1,
	}
}

func mockGatewayScopeSettings(t *testing.T, appURL, sessionDomain string, requireHTTPS bool) {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.AppURL, appURL))
	t.Cleanup(test.MockVariableValue(&setting.SessionConfig.Domain, sessionDomain))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GatewayRequireHTTPS, requireHTTPS))
}
