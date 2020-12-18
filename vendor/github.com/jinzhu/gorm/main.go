package gorm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// DB contains information for current db connection
type DB struct {
	sync.RWMutex
	Value        interface{}
	Error        error
	RowsAffected int64

	// single db
	db                SQLCommon
	blockGlobalUpdate bool
	logMode           logModeValue
	logger            logger
	search            *search
	values            sync.Map

	// global db
	parent        *DB
	callbacks     *Callback
	dialect       Dialect
	singularTable bool

	// function to be used to override the creating of a new timestamp
	nowFuncOverride func() time.Time
}

type logModeValue int

const (
	defaultLogMode logModeValue = iota
	noLogMode
	detailedLogMode
)

// Open initialize a new db connection, need to import driver first, e.g:
//
//     import _ "github.com/go-sql-driver/mysql"
//     func main() {
//       db, err := gorm.Open("mysql", "user:password@/dbname?charset=utf8&parseTime=True&loc=Local")
//     }
// GORM has wrapped some drivers, for easier to remember driver's import path, so you could import the mysql driver with
//    import _ "github.com/jinzhu/gorm/dialects/mysql"
//    // import _ "github.com/jinzhu/gorm/dialects/postgres"
//    // import _ "github.com/jinzhu/gorm/dialects/sqlite"
//    // import _ "github.com/jinzhu/gorm/dialects/mssql"
func Open(dialect string, args ...interface{}) (db *DB, err error) {
	if len(args) == 0 {
		err = errors.New("invalid database source")
		return nil, err
	}
	var source string
	var dbSQL SQLCommon
	var ownDbSQL bool

	switch value := args[0].(type) {
	case string:
		var driver = dialect
		if len(args) == 1 {
			source = value
		} else if len(args) >= 2 {
			driver = value
			source = args[1].(string)
		}
		dbSQL, err = sql.Open(driver, source)
		ownDbSQL = true
	case SQLCommon:
		dbSQL = value
		ownDbSQL = false
	default:
		return nil, fmt.Errorf("invalid database source: %v is not a valid type", value)
	}

	db = &DB{
		db:        dbSQL,
		logger:    defaultLogger,
		callbacks: DefaultCallback,
		dialect:   newDialect(dialect, dbSQL),
	}
	db.parent = db
	if err != nil {
		return
	}
	// Send a ping to make sure the database connection is alive.
	if d, ok := dbSQL.(*sql.DB); ok {
		if err = d.Ping(); err != nil && ownDbSQL {
			d.Close()
		}
	}
	return
}

// New clone a new db connection without search conditions
func (s *DB) New() *DB {
	clone := s.clone()
	clone.search = nil
	clone.Value = nil
	return clone
}

type closer interface {
	Close() error
}

// Close close current db connection.  If database connection is not an io.Closer, returns an error.
func (s *DB) Close() error {
	if db, ok := s.parent.db.(closer); ok {
		return db.Close()
	}
	return errors.New("can't close current db")
}

// DB get `*sql.DB` from current connection
// If the underlying database connection is not a *sql.DB, returns nil
func (s *DB) DB() *sql.DB {
	db, _ := s.db.(*sql.DB)
	return db
}

// CommonDB return the underlying `*sql.DB` or `*sql.Tx` instance, mainly intended to allow coexistence with legacy non-GORM code.
func (s *DB) CommonDB() SQLCommon {
	return s.db
}

// Dialect get dialect
func (s *DB) Dialect() Dialect {
	return s.dialect
}

// Callback return `Callbacks` container, you could add/change/delete callbacks with it
//     db.Callback().Create().Register("update_created_at", updateCreated)
// Refer https://jinzhu.github.io/gorm/development.html#callbacks
func (s *DB) Callback() *Callback {
	s.parent.callbacks = s.parent.callbacks.clone(s.logger)
	return s.parent.callbacks
}

// SetLogger replace default logger
func (s *DB) SetLogger(log logger) {
	s.logger = log
}

