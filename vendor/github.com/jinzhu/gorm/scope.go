package gorm

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// Scope contain current operation's information when you perform any operation on the database
type Scope struct {
	Search          *search
	Value           interface{}
	SQL             string
	SQLVars         []interface{}
	db              *DB
	instanceID      string
	primaryKeyField *Field
	skipLeft        bool
	fields          *[]*Field
	selectAttrs     *[]string
}

// IndirectValue return scope's reflect value's indirect value
func (scope *Scope) IndirectValue() reflect.Value {
	return indirect(reflect.ValueOf(scope.Value))
}

// New create a new Scope without search information
func (scope *Scope) New(value interface{}) *Scope {
	return &Scope{db: scope.NewDB(), Search: &search{}, Value: value}
}

////////////////////////////////////////////////////////////////////////////////
// Scope DB
////////////////////////////////////////////////////////////////////////////////

// DB return scope's DB connection
func (scope *Scope) DB() *DB {
	return scope.db
}

// NewDB create a new DB without search information
func (scope *Scope) NewDB() *DB {
	if scope.db != nil {
		db := scope.db.clone()
		db.search = nil
		db.Value = nil
		return db
	}
	return nil
}

// SQLDB return *sql.DB
func (scope *Scope) SQLDB() SQLCommon {
	return scope.db.db
}

// Dialect get dialect
func (scope *Scope) Dialect() Dialect {
	return scope.db.dialect
}

// Quote used to quote string to escape them for database
func (scope *Scope) Quote(str string) string {
	if strings.Contains(str, ".") {
		newStrs := []string{}
		for _, str := range strings.Split(str, ".") {
			newStrs = append(newStrs, scope.Dialect().Quote(str))
		}
		return strings.Join(newStrs, ".")
	}

	return scope.Dialect().Quote(str)
}

// Err add error to Scope
func (scope *Scope) Err(err error) error {
	if err != nil {
		scope.db.AddError(err)
	}
	return err
}

// HasError check if there are any error
func (scope *Scope) HasError() bool {
	return scope.db.Error != nil
}

// Log print log message
func (scope *Scope) Log(v ...interface{}) {
	scope.db.log(v...)
}

// SkipLeft skip remaining callbacks
func (scope *Scope) SkipLeft() {
	scope.skipLeft = true
}

// Fields get value's fields
func (scope *Scope) Fields() []*Field {
	if scope.fields == nil {
		var (
			fields             []*Field
			indirectScopeValue = scope.IndirectValue()
			isStruct           = indirectScopeValue.Kind() == reflect.Struct
		)

		for _, structField := range scope.GetModelStruct().StructFields {
			if isStruct {
				fieldValue := indirectScopeValue
				for _, name := range structField.Names {
					if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
						fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
					}
					fieldValue = reflect.Indirect(fieldValue).FieldByName(name)
				}
				fields = append(fields, &Field{StructField: structField, Field: fieldValue, IsBlank: isBlank(fieldValue)})
			} else {
				fields = append(fields, &Field{StructField: structField, IsBlank: true})
			}
		}
		scope.fields = &fields
	}

	return *scope.fields
}

// FieldByName find `gorm.Field` with field name or db name
func (scope *Scope) FieldByName(name string) (field *Field, ok bool) {
	var (
		dbName           = ToColumnName(name)
		mostMatchedField *Field
	)

	for _, field := range scope.Fields() {
		if field.Name == name || field.DBName == name {
			return field, true
		}
		if field.DBName == dbName {
			mostMatchedField = field
		}
	}
	return mostMatchedField, mostMatchedField != nil
}

// PrimaryFields return scope's primary fields
func (scope *Scope) PrimaryFields() (fields []*Field) {
	for _, field := range scope.Fields() {
		if field.IsPrimaryKey {
			fields = append(fields, field)
		}
	}
	return fields
}

// PrimaryField return scope's main primary field, if defined more that one primary fields, will return the one having column name `id` or the first one
func (scope *Scope) PrimaryField() *Field {
	if primaryFields := scope.GetModelStruct().PrimaryFields; len(primaryFields) > 0 {
		if len(primaryFields) > 1 {
			if field, ok := scope.FieldByName("id"); ok {
				return field
			}
		}
		return scope.PrimaryFields()[0]
	}
	return nil
}

// PrimaryKey get main primary field's db name
func (scope *Scope) PrimaryKey() string {
	if field := scope.PrimaryField(); field != nil {
		return field.DBName
	}
	return ""
}

// PrimaryKeyZero check main primary field's value is blank or not
func (scope *Scope) PrimaryKeyZero() bool {
	field := scope.PrimaryField()
	return field == nil || field.IsBlank
}

// PrimaryKeyValue get the primary key's value
func (scope *Scope) PrimaryKeyValue() interface{} {
	if field := scope.PrimaryField(); field != nil && field.Field.IsValid() {
		return field.Field.Interface()
	}
	return 0
}

// HasColumn to check if has column
func (scope *Scope) HasColumn(column string) bool {
	for _, field := range scope.GetStructFields() {
		if field.IsNormal && (field.Name == column || field.DBName == column) {
			return true
		}
	}
	return false
}

// SetColumn to set the column's value, column could be field or field's name/dbname
func (scope *Scope) SetColumn(column interface{}, value interface{}) error {
	var updateAttrs = map[string]interface{}{}
	if attrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
		updateAttrs = attrs.(map[string]interface{})
		defer scope.InstanceSet("gorm:update_attrs", updateAttrs)
	}

	if field, ok := column.(*Field); ok {
		updateAttrs[field.DBName] = value
		return field.Set(value)
	} else if name, ok := column.(string); ok {
		var (
			dbName           = ToDBName(name)
			mostMatchedField *Field
		)
		for _, field := range scope.Fields() {
			if field.DBName == value {
				updateAttrs[field.DBName] = value
				return field.Set(value)
			}
			if (field.DBName == dbName) || (field.Name == name && mostMatchedField == nil) {
				mostMatchedField = field
			}
		}

		if mostMatchedField != nil {
			updateAttrs[mostMatchedField.DBName] = value
			return mostMatchedField.Set(value)
		}
	}
	return errors.New("could not convert column to field")
}

