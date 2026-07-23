// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package manager

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	"gitea.dev/codespace-proto-go/codespace/v1/codespacev1connect"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	codespace_service "gitea.dev/services/codespace"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestManagerServiceProtocolAuthenticationAndRegistration(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	client, cleanup := newManagerTestClient(t)
	defer cleanup()

	_, err := client.RegisterManager(t.Context(), connect.NewRequest(&codespacev1.RegisterManagerRequest{
		ProtocolVersion:   0,
		RegistrationToken: "missing",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "protocol_mismatch", failureCategory(t, err))

	_, err = client.RegisterManager(t.Context(), connect.NewRequest(&codespacev1.RegisterManagerRequest{
		ProtocolVersion:   1,
		RegistrationToken: "missing",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	assert.Equal(t, "unauthenticated", failureCategory(t, err))

	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerToken{
		Token:   "registration-token",
		OwnerID: 0,
	}))
	registered, err := client.RegisterManager(t.Context(), connect.NewRequest(&codespacev1.RegisterManagerRequest{
		ProtocolVersion:   1,
		RegistrationToken: "registration-token",
	}))
	require.NoError(t, err)
	require.Positive(t, registered.Msg.GetManagerId())
	require.Len(t, registered.Msg.GetManagerSecret(), 64)

	declaration := &codespacev1.DeclareManagerRequest{
		ProtocolVersion:                    1,
		GatewayUrl:                         "https://WorkSpace.EXAMPLE.com:443/",
		GatewaySshAddr:                     "WorkSpace.EXAMPLE.com:0022",
		Tags:                               []string{"Default", "incus", "default"},
		Version:                            " 0.1.0 ",
		Name:                               " manager-one ",
		ManagerRuntimeState:                codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE,
		GatewaySshHostKeyAlgorithm:         " ssh-ed25519 ",
		GatewaySshHostKeyFingerprintSha256: " SHA256:test ",
		GatewaySshHostKeyUpdatedUnix:       1,
		CapacityTotal:                      4,
		CapacityAvailable:                  3,
	}
	_, err = client.DeclareManager(t.Context(), managerRequest(registered.Msg.GetManagerId(), "bad-secret", declaration))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	assert.Equal(t, "unauthenticated", failureCategory(t, err))

	_, err = client.DeclareManager(t.Context(), managerRequest(registered.Msg.GetManagerId()+1000, registered.Msg.GetManagerSecret(), declaration))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	assert.Equal(t, "manager_unregistered", failureCategory(t, err))

	declaration.ProtocolVersion = 0
	_, err = client.DeclareManager(t.Context(), managerRequest(registered.Msg.GetManagerId(), registered.Msg.GetManagerSecret(), declaration))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "protocol_mismatch", failureCategory(t, err))

	declaration.ProtocolVersion = 1
	declared, err := client.DeclareManager(t.Context(), managerRequest(registered.Msg.GetManagerId(), registered.Msg.GetManagerSecret(), declaration))
	require.NoError(t, err)
	assert.Positive(t, declared.Msg.GetHeartbeatIntervalMilliseconds())
	assert.Positive(t, declared.Msg.GetRuntimeMetadataRefreshIntervalMilliseconds())
	assert.Positive(t, declared.Msg.GetControlPlaneMaxMessageSizeBytes())
	assert.NotEmpty(t, declared.Msg.GetGiteaWebUrl())

	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(t.Context()).ID(registered.Msg.GetManagerId()).Get(manager)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, "manager-one", manager.Name)
	assert.JSONEq(t, `["default","incus"]`, manager.TagsJSON)
	assert.JSONEq(t, `{
		"version": "0.1.0",
		"gateway_ssh_host_key_algorithm": "ssh-ed25519",
		"gateway_ssh_host_key_fingerprint_sha256": "SHA256:test",
		"gateway_ssh_host_key_updated_unix": 1,
		"last_capacity_total": 4,
		"last_capacity_available": 3
	}`, manager.MetaJSON)

	count, err := db.GetEngine(t.Context()).Where("manager_id = ?", manager.ID).Count(new(codespace_model.ManagerAddress))
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
	addresses := make([]*codespace_model.ManagerAddress, 0, 2)
	require.NoError(t, db.GetEngine(t.Context()).Where("manager_id = ?", manager.ID).Find(&addresses))
	addressByKind := map[string]string{}
	for _, address := range addresses {
		addressByKind[address.Kind] = address.Address
	}
	assert.Equal(t, "https://workspace.example.com", addressByKind[codespace_model.ManagerAddressGateway])
	assert.Equal(t, "workspace.example.com:22", addressByKind[codespace_model.ManagerAddressSSH])
}

func TestManagerServiceRejectsProtocolMismatchForAllRPCs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	client, cleanup := newManagerTestClient(t)
	defer cleanup()
	manager, secret := insertManagerTestIdentity(t, 7)
	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerToken{
		Token:   "protocol-token",
		OwnerID: 0,
	}))
	codespaceUUID := "81818181-8181-4818-8818-818181818181"

	cases := []struct {
		name string
		call func(protocolVersion int32) error
	}{
		{
			name: "RegisterManager",
			call: func(protocolVersion int32) error {
				_, err := client.RegisterManager(t.Context(), connect.NewRequest(&codespacev1.RegisterManagerRequest{
					ProtocolVersion:   protocolVersion,
					RegistrationToken: "protocol-token",
				}))
				return err
			},
		},
		{
			name: "DeclareManager",
			call: func(protocolVersion int32) error {
				request := managerTestDeclaration("https://protocol.example.com", "protocol.example.com:22")
				request.ProtocolVersion = protocolVersion
				request.Name = "should-not-write"
				_, err := client.DeclareManager(t.Context(), managerRequest(manager.ID, secret, request))
				return err
			},
		},
		{
			name: "FetchOperations",
			call: func(protocolVersion int32) error {
				_, err := client.FetchOperations(t.Context(), managerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
					ProtocolVersion: protocolVersion,
					MaxOperations:   1,
				}))
				return err
			},
		},
		{
			name: "ReportInstances",
			call: func(protocolVersion int32) error {
				_, err := client.ReportInstances(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ReportInstancesRequest{
					ProtocolVersion:     protocolVersion,
					InventoryGeneration: 8,
					Instances: []*codespacev1.RuntimeInstanceRef{{
						CodespaceUuid: codespaceUUID,
						RuntimeState:  codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
					}},
				}))
				return err
			},
		},
		{
			name: "FinalizeOperation",
			call: func(protocolVersion int32) error {
				_, err := client.FinalizeOperation(t.Context(), managerRequest(manager.ID, secret, &codespacev1.FinalizeOperationRequest{
					ProtocolVersion:   protocolVersion,
					CodespaceUuid:     codespaceUUID,
					OperationRversion: 1,
					Final: &codespacev1.FinalResult{
						Status:        codespacev1.FinalStatus_FINAL_STATUS_DONE,
						OperationType: codespacev1.OperationType_OPERATION_TYPE_CREATE,
					},
				}))
				return err
			},
		},
		{
			name: "UpdateLog",
			call: func(protocolVersion int32) error {
				_, err := client.UpdateLog(t.Context(), managerRequest(manager.ID, secret, &codespacev1.UpdateLogRequest{
					ProtocolVersion:   protocolVersion,
					CodespaceUuid:     codespaceUUID,
					OperationRversion: 1,
				}))
				return err
			},
		},
		{
			name: "ReportRuntimeMetadata",
			call: func(protocolVersion int32) error {
				_, err := client.ReportRuntimeMetadata(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
					ProtocolVersion:    protocolVersion,
					CodespaceUuid:      codespaceUUID,
					MetadataJson:       "{}",
					MetadataGeneration: 1,
				}))
				return err
			},
		},
		{
			name: "ReportRuntimeTransition",
			call: func(protocolVersion int32) error {
				_, err := client.ReportRuntimeTransition(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ReportRuntimeTransitionRequest{
					ProtocolVersion:           protocolVersion,
					CodespaceUuid:             codespaceUUID,
					RuntimeGeneration:         1,
					ObservedOperationRversion: 1,
					RuntimeState:              codespacev1.RuntimeState_RUNTIME_STATE_STOPPED,
				}))
				return err
			},
		},
		{
			name: "RequestGiteaToken",
			call: func(protocolVersion int32) error {
				_, err := client.RequestGiteaToken(t.Context(), managerRequest(manager.ID, secret, &codespacev1.RequestGiteaTokenRequest{
					ProtocolVersion: protocolVersion,
					CodespaceUuid:   codespaceUUID,
				}))
				return err
			},
		},
		{
			name: "EnsureCodespaceGitSSHKey",
			call: func(protocolVersion int32) error {
				_, err := client.EnsureCodespaceGitSSHKey(t.Context(), managerRequest(manager.ID, secret, &codespacev1.EnsureCodespaceGitSSHKeyRequest{
					ProtocolVersion: protocolVersion,
					CodespaceUuid:   codespaceUUID,
					PublicKey:       []byte("not-a-key"),
				}))
				return err
			},
		},
		{
			name: "RequestIdleStop",
			call: func(protocolVersion int32) error {
				_, err := client.RequestIdleStop(t.Context(), managerRequest(manager.ID, secret, &codespacev1.RequestIdleStopRequest{
					ProtocolVersion:               protocolVersion,
					CodespaceUuid:                 codespaceUUID,
					ObservedAutoStopEnabled:       true,
					ObservedIdleTimeoutSeconds:    1800,
					ObservedInteractionGeneration: 1,
				}))
				return err
			},
		},
		{
			name: "ValidatePublicEndpoint",
			call: func(protocolVersion int32) error {
				_, err := client.ValidatePublicEndpoint(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ValidatePublicEndpointRequest{
					ProtocolVersion: protocolVersion,
					CodespaceUuid:   codespaceUUID,
					EndpointId:      "web",
				}))
				return err
			},
		},
		{
			name: "ValidateOpenToken",
			call: func(protocolVersion int32) error {
				_, err := client.ValidateOpenToken(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ValidateOpenTokenRequest{
					ProtocolVersion: protocolVersion,
					Code:            "open-code",
				}))
				return err
			},
		},
		{
			name: "VerifySSHPublicKey",
			call: func(protocolVersion int32) error {
				_, err := client.VerifySSHPublicKey(t.Context(), managerRequest(manager.ID, secret, &codespacev1.VerifySSHPublicKeyRequest{
					ProtocolVersion: protocolVersion,
					CodespaceUuid:   codespaceUUID,
					PublicKey:       []byte("not-a-key"),
				}))
				return err
			},
		},
		{
			name: "RevalidateGatewaySession",
			call: func(protocolVersion int32) error {
				_, err := client.RevalidateGatewaySession(t.Context(), managerRequest(manager.ID, secret, &codespacev1.RevalidateGatewaySessionRequest{
					ProtocolVersion: protocolVersion,
					Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{
						Endpoint: &codespacev1.EndpointSessionBinding{
							UserId:        1,
							CodespaceUuid: codespaceUUID,
							EndpointId:    "workspace",
						},
					},
				}))
				return err
			},
		},
	}

	for _, protocolVersion := range []int32{0, -1, 2} {
		for _, testCase := range cases {
			t.Run(testCase.name+"/"+strconv.FormatInt(int64(protocolVersion), 10), func(t *testing.T) {
				err := testCase.call(protocolVersion)
				require.Error(t, err)
				assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
				assert.Equal(t, "protocol_mismatch", failureCategory(t, err))
			})
		}
	}

	current := new(codespace_model.Manager)
	has, err := db.GetEngine(t.Context()).ID(manager.ID).Get(current)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, manager.Name, current.Name)
	assert.Equal(t, manager.MetaJSON, current.MetaJSON)
	assert.Equal(t, manager.InventoryGeneration, current.InventoryGeneration)
	count, err := db.GetEngine(t.Context()).Count(new(codespace_model.Manager))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestManagerServiceRequestProtocolVersionFieldNumbers(t *testing.T) {
	requests := []proto.Message{
		&codespacev1.RegisterManagerRequest{},
		&codespacev1.DeclareManagerRequest{},
		&codespacev1.FetchOperationsRequest{},
		&codespacev1.ReportInstancesRequest{},
		&codespacev1.FinalizeOperationRequest{},
		&codespacev1.UpdateLogRequest{},
		&codespacev1.ReportRuntimeMetadataRequest{},
		&codespacev1.ReportRuntimeTransitionRequest{},
		&codespacev1.RequestGiteaTokenRequest{},
		&codespacev1.EnsureCodespaceGitSSHKeyRequest{},
		&codespacev1.RequestIdleStopRequest{},
		&codespacev1.ValidatePublicEndpointRequest{},
		&codespacev1.ValidateOpenTokenRequest{},
		&codespacev1.VerifySSHPublicKeyRequest{},
		&codespacev1.RevalidateGatewaySessionRequest{},
	}
	for _, request := range requests {
		t.Run(string(request.ProtoReflect().Descriptor().FullName()), func(t *testing.T) {
			fields := request.ProtoReflect().Descriptor().Fields()
			protocolField := fields.ByName("protocol_version")
			require.NotNil(t, protocolField)
			assert.EqualValues(t, 1, protocolField.Number())
		})
	}
}

