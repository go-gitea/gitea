// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

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

package tidb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/executor"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/perfschema"
	"github.com/pingcap/tidb/privilege"
	"github.com/pingcap/tidb/privilege/privileges"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/autocommit"
	"github.com/pingcap/tidb/sessionctx/db"
	"github.com/pingcap/tidb/sessionctx/forupdate"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/store/localstore"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util"
	"github.com/pingcap/tidb/util/types"
)

// Session context
type Session interface {
	Status() uint16                              // Flag of current status, such as autocommit
	LastInsertID() uint64                        // Last inserted auto_increment id
	AffectedRows() uint64                        // Affected rows by lastest executed stmt
	Execute(sql string) ([]ast.RecordSet, error) // Execute a sql statement
	String() string                              // For debug
	FinishTxn(rollback bool) error
	// For execute prepare statement in binary protocol
	PrepareStmt(sql string) (stmtID uint32, paramCount int, fields []*ast.ResultField, err error)
	// Execute a prepared statement
	ExecutePreparedStmt(stmtID uint32, param ...interface{}) (ast.RecordSet, error)
	DropPreparedStmt(stmtID uint32) error
	SetClientCapability(uint32) // Set client capability flags
	SetConnectionID(uint64)
	Close() error
	Retry() error
	Auth(user string, auth []byte, salt []byte) bool
}

var (
	_         Session = (*session)(nil)
	sessionID int64
	sessionMu sync.Mutex
)

type stmtRecord struct {
	stmtID uint32
	st     ast.Statement
	params []interface{}
}

type stmtHistory struct {
	history []*stmtRecord
}

func (h *stmtHistory) add(stmtID uint32, st ast.Statement, params ...interface{}) {
	s := &stmtRecord{
		stmtID: stmtID,
		st:     st,
		params: append(([]interface{})(nil), params...),
	}
	h.history = append(h.history, s)
}

func (h *stmtHistory) reset() {
	if len(h.history) > 0 {
		h.history = h.history[:0]
	}
}

func (h *stmtHistory) clone() *stmtHistory {
	nh := *h
	nh.history = make([]*stmtRecord, len(h.history))
	copy(nh.history, h.history)
	return &nh
}

const unlimitedRetryCnt = -1

type session struct {
	txn         kv.Transaction // Current transaction
	args        []interface{}  // Statment execution args, this should be cleaned up after exec
	values      map[fmt.Stringer]interface{}
	store       kv.Storage
	sid         int64
	history     stmtHistory
	initing     bool // Running bootstrap using this session.
	retrying    bool
	maxRetryCnt int // Max retry times. If maxRetryCnt <=0, there is no limitation for retry times.

	debugInfos map[string]interface{} // Vars for debug and unit tests.

	// For performance_schema only.
	stmtState *perfschema.StatementState
}

func (s *session) Status() uint16 {
	return variable.GetSessionVars(s).Status
}

func (s *session) LastInsertID() uint64 {
	return variable.GetSessionVars(s).LastInsertID
}

func (s *session) AffectedRows() uint64 {
	return variable.GetSessionVars(s).AffectedRows
}

func (s *session) resetHistory() {
	s.ClearValue(forupdate.ForUpdateKey)
	s.history.reset()
}

func (s *session) SetClientCapability(capability uint32) {
	variable.GetSessionVars(s).ClientCapability = capability
}

func (s *session) SetConnectionID(connectionID uint64) {
	variable.GetSessionVars(s).ConnectionID = connectionID
}

func (s *session) FinishTxn(rollback bool) error {
	// transaction has already been committed or rolled back
	if s.txn == nil {
		return nil
	}
	defer func() {
		s.txn = nil
		variable.GetSessionVars(s).SetStatusFlag(mysql.ServerStatusInTrans, false)
	}()

	if rollback {
		s.resetHistory()
		return s.txn.Rollback()
	}

	err := s.txn.Commit()
	if err != nil {
		if !s.retrying && kv.IsRetryableError(err) {
			err = s.Retry()
		}
		if err != nil {
			log.Warnf("txn:%s, %v", s.txn, err)
			return errors.Trace(err)
		}
	}

	s.resetHistory()
	return nil
}