// CallMethod call scope value's method, if it is a slice, will call its element's method one by one
func (scope *Scope) CallMethod(methodName string) {
	if scope.Value == nil {
		return
	}

	if indirectScopeValue := scope.IndirectValue(); indirectScopeValue.Kind() == reflect.Slice {
		for i := 0; i < indirectScopeValue.Len(); i++ {
			scope.callMethod(methodName, indirectScopeValue.Index(i))
		}
	} else {
		scope.callMethod(methodName, indirectScopeValue)
	}
}

// AddToVars add value as sql's vars, used to prevent SQL injection
func (scope *Scope) AddToVars(value interface{}) string {
	_, skipBindVar := scope.InstanceGet("skip_bindvar")

	if expr, ok := value.(*expr); ok {
		exp := expr.expr
		for _, arg := range expr.args {
			if skipBindVar {
				scope.AddToVars(arg)
			} else {
				exp = strings.Replace(exp, "?", scope.AddToVars(arg), 1)
			}
		}
		return exp
	}

	scope.SQLVars = append(scope.SQLVars, value)

	if skipBindVar {
		return "?"
	}
	return scope.Dialect().BindVar(len(scope.SQLVars))
}

// SelectAttrs return selected attributes
func (scope *Scope) SelectAttrs() []string {
	if scope.selectAttrs == nil {
		attrs := []string{}
		for _, value := range scope.Search.selects {
			if str, ok := value.(string); ok {
				attrs = append(attrs, str)
			} else if strs, ok := value.([]string); ok {
				attrs = append(attrs, strs...)
			} else if strs, ok := value.([]interface{}); ok {
				for _, str := range strs {
					attrs = append(attrs, fmt.Sprintf("%v", str))
				}
			}
		}
		scope.selectAttrs = &attrs
	}
	return *scope.selectAttrs
}

// OmitAttrs return omitted attributes
func (scope *Scope) OmitAttrs() []string {
	return scope.Search.omits
}

type tabler interface {
	TableName() string
}

type dbTabler interface {
	TableName(*DB) string
}

// TableName return table name
func (scope *Scope) TableName() string {
	if scope.Search != nil && len(scope.Search.tableName) > 0 {
		return scope.Search.tableName
	}

	if tabler, ok := scope.Value.(tabler); ok {
		return tabler.TableName()
	}

	if tabler, ok := scope.Value.(dbTabler); ok {
		return tabler.TableName(scope.db)
	}

	return scope.GetModelStruct().TableName(scope.db.Model(scope.Value))
}

// QuotedTableName return quoted table name
func (scope *Scope) QuotedTableName() (name string) {
	if scope.Search != nil && len(scope.Search.tableName) > 0 {
		if strings.Contains(scope.Search.tableName, " ") {
			return scope.Search.tableName
		}
		return scope.Quote(scope.Search.tableName)
	}

	return scope.Quote(scope.TableName())
}

// CombinedConditionSql return combined condition sql
func (scope *Scope) CombinedConditionSql() string {
	joinSQL := scope.joinsSQL()
	whereSQL := scope.whereSQL()
	if scope.Search.raw {
		whereSQL = strings.TrimSuffix(strings.TrimPrefix(whereSQL, "WHERE ("), ")")
	}
	return joinSQL + whereSQL + scope.groupSQL() +
		scope.havingSQL() + scope.orderSQL() + scope.limitAndOffsetSQL()
}

// Raw set raw sql
func (scope *Scope) Raw(sql string) *Scope {
	scope.SQL = strings.Replace(sql, "$$$", "?", -1)
	return scope
}

// Exec perform generated SQL
func (scope *Scope) Exec() *Scope {
	defer scope.trace(scope.db.nowFunc())

	if !scope.HasError() {
		if result, err := scope.SQLDB().Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
			if count, err := result.RowsAffected(); scope.Err(err) == nil {
				scope.db.RowsAffected = count
			}
		}
	}
	return scope
}

// Set set value by name
func (scope *Scope) Set(name string, value interface{}) *Scope {
	scope.db.InstantSet(name, value)
	return scope
}

// Get get setting by name
func (scope *Scope) Get(name string) (interface{}, bool) {
	return scope.db.Get(name)
}

// InstanceID get InstanceID for scope
func (scope *Scope) InstanceID() string {
	if scope.instanceID == "" {
		scope.instanceID = fmt.Sprintf("%v%v", &scope, &scope.db)
	}
	return scope.instanceID
}

// InstanceSet set instance setting for current operation, but not for operations in callbacks, like saving associations callback
func (scope *Scope) InstanceSet(name string, value interface{}) *Scope {
	return scope.Set(name+scope.InstanceID(), value)
}

// InstanceGet get instance setting from current operation
func (scope *Scope) InstanceGet(name string) (interface{}, bool) {
	return scope.Get(name + scope.InstanceID())
}

// Begin start a transaction
func (scope *Scope) Begin() *Scope {
	if db, ok := scope.SQLDB().(sqlDb); ok {
		if tx, err := db.Begin(); scope.Err(err) == nil {
			scope.db.db = interface{}(tx).(SQLCommon)
			scope.InstanceSet("gorm:started_transaction", true)
		}
	}
	return scope
}

