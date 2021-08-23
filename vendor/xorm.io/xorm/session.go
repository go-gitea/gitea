// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"reflect"
	"strconv"
	"strings"

	"xorm.io/xorm/contexts"
	"xorm.io/xorm/convert"
	"xorm.io/xorm/core"
	"xorm.io/xorm/internal/json"
	"xorm.io/xorm/internal/statements"
	"xorm.io/xorm/log"
	"xorm.io/xorm/schemas"
)

// ErrFieldIsNotExist columns does not exist
type ErrFieldIsNotExist struct {
	FieldName string
	TableName string
}

func (e ErrFieldIsNotExist) Error() string {
	return fmt.Sprintf("field %s is not exist on table %s", e.FieldName, e.TableName)
}

// ErrFieldIsNotValid is not valid
type ErrFieldIsNotValid struct {
	FieldName string
	TableName string
}

func (e ErrFieldIsNotValid) Error() string {
	return fmt.Sprintf("field %s is not valid on table %s", e.FieldName, e.TableName)
}

type sessionType bool

const (
	engineSession sessionType = false
	groupSession  sessionType = true
)

// Session keep a pointer to sql.DB and provides all execution of all
// kind of database operations.
type Session struct {
	engine                 *Engine
	tx                     *core.Tx
	statement              *statements.Statement
	isAutoCommit           bool
	isCommitedOrRollbacked bool
	isAutoClose            bool
	isClosed               bool
	prepareStmt            bool
	// Automatically reset the statement after operations that execute a SQL
	// query such as Count(), Find(), Get(), ...
	autoResetStatement bool

	// !nashtsai! storing these beans due to yet committed tx
	afterInsertBeans map[interface{}]*[]func(interface{})
	afterUpdateBeans map[interface{}]*[]func(interface{})
	afterDeleteBeans map[interface{}]*[]func(interface{})
	// --

	beforeClosures  []func(interface{})
	afterClosures   []func(interface{})
	afterProcessors []executedProcessor

	stmtCache map[uint32]*core.Stmt //key: hash.Hash32 of (queryStr, len(queryStr))

	lastSQL     string
	lastSQLArgs []interface{}

	ctx         context.Context
	sessionType sessionType
}

func newSessionID() string {
	hash := sha256.New()
	_, err := io.CopyN(hash, rand.Reader, 50)
	if err != nil {
		return "????????????????????"
	}
	md := hash.Sum(nil)
	mdStr := hex.EncodeToString(md)
	return mdStr[0:20]
}

func newSession(engine *Engine) *Session {
	var ctx context.Context
	if engine.logSessionID {
		ctx = context.WithValue(engine.defaultContext, log.SessionIDKey, newSessionID())
	} else {
		ctx = engine.defaultContext
	}

	session := &Session{
		ctx:    ctx,
		engine: engine,
		tx:     nil,
		statement: statements.NewStatement(
			engine.dialect,
			engine.tagParser,
			engine.DatabaseTZ,
		),
		isClosed:               false,
		isAutoCommit:           true,
		isCommitedOrRollbacked: false,
		isAutoClose:            false,
		autoResetStatement:     true,
		prepareStmt:            false,

		afterInsertBeans: make(map[interface{}]*[]func(interface{}), 0),
		afterUpdateBeans: make(map[interface{}]*[]func(interface{}), 0),
		afterDeleteBeans: make(map[interface{}]*[]func(interface{}), 0),
		beforeClosures:   make([]func(interface{}), 0),
		afterClosures:    make([]func(interface{}), 0),
		afterProcessors:  make([]executedProcessor, 0),
		stmtCache:        make(map[uint32]*core.Stmt),

		lastSQL:     "",
		lastSQLArgs: make([]interface{}, 0),

		sessionType: engineSession,
	}
	if engine.logSessionID {
		session.ctx = context.WithValue(session.ctx, log.SessionKey, session)
	}
	return session
}

// Close release the connection from pool
func (session *Session) Close() error {
	for _, v := range session.stmtCache {
		if err := v.Close(); err != nil {
			return err
		}
	}

	if !session.isClosed {
		// When Close be called, if session is a transaction and do not call
		// Commit or Rollback, then call Rollback.
		if session.tx != nil && !session.isCommitedOrRollbacked {
			if err := session.Rollback(); err != nil {
				return err
			}
		}
		session.tx = nil
		session.stmtCache = nil
		session.isClosed = true
	}
	return nil
}