func (s *session) String() string {
	// TODO: how to print binded context in values appropriately?
	data := map[string]interface{}{
		"currDBName": db.GetCurrentSchema(s),
		"sid":        s.sid,
	}

	if s.txn != nil {
		// if txn is committed or rolled back, txn is nil.
		data["txn"] = s.txn.String()
	}

	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b)
}

func (s *session) Retry() error {
	s.retrying = true
	nh := s.history.clone()
	// Debug infos.
	if len(nh.history) == 0 {
		s.debugInfos[retryEmptyHistoryList] = true
	} else {
		s.debugInfos[retryEmptyHistoryList] = false
	}
	defer func() {
		s.history.history = nh.history
		s.retrying = false
	}()

	if forUpdate := s.Value(forupdate.ForUpdateKey); forUpdate != nil {
		return errors.Errorf("can not retry select for update statement")
	}
	var err error
	retryCnt := 0
	for {
		s.resetHistory()
		s.FinishTxn(true)
		success := true
		for _, sr := range nh.history {
			st := sr.st
			log.Warnf("Retry %s", st.OriginText())
			_, err = runStmt(s, st)
			if err != nil {
				if kv.IsRetryableError(err) {
					success = false
					break
				}
				log.Warnf("session:%v, err:%v", s, err)
				return errors.Trace(err)
			}
		}
		if success {
			err = s.FinishTxn(false)
			if !kv.IsRetryableError(err) {
				break
			}
		}
		retryCnt++
		if (s.maxRetryCnt != unlimitedRetryCnt) && (retryCnt >= s.maxRetryCnt) {
			return errors.Trace(err)
		}
		kv.BackOff(retryCnt)
	}
	return err
}

// ExecRestrictedSQL implements SQLHelper interface.
// This is used for executing some restricted sql statements.
func (s *session) ExecRestrictedSQL(ctx context.Context, sql string) (ast.RecordSet, error) {
	rawStmts, err := Parse(ctx, sql)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(rawStmts) != 1 {
		log.Errorf("ExecRestrictedSQL only executes one statement. Too many/few statement in %s", sql)
		return nil, errors.New("Wrong number of statement.")
	}
	st, err := Compile(s, rawStmts[0])
	if err != nil {
		log.Errorf("Compile %s with error: %v", sql, err)
		return nil, errors.Trace(err)
	}
	// Check statement for some restriction
	// For example only support DML on system meta table.
	// TODO: Add more restrictions.
	log.Debugf("Executing %s [%s]", st.OriginText(), sql)
	rs, err := st.Exec(ctx)
	return rs, errors.Trace(err)
}

// getExecRet executes restricted sql and the result is one column.
// It returns a string value.
func (s *session) getExecRet(ctx context.Context, sql string) (string, error) {
	cleanTxn := s.txn == nil
	rs, err := s.ExecRestrictedSQL(ctx, sql)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer rs.Close()
	row, err := rs.Next()
	if err != nil {
		return "", errors.Trace(err)
	}
	if row == nil {
		return "", terror.ExecResultIsEmpty
	}
	value, err := types.ToString(row.Data[0].GetValue())
	if err != nil {
		return "", errors.Trace(err)
	}
	if cleanTxn {
		// This function has some side effect. Run select may create new txn.
		// We should make environment unchanged.
		s.txn = nil
	}
	return value, nil
}

