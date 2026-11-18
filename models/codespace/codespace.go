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
	"gitea.dev/modules/util"

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

// Codespace stores Gitea-owned lifecycle state for one remote development environment.
type Codespace struct {
	ID                        int64
	UserID                    int64  `xorm:"NOT NULL DEFAULT 0"`
	RepoID                    int64  `xorm:"NOT NULL DEFAULT 0"`
	RefType                   string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	RefName                   string `xorm:"TEXT NOT NULL"`
	EnvironmentTag            string `xorm:"VARCHAR(64) NOT NULL"`
	CommitSHA                 string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	DevContainerPath          string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	DevContainerContent       string `xorm:"TEXT NOT NULL"`
	SecretsEncrypted          string `xorm:"LONGTEXT" json:"-"`
	PermissionAuthorizationID int64  `xorm:"NOT NULL DEFAULT 0 index"`
	ManagerID                 int64  `xorm:"NOT NULL DEFAULT 0"`
	Status                    string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationRVersion         int64  `xorm:"NOT NULL DEFAULT 0"`
	RuntimeOperationRVersion  int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationTrigger          string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationCreatedUnix      int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationStartedUnix      int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationDeadlineUnix     int64  `xorm:"NOT NULL DEFAULT 0"`
	LastActiveUnix            int64  `xorm:"NOT NULL DEFAULT 0"`
	AutoStopMode              string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'default'"`
	AutoStopTimeoutSeconds    int64  `xorm:"NOT NULL DEFAULT 0"`
	InteractionGeneration     int64  `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix               int64  `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix               int64  `xorm:"NOT NULL DEFAULT 0"`
	LogSize                   int64  `xorm:"NOT NULL DEFAULT 0"`
}

// Manager stores one registered Manager identity and latest declaration summary.
type Manager struct {
	ID                                 int64
	Name                               string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	UserID                             int64  `xorm:"NOT NULL DEFAULT 0"`
	SecretHash                         string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	SecretSalt                         string `xorm:"VARCHAR(32) NOT NULL DEFAULT ''"`
	TagsJSON                           string `xorm:"TEXT NOT NULL"`
	RuntimeState                       string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'recovering'"`
	LastOnlineUnix                     int64  `xorm:"NOT NULL DEFAULT 0"`
	InventoryGeneration                int64  `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix                        int64  `xorm:"NOT NULL DEFAULT 0"`
	Version                            string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	GatewayURL                         string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	GatewaySSHAddr                     string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	GatewaySSHHostKeyAlgorithm         string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	GatewaySSHHostKeyFingerprintSHA256 string `xorm:"gateway_ssh_host_key_fingerprint_sha256 VARCHAR(255) NOT NULL DEFAULT ''"`
}

// GiteaToken stores the current Gitea API/Git HTTP token for one Codespace.
type GiteaToken struct {
	CodespaceID    int64  `xorm:"pk"`
	TokenHash      string `xorm:"VARCHAR(100) NOT NULL UNIQUE"`
	TokenSalt      string `xorm:"VARCHAR(10) NOT NULL"`
	TokenLastEight string `xorm:"VARCHAR(8) NOT NULL index"`
	TokenEncrypted string `xorm:"TEXT NOT NULL"`
}

// SSHKey stores the Git SSH public key binding for one Codespace.
type SSHKey struct {
	CodespaceID int64 `xorm:"pk"`
	KeyID       int64 `xorm:"NOT NULL UNIQUE"`
}

// OpenToken stores one short-lived, single-use private endpoint authorization code.
type OpenToken struct {
	CodeHash    string `xorm:"CHAR(64) pk"`
	CodespaceID int64  `xorm:"NOT NULL index"`
	UserID      int64  `xorm:"NOT NULL"`
	ManagerID   int64  `xorm:"NOT NULL"`
	EndpointID  string `xorm:"VARCHAR(64) NOT NULL"`
	ExpiresUnix int64  `xorm:"NOT NULL index"`
}

// DevContainerTemplate stores a manually named non-repository Dev Container configuration.
type DevContainerTemplate struct {
	ID          int64
	UserID      int64  `xorm:"NOT NULL DEFAULT 0 index"`
	Name        string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	Content     string `xorm:"TEXT NOT NULL"`
	CreatedUnix int64  `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix int64  `xorm:"NOT NULL DEFAULT 0"`
}

func (*Manager) TableName() string {
	return "codespace_manager"
}

// TableIndices returns Codespace indexes in the same order as their main queries.
func (*Codespace) TableIndices() []*schemas.Index {
	userUpdated := schemas.NewIndex("user_updated", schemas.IndexType)
	userUpdated.AddColumn("user_id", "updated_unix", "created_unix", "id")

	repo := schemas.NewIndex("repo", schemas.IndexType)
	repo.AddColumn("repo_id")

	createClaim := schemas.NewIndex("create_claim", schemas.IndexType)
	createClaim.AddColumn("operation_started_unix", "status", "manager_id", "environment_tag", "operation_created_unix", "id")

	managerActive := schemas.NewIndex("manager_active", schemas.IndexType)
	managerActive.AddColumn("manager_id", "operation_started_unix", "operation_created_unix", "id")

	runningTimeout := schemas.NewIndex("running_timeout", schemas.IndexType)
	runningTimeout.AddColumn("operation_deadline_unix", "id")

	failedRetention := schemas.NewIndex("failed_retention", schemas.IndexType)
	failedRetention.AddColumn("status", "updated_unix", "id")

	return []*schemas.Index{userUpdated, repo, createClaim, managerActive, runningTimeout, failedRetention}
}