func (session *Session) db() *core.DB {
	return session.engine.db
}

// Engine returns session Engine
func (session *Session) Engine() *Engine {
	return session.engine
}

func (session *Session) getQueryer() core.Queryer {
	if session.tx != nil {
		return session.tx
	}
	return session.db()
}

// ContextCache enable context cache or not
func (session *Session) ContextCache(context contexts.ContextCache) *Session {
	session.statement.SetContextCache(context)
	return session
}

// IsClosed returns if session is closed
func (session *Session) IsClosed() bool {
	return session.isClosed
}

func (session *Session) resetStatement() {
	if session.autoResetStatement {
		session.statement.Reset()
	}
}

// Prepare set a flag to session that should be prepare statement before execute query
func (session *Session) Prepare() *Session {
	session.prepareStmt = true
	return session
}

// Before Apply before Processor, affected bean is passed to closure arg
func (session *Session) Before(closures func(interface{})) *Session {
	if closures != nil {
		session.beforeClosures = append(session.beforeClosures, closures)
	}
	return session
}

// After Apply after Processor, affected bean is passed to closure arg
func (session *Session) After(closures func(interface{})) *Session {
	if closures != nil {
		session.afterClosures = append(session.afterClosures, closures)
	}
	return session
}

// Table can input a string or pointer to struct for special a table to operate.
func (session *Session) Table(tableNameOrBean interface{}) *Session {
	if err := session.statement.SetTable(tableNameOrBean); err != nil {
		session.statement.LastError = err
	}
	return session
}

// Alias set the table alias
func (session *Session) Alias(alias string) *Session {
	session.statement.Alias(alias)
	return session
}

// NoCascade indicate that no cascade load child object
func (session *Session) NoCascade() *Session {
	session.statement.UseCascade = false
	return session
}

// ForUpdate Set Read/Write locking for UPDATE
func (session *Session) ForUpdate() *Session {
	session.statement.IsForUpdate = true
	return session
}

// NoAutoCondition disable generate SQL condition from beans
func (session *Session) NoAutoCondition(no ...bool) *Session {
	session.statement.SetNoAutoCondition(no...)
	return session
}

// Limit provide limit and offset query condition
func (session *Session) Limit(limit int, start ...int) *Session {
	session.statement.Limit(limit, start...)
	return session
}

// OrderBy provide order by query condition, the input parameter is the content
// after order by on a sql statement.
func (session *Session) OrderBy(order string) *Session {
	session.statement.OrderBy(order)
	return session
}

// Desc provide desc order by query condition, the input parameters are columns.
func (session *Session) Desc(colNames ...string) *Session {
	session.statement.Desc(colNames...)
	return session
}

// Asc provide asc order by query condition, the input parameters are columns.
func (session *Session) Asc(colNames ...string) *Session {
	session.statement.Asc(colNames...)
	return session
}

// StoreEngine is only avialble mysql dialect currently
func (session *Session) StoreEngine(storeEngine string) *Session {
	session.statement.StoreEngine = storeEngine
	return session
}

// Charset is only avialble mysql dialect currently
func (session *Session) Charset(charset string) *Session {
	session.statement.Charset = charset
	return session
}

// Cascade indicates if loading sub Struct
func (session *Session) Cascade(trueOrFalse ...bool) *Session {
	if len(trueOrFalse) >= 1 {
		session.statement.UseCascade = trueOrFalse[0]
	}
	return session
}

// MustLogSQL means record SQL or not and don't follow engine's setting
func (session *Session) MustLogSQL(logs ...bool) *Session {
	var showSQL = true
	if len(logs) > 0 {
		showSQL = logs[0]
	}
	session.ctx = context.WithValue(session.ctx, log.SessionShowSQLKey, showSQL)
	return session
}

// NoCache ask this session do not retrieve data from cache system and
// get data from database directly.
func (session *Session) NoCache() *Session {
	session.statement.UseCache = false
	return session
}

// Join join_operator should be one of INNER, LEFT OUTER, CROSS etc - this will be prepended to JOIN
func (session *Session) Join(joinOperator string, tablename interface{}, condition string, args ...interface{}) *Session {
	session.statement.Join(joinOperator, tablename, condition, args...)
	return session
}

