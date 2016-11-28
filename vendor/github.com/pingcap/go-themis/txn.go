package themis

import (
	"errors"

	"github.com/pingcap/go-hbase"
)

var (
	ErrLockNotExpired  = errors.New("lock not expired")
	ErrCleanLockFailed = errors.New("clean lock failed")
	ErrWrongRegion     = errors.New("wrong region, please retry")
	ErrTooManyRows     = errors.New("too many rows in one transaction")
	ErrRetryable       = errors.New("try again later")
)

type Txn interface {
	Get(t string, get *hbase.Get) (*hbase.ResultRow, error)
	Gets(t string, gets []*hbase.Get) ([]*hbase.ResultRow, error)
	LockRow(t string, row []byte) error
	Put(t string, put *hbase.Put)
	Delete(t string, del *hbase.Delete) error
	GetScanner(tbl []byte, startKey, endKey []byte, batchSize int) *ThemisScanner
	Release()
	Commit() error
	GetStartTS() uint64
	GetCommitTS() uint64
}