// CommitOrRollback commit current transaction if no error happened, otherwise will rollback it
func (scope *Scope) CommitOrRollback() *Scope {
	if _, ok := scope.InstanceGet("gorm:started_transaction"); ok {
		if db, ok := scope.db.db.(sqlTx); ok {
			if scope.HasError() {
				db.Rollback()
			} else {
				scope.Err(db.Commit())
			}
			scope.db.db = scope.db.parent.db
		}
	}
	return scope
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.Scope
////////////////////////////////////////////////////////////////////////////////

func (scope *Scope) callMethod(methodName string, reflectValue reflect.Value) {
	// Only get address from non-pointer
	if reflectValue.CanAddr() && reflectValue.Kind() != reflect.Ptr {
		reflectValue = reflectValue.Addr()
	}

	if methodValue := reflectValue.MethodByName(methodName); methodValue.IsValid() {
		switch method := methodValue.Interface().(type) {
		case func():
			method()
		case func(*Scope):
			method(scope)
		case func(*DB):
			newDB := scope.NewDB()
			method(newDB)
			scope.Err(newDB.Error)
		case func() error:
			scope.Err(method())
		case func(*Scope) error:
			scope.Err(method(scope))
		case func(*DB) error:
			newDB := scope.NewDB()
			scope.Err(method(newDB))
			scope.Err(newDB.Error)
		default:
			scope.Err(fmt.Errorf("unsupported function %v", methodName))
		}
	}
}

var (
	columnRegexp        = regexp.MustCompile("^[a-zA-Z\\d]+(\\.[a-zA-Z\\d]+)*$") // only match string like `name`, `users.name`
	isNumberRegexp      = regexp.MustCompile("^\\s*\\d+\\s*$")                   // match if string is number
	comparisonRegexp    = regexp.MustCompile("(?i) (=|<>|(>|<)(=?)|LIKE|IS|IN) ")
	countingQueryRegexp = regexp.MustCompile("(?i)^count(.+)$")
)

func (scope *Scope) quoteIfPossible(str string) string {
	if columnRegexp.MatchString(str) {
		return scope.Quote(str)
	}
	return str
}

func (scope *Scope) scan(rows *sql.Rows, columns []string, fields []*Field) {
	var (
		ignored            interface{}
		values             = make([]interface{}, len(columns))
		selectFields       []*Field
		selectedColumnsMap = map[string]int{}
		resetFields        = map[int]*Field{}
	)

	for index, column := range columns {
		values[index] = &ignored

		selectFields = fields
		offset := 0
		if idx, ok := selectedColumnsMap[column]; ok {
			offset = idx + 1
			selectFields = selectFields[offset:]
		}

		for fieldIndex, field := range selectFields {
			if field.DBName == column {
				if field.Field.Kind() == reflect.Ptr {
					values[index] = field.Field.Addr().Interface()
				} else {
					reflectValue := reflect.New(reflect.PtrTo(field.Struct.Type))
					reflectValue.Elem().Set(field.Field.Addr())
					values[index] = reflectValue.Interface()
					resetFields[index] = field
				}

				selectedColumnsMap[column] = offset + fieldIndex

				if field.IsNormal {
					break
				}
			}
		}
	}

	scope.Err(rows.Scan(values...))

	for index, field := range resetFields {
		if v := reflect.ValueOf(values[index]).Elem().Elem(); v.IsValid() {
			field.Field.Set(v)
		}
	}
}

func (scope *Scope) primaryCondition(value interface{}) string {
	return fmt.Sprintf("(%v.%v = %v)", scope.QuotedTableName(), scope.Quote(scope.PrimaryKey()), value)
}

func (scope *Scope) buildCondition(clause map[string]interface{}, include bool) (str string) {
	var (
		quotedTableName  = scope.QuotedTableName()
		quotedPrimaryKey = scope.Quote(scope.PrimaryKey())
		equalSQL         = "="
		inSQL            = "IN"
	)

	// If building not conditions
	if !include {
		equalSQL = "<>"
		inSQL = "NOT IN"
	}

	switch value := clause["query"].(type) {
	case sql.NullInt64:
		return fmt.Sprintf("(%v.%v %s %v)", quotedTableName, quotedPrimaryKey, equalSQL, value.Int64)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("(%v.%v %s %v)", quotedTableName, quotedPrimaryKey, equalSQL, value)
	case []int, []int8, []int16, []int32, []int64, []uint, []uint8, []uint16, []uint32, []uint64, []string, []interface{}:
		if !include && reflect.ValueOf(value).Len() == 0 {
			return
		}
		str = fmt.Sprintf("(%v.%v %s (?))", quotedTableName, quotedPrimaryKey, inSQL)
		clause["args"] = []interface{}{value}
	case string:
		if isNumberRegexp.MatchString(value) {
			return fmt.Sprintf("(%v.%v %s %v)", quotedTableName, quotedPrimaryKey, equalSQL, scope.AddToVars(value))
		}

		if value != "" {
			if !include {
				if comparisonRegexp.MatchString(value) {
					str = fmt.Sprintf("NOT (%v)", value)
				} else {
					str = fmt.Sprintf("(%v.%v NOT IN (?))", quotedTableName, scope.Quote(value))
				}
			} else {
				str = fmt.Sprintf("(%v)", value)
			}
		}
	case map[string]interface{}:
		var sqls []string
		for key, value := range value {
			if value != nil {
				sqls = append(sqls, fmt.Sprintf("(%v.%v %s %v)", quotedTableName, scope.Quote(key), equalSQL, scope.AddToVars(value)))
			} else {
				if !include {
					sqls = append(sqls, fmt.Sprintf("(%v.%v IS NOT NULL)", quotedTableName, scope.Quote(key)))
				} else {
					sqls = append(sqls, fmt.Sprintf("(%v.%v IS NULL)", quotedTableName, scope.Quote(key)))
				}
			}
		}
		return strings.Join(sqls, " AND ")
	case interface{}:
		var sqls []string
		newScope := scope.New(value)

		if len(newScope.Fields()) == 0 {
			scope.Err(fmt.Errorf("invalid query condition: %v", value))
			return
		}
		scopeQuotedTableName := newScope.QuotedTableName()
		for _, field := range newScope.Fields() {
			if !field.IsIgnored && !field.IsBlank {
				sqls = append(sqls, fmt.Sprintf("(%v.%v %s %v)", scopeQuotedTableName, scope.Quote(field.DBName), equalSQL, scope.AddToVars(field.Field.Interface())))
			}
		}
		return strings.Join(sqls, " AND ")
	default:
		scope.Err(fmt.Errorf("invalid query condition: %v", value))
		return
	}

	replacements := []string{}
	args := clause["args"].([]interface{})
	for _, arg := range args {
		var err error
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice: // For where("id in (?)", []int64{1,2})
			if scanner, ok := interface{}(arg).(driver.Valuer); ok {
				arg, err = scanner.Value()
				replacements = append(replacements, scope.AddToVars(arg))
			} else if b, ok := arg.([]byte); ok {
				replacements = append(replacements, scope.AddToVars(b))
			} else if as, ok := arg.([][]interface{}); ok {
				var tempMarks []string
				for _, a := range as {
					var arrayMarks []string
					for _, v := range a {
						arrayMarks = append(arrayMarks, scope.AddToVars(v))
					}

					if len(arrayMarks) > 0 {
						tempMarks = append(tempMarks, fmt.Sprintf("(%v)", strings.Join(arrayMarks, ",")))
					}
				}

				if len(tempMarks) > 0 {
					replacements = append(replacements, strings.Join(tempMarks, ","))
				}
			} else if values := reflect.ValueOf(arg); values.Len() > 0 {
				var tempMarks []string
				for i := 0; i < values.Len(); i++ {
					tempMarks = append(tempMarks, scope.AddToVars(values.Index(i).Interface()))
				}
				replacements = append(replacements, strings.Join(tempMarks, ","))
			} else {
				replacements = append(replacements, scope.AddToVars(Expr("NULL")))
			}
		default:
			if valuer, ok := interface{}(arg).(driver.Valuer); ok {
				arg, err = valuer.Value()
			}

			replacements = append(replacements, scope.AddToVars(arg))
		}

		if err != nil {
			scope.Err(err)
		}
	}

	buff := bytes.NewBuffer([]byte{})
	i := 0
	for _, s := range str {
		if s == '?' && len(replacements) > i {
			buff.WriteString(replacements[i])
			i++
		} else {
			buff.WriteRune(s)
		}
	}

	str = buff.String()

	return
}