// GroupBy Generate Group By statement
func (session *Session) GroupBy(keys string) *Session {
	session.statement.GroupBy(keys)
	return session
}

// Having Generate Having statement
func (session *Session) Having(conditions string) *Session {
	session.statement.Having(conditions)
	return session
}

// DB db return the wrapper of sql.DB
func (session *Session) DB() *core.DB {
	return session.db()
}

func (session *Session) canCache() bool {
	if session.statement.RefTable == nil ||
		session.statement.JoinStr != "" ||
		session.statement.RawSQL != "" ||
		!session.statement.UseCache ||
		session.statement.IsForUpdate ||
		session.tx != nil ||
		len(session.statement.SelectStr) > 0 {
		return false
	}
	return true
}

func (session *Session) doPrepare(db *core.DB, sqlStr string) (stmt *core.Stmt, err error) {
	crc := crc32.ChecksumIEEE([]byte(sqlStr))
	// TODO try hash(sqlStr+len(sqlStr))
	var has bool
	stmt, has = session.stmtCache[crc]
	if !has {
		stmt, err = db.PrepareContext(session.ctx, sqlStr)
		if err != nil {
			return nil, err
		}
		session.stmtCache[crc] = stmt
	}
	return
}

func (session *Session) getField(dataStruct *reflect.Value, table *schemas.Table, colName string, idx int) (*schemas.Column, *reflect.Value, error) {
	var col = table.GetColumnIdx(colName, idx)
	if col == nil {
		return nil, nil, ErrFieldIsNotExist{colName, table.Name}
	}

	fieldValue, err := col.ValueOfV(dataStruct)
	if err != nil {
		return nil, nil, err
	}
	if fieldValue == nil {
		return nil, nil, ErrFieldIsNotValid{colName, table.Name}
	}
	if !fieldValue.IsValid() || !fieldValue.CanSet() {
		return nil, nil, ErrFieldIsNotValid{colName, table.Name}
	}

	return col, fieldValue, nil
}

// Cell cell is a result of one column field
type Cell *interface{}

func (session *Session) rows2Beans(rows *core.Rows, fields []string, types []*sql.ColumnType,
	table *schemas.Table, newElemFunc func([]string) reflect.Value,
	sliceValueSetFunc func(*reflect.Value, schemas.PK) error) error {
	for rows.Next() {
		var newValue = newElemFunc(fields)
		bean := newValue.Interface()
		dataStruct := newValue.Elem()

		// handle beforeClosures
		scanResults, err := session.row2Slice(rows, fields, types, bean)
		if err != nil {
			return err
		}
		pk, err := session.slice2Bean(scanResults, fields, bean, &dataStruct, table)
		if err != nil {
			return err
		}
		session.afterProcessors = append(session.afterProcessors, executedProcessor{
			fun: func(*Session, interface{}) error {
				return sliceValueSetFunc(&newValue, pk)
			},
			session: session,
			bean:    bean,
		})
	}
	return rows.Err()
}

func (session *Session) row2Slice(rows *core.Rows, fields []string, types []*sql.ColumnType, bean interface{}) ([]interface{}, error) {
	for _, closure := range session.beforeClosures {
		closure(bean)
	}

	scanResults := make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		var cell interface{}
		scanResults[i] = &cell
	}
	if err := session.engine.scan(rows, fields, types, scanResults...); err != nil {
		return nil, err
	}

	executeBeforeSet(bean, fields, scanResults)

	return scanResults, nil
}

func (session *Session) setJSON(fieldValue *reflect.Value, fieldType reflect.Type, scanResult interface{}) error {
	bs, ok := convert.AsBytes(scanResult)
	if !ok {
		return fmt.Errorf("unsupported database data type: %#v", scanResult)
	}
	if len(bs) == 0 {
		return nil
	}

	if fieldType.Kind() == reflect.String {
		fieldValue.SetString(string(bs))
		return nil
	}

	if fieldValue.CanAddr() {
		err := json.DefaultJSONHandler.Unmarshal(bs, fieldValue.Addr().Interface())
		if err != nil {
			return err
		}
	} else {
		x := reflect.New(fieldType)
		err := json.DefaultJSONHandler.Unmarshal(bs, x.Interface())
		if err != nil {
			return err
		}
		fieldValue.Set(x.Elem())
	}
	return nil
}