// LogMode set log mode, `true` for detailed logs, `false` for no log, default, will only print error logs
func (s *DB) LogMode(enable bool) *DB {
	if enable {
		s.logMode = detailedLogMode
	} else {
		s.logMode = noLogMode
	}
	return s
}

// SetNowFuncOverride set the function to be used when creating a new timestamp
func (s *DB) SetNowFuncOverride(nowFuncOverride func() time.Time) *DB {
	s.nowFuncOverride = nowFuncOverride
	return s
}

// Get a new timestamp, using the provided nowFuncOverride on the DB instance if set,
// otherwise defaults to the global NowFunc()
func (s *DB) nowFunc() time.Time {
	if s.nowFuncOverride != nil {
		return s.nowFuncOverride()
	}

	return NowFunc()
}

// BlockGlobalUpdate if true, generates an error on update/delete without where clause.
// This is to prevent eventual error with empty objects updates/deletions
func (s *DB) BlockGlobalUpdate(enable bool) *DB {
	s.blockGlobalUpdate = enable
	return s
}

// HasBlockGlobalUpdate return state of block
func (s *DB) HasBlockGlobalUpdate() bool {
	return s.blockGlobalUpdate
}

// SingularTable use singular table by default
func (s *DB) SingularTable(enable bool) {
	s.parent.Lock()
	defer s.parent.Unlock()
	s.parent.singularTable = enable
}

// NewScope create a scope for current operation
func (s *DB) NewScope(value interface{}) *Scope {
	dbClone := s.clone()
	dbClone.Value = value
	scope := &Scope{db: dbClone, Value: value}
	if s.search != nil {
		scope.Search = s.search.clone()
	} else {
		scope.Search = &search{}
	}
	return scope
}

// QueryExpr returns the query as expr object
func (s *DB) QueryExpr() *expr {
	scope := s.NewScope(s.Value)
	scope.InstanceSet("skip_bindvar", true)
	scope.prepareQuerySQL()

	return Expr(scope.SQL, scope.SQLVars...)
}

// SubQuery returns the query as sub query
func (s *DB) SubQuery() *expr {
	scope := s.NewScope(s.Value)
	scope.InstanceSet("skip_bindvar", true)
	scope.prepareQuerySQL()

	return Expr(fmt.Sprintf("(%v)", scope.SQL), scope.SQLVars...)
}

// Where return a new relation, filter records with given conditions, accepts `map`, `struct` or `string` as conditions, refer http://jinzhu.github.io/gorm/crud.html#query
func (s *DB) Where(query interface{}, args ...interface{}) *DB {
	return s.clone().search.Where(query, args...).db
}

// Or filter records that match before conditions or this one, similar to `Where`
func (s *DB) Or(query interface{}, args ...interface{}) *DB {
	return s.clone().search.Or(query, args...).db
}

// Not filter records that don't match current conditions, similar to `Where`
func (s *DB) Not(query interface{}, args ...interface{}) *DB {
	return s.clone().search.Not(query, args...).db
}

// Limit specify the number of records to be retrieved
func (s *DB) Limit(limit interface{}) *DB {
	return s.clone().search.Limit(limit).db
}

// Offset specify the number of records to skip before starting to return the records
func (s *DB) Offset(offset interface{}) *DB {
	return s.clone().search.Offset(offset).db
}

// Order specify order when retrieve records from database, set reorder to `true` to overwrite defined conditions
//     db.Order("name DESC")
//     db.Order("name DESC", true) // reorder
//     db.Order(gorm.Expr("name = ? DESC", "first")) // sql expression
func (s *DB) Order(value interface{}, reorder ...bool) *DB {
	return s.clone().search.Order(value, reorder...).db
}

// Select specify fields that you want to retrieve from database when querying, by default, will select all fields;
// When creating/updating, specify fields that you want to save to database
func (s *DB) Select(query interface{}, args ...interface{}) *DB {
	return s.clone().search.Select(query, args...).db
}

// Omit specify fields that you want to ignore when saving to database for creating, updating
func (s *DB) Omit(columns ...string) *DB {
	return s.clone().search.Omit(columns...).db
}

