// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package meta

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/structure"
)

var (
	globalIDMutex sync.Mutex
)

// Meta structure:
//	NextGlobalID -> int64
//	SchemaVersion -> int64
//	DBs -> {
//		DB:1 -> db meta data []byte
//		DB:2 -> db meta data []byte
//	}
//	DB:1 -> {
//		Table:1 -> table meta data []byte
//		Table:2 -> table meta data []byte
//		TID:1 -> int64
//		TID:2 -> int64
//	}
//

var (
	mNextGlobalIDKey  = []byte("NextGlobalID")
	mSchemaVersionKey = []byte("SchemaVersionKey")
	mDBs              = []byte("DBs")
	mDBPrefix         = "DB"
	mTablePrefix      = "Table"
	mTableIDPrefix    = "TID"
	mBootstrapKey     = []byte("BootstrapKey")
)

var (
	// ErrDBExists is the error for db exists.
	ErrDBExists = errors.New("database already exists")
	// ErrDBNotExists is the error for db not exists.
	ErrDBNotExists = errors.New("database doesn't exist")
	// ErrTableExists is the error for table exists.
	ErrTableExists = errors.New("table already exists")
	// ErrTableNotExists is the error for table not exists.
	ErrTableNotExists = errors.New("table doesn't exist")
)

// Meta is for handling meta information in a transaction.
type Meta struct {
	txn *structure.TxStructure
}

// NewMeta creates a Meta in transaction txn.
func NewMeta(txn kv.Transaction) *Meta {
	t := structure.NewStructure(txn, []byte{'m'})
	return &Meta{txn: t}
}

// GenGlobalID generates next id globally.
func (m *Meta) GenGlobalID() (int64, error) {
	globalIDMutex.Lock()
	defer globalIDMutex.Unlock()

	return m.txn.Inc(mNextGlobalIDKey, 1)
}

// GetGlobalID gets current global id.
func (m *Meta) GetGlobalID() (int64, error) {
	return m.txn.GetInt64(mNextGlobalIDKey)
}

func (m *Meta) dbKey(dbID int64) []byte {
	return []byte(fmt.Sprintf("%s:%d", mDBPrefix, dbID))
}

func (m *Meta) parseDatabaseID(key string) (int64, error) {
	seps := strings.Split(key, ":")
	if len(seps) != 2 {
		return 0, errors.Errorf("invalid db key %s", key)
	}

	n, err := strconv.ParseInt(seps[1], 10, 64)
	return n, errors.Trace(err)
}

func (m *Meta) autoTalbeIDKey(tableID int64) []byte {
	return []byte(fmt.Sprintf("%s:%d", mTableIDPrefix, tableID))
}

func (m *Meta) tableKey(tableID int64) []byte {
	return []byte(fmt.Sprintf("%s:%d", mTablePrefix, tableID))
}

func (m *Meta) parseTableID(key string) (int64, error) {
	seps := strings.Split(key, ":")
	if len(seps) != 2 {
		return 0, errors.Errorf("invalid table meta key %s", key)
	}

	n, err := strconv.ParseInt(seps[1], 10, 64)
	return n, errors.Trace(err)
}

// GenAutoTableID adds step to the auto id of the table and returns the sum.
func (m *Meta) GenAutoTableID(dbID int64, tableID int64, step int64) (int64, error) {
	// Check if db exists.
	dbKey := m.dbKey(dbID)
	if err := m.checkDBExists(dbKey); err != nil {
		return 0, errors.Trace(err)
	}

	// Check if table exists.
	tableKey := m.tableKey(tableID)
	if err := m.checkTableExists(dbKey, tableKey); err != nil {
		return 0, errors.Trace(err)
	}

	return m.txn.HInc(dbKey, m.autoTalbeIDKey(tableID), step)
}

// GetAutoTableID gets current auto id with table id.
func (m *Meta) GetAutoTableID(dbID int64, tableID int64) (int64, error) {
	return m.txn.HGetInt64(m.dbKey(dbID), m.autoTalbeIDKey(tableID))
}

