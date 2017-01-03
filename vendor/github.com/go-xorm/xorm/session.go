// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-xorm/builder"
	"github.com/go-xorm/core"
)

// Session keep a pointer to sql.DB and provides all execution of all
// kind of database operations.
type Session struct {
	db                     *core.DB
	Engine                 *Engine
	Tx                     *core.Tx
	Statement              Statement
	IsAutoCommit           bool
	IsCommitedOrRollbacked bool
	TransType              string
	IsAutoClose            bool

	// Automatically reset the statement after operations that execute a SQL
	// query such as Count(), Find(), Get(), ...
	AutoResetStatement bool

	// !nashtsai! storing these beans due to yet committed tx
	afterInsertBeans map[interface{}]*[]func(interface{})
	afterUpdateBeans map[interface{}]*[]func(interface{})
	afterDeleteBeans map[interface{}]*[]func(interface{})
	// --

	beforeClosures []func(interface{})
	afterClosures  []func(interface{})

	prepareStmt bool
	stmtCache   map[uint32]*core.Stmt //key: hash.Hash32 of (queryStr, len(queryStr))
	cascadeDeep int

	// !evalphobia! stored the last executed query on this session
	//beforeSQLExec func(string, ...interface{})
	lastSQL     string
	lastSQLArgs []interface{}
}

// Clone copy all the session's content and return a new session
func (session *Session) Clone() *Session {
	var sess = *session
	return &sess
}

// Init reset the session as the init status.
func (session *Session) Init() {
	session.Statement.Init()
	session.Statement.Engine = session.Engine
	session.IsAutoCommit = true
	session.IsCommitedOrRollbacked = false
	session.IsAutoClose = false
	session.AutoResetStatement = true
	session.prepareStmt = false

	// !nashtsai! is lazy init better?
	session.afterInsertBeans = make(map[interface{}]*[]func(interface{}), 0)
	session.afterUpdateBeans = make(map[interface{}]*[]func(interface{}), 0)
	session.afterDeleteBeans = make(map[interface{}]*[]func(interface{}), 0)
	session.beforeClosures = make([]func(interface{}), 0)
	session.afterClosures = make([]func(interface{}), 0)

	session.lastSQL = ""
	session.lastSQLArgs = []interface{}{}
}

// Close release the connection from pool
func (session *Session) Close() {
	for _, v := range session.stmtCache {
		v.Close()
	}

	if session.db != nil {
		// When Close be called, if session is a transaction and do not call
		// Commit or Rollback, then call Rollback.
		if session.Tx != nil && !session.IsCommitedOrRollbacked {
			session.Rollback()
		}
		session.Tx = nil
		session.stmtCache = nil
		session.Init()
		session.db = nil
	}
}

func (session *Session) resetStatement() {
	if session.AutoResetStatement {
		session.Statement.Init()
	}
}

// Prepare set a flag to session that should be prepare statment before execute query
func (session *Session) Prepare() *Session {
	session.prepareStmt = true
	return session
}

// Sql provides raw sql input parameter. When you have a complex SQL statement
// and cannot use Where, Id, In and etc. Methods to describe, you can use SQL.
//
// Deprecated: use SQL instead.
func (session *Session) Sql(query string, args ...interface{}) *Session {
	return session.SQL(query, args...)
}

// SQL provides raw sql input parameter. When you have a complex SQL statement
// and cannot use Where, Id, In and etc. Methods to describe, you can use SQL.
func (session *Session) SQL(query interface{}, args ...interface{}) *Session {
	session.Statement.SQL(query, args...)
	return session
}

// Where provides custom query condition.
func (session *Session) Where(query interface{}, args ...interface{}) *Session {
	session.Statement.Where(query, args...)
	return session
}

// And provides custom query condition.
func (session *Session) And(query interface{}, args ...interface{}) *Session {
	session.Statement.And(query, args...)
	return session
}

// Or provides custom query condition.
func (session *Session) Or(query interface{}, args ...interface{}) *Session {
	session.Statement.Or(query, args...)
	return session
}

// Id provides converting id as a query condition
//
// Deprecated: use ID instead
func (session *Session) Id(id interface{}) *Session {
	return session.ID(id)
}

// ID provides converting id as a query condition
func (session *Session) ID(id interface{}) *Session {
	session.Statement.ID(id)
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
	session.Statement.Table(tableNameOrBean)
	return session
}

// Alias set the table alias
func (session *Session) Alias(alias string) *Session {
	session.Statement.Alias(alias)
	return session
}

// In provides a query string like "id in (1, 2, 3)"
func (session *Session) In(column string, args ...interface{}) *Session {
	session.Statement.In(column, args...)
	return session
}

// NotIn provides a query string like "id in (1, 2, 3)"
func (session *Session) NotIn(column string, args ...interface{}) *Session {
	session.Statement.NotIn(column, args...)
	return session
}

// Incr provides a query string like "count = count + 1"
func (session *Session) Incr(column string, arg ...interface{}) *Session {
	session.Statement.Incr(column, arg...)
	return session
}

// Decr provides a query string like "count = count - 1"
func (session *Session) Decr(column string, arg ...interface{}) *Session {
	session.Statement.Decr(column, arg...)
	return session
}

// SetExpr provides a query string like "column = {expression}"
func (session *Session) SetExpr(column string, expression string) *Session {
	session.Statement.SetExpr(column, expression)
	return session
}

// Select provides some columns to special
func (session *Session) Select(str string) *Session {
	session.Statement.Select(str)
	return session
}

// Cols provides some columns to special
func (session *Session) Cols(columns ...string) *Session {
	session.Statement.Cols(columns...)
	return session
}

// AllCols ask all columns
func (session *Session) AllCols() *Session {
	session.Statement.AllCols()
	return session
}

// MustCols specify some columns must use even if they are empty
func (session *Session) MustCols(columns ...string) *Session {
	session.Statement.MustCols(columns...)
	return session
}

// NoCascade indicate that no cascade load child object
func (session *Session) NoCascade() *Session {
	session.Statement.UseCascade = false
	return session
}

// UseBool automatically retrieve condition according struct, but
// if struct has bool field, it will ignore them. So use UseBool
// to tell system to do not ignore them.
// If no paramters, it will use all the bool field of struct, or
// it will use paramters's columns
func (session *Session) UseBool(columns ...string) *Session {
	session.Statement.UseBool(columns...)
	return session
}