// Group specify the group method on the find
func (s *DB) Group(query string) *DB {
	return s.clone().search.Group(query).db
}

// Having specify HAVING conditions for GROUP BY
func (s *DB) Having(query interface{}, values ...interface{}) *DB {
	return s.clone().search.Having(query, values...).db
}

// Joins specify Joins conditions
//     db.Joins("JOIN emails ON emails.user_id = users.id AND emails.email = ?", "jinzhu@example.org").Find(&user)
func (s *DB) Joins(query string, args ...interface{}) *DB {
	return s.clone().search.Joins(query, args...).db
}

// Scopes pass current database connection to arguments `func(*DB) *DB`, which could be used to add conditions dynamically
//     func AmountGreaterThan1000(db *gorm.DB) *gorm.DB {
//         return db.Where("amount > ?", 1000)
//     }
//
//     func OrderStatus(status []string) func (db *gorm.DB) *gorm.DB {
//         return func (db *gorm.DB) *gorm.DB {
//             return db.Scopes(AmountGreaterThan1000).Where("status in (?)", status)
//         }
//     }
//
//     db.Scopes(AmountGreaterThan1000, OrderStatus([]string{"paid", "shipped"})).Find(&orders)
// Refer https://jinzhu.github.io/gorm/crud.html#scopes
func (s *DB) Scopes(funcs ...func(*DB) *DB) *DB {
	for _, f := range funcs {
		s = f(s)
	}
	return s
}

// Unscoped return all record including deleted record, refer Soft Delete https://jinzhu.github.io/gorm/crud.html#soft-delete
func (s *DB) Unscoped() *DB {
	return s.clone().search.unscoped().db
}

// Attrs initialize struct with argument if record not found with `FirstOrInit` https://jinzhu.github.io/gorm/crud.html#firstorinit or `FirstOrCreate` https://jinzhu.github.io/gorm/crud.html#firstorcreate
func (s *DB) Attrs(attrs ...interface{}) *DB {
	return s.clone().search.Attrs(attrs...).db
}

// Assign assign result with argument regardless it is found or not with `FirstOrInit` https://jinzhu.github.io/gorm/crud.html#firstorinit or `FirstOrCreate` https://jinzhu.github.io/gorm/crud.html#firstorcreate
func (s *DB) Assign(attrs ...interface{}) *DB {
	return s.clone().search.Assign(attrs...).db
}

// First find first record that match given conditions, order by primary key
func (s *DB) First(out interface{}, where ...interface{}) *DB {
	newScope := s.NewScope(out)
	newScope.Search.Limit(1)

	return newScope.Set("gorm:order_by_primary_key", "ASC").
		inlineCondition(where...).callCallbacks(s.parent.callbacks.queries).db
}

// Take return a record that match given conditions, the order will depend on the database implementation
func (s *DB) Take(out interface{}, where ...interface{}) *DB {
	newScope := s.NewScope(out)
	newScope.Search.Limit(1)
	return newScope.inlineCondition(where...).callCallbacks(s.parent.callbacks.queries).db
}

// Last find last record that match given conditions, order by primary key
func (s *DB) Last(out interface{}, where ...interface{}) *DB {
	newScope := s.NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "DESC").
		inlineCondition(where...).callCallbacks(s.parent.callbacks.queries).db
}

// Find find records that match given conditions
func (s *DB) Find(out interface{}, where ...interface{}) *DB {
	return s.NewScope(out).inlineCondition(where...).callCallbacks(s.parent.callbacks.queries).db
}

//Preloads preloads relations, don`t touch out
func (s *DB) Preloads(out interface{}) *DB {
	return s.NewScope(out).InstanceSet("gorm:only_preload", 1).callCallbacks(s.parent.callbacks.queries).db
}

// Scan scan value to a struct
func (s *DB) Scan(dest interface{}) *DB {
	return s.NewScope(s.Value).Set("gorm:query_destination", dest).callCallbacks(s.parent.callbacks.queries).db
}

// Row return `*sql.Row` with given conditions
func (s *DB) Row() *sql.Row {
	return s.NewScope(s.Value).row()
}