// GetGlobalSysVar implements GlobalVarAccessor.GetGlobalSysVar interface.
func (s *session) GetGlobalSysVar(ctx context.Context, name string) (string, error) {
	sql := fmt.Sprintf(`SELECT VARIABLE_VALUE FROM %s.%s WHERE VARIABLE_NAME="%s";`,
		mysql.SystemDB, mysql.GlobalVariablesTable, name)
	sysVar, err := s.getExecRet(ctx, sql)
	if err != nil {
		if terror.ExecResultIsEmpty.Equal(err) {
			return "", variable.UnknownSystemVar.Gen("unknown sys variable:%s", name)
		}
		return "", errors.Trace(err)
	}
	return sysVar, nil
}

// SetGlobalSysVar implements GlobalVarAccessor.SetGlobalSysVar interface.
func (s *session) SetGlobalSysVar(ctx context.Context, name string, value string) error {
	sql := fmt.Sprintf(`UPDATE  %s.%s SET VARIABLE_VALUE="%s" WHERE VARIABLE_NAME="%s";`,
		mysql.SystemDB, mysql.GlobalVariablesTable, value, strings.ToLower(name))
	_, err := s.ExecRestrictedSQL(ctx, sql)
	return errors.Trace(err)
}

// IsAutocommit checks if it is in the auto-commit mode.
func (s *session) isAutocommit(ctx context.Context) bool {
	autocommit, ok := variable.GetSessionVars(ctx).Systems["autocommit"]
	if !ok {
		if s.initing {
			return false
		}
		var err error
		autocommit, err = s.GetGlobalSysVar(ctx, "autocommit")
		if err != nil {
			log.Errorf("Get global sys var error: %v", err)
			return false
		}
		variable.GetSessionVars(ctx).Systems["autocommit"] = autocommit
		ok = true
	}
	if ok && (autocommit == "ON" || autocommit == "on" || autocommit == "1") {
		variable.GetSessionVars(ctx).SetStatusFlag(mysql.ServerStatusAutocommit, true)
		return true
	}
	variable.GetSessionVars(ctx).SetStatusFlag(mysql.ServerStatusAutocommit, false)
	return false
}

func (s *session) ShouldAutocommit(ctx context.Context) bool {
	// With START TRANSACTION, autocommit remains disabled until you end
	// the transaction with COMMIT or ROLLBACK.
	if variable.GetSessionVars(ctx).Status&mysql.ServerStatusInTrans == 0 && s.isAutocommit(ctx) {
		return true
	}
	return false
}

func (s *session) Execute(sql string) ([]ast.RecordSet, error) {
	rawStmts, err := Parse(s, sql)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var rs []ast.RecordSet
	for i, rst := range rawStmts {
		st, err1 := Compile(s, rst)
		if err1 != nil {
			log.Errorf("Syntax error: %s", sql)
			log.Errorf("Error occurs at %s.", err1)
			return nil, errors.Trace(err1)
		}
		id := variable.GetSessionVars(s).ConnectionID
		s.stmtState = perfschema.PerfHandle.StartStatement(sql, id, perfschema.CallerNameSessionExecute, rawStmts[i])
		r, err := runStmt(s, st)
		perfschema.PerfHandle.EndStatement(s.stmtState)
		if err != nil {
			log.Warnf("session:%v, err:%v", s, err)
			return nil, errors.Trace(err)
		}
		if r != nil {
			rs = append(rs, r)
		}
	}
	return rs, nil
}

// For execute prepare statement in binary protocol
func (s *session) PrepareStmt(sql string) (stmtID uint32, paramCount int, fields []*ast.ResultField, err error) {
	prepareExec := &executor.PrepareExec{
		IS:      sessionctx.GetDomain(s).InfoSchema(),
		Ctx:     s,
		SQLText: sql,
	}
	prepareExec.DoPrepare()
	return prepareExec.ID, prepareExec.ParamCount, prepareExec.ResultFields, prepareExec.Err
}