// GetSchemaVersion gets current global schema version.
func (m *Meta) GetSchemaVersion() (int64, error) {
	return m.txn.GetInt64(mSchemaVersionKey)
}

// GenSchemaVersion generates next schema version.
func (m *Meta) GenSchemaVersion() (int64, error) {
	return m.txn.Inc(mSchemaVersionKey, 1)
}

func (m *Meta) checkDBExists(dbKey []byte) error {
	v, err := m.txn.HGet(mDBs, dbKey)
	if err != nil {
		return errors.Trace(err)
	} else if v == nil {
		return ErrDBNotExists
	}

	return nil
}

func (m *Meta) checkDBNotExists(dbKey []byte) error {
	v, err := m.txn.HGet(mDBs, dbKey)
	if err != nil {
		return errors.Trace(err)
	}

	if v != nil {
		return ErrDBExists
	}

	return nil
}

func (m *Meta) checkTableExists(dbKey []byte, tableKey []byte) error {
	v, err := m.txn.HGet(dbKey, tableKey)
	if err != nil {
		return errors.Trace(err)
	}

	if v == nil {
		return ErrTableNotExists
	}

	return nil
}

func (m *Meta) checkTableNotExists(dbKey []byte, tableKey []byte) error {
	v, err := m.txn.HGet(dbKey, tableKey)
	if err != nil {
		return errors.Trace(err)
	}

	if v != nil {
		return ErrTableExists
	}

	return nil
}

// CreateDatabase creates a database with db info.
func (m *Meta) CreateDatabase(dbInfo *model.DBInfo) error {
	dbKey := m.dbKey(dbInfo.ID)

	if err := m.checkDBNotExists(dbKey); err != nil {
		return errors.Trace(err)
	}

	data, err := json.Marshal(dbInfo)
	if err != nil {
		return errors.Trace(err)
	}

	return m.txn.HSet(mDBs, dbKey, data)
}

// UpdateDatabase updates a database with db info.
func (m *Meta) UpdateDatabase(dbInfo *model.DBInfo) error {
	dbKey := m.dbKey(dbInfo.ID)

	if err := m.checkDBExists(dbKey); err != nil {
		return errors.Trace(err)
	}

	data, err := json.Marshal(dbInfo)
	if err != nil {
		return errors.Trace(err)
	}

	return m.txn.HSet(mDBs, dbKey, data)
}

// CreateTable creates a table with tableInfo in database.
func (m *Meta) CreateTable(dbID int64, tableInfo *model.TableInfo) error {
	// Check if db exists.
	dbKey := m.dbKey(dbID)
	if err := m.checkDBExists(dbKey); err != nil {
		return errors.Trace(err)
	}

	// Check if table exists.
	tableKey := m.tableKey(tableInfo.ID)
	if err := m.checkTableNotExists(dbKey, tableKey); err != nil {
		return errors.Trace(err)
	}

	data, err := json.Marshal(tableInfo)
	if err != nil {
		return errors.Trace(err)
	}

	return m.txn.HSet(dbKey, tableKey, data)
}