// Rows return `*sql.Rows` with given conditions
func (s *DB) Rows() (*sql.Rows, error) {
	return s.NewScope(s.Value).rows()
}

// ScanRows scan `*sql.Rows` to give struct
func (s *DB) ScanRows(rows *sql.Rows, result interface{}) error {
	var (
		scope        = s.NewScope(result)
		clone        = scope.db
		columns, err = rows.Columns()
	)

	if clone.AddError(err) == nil {
		scope.scan(rows, columns, scope.Fields())
	}

	return clone.Error
}

// Pluck used to query single column from a model as a map
//     var ages []int64
//     db.Find(&users).Pluck("age", &ages)
func (s *DB) Pluck(column string, value interface{}) *DB {
	return s.NewScope(s.Value).pluck(column, value).db
}

// Count get how many records for a model
func (s *DB) Count(value interface{}) *DB {
	return s.NewScope(s.Value).count(value).db
}

// Related get related associations
func (s *DB) Related(value interface{}, foreignKeys ...string) *DB {
	return s.NewScope(s.Value).related(value, foreignKeys...).db
}

// FirstOrInit find first matched record or initialize a new one with given conditions (only works with struct, map conditions)
// https://jinzhu.github.io/gorm/crud.html#firstorinit
func (s *DB) FirstOrInit(out interface{}, where ...interface{}) *DB {
	c := s.clone()
	if result := c.First(out, where...); result.Error != nil {
		if !result.RecordNotFound() {
			return result
		}
		c.NewScope(out).inlineCondition(where...).initialize()
	} else {
		c.NewScope(out).updatedAttrsWithValues(c.search.assignAttrs)
	}
	return c
}

// FirstOrCreate find first matched record or create a new one with given conditions (only works with struct, map conditions)
// https://jinzhu.github.io/gorm/crud.html#firstorcreate
func (s *DB) FirstOrCreate(out interface{}, where ...interface{}) *DB {
	c := s.clone()
	if result := s.First(out, where...); result.Error != nil {
		if !result.RecordNotFound() {
			return result
		}
		return c.NewScope(out).inlineCondition(where...).initialize().callCallbacks(c.parent.callbacks.creates).db
	} else if len(c.search.assignAttrs) > 0 {
		return c.NewScope(out).InstanceSet("gorm:update_interface", c.search.assignAttrs).callCallbacks(c.parent.callbacks.updates).db
	}
	return c
}

// Update update attributes with callbacks, refer: https://jinzhu.github.io/gorm/crud.html#update
func (s *DB) Update(attrs ...interface{}) *DB {
	return s.Updates(toSearchableMap(attrs...), true)
}

// Updates update attributes with callbacks, refer: https://jinzhu.github.io/gorm/crud.html#update
func (s *DB) Updates(values interface{}, ignoreProtectedAttrs ...bool) *DB {
	return s.NewScope(s.Value).
		Set("gorm:ignore_protected_attrs", len(ignoreProtectedAttrs) > 0).
		InstanceSet("gorm:update_interface", values).
		callCallbacks(s.parent.callbacks.updates).db
}

// UpdateColumn update attributes without callbacks, refer: https://jinzhu.github.io/gorm/crud.html#update
func (s *DB) UpdateColumn(attrs ...interface{}) *DB {
	return s.UpdateColumns(toSearchableMap(attrs...))
}

// UpdateColumns update attributes without callbacks, refer: https://jinzhu.github.io/gorm/crud.html#update
func (s *DB) UpdateColumns(values interface{}) *DB {
	return s.NewScope(s.Value).
		Set("gorm:update_column", true).
		Set("gorm:save_associations", false).
		InstanceSet("gorm:update_interface", values).
		callCallbacks(s.parent.callbacks.updates).db
}

// Save update value in database, if the value doesn't have primary key, will insert it
func (s *DB) Save(value interface{}) *DB {
	scope := s.NewScope(value)
	if !scope.PrimaryKeyZero() {
		newDB := scope.callCallbacks(s.parent.callbacks.updates).db
		if newDB.Error == nil && newDB.RowsAffected == 0 {
			return s.New().Table(scope.TableName()).FirstOrCreate(value)
		}
		return newDB
	}
	return scope.callCallbacks(s.parent.callbacks.creates).db
}