func asKind(vv reflect.Value, tp reflect.Type) (interface{}, error) {
	switch tp.Kind() {
	case reflect.Ptr:
		return asKind(vv.Elem(), tp.Elem())
	case reflect.Int64:
		return vv.Int(), nil
	case reflect.Int:
		return int(vv.Int()), nil
	case reflect.Int32:
		return int32(vv.Int()), nil
	case reflect.Int16:
		return int16(vv.Int()), nil
	case reflect.Int8:
		return int8(vv.Int()), nil
	case reflect.Uint64:
		return vv.Uint(), nil
	case reflect.Uint:
		return uint(vv.Uint()), nil
	case reflect.Uint32:
		return uint32(vv.Uint()), nil
	case reflect.Uint16:
		return uint16(vv.Uint()), nil
	case reflect.Uint8:
		return uint8(vv.Uint()), nil
	case reflect.String:
		return vv.String(), nil
	case reflect.Slice:
		if tp.Elem().Kind() == reflect.Uint8 {
			v, err := strconv.ParseInt(string(vv.Interface().([]byte)), 10, 64)
			if err != nil {
				return nil, err
			}
			return v, nil
		}
	}
	return nil, fmt.Errorf("unsupported primary key type: %v, %v", tp, vv)
}

func (session *Session) convertBeanField(col *schemas.Column, fieldValue *reflect.Value,
	scanResult interface{}, table *schemas.Table) error {
	v, ok := scanResult.(*interface{})
	if ok {
		scanResult = *v
	}
	if scanResult == nil {
		return nil
	}

	if fieldValue.CanAddr() {
		if structConvert, ok := fieldValue.Addr().Interface().(convert.Conversion); ok {
			data, ok := convert.AsBytes(scanResult)
			if !ok {
				return fmt.Errorf("cannot convert %#v as bytes", scanResult)
			}
			return structConvert.FromDB(data)
		}
	}

	if structConvert, ok := fieldValue.Interface().(convert.Conversion); ok {
		data, ok := convert.AsBytes(scanResult)
		if !ok {
			return fmt.Errorf("cannot convert %#v as bytes", scanResult)
		}
		if data == nil {
			return nil
		}

		if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
			fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			return fieldValue.Interface().(convert.Conversion).FromDB(data)
		}
		return structConvert.FromDB(data)
	}

	vv := reflect.ValueOf(scanResult)
	fieldType := fieldValue.Type()

	if col.IsJSON {
		return session.setJSON(fieldValue, fieldType, scanResult)
	}

	switch fieldType.Kind() {
	case reflect.Ptr:
		var e reflect.Value
		if fieldValue.IsNil() {
			e = reflect.New(fieldType.Elem()).Elem()
		} else {
			e = fieldValue.Elem()
		}
		if err := session.convertBeanField(col, &e, scanResult, table); err != nil {
			return err
		}
		if fieldValue.IsNil() {
			fieldValue.Set(e.Addr())
		}
		return nil
	case reflect.Complex64, reflect.Complex128:
		return session.setJSON(fieldValue, fieldType, scanResult)
	case reflect.Slice, reflect.Array:
		bs, ok := convert.AsBytes(scanResult)
		if ok && fieldType.Elem().Kind() == reflect.Uint8 {
			if col.SQLType.IsText() {
				x := reflect.New(fieldType)
				err := json.DefaultJSONHandler.Unmarshal(bs, x.Interface())
				if err != nil {
					return err
				}
				fieldValue.Set(x.Elem())
			} else {
				if fieldValue.Len() > 0 {
					for i := 0; i < fieldValue.Len(); i++ {
						if i < vv.Len() {
							fieldValue.Index(i).Set(vv.Index(i))
						}
					}
				} else {
					for i := 0; i < vv.Len(); i++ {
						fieldValue.Set(reflect.Append(*fieldValue, vv.Index(i)))
					}
				}
			}
			return nil
		}
	case reflect.Struct:
		if fieldType.ConvertibleTo(schemas.BigFloatType) {
			v, err := convert.AsBigFloat(scanResult)
			if err != nil {
				return err
			}
			fieldValue.Set(reflect.ValueOf(v).Elem().Convert(fieldType))
			return nil
		}

		if fieldType.ConvertibleTo(schemas.TimeType) {
			dbTZ := session.engine.DatabaseTZ
			if col.TimeZone != nil {
				dbTZ = col.TimeZone
			}

			t, err := convert.AsTime(scanResult, dbTZ, session.engine.TZLocation)
			if err != nil {
				return err
			}

			fieldValue.Set(reflect.ValueOf(*t).Convert(fieldType))
			return nil
		} else if nulVal, ok := fieldValue.Addr().Interface().(sql.Scanner); ok {
			err := nulVal.Scan(scanResult)
			if err == nil {
				return nil
			}
			session.engine.logger.Errorf("sql.Sanner error: %v", err)
		} else if session.statement.UseCascade {
			table, err := session.engine.tagParser.ParseWithCache(*fieldValue)
			if err != nil {
				return err
			}

			if len(table.PrimaryKeys) != 1 {
				return errors.New("unsupported non or composited primary key cascade")
			}
			var pk = make(schemas.PK, len(table.PrimaryKeys))
			pk[0], err = asKind(vv, reflect.TypeOf(scanResult))
			if err != nil {
				return err
			}

			if !pk.IsZero() {
				// !nashtsai! TODO for hasOne relationship, it's preferred to use join query for eager fetch
				// however, also need to consider adding a 'lazy' attribute to xorm tag which allow hasOne
				// property to be fetched lazily
				structInter := reflect.New(fieldValue.Type())
				has, err := session.ID(pk).NoCascade().get(structInter.Interface())
				if err != nil {
					return err
				}
				if has {
					fieldValue.Set(structInter.Elem())
				} else {
					return errors.New("cascade obj is not exist")
				}
			}
			return nil
		}
	} // switch fieldType.Kind()

	return convert.AssignValue(fieldValue.Addr(), scanResult)
}

