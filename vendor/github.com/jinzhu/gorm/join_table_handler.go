package gorm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// JoinTableHandlerInterface is an interface for how to handle many2many relations
type JoinTableHandlerInterface interface {
	// initialize join table handler
	Setup(relationship *Relationship, tableName string, source reflect.Type, destination reflect.Type)
	// Table return join table's table name
	Table(db *DB) string
	// Add create relationship in join table for source and destination
	Add(handler JoinTableHandlerInterface, db *DB, source interface{}, destination interface{}) error
	// Delete delete relationship in join table for sources
	Delete(handler JoinTableHandlerInterface, db *DB, sources ...interface{}) error
	// JoinWith query with `Join` conditions
	JoinWith(handler JoinTableHandlerInterface, db *DB, source interface{}) *DB
	// SourceForeignKeys return source foreign keys
	SourceForeignKeys() []JoinTableForeignKey
	// DestinationForeignKeys return destination foreign keys
	DestinationForeignKeys() []JoinTableForeignKey
}

// JoinTableForeignKey join table foreign key struct
type JoinTableForeignKey struct {
	DBName            string
	AssociationDBName string
}

// JoinTableSource is a struct that contains model type and foreign keys
type JoinTableSource struct {
	ModelType   reflect.Type
	ForeignKeys []JoinTableForeignKey
}

// JoinTableHandler default join table handler
type JoinTableHandler struct {
	TableName   string          `sql:"-"`
	Source      JoinTableSource `sql:"-"`
	Destination JoinTableSource `sql:"-"`
}

// SourceForeignKeys return source foreign keys
func (s *JoinTableHandler) SourceForeignKeys() []JoinTableForeignKey {
	return s.Source.ForeignKeys
}

// DestinationForeignKeys return destination foreign keys
func (s *JoinTableHandler) DestinationForeignKeys() []JoinTableForeignKey {
	return s.Destination.ForeignKeys
}

// Setup initialize a default join table handler
func (s *JoinTableHandler) Setup(relationship *Relationship, tableName string, source reflect.Type, destination reflect.Type) {
	s.TableName = tableName

	s.Source = JoinTableSource{ModelType: source}
	s.Source.ForeignKeys = []JoinTableForeignKey{}
	for idx, dbName := range relationship.ForeignFieldNames {
		s.Source.ForeignKeys = append(s.Source.ForeignKeys, JoinTableForeignKey{
			DBName:            relationship.ForeignDBNames[idx],
			AssociationDBName: dbName,
		})
	}

	s.Destination = JoinTableSource{ModelType: destination}
	s.Destination.ForeignKeys = []JoinTableForeignKey{}
	for idx, dbName := range relationship.AssociationForeignFieldNames {
		s.Destination.ForeignKeys = append(s.Destination.ForeignKeys, JoinTableForeignKey{
			DBName:            relationship.AssociationForeignDBNames[idx],
			AssociationDBName: dbName,
		})
	}
}

// Table return join table's table name
func (s JoinTableHandler) Table(db *DB) string {
	return DefaultTableNameHandler(db, s.TableName)
}

func (s JoinTableHandler) updateConditionMap(conditionMap map[string]interface{}, db *DB, joinTableSources []JoinTableSource, sources ...interface{}) {
	for _, source := range sources {
		scope := db.NewScope(source)
		modelType := scope.GetModelStruct().ModelType

		for _, joinTableSource := range joinTableSources {
			if joinTableSource.ModelType == modelType {
				for _, foreignKey := range joinTableSource.ForeignKeys {
					if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
						conditionMap[foreignKey.DBName] = field.Field.Interface()
					}
				}
				break
			}
		}
	}
}