// Create insert the value into database
func (s *DB) Create(value interface{}) *DB {
	scope := s.NewScope(value)
	return scope.callCallbacks(s.parent.callbacks.creates).db
}

// Delete delete value match given conditions, if the value has primary key, then will including the primary key as condition
func (s *DB) Delete(value interface{}, where ...interface{}) *DB {
	return s.NewScope(value).inlineCondition(where...).callCallbacks(s.parent.callbacks.deletes).db
}

// Raw use raw sql as conditions, won't run it unless invoked by other methods
//    db.Raw("SELECT name, age FROM users WHERE name = ?", 3).Scan(&result)
func (s *DB) Raw(sql string, values ...interface{}) *DB {
	return s.clone().search.Raw(true).Where(sql, values...).db
}

// Exec execute raw sql
func (s *DB) Exec(sql string, values ...interface{}) *DB {
	scope := s.NewScope(nil)
	generatedSQL := scope.buildCondition(map[string]interface{}{"query": sql, "args": values}, true)
	generatedSQL = strings.TrimSuffix(strings.TrimPrefix(generatedSQL, "("), ")")
	scope.Raw(generatedSQL)
	return scope.Exec().db
}

// Model specify the model you would like to run db operations
//    // update all users's name to `hello`
//    db.Model(&User{}).Update("name", "hello")
//    // if user's primary key is non-blank, will use it as condition, then will only update the user's name to `hello`
//    db.Model(&user).Update("name", "hello")
func (s *DB) Model(value interface{}) *DB {
	c := s.clone()
	c.Value = value
	return c
}

// Table specify the table you would like to run db operations
func (s *DB) Table(name string) *DB {
	clone := s.clone()
	clone.search.Table(name)
	clone.Value = nil
	return clone
}

// Debug start debug mode
func (s *DB) Debug() *DB {
	return s.clone().LogMode(true)
}

// Begin begins a transaction
func (s *DB) Begin() *DB {
	return s.BeginTx(context.Background(), &sql.TxOptions{})
}

// BeginTx begins a transaction with options
func (s *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) *DB {
	c := s.clone()
	if db, ok := c.db.(sqlDb); ok && db != nil {
		tx, err := db.BeginTx(ctx, opts)
		c.db = interface{}(tx).(SQLCommon)

		c.dialect.SetDB(c.db)
		c.AddError(err)
	} else {
		c.AddError(ErrCantStartTransaction)
	}
	return c
}

// Commit commit a transaction
func (s *DB) Commit() *DB {
	var emptySQLTx *sql.Tx
	if db, ok := s.db.(sqlTx); ok && db != nil && db != emptySQLTx {
		s.AddError(db.Commit())
	} else {
		s.AddError(ErrInvalidTransaction)
	}
	return s
}

// Rollback rollback a transaction
func (s *DB) Rollback() *DB {
	var emptySQLTx *sql.Tx
	if db, ok := s.db.(sqlTx); ok && db != nil && db != emptySQLTx {
		if err := db.Rollback(); err != nil && err != sql.ErrTxDone {
			s.AddError(err)
		}
	} else {
		s.AddError(ErrInvalidTransaction)
	}
	return s
}

// RollbackUnlessCommitted rollback a transaction if it has not yet been
// committed.
func (s *DB) RollbackUnlessCommitted() *DB {
	var emptySQLTx *sql.Tx
	if db, ok := s.db.(sqlTx); ok && db != nil && db != emptySQLTx {
		err := db.Rollback()
		// Ignore the error indicating that the transaction has already
		// been committed.
		if err != sql.ErrTxDone {
			s.AddError(err)
		}
	} else {
		s.AddError(ErrInvalidTransaction)
	}
	return s
}

// NewRecord check if value's primary key is blank
func (s *DB) NewRecord(value interface{}) bool {
	return s.NewScope(value).PrimaryKeyZero()
}

