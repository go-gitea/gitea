// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"github.com/google/uuid"
	"xorm.io/xorm/schemas"
)

// Codespace persistent lifecycle states.
const (
	StatusCreating = "creating"
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusDeleting = "deleting"
	StatusFailed   = "failed"
)

// Codespace operation types.
const (
	OperationCreate = "create"
	OperationResume = "resume"
	OperationStop   = "stop"
	OperationDelete = "delete"
)

// Codespace operation statuses.
const (
	OperationStatusQueued  = "queued"
	OperationStatusRunning = "running"
)

// Codespace operation triggers.
const (
	OperationTriggerUser = "user"
	OperationTriggerIdle = "idle"
)

// Codespace git protocol values.
const (
	GitProtocolHTTP = "http"
	GitProtocolSSH  = "ssh"
)

// Codespace auto-stop modes.
const (
	AutoStopModeDefault = "default"
	AutoStopModeCustom  = "custom"
	AutoStopModeNever   = "never"
)

// Manager runtime states persisted by Gitea.
const (
	ManagerRuntimeStateOnline     = "online"
	ManagerRuntimeStateRecovering = "recovering"
)

// Manager address kinds.
const (
	ManagerAddressGateway = "gateway"
	ManagerAddressSSH     = "ssh"
)

// GiteaTokenAuthDataKey stores the Codespace Token auth snapshot in request data.
const GiteaTokenAuthDataKey = "CodespaceToken"

// GiteaTokenRoutePolicyDataKey stores the Codespace Token route policy selected by routing.
const GiteaTokenRoutePolicyDataKey = "CodespaceTokenRoutePolicy"

// Codespace Token route policies.
const (
	GiteaTokenRoutePolicySelf            = "self"
	GiteaTokenRoutePolicyPublicInfo      = "public_info"
	GiteaTokenRoutePolicyRepositoryGroup = "repository_group"
	GiteaTokenRoutePolicySignedArtifact  = "signed_artifact"
)

// Codespace stores Gitea-owned lifecycle state for one remote development environment.
type Codespace struct {
	UUID                   string `xorm:"pk CHAR(36)"`
	UserID                 int64  `xorm:"NOT NULL DEFAULT 0"`
	RepoID                 int64  `xorm:"NOT NULL DEFAULT 0"`
	RefType                string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	RefName                string `xorm:"TEXT NOT NULL"`
	RepoTag                string `xorm:"VARCHAR(64) NOT NULL DEFAULT 'default'"`
	GitProtocol            string `xorm:"VARCHAR(8) NOT NULL DEFAULT 'http'"`
	CommitSHA              string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	ManagerID              int64  `xorm:"NOT NULL DEFAULT 0"`
	Status                 string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationRVersion      int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationType          string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationStatus        string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationTrigger       string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationCreatedUnix   int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationStartedUnix   int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationDeadlineUnix  int64  `xorm:"NOT NULL DEFAULT 0"`
	RuntimeGeneration      int64  `xorm:"NOT NULL DEFAULT 0"`
	LastActiveUnix         int64  `xorm:"NOT NULL DEFAULT 0"`
	AutoStopMode           string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'default'"`
	AutoStopTimeoutSeconds int64  `xorm:"NOT NULL DEFAULT 0"`
	InteractionGeneration  int64  `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix            int64  `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix            int64  `xorm:"NOT NULL DEFAULT 0"`
	StoppedUnix            int64  `xorm:"NOT NULL DEFAULT 0"`
	LogFilename            string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	LogLineCount           int64  `xorm:"NOT NULL DEFAULT 0"`
	LogSize                int64  `xorm:"NOT NULL DEFAULT 0"`
}