// Distinct use for distinct columns. Caution: when you are using cache,
// distinct will not be cached because cache system need id,
// but distinct will not provide id
func (session *Session) Distinct(columns ...string) *Session {
	session.Statement.Distinct(columns...)
	return session
}

// ForUpdate Set Read/Write locking for UPDATE
func (session *Session) ForUpdate() *Session {
	session.Statement.IsForUpdate = true
	return session
}

// Omit Only not use the paramters as select or update columns
func (session *Session) Omit(columns ...string) *Session {
	session.Statement.Omit(columns...)
	return session
}

// Nullable Set null when column is zero-value and nullable for update
func (session *Session) Nullable(columns ...string) *Session {
	session.Statement.Nullable(columns...)
	return session
}

// NoAutoTime means do not automatically give created field and updated field
// the current time on the current session temporarily
func (session *Session) NoAutoTime() *Session {
	session.Statement.UseAutoTime = false
	return session
}

// NoAutoCondition disable generate SQL condition from beans
func (session *Session) NoAutoCondition(no ...bool) *Session {
	session.Statement.NoAutoCondition(no...)
	return session
}

// Limit provide limit and offset query condition
func (session *Session) Limit(limit int, start ...int) *Session {
	session.Statement.Limit(limit, start...)
	return session
}

// OrderBy provide order by query condition, the input parameter is the content
// after order by on a sql statement.
func (session *Session) OrderBy(order string) *Session {
	session.Statement.OrderBy(order)
	return session
}

// Desc provide desc order by query condition, the input parameters are columns.
func (session *Session) Desc(colNames ...string) *Session {
	session.Statement.Desc(colNames...)
	return session
}

// Asc provide asc order by query condition, the input parameters are columns.
func (session *Session) Asc(colNames ...string) *Session {
	session.Statement.Asc(colNames...)
	return session
}

// StoreEngine is only avialble mysql dialect currently
func (session *Session) StoreEngine(storeEngine string) *Session {
	session.Statement.StoreEngine = storeEngine
	return session
}

// Charset is only avialble mysql dialect currently
func (session *Session) Charset(charset string) *Session {
	session.Statement.Charset = charset
	return session
}

// Cascade indicates if loading sub Struct
func (session *Session) Cascade(trueOrFalse ...bool) *Session {
	if len(trueOrFalse) >= 1 {
		session.Statement.UseCascade = trueOrFalse[0]
	}
	return session
}

// NoCache ask this session do not retrieve data from cache system and
// get data from database directly.
func (session *Session) NoCache() *Session {
	session.Statement.UseCache = false
	return session
}

// Join join_operator should be one of INNER, LEFT OUTER, CROSS etc - this will be prepended to JOIN
func (session *Session) Join(joinOperator string, tablename interface{}, condition string, args ...interface{}) *Session {
	session.Statement.Join(joinOperator, tablename, condition, args...)
	return session
}

// GroupBy Generate Group By statement
func (session *Session) GroupBy(keys string) *Session {
	session.Statement.GroupBy(keys)
	return session
}

// Having Generate Having statement
func (session *Session) Having(conditions string) *Session {
	session.Statement.Having(conditions)
	return session
}

// DB db return the wrapper of sql.DB
func (session *Session) DB() *core.DB {
	if session.db == nil {
		session.db = session.Engine.db
		session.stmtCache = make(map[uint32]*core.Stmt, 0)
	}
	return session.db
}

// Conds returns session query conditions
func (session *Session) Conds() builder.Cond {
	return session.Statement.cond
}

func cleanupProcessorsClosures(slices *[]func(interface{})) {
	if len(*slices) > 0 {
		*slices = make([]func(interface{}), 0)
	}
}