// RecordNotFound check if returning ErrRecordNotFound error
func (s *DB) RecordNotFound() bool {
	for _, err := range s.GetErrors() {
		if err == ErrRecordNotFound {
			return true
		}
	}
	return false
}

// CreateTable create table for models
func (s *DB) CreateTable(models ...interface{}) *DB {
	db := s.Unscoped()
	for _, model := range models {
		db = db.NewScope(model).createTable().db
	}
	return db
}

// DropTable drop table for models
func (s *DB) DropTable(values ...interface{}) *DB {
	db := s.clone()
	for _, value := range values {
		if tableName, ok := value.(string); ok {
			db = db.Table(tableName)
		}

		db = db.NewScope(value).dropTable().db
	}
	return db
}

// DropTableIfExists drop table if it is exist
func (s *DB) DropTableIfExists(values ...interface{}) *DB {
	db := s.clone()
	for _, value := range values {
		if s.HasTable(value) {
			db.AddError(s.DropTable(value).Error)
		}
	}
	return db
}

// HasTable check has table or not
func (s *DB) HasTable(value interface{}) bool {
	var (
		scope     = s.NewScope(value)
		tableName string
	)

	if name, ok := value.(string); ok {
		tableName = name
	} else {
		tableName = scope.TableName()
	}

	has := scope.Dialect().HasTable(tableName)
	s.AddError(scope.db.Error)
	return has
}

// AutoMigrate run auto migration for given models, will only add missing fields, won't delete/change current data
func (s *DB) AutoMigrate(values ...interface{}) *DB {
	db := s.Unscoped()
	for _, value := range values {
		db = db.NewScope(value).autoMigrate().db
	}
	return db
}

// ModifyColumn modify column to type
func (s *DB) ModifyColumn(column string, typ string) *DB {
	scope := s.NewScope(s.Value)
	scope.modifyColumn(column, typ)
	return scope.db
}

// DropColumn drop a column
func (s *DB) DropColumn(column string) *DB {
	scope := s.NewScope(s.Value)
	scope.dropColumn(column)
	return scope.db
}

// AddIndex add index for columns with given name
func (s *DB) AddIndex(indexName string, columns ...string) *DB {
	scope := s.Unscoped().NewScope(s.Value)
	scope.addIndex(false, indexName, columns...)
	return scope.db
}

// AddUniqueIndex add unique index for columns with given name
func (s *DB) AddUniqueIndex(indexName string, columns ...string) *DB {
	scope := s.Unscoped().NewScope(s.Value)
	scope.addIndex(true, indexName, columns...)
	return scope.db
}

// RemoveIndex remove index with name
func (s *DB) RemoveIndex(indexName string) *DB {
	scope := s.NewScope(s.Value)
	scope.removeIndex(indexName)
	return scope.db
}

// AddForeignKey Add foreign key to the given scope, e.g:
//     db.Model(&User{}).AddForeignKey("city_id", "cities(id)", "RESTRICT", "RESTRICT")
func (s *DB) AddForeignKey(field string, dest string, onDelete string, onUpdate string) *DB {
	scope := s.NewScope(s.Value)
	scope.addForeignKey(field, dest, onDelete, onUpdate)
	return scope.db
}

// RemoveForeignKey Remove foreign key from the given scope, e.g:
//     db.Model(&User{}).RemoveForeignKey("city_id", "cities(id)")
func (s *DB) RemoveForeignKey(field string, dest string) *DB {
	scope := s.clone().NewScope(s.Value)
	scope.removeForeignKey(field, dest)
	return scope.db
}

