package themis

import (
	"fmt"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase"
	"github.com/pingcap/go-themis/oracle"
)

type TxnConfig struct {
	ConcurrentPrewriteAndCommit bool
	WaitSecondaryCommit         bool
	TTLInMs                     uint64
	MaxRowsInOneTxn             int
	// options below is for debugging and testing
	brokenPrewriteSecondaryTest            bool
	brokenPrewriteSecondaryAndRollbackTest bool
	brokenCommitPrimaryTest                bool
	brokenCommitSecondaryTest              bool
}

var defaultTxnConf = TxnConfig{
	ConcurrentPrewriteAndCommit:            true,
	WaitSecondaryCommit:                    false,
	MaxRowsInOneTxn:                        50000,
	TTLInMs:                                5 * 1000, // default txn TTL: 5s
	brokenPrewriteSecondaryTest:            false,
	brokenPrewriteSecondaryAndRollbackTest: false,
	brokenCommitPrimaryTest:                false,
	brokenCommitSecondaryTest:              false,
}

type themisTxn struct {
	client             hbase.HBaseClient
	rpc                *themisRPC
	lockCleaner        LockManager
	oracle             oracle.Oracle
	mutationCache      *columnMutationCache
	startTs            uint64
	commitTs           uint64
	primaryRow         *rowMutation
	primary            *hbase.ColumnCoordinate
	secondaryRows      []*rowMutation
	secondary          []*hbase.ColumnCoordinate
	primaryRowOffset   int
	singleRowTxn       bool
	secondaryLockBytes []byte
	conf               TxnConfig
	hooks              *txnHook
}

var _ Txn = (*themisTxn)(nil)

var (
	// ErrSimulated is used when maybe rollback occurs error too.
	ErrSimulated           = errors.New("simulated error")
	maxCleanLockRetryCount = 30
	pauseTime              = 300 * time.Millisecond
)

func NewTxn(c hbase.HBaseClient, oracle oracle.Oracle) (Txn, error) {
	return NewTxnWithConf(c, defaultTxnConf, oracle)
}

func NewTxnWithConf(c hbase.HBaseClient, conf TxnConfig, oracle oracle.Oracle) (Txn, error) {
	var err error
	txn := &themisTxn{
		client:           c,
		mutationCache:    newColumnMutationCache(),
		oracle:           oracle,
		primaryRowOffset: -1,
		conf:             conf,
		rpc:              newThemisRPC(c, oracle, conf),
		hooks:            newHook(),
	}
	txn.startTs, err = txn.oracle.GetTimestamp()
	if err != nil {
		return nil, errors.Trace(err)
	}
	txn.lockCleaner = newThemisLockManager(txn.rpc, c)
	return txn, nil
}

func (txn *themisTxn) setHook(hooks *txnHook) {
	txn.hooks = hooks
}