func (scope *Scope) buildSelectQuery(clause map[string]interface{}) (str string) {
	switch value := clause["query"].(type) {
	case string:
		str = value
	case []string:
		str = strings.Join(value, ", ")
	}

	args := clause["args"].([]interface{})
	replacements := []string{}
	for _, arg := range args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice:
			values := reflect.ValueOf(arg)
			var tempMarks []string
			for i := 0; i < values.Len(); i++ {
				tempMarks = append(tempMarks, scope.AddToVars(values.Index(i).Interface()))
			}
			replacements = append(replacements, strings.Join(tempMarks, ","))
		default:
			if valuer, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = valuer.Value()
			}
			replacements = append(replacements, scope.AddToVars(arg))
		}
	}

	buff := bytes.NewBuffer([]byte{})
	i := 0
	for pos, char := range str {
		if str[pos] == '?' {
			buff.WriteString(replacements[i])
			i++
		} else {
			buff.WriteRune(char)
		}
	}

	str = buff.String()

	return
}

func (scope *Scope) whereSQL() (sql string) {
	var (
		quotedTableName                                = scope.QuotedTableName()
		deletedAtField, hasDeletedAtField              = scope.FieldByName("DeletedAt")
		primaryConditions, andConditions, orConditions []string
	)

	if !scope.Search.Unscoped && hasDeletedAtField {
		sql := fmt.Sprintf("%v.%v IS NULL", quotedTableName, scope.Quote(deletedAtField.DBName))
		primaryConditions = append(primaryConditions, sql)
	}

	if !scope.PrimaryKeyZero() {
		for _, field := range scope.PrimaryFields() {
			sql := fmt.Sprintf("%v.%v = %v", quotedTableName, scope.Quote(field.DBName), scope.AddToVars(field.Field.Interface()))
			primaryConditions = append(primaryConditions, sql)
		}
	}

	for _, clause := range scope.Search.whereConditions {
		if sql := scope.buildCondition(clause, true); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	for _, clause := range scope.Search.orConditions {
		if sql := scope.buildCondition(clause, true); sql != "" {
			orConditions = append(orConditions, sql)
		}
	}

	for _, clause := range scope.Search.notConditions {
		if sql := scope.buildCondition(clause, false); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	orSQL := strings.Join(orConditions, " OR ")
	combinedSQL := strings.Join(andConditions, " AND ")
	if len(combinedSQL) > 0 {
		if len(orSQL) > 0 {
			combinedSQL = combinedSQL + " OR " + orSQL
		}
	} else {
		combinedSQL = orSQL
	}

	if len(primaryConditions) > 0 {
		sql = "WHERE " + strings.Join(primaryConditions, " AND ")
		if len(combinedSQL) > 0 {
			sql = sql + " AND (" + combinedSQL + ")"
		}
	} else if len(combinedSQL) > 0 {
		sql = "WHERE " + combinedSQL
	}
	return
}

func (scope *Scope) selectSQL() string {
	if len(scope.Search.selects) == 0 {
		if len(scope.Search.joinConditions) > 0 {
			return fmt.Sprintf("%v.*", scope.QuotedTableName())
		}
		return "*"
	}
	return scope.buildSelectQuery(scope.Search.selects)
}

func (scope *Scope) orderSQL() string {
	if len(scope.Search.orders) == 0 || scope.Search.ignoreOrderQuery {
		return ""
	}

	var orders []string
	for _, order := range scope.Search.orders {
		if str, ok := order.(string); ok {
			orders = append(orders, scope.quoteIfPossible(str))
		} else if expr, ok := order.(*expr); ok {
			exp := expr.expr
			for _, arg := range expr.args {
				exp = strings.Replace(exp, "?", scope.AddToVars(arg), 1)
			}
			orders = append(orders, exp)
		}
	}
	return " ORDER BY " + strings.Join(orders, ",")
}

func (scope *Scope) limitAndOffsetSQL() string {
	return scope.Dialect().LimitAndOffsetSQL(scope.Search.limit, scope.Search.offset)
}

func (scope *Scope) groupSQL() string {
	if len(scope.Search.group) == 0 {
		return ""
	}
	return " GROUP BY " + scope.Search.group
}

func (scope *Scope) havingSQL() string {
	if len(scope.Search.havingConditions) == 0 {
		return ""
	}

	var andConditions []string
	for _, clause := range scope.Search.havingConditions {
		if sql := scope.buildCondition(clause, true); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	combinedSQL := strings.Join(andConditions, " AND ")
	if len(combinedSQL) == 0 {
		return ""
	}

	return " HAVING " + combinedSQL
}

func (scope *Scope) joinsSQL() string {
	var joinConditions []string
	for _, clause := range scope.Search.joinConditions {
		if sql := scope.buildCondition(clause, true); sql != "" {
			joinConditions = append(joinConditions, strings.TrimSuffix(strings.TrimPrefix(sql, "("), ")"))
		}
	}

	return strings.Join(joinConditions, " ") + " "
}

func (scope *Scope) prepareQuerySQL() {
	if scope.Search.raw {
		scope.Raw(scope.CombinedConditionSql())
	} else {
		scope.Raw(fmt.Sprintf("SELECT %v FROM %v %v", scope.selectSQL(), scope.QuotedTableName(), scope.CombinedConditionSql()))
	}
	return
}

func (scope *Scope) inlineCondition(values ...interface{}) *Scope {
	if len(values) > 0 {
		scope.Search.Where(values[0], values[1:]...)
	}
	return scope
}

func (scope *Scope) callCallbacks(funcs []*func(s *Scope)) *Scope {
	defer func() {
		if err := recover(); err != nil {
			if db, ok := scope.db.db.(sqlTx); ok {
				db.Rollback()
			}
			panic(err)
		}
	}()
	for _, f := range funcs {
		(*f)(scope)
		if scope.skipLeft {
			break
		}
	}
	return scope
}

func convertInterfaceToMap(values interface{}, withIgnoredField bool, db *DB) map[string]interface{} {
	var attrs = map[string]interface{}{}

	switch value := values.(type) {
	case map[string]interface{}:
		return value
	case []interface{}:
		for _, v := range value {
			for key, value := range convertInterfaceToMap(v, withIgnoredField, db) {
				attrs[key] = value
			}
		}
	case interface{}:
		reflectValue := reflect.ValueOf(values)

		switch reflectValue.Kind() {
		case reflect.Map:
			for _, key := range reflectValue.MapKeys() {
				attrs[ToColumnName(key.Interface().(string))] = reflectValue.MapIndex(key).Interface()
			}
		default:
			for _, field := range (&Scope{Value: values, db: db}).Fields() {
				if !field.IsBlank && (withIgnoredField || !field.IsIgnored) {
					attrs[field.DBName] = field.Field.Interface()
				}
			}
		}
	}
	return attrs
}

func (scope *Scope) updatedAttrsWithValues(value interface{}) (results map[string]interface{}, hasUpdate bool) {
	if scope.IndirectValue().Kind() != reflect.Struct {
		return convertInterfaceToMap(value, false, scope.db), true
	}

	results = map[string]interface{}{}

	for key, value := range convertInterfaceToMap(value, true, scope.db) {
		if field, ok := scope.FieldByName(key); ok && scope.changeableField(field) {
			if _, ok := value.(*expr); ok {
				hasUpdate = true
				results[field.DBName] = value
			} else {
				err := field.Set(value)
				if field.IsNormal && !field.IsIgnored {
					hasUpdate = true
					if err == ErrUnaddressable {
						results[field.DBName] = value
					} else {
						results[field.DBName] = field.Field.Interface()
					}
				}
			}
		}
	}
	return
}

func (scope *Scope) row() *sql.Row {
	defer scope.trace(scope.db.nowFunc())

	result := &RowQueryResult{}
	scope.InstanceSet("row_query_result", result)
	scope.callCallbacks(scope.db.parent.callbacks.rowQueries)

	return result.Row
}

func (scope *Scope) rows() (*sql.Rows, error) {
	defer scope.trace(scope.db.nowFunc())

	result := &RowsQueryResult{}
	scope.InstanceSet("row_query_result", result)
	scope.callCallbacks(scope.db.parent.callbacks.rowQueries)

	return result.Rows, result.Error
}

func (scope *Scope) initialize() *Scope {
	for _, clause := range scope.Search.whereConditions {
		scope.updatedAttrsWithValues(clause["query"])
	}
	scope.updatedAttrsWithValues(scope.Search.initAttrs)
	scope.updatedAttrsWithValues(scope.Search.assignAttrs)
	return scope
}

func (scope *Scope) isQueryForColumn(query interface{}, column string) bool {
	queryStr := strings.ToLower(fmt.Sprint(query))
	if queryStr == column {
		return true
	}

	if strings.HasSuffix(queryStr, "as "+column) {
		return true
	}

	if strings.HasSuffix(queryStr, "as "+scope.Quote(column)) {
		return true
	}

	return false
}

func (scope *Scope) pluck(column string, value interface{}) *Scope {
	dest := reflect.Indirect(reflect.ValueOf(value))
	if dest.Kind() != reflect.Slice {
		scope.Err(fmt.Errorf("results should be a slice, not %s", dest.Kind()))
		return scope
	}

	if dest.Len() > 0 {
		dest.Set(reflect.Zero(dest.Type()))
	}

	if query, ok := scope.Search.selects["query"]; !ok || !scope.isQueryForColumn(query, column) {
		scope.Search.Select(column)
	}

	rows, err := scope.rows()
	if scope.Err(err) == nil {
		defer rows.Close()
		for rows.Next() {
			elem := reflect.New(dest.Type().Elem()).Interface()
			scope.Err(rows.Scan(elem))
			dest.Set(reflect.Append(dest, reflect.ValueOf(elem).Elem()))
		}

		if err := rows.Err(); err != nil {
			scope.Err(err)
		}
	}
	return scope
}

func (scope *Scope) count(value interface{}) *Scope {
	if query, ok := scope.Search.selects["query"]; !ok || !countingQueryRegexp.MatchString(fmt.Sprint(query)) {
		if len(scope.Search.group) != 0 {
			if len(scope.Search.havingConditions) != 0 {
				scope.prepareQuerySQL()
				scope.Search = &search{}
				scope.Search.Select("count(*)")
				scope.Search.Table(fmt.Sprintf("( %s ) AS count_table", scope.SQL))
			} else {
				scope.Search.Select("count(*) FROM ( SELECT count(*) as name ")
				scope.Search.group += " ) AS count_table"
			}
		} else {
			scope.Search.Select("count(*)")
		}
	}
	scope.Search.ignoreOrderQuery = true
	scope.Err(scope.row().Scan(value))
	return scope
}

func (scope *Scope) typeName() string {
	typ := scope.IndirectValue().Type()

	for typ.Kind() == reflect.Slice || typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return typ.Name()
}

// trace print sql log
func (scope *Scope) trace(t time.Time) {
	if len(scope.SQL) > 0 {
		scope.db.slog(scope.SQL, t, scope.SQLVars...)
	}
}

func (scope *Scope) changeableField(field *Field) bool {
	if selectAttrs := scope.SelectAttrs(); len(selectAttrs) > 0 {
		for _, attr := range selectAttrs {
			if field.Name == attr || field.DBName == attr {
				return true
			}
		}
		return false
	}

	for _, attr := range scope.OmitAttrs() {
		if field.Name == attr || field.DBName == attr {
			return false
		}
	}

	return true
}

func (scope *Scope) related(value interface{}, foreignKeys ...string) *Scope {
	toScope := scope.db.NewScope(value)
	tx := scope.db.Set("gorm:association:source", scope.Value)

	for _, foreignKey := range append(foreignKeys, toScope.typeName()+"Id", scope.typeName()+"Id") {
		fromField, _ := scope.FieldByName(foreignKey)
		toField, _ := toScope.FieldByName(foreignKey)

		if fromField != nil {
			if relationship := fromField.Relationship; relationship != nil {
				if relationship.Kind == "many_to_many" {
					joinTableHandler := relationship.JoinTableHandler
					scope.Err(joinTableHandler.JoinWith(joinTableHandler, tx, scope.Value).Find(value).Error)
				} else if relationship.Kind == "belongs_to" {
					for idx, foreignKey := range relationship.ForeignDBNames {
						if field, ok := scope.FieldByName(foreignKey); ok {
							tx = tx.Where(fmt.Sprintf("%v = ?", scope.Quote(relationship.AssociationForeignDBNames[idx])), field.Field.Interface())
						}
					}
					scope.Err(tx.Find(value).Error)
				} else if relationship.Kind == "has_many" || relationship.Kind == "has_one" {
					for idx, foreignKey := range relationship.ForeignDBNames {
						if field, ok := scope.FieldByName(relationship.AssociationForeignDBNames[idx]); ok {
							tx = tx.Where(fmt.Sprintf("%v = ?", scope.Quote(foreignKey)), field.Field.Interface())
						}
					}

					if relationship.PolymorphicType != "" {
						tx = tx.Where(fmt.Sprintf("%v = ?", scope.Quote(relationship.PolymorphicDBName)), relationship.PolymorphicValue)
					}
					scope.Err(tx.Find(value).Error)
				}
			} else {
				sql := fmt.Sprintf("%v = ?", scope.Quote(toScope.PrimaryKey()))
				scope.Err(tx.Where(sql, fromField.Field.Interface()).Find(value).Error)
			}
			return scope
		} else if toField != nil {
			sql := fmt.Sprintf("%v = ?", scope.Quote(toField.DBName))
			scope.Err(tx.Where(sql, scope.PrimaryKeyValue()).Find(value).Error)
			return scope
		}
	}

	scope.Err(fmt.Errorf("invalid association %v", foreignKeys))
	return scope
}

// getTableOptions return the table options string or an empty string if the table options does not exist
func (scope *Scope) getTableOptions() string {
	tableOptions, ok := scope.Get("gorm:table_options")
	if !ok {
		return ""
	}
	return " " + tableOptions.(string)
}

func (scope *Scope) createJoinTable(field *StructField) {
	if relationship := field.Relationship; relationship != nil && relationship.JoinTableHandler != nil {
		joinTableHandler := relationship.JoinTableHandler
		joinTable := joinTableHandler.Table(scope.db)
		if !scope.Dialect().HasTable(joinTable) {
			toScope := &Scope{Value: reflect.New(field.Struct.Type).Interface()}

			var sqlTypes, primaryKeys []string
			for idx, fieldName := range relationship.ForeignFieldNames {
				if field, ok := scope.FieldByName(fieldName); ok {
					foreignKeyStruct := field.clone()
					foreignKeyStruct.IsPrimaryKey = false
					foreignKeyStruct.TagSettingsSet("IS_JOINTABLE_FOREIGNKEY", "true")
					foreignKeyStruct.TagSettingsDelete("AUTO_INCREMENT")
					sqlTypes = append(sqlTypes, scope.Quote(relationship.ForeignDBNames[idx])+" "+scope.Dialect().DataTypeOf(foreignKeyStruct))
					primaryKeys = append(primaryKeys, scope.Quote(relationship.ForeignDBNames[idx]))
				}
			}

			for idx, fieldName := range relationship.AssociationForeignFieldNames {
				if field, ok := toScope.FieldByName(fieldName); ok {
					foreignKeyStruct := field.clone()
					foreignKeyStruct.IsPrimaryKey = false
					foreignKeyStruct.TagSettingsSet("IS_JOINTABLE_FOREIGNKEY", "true")
					foreignKeyStruct.TagSettingsDelete("AUTO_INCREMENT")
					sqlTypes = append(sqlTypes, scope.Quote(relationship.AssociationForeignDBNames[idx])+" "+scope.Dialect().DataTypeOf(foreignKeyStruct))
					primaryKeys = append(primaryKeys, scope.Quote(relationship.AssociationForeignDBNames[idx]))
				}
			}

			scope.Err(scope.NewDB().Exec(fmt.Sprintf("CREATE TABLE %v (%v, PRIMARY KEY (%v))%s", scope.Quote(joinTable), strings.Join(sqlTypes, ","), strings.Join(primaryKeys, ","), scope.getTableOptions())).Error)
		}
		scope.NewDB().Table(joinTable).AutoMigrate(joinTableHandler)
	}
}

func (scope *Scope) createTable() *Scope {
	var tags []string
	var primaryKeys []string
	var primaryKeyInColumnType = false
	for _, field := range scope.GetModelStruct().StructFields {
		if field.IsNormal {
			sqlTag := scope.Dialect().DataTypeOf(field)

			// Check if the primary key constraint was specified as
			// part of the column type. If so, we can only support
			// one column as the primary key.
			if strings.Contains(strings.ToLower(sqlTag), "primary key") {
				primaryKeyInColumnType = true
			}

			tags = append(tags, scope.Quote(field.DBName)+" "+sqlTag)
		}

		if field.IsPrimaryKey {
			primaryKeys = append(primaryKeys, scope.Quote(field.DBName))
		}
		scope.createJoinTable(field)
	}

	var primaryKeyStr string
	if len(primaryKeys) > 0 && !primaryKeyInColumnType {
		primaryKeyStr = fmt.Sprintf(", PRIMARY KEY (%v)", strings.Join(primaryKeys, ","))
	}

	scope.Raw(fmt.Sprintf("CREATE TABLE %v (%v %v)%s", scope.QuotedTableName(), strings.Join(tags, ","), primaryKeyStr, scope.getTableOptions())).Exec()

	scope.autoIndex()
	return scope
}

func (scope *Scope) dropTable() *Scope {
	scope.Raw(fmt.Sprintf("DROP TABLE %v", scope.QuotedTableName())).Exec()
	return scope
}

func (scope *Scope) modifyColumn(column string, typ string) {
	scope.db.AddError(scope.Dialect().ModifyColumn(scope.QuotedTableName(), scope.Quote(column), typ))
}

func (scope *Scope) dropColumn(column string) {
	scope.Raw(fmt.Sprintf("ALTER TABLE %v DROP COLUMN %v", scope.QuotedTableName(), scope.Quote(column))).Exec()
}

func (scope *Scope) addIndex(unique bool, indexName string, column ...string) {
	if scope.Dialect().HasIndex(scope.TableName(), indexName) {
		return
	}

	var columns []string
	for _, name := range column {
		columns = append(columns, scope.quoteIfPossible(name))
	}

	sqlCreate := "CREATE INDEX"
	if unique {
		sqlCreate = "CREATE UNIQUE INDEX"
	}

	scope.Raw(fmt.Sprintf("%s %v ON %v(%v) %v", sqlCreate, indexName, scope.QuotedTableName(), strings.Join(columns, ", "), scope.whereSQL())).Exec()
}

func (scope *Scope) addForeignKey(field string, dest string, onDelete string, onUpdate string) {
	// Compatible with old generated key
	keyName := scope.Dialect().BuildKeyName(scope.TableName(), field, dest, "foreign")

	if scope.Dialect().HasForeignKey(scope.TableName(), keyName) {
		return
	}
	var query = `ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s ON DELETE %s ON UPDATE %s;`
	scope.Raw(fmt.Sprintf(query, scope.QuotedTableName(), scope.quoteIfPossible(keyName), scope.quoteIfPossible(field), dest, onDelete, onUpdate)).Exec()
}

func (scope *Scope) removeForeignKey(field string, dest string) {
	keyName := scope.Dialect().BuildKeyName(scope.TableName(), field, dest, "foreign")
	if !scope.Dialect().HasForeignKey(scope.TableName(), keyName) {
		return
	}
	var mysql mysql
	var query string
	if scope.Dialect().GetName() == mysql.GetName() {
		query = `ALTER TABLE %s DROP FOREIGN KEY %s;`
	} else {
		query = `ALTER TABLE %s DROP CONSTRAINT %s;`
	}

	scope.Raw(fmt.Sprintf(query, scope.QuotedTableName(), scope.quoteIfPossible(keyName))).Exec()
}

func (scope *Scope) removeIndex(indexName string) {
	scope.Dialect().RemoveIndex(scope.TableName(), indexName)
}

func (scope *Scope) autoMigrate() *Scope {
	tableName := scope.TableName()
	quotedTableName := scope.QuotedTableName()

	if !scope.Dialect().HasTable(tableName) {
		scope.createTable()
	} else {
		for _, field := range scope.GetModelStruct().StructFields {
			if !scope.Dialect().HasColumn(tableName, field.DBName) {
				if field.IsNormal {
					sqlTag := scope.Dialect().DataTypeOf(field)
					scope.Raw(fmt.Sprintf("ALTER TABLE %v ADD %v %v;", quotedTableName, scope.Quote(field.DBName), sqlTag)).Exec()
				}
			}
			scope.createJoinTable(field)
		}
		scope.autoIndex()
	}
	return scope
}

func (scope *Scope) autoIndex() *Scope {
	var indexes = map[string][]string{}
	var uniqueIndexes = map[string][]string{}

	for _, field := range scope.GetStructFields() {
		if name, ok := field.TagSettingsGet("INDEX"); ok {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "INDEX" || name == "" {
					name = scope.Dialect().BuildKeyName("idx", scope.TableName(), field.DBName)
				}
				name, column := scope.Dialect().NormalizeIndexAndColumn(name, field.DBName)
				indexes[name] = append(indexes[name], column)
			}
		}

		if name, ok := field.TagSettingsGet("UNIQUE_INDEX"); ok {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "UNIQUE_INDEX" || name == "" {
					name = scope.Dialect().BuildKeyName("uix", scope.TableName(), field.DBName)
				}
				name, column := scope.Dialect().NormalizeIndexAndColumn(name, field.DBName)
				uniqueIndexes[name] = append(uniqueIndexes[name], column)
			}
		}
	}

	for name, columns := range indexes {
		if db := scope.NewDB().Table(scope.TableName()).Model(scope.Value).AddIndex(name, columns...); db.Error != nil {
			scope.db.AddError(db.Error)
		}
	}

	for name, columns := range uniqueIndexes {
		if db := scope.NewDB().Table(scope.TableName()).Model(scope.Value).AddUniqueIndex(name, columns...); db.Error != nil {
			scope.db.AddError(db.Error)
		}
	}

	return scope
}

func (scope *Scope) getColumnAsArray(columns []string, values ...interface{}) (results [][]interface{}) {
	resultMap := make(map[string][]interface{})
	for _, value := range values {
		indirectValue := indirect(reflect.ValueOf(value))

		switch indirectValue.Kind() {
		case reflect.Slice:
			for i := 0; i < indirectValue.Len(); i++ {
				var result []interface{}
				var object = indirect(indirectValue.Index(i))
				var hasValue = false
				for _, column := range columns {
					field := object.FieldByName(column)
					if hasValue || !isBlank(field) {
						hasValue = true
					}
					result = append(result, field.Interface())
				}

				if hasValue {
					h := fmt.Sprint(result...)
					if _, exist := resultMap[h]; !exist {
						resultMap[h] = result
					}
				}
			}
		case reflect.Struct:
			var result []interface{}
			var hasValue = false
			for _, column := range columns {
				field := indirectValue.FieldByName(column)
				if hasValue || !isBlank(field) {
					hasValue = true
				}
				result = append(result, field.Interface())
			}

			if hasValue {
				h := fmt.Sprint(result...)
				if _, exist := resultMap[h]; !exist {
					resultMap[h] = result
				}
			}
		}
	}
	for _, v := range resultMap {
		results = append(results, v)
	}
	return
}

func (scope *Scope) getColumnAsScope(column string) *Scope {
	indirectScopeValue := scope.IndirectValue()

	switch indirectScopeValue.Kind() {
	case reflect.Slice:
		if fieldStruct, ok := scope.GetModelStruct().ModelType.FieldByName(column); ok {
			fieldType := fieldStruct.Type
			if fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			resultsMap := map[interface{}]bool{}
			results := reflect.New(reflect.SliceOf(reflect.PtrTo(fieldType))).Elem()

			for i := 0; i < indirectScopeValue.Len(); i++ {
				result := indirect(indirect(indirectScopeValue.Index(i)).FieldByName(column))

				if result.Kind() == reflect.Slice {
					for j := 0; j < result.Len(); j++ {
						if elem := result.Index(j); elem.CanAddr() && resultsMap[elem.Addr()] != true {
							resultsMap[elem.Addr()] = true
							results = reflect.Append(results, elem.Addr())
						}
					}
				} else if result.CanAddr() && resultsMap[result.Addr()] != true {
					resultsMap[result.Addr()] = true
					results = reflect.Append(results, result.Addr())
				}
			}
			return scope.New(results.Interface())
		}
	case reflect.Struct:
		if field := indirectScopeValue.FieldByName(column); field.CanAddr() {
			return scope.New(field.Addr().Interface())
		}
	}
	return nil
}

func (scope *Scope) hasConditions() bool {
	return !scope.PrimaryKeyZero() ||
		len(scope.Search.whereConditions) > 0 ||
		len(scope.Search.orConditions) > 0 ||
		len(scope.Search.notConditions) > 0
}
