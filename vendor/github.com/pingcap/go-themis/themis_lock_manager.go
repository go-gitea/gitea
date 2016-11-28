package themis

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase"
)

var _ LockManager = (*themisLockManager)(nil)

type themisLockManager struct {
	rpc         *themisRPC
	hbaseClient hbase.HBaseClient
}

func newThemisLockManager(rpc *themisRPC, hbaseCli hbase.HBaseClient) LockManager {
	return &themisLockManager{
		rpc:         rpc,
		hbaseClient: hbaseCli,
	}
}

func getDataColFromMetaCol(lockOrWriteCol hbase.Column) hbase.Column {
	// get data column from lock column
	// key is like => L:family#qual, #p:family#qual
	parts := strings.Split(string(lockOrWriteCol.Qual), "#")
	if len(parts) != 2 {
		return lockOrWriteCol
	}
	c := hbase.Column{
		Family: []byte(parts[0]),
		Qual:   []byte(parts[1]),
	}
	return c
}

func getLocksFromResults(tbl []byte, lockKvs []*hbase.Kv, client *themisRPC) ([]Lock, error) {
	var locks []Lock
	for _, kv := range lockKvs {
		col := &hbase.ColumnCoordinate{
			Table: tbl,
			Row:   kv.Row,
			Column: hbase.Column{
				Family: kv.Family,
				Qual:   kv.Qual,
			},
		}
		if !isLockColumn(col.Column) {
			return nil, errors.New("invalid lock")
		}
		l, err := parseLockFromBytes(kv.Value)
		if err != nil {
			return nil, errors.Trace(err)
		}
		cc := &hbase.ColumnCoordinate{
			Table:  tbl,
			Row:    kv.Row,
			Column: getDataColFromMetaCol(col.Column),
		}
		l.SetCoordinate(cc)
		client.checkAndSetLockIsExpired(l)
		locks = append(locks, l)
	}
	return locks, nil
}

func (m *themisLockManager) IsLockExists(cc *hbase.ColumnCoordinate, startTs, endTs uint64) (bool, error) {
	get := hbase.NewGet(cc.Row)
	get.AddTimeRange(startTs, endTs+1)
	get.AddStringColumn(string(LockFamilyName), string(cc.Family)+"#"+string(cc.Qual))
	// check if lock exists
	rs, err := m.hbaseClient.Get(string(cc.Table), get)
	if err != nil {
		return false, errors.Trace(err)
	}
	// primary lock has been released
	if rs == nil {
		return false, nil
	}
	return true, nil
}

func (m *themisLockManager) GetCommitTimestamp(cc *hbase.ColumnCoordinate, prewriteTs uint64) (uint64, error) {
	g := hbase.NewGet(cc.Row)
	// add put write column
	qual := string(cc.Family) + "#" + string(cc.Qual)
	g.AddStringColumn("#p", qual)
	// add del write column
	g.AddStringColumn("#d", qual)
	// time range => [ours startTs, +Inf)
	g.AddTimeRange(prewriteTs, math.MaxInt64)
	g.SetMaxVersion(math.MaxInt32)
	r, err := m.hbaseClient.Get(string(cc.Table), g)
	if err != nil {
		return 0, errors.Trace(err)
	}
	// may delete by other client
	if r == nil {
		return 0, nil
	}
	for _, kv := range r.SortedColumns {
		for commitTs, val := range kv.Values {
			var ts uint64
			binary.Read(bytes.NewBuffer(val), binary.BigEndian, &ts)
			if ts == prewriteTs {
				// get this commit's commitTs
				return commitTs, nil
			}
		}
	}
	// no such transction
	return 0, nil
}

func (m *themisLockManager) CleanLock(cc *hbase.ColumnCoordinate, prewriteTs uint64) (uint64, Lock, error) {
	l, err := m.rpc.getLockAndErase(cc, prewriteTs)
	if err != nil {
		return 0, nil, errors.Trace(err)
	}
	pl, _ := l.(*themisPrimaryLock)
	// if primary lock is nil, means someothers have already committed
	if pl == nil {
		commitTs, err := m.GetCommitTimestamp(cc, prewriteTs)
		if err != nil {
			return 0, nil, errors.Trace(err)
		}
		return commitTs, nil, nil
	}
	return 0, pl, nil
}

func (m *themisLockManager) EraseLockAndData(cc *hbase.ColumnCoordinate, prewriteTs uint64) error {
	log.Debugf("erase row=%q txn=%d", cc.Row, prewriteTs)
	d := hbase.NewDelete(cc.Row)
	d.AddColumnWithTimestamp(LockFamilyName, []byte(string(cc.Family)+"#"+string(cc.Qual)), prewriteTs)
	d.AddColumnWithTimestamp(cc.Family, cc.Qual, prewriteTs)
	ok, err := m.hbaseClient.Delete(string(cc.Table), d)
	if !ok {
		log.Error(err)
	}
	return errors.Trace(err)
}