// Add create relationship in join table for source and destination
func (s JoinTableHandler) Add(handler JoinTableHandlerInterface, db *DB, source interface{}, destination interface{}) error {
	var (
		scope        = db.NewScope("")
		conditionMap = map[string]interface{}{}
	)

	// Update condition map for source
	s.updateConditionMap(conditionMap, db, []JoinTableSource{s.Source}, source)

	// Update condition map for destination
	s.updateConditionMap(conditionMap, db, []JoinTableSource{s.Destination}, destination)

	var assignColumns, binVars, conditions []string
	var values []interface{}
	for key, value := range conditionMap {
		assignColumns = append(assignColumns, scope.Quote(key))
		binVars = append(binVars, `?`)
		conditions = append(conditions, fmt.Sprintf("%v = ?", scope.Quote(key)))
		values = append(values, value)
	}

	for _, value := range values {
		values = append(values, value)
	}

	quotedTable := scope.Quote(handler.Table(db))
	sql := fmt.Sprintf(
		"INSERT INTO %v (%v) SELECT %v %v WHERE NOT EXISTS (SELECT * FROM %v WHERE %v)",
		quotedTable,
		strings.Join(assignColumns, ","),
		strings.Join(binVars, ","),
		scope.Dialect().SelectFromDummyTable(),
		quotedTable,
		strings.Join(conditions, " AND "),
	)

	return db.Exec(sql, values...).Error
}

// Delete delete relationship in join table for sources
func (s JoinTableHandler) Delete(handler JoinTableHandlerInterface, db *DB, sources ...interface{}) error {
	var (
		scope        = db.NewScope(nil)
		conditions   []string
		values       []interface{}
		conditionMap = map[string]interface{}{}
	)

	s.updateConditionMap(conditionMap, db, []JoinTableSource{s.Source, s.Destination}, sources...)

	for key, value := range conditionMap {
		conditions = append(conditions, fmt.Sprintf("%v = ?", scope.Quote(key)))
		values = append(values, value)
	}

	return db.Table(handler.Table(db)).Where(strings.Join(conditions, " AND "), values...).Delete("").Error
}

// JoinWith query with `Join` conditions
func (s JoinTableHandler) JoinWith(handler JoinTableHandlerInterface, db *DB, source interface{}) *DB {
	var (
		scope           = db.NewScope(source)
		tableName       = handler.Table(db)
		quotedTableName = scope.Quote(tableName)
		joinConditions  []string
		values          []interface{}
	)

	if s.Source.ModelType == scope.GetModelStruct().ModelType {
		destinationTableName := db.NewScope(reflect.New(s.Destination.ModelType).Interface()).QuotedTableName()
		for _, foreignKey := range s.Destination.ForeignKeys {
			joinConditions = append(joinConditions, fmt.Sprintf("%v.%v = %v.%v", quotedTableName, scope.Quote(foreignKey.DBName), destinationTableName, scope.Quote(foreignKey.AssociationDBName)))
		}

		var foreignDBNames []string
		var foreignFieldNames []string

		for _, foreignKey := range s.Source.ForeignKeys {
			foreignDBNames = append(foreignDBNames, foreignKey.DBName)
			if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
				foreignFieldNames = append(foreignFieldNames, field.Name)
			}
		}

		foreignFieldValues := scope.getColumnAsArray(foreignFieldNames, scope.Value)

		var condString string
		if len(foreignFieldValues) > 0 {
			var quotedForeignDBNames []string
			for _, dbName := range foreignDBNames {
				quotedForeignDBNames = append(quotedForeignDBNames, tableName+"."+dbName)
			}

			condString = fmt.Sprintf("%v IN (%v)", toQueryCondition(scope, quotedForeignDBNames), toQueryMarks(foreignFieldValues))

			keys := scope.getColumnAsArray(foreignFieldNames, scope.Value)
			values = append(values, toQueryValues(keys))
		} else {
			condString = fmt.Sprintf("1 <> 1")
		}

		return db.Joins(fmt.Sprintf("INNER JOIN %v ON %v", quotedTableName, strings.Join(joinConditions, " AND "))).
			Where(condString, toQueryValues(foreignFieldValues)...)
	}

	db.Error = errors.New("wrong source type for join table handler")
	return db
}