// Association start `Association Mode` to handler relations things easir in that mode, refer: https://jinzhu.github.io/gorm/associations.html#association-mode
func (s *DB) Association(column string) *Association {
	var err error
	var scope = s.Set("gorm:association:source", s.Value).NewScope(s.Value)

	if primaryField := scope.PrimaryField(); primaryField.IsBlank {
		err = errors.New("primary key can't be nil")
	} else {
		if field, ok := scope.FieldByName(column); ok {
			if field.Relationship == nil || len(field.Relationship.ForeignFieldNames) == 0 {
				err = fmt.Errorf("invalid association %v for %v", column, scope.IndirectValue().Type())
			} else {
				return &Association{scope: scope, column: column, field: field}
			}
		} else {
			err = fmt.Errorf("%v doesn't have column %v", scope.IndirectValue().Type(), column)
		}
	}

	return &Association{Error: err}
}

// Preload preload associations with given conditions
//    db.Preload("Orders", "state NOT IN (?)", "cancelled").Find(&users)
func (s *DB) Preload(column string, conditions ...interface{}) *DB {
	return s.clone().search.Preload(column, conditions...).db
}

// Set set setting by name, which could be used in callbacks, will clone a new db, and update its setting
func (s *DB) Set(name string, value interface{}) *DB {
	return s.clone().InstantSet(name, value)
}

// InstantSet instant set setting, will affect current db
func (s *DB) InstantSet(name string, value interface{}) *DB {
	s.values.Store(name, value)
	return s
}

// Get get setting by name
func (s *DB) Get(name string) (value interface{}, ok bool) {
	value, ok = s.values.Load(name)
	return
}

// SetJoinTableHandler set a model's join table handler for a relation
func (s *DB) SetJoinTableHandler(source interface{}, column string, handler JoinTableHandlerInterface) {
	scope := s.NewScope(source)
	for _, field := range scope.GetModelStruct().StructFields {
		if field.Name == column || field.DBName == column {
			if many2many, _ := field.TagSettingsGet("MANY2MANY"); many2many != "" {
				source := (&Scope{Value: source}).GetModelStruct().ModelType
				destination := (&Scope{Value: reflect.New(field.Struct.Type).Interface()}).GetModelStruct().ModelType
				handler.Setup(field.Relationship, many2many, source, destination)
				field.Relationship.JoinTableHandler = handler
				if table := handler.Table(s); scope.Dialect().HasTable(table) {
					s.Table(table).AutoMigrate(handler)
				}
			}
		}
	}
}

// AddError add error to the db
func (s *DB) AddError(err error) error {
	if err != nil {
		if err != ErrRecordNotFound {
			if s.logMode == defaultLogMode {
				go s.print("error", fileWithLineNum(), err)
			} else {
				s.log(err)
			}

			errors := Errors(s.GetErrors())
			errors = errors.Add(err)
			if len(errors) > 1 {
				err = errors
			}
		}

		s.Error = err
	}
	return err
}

// GetErrors get happened errors from the db
func (s *DB) GetErrors() []error {
	if errs, ok := s.Error.(Errors); ok {
		return errs
	} else if s.Error != nil {
		return []error{s.Error}
	}
	return []error{}
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For DB
////////////////////////////////////////////////////////////////////////////////

func (s *DB) clone() *DB {
	db := &DB{
		db:                s.db,
		parent:            s.parent,
		logger:            s.logger,
		logMode:           s.logMode,
		Value:             s.Value,
		Error:             s.Error,
		blockGlobalUpdate: s.blockGlobalUpdate,
		dialect:           newDialect(s.dialect.GetName(), s.db),
		nowFuncOverride:   s.nowFuncOverride,
	}

	s.values.Range(func(k, v interface{}) bool {
		db.values.Store(k, v)
		return true
	})

	if s.search == nil {
		db.search = &search{limit: -1, offset: -1}
	} else {
		db.search = s.search.clone()
	}

	db.search.db = db
	return db
}

func (s *DB) print(v ...interface{}) {
	s.logger.Print(v...)
}

func (s *DB) log(v ...interface{}) {
	if s != nil && s.logMode == detailedLogMode {
		s.print(append([]interface{}{"log", fileWithLineNum()}, v...)...)
	}
}

func (s *DB) slog(sql string, t time.Time, vars ...interface{}) {
	if s.logMode == detailedLogMode {
		s.print("sql", fileWithLineNum(), NowFunc().Sub(t), sql, vars, s.RowsAffected)
	}
}