// checkArgs makes sure all the arguments' types are known and can be handled.
// integer types are converted to int64 and uint64, time.Time is converted to mysql.Time.
// time.Duration is converted to mysql.Duration, other known types are leaved as it is.
func checkArgs(args ...interface{}) error {
	for i, v := range args {
		switch x := v.(type) {
		case bool:
			if x {
				args[i] = int64(1)
			} else {
				args[i] = int64(0)
			}
		case int8:
			args[i] = int64(x)
		case int16:
			args[i] = int64(x)
		case int32:
			args[i] = int64(x)
		case int:
			args[i] = int64(x)
		case uint8:
			args[i] = uint64(x)
		case uint16:
			args[i] = uint64(x)
		case uint32:
			args[i] = uint64(x)
		case uint:
			args[i] = uint64(x)
		case int64:
		case uint64:
		case float32:
		case float64:
		case string:
		case []byte:
		case time.Duration:
			args[i] = mysql.Duration{Duration: x}
		case time.Time:
			args[i] = mysql.Time{Time: x, Type: mysql.TypeDatetime}
		case nil:
		default:
			return errors.Errorf("cannot use arg[%d] (type %T):unsupported type", i, v)
		}
	}
	return nil
}

// Execute a prepared statement
func (s *session) ExecutePreparedStmt(stmtID uint32, args ...interface{}) (ast.RecordSet, error) {
	err := checkArgs(args...)
	if err != nil {
		return nil, err
	}
	st := executor.CompileExecutePreparedStmt(s, stmtID, args...)
	r, err := runStmt(s, st, args...)
	return r, errors.Trace(err)
}

func (s *session) DropPreparedStmt(stmtID uint32) error {
	vars := variable.GetSessionVars(s)
	if _, ok := vars.PreparedStmts[stmtID]; !ok {
		return executor.ErrStmtNotFound
	}
	delete(vars.PreparedStmts, stmtID)
	return nil
}

// If forceNew is true, GetTxn() must return a new transaction.
// In this situation, if current transaction is still in progress,
// there will be an implicit commit and create a new transaction.
func (s *session) GetTxn(forceNew bool) (kv.Transaction, error) {
	var err error
	if s.txn == nil {
		s.resetHistory()
		s.txn, err = s.store.Begin()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !s.isAutocommit(s) {
			variable.GetSessionVars(s).SetStatusFlag(mysql.ServerStatusInTrans, true)
		}
		log.Infof("New txn:%s in session:%d", s.txn, s.sid)
		return s.txn, nil
	}
	if forceNew {
		err = s.FinishTxn(false)
		if err != nil {
			return nil, errors.Trace(err)
		}
		s.txn, err = s.store.Begin()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !s.isAutocommit(s) {
			variable.GetSessionVars(s).SetStatusFlag(mysql.ServerStatusInTrans, true)
		}
		log.Warnf("Force new txn:%s in session:%d", s.txn, s.sid)
	}
	return s.txn, nil
}

func (s *session) SetValue(key fmt.Stringer, value interface{}) {
	s.values[key] = value
}

func (s *session) Value(key fmt.Stringer) interface{} {
	value := s.values[key]
	return value
}

func (s *session) ClearValue(key fmt.Stringer) {
	delete(s.values, key)
}

// Close function does some clean work when session end.
func (s *session) Close() error {
	return s.FinishTxn(true)
}

func (s *session) getPassword(name, host string) (string, error) {
	// Get password for name and host.
	authSQL := fmt.Sprintf("SELECT Password FROM %s.%s WHERE User='%s' and Host='%s';", mysql.SystemDB, mysql.UserTable, name, host)
	pwd, err := s.getExecRet(s, authSQL)
	if err == nil {
		return pwd, nil
	} else if !terror.ExecResultIsEmpty.Equal(err) {
		return "", errors.Trace(err)
	}
	//Try to get user password for name with any host(%).
	authSQL = fmt.Sprintf("SELECT Password FROM %s.%s WHERE User='%s' and Host='%%';", mysql.SystemDB, mysql.UserTable, name)
	pwd, err = s.getExecRet(s, authSQL)
	return pwd, errors.Trace(err)
}

