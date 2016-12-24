package zkhelper

import (
	zk "github.com/ngaut/go-zookeeper/zk"
)

/*
type Stat interface {
	Czxid() int64
	Mzxid() int64
	CTime() time.Time
	MTime() time.Time
	Version() int
	CVersion() int
	AVersion() int
	EphemeralOwner() int64
	DataLength() int
	NumChildren() int
	Pzxid() int64
}
*/

// This interface is really close to the zookeeper connection
// interface.  It uses the Stat interface defined here instead of the
// zookeeper.Stat structure for stats. Everything else is the same as
// in zookeeper.  So refer to the zookeeper docs for the conventions
// used here (for instance, using -1 as version to specify any
// version)
type Conn interface {
	Get(path string) (data []byte, stat zk.Stat, err error)
	GetW(path string) (data []byte, stat zk.Stat, watch <-chan zk.Event, err error)

	Children(path string) (children []string, stat zk.Stat, err error)
	ChildrenW(path string) (children []string, stat zk.Stat, watch <-chan zk.Event, err error)

	Exists(path string) (exist bool, stat zk.Stat, err error)
	ExistsW(path string) (exist bool, stat zk.Stat, watch <-chan zk.Event, err error)

	Create(path string, value []byte, flags int32, aclv []zk.ACL) (pathCreated string, err error)

	Set(path string, value []byte, version int32) (stat zk.Stat, err error)

	Delete(path string, version int32) (err error)

	Close()

	//RetryChange(path string, flags int, acl []ACL, changeFunc ChangeFunc) error

	GetACL(path string) ([]zk.ACL, zk.Stat, error)
	SetACL(path string, aclv []zk.ACL, version int32) (zk.Stat, error)

	Seq2Str(seq int64) string
}
