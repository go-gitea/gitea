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
		Token:  "registration-token",
		UserID: 0,
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
		Environments:                       []*codespacev1.EnvironmentTag{{Tag: "Default", Description: "Default environment"}, {Tag: "incus"}},
		Version:                            " 0.1.0 ",
		Name:                               " manager-one ",
		ManagerRuntimeState:                codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE,
		GatewaySshHostKeyAlgorithm:         " ssh-ed25519 ",
		GatewaySshHostKeyFingerprintSha256: " SHA256:test ",
		GatewaySshHostKeyUpdatedUnix:       1,
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
	assert.JSONEq(t, `[{"tag":"default","description":"Default environment"},{"tag":"incus"}]`, manager.TagsJSON)
	assert.Equal(t, "0.1.0", manager.Version)
	assert.Equal(t, "ssh-ed25519", manager.GatewaySSHHostKeyAlgorithm)
	assert.Equal(t, "SHA256:test", manager.GatewaySSHHostKeyFingerprintSHA256)
	assert.EqualValues(t, 1, manager.GatewaySSHHostKeyUpdatedUnix)

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
		&codespacev1.RequestRuntimeAccessRequest{},
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

func TestManagerServiceDeclareAcceptsCookieScopeWarning(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	client, cleanup := newManagerTestClient(t)
	defer cleanup()
	t.Cleanup(test.MockVariableValue(&setting.AppURL, "https://gitea.example.com/"))

	manager, secret := insertManagerTestIdentity(t, 0)

	_, err := client.DeclareManager(t.Context(), managerRequest(manager.ID, secret,
		managerTestDeclaration("https://workspace.example.com", "workspace.example.com:22")))
	require.NoError(t, err)

	count, err := db.GetEngine(t.Context()).Where("manager_id = ?", manager.ID).Count(new(codespace_model.ManagerAddress))
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
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
		ProtocolVersion:          1,
		StartupCapacityAvailable: 1,
		AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
			codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE,
		},
		AcceptedCreateTags: []string{"default"},
	}))
	require.NoError(t, err)
	require.Len(t, fetched.Msg.GetOperations(), 1)
	operation := fetched.Msg.GetOperations()[0]
	assert.Equal(t, codespaceUUID, operation.GetCodespaceUuid())
	assert.EqualValues(t, 41, operation.GetOperationRversion())
	assert.Positive(t, operation.GetLeaseValidForMilliseconds())
	require.NotNil(t, operation.GetCreate())
	assert.NotEmpty(t, operation.GetCreate().GetRepoCloneHttpUrl())
	assert.NotEmpty(t, operation.GetCreate().GetRepoCloneSshUrl())
	assert.Equal(t, codespacev1.GitProtocol_GIT_PROTOCOL_HTTP, operation.GetCreate().GetGitProtocol())
	assert.EqualValues(t, 7, operation.GetCreate().GetRuntimeSettings().GetInteractionGeneration())

	renewed, err := client.FetchOperations(t.Context(), managerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
		ProtocolVersion: 1,
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
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	assert.Equal(t, "manager_offline", failureCategory(t, err))

	_, err = client.RequestIdleStop(t.Context(), managerRequest(manager.ID, secret, &codespacev1.RequestIdleStopRequest{
		ProtocolVersion: 1,
		CodespaceUuid:   "93939393-9393-4939-8939-939393939393",
		ObservedSettings: &codespacev1.EffectiveCodespaceRuntimeSettings{
			AutoStopEnabled:    true,
			IdleTimeoutSeconds: 1800,
		},
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
		Metadata:           managerTestRuntimeMetadata(1),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	assert.Equal(t, "manager_offline", failureCategory(t, err))

	_, err = client.RequestRuntimeAccess(t.Context(), managerRequest(offlineManager.ID, offlineSecret, &codespacev1.RequestRuntimeAccessRequest{
		ProtocolVersion:   1,
		CodespaceUuid:     "93939393-9393-4939-8939-939393939393",
		OperationRversion: 1,
		GitSshPublicKey:   []byte("not-a-key"),
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
		Metadata:           managerTestRuntimeMetadata(31),
	}))
	require.NoError(t, err)

	_, err = client.ReportRuntimeMetadata(t.Context(), managerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
		ProtocolVersion:    1,
		CodespaceUuid:      codespaceUUID,
		MetadataGeneration: math.MaxInt64 - 1,
		Metadata:           managerTestRuntimeMetadata(31),
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
		Metadata:           managerTestRuntimeMetadata(1),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, "state_unavailable", failureCategory(t, err))
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
		Environments:                       []*codespacev1.EnvironmentTag{{Tag: "default"}},
		Version:                            "0.1.0",
		Name:                               "manager",
		ManagerRuntimeState:                codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE,
		GatewaySshHostKeyAlgorithm:         "ssh-ed25519",
		GatewaySshHostKeyFingerprintSha256: "SHA256:test",
		GatewaySshHostKeyUpdatedUnix:       1,
	}
}

func insertManagerTestIdentity(t *testing.T, inventoryGeneration int64) (*codespace_model.Manager, string) {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:                "manager",
		RuntimeState:        codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:            `[{"tag":"default"}]`,
		LastOnlineUnix:      time.Now().Unix(),
		InventoryGeneration: inventoryGeneration,
		CreatedUnix:         time.Now().Unix(),
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
	codespace.EnvironmentTag = "default"
	codespace.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	codespace.DevContainerDefaultImage = "mcr.microsoft.com/devcontainers/base:ubuntu"
	codespace.AutoStopMode = codespace_model.AutoStopModeDefault
	codespace.CreatedUnix = 1
	codespace.UpdatedUnix = 1
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

func managerTestRuntimeMetadata(operationRVersion int64) *codespacev1.RuntimeMetadata {
	return &codespacev1.RuntimeMetadata{
		Endpoints: []*codespacev1.RuntimeEndpoint{{EndpointId: "workspace", Label: "Workspace"}},
		Boot: &codespacev1.RuntimeBoot{
			OperationRversion: operationRVersion,
			Stage:             codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY,
			StartedUnix:       100,
			LastUpdateUnix:    101,
		},
		ResourceUsage: &codespacev1.RuntimeResourceUsage{
			Cpu:          &codespacev1.RuntimeCPUUsage{UsedMillicores: 125, LimitMillicores: 1000},
			Memory:       &codespacev1.RuntimeMemoryUsage{UsedBytes: 256 * 1024 * 1024, LimitBytes: 1024 * 1024 * 1024},
			Disk:         &codespacev1.RuntimeDiskUsage{UsedBytes: 512 * 1024 * 1024, LimitBytes: 10 * 1024 * 1024 * 1024},
			ObservedUnix: 101,
		},
	}
}