func (s *session) Auth(user string, auth []byte, salt []byte) bool {
	strs := strings.Split(user, "@")
	if len(strs) != 2 {
		log.Warnf("Invalid format for user: %s", user)
		return false
	}
	// Get user password.
	name := strs[0]
	host := strs[1]
	pwd, err := s.getPassword(name, host)
	if err != nil {
		if terror.ExecResultIsEmpty.Equal(err) {
			log.Errorf("User [%s] not exist %v", name, err)
		} else {
			log.Errorf("Get User [%s] password from SystemDB error %v", name, err)
		}
		return false
	}
	if len(pwd) != 0 && len(pwd) != 40 {
		log.Errorf("User [%s] password from SystemDB not like a sha1sum", name)
		return false
	}
	hpwd, err := util.DecodePassword(pwd)
	if err != nil {
		log.Errorf("Decode password string error %v", err)
		return false
	}
	checkAuth := util.CalcPassword(salt, hpwd)
	if !bytes.Equal(auth, checkAuth) {
		return false
	}
	variable.GetSessionVars(s).SetCurrentUser(user)
	return true
}

// Some vars name for debug.
const (
	retryEmptyHistoryList = "RetryEmptyHistoryList"
)

// CreateSession creates a new session environment.
func CreateSession(store kv.Storage) (Session, error) {
	s := &session{
		values:      make(map[fmt.Stringer]interface{}),
		store:       store,
		sid:         atomic.AddInt64(&sessionID, 1),
		debugInfos:  make(map[string]interface{}),
		retrying:    false,
		maxRetryCnt: 10,
	}
	domain, err := domap.Get(store)
	if err != nil {
		return nil, err
	}
	sessionctx.BindDomain(s, domain)

	variable.BindSessionVars(s)
	variable.GetSessionVars(s).SetStatusFlag(mysql.ServerStatusAutocommit, true)

	// session implements variable.GlobalVarAccessor. Bind it to ctx.
	variable.BindGlobalVarAccessor(s, s)

	// session implements autocommit.Checker. Bind it to ctx
	autocommit.BindAutocommitChecker(s, s)
	sessionMu.Lock()
	defer sessionMu.Unlock()

	ok := isBoostrapped(store)
	if !ok {
		// if no bootstrap and storage is remote, we must use a little lease time to
		// bootstrap quickly, after bootstrapped, we will reset the lease time.
		// TODO: Using a bootstap tool for doing this may be better later.
		if !localstore.IsLocalStore(store) {
			sessionctx.GetDomain(s).SetLease(100 * time.Millisecond)
		}

		s.initing = true
		bootstrap(s)
		s.initing = false

		if !localstore.IsLocalStore(store) {
			sessionctx.GetDomain(s).SetLease(schemaLease)
		}

		finishBoostrap(store)
	}

	// TODO: Add auth here
	privChecker := &privileges.UserPrivileges{}
	privilege.BindPrivilegeChecker(s, privChecker)
	return s, nil
}

func isBoostrapped(store kv.Storage) bool {
	// check in memory
	_, ok := storeBootstrapped[store.UUID()]
	if ok {
		return true
	}

	// check in kv store
	err := kv.RunInNewTxn(store, false, func(txn kv.Transaction) error {
		var err error
		t := meta.NewMeta(txn)
		ok, err = t.IsBootstrapped()
		return errors.Trace(err)
	})

	if err != nil {
		log.Fatalf("check bootstrapped err %v", err)
	}

	if ok {
		// here mean memory is not ok, but other server has already finished it
		storeBootstrapped[store.UUID()] = true
	}

	return ok
}

func finishBoostrap(store kv.Storage) {
	storeBootstrapped[store.UUID()] = true

	err := kv.RunInNewTxn(store, true, func(txn kv.Transaction) error {
		t := meta.NewMeta(txn)
		err := t.FinishBootstrap()
		return errors.Trace(err)
	})
	if err != nil {
		log.Fatalf("finish bootstrap err %v", err)
	}
}
