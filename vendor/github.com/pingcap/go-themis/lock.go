package themis

import "github.com/pingcap/go-hbase"

// LockRole is the role of lock
type LockRole int

func (l LockRole) String() string {
	if l == RolePrimary {
		return "primary"
	}
	return "secondary"
}

const (
	// RolePrimary means this row is primary
	RolePrimary LockRole = iota
	// RoleSecondary means this row is secondary
	RoleSecondary
)

type Lock interface {
	// SetCoordinate sets lock's coordinate
	SetCoordinate(c *hbase.ColumnCoordinate)
	// Coordinate returns the lock's coordinate
	Coordinate() *hbase.ColumnCoordinate
	// Timestamp returns startTs of the transction which owned this lock
	Timestamp() uint64
	// SetExpired sets the lock's expired status.
	SetExpired(b bool)
	// IsExpired returns if lock is expired.
	IsExpired() bool
	// Type returns the lock's type, Put or Delete
	Type() hbase.Type
	// Role returns LockRole, primary or secondary
	Role() LockRole
	// not used now
	Context() interface{}
	// valid only  Role == Primary
	Secondaries() []Lock
	// Primary returns the primary lock of this lock
	Primary() Lock
	// Encode encodes the lock to byte slice
	Encode() []byte
}

type LockManager interface {
	// CleanLock if clean lock success, first return value is transction's commit
	// timestamp, otherwise, the second return value is transction's primary
	// lock.
	CleanLock(c *hbase.ColumnCoordinate, prewriteTs uint64) (uint64, Lock, error)
	// EraseLockAndData removes lock and data.
	EraseLockAndData(c *hbase.ColumnCoordinate, prewriteTs uint64) error
	// GetCommitTimestamp returns a committed transction's commit timestamp.
	GetCommitTimestamp(c *hbase.ColumnCoordinate, prewriteTs uint64) (uint64, error)
	// [startTs, endTs]
	IsLockExists(c *hbase.ColumnCoordinate, startTs, endTs uint64) (bool, error)
}
