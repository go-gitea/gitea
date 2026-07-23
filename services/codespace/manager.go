// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"

	"golang.org/x/net/publicsuffix"
	"google.golang.org/protobuf/proto"
)

var (
	tagPattern                  = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	dnsLabelPattern             = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	sshHostKeyFingerprintRegexp = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]+={0,2}$`)
)

var (
	// ErrRegistrationUnauthenticated is returned when a registration token is not current.
	ErrRegistrationUnauthenticated = errors.New("manager registration unauthenticated")
	// ErrRegistrationStateUnavailable is returned when Codespace is not accepting new Manager registrations.
	ErrRegistrationStateUnavailable = errors.New("manager registration state unavailable")
	// ErrManagerUnregistered is returned when the Manager ID has no current row.
	ErrManagerUnregistered = errors.New("manager unregistered")
	// ErrManagerUnauthenticated is returned when the Manager credential is not valid.
	ErrManagerUnauthenticated = errors.New("manager unauthenticated")
	// ErrDeclareGatewayURLConflict is returned when another Manager already uses the Gateway URL.
	ErrDeclareGatewayURLConflict = errors.New("manager gateway url conflict")
	// ErrDeclareGatewaySSHAddrConflict is returned when another Manager already uses the Gateway SSH address.
	ErrDeclareGatewaySSHAddrConflict = errors.New("manager gateway ssh address conflict")
	// ErrDeclareGatewayCookieScopeConflict is returned when the Gateway domain overlaps Gitea's login cookie scope.
	ErrDeclareGatewayCookieScopeConflict = errors.New("manager gateway cookie scope conflict")
)

// DeclareManagerOptions contains the full Manager declaration accepted by Gitea.
type DeclareManagerOptions struct {
	GatewayURL                         string
	GatewaySSHAddr                     string
	Tags                               []string
	Version                            string
	Name                               string
	RuntimeState                       string
	GatewaySSHHostKeyAlgorithm         string
	GatewaySSHHostKeyFingerprintSHA256 string
	GatewaySSHHostKeyUpdatedUnix       int64
	CapacityTotal                      int32
	CapacityAvailable                  int32
}

// RegisterManager exchanges the current owner-scoped registration token for a Manager identity.
func RegisterManager(ctx context.Context, registrationToken string) (*codespace_model.Manager, string, error) {
	if !setting.Codespace.Enabled {
		return nil, "", ErrRegistrationStateUnavailable
	}
	registrationToken = strings.TrimSpace(registrationToken)
	if registrationToken == "" {
		return nil, "", ErrRegistrationUnauthenticated
	}

	token, err := loadRegistrationTokenByValue(ctx, registrationToken)
	if err != nil {
		return nil, "", err
	}
	if token == nil {
		return nil, "", ErrRegistrationUnauthenticated
	}

	var manager *codespace_model.Manager
	var secret string
	err = globallock.LockAndDo(ctx, codespaceOwnerRelationLockKey(token.OwnerID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentToken, err := loadRegistrationTokenByValue(ctx, registrationToken)
			if err != nil {
				return err
			}
			if currentToken == nil || currentToken.OwnerID != token.OwnerID {
				return ErrRegistrationUnauthenticated
			}
			if currentToken.OwnerID > 0 {
				if _, err := user_model.GetUserByID(ctx, currentToken.OwnerID); err != nil {
					if user_model.IsErrUserNotExist(err) {
						return ErrRegistrationUnauthenticated
					}
					return err
				}
			}
			manager = &codespace_model.Manager{
				OwnerID:      currentToken.OwnerID,
				RuntimeState: codespace_model.ManagerRuntimeStateRecovering,
				TagsJSON:     "[]",
				MetaJSON:     "{}",
				CreatedUnix:  time.Now().Unix(),
			}
			secret = manager.GenerateManagerSecret()
			if _, err := db.GetEngine(ctx).Insert(manager); err != nil {
				return err
			}
			return nil
		})
	})
	if err != nil {
		return nil, "", err
	}
	return manager, secret, nil
}

// AuthenticateManager verifies a Manager id and plaintext secret.
func AuthenticateManager(ctx context.Context, managerID int64, secret string) (*codespace_model.Manager, error) {
	if managerID <= 0 || strings.TrimSpace(secret) == "" {
		return nil, ErrManagerUnauthenticated
	}
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(ctx).ID(managerID).Get(manager)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrManagerUnregistered
	}
	if !manager.VerifyManagerSecret(secret) {
		return nil, ErrManagerUnauthenticated
	}
	return manager, nil
}

// DeclareManager stores the latest Manager declaration and replaces routable addresses atomically.
func DeclareManager(ctx context.Context, manager *codespace_model.Manager, opts DeclareManagerOptions) error {
	if manager == nil || manager.ID <= 0 {
		return errors.New("manager is required")
	}
	normalizedOpts, err := normalizeDeclareManagerOptions(opts)
	if err != nil {
		return err
	}
	opts = normalizedOpts

	tags, err := normalizeTags(opts.Tags)
	if err != nil {
		return err
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return fmt.Errorf("encode tags: %w", err)
	}
	metaJSON, err := json.Marshal(map[string]any{
		"version":                                 opts.Version,
		"gateway_ssh_host_key_algorithm":          opts.GatewaySSHHostKeyAlgorithm,
		"gateway_ssh_host_key_fingerprint_sha256": opts.GatewaySSHHostKeyFingerprintSHA256,
		"gateway_ssh_host_key_updated_unix":       opts.GatewaySSHHostKeyUpdatedUnix,
		"last_capacity_total":                     opts.CapacityTotal,
		"last_capacity_available":                 opts.CapacityAvailable,
	})
	if err != nil {
		return fmt.Errorf("encode manager metadata: %w", err)
	}

	return globallock.LockAndDo(ctx, fetchManagerLockKey(manager.ID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentManager := new(codespace_model.Manager)
			has, err := db.GetEngine(ctx).ID(manager.ID).Get(currentManager)
			if err != nil {
				return err
			}
			if !has {
				return ErrManagerUnregistered
			}
			if err := checkManagerAddressConflict(ctx, currentManager.ID, codespace_model.ManagerAddressGateway, opts.GatewayURL); err != nil {
				return err
			}
			if err := checkManagerAddressConflict(ctx, currentManager.ID, codespace_model.ManagerAddressSSH, opts.GatewaySSHAddr); err != nil {
				return err
			}

			now := time.Now().Unix()
			updates := &codespace_model.Manager{
				Name:           opts.Name,
				TagsJSON:       string(tagsJSON),
				RuntimeState:   opts.RuntimeState,
				LastOnlineUnix: now,
				MetaJSON:       string(metaJSON),
			}
			affected, err := db.GetEngine(ctx).ID(currentManager.ID).Cols("name", "tags_json", "runtime_state", "last_online_unix", "meta_json").Update(updates)
			if err != nil {
				return err
			}
			if affected == 0 {
				return ErrManagerUnregistered
			}
			if _, err := db.GetEngine(ctx).Where("manager_id = ?", currentManager.ID).Delete(new(codespace_model.ManagerAddress)); err != nil {
				return err
			}
			addresses := []*codespace_model.ManagerAddress{
				{ManagerID: currentManager.ID, Kind: codespace_model.ManagerAddressGateway, Address: opts.GatewayURL},
				{ManagerID: currentManager.ID, Kind: codespace_model.ManagerAddressSSH, Address: opts.GatewaySSHAddr},
			}
			if _, err := db.GetEngine(ctx).Insert(addresses); err != nil {
				return err
			}
			return nil
		})
	})
}

func checkManagerAddressConflict(ctx context.Context, managerID int64, kind, address string) error {
	existing := new(codespace_model.ManagerAddress)
	has, err := db.GetEngine(ctx).
		Where("kind = ? AND address = ? AND manager_id <> ?", kind, address, managerID).
		Get(existing)
	if err != nil || !has {
		return err
	}
	switch kind {
	case codespace_model.ManagerAddressGateway:
		return ErrDeclareGatewayURLConflict
	case codespace_model.ManagerAddressSSH:
		return ErrDeclareGatewaySSHAddrConflict
	default:
		return errors.New("manager address conflict")
	}
}

// Init validates persisted Codespace runtime entrypoint configuration during web startup.
func Init(ctx context.Context) error {
	if err := ValidateCodespaceConfig(); err != nil {
		return err
	}
	if !setting.Codespace.Enabled {
		return nil
	}
	if err := ValidateGitTransports(); err != nil {
		return err
	}
	return ValidateManagerGatewayAddresses(ctx)
}

// ValidateCodespaceConfig verifies cross-field Codespace settings before runtime entrypoints start.
func ValidateCodespaceConfig() error {
	if setting.Codespace.ControlPlaneTimeout <= 0 {
		return errors.New("[codespace] CONTROL_PLANE_TIMEOUT must be positive")
	}
	if setting.Codespace.ControlPlaneMaxSize <= 0 {
		return errors.New("[codespace] CONTROL_PLANE_MAX_MESSAGE_SIZE must be positive")
	}
	if setting.Codespace.ManagerOfflineTimeout <= 0 {
		return errors.New("[codespace] MANAGER_OFFLINE_TIMEOUT must be positive")
	}
	if setting.Codespace.OperationLeaseTimeout <= 0 {
		return errors.New("[codespace] OPERATION_LEASE_TIMEOUT must be positive")
	}
	if setting.Codespace.OperationLeaseTimeout%time.Millisecond != 0 {
		return errors.New("[codespace] OPERATION_LEASE_TIMEOUT must be a positive whole number of milliseconds")
	}
	if setting.Codespace.OperationMaxDuration <= setting.Codespace.OperationLeaseTimeout {
		return errors.New("[codespace] OPERATION_MAX_DURATION must be greater than OPERATION_LEASE_TIMEOUT")
	}
	if setting.Codespace.OperationMaxDuration%time.Second != 0 {
		return errors.New("[codespace] OPERATION_MAX_DURATION must be a positive whole number of seconds")
	}
	if setting.Codespace.ManagerOfflineTimeout%time.Second != 0 {
		return errors.New("[codespace] MANAGER_OFFLINE_TIMEOUT must be a positive whole number of seconds")
	}
	if setting.Codespace.QueueTimeout <= 0 {
		return errors.New("[codespace] QUEUE_TIMEOUT must be positive")
	}
	if setting.Codespace.QueueTimeout%time.Second != 0 {
		return errors.New("[codespace] QUEUE_TIMEOUT must be a positive whole number of seconds")
	}
	if setting.Codespace.OpenTokenExpire <= 0 || setting.Codespace.OpenTokenExpire%time.Second != 0 {
		return errors.New("[codespace] OPEN_TOKEN_EXPIRE must be a positive whole number of seconds")
	}
	if setting.Codespace.ControlPlaneTimeout > setting.Codespace.ManagerOfflineTimeout/4 {
		return errors.New("[codespace] CONTROL_PLANE_TIMEOUT must be no greater than MANAGER_OFFLINE_TIMEOUT/4")
	}
	if setting.Codespace.AutoStopMinTimeout <= 0 ||
		setting.Codespace.AutoStopDefaultTimeout <= 0 ||
		setting.Codespace.AutoStopMaxTimeout <= 0 {
		return errors.New("[codespace] AUTO_STOP timeouts must be positive")
	}
	if setting.Codespace.AutoStopMinTimeout%time.Second != 0 ||
		setting.Codespace.AutoStopDefaultTimeout%time.Second != 0 ||
		setting.Codespace.AutoStopMaxTimeout%time.Second != 0 {
		return errors.New("[codespace] AUTO_STOP timeouts must be whole seconds")
	}
	if setting.Codespace.AutoStopMinTimeout > setting.Codespace.AutoStopDefaultTimeout ||
		setting.Codespace.AutoStopDefaultTimeout > setting.Codespace.AutoStopMaxTimeout {
		return errors.New("[codespace] AUTO_STOP_MIN_TIMEOUT <= AUTO_STOP_DEFAULT_TIMEOUT <= AUTO_STOP_MAX_TIMEOUT is required")
	}
	if setting.Codespace.LogMaxSize <= 0 {
		return errors.New("[codespace] LOG_MAX_SIZE must be positive")
	}
	if LogReadMaxBytes >= setting.Codespace.LogMaxSize ||
		codespaceLogFinalSummaryReserve >= setting.Codespace.LogMaxSize {
		return errors.New("[codespace] LOG_MAX_SIZE must be greater than the internal log page and final summary reserve sizes")
	}
	if setting.Codespace.RuntimeMetadataMaxSize <= 0 {
		return errors.New("[codespace] RUNTIME_METADATA_MAX_SIZE must be positive")
	}
	minControlPlaneSize, minControlPlaneMessage := minimumControlPlaneMaxMessageSize()
	if setting.Codespace.ControlPlaneMaxSize < minControlPlaneSize {
		return fmt.Errorf("[codespace] CONTROL_PLANE_MAX_MESSAGE_SIZE=%d must be at least %d bytes for %s", setting.Codespace.ControlPlaneMaxSize, minControlPlaneSize, minControlPlaneMessage)
	}
	if setting.Codespace.CodespaceRepoConfigMaxSize <= 0 {
		return errors.New("[codespace] CODESPACE_REPO_CONFIG_MAX_SIZE must be positive")
	}
	return nil
}

type gitTransportCapabilities struct {
	HTTPEnabled  bool
	SSHEnabled   bool
	HTTPDisabled string
	SSHDisabled  string
}

// ValidateGitTransports verifies that new Codespaces have a usable Git clone entrypoint.
func ValidateGitTransports() error {
	protocol, err := createGitProtocol()
	if err != nil {
		return err
	}
	_, err = resolveGitTransportCapabilities(protocol)
	return err
}

func resolveGitTransportCapabilities(protocol string) (*gitTransportCapabilities, error) {
	capabilities := &gitTransportCapabilities{
		HTTPEnabled: true,
	}
	var unavailable []string
	if setting.Repository.DisableHTTPGit {
		capabilities.HTTPEnabled = false
		capabilities.HTTPDisabled = "[repository] DISABLE_HTTP_GIT=true"
		unavailable = append(unavailable, "http: "+capabilities.HTTPDisabled)
	}
	if disabled := gitSSHCloneDisabledReason(); disabled != "" {
		capabilities.SSHDisabled = disabled
		unavailable = append(unavailable, "ssh: "+disabled)
	} else {
		capabilities.SSHEnabled = true
	}

	if !capabilities.HTTPEnabled && !capabilities.SSHEnabled {
		return nil, fmt.Errorf("codespace git transport unavailable: %s", strings.Join(unavailable, "; "))
	}
	switch protocol {
	case codespace_model.GitProtocolHTTP:
		if !capabilities.HTTPEnabled {
			return nil, fmt.Errorf("codespace git transport unavailable: http: %s", capabilities.HTTPDisabled)
		}
	case codespace_model.GitProtocolSSH:
		if !capabilities.SSHEnabled {
			return nil, fmt.Errorf("codespace git transport unavailable: ssh: %s", capabilities.SSHDisabled)
		}
	default:
		return nil, fmt.Errorf("invalid codespace git protocol %q", protocol)
	}
	return capabilities, nil
}

// ValidateManagerGatewayAddresses validates stored Gateway base domains against the current site cookie scope.
func ValidateManagerGatewayAddresses(ctx context.Context) error {
	var addresses []*codespace_model.ManagerAddress
	if err := db.GetEngine(ctx).Where("kind = ?", codespace_model.ManagerAddressGateway).Find(&addresses); err != nil {
		return err
	}
	for _, address := range addresses {
		normalized, err := normalizeGatewayURL(address.Address)
		if err != nil {
			return fmt.Errorf("gateway_cookie_scope_conflict: manager_id=%d address=%q: %w", address.ManagerID, address.Address, err)
		}
		if err := validateGatewayCookieScope(normalized); err != nil {
			return fmt.Errorf("gateway_cookie_scope_conflict: manager_id=%d address=%q: %w", address.ManagerID, address.Address, err)
		}
	}
	return nil
}

// ManagerServiceTimings returns server-selected ManagerService control values.
func ManagerServiceTimings() (heartbeatMillis, metadataRefreshMillis, maxMessageBytes int64, giteaWebURL string) {
	return int64((setting.Codespace.ManagerOfflineTimeout / 4) / time.Millisecond),
		int64((setting.Codespace.ManagerOfflineTimeout / 2) / time.Millisecond),
		setting.Codespace.ControlPlaneMaxSize,
		setting.AppURL
}

func minimumControlPlaneMaxMessageSize() (int64, string) {
	maxString := strings.Repeat("x", 512)
	maxName := strings.Repeat("n", 255)
	maxUUID := "ffffffff-ffff-4fff-8fff-ffffffffffff"
	maxRuntimeSettings := &codespacev1.EffectiveCodespaceRuntimeSettings{
		AutoStopEnabled:       true,
		IdleTimeoutSeconds:    86_400,
		InteractionGeneration: 1<<62 - 1,
	}

	var minSize int64
	var minName string
	track := func(name string, message proto.Message) {
		if size := int64(proto.Size(message)); size > minSize {
			minSize = size
			minName = name
		}
	}

	fetchRequest := &codespacev1.FetchOperationsRequest{
		ProtocolVersion:          1,
		CapacityAvailable:        10_000,
		AcceptedOperationTypes:   []codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE, codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME},
		MaxOperations:            fetchMaxOperations,
		CleanupCapacityAvailable: 256,
		ObservedOperations:       make([]*codespacev1.ObservedOperation, 0, fetchMaxObservedOperations),
	}
	reportRequest := &codespacev1.ReportInstancesRequest{
		ProtocolVersion:     1,
		InventoryGeneration: 1<<62 - 1,
		Instances:           make([]*codespacev1.RuntimeInstanceRef, 0, fetchMaxObservedOperations),
	}
	reportResponse := &codespacev1.ReportInstancesResponse{
		Results: make([]*codespacev1.RuntimeInstanceResult, 0, fetchMaxObservedOperations),
	}
	for i := range fetchMaxObservedOperations {
		uuid := fmt.Sprintf("%08x-ffff-4fff-8fff-ffffffffffff", i)
		fetchRequest.ObservedOperations = append(fetchRequest.ObservedOperations, &codespacev1.ObservedOperation{
			CodespaceUuid:     uuid,
			OperationRversion: 1<<62 - 1,
		})
		reportRequest.Instances = append(reportRequest.Instances, &codespacev1.RuntimeInstanceRef{
			CodespaceUuid:             uuid,
			RuntimeState:              codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
			ObservedOperationRversion: 1<<62 - 1,
		})
		reportResponse.Results = append(reportResponse.Results, &codespacev1.RuntimeInstanceResult{
			CodespaceUuid:   uuid,
			RuntimeSettings: maxRuntimeSettings,
			Action: &codespacev1.RuntimeInstanceResult_StopLocalRuntime{
				StopLocalRuntime: &codespacev1.StopLocalRuntime{CurrentOperationRversion: 1<<62 - 1},
			},
		})
	}
	track("FetchOperationsRequest", fetchRequest)
	track("ReportInstancesRequest", reportRequest)
	track("ReportInstancesResponse", reportResponse)

	createOperation := &codespacev1.OperationPayload{
		OperationRversion:         1<<62 - 1,
		CodespaceUuid:             maxUUID,
		LogOffset:                 1<<62 - 1,
		LeaseValidForMilliseconds: int64(setting.Codespace.OperationLeaseTimeout / time.Millisecond),
		Command: &codespacev1.OperationPayload_Create{Create: &codespacev1.CreateOperationPayload{
			RepoId:             1<<62 - 1,
			RepoFullName:       maxString,
			RepoName:           maxName,
			RepoCloneHttpUrl:   maxString,
			RepoWebUrl:         maxString,
			OwnerId:            1<<62 - 1,
			OwnerName:          maxName,
			OwnerType:          "organization",
			OwnerDisplayName:   maxName,
			CodespaceOwnerName: maxName,
			StartRef:           maxString,
			RefType:            "branch",
			RefName:            maxString,
			CommitSha:          strings.Repeat("f", 64),
			RepoTag:            strings.Repeat("t", 64),
			RuntimeSettings:    maxRuntimeSettings,
			GitProtocol:        codespacev1.GitProtocol_GIT_PROTOCOL_SSH,
			RepoCloneSshUrl:    maxString,
		}},
	}
	fetchResponse := &codespacev1.FetchOperationsResponse{
		Operations:    make([]*codespacev1.OperationPayload, 0, fetchMaxOperations),
		RenewedLeases: make([]*codespacev1.RenewedOperationLease, 0, fetchMaxObservedOperations),
	}
	for range fetchMaxOperations {
		fetchResponse.Operations = append(fetchResponse.Operations, createOperation)
	}
	for i := range fetchMaxObservedOperations {
		fetchResponse.RenewedLeases = append(fetchResponse.RenewedLeases, &codespacev1.RenewedOperationLease{
			CodespaceUuid:             fmt.Sprintf("%08x-ffff-4fff-8fff-ffffffffffff", i),
			OperationRversion:         1<<62 - 1,
			LeaseValidForMilliseconds: int64(setting.Codespace.OperationLeaseTimeout / time.Millisecond),
		})
	}
	track("FetchOperationsResponse", fetchResponse)
	track("UpdateLogRequest", &codespacev1.UpdateLogRequest{
		ProtocolVersion:   1,
		CodespaceUuid:     maxUUID,
		OperationRversion: 1<<62 - 1,
		Offset:            1<<62 - 1,
		Lines: []*codespacev1.LogLine{{
			TimestampUnixNano: 1<<62 - 1,
			Message:           strings.Repeat("l", int(codespaceLogMaxLineSize)),
		}},
	})
	track("ReportRuntimeMetadataRequest", &codespacev1.ReportRuntimeMetadataRequest{
		ProtocolVersion:    1,
		CodespaceUuid:      maxUUID,
		MetadataJson:       strings.Repeat("m", int(setting.Codespace.RuntimeMetadataMaxSize)),
		MetadataGeneration: 1<<62 - 1,
	})
	return minSize, minName
}

func normalizeDeclareManagerOptions(opts DeclareManagerOptions) (DeclareManagerOptions, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	if opts.Name == "" {
		return opts, errors.New("manager name is required")
	}
	if len(opts.Name) > 255 {
		return opts, errors.New("manager name is too long")
	}
	opts.Version = strings.TrimSpace(opts.Version)
	if opts.Version == "" {
		return opts, errors.New("manager version is required")
	}
	if len(opts.Version) > 64 {
		return opts, errors.New("manager version is too long")
	}
	if opts.RuntimeState != codespace_model.ManagerRuntimeStateOnline && opts.RuntimeState != codespace_model.ManagerRuntimeStateRecovering {
		return opts, fmt.Errorf("invalid manager runtime state %q", opts.RuntimeState)
	}
	if opts.CapacityTotal < 1 || opts.CapacityTotal > 10000 {
		return opts, errors.New("capacity_total must be between 1 and 10000")
	}
	if opts.CapacityAvailable < 0 || opts.CapacityAvailable > opts.CapacityTotal {
		return opts, errors.New("capacity_available must be between 0 and capacity_total")
	}
	gatewayURL, err := normalizeGatewayURL(opts.GatewayURL)
	if err != nil {
		return opts, err
	}
	opts.GatewayURL = gatewayURL
	if err := validateGatewayCookieScope(opts.GatewayURL); err != nil {
		return opts, err
	}
	gatewaySSHAddr, err := normalizeGatewaySSHAddr(opts.GatewaySSHAddr)
	if err != nil {
		return opts, err
	}
	opts.GatewaySSHAddr = gatewaySSHAddr
	opts.GatewaySSHHostKeyAlgorithm = strings.TrimSpace(opts.GatewaySSHHostKeyAlgorithm)
	if opts.GatewaySSHHostKeyAlgorithm == "" {
		return opts, errors.New("gateway ssh host key algorithm is required")
	}
	if len(opts.GatewaySSHHostKeyAlgorithm) > 64 {
		return opts, errors.New("gateway ssh host key algorithm is too long")
	}
	opts.GatewaySSHHostKeyFingerprintSHA256 = strings.TrimSpace(opts.GatewaySSHHostKeyFingerprintSHA256)
	if !sshHostKeyFingerprintRegexp.MatchString(opts.GatewaySSHHostKeyFingerprintSHA256) {
		return opts, errors.New("invalid gateway ssh host key fingerprint")
	}
	if opts.GatewaySSHHostKeyUpdatedUnix < 0 {
		return opts, errors.New("gateway ssh host key updated time must not be negative")
	}
	return opts, nil
}

func normalizeGatewayURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse gateway url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("gateway url must use http or https")
	}
	if setting.Codespace.GatewayRequireHTTPS && parsed.Scheme != "https" {
		return "", errors.New("gateway url must use https")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", errors.New("gateway url host is required")
	}
	if err := validateDNSHost(host); err != nil {
		return "", fmt.Errorf("invalid gateway url host: %w", err)
	}
	if len(strings.Repeat("a", 30)+"-"+strings.Repeat("0", 32)+"."+host) > 253 {
		return "", errors.New("derived gateway endpoint host is too long")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("gateway url must not contain userinfo, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("gateway url must not contain a business path")
	}
	port := parsed.Port()
	if port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return "", errors.New("invalid gateway url port")
		}
		if (parsed.Scheme == "http" && portNumber == 80) || (parsed.Scheme == "https" && portNumber == 443) {
			port = ""
		} else {
			port = strconv.Itoa(portNumber)
		}
	}
	normalized := parsed.Scheme + "://" + host
	if port != "" {
		normalized += ":" + port
	}
	if len(normalized) > 512 {
		return "", errors.New("gateway url is too long")
	}
	return normalized, nil
}

func validateGatewayCookieScope(gatewayURL string) error {
	parsed, err := url.Parse(gatewayURL)
	if err != nil {
		return err
	}
	gatewayHost := parsed.Hostname()

	giteaURL, err := url.Parse(setting.AppURL)
	if err != nil {
		return err
	}
	giteaHost := strings.ToLower(giteaURL.Hostname())
	if giteaHost != "" && net.ParseIP(giteaHost) == nil {
		if err := validateDNSHost(giteaHost); err != nil {
			return ErrDeclareGatewayCookieScopeConflict
		}
		if sameRegistrableDomain(gatewayHost, giteaHost) || isSameOrSubdomain(giteaHost, gatewayHost) {
			return ErrDeclareGatewayCookieScopeConflict
		}
	}

	sessionDomain := normalizeCookieDomain(setting.SessionConfig.Domain)
	if sessionDomain == "" {
		return nil
	}
	if err := validateDNSHost(sessionDomain); err != nil {
		return ErrDeclareGatewayCookieScopeConflict
	}
	if isSameOrSubdomain(gatewayHost, sessionDomain) {
		return ErrDeclareGatewayCookieScopeConflict
	}
	return nil
}

func sameRegistrableDomain(a, b string) bool {
	aSite, err := publicsuffix.EffectiveTLDPlusOne(a)
	if err != nil {
		return true
	}
	bSite, err := publicsuffix.EffectiveTLDPlusOne(b)
	if err != nil {
		return true
	}
	return aSite == bSite
}

func normalizeCookieDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, ".")
	return strings.TrimSuffix(domain, ".")
}

func isSameOrSubdomain(host, parent string) bool {
	return host == parent || strings.HasSuffix(host, "."+parent)
}

func normalizeGatewaySSHAddr(rawAddr string) (string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(rawAddr))
	if err != nil {
		return "", errors.New("gateway ssh address must use host:port")
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if err := validateDNSHost(host); err != nil {
		return "", fmt.Errorf("invalid gateway ssh host: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", errors.New("invalid gateway ssh port")
	}
	normalized := net.JoinHostPort(host, strconv.Itoa(portNumber))
	if len(normalized) > 512 {
		return "", errors.New("gateway ssh address is too long")
	}
	return normalized, nil
}

func validateDNSHost(host string) error {
	if host == "" || strings.HasSuffix(host, ".") {
		return errors.New("host must be a DNS name without trailing dot")
	}
	if net.ParseIP(host) != nil {
		return errors.New("host must not be an IP address")
	}
	if len(host) > 253 {
		return errors.New("host is too long")
	}
	for label := range strings.SplitSeq(host, ".") {
		if !dnsLabelPattern.MatchString(label) {
			return fmt.Errorf("invalid DNS label %q", label)
		}
	}
	return nil
}

func normalizeTags(tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if !tagPattern.MatchString(tag) {
			return nil, fmt.Errorf("invalid manager tag %q", tag)
		}
		if !slices.Contains(normalized, tag) {
			normalized = append(normalized, tag)
		}
	}
	if len(normalized) == 0 {
		normalized = append(normalized, "default")
	}
	if len(normalized) > 64 {
		return nil, errors.New("manager tags exceed 64")
	}
	return normalized, nil
}

func loadRegistrationTokenByValue(ctx context.Context, tokenValue string) (*codespace_model.ManagerToken, error) {
	token := new(codespace_model.ManagerToken)
	has, err := db.GetEngine(ctx).Where("token = ?", tokenValue).Get(token)
	if err != nil || !has {
		return nil, err
	}
	return token, nil
}