// DropDatabase drops whole database.
func (m *Meta) DropDatabase(dbID int64) error {
	// Check if db exists.
	dbKey := m.dbKey(dbID)
	if err := m.txn.HClear(dbKey); err != nil {
		return errors.Trace(err)
	}

	if err := m.txn.HDel(mDBs, dbKey); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// DropTable drops table in database.
func (m *Meta) DropTable(dbID int64, tableID int64) error {
	// Check if db exists.
	dbKey := m.dbKey(dbID)
	if err := m.checkDBExists(dbKey); err != nil {
		return errors.Trace(err)
	}

	// Check if table exists.
	tableKey := m.tableKey(tableID)
	if err := m.checkTableExists(dbKey, tableKey); err != nil {
		return errors.Trace(err)
	}

	if err := m.txn.HDel(dbKey, tableKey); err != nil {
		return errors.Trace(err)
	}

	if err := m.txn.HDel(dbKey, m.autoTalbeIDKey(tableID)); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// UpdateTable updates the table with table info.
func (m *Meta) UpdateTable(dbID int64, tableInfo *model.TableInfo) error {
	// Check if db exists.
	dbKey := m.dbKey(dbID)
	if err := m.checkDBExists(dbKey); err != nil {
		return errors.Trace(err)
	}

	// Check if table exists.
	tableKey := m.tableKey(tableInfo.ID)
	if err := m.checkTableExists(dbKey, tableKey); err != nil {
		return errors.Trace(err)
	}

	data, err := json.Marshal(tableInfo)
	if err != nil {
		return errors.Trace(err)
	}

	err = m.txn.HSet(dbKey, tableKey, data)
	return errors.Trace(err)
}

// ListTables shows all tables in database.
func (m *Meta) ListTables(dbID int64) ([]*model.TableInfo, error) {
	dbKey := m.dbKey(dbID)
	if err := m.checkDBExists(dbKey); err != nil {
		return nil, errors.Trace(err)
	}

	res, err := m.txn.HGetAll(dbKey)
	if err != nil {
		return nil, errors.Trace(err)
	}

	tables := make([]*model.TableInfo, 0, len(res)/2)
	for _, r := range res {
		// only handle table meta
		tableKey := string(r.Field)
		if !strings.HasPrefix(tableKey, mTablePrefix) {
			continue
		}

		tbInfo := &model.TableInfo{}
		err = json.Unmarshal(r.Value, tbInfo)
		if err != nil {
			return nil, errors.Trace(err)
		}

		tables = append(tables, tbInfo)
	}

	return tables, nil
}

// ListDatabases shows all databases.
func (m *Meta) ListDatabases() ([]*model.DBInfo, error) {
	res, err := m.txn.HGetAll(mDBs)
	if err != nil {
		return nil, errors.Trace(err)
	}

	dbs := make([]*model.DBInfo, 0, len(res))
	for _, r := range res {
		dbInfo := &model.DBInfo{}
		err = json.Unmarshal(r.Value, dbInfo)
		if err != nil {
			return nil, errors.Trace(err)
		}
		dbs = append(dbs, dbInfo)
	}
	return dbs, nil
}

// GetDatabase gets the database value with ID.
func (m *Meta) GetDatabase(dbID int64) (*model.DBInfo, error) {
	dbKey := m.dbKey(dbID)
	value, err := m.txn.HGet(mDBs, dbKey)
	if err != nil || value == nil {
		return nil, errors.Trace(err)
	}

	dbInfo := &model.DBInfo{}
	err = json.Unmarshal(value, dbInfo)
	return dbInfo, errors.Trace(err)
}

// GetTable gets the table value in database with tableID.
func (m *Meta) GetTable(dbID int64, tableID int64) (*model.TableInfo, error) {
	// Check if db exists.
	dbKey := m.dbKey(dbID)
	if err := m.checkDBExists(dbKey); err != nil {
		return nil, errors.Trace(err)
	}

	tableKey := m.tableKey(tableID)
	value, err := m.txn.HGet(dbKey, tableKey)
	if err != nil || value == nil {
		return nil, errors.Trace(err)
	}

	tableInfo := &model.TableInfo{}
	err = json.Unmarshal(value, tableInfo)
	return tableInfo, errors.Trace(err)
}

// DDL job structure
//	DDLOnwer: []byte
//	DDLJobList: list jobs
//	DDLJobHistory: hash
//	DDLJobReorg: hash
//
// for multi DDL workers, only one can become the owner
// to operate DDL jobs, and dispatch them to MR Jobs.

var (
	mDDLJobOwnerKey   = []byte("DDLJobOwner")
	mDDLJobListKey    = []byte("DDLJobList")
	mDDLJobHistoryKey = []byte("DDLJobHistory")
	mDDLJobReorgKey   = []byte("DDLJobReorg")
)

func (m *Meta) getJobOwner(key []byte) (*model.Owner, error) {
	value, err := m.txn.Get(key)
	if err != nil || value == nil {
		return nil, errors.Trace(err)
	}

	owner := &model.Owner{}
	err = json.Unmarshal(value, owner)
	return owner, errors.Trace(err)
}

// GetDDLJobOwner gets the current owner for DDL.
func (m *Meta) GetDDLJobOwner() (*model.Owner, error) {
	return m.getJobOwner(mDDLJobOwnerKey)
}

func (m *Meta) setJobOwner(key []byte, o *model.Owner) error {
	b, err := json.Marshal(o)
	if err != nil {
		return errors.Trace(err)
	}
	return m.txn.Set(key, b)
}

// SetDDLJobOwner sets the current owner for DDL.
func (m *Meta) SetDDLJobOwner(o *model.Owner) error {
	return m.setJobOwner(mDDLJobOwnerKey, o)
}

func (m *Meta) enQueueDDLJob(key []byte, job *model.Job) error {
	b, err := job.Encode()
	if err != nil {
		return errors.Trace(err)
	}
	return m.txn.RPush(key, b)
}

// EnQueueDDLJob adds a DDL job to the list.
func (m *Meta) EnQueueDDLJob(job *model.Job) error {
	return m.enQueueDDLJob(mDDLJobListKey, job)
}

func (m *Meta) deQueueDDLJob(key []byte) (*model.Job, error) {
	value, err := m.txn.LPop(key)
	if err != nil || value == nil {
		return nil, errors.Trace(err)
	}

	job := &model.Job{}
	err = job.Decode(value)
	return job, errors.Trace(err)
}

// DeQueueDDLJob pops a DDL job from the list.
func (m *Meta) DeQueueDDLJob() (*model.Job, error) {
	return m.deQueueDDLJob(mDDLJobListKey)
}

func (m *Meta) getDDLJob(key []byte, index int64) (*model.Job, error) {
	value, err := m.txn.LIndex(key, index)
	if err != nil || value == nil {
		return nil, errors.Trace(err)
	}

	job := &model.Job{}
	err = job.Decode(value)
	return job, errors.Trace(err)
}

// GetDDLJob returns the DDL job with index.
func (m *Meta) GetDDLJob(index int64) (*model.Job, error) {
	job, err := m.getDDLJob(mDDLJobListKey, index)
	return job, errors.Trace(err)
}

func (m *Meta) updateDDLJob(index int64, job *model.Job, key []byte) error {
	// TODO: use timestamp allocated by TSO
	job.LastUpdateTS = time.Now().UnixNano()
	b, err := job.Encode()
	if err != nil {
		return errors.Trace(err)
	}
	return m.txn.LSet(key, index, b)
}

// UpdateDDLJob updates the DDL job with index.
func (m *Meta) UpdateDDLJob(index int64, job *model.Job) error {
	return m.updateDDLJob(index, job, mDDLJobListKey)
}

// DDLJobQueueLen returns the DDL job queue length.
func (m *Meta) DDLJobQueueLen() (int64, error) {
	return m.txn.LLen(mDDLJobListKey)
}

func (m *Meta) jobIDKey(id int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(id))
	return b
}

func (m *Meta) addHistoryDDLJob(key []byte, job *model.Job) error {
	b, err := job.Encode()
	if err != nil {
		return errors.Trace(err)
	}

	return m.txn.HSet(key, m.jobIDKey(job.ID), b)
}

// AddHistoryDDLJob adds DDL job to history.
func (m *Meta) AddHistoryDDLJob(job *model.Job) error {
	return m.addHistoryDDLJob(mDDLJobHistoryKey, job)
}

func (m *Meta) getHistoryDDLJob(key []byte, id int64) (*model.Job, error) {
	value, err := m.txn.HGet(key, m.jobIDKey(id))
	if err != nil || value == nil {
		return nil, errors.Trace(err)
	}

	job := &model.Job{}
	err = job.Decode(value)
	return job, errors.Trace(err)
}

// GetHistoryDDLJob gets a history DDL job.
func (m *Meta) GetHistoryDDLJob(id int64) (*model.Job, error) {
	return m.getHistoryDDLJob(mDDLJobHistoryKey, id)
}

// IsBootstrapped returns whether we have already run bootstrap or not.
// return true means we don't need doing any other bootstrap.
func (m *Meta) IsBootstrapped() (bool, error) {
	value, err := m.txn.GetInt64(mBootstrapKey)
	if err != nil {
		return false, errors.Trace(err)
	}
	return value == 1, nil
}

// FinishBootstrap finishes bootstrap.
func (m *Meta) FinishBootstrap() error {
	err := m.txn.Set(mBootstrapKey, []byte("1"))
	return errors.Trace(err)
}

// UpdateDDLReorgHandle saves the job reorganization latest processed handle for later resuming.
func (m *Meta) UpdateDDLReorgHandle(job *model.Job, handle int64) error {
	err := m.txn.HSet(mDDLJobReorgKey, m.jobIDKey(job.ID), []byte(strconv.FormatInt(handle, 10)))
	return errors.Trace(err)
}

// RemoveDDLReorgHandle removes the job reorganization handle.
func (m *Meta) RemoveDDLReorgHandle(job *model.Job) error {
	err := m.txn.HDel(mDDLJobReorgKey, m.jobIDKey(job.ID))
	return errors.Trace(err)
}

// GetDDLReorgHandle gets the latest processed handle.
func (m *Meta) GetDDLReorgHandle(job *model.Job) (int64, error) {
	value, err := m.txn.HGetInt64(mDDLJobReorgKey, m.jobIDKey(job.ID))
	return value, errors.Trace(err)
}

// DDL background job structure
//	BgJobOnwer: []byte
//	BgJobList: list jobs
//	BgJobHistory: hash
//	BgJobReorg: hash
//
// for multi background worker, only one can become the owner
// to operate background job, and dispatch them to MR background job.

var (
	mBgJobOwnerKey   = []byte("BgJobOwner")
	mBgJobListKey    = []byte("BgJobList")
	mBgJobHistoryKey = []byte("BgJobHistory")
)

// UpdateBgJob updates the background job with index.
func (m *Meta) UpdateBgJob(index int64, job *model.Job) error {
	return m.updateDDLJob(index, job, mBgJobListKey)
}

// GetBgJob returns the background job with index.
func (m *Meta) GetBgJob(index int64) (*model.Job, error) {
	job, err := m.getDDLJob(mBgJobListKey, index)

	return job, errors.Trace(err)
}

// EnQueueBgJob adds a background job to the list.
func (m *Meta) EnQueueBgJob(job *model.Job) error {
	return m.enQueueDDLJob(mBgJobListKey, job)
}

// BgJobQueueLen returns the background job queue length.
func (m *Meta) BgJobQueueLen() (int64, error) {
	return m.txn.LLen(mBgJobListKey)
}

// AddHistoryBgJob adds background job to history.
func (m *Meta) AddHistoryBgJob(job *model.Job) error {
	return m.addHistoryDDLJob(mBgJobHistoryKey, job)
}

// GetHistoryBgJob gets a history background job.
func (m *Meta) GetHistoryBgJob(id int64) (*model.Job, error) {
	return m.getHistoryDDLJob(mBgJobHistoryKey, id)
}

// DeQueueBgJob pops a background job from the list.
func (m *Meta) DeQueueBgJob() (*model.Job, error) {
	return m.deQueueDDLJob(mBgJobListKey)
}

// GetBgJobOwner gets the current background job owner.
func (m *Meta) GetBgJobOwner() (*model.Owner, error) {
	return m.getJobOwner(mBgJobOwnerKey)
}

// SetBgJobOwner sets the current background job owner.
func (m *Meta) SetBgJobOwner(o *model.Owner) error {
	return m.setJobOwner(mBgJobOwnerKey, o)
}