// Manager stores one registered Manager identity and latest declaration summary.
type Manager struct {
	ID                  int64
	Name                string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	OwnerID             int64  `xorm:"NOT NULL DEFAULT 0"`
	SecretHash          string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	SecretSalt          string `xorm:"VARCHAR(32) NOT NULL DEFAULT ''"`
	TagsJSON            string `xorm:"TEXT NOT NULL"`
	RuntimeState        string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'recovering'"`
	LastOnlineUnix      int64  `xorm:"NOT NULL DEFAULT 0"`
	InventoryGeneration int64  `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix         int64  `xorm:"NOT NULL DEFAULT 0"`
	MetaJSON            string `xorm:"TEXT NOT NULL"`
}

// ManagerAddress stores current routable addresses declared by a Manager.
type ManagerAddress struct {
	ID        int64
	ManagerID int64  `xorm:"NOT NULL DEFAULT 0 unique(manager_kind)"`
	Kind      string `xorm:"VARCHAR(16) NOT NULL DEFAULT '' unique(manager_kind) unique(kind_address)"`
	Address   string `xorm:"VARCHAR(512) NOT NULL DEFAULT '' unique(kind_address)"`
}

// ManagerToken stores the current owner-scoped Manager registration token.
type ManagerToken struct {
	ID      int64
	Token   string             `xorm:"VARCHAR(64) NOT NULL UNIQUE"`
	OwnerID int64              `xorm:"NOT NULL DEFAULT 0 UNIQUE"`
	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
}

// GiteaToken stores the current Gitea API/Git HTTP token for one Codespace.
type GiteaToken struct {
	CodespaceUUID  string `xorm:"pk CHAR(36)"`
	TokenHash      string `xorm:"VARCHAR(100) NOT NULL UNIQUE"`
	TokenSalt      string `xorm:"VARCHAR(10) NOT NULL"`
	TokenLastEight string `xorm:"VARCHAR(8) NOT NULL index"`
	TokenEncrypted string `xorm:"TEXT NOT NULL"`
	CreatedUnix    int64  `xorm:"NOT NULL DEFAULT 0"`
}

// SSHKey stores the Git SSH public key binding for one Codespace.
type SSHKey struct {
	CodespaceUUID string `xorm:"pk CHAR(36)"`
	KeyID         int64  `xorm:"NOT NULL UNIQUE"`
	CreatedUnix   int64  `xorm:"NOT NULL DEFAULT 0"`
}

func (*Manager) TableName() string {
	return "codespace_manager"
}

// TableIndices returns Codespace indexes in the same order as their main queries.
func (*Codespace) TableIndices() []*schemas.Index {
	userStatus := schemas.NewIndex("user_status", schemas.IndexType)
	userStatus.AddColumn("user_id", "status")

	repoStatus := schemas.NewIndex("repo_status", schemas.IndexType)
	repoStatus.AddColumn("repo_id", "status")

	createClaim := schemas.NewIndex("create_claim", schemas.IndexType)
	createClaim.AddColumn("status", "operation_type", "operation_status", "manager_id", "repo_tag", "operation_created_unix", "uuid")

	managerActive := schemas.NewIndex("manager_active", schemas.IndexType)
	managerActive.AddColumn("manager_id", "operation_type", "operation_status", "status", "operation_created_unix", "uuid")

	queuedTimeout := schemas.NewIndex("queued_timeout", schemas.IndexType)
	queuedTimeout.AddColumn("operation_status", "operation_created_unix", "uuid")

	runningTimeout := schemas.NewIndex("running_timeout", schemas.IndexType)
	runningTimeout.AddColumn("operation_status", "operation_deadline_unix", "uuid")

	failedRetention := schemas.NewIndex("failed_retention", schemas.IndexType)
	failedRetention.AddColumn("status", "updated_unix", "uuid")

	return []*schemas.Index{userStatus, repoStatus, createClaim, managerActive, queuedTimeout, runningTimeout, failedRetention}
}

// TableIndices returns Manager indexes in the same order as their main queries.
func (*Manager) TableIndices() []*schemas.Index {
	ownerRuntime := schemas.NewIndex("owner_runtime", schemas.IndexType)
	ownerRuntime.AddColumn("owner_id", "runtime_state")

	runtimeOnline := schemas.NewIndex("runtime_online", schemas.IndexType)
	runtimeOnline.AddColumn("runtime_state", "last_online_unix")

	return []*schemas.Index{ownerRuntime, runtimeOnline}
}

func (*ManagerAddress) TableName() string {
	return "codespace_manager_address"
}

func (*ManagerToken) TableName() string {
	return "codespace_manager_token"
}

func (*GiteaToken) TableName() string {
	return "codespace_gitea_token"
}

func (*SSHKey) TableName() string {
	return "codespace_ssh_key"
}

func init() {
	db.RegisterModel(new(Codespace))
	db.RegisterModel(new(Manager))
	db.RegisterModel(new(ManagerAddress))
	db.RegisterModel(new(ManagerToken))
	db.RegisterModel(new(GiteaToken))
	db.RegisterModel(new(SSHKey))
}

// NewUUID returns a canonical lower-case RFC 4122 UUID v4 string.
func NewUUID() string {
	return uuid.NewString()
}

// GenerateManagerSecret fills the salted Manager secret verifier and returns the plaintext secret.
func (m *Manager) GenerateManagerSecret() string {
	secret := hex.EncodeToString(util.CryptoRandomBytes(32))
	m.SecretSalt = hex.EncodeToString(util.CryptoRandomBytes(16))
	m.SecretHash = managerSecretHash(secret, m.SecretSalt)
	return secret
}

// VerifyManagerSecret checks a plaintext Manager secret against the stored verifier.
func (m *Manager) VerifyManagerSecret(secret string) bool {
	if m == nil || m.SecretHash == "" || m.SecretSalt == "" || secret == "" {
		return false
	}
	hash := managerSecretHash(secret, m.SecretSalt)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(m.SecretHash)) == 1
}

// ValidateUUID checks that the input is the canonical lower-case UUID form used by Codespace.
func ValidateUUID(codespaceUUID string) error {
	parsed, err := uuid.Parse(codespaceUUID)
	if err != nil {
		return fmt.Errorf("invalid codespace uuid: %w", err)
	}
	if parsed.Version() != 4 {
		return errors.New("codespace uuid must be version 4")
	}
	if parsed.String() != codespaceUUID {
		return errors.New("codespace uuid must be canonical lower-case form")
	}
	return nil
}

// UUID32 returns the 32-character host-safe form derived from a canonical Codespace UUID.
func UUID32(codespaceUUID string) (string, error) {
	if err := ValidateUUID(codespaceUUID); err != nil {
		return "", err
	}
	return strings.ReplaceAll(codespaceUUID, "-", ""), nil
}

// NextVersion returns the next positive version or fails without wrapping.
func NextVersion(current int64) (int64, error) {
	if current < 0 {
		return 0, errors.New("version must not be negative")
	}
	if current == math.MaxInt64 {
		return 0, errors.New("version exhausted")
	}
	return current + 1, nil
}

// ValidateCodespace validates enum-like fields stored on a Codespace row.
func ValidateCodespace(codespace *Codespace) error {
	if codespace == nil {
		return errors.New("codespace is nil")
	}
	if err := ValidateUUID(codespace.UUID); err != nil {
		return err
	}
	if !validStatus(codespace.Status) {
		return fmt.Errorf("invalid codespace status %q", codespace.Status)
	}
	if !validGitProtocol(codespace.GitProtocol) {
		return fmt.Errorf("invalid git protocol %q", codespace.GitProtocol)
	}
	if !validAutoStopMode(codespace.AutoStopMode) {
		return fmt.Errorf("invalid auto stop mode %q", codespace.AutoStopMode)
	}
	if codespace.OperationType == "" && codespace.OperationStatus == "" && codespace.OperationTrigger == "" {
		return nil
	}
	if !validOperationType(codespace.OperationType) {
		return fmt.Errorf("invalid operation type %q", codespace.OperationType)
	}
	if !validOperationStatus(codespace.OperationStatus) {
		return fmt.Errorf("invalid operation status %q", codespace.OperationStatus)
	}
	if !validOperationTrigger(codespace.OperationTrigger) {
		return fmt.Errorf("invalid operation trigger %q", codespace.OperationTrigger)
	}
	return nil
}

// ValidateManager validates enum-like fields stored on a Manager row.
func ValidateManager(manager *Manager) error {
	if manager == nil {
		return errors.New("manager is nil")
	}
	if !validManagerRuntimeState(manager.RuntimeState) {
		return fmt.Errorf("invalid manager runtime state %q", manager.RuntimeState)
	}
	return nil
}

func validStatus(status string) bool {
	switch status {
	case StatusCreating, StatusRunning, StatusStopped, StatusDeleting, StatusFailed:
		return true
	default:
		return false
	}
}

func validOperationType(operationType string) bool {
	switch operationType {
	case OperationCreate, OperationResume, OperationStop, OperationDelete:
		return true
	default:
		return false
	}
}

func validOperationStatus(operationStatus string) bool {
	switch operationStatus {
	case OperationStatusQueued, OperationStatusRunning:
		return true
	default:
		return false
	}
}

func validOperationTrigger(operationTrigger string) bool {
	switch operationTrigger {
	case OperationTriggerUser, OperationTriggerIdle:
		return true
	default:
		return false
	}
}

func validGitProtocol(protocol string) bool {
	switch protocol {
	case GitProtocolHTTP, GitProtocolSSH:
		return true
	default:
		return false
	}
}

func validAutoStopMode(mode string) bool {
	switch mode {
	case AutoStopModeDefault, AutoStopModeCustom, AutoStopModeNever:
		return true
	default:
		return false
	}
}

func validManagerRuntimeState(state string) bool {
	switch state {
	case ManagerRuntimeStateOnline, ManagerRuntimeStateRecovering:
		return true
	default:
		return false
	}
}

func managerSecretHash(secret, salt string) string {
	secretBytes, secretErr := hex.DecodeString(secret)
	saltBytes, saltErr := hex.DecodeString(salt)
	if secretErr != nil || saltErr != nil {
		return ""
	}
	payload := make([]byte, 0, len(saltBytes)+len(secretBytes))
	payload = append(payload, saltBytes...)
	payload = append(payload, secretBytes...)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