func (session *Session) slice2Bean(scanResults []interface{}, fields []string, bean interface{}, dataStruct *reflect.Value, table *schemas.Table) (schemas.PK, error) {
	defer func() {
		executeAfterSet(bean, fields, scanResults)
	}()

	buildAfterProcessors(session, bean)

	var tempMap = make(map[string]int)
	var pk schemas.PK
	for i, colName := range fields {
		var idx int
		var lKey = strings.ToLower(colName)
		var ok bool

		if idx, ok = tempMap[lKey]; !ok {
			idx = 0
		} else {
			idx = idx + 1
		}
		tempMap[lKey] = idx

		col, fieldValue, err := session.getField(dataStruct, table, colName, idx)
		if err != nil {
			if _, ok := err.(ErrFieldIsNotExist); ok {
				continue
			} else {
				return nil, err
			}
		}
		if fieldValue == nil {
			continue
		}

		if err := session.convertBeanField(col, fieldValue, scanResults[i], table); err != nil {
			return nil, err
		}
		if col.IsPrimaryKey {
			pk = append(pk, scanResults[i])
		}
	}
	return pk, nil
}

// saveLastSQL stores executed query information
func (session *Session) saveLastSQL(sql string, args ...interface{}) {
	session.lastSQL = sql
	session.lastSQLArgs = args
}

// LastSQL returns last query information
func (session *Session) LastSQL() (string, []interface{}) {
	return session.lastSQL, session.lastSQLArgs
}

// Unscoped always disable struct tag "deleted"
func (session *Session) Unscoped() *Session {
	session.statement.SetUnscoped()
	return session
}

func (session *Session) incrVersionFieldValue(fieldValue *reflect.Value) {
	switch fieldValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fieldValue.SetInt(fieldValue.Int() + 1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fieldValue.SetUint(fieldValue.Uint() + 1)
	}
}

// Context sets the context on this session
func (session *Session) Context(ctx context.Context) *Session {
	session.ctx = ctx
	return session
}

// PingContext test if database is ok
func (session *Session) PingContext(ctx context.Context) error {
	if session.isAutoClose {
		defer session.Close()
	}

	session.engine.logger.Infof("PING DATABASE %v", session.engine.DriverName())
	return session.DB().PingContext(ctx)
}