// TableIndices returns Manager indexes in the same order as their main queries.
func (*Manager) TableIndices() []*schemas.Index {
	user := schemas.NewIndex("user", schemas.IndexType)
	user.AddColumn("user_id")
	return []*schemas.Index{user}
}

func (*GiteaToken) TableName() string {
	return "codespace_gitea_token"
}

func (*SSHKey) TableName() string {
	return "codespace_ssh_key"
}

func (*OpenToken) TableName() string {
	return "codespace_open_token"
}

func (*DevContainerTemplate) TableName() string {
	return "codespace_dev_container_template"
}

func init() {
	db.RegisterModel(new(Codespace))
	db.RegisterModel(new(Manager))
	db.RegisterModel(new(GiteaToken))
	db.RegisterModel(new(SSHKey))
	db.RegisterModel(new(OpenToken))
	db.RegisterModel(new(DevContainerTemplate))
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

// ValidateCodespace validates a complete persisted Codespace row.
func ValidateCodespace(codespace *Codespace) error {
	if err := ValidateCodespaceLifecycle(codespace); err != nil {
		return err
	}
	hasDevContainerPath := strings.TrimSpace(codespace.DevContainerPath) != ""
	hasDevContainerContent := strings.TrimSpace(codespace.DevContainerContent) != ""
	if hasDevContainerPath == hasDevContainerContent {
		return errors.New("invalid Dev Container configuration")
	}
	return nil
}

// ValidateCodespaceLifecycle validates state independently of the creation configuration.
func ValidateCodespaceLifecycle(codespace *Codespace) error {
	if codespace == nil {
		return errors.New("codespace is nil")
	}
	if !validStatus(codespace.Status) {
		return fmt.Errorf("invalid codespace status %q", codespace.Status)
	}
	if !validAutoStopMode(codespace.AutoStopMode) {
		return fmt.Errorf("invalid auto stop mode %q", codespace.AutoStopMode)
	}
	if codespace.OperationRVersion <= 0 {
		return errors.New("operation_r_version must be positive")
	}
	if codespace.RuntimeOperationRVersion < 0 || codespace.RuntimeOperationRVersion > codespace.OperationRVersion {
		return errors.New("runtime_operation_r_version must identify a completed operation")
	}
	if codespace.Status == StatusRunning {
		if codespace.RuntimeOperationRVersion == 0 {
			return errors.New("running codespace requires a runtime operation version")
		}
	} else if codespace.RuntimeOperationRVersion != 0 {
		return errors.New("non-running codespace cannot retain a runtime operation version")
	}
	if (codespace.Status == StatusRunning || codespace.Status == StatusStopped) && codespace.ManagerID <= 0 {
		return errors.New("running or stopped codespace requires a manager")
	}
	hasOperation := codespace.OperationCreatedUnix > 0
	if !hasOperation {
		if codespace.OperationTrigger != "" || codespace.OperationCreatedUnix != 0 || codespace.OperationStartedUnix != 0 || codespace.OperationDeadlineUnix != 0 {
			return errors.New("inactive operation has state")
		}
		if codespace.Status == StatusCreating || codespace.Status == StatusDeleting {
			return fmt.Errorf("codespace status %q requires an active operation", codespace.Status)
		}
		return nil
	}
	if !validOperationTrigger(codespace.OperationTrigger) {
		return fmt.Errorf("invalid operation trigger %q", codespace.OperationTrigger)
	}
	operationType := ActiveOperationType(codespace)
	if operationType == "" {
		return fmt.Errorf("codespace status %q cannot own an active operation", codespace.Status)
	}
	if codespace.OperationTrigger == OperationTriggerIdle && operationType != OperationStop {
		return errors.New("idle trigger is only valid for stop operations")
	}
	if codespace.OperationStartedUnix == 0 {
		if codespace.OperationDeadlineUnix != 0 {
			return errors.New("queued operation has running timestamps")
		}
		if operationType != OperationCreate && codespace.ManagerID <= 0 {
			return errors.New("queued lifecycle operation requires a manager")
		}
	} else if codespace.ManagerID <= 0 || codespace.OperationStartedUnix <= 0 || codespace.OperationDeadlineUnix < codespace.OperationStartedUnix {
		return errors.New("running operation has invalid ownership or deadline")
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

// ActiveOperationType derives the command owned by an active Codespace row.
func ActiveOperationType(codespace *Codespace) string {
	if codespace == nil || codespace.OperationCreatedUnix <= 0 {
		return ""
	}
	switch codespace.Status {
	case StatusCreating:
		return OperationCreate
	case StatusStopped:
		return OperationResume
	case StatusRunning:
		return OperationStop
	case StatusDeleting:
		return OperationDelete
	default:
		return ""
	}
}

// IsOperationQueued reports whether an active operation has not started.
func IsOperationQueued(codespace *Codespace) bool {
	return ActiveOperationType(codespace) != "" && codespace.OperationStartedUnix == 0
}

// IsOperationRunning reports whether an active operation owns a lease.
func IsOperationRunning(codespace *Codespace) bool {
	return ActiveOperationType(codespace) != "" && codespace.OperationStartedUnix > 0
}

func validOperationTrigger(operationTrigger string) bool {
	switch operationTrigger {
	case OperationTriggerUser, OperationTriggerIdle:
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