func TestManagerServiceDeclareAddressConflicts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	client, cleanup := newManagerTestClient(t)
	defer cleanup()

	firstManager, firstSecret := insertManagerTestIdentity(t, 0)
	secondManager, secondSecret := insertManagerTestIdentity(t, 0)

	firstDeclaration := managerTestDeclaration("https://workspace.example.com", "workspace.example.com:22")
	_, err := client.DeclareManager(t.Context(), managerRequest(firstManager.ID, firstSecret, firstDeclaration))
	require.NoError(t, err)

	_, err = client.DeclareManager(t.Context(), managerRequest(secondManager.ID, secondSecret,
		managerTestDeclaration("https://workspace.example.com", "other-ssh.example.com:22")))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "gateway_url_conflict", failureCategory(t, err))

	_, err = client.DeclareManager(t.Context(), managerRequest(secondManager.ID, secondSecret,
		managerTestDeclaration("https://other-gateway.example.com", "workspace.example.com:22")))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "gateway_ssh_addr_conflict", failureCategory(t, err))

	count, err := db.GetEngine(t.Context()).Where("manager_id = ?", secondManager.ID).Count(new(codespace_model.ManagerAddress))
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestManagerServiceDeclareCookieScopeConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	client, cleanup := newManagerTestClient(t)
	defer cleanup()
	t.Cleanup(test.MockVariableValue(&setting.AppURL, "https://gitea.example.com/"))

	manager, secret := insertManagerTestIdentity(t, 0)

	_, err := client.DeclareManager(t.Context(), managerRequest(manager.ID, secret,
		managerTestDeclaration("https://workspace.example.com", "workspace.example.com:22")))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "gateway_cookie_scope_conflict", failureCategory(t, err))

	count, err := db.GetEngine(t.Context()).Where("manager_id = ?", manager.ID).Count(new(codespace_model.ManagerAddress))
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestManagerServiceFetchPayloadAndLease(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Repository.DisableHTTPGit, false))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Disabled, false))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Domain, "localhost"))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Port, 22))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, false))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, []string{
		"localhost ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf",
	}))
	manager, secret := insertManagerTestIdentity(t, 0)
	client, cleanup := newManagerTestClient(t)
	defer cleanup()

	codespaceUUID := "11111111-2222-4111-8111-111111111111"
	insertManagerTestCodespace(t, 0, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     41,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusQueued,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		InteractionGeneration: 7,
	})

	fetched, err := client.FetchOperations(t.Context(), managerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
		ProtocolVersion:   1,
		CapacityAvailable: 1,
		AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
			codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE,
		},
		MaxOperations: 1,
	}))
	require.NoError(t, err)
	require.Len(t, fetched.Msg.GetOperations(), 1)
	operation := fetched.Msg.GetOperations()[0]
	assert.Equal(t, codespaceUUID, operation.GetCodespaceUuid())
	assert.EqualValues(t, 41, operation.GetOperationRversion())
	assert.Positive(t, operation.GetLeaseValidForMilliseconds())
	require.NotNil(t, operation.GetCreate())
	assert.EqualValues(t, 2, operation.GetCreate().GetRepoId())
	assert.NotEmpty(t, operation.GetCreate().GetRepoCloneHttpUrl())
	assert.NotEmpty(t, operation.GetCreate().GetRepoCloneSshUrl())
	assert.Equal(t, codespacev1.GitProtocol_GIT_PROTOCOL_HTTP, operation.GetCreate().GetGitProtocol())
	assert.EqualValues(t, 7, operation.GetCreate().GetRuntimeSettings().GetInteractionGeneration())

	renewed, err := client.FetchOperations(t.Context(), managerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
		ProtocolVersion: 1,
		MaxOperations:   1,
		ObservedOperations: []*codespacev1.ObservedOperation{{
			CodespaceUuid:     codespaceUUID,
			OperationRversion: 41,
		}},
	}))
	require.NoError(t, err)
	assert.Empty(t, renewed.Msg.GetOperations())
	require.Len(t, renewed.Msg.GetRenewedLeases(), 1)
	assert.Equal(t, codespaceUUID, renewed.Msg.GetRenewedLeases()[0].GetCodespaceUuid())
}