func (session *Session) scanMapIntoStruct(obj interface{}, objMap map[string][]byte) error {
	dataStruct := rValue(obj)
	if dataStruct.Kind() != reflect.Struct {
		return errors.New("Expected a pointer to a struct")
	}

	var col *core.Column
	session.Statement.setRefValue(dataStruct)
	table := session.Statement.RefTable
	tableName := session.Statement.tableName

	for key, data := range objMap {
		if col = table.GetColumn(key); col == nil {
			session.Engine.logger.Warnf("struct %v's has not field %v. %v",
				table.Type.Name(), key, table.ColumnsSeq())
			continue
		}

		fieldName := col.FieldName
		fieldPath := strings.Split(fieldName, ".")
		var fieldValue reflect.Value
		if len(fieldPath) > 2 {
			session.Engine.logger.Error("Unsupported mutliderive", fieldName)
			continue
		} else if len(fieldPath) == 2 {
			parentField := dataStruct.FieldByName(fieldPath[0])
			if parentField.IsValid() {
				fieldValue = parentField.FieldByName(fieldPath[1])
			}
		} else {
			fieldValue = dataStruct.FieldByName(fieldName)
		}
		if !fieldValue.IsValid() || !fieldValue.CanSet() {
			session.Engine.logger.Warnf("table %v's column %v is not valid or cannot set", tableName, key)
			continue
		}

		err := session.bytes2Value(col, &fieldValue, data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (session *Session) canCache() bool {
	if session.Statement.RefTable == nil ||
		session.Statement.JoinStr != "" ||
		session.Statement.RawSQL != "" ||
		!session.Statement.UseCache ||
		session.Statement.IsForUpdate ||
		session.Tx != nil ||
		len(session.Statement.selectStr) > 0 {
		return false
	}
	return true
}

func (session *Session) doPrepare(sqlStr string) (stmt *core.Stmt, err error) {
	crc := crc32.ChecksumIEEE([]byte(sqlStr))
	// TODO try hash(sqlStr+len(sqlStr))
	var has bool
	stmt, has = session.stmtCache[crc]
	if !has {
		stmt, err = session.DB().Prepare(sqlStr)
		if err != nil {
			return nil, err
		}
		session.stmtCache[crc] = stmt
	}
	return
}

func (session *Session) getField(dataStruct *reflect.Value, key string, table *core.Table, idx int) *reflect.Value {
	var col *core.Column
	if col = table.GetColumnIdx(key, idx); col == nil {
		//session.Engine.logger.Warnf("table %v has no column %v. %v", table.Name, key, table.ColumnsSeq())
		return nil
	}

	fieldValue, err := col.ValueOfV(dataStruct)
	if err != nil {
		session.Engine.logger.Error(err)
		return nil
	}

	if !fieldValue.IsValid() || !fieldValue.CanSet() {
		session.Engine.logger.Warnf("table %v's column %v is not valid or cannot set", table.Name, key)
		return nil
	}
	return fieldValue
}

// Cell cell is a result of one column field
type Cell *interface{}

func (session *Session) rows2Beans(rows *core.Rows, fields []string, fieldsCount int,
	table *core.Table, newElemFunc func() reflect.Value,
	sliceValueSetFunc func(*reflect.Value)) error {
	for rows.Next() {
		var newValue = newElemFunc()
		bean := newValue.Interface()
		dataStruct := rValue(bean)
		err := session._row2Bean(rows, fields, fieldsCount, bean, &dataStruct, table)
		if err != nil {
			return err
		}
		sliceValueSetFunc(&newValue)
	}
	return nil
}

func (session *Session) row2Bean(rows *core.Rows, fields []string, fieldsCount int, bean interface{}) error {
	dataStruct := rValue(bean)
	if dataStruct.Kind() != reflect.Struct {
		return errors.New("Expected a pointer to a struct")
	}

	session.Statement.setRefValue(dataStruct)

	return session._row2Bean(rows, fields, fieldsCount, bean, &dataStruct, session.Statement.RefTable)
}

func (session *Session) _row2Bean(rows *core.Rows, fields []string, fieldsCount int, bean interface{}, dataStruct *reflect.Value, table *core.Table) error {
	scanResults := make([]interface{}, fieldsCount)
	for i := 0; i < len(fields); i++ {
		var cell interface{}
		scanResults[i] = &cell
	}
	if err := rows.Scan(scanResults...); err != nil {
		return err
	}

	if b, hasBeforeSet := bean.(BeforeSetProcessor); hasBeforeSet {
		for ii, key := range fields {
			b.BeforeSet(key, Cell(scanResults[ii].(*interface{})))
		}
	}

	defer func() {
		if b, hasAfterSet := bean.(AfterSetProcessor); hasAfterSet {
			for ii, key := range fields {
				b.AfterSet(key, Cell(scanResults[ii].(*interface{})))
			}
		}
	}()

	var tempMap = make(map[string]int)
	for ii, key := range fields {
		var idx int
		var ok bool
		var lKey = strings.ToLower(key)
		if idx, ok = tempMap[lKey]; !ok {
			idx = 0
		} else {
			idx = idx + 1
		}
		tempMap[lKey] = idx

		if fieldValue := session.getField(dataStruct, key, table, idx); fieldValue != nil {
			rawValue := reflect.Indirect(reflect.ValueOf(scanResults[ii]))

			// if row is null then ignore
			if rawValue.Interface() == nil {
				continue
			}

			if fieldValue.CanAddr() {
				if structConvert, ok := fieldValue.Addr().Interface().(core.Conversion); ok {
					if data, err := value2Bytes(&rawValue); err == nil {
						structConvert.FromDB(data)
					} else {
						session.Engine.logger.Error(err)
					}
					continue
				}
			}

			if _, ok := fieldValue.Interface().(core.Conversion); ok {
				if data, err := value2Bytes(&rawValue); err == nil {
					if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
						fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
					}
					fieldValue.Interface().(core.Conversion).FromDB(data)
				} else {
					session.Engine.logger.Error(err)
				}
				continue
			}

			rawValueType := reflect.TypeOf(rawValue.Interface())
			vv := reflect.ValueOf(rawValue.Interface())

			fieldType := fieldValue.Type()
			hasAssigned := false
			col := table.GetColumnIdx(key, idx)

			if col.SQLType.IsJson() {
				var bs []byte
				if rawValueType.Kind() == reflect.String {
					bs = []byte(vv.String())
				} else if rawValueType.ConvertibleTo(core.BytesType) {
					bs = vv.Bytes()
				} else {
					return fmt.Errorf("unsupported database data type: %s %v", key, rawValueType.Kind())
				}

				hasAssigned = true

				if len(bs) > 0 {
					if fieldValue.CanAddr() {
						err := json.Unmarshal(bs, fieldValue.Addr().Interface())
						if err != nil {
							session.Engine.logger.Error(key, err)
							return err
						}
					} else {
						x := reflect.New(fieldType)
						err := json.Unmarshal(bs, x.Interface())
						if err != nil {
							session.Engine.logger.Error(key, err)
							return err
						}
						fieldValue.Set(x.Elem())
					}
				}

				continue
			}

			switch fieldType.Kind() {
			case reflect.Complex64, reflect.Complex128:
				// TODO: reimplement this
				var bs []byte
				if rawValueType.Kind() == reflect.String {
					bs = []byte(vv.String())
				} else if rawValueType.ConvertibleTo(core.BytesType) {
					bs = vv.Bytes()
				}

				hasAssigned = true
				if len(bs) > 0 {
					if fieldValue.CanAddr() {
						err := json.Unmarshal(bs, fieldValue.Addr().Interface())
						if err != nil {
							session.Engine.logger.Error(err)
							return err
						}
					} else {
						x := reflect.New(fieldType)
						err := json.Unmarshal(bs, x.Interface())
						if err != nil {
							session.Engine.logger.Error(err)
							return err
						}
						fieldValue.Set(x.Elem())
					}
				}
			case reflect.Slice, reflect.Array:
				switch rawValueType.Kind() {
				case reflect.Slice, reflect.Array:
					switch rawValueType.Elem().Kind() {
					case reflect.Uint8:
						if fieldType.Elem().Kind() == reflect.Uint8 {
							hasAssigned = true
							fieldValue.Set(vv)
						}
					}
				}
			case reflect.String:
				if rawValueType.Kind() == reflect.String {
					hasAssigned = true
					fieldValue.SetString(vv.String())
				}
			case reflect.Bool:
				if rawValueType.Kind() == reflect.Bool {
					hasAssigned = true
					fieldValue.SetBool(vv.Bool())
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				switch rawValueType.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					hasAssigned = true
					fieldValue.SetInt(vv.Int())
				}
			case reflect.Float32, reflect.Float64:
				switch rawValueType.Kind() {
				case reflect.Float32, reflect.Float64:
					hasAssigned = true
					fieldValue.SetFloat(vv.Float())
				}
			case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
				switch rawValueType.Kind() {
				case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
					hasAssigned = true
					fieldValue.SetUint(vv.Uint())
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					hasAssigned = true
					fieldValue.SetUint(uint64(vv.Int()))
				}
			case reflect.Struct:
				if fieldType.ConvertibleTo(core.TimeType) {
					if rawValueType == core.TimeType {
						hasAssigned = true

						t := vv.Convert(core.TimeType).Interface().(time.Time)

						z, _ := t.Zone()
						dbTZ := session.Engine.DatabaseTZ
						if dbTZ == nil {
							if session.Engine.dialect.DBType() == core.SQLITE {
								dbTZ = time.UTC
							} else {
								dbTZ = time.Local
							}
						}

						// set new location if database don't save timezone or give an incorrect timezone
						if len(z) == 0 || t.Year() == 0 || t.Location().String() != dbTZ.String() { // !nashtsai! HACK tmp work around for lib/pq doesn't properly time with location
							session.Engine.logger.Debugf("empty zone key[%v] : %v | zone: %v | location: %+v\n", key, t, z, *t.Location())
							t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(),
								t.Minute(), t.Second(), t.Nanosecond(), dbTZ)
						}

						// !nashtsai! convert to engine location
						if col.TimeZone == nil {
							t = t.In(session.Engine.TZLocation)
						} else {
							t = t.In(col.TimeZone)
						}
						fieldValue.Set(reflect.ValueOf(t).Convert(fieldType))

						// t = fieldValue.Interface().(time.Time)
						// z, _ = t.Zone()
						// session.Engine.LogDebug("fieldValue key[%v]: %v | zone: %v | location: %+v\n", key, t, z, *t.Location())
					} else if rawValueType == core.IntType || rawValueType == core.Int64Type ||
						rawValueType == core.Int32Type {
						hasAssigned = true
						var tz *time.Location
						if col.TimeZone == nil {
							tz = session.Engine.TZLocation
						} else {
							tz = col.TimeZone
						}
						t := time.Unix(vv.Int(), 0).In(tz)
						//vv = reflect.ValueOf(t)
						fieldValue.Set(reflect.ValueOf(t).Convert(fieldType))
					} else {
						if d, ok := vv.Interface().([]uint8); ok {
							hasAssigned = true
							t, err := session.byte2Time(col, d)
							if err != nil {
								session.Engine.logger.Error("byte2Time error:", err.Error())
								hasAssigned = false
							} else {
								fieldValue.Set(reflect.ValueOf(t).Convert(fieldType))
							}
						} else if d, ok := vv.Interface().(string); ok {
							hasAssigned = true
							t, err := session.str2Time(col, d)
							if err != nil {
								session.Engine.logger.Error("byte2Time error:", err.Error())
								hasAssigned = false
							} else {
								fieldValue.Set(reflect.ValueOf(t).Convert(fieldType))
							}
						} else {
							panic(fmt.Sprintf("rawValueType is %v, value is %v", rawValueType, vv.Interface()))
						}
					}
				} else if nulVal, ok := fieldValue.Addr().Interface().(sql.Scanner); ok {
					// !<winxxp>! 增加支持sql.Scanner接口的结构，如sql.NullString
					hasAssigned = true
					if err := nulVal.Scan(vv.Interface()); err != nil {
						session.Engine.logger.Error("sql.Sanner error:", err.Error())
						hasAssigned = false
					}
				} else if col.SQLType.IsJson() {
					if rawValueType.Kind() == reflect.String {
						hasAssigned = true
						x := reflect.New(fieldType)
						if len([]byte(vv.String())) > 0 {
							err := json.Unmarshal([]byte(vv.String()), x.Interface())
							if err != nil {
								session.Engine.logger.Error(err)
								return err
							}
							fieldValue.Set(x.Elem())
						}
					} else if rawValueType.Kind() == reflect.Slice {
						hasAssigned = true
						x := reflect.New(fieldType)
						if len(vv.Bytes()) > 0 {
							err := json.Unmarshal(vv.Bytes(), x.Interface())
							if err != nil {
								session.Engine.logger.Error(err)
								return err
							}
							fieldValue.Set(x.Elem())
						}
					}
				} else if session.Statement.UseCascade {
					table := session.Engine.autoMapType(*fieldValue)
					if table != nil {
						hasAssigned = true
						if len(table.PrimaryKeys) != 1 {
							panic("unsupported non or composited primary key cascade")
						}
						var pk = make(core.PK, len(table.PrimaryKeys))

						switch rawValueType.Kind() {
						case reflect.Int64:
							pk[0] = vv.Int()
						case reflect.Int:
							pk[0] = int(vv.Int())
						case reflect.Int32:
							pk[0] = int32(vv.Int())
						case reflect.Int16:
							pk[0] = int16(vv.Int())
						case reflect.Int8:
							pk[0] = int8(vv.Int())
						case reflect.Uint64:
							pk[0] = vv.Uint()
						case reflect.Uint:
							pk[0] = uint(vv.Uint())
						case reflect.Uint32:
							pk[0] = uint32(vv.Uint())
						case reflect.Uint16:
							pk[0] = uint16(vv.Uint())
						case reflect.Uint8:
							pk[0] = uint8(vv.Uint())
						case reflect.String:
							pk[0] = vv.String()
						case reflect.Slice:
							pk[0], _ = strconv.ParseInt(string(rawValue.Interface().([]byte)), 10, 64)
						default:
							panic(fmt.Sprintf("unsupported primary key type: %v, %v", rawValueType, fieldValue))
						}

						if !isPKZero(pk) {
							// !nashtsai! TODO for hasOne relationship, it's preferred to use join query for eager fetch
							// however, also need to consider adding a 'lazy' attribute to xorm tag which allow hasOne
							// property to be fetched lazily
							structInter := reflect.New(fieldValue.Type())
							newsession := session.Engine.NewSession()
							defer newsession.Close()
							has, err := newsession.Id(pk).NoCascade().Get(structInter.Interface())
							if err != nil {
								return err
							}
							if has {
								//v := structInter.Elem().Interface()
								//fieldValue.Set(reflect.ValueOf(v))
								fieldValue.Set(structInter.Elem())
							} else {
								return errors.New("cascade obj is not exist")
							}
						}
					} else {
						session.Engine.logger.Error("unsupported struct type in Scan: ", fieldValue.Type().String())
					}
				}
			case reflect.Ptr:
				// !nashtsai! TODO merge duplicated codes above
				//typeStr := fieldType.String()
				switch fieldType {
				// following types case matching ptr's native type, therefore assign ptr directly
				case core.PtrStringType:
					if rawValueType.Kind() == reflect.String {
						x := vv.String()
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrBoolType:
					if rawValueType.Kind() == reflect.Bool {
						x := vv.Bool()
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrTimeType:
					if rawValueType == core.PtrTimeType {
						hasAssigned = true
						var x = rawValue.Interface().(time.Time)
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrFloat64Type:
					if rawValueType.Kind() == reflect.Float64 {
						x := vv.Float()
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrUint64Type:
					if rawValueType.Kind() == reflect.Int64 {
						var x = uint64(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrInt64Type:
					if rawValueType.Kind() == reflect.Int64 {
						x := vv.Int()
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrFloat32Type:
					if rawValueType.Kind() == reflect.Float64 {
						var x = float32(vv.Float())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrIntType:
					if rawValueType.Kind() == reflect.Int64 {
						var x = int(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrInt32Type:
					if rawValueType.Kind() == reflect.Int64 {
						var x = int32(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrInt8Type:
					if rawValueType.Kind() == reflect.Int64 {
						var x = int8(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrInt16Type:
					if rawValueType.Kind() == reflect.Int64 {
						var x = int16(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrUintType:
					if rawValueType.Kind() == reflect.Int64 {
						var x = uint(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.PtrUint32Type:
					if rawValueType.Kind() == reflect.Int64 {
						var x = uint32(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.Uint8Type:
					if rawValueType.Kind() == reflect.Int64 {
						var x = uint8(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.Uint16Type:
					if rawValueType.Kind() == reflect.Int64 {
						var x = uint16(vv.Int())
						hasAssigned = true
						fieldValue.Set(reflect.ValueOf(&x))
					}
				case core.Complex64Type:
					var x complex64
					if len([]byte(vv.String())) > 0 {
						err := json.Unmarshal([]byte(vv.String()), &x)
						if err != nil {
							session.Engine.logger.Error(err)
						} else {
							fieldValue.Set(reflect.ValueOf(&x))
						}
					}
					hasAssigned = true
				case core.Complex128Type:
					var x complex128
					if len([]byte(vv.String())) > 0 {
						err := json.Unmarshal([]byte(vv.String()), &x)
						if err != nil {
							session.Engine.logger.Error(err)
						} else {
							fieldValue.Set(reflect.ValueOf(&x))
						}
					}
					hasAssigned = true
				} // switch fieldType
				// default:
				// 	session.Engine.LogError("unsupported type in Scan: ", reflect.TypeOf(v).String())
			} // switch fieldType.Kind()

			// !nashtsai! for value can't be assigned directly fallback to convert to []byte then back to value
			if !hasAssigned {
				data, err := value2Bytes(&rawValue)
				if err == nil {
					session.bytes2Value(col, fieldValue, data)
				} else {
					session.Engine.logger.Error(err.Error())
				}
			}
		}
	}
	return nil
}

func (session *Session) queryPreprocess(sqlStr *string, paramStr ...interface{}) {
	for _, filter := range session.Engine.dialect.Filters() {
		*sqlStr = filter.Do(*sqlStr, session.Engine.dialect, session.Statement.RefTable)
	}

	session.saveLastSQL(*sqlStr, paramStr...)
}

func (session *Session) str2Time(col *core.Column, data string) (outTime time.Time, outErr error) {
	sdata := strings.TrimSpace(data)
	var x time.Time
	var err error

	if sdata == "0000-00-00 00:00:00" ||
		sdata == "0001-01-01 00:00:00" {
	} else if !strings.ContainsAny(sdata, "- :") { // !nashtsai! has only found that mymysql driver is using this for time type column
		// time stamp
		sd, err := strconv.ParseInt(sdata, 10, 64)
		if err == nil {
			x = time.Unix(sd, 0)
			// !nashtsai! HACK mymysql driver is casuing Local location being change to CHAT and cause wrong time conversion
			if col.TimeZone == nil {
				x = x.In(session.Engine.TZLocation)
			} else {
				x = x.In(col.TimeZone)
			}
			session.Engine.logger.Debugf("time(0) key[%v]: %+v | sdata: [%v]\n", col.FieldName, x, sdata)
		} else {
			session.Engine.logger.Debugf("time(0) err key[%v]: %+v | sdata: [%v]\n", col.FieldName, x, sdata)
		}
	} else if len(sdata) > 19 && strings.Contains(sdata, "-") {
		x, err = time.ParseInLocation(time.RFC3339Nano, sdata, session.Engine.TZLocation)
		session.Engine.logger.Debugf("time(1) key[%v]: %+v | sdata: [%v]\n", col.FieldName, x, sdata)
		if err != nil {
			x, err = time.ParseInLocation("2006-01-02 15:04:05.999999999", sdata, session.Engine.TZLocation)
			session.Engine.logger.Debugf("time(2) key[%v]: %+v | sdata: [%v]\n", col.FieldName, x, sdata)
		}
		if err != nil {
			x, err = time.ParseInLocation("2006-01-02 15:04:05.9999999 Z07:00", sdata, session.Engine.TZLocation)
			session.Engine.logger.Debugf("time(3) key[%v]: %+v | sdata: [%v]\n", col.FieldName, x, sdata)
		}

	} else if len(sdata) == 19 && strings.Contains(sdata, "-") {
		x, err = time.ParseInLocation("2006-01-02 15:04:05", sdata, session.Engine.TZLocation)
		session.Engine.logger.Debugf("time(4) key[%v]: %+v | sdata: [%v]\n", col.FieldName, x, sdata)
	} else if len(sdata) == 10 && sdata[4] == '-' && sdata[7] == '-' {
		x, err = time.ParseInLocation("2006-01-02", sdata, session.Engine.TZLocation)
		session.Engine.logger.Debugf("time(5) key[%v]: %+v | sdata: [%v]\n", col.FieldName, x, sdata)
	} else if col.SQLType.Name == core.Time {
		if strings.Contains(sdata, " ") {
			ssd := strings.Split(sdata, " ")
			sdata = ssd[1]
		}

		sdata = strings.TrimSpace(sdata)
		if session.Engine.dialect.DBType() == core.MYSQL && len(sdata) > 8 {
			sdata = sdata[len(sdata)-8:]
		}

		st := fmt.Sprintf("2006-01-02 %v", sdata)
		x, err = time.ParseInLocation("2006-01-02 15:04:05", st, session.Engine.TZLocation)
		session.Engine.logger.Debugf("time(6) key[%v]: %+v | sdata: [%v]\n", col.FieldName, x, sdata)
	} else {
		outErr = fmt.Errorf("unsupported time format %v", sdata)
		return
	}
	if err != nil {
		outErr = fmt.Errorf("unsupported time format %v: %v", sdata, err)
		return
	}
	outTime = x
	return
}

func (session *Session) byte2Time(col *core.Column, data []byte) (outTime time.Time, outErr error) {
	return session.str2Time(col, string(data))
}

// convert a db data([]byte) to a field value
func (session *Session) bytes2Value(col *core.Column, fieldValue *reflect.Value, data []byte) error {
	if structConvert, ok := fieldValue.Addr().Interface().(core.Conversion); ok {
		return structConvert.FromDB(data)
	}

	if structConvert, ok := fieldValue.Interface().(core.Conversion); ok {
		return structConvert.FromDB(data)
	}

	var v interface{}
	key := col.Name
	fieldType := fieldValue.Type()

	switch fieldType.Kind() {
	case reflect.Complex64, reflect.Complex128:
		x := reflect.New(fieldType)
		if len(data) > 0 {
			err := json.Unmarshal(data, x.Interface())
			if err != nil {
				session.Engine.logger.Error(err)
				return err
			}
			fieldValue.Set(x.Elem())
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		v = data
		t := fieldType.Elem()
		k := t.Kind()
		if col.SQLType.IsText() {
			x := reflect.New(fieldType)
			if len(data) > 0 {
				err := json.Unmarshal(data, x.Interface())
				if err != nil {
					session.Engine.logger.Error(err)
					return err
				}
				fieldValue.Set(x.Elem())
			}
		} else if col.SQLType.IsBlob() {
			if k == reflect.Uint8 {
				fieldValue.Set(reflect.ValueOf(v))
			} else {
				x := reflect.New(fieldType)
				if len(data) > 0 {
					err := json.Unmarshal(data, x.Interface())
					if err != nil {
						session.Engine.logger.Error(err)
						return err
					}
					fieldValue.Set(x.Elem())
				}
			}
		} else {
			return ErrUnSupportedType
		}
	case reflect.String:
		fieldValue.SetString(string(data))
	case reflect.Bool:
		d := string(data)
		v, err := strconv.ParseBool(d)
		if err != nil {
			return fmt.Errorf("arg %v as bool: %s", key, err.Error())
		}
		fieldValue.Set(reflect.ValueOf(v))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		sdata := string(data)
		var x int64
		var err error
		// for mysql, when use bit, it returned \x01
		if col.SQLType.Name == core.Bit &&
			session.Engine.dialect.DBType() == core.MYSQL { // !nashtsai! TODO dialect needs to provide conversion interface API
			if len(data) == 1 {
				x = int64(data[0])
			} else {
				x = 0
			}
		} else if strings.HasPrefix(sdata, "0x") {
			x, err = strconv.ParseInt(sdata, 16, 64)
		} else if strings.HasPrefix(sdata, "0") {
			x, err = strconv.ParseInt(sdata, 8, 64)
		} else if strings.EqualFold(sdata, "true") {
			x = 1
		} else if strings.EqualFold(sdata, "false") {
			x = 0
		} else {
			x, err = strconv.ParseInt(sdata, 10, 64)
		}
		if err != nil {
			return fmt.Errorf("arg %v as int: %s", key, err.Error())
		}
		fieldValue.SetInt(x)
	case reflect.Float32, reflect.Float64:
		x, err := strconv.ParseFloat(string(data), 64)
		if err != nil {
			return fmt.Errorf("arg %v as float64: %s", key, err.Error())
		}
		fieldValue.SetFloat(x)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		x, err := strconv.ParseUint(string(data), 10, 64)
		if err != nil {
			return fmt.Errorf("arg %v as int: %s", key, err.Error())
		}
		fieldValue.SetUint(x)
	//Currently only support Time type
	case reflect.Struct:
		// !<winxxp>! 增加支持sql.Scanner接口的结构，如sql.NullString
		if nulVal, ok := fieldValue.Addr().Interface().(sql.Scanner); ok {
			if err := nulVal.Scan(data); err != nil {
				return fmt.Errorf("sql.Scan(%v) failed: %s ", data, err.Error())
			}
		} else {
			if fieldType.ConvertibleTo(core.TimeType) {
				x, err := session.byte2Time(col, data)
				if err != nil {
					return err
				}
				v = x
				fieldValue.Set(reflect.ValueOf(v).Convert(fieldType))
			} else if session.Statement.UseCascade {
				table := session.Engine.autoMapType(*fieldValue)
				if table != nil {
					// TODO: current only support 1 primary key
					if len(table.PrimaryKeys) > 1 {
						panic("unsupported composited primary key cascade")
					}
					var pk = make(core.PK, len(table.PrimaryKeys))
					rawValueType := table.ColumnType(table.PKColumns()[0].FieldName)
					var err error
					pk[0], err = str2PK(string(data), rawValueType)
					if err != nil {
						return err
					}

					if !isPKZero(pk) {
						// !nashtsai! TODO for hasOne relationship, it's preferred to use join query for eager fetch
						// however, also need to consider adding a 'lazy' attribute to xorm tag which allow hasOne
						// property to be fetched lazily
						structInter := reflect.New(fieldValue.Type())
						newsession := session.Engine.NewSession()
						defer newsession.Close()
						has, err := newsession.Id(pk).NoCascade().Get(structInter.Interface())
						if err != nil {
							return err
						}
						if has {
							v = structInter.Elem().Interface()
							fieldValue.Set(reflect.ValueOf(v))
						} else {
							return errors.New("cascade obj is not exist")
						}
					}
				} else {
					return fmt.Errorf("unsupported struct type in Scan: %s", fieldValue.Type().String())
				}
			}
		}
	case reflect.Ptr:
		// !nashtsai! TODO merge duplicated codes above
		//typeStr := fieldType.String()
		switch fieldType.Elem().Kind() {
		// case "*string":
		case core.StringType.Kind():
			x := string(data)
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*bool":
		case core.BoolType.Kind():
			d := string(data)
			v, err := strconv.ParseBool(d)
			if err != nil {
				return fmt.Errorf("arg %v as bool: %s", key, err.Error())
			}
			fieldValue.Set(reflect.ValueOf(&v).Convert(fieldType))
		// case "*complex64":
		case core.Complex64Type.Kind():
			var x complex64
			if len(data) > 0 {
				err := json.Unmarshal(data, &x)
				if err != nil {
					session.Engine.logger.Error(err)
					return err
				}
				fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
			}
		// case "*complex128":
		case core.Complex128Type.Kind():
			var x complex128
			if len(data) > 0 {
				err := json.Unmarshal(data, &x)
				if err != nil {
					session.Engine.logger.Error(err)
					return err
				}
				fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
			}
		// case "*float64":
		case core.Float64Type.Kind():
			x, err := strconv.ParseFloat(string(data), 64)
			if err != nil {
				return fmt.Errorf("arg %v as float64: %s", key, err.Error())
			}
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*float32":
		case core.Float32Type.Kind():
			var x float32
			x1, err := strconv.ParseFloat(string(data), 32)
			if err != nil {
				return fmt.Errorf("arg %v as float32: %s", key, err.Error())
			}
			x = float32(x1)
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*uint64":
		case core.Uint64Type.Kind():
			var x uint64
			x, err := strconv.ParseUint(string(data), 10, 64)
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*uint":
		case core.UintType.Kind():
			var x uint
			x1, err := strconv.ParseUint(string(data), 10, 64)
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			x = uint(x1)
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*uint32":
		case core.Uint32Type.Kind():
			var x uint32
			x1, err := strconv.ParseUint(string(data), 10, 64)
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			x = uint32(x1)
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*uint8":
		case core.Uint8Type.Kind():
			var x uint8
			x1, err := strconv.ParseUint(string(data), 10, 64)
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			x = uint8(x1)
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*uint16":
		case core.Uint16Type.Kind():
			var x uint16
			x1, err := strconv.ParseUint(string(data), 10, 64)
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			x = uint16(x1)
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*int64":
		case core.Int64Type.Kind():
			sdata := string(data)
			var x int64
			var err error
			// for mysql, when use bit, it returned \x01
			if col.SQLType.Name == core.Bit &&
				strings.Contains(session.Engine.DriverName(), "mysql") {
				if len(data) == 1 {
					x = int64(data[0])
				} else {
					x = 0
				}
			} else if strings.HasPrefix(sdata, "0x") {
				x, err = strconv.ParseInt(sdata, 16, 64)
			} else if strings.HasPrefix(sdata, "0") {
				x, err = strconv.ParseInt(sdata, 8, 64)
			} else {
				x, err = strconv.ParseInt(sdata, 10, 64)
			}
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*int":
		case core.IntType.Kind():
			sdata := string(data)
			var x int
			var x1 int64
			var err error
			// for mysql, when use bit, it returned \x01
			if col.SQLType.Name == core.Bit &&
				strings.Contains(session.Engine.DriverName(), "mysql") {
				if len(data) == 1 {
					x = int(data[0])
				} else {
					x = 0
				}
			} else if strings.HasPrefix(sdata, "0x") {
				x1, err = strconv.ParseInt(sdata, 16, 64)
				x = int(x1)
			} else if strings.HasPrefix(sdata, "0") {
				x1, err = strconv.ParseInt(sdata, 8, 64)
				x = int(x1)
			} else {
				x1, err = strconv.ParseInt(sdata, 10, 64)
				x = int(x1)
			}
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*int32":
		case core.Int32Type.Kind():
			sdata := string(data)
			var x int32
			var x1 int64
			var err error
			// for mysql, when use bit, it returned \x01
			if col.SQLType.Name == core.Bit &&
				session.Engine.dialect.DBType() == core.MYSQL {
				if len(data) == 1 {
					x = int32(data[0])
				} else {
					x = 0
				}
			} else if strings.HasPrefix(sdata, "0x") {
				x1, err = strconv.ParseInt(sdata, 16, 64)
				x = int32(x1)
			} else if strings.HasPrefix(sdata, "0") {
				x1, err = strconv.ParseInt(sdata, 8, 64)
				x = int32(x1)
			} else {
				x1, err = strconv.ParseInt(sdata, 10, 64)
				x = int32(x1)
			}
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*int8":
		case core.Int8Type.Kind():
			sdata := string(data)
			var x int8
			var x1 int64
			var err error
			// for mysql, when use bit, it returned \x01
			if col.SQLType.Name == core.Bit &&
				strings.Contains(session.Engine.DriverName(), "mysql") {
				if len(data) == 1 {
					x = int8(data[0])
				} else {
					x = 0
				}
			} else if strings.HasPrefix(sdata, "0x") {
				x1, err = strconv.ParseInt(sdata, 16, 64)
				x = int8(x1)
			} else if strings.HasPrefix(sdata, "0") {
				x1, err = strconv.ParseInt(sdata, 8, 64)
				x = int8(x1)
			} else {
				x1, err = strconv.ParseInt(sdata, 10, 64)
				x = int8(x1)
			}
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*int16":
		case core.Int16Type.Kind():
			sdata := string(data)
			var x int16
			var x1 int64
			var err error
			// for mysql, when use bit, it returned \x01
			if col.SQLType.Name == core.Bit &&
				strings.Contains(session.Engine.DriverName(), "mysql") {
				if len(data) == 1 {
					x = int16(data[0])
				} else {
					x = 0
				}
			} else if strings.HasPrefix(sdata, "0x") {
				x1, err = strconv.ParseInt(sdata, 16, 64)
				x = int16(x1)
			} else if strings.HasPrefix(sdata, "0") {
				x1, err = strconv.ParseInt(sdata, 8, 64)
				x = int16(x1)
			} else {
				x1, err = strconv.ParseInt(sdata, 10, 64)
				x = int16(x1)
			}
			if err != nil {
				return fmt.Errorf("arg %v as int: %s", key, err.Error())
			}
			fieldValue.Set(reflect.ValueOf(&x).Convert(fieldType))
		// case "*SomeStruct":
		case reflect.Struct:
			switch fieldType {
			// case "*.time.Time":
			case core.PtrTimeType:
				x, err := session.byte2Time(col, data)
				if err != nil {
					return err
				}
				v = x
				fieldValue.Set(reflect.ValueOf(&x))
			default:
				if session.Statement.UseCascade {
					structInter := reflect.New(fieldType.Elem())
					table := session.Engine.autoMapType(structInter.Elem())
					if table != nil {
						if len(table.PrimaryKeys) > 1 {
							panic("unsupported composited primary key cascade")
						}
						var pk = make(core.PK, len(table.PrimaryKeys))
						var err error
						rawValueType := table.ColumnType(table.PKColumns()[0].FieldName)
						pk[0], err = str2PK(string(data), rawValueType)
						if err != nil {
							return err
						}

						if !isPKZero(pk) {
							// !nashtsai! TODO for hasOne relationship, it's preferred to use join query for eager fetch
							// however, also need to consider adding a 'lazy' attribute to xorm tag which allow hasOne
							// property to be fetched lazily
							newsession := session.Engine.NewSession()
							defer newsession.Close()
							has, err := newsession.Id(pk).NoCascade().Get(structInter.Interface())
							if err != nil {
								return err
							}
							if has {
								v = structInter.Interface()
								fieldValue.Set(reflect.ValueOf(v))
							} else {
								return errors.New("cascade obj is not exist")
							}
						}
					}
				} else {
					return fmt.Errorf("unsupported struct type in Scan: %s", fieldValue.Type().String())
				}
			}
		default:
			return fmt.Errorf("unsupported type in Scan: %s", fieldValue.Type().String())
		}
	default:
		return fmt.Errorf("unsupported type in Scan: %s", fieldValue.Type().String())
	}

	return nil
}

// convert a field value of a struct to interface for put into db
func (session *Session) value2Interface(col *core.Column, fieldValue reflect.Value) (interface{}, error) {
	if fieldValue.CanAddr() {
		if fieldConvert, ok := fieldValue.Addr().Interface().(core.Conversion); ok {
			data, err := fieldConvert.ToDB()
			if err != nil {
				return 0, err
			}
			if col.SQLType.IsBlob() {
				return data, nil
			}
			return string(data), nil
		}
	}

	if fieldConvert, ok := fieldValue.Interface().(core.Conversion); ok {
		data, err := fieldConvert.ToDB()
		if err != nil {
			return 0, err
		}
		if col.SQLType.IsBlob() {
			return data, nil
		}
		return string(data), nil
	}

	fieldType := fieldValue.Type()
	k := fieldType.Kind()
	if k == reflect.Ptr {
		if fieldValue.IsNil() {
			return nil, nil
		} else if !fieldValue.IsValid() {
			session.Engine.logger.Warn("the field[", col.FieldName, "] is invalid")
			return nil, nil
		} else {
			// !nashtsai! deference pointer type to instance type
			fieldValue = fieldValue.Elem()
			fieldType = fieldValue.Type()
			k = fieldType.Kind()
		}
	}

	switch k {
	case reflect.Bool:
		return fieldValue.Bool(), nil
	case reflect.String:
		return fieldValue.String(), nil
	case reflect.Struct:
		if fieldType.ConvertibleTo(core.TimeType) {
			t := fieldValue.Convert(core.TimeType).Interface().(time.Time)
			if session.Engine.dialect.DBType() == core.MSSQL {
				if t.IsZero() {
					return nil, nil
				}
			}
			tf := session.Engine.FormatTime(col.SQLType.Name, t)
			return tf, nil
		}

		if !col.SQLType.IsJson() {
			// !<winxxp>! 增加支持driver.Valuer接口的结构，如sql.NullString
			if v, ok := fieldValue.Interface().(driver.Valuer); ok {
				return v.Value()
			}

			fieldTable := session.Engine.autoMapType(fieldValue)
			if len(fieldTable.PrimaryKeys) == 1 {
				pkField := reflect.Indirect(fieldValue).FieldByName(fieldTable.PKColumns()[0].FieldName)
				return pkField.Interface(), nil
			}
			return 0, fmt.Errorf("no primary key for col %v", col.Name)
		}

		if col.SQLType.IsText() {
			bytes, err := json.Marshal(fieldValue.Interface())
			if err != nil {
				session.Engine.logger.Error(err)
				return 0, err
			}
			return string(bytes), nil
		} else if col.SQLType.IsBlob() {
			bytes, err := json.Marshal(fieldValue.Interface())
			if err != nil {
				session.Engine.logger.Error(err)
				return 0, err
			}
			return bytes, nil
		}
		return nil, fmt.Errorf("Unsupported type %v", fieldValue.Type())
	case reflect.Complex64, reflect.Complex128:
		bytes, err := json.Marshal(fieldValue.Interface())
		if err != nil {
			session.Engine.logger.Error(err)
			return 0, err
		}
		return string(bytes), nil
	case reflect.Array, reflect.Slice, reflect.Map:
		if !fieldValue.IsValid() {
			return fieldValue.Interface(), nil
		}

		if col.SQLType.IsText() {
			bytes, err := json.Marshal(fieldValue.Interface())
			if err != nil {
				session.Engine.logger.Error(err)
				return 0, err
			}
			return string(bytes), nil
		} else if col.SQLType.IsBlob() {
			var bytes []byte
			var err error
			if (k == reflect.Array || k == reflect.Slice) &&
				(fieldValue.Type().Elem().Kind() == reflect.Uint8) {
				bytes = fieldValue.Bytes()
			} else {
				bytes, err = json.Marshal(fieldValue.Interface())
				if err != nil {
					session.Engine.logger.Error(err)
					return 0, err
				}
			}
			return bytes, nil
		}
		return nil, ErrUnSupportedType
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return int64(fieldValue.Uint()), nil
	default:
		return fieldValue.Interface(), nil
	}
}

// saveLastSQL stores executed query information
func (session *Session) saveLastSQL(sql string, args ...interface{}) {
	session.lastSQL = sql
	session.lastSQLArgs = args
	session.Engine.logSQL(sql, args...)
}

// LastSQL returns last query information
func (session *Session) LastSQL() (string, []interface{}) {
	return session.lastSQL, session.lastSQLArgs
}

// tbName get some table's table name
func (session *Session) tbNameNoSchema(table *core.Table) string {
	if len(session.Statement.AltTableName) > 0 {
		return session.Statement.AltTableName
	}

	return table.Name
}

// Unscoped always disable struct tag "deleted"
func (session *Session) Unscoped() *Session {
	session.Statement.Unscoped()
	return session
}