func (txn *themisTxn) Gets(tbl string, gets []*hbase.Get) ([]*hbase.ResultRow, error) {
	results, err := txn.rpc.themisBatchGet([]byte(tbl), gets, txn.startTs, false)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var ret []*hbase.ResultRow
	hasLock := false
	for _, r := range results {
		// if this row is locked, try clean lock and get again
		if isLockResult(r) {
			hasLock = true
			err = txn.constructLockAndClean([]byte(tbl), r.SortedColumns)
			if err != nil {
				// TODO if it's a conflict error, it means this lock
				// isn't expired, maybe we can retry or return partial results.
				return nil, errors.Trace(err)
			}
		}
		// it's OK, because themisBatchGet doesn't return nil value.
		ret = append(ret, r)
	}
	if hasLock {
		// after we cleaned locks, try to get again.
		ret, err = txn.rpc.themisBatchGet([]byte(tbl), gets, txn.startTs, true)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return ret, nil
}

func (txn *themisTxn) Get(tbl string, g *hbase.Get) (*hbase.ResultRow, error) {
	r, err := txn.rpc.themisGet([]byte(tbl), g, txn.startTs, false)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// contain locks, try to clean and get again
	if r != nil && isLockResult(r) {
		r, err = txn.tryToCleanLockAndGetAgain([]byte(tbl), g, r.SortedColumns)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return r, nil
}

func (txn *themisTxn) Put(tbl string, p *hbase.Put) {
	// add mutation to buffer
	for _, e := range getEntriesFromPut(p) {
		txn.mutationCache.addMutation([]byte(tbl), p.Row, e.Column, e.typ, e.value, false)
	}
}

func (txn *themisTxn) Delete(tbl string, p *hbase.Delete) error {
	entries, err := getEntriesFromDel(p)
	if err != nil {
		return errors.Trace(err)
	}
	for _, e := range entries {
		txn.mutationCache.addMutation([]byte(tbl), p.Row, e.Column, e.typ, e.value, false)
	}
	return nil
}

func (txn *themisTxn) Commit() error {
	if txn.mutationCache.getMutationCount() == 0 {
		return nil
	}
	if txn.mutationCache.getRowCount() > txn.conf.MaxRowsInOneTxn {
		return ErrTooManyRows
	}

	txn.selectPrimaryAndSecondaries()
	err := txn.prewritePrimary()
	if err != nil {
		// no need to check wrong region here, hbase client will retry when
		// occurs single row NotInRegion error.
		log.Error(errors.ErrorStack(err))
		// it's safe to retry, because this transaction is not committed.
		return ErrRetryable
	}

	err = txn.prewriteSecondary()
	if err != nil {
		if isWrongRegionErr(err) {
			log.Warn("region info outdated")
			// reset hbase client buffered region info
			txn.client.CleanAllRegionCache()
		}
		return ErrRetryable
	}

	txn.commitTs, err = txn.oracle.GetTimestamp()
	if err != nil {
		log.Error(errors.ErrorStack(err))
		return ErrRetryable
	}
	err = txn.commitPrimary()
	if err != nil {
		// commit primary error, rollback
		log.Error("commit primary row failed", txn.startTs, err)
		txn.rollbackRow(txn.primaryRow.tbl, txn.primaryRow)
		txn.rollbackSecondaryRow(len(txn.secondaryRows) - 1)
		return ErrRetryable
	}
	txn.commitSecondary()
	log.Debug("themis txn commit successfully", txn.startTs, txn.commitTs)
	return nil
}

func (txn *themisTxn) commitSecondary() {
	if bypass, _, _ := txn.hooks.beforeCommitSecondary(txn, nil); !bypass {
		return
	}
	if txn.conf.brokenCommitSecondaryTest {
		txn.brokenCommitSecondary()
		return
	}
	if txn.conf.ConcurrentPrewriteAndCommit {
		txn.batchCommitSecondary(txn.conf.WaitSecondaryCommit)
	} else {
		txn.commitSecondarySync()
	}
}

func (txn *themisTxn) commitSecondarySync() {
	for _, r := range txn.secondaryRows {
		err := txn.rpc.commitSecondaryRow(r.tbl, r.row, r.mutationList(false), txn.startTs, txn.commitTs)
		if err != nil {
			// fail of secondary commit will not stop the commits of next
			// secondaries
			log.Warning(err)
		}
	}
}

func (txn *themisTxn) batchCommitSecondary(wait bool) error {
	//will batch commit all rows in a region
	rsRowMap, err := txn.groupByRegion()
	if err != nil {
		return errors.Trace(err)
	}

	wg := sync.WaitGroup{}
	for _, regionRowMap := range rsRowMap {
		wg.Add(1)
		_, firstRowM := getFirstEntity(regionRowMap)
		go func(cli *themisRPC, tbl string, rMap map[string]*rowMutation, startTs, commitTs uint64) {
			defer wg.Done()
			err := cli.batchCommitSecondaryRows([]byte(tbl), rMap, startTs, commitTs)
			if err != nil {
				// fail of secondary commit will not stop the commits of next
				// secondaries
				if isWrongRegionErr(err) {
					txn.client.CleanAllRegionCache()
					log.Warn("region info outdated when committing secondary rows, don't panic")
				}
			}
		}(txn.rpc, string(firstRowM.tbl), regionRowMap, txn.startTs, txn.commitTs)
	}
	if wait {
		wg.Wait()
	}
	return nil
}

func (txn *themisTxn) groupByRegion() (map[string]map[string]*rowMutation, error) {
	rsRowMap := make(map[string]map[string]*rowMutation)
	for _, rm := range txn.secondaryRows {
		region, err := txn.client.LocateRegion(rm.tbl, rm.row, true)
		if err != nil {
			return nil, errors.Trace(err)
		}
		key := getBatchGroupKey(region, string(rm.tbl))
		if _, exists := rsRowMap[key]; !exists {
			rsRowMap[key] = map[string]*rowMutation{}
		}
		rsRowMap[key][string(rm.row)] = rm
	}
	return rsRowMap, nil
}

func (txn *themisTxn) commitPrimary() error {
	if txn.conf.brokenCommitPrimaryTest {
		return txn.brokenCommitPrimary()
	}
	return txn.rpc.commitRow(txn.primary.Table, txn.primary.Row,
		txn.primaryRow.mutationList(false),
		txn.startTs, txn.commitTs, txn.primaryRowOffset)
}

func (txn *themisTxn) selectPrimaryAndSecondaries() {
	txn.secondary = nil
	for tblName, rowMutations := range txn.mutationCache.mutations {
		for _, rowMutation := range rowMutations {
			row := rowMutation.row
			findPrimaryInRow := false
			for i, mutation := range rowMutation.mutationList(true) {
				colcord := hbase.NewColumnCoordinate([]byte(tblName), row, mutation.Family, mutation.Qual)
				// set the first column as primary if primary is not set by user
				if txn.primaryRowOffset == -1 &&
					(txn.primary == nil || txn.primary.Equal(colcord)) {
					txn.primary = colcord
					txn.primaryRowOffset = i
					txn.primaryRow = rowMutation
					findPrimaryInRow = true
				} else {
					txn.secondary = append(txn.secondary, colcord)
				}
			}
			if !findPrimaryInRow {
				txn.secondaryRows = append(txn.secondaryRows, rowMutation)
			}
		}
	}

	// hook for test
	if bypass, _, _ := txn.hooks.afterChoosePrimaryAndSecondary(txn, nil); !bypass {
		return
	}

	if len(txn.secondaryRows) == 0 {
		txn.singleRowTxn = true
	}
	// construct secondary lock
	secondaryLock := txn.constructSecondaryLock(hbase.TypePut)
	if secondaryLock != nil {
		txn.secondaryLockBytes = secondaryLock.Encode()
	} else {
		txn.secondaryLockBytes = nil
	}
}

func (txn *themisTxn) constructSecondaryLock(typ hbase.Type) *themisSecondaryLock {
	if txn.primaryRow.getSize() <= 1 && len(txn.secondaryRows) == 0 {
		return nil
	}
	l := newThemisSecondaryLock()
	l.primaryCoordinate = txn.primary
	l.ts = txn.startTs
	// TODO set client addr
	return l
}

func (txn *themisTxn) constructPrimaryLock() *themisPrimaryLock {
	l := newThemisPrimaryLock()
	l.typ = txn.primaryRow.getType(txn.primary.Column)
	l.ts = txn.startTs
	for _, c := range txn.secondary {
		l.addSecondary(c, txn.mutationCache.getMutation(c).typ)
	}
	return l
}

func (txn *themisTxn) constructLockAndClean(tbl []byte, lockKvs []*hbase.Kv) error {
	locks, err := getLocksFromResults([]byte(tbl), lockKvs, txn.rpc)
	if err != nil {
		return errors.Trace(err)
	}
	for _, lock := range locks {
		err := txn.cleanLockWithRetry(lock)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (txn *themisTxn) tryToCleanLockAndGetAgain(tbl []byte, g *hbase.Get, lockKvs []*hbase.Kv) (*hbase.ResultRow, error) {
	// try to clean locks
	err := txn.constructLockAndClean(tbl, lockKvs)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// get again, ignore lock
	r, err := txn.rpc.themisGet([]byte(tbl), g, txn.startTs, true)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return r, nil
}

func (txn *themisTxn) commitSecondaryAndCleanLock(lock *themisSecondaryLock, commitTs uint64) error {
	cc := lock.Coordinate()
	mutation := &columnMutation{
		Column: &cc.Column,
		mutationValuePair: &mutationValuePair{
			typ: lock.typ,
		},
	}
	err := txn.rpc.commitSecondaryRow(cc.Table, cc.Row,
		[]*columnMutation{mutation}, lock.Timestamp(), commitTs)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (txn *themisTxn) cleanLockWithRetry(lock Lock) error {
	for i := 0; i < maxCleanLockRetryCount; i++ {
		if exists, err := txn.lockCleaner.IsLockExists(lock.Coordinate(), 0, lock.Timestamp()); err != nil || !exists {
			return errors.Trace(err)
		}
		log.Warnf("lock exists txn: %v lock-txn: %v row: %q", txn.startTs, lock.Timestamp(), lock.Coordinate().Row)
		// try clean lock
		err := txn.tryToCleanLock(lock)
		if errorEqual(err, ErrLockNotExpired) {
			log.Warn("sleep a while, and retry clean lock", txn.startTs)
			// TODO(dongxu) use cleverer retry sleep time interval
			time.Sleep(pauseTime)
			continue
		} else if err != nil {
			return errors.Trace(err)
		}
		// lock cleaned successfully
		return nil
	}
	return ErrCleanLockFailed
}

func (txn *themisTxn) tryToCleanLock(lock Lock) error {
	// if it's secondary lock, first we'll check if its primary lock has been released.
	if lock.Role() == RoleSecondary {
		// get primary lock
		pl := lock.Primary()
		// check primary lock is exists
		exists, err := txn.lockCleaner.IsLockExists(pl.Coordinate(), 0, pl.Timestamp())
		if err != nil {
			return errors.Trace(err)
		}
		if !exists {
			// primary row is committed, commit this row
			cc := pl.Coordinate()
			commitTs, err := txn.lockCleaner.GetCommitTimestamp(cc, pl.Timestamp())
			if err != nil {
				return errors.Trace(err)
			}
			if commitTs > 0 {
				// if this transction has been committed
				log.Info("txn has been committed, ts:", commitTs, "prewriteTs:", pl.Timestamp())
				// commit secondary rows
				err := txn.commitSecondaryAndCleanLock(lock.(*themisSecondaryLock), commitTs)
				if err != nil {
					return errors.Trace(err)
				}
				return nil
			}
		}
	}
	expired, err := txn.rpc.checkAndSetLockIsExpired(lock)
	if err != nil {
		return errors.Trace(err)
	}
	// only clean expired lock
	if expired {
		// try to clean primary lock
		pl := lock.Primary()
		commitTs, cleanedLock, err := txn.lockCleaner.CleanLock(pl.Coordinate(), pl.Timestamp())
		if err != nil {
			return errors.Trace(err)
		}
		if cleanedLock != nil {
			pl = cleanedLock
		}
		log.Info("try clean secondary locks", pl.Timestamp())
		// clean secondary locks
		// erase lock and data if commitTs is 0; otherwise, commit it.
		for k, v := range pl.(*themisPrimaryLock).secondaries {
			cc := &hbase.ColumnCoordinate{}
			if err = cc.ParseFromString(k); err != nil {
				return errors.Trace(err)
			}
			if commitTs == 0 {
				// commitTs == 0, means clean primary lock successfully
				// expire trx havn't committed yet, we must delete lock and
				// dirty data
				err = txn.lockCleaner.EraseLockAndData(cc, pl.Timestamp())
				if err != nil {
					return errors.Trace(err)
				}
			} else {
				// primary row is committed, so we must commit other
				// secondary rows
				mutation := &columnMutation{
					Column: &cc.Column,
					mutationValuePair: &mutationValuePair{
						typ: v,
					},
				}
				err = txn.rpc.commitSecondaryRow(cc.Table, cc.Row,
					[]*columnMutation{mutation}, pl.Timestamp(), commitTs)
				if err != nil {
					return errors.Trace(err)
				}
			}
		}
	} else {
		return ErrLockNotExpired
	}
	return nil
}

func (txn *themisTxn) batchPrewriteSecondaryRowsWithLockClean(tbl []byte, rowMs map[string]*rowMutation) error {
	locks, err := txn.batchPrewriteSecondaryRows(tbl, rowMs)
	if err != nil {
		return errors.Trace(err)
	}

	// lock clean
	if locks != nil && len(locks) > 0 {
		// hook for test
		if bypass, _, err := txn.hooks.onSecondaryOccursLock(txn, locks); !bypass {
			return errors.Trace(err)
		}
		// try one more time after clean lock successfully
		for _, lock := range locks {
			err = txn.cleanLockWithRetry(lock)
			if err != nil {
				return errors.Trace(err)
			}

			// prewrite all secondary rows
			locks, err = txn.batchPrewriteSecondaryRows(tbl, rowMs)
			if err != nil {
				return errors.Trace(err)
			}
			if len(locks) > 0 {
				for _, l := range locks {
					log.Errorf("can't clean lock, column:%q; conflict lock: %+v, lock ts: %d", l.Coordinate(), l, l.Timestamp())
				}
				return ErrRetryable
			}
		}
	}
	return nil
}

func (txn *themisTxn) prewriteRowWithLockClean(tbl []byte, mutation *rowMutation, containPrimary bool) error {
	lock, err := txn.prewriteRow(tbl, mutation, containPrimary)
	if err != nil {
		return errors.Trace(err)
	}
	// lock clean
	if lock != nil {
		// hook for test
		if bypass, _, err := txn.hooks.beforePrewriteLockClean(txn, lock); !bypass {
			return errors.Trace(err)
		}
		err = txn.cleanLockWithRetry(lock)
		if err != nil {
			return errors.Trace(err)
		}
		// try one more time after clean lock successfully
		lock, err = txn.prewriteRow(tbl, mutation, containPrimary)
		if err != nil {
			return errors.Trace(err)
		}
		if lock != nil {
			log.Errorf("can't clean lock, column:%q; conflict lock: %+v, lock ts: %d", lock.Coordinate(), lock, lock.Timestamp())
			return ErrRetryable
		}
	}
	return nil
}

func (txn *themisTxn) batchPrewriteSecondaryRows(tbl []byte, rowMs map[string]*rowMutation) (map[string]Lock, error) {
	return txn.rpc.batchPrewriteSecondaryRows(tbl, rowMs, txn.startTs, txn.secondaryLockBytes)
}

func (txn *themisTxn) prewriteRow(tbl []byte, mutation *rowMutation, containPrimary bool) (Lock, error) {
	// hook for test
	if bypass, ret, err := txn.hooks.onPrewriteRow(txn, []interface{}{mutation, containPrimary}); !bypass {
		return ret.(Lock), errors.Trace(err)
	}
	if containPrimary {
		// try to get lock
		return txn.rpc.prewriteRow(tbl, mutation.row,
			mutation.mutationList(true),
			txn.startTs,
			txn.constructPrimaryLock().Encode(),
			txn.secondaryLockBytes, txn.primaryRowOffset)
	}
	return txn.rpc.prewriteSecondaryRow(tbl, mutation.row,
		mutation.mutationList(true),
		txn.startTs,
		txn.secondaryLockBytes)
}

func (txn *themisTxn) prewritePrimary() error {
	// hook for test
	if bypass, _, err := txn.hooks.beforePrewritePrimary(txn, nil); !bypass {
		return err
	}
	err := txn.prewriteRowWithLockClean(txn.primary.Table, txn.primaryRow, true)
	if err != nil {
		log.Debugf("prewrite primary %v %q failed: %v", txn.startTs, txn.primaryRow.row, err.Error())
		return errors.Trace(err)
	}
	log.Debugf("prewrite primary %v %q successfully", txn.startTs, txn.primaryRow.row)
	return nil
}

func (txn *themisTxn) prewriteSecondary() error {
	// hook for test
	if bypass, _, err := txn.hooks.beforePrewriteSecondary(txn, nil); !bypass {
		return err
	}
	if txn.conf.brokenPrewriteSecondaryTest {
		return txn.brokenPrewriteSecondary()
	}
	if txn.conf.ConcurrentPrewriteAndCommit {
		return txn.batchPrewriteSecondaries()
	}
	return txn.prewriteSecondarySync()
}

func (txn *themisTxn) prewriteSecondarySync() error {
	for i, mu := range txn.secondaryRows {
		err := txn.prewriteRowWithLockClean(mu.tbl, mu, false)
		if err != nil {
			// rollback
			txn.rollbackRow(txn.primaryRow.tbl, txn.primaryRow)
			txn.rollbackSecondaryRow(i)
			return errors.Trace(err)
		}
	}
	return nil
}

// just for test
func (txn *themisTxn) brokenCommitPrimary() error {
	// do nothing
	log.Warn("Simulating primary commit failed")
	return nil
}

// just for test
func (txn *themisTxn) brokenCommitSecondary() {
	// do nothing
	log.Warn("Simulating secondary commit failed")
}

func (txn *themisTxn) brokenPrewriteSecondary() error {
	log.Warn("Simulating prewrite secondary failed")
	for i, rm := range txn.secondaryRows {
		if i == len(txn.secondary)-1 {
			if !txn.conf.brokenPrewriteSecondaryAndRollbackTest {
				// simulating prewrite failed, need rollback
				txn.rollbackRow(txn.primaryRow.tbl, txn.primaryRow)
				txn.rollbackSecondaryRow(i)
			}
			// maybe rollback occurs error too
			return ErrSimulated
		}
		txn.prewriteRowWithLockClean(rm.tbl, rm, false)
	}
	return nil
}

func (txn *themisTxn) batchPrewriteSecondaries() error {
	wg := sync.WaitGroup{}
	//will batch prewrite all rows in a region
	rsRowMap, err := txn.groupByRegion()
	if err != nil {
		return errors.Trace(err)
	}

	errChan := make(chan error, len(rsRowMap))
	defer close(errChan)
	successChan := make(chan map[string]*rowMutation, len(rsRowMap))
	defer close(successChan)

	for _, regionRowMap := range rsRowMap {
		wg.Add(1)
		_, firstRowM := getFirstEntity(regionRowMap)
		go func(tbl []byte, rMap map[string]*rowMutation) {
			defer wg.Done()
			err := txn.batchPrewriteSecondaryRowsWithLockClean(tbl, rMap)
			if err != nil {
				errChan <- err
			} else {
				successChan <- rMap
			}
		}(firstRowM.tbl, regionRowMap)
	}
	wg.Wait()

	if len(errChan) != 0 {
		// occur error, clean success prewrite mutations
		log.Warnf("batch prewrite secondary rows error, rolling back %d %d", len(successChan), txn.startTs)
		txn.rollbackRow(txn.primaryRow.tbl, txn.primaryRow)
	L:
		for {
			select {
			case succMutMap := <-successChan:
				{
					for _, rowMut := range succMutMap {
						txn.rollbackRow(rowMut.tbl, rowMut)
					}
				}
			default:
				break L
			}
		}

		err := <-errChan
		if err != nil {
			log.Error("batch prewrite secondary rows error, txn:", txn.startTs, err)
		}
		return errors.Trace(err)
	}
	return nil
}

func getFirstEntity(rowMap map[string]*rowMutation) (string, *rowMutation) {
	for row, rowM := range rowMap {
		return row, rowM
	}
	return "", nil
}

func getBatchGroupKey(rInfo *hbase.RegionInfo, tblName string) string {
	return rInfo.Server + "_" + rInfo.Name
}

func (txn *themisTxn) rollbackRow(tbl []byte, mutation *rowMutation) error {
	l := fmt.Sprintf("\nrolling back %q %d {\n", mutation.row, txn.startTs)
	for _, v := range mutation.getColumns() {
		l += fmt.Sprintf("\t%s:%s\n", string(v.Family), string(v.Qual))
	}
	l += "}\n"
	log.Warn(l)
	for _, col := range mutation.getColumns() {
		cc := &hbase.ColumnCoordinate{
			Table:  tbl,
			Row:    mutation.row,
			Column: col,
		}
		err := txn.lockCleaner.EraseLockAndData(cc, txn.startTs)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (txn *themisTxn) rollbackSecondaryRow(successIndex int) error {
	for i := successIndex; i >= 0; i-- {
		r := txn.secondaryRows[i]
		err := txn.rollbackRow(r.tbl, r)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (txn *themisTxn) GetScanner(tbl []byte, startKey, endKey []byte, batchSize int) *ThemisScanner {
	scanner := newThemisScanner(tbl, txn, batchSize, txn.client)
	if startKey != nil {
		scanner.setStartRow(startKey)
	}
	if endKey != nil {
		scanner.setStopRow(endKey)
	}
	return scanner
}

func (txn *themisTxn) Release() {
	txn.primary = nil
	txn.primaryRow = nil
	txn.secondary = nil
	txn.secondaryRows = nil
	txn.startTs = 0
	txn.commitTs = 0
}

func (txn *themisTxn) String() string {
	return fmt.Sprintf("%d", txn.startTs)
}

func (txn *themisTxn) GetCommitTS() uint64 {
	return txn.commitTs
}

func (txn *themisTxn) GetStartTS() uint64 {
	return txn.startTs
}

func (txn *themisTxn) LockRow(tbl string, rowkey []byte) error {
	g := hbase.NewGet(rowkey)
	r, err := txn.Get(tbl, g)
	if err != nil {
		log.Warnf("get row error, table:%s, row:%q, error:%v", tbl, rowkey, err)
		return errors.Trace(err)
	}
	if r == nil {
		log.Warnf("has not data to lock, table:%s, row:%q", tbl, rowkey)
		return nil
	}
	for _, v := range r.Columns {
		txn.mutationCache.addMutation([]byte(tbl), rowkey, &v.Column, hbase.TypeMinimum, nil, true)
	}
	return nil
}