func TestManagerServiceStructuredErrorDetails(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	manager, secret := insertManagerTestIdentity(t, 3)
	client, cleanup := newManagerTestClient(t)
	defer cleanup()

	_, err := client.ReportInstances(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ReportInstancesRequest{
		ProtocolVersion:     1,
		InventoryGeneration: 2,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "stale_generation", failureCategory(t, err))
	assert.EqualValues(t, 3, staleGenerationCurrent(t, err))

	codespaceUUID := "90909090-9090-4909-8909-909090909090"
	insertManagerTestCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     25,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	written, err := client.UpdateLog(t.Context(), managerRequest(manager.ID, secret, &codespacev1.UpdateLogRequest{
		ProtocolVersion:   1,
		CodespaceUuid:     codespaceUUID,
		OperationRversion: 25,
		Lines: []*codespacev1.LogLine{{
			TimestampUnixNano: time.Now().UnixNano(),
			Message:           "first",
		}},
	}))
	require.NoError(t, err)
	require.Positive(t, written.Msg.GetNextOffset())

	_, err = client.UpdateLog(t.Context(), managerRequest(manager.ID, secret, &codespacev1.UpdateLogRequest{
		ProtocolVersion:   1,
		CodespaceUuid:     codespaceUUID,
		OperationRversion: 25,
		Offset:            written.Msg.GetNextOffset() + 1,
		Lines: []*codespacev1.LogLine{{
			TimestampUnixNano: time.Now().UnixNano(),
			Message:           "gap",
		}},
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeAborted, connect.CodeOf(err))
	assert.Equal(t, "offset_gap", failureCategory(t, err))
	assert.Equal(t, written.Msg.GetNextOffset(), logOffsetCurrent(t, err))
}

func TestManagerServiceManagerOfflineCategory(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	manager, secret := insertManagerTestIdentity(t, 0)
	manager.RuntimeState = codespace_model.ManagerRuntimeStateRecovering
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("runtime_state").Update(manager)
	require.NoError(t, err)
	client, cleanup := newManagerTestClient(t)
	defer cleanup()

	_, err = client.FetchOperations(t.Context(), managerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
		ProtocolVersion: 1,
		MaxOperations:   1,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	assert.Equal(t, "manager_offline", failureCategory(t, err))

	_, err = client.RequestIdleStop(t.Context(), managerRequest(manager.ID, secret, &codespacev1.RequestIdleStopRequest{
		ProtocolVersion:               1,
		CodespaceUuid:                 "93939393-9393-4939-8939-939393939393",
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	assert.Equal(t, "manager_offline", failureCategory(t, err))

	offlineManager, offlineSecret := insertManagerTestIdentity(t, 0)
	offlineManager.RuntimeState = ""
	_, err = db.GetEngine(t.Context()).ID(offlineManager.ID).Cols("runtime_state").Update(offlineManager)
	require.NoError(t, err)

	_, err = client.ReportRuntimeMetadata(t.Context(), managerRequest(offlineManager.ID, offlineSecret, &codespacev1.ReportRuntimeMetadataRequest{
		ProtocolVersion:    1,
		CodespaceUuid:      "93939393-9393-4939-8939-939393939393",
		MetadataGeneration: 1,
		MetadataJson:       "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	assert.Equal(t, "manager_offline", failureCategory(t, err))

	_, err = client.RequestGiteaToken(t.Context(), managerRequest(offlineManager.ID, offlineSecret, &codespacev1.RequestGiteaTokenRequest{
		ProtocolVersion: 1,
		CodespaceUuid:   "93939393-9393-4939-8939-939393939393",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	assert.Equal(t, "manager_offline", failureCategory(t, err))

	_, err = client.EnsureCodespaceGitSSHKey(t.Context(), managerRequest(offlineManager.ID, offlineSecret, &codespacev1.EnsureCodespaceGitSSHKeyRequest{
		ProtocolVersion: 1,
		CodespaceUuid:   "93939393-9393-4939-8939-939393939393",
		PublicKey:       []byte("not-a-key"),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	assert.Equal(t, "manager_offline", failureCategory(t, err))
}

func TestManagerServiceReportRuntimeMetadataVersionExhausted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	manager, secret := insertManagerTestIdentity(t, 0)
	client, cleanup := newManagerTestClient(t)
	defer cleanup()

	codespaceUUID := "94949494-9494-4949-8949-949494949494"
	insertManagerTestCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     31,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	_, err := client.ReportRuntimeMetadata(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
		ProtocolVersion:    1,
		CodespaceUuid:      codespaceUUID,
		MetadataGeneration: math.MaxInt64,
		MetadataJson:       managerTestRuntimeMetadataJSON(31),
	}))
	require.NoError(t, err)

	_, err = client.ReportRuntimeMetadata(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
		ProtocolVersion:    1,
		CodespaceUuid:      codespaceUUID,
		MetadataGeneration: math.MaxInt64 - 1,
		MetadataJson:       managerTestRuntimeMetadataJSON(31),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "version_exhausted", failureCategory(t, err))
}

func TestManagerServiceReportRuntimeMetadataDisabled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	manager, secret := insertManagerTestIdentity(t, 0)
	client, cleanup := newManagerTestClient(t)
	defer cleanup()

	_, err := client.ReportRuntimeMetadata(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
		ProtocolVersion:    1,
		CodespaceUuid:      "93939393-9393-4939-8939-939393939393",
		MetadataGeneration: 1,
		MetadataJson:       "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "state_unavailable", failureCategory(t, err))
}

func TestManagerRequestConversions(t *testing.T) {
	assert.Equal(t, codespace_model.ManagerRuntimeStateOnline, managerRuntimeState(codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE))
	assert.Empty(t, managerRuntimeState(codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_UNSPECIFIED))

	status, err := finalStatus(codespacev1.FinalStatus_FINAL_STATUS_DONE)
	require.NoError(t, err)
	assert.Equal(t, codespace_service.FinalStatusDone, status)
	_, err = finalStatus(codespacev1.FinalStatus_FINAL_STATUS_UNSPECIFIED)
	require.Error(t, err)

	types, err := acceptedOperationTypes([]codespacev1.AcceptedOperationType{
		codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE,
		codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{codespace_model.OperationCreate, codespace_model.OperationResume}, types)
	_, err = acceptedOperationTypes([]codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_UNSPECIFIED})
	require.Error(t, err)

	endpoint, err := revalidateGatewaySessionOptions(&codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{Endpoint: &codespacev1.EndpointSessionBinding{
			UserId: 1, CodespaceUuid: "uuid", EndpointId: "workspace",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_service.RevalidateSessionEndpoint, endpoint.Kind)
	assert.Equal(t, "workspace", endpoint.EndpointID)

	ssh, err := revalidateGatewaySessionOptions(&codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Ssh{Ssh: &codespacev1.SSHSessionBinding{
			UserId: 1, CodespaceUuid: "uuid",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_service.RevalidateSessionSSH, ssh.Kind)
	_, err = revalidateGatewaySessionOptions(&codespacev1.RevalidateGatewaySessionRequest{})
	require.Error(t, err)
}

func TestManagerOperationAndInventoryResponses(t *testing.T) {
	operation, err := operationPayload(codespace_service.OperationPayload{
		Command: codespace_model.OperationCreate,
		Create: &codespace_service.CreateOperationPayload{
			GitProtocol: codespace_model.GitProtocolSSH,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, codespacev1.GitProtocol_GIT_PROTOCOL_SSH, operation.GetCreate().GetGitProtocol())

	for _, command := range []string{codespace_model.OperationResume, codespace_model.OperationStop, codespace_model.OperationDelete, codespace_service.OperationCommandAbortCreate, codespace_service.OperationCommandAbortResume} {
		payload := codespace_service.OperationPayload{Command: command}
		if command == codespace_model.OperationResume {
			payload.Resume = &codespace_service.ResumeOperationPayload{GitProtocol: codespace_model.GitProtocolHTTP}
		}
		converted, convertErr := operationPayload(payload)
		require.NoError(t, convertErr)
		assert.NotNil(t, converted.GetCommand())
	}
	_, err = operationPayload(codespace_service.OperationPayload{Command: "invalid"})
	require.Error(t, err)

	inventory, err := reportInstancesResponse(&codespace_service.ReportInstancesResult{Results: []codespace_service.RuntimeInstanceResult{
		{CodespaceUUID: "cleanup", Action: codespace_service.InventoryActionCleanupLocalRuntime},
		{CodespaceUUID: "clear", Action: codespace_service.InventoryActionClearOperationContext, CurrentOperationRVersion: 4},
	}})
	require.NoError(t, err)
	require.Len(t, inventory.GetResults(), 2)
	assert.NotNil(t, inventory.GetResults()[0].GetCleanupLocalRuntime())
	assert.EqualValues(t, 4, inventory.GetResults()[1].GetClearOperationContext().GetCurrentOperationRversion())
}

func TestManagerOutcomeResponses(t *testing.T) {
	idle, err := requestIdleStopResponse(&codespace_service.RequestIdleStopResult{
		Outcome:           codespace_service.IdleStopOutcomePending,
		OperationRVersion: 8,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 8, idle.GetPending().GetOperationRversion())

	idle, err = requestIdleStopResponse(&codespace_service.RequestIdleStopResult{
		Outcome: codespace_service.IdleStopOutcomeObservationChanged,
		RuntimeSettings: codespace_service.RuntimeSettings{
			AutoStopEnabled: true, IdleTimeoutSeconds: 600, InteractionGeneration: 4,
		},
	})
	require.NoError(t, err)
	assert.EqualValues(t, 4, idle.GetObservationChanged().GetRuntimeSettings().GetInteractionGeneration())

	idle, err = requestIdleStopResponse(&codespace_service.RequestIdleStopResult{
		Outcome:             codespace_service.IdleStopOutcomeNotApplicable,
		NotApplicableReason: codespace_service.IdleStopReasonAlreadyStopped,
	})
	require.NoError(t, err)
	assert.Equal(t, codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_ALREADY_STOPPED, idle.GetNotApplicable().GetReason())

	assert.NotNil(t, finalizeResponse(codespace_service.FinalizeOutcomeAccepted).GetFinalAccepted())
	assert.NotNil(t, finalizeResponse(codespace_service.FinalizeOutcomeIdempotent).GetIdempotentDone())
	assert.NotNil(t, finalizeResponse(codespace_service.FinalizeOutcomeResourceAbsent).GetResourceAbsent())
	assert.NotNil(t, finalizeResponse(codespace_service.FinalizeOutcomeStale).GetStaleOperation())
}

func newManagerTestClient(t *testing.T) (codespacev1connect.ManagerServiceClient, func()) {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.AppURL, "http://127.0.0.1:3000/"))
	t.Cleanup(test.MockVariableValue(&setting.SessionConfig.Domain, ""))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GatewayRequireHTTPS, false))
	path, handler := NewManagerServiceHandler()
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	return codespacev1connect.NewManagerServiceClient(server.Client(), server.URL), server.Close
}

func managerRequest[T any](managerID int64, managerSecret string, message *T) *connect.Request[T] {
	request := connect.NewRequest(message)
	request.Header().Set(managerIDHeader, strconv.FormatInt(managerID, 10))
	request.Header().Set(managerSecretHeader, managerSecret)
	return request
}

func managerTestDeclaration(gatewayURL, gatewaySSHAddr string) *codespacev1.DeclareManagerRequest {
	return &codespacev1.DeclareManagerRequest{
		ProtocolVersion:                    1,
		GatewayUrl:                         gatewayURL,
		GatewaySshAddr:                     gatewaySSHAddr,
		Tags:                               []string{"default"},
		Version:                            "0.1.0",
		Name:                               "manager",
		ManagerRuntimeState:                codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE,
		GatewaySshHostKeyAlgorithm:         "ssh-ed25519",
		GatewaySshHostKeyFingerprintSha256: "SHA256:test",
		GatewaySshHostKeyUpdatedUnix:       1,
		CapacityTotal:                      4,
		CapacityAvailable:                  3,
	}
}

func insertManagerTestIdentity(t *testing.T, inventoryGeneration int64) (*codespace_model.Manager, string) {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:                "manager",
		RuntimeState:        codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:            `["default"]`,
		LastOnlineUnix:      time.Now().Unix(),
		InventoryGeneration: inventoryGeneration,
		CreatedUnix:         time.Now().Unix(),
		MetaJSON:            "{}",
	}
	secret := manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))
	return manager, secret
}

func insertManagerTestCodespace(t *testing.T, managerID int64, codespace *codespace_model.Codespace) {
	t.Helper()
	codespace.UserID = 1
	codespace.RepoID = 2
	codespace.ManagerID = managerID
	codespace.RefType = "branch"
	codespace.RefName = "main"
	codespace.RepoTag = "default"
	if codespace.GitProtocol == "" {
		codespace.GitProtocol = codespace_model.GitProtocolHTTP
	}
	codespace.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	codespace.AutoStopMode = codespace_model.AutoStopModeDefault
	codespace.CreatedUnix = 1
	codespace.UpdatedUnix = 1
	codespace.LogFilename = codespace.UUID + ".log"
	require.NoError(t, db.Insert(t.Context(), codespace))
}

func failureCategory(t *testing.T, err error) string {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		require.NoError(t, detailErr)
		if failure, ok := value.(*codespacev1.FailureDetail); ok {
			return failure.GetCategory()
		}
	}
	require.FailNow(t, "missing failure detail")
	return ""
}

func staleGenerationCurrent(t *testing.T, err error) int64 {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		require.NoError(t, detailErr)
		if stale, ok := value.(*codespacev1.StaleGenerationDetail); ok {
			return stale.GetCurrentGeneration()
		}
	}
	require.FailNow(t, "missing stale generation detail")
	return 0
}

func logOffsetCurrent(t *testing.T, err error) int64 {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		require.NoError(t, detailErr)
		if offset, ok := value.(*codespacev1.LogOffsetDetail); ok {
			return offset.GetCurrentOffset()
		}
	}
	require.FailNow(t, "missing log offset detail")
	return 0
}

func managerTestRuntimeMetadataJSON(operationRVersion int64) string {
	return `{"endpoints":[],"boot":{"operation_rversion":` +
		strconv.FormatInt(operationRVersion, 10) +
		`,"stage":"ready","started_unix":100,"last_update_unix":101}}`
}
