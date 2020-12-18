package gorm

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// preloadCallback used to preload associations
func preloadCallback(scope *Scope) {
	if _, skip := scope.InstanceGet("gorm:skip_query_callback"); skip {
		return
	}

	if ap, ok := scope.Get("gorm:auto_preload"); ok {
		// If gorm:auto_preload IS NOT a bool then auto preload.
		// Else if it IS a bool, use the value
		if apb, ok := ap.(bool); !ok {
			autoPreload(scope)
		} else if apb {
			autoPreload(scope)
		}
	}

	if scope.Search.preload == nil || scope.HasError() {
		return
	}

	var (
		preloadedMap = map[string]bool{}
		fields       = scope.Fields()
	)

	for _, preload := range scope.Search.preload {
		var (
			preloadFields = strings.Split(preload.schema, ".")
			currentScope  = scope
			currentFields = fields
		)

		for idx, preloadField := range preloadFields {
			var currentPreloadConditions []interface{}

			if currentScope == nil {
				continue
			}

			// if not preloaded
			if preloadKey := strings.Join(preloadFields[:idx+1], "."); !preloadedMap[preloadKey] {

				// assign search conditions to last preload
				if idx == len(preloadFields)-1 {
					currentPreloadConditions = preload.conditions
				}

				for _, field := range currentFields {
					if field.Name != preloadField || field.Relationship == nil {
						continue
					}

					switch field.Relationship.Kind {
					case "has_one":
						currentScope.handleHasOnePreload(field, currentPreloadConditions)
					case "has_many":
						currentScope.handleHasManyPreload(field, currentPreloadConditions)
					case "belongs_to":
						currentScope.handleBelongsToPreload(field, currentPreloadConditions)
					case "many_to_many":
						currentScope.handleManyToManyPreload(field, currentPreloadConditions)
					default:
						scope.Err(errors.New("unsupported relation"))
					}

					preloadedMap[preloadKey] = true
					break
				}

				if !preloadedMap[preloadKey] {
					scope.Err(fmt.Errorf("can't preload field %s for %s", preloadField, currentScope.GetModelStruct().ModelType))
					return
				}
			}

			// preload next level
			if idx < len(preloadFields)-1 {
				currentScope = currentScope.getColumnAsScope(preloadField)
				if currentScope != nil {
					currentFields = currentScope.Fields()
				}
			}
		}
	}
}

func autoPreload(scope *Scope) {
	for _, field := range scope.Fields() {
		if field.Relationship == nil {
			continue
		}

		if val, ok := field.TagSettingsGet("PRELOAD"); ok {
			if preload, err := strconv.ParseBool(val); err != nil {
				scope.Err(errors.New("invalid preload option"))
				return
			} else if !preload {
				continue
			}
		}

		scope.Search.Preload(field.Name)
	}
}

func (scope *Scope) generatePreloadDBWithConditions(conditions []interface{}) (*DB, []interface{}) {
	var (
		preloadDB         = scope.NewDB()
		preloadConditions []interface{}
	)

	for _, condition := range conditions {
		if scopes, ok := condition.(func(*DB) *DB); ok {
			preloadDB = scopes(preloadDB)
		} else {
			preloadConditions = append(preloadConditions, condition)
		}
	}

	return preloadDB, preloadConditions
}

// handleHasOnePreload used to preload has one associations
func (scope *Scope) handleHasOnePreload(field *Field, conditions []interface{}) {
	relation := field.Relationship

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.AssociationForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// find relations
	query := fmt.Sprintf("%v IN (%v)", toQueryCondition(scope, relation.ForeignDBNames), toQueryMarks(primaryKeys))
	values := toQueryValues(primaryKeys)
	if relation.PolymorphicType != "" {
		query += fmt.Sprintf(" AND %v = ?", scope.Quote(relation.PolymorphicDBName))
		values = append(values, relation.PolymorphicValue)
	}

	results := makeSlice(field.Struct.Type)
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		resultsValue       = indirect(reflect.ValueOf(results))
		indirectScopeValue = scope.IndirectValue()
	)

	if indirectScopeValue.Kind() == reflect.Slice {
		foreignValuesToResults := make(map[string]reflect.Value)
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			foreignValues := toString(getValueFromFields(result, relation.ForeignFieldNames))
			foreignValuesToResults[foreignValues] = result
		}
		for j := 0; j < indirectScopeValue.Len(); j++ {
			indirectValue := indirect(indirectScopeValue.Index(j))
			valueString := toString(getValueFromFields(indirectValue, relation.AssociationForeignFieldNames))
			if result, found := foreignValuesToResults[valueString]; found {
				indirectValue.FieldByName(field.Name).Set(result)
			}
		}
	} else {
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			scope.Err(field.Set(result))
		}
	}
}

// handleHasManyPreload used to preload has many associations
func (scope *Scope) handleHasManyPreload(field *Field, conditions []interface{}) {
	relation := field.Relationship

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.AssociationForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// find relations
	query := fmt.Sprintf("%v IN (%v)", toQueryCondition(scope, relation.ForeignDBNames), toQueryMarks(primaryKeys))
	values := toQueryValues(primaryKeys)
	if relation.PolymorphicType != "" {
		query += fmt.Sprintf(" AND %v = ?", scope.Quote(relation.PolymorphicDBName))
		values = append(values, relation.PolymorphicValue)
	}

	results := makeSlice(field.Struct.Type)
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		resultsValue       = indirect(reflect.ValueOf(results))
		indirectScopeValue = scope.IndirectValue()
	)

	if indirectScopeValue.Kind() == reflect.Slice {
		preloadMap := make(map[string][]reflect.Value)
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			foreignValues := getValueFromFields(result, relation.ForeignFieldNames)
			preloadMap[toString(foreignValues)] = append(preloadMap[toString(foreignValues)], result)
		}

		for j := 0; j < indirectScopeValue.Len(); j++ {
			object := indirect(indirectScopeValue.Index(j))
			objectRealValue := getValueFromFields(object, relation.AssociationForeignFieldNames)
			f := object.FieldByName(field.Name)
			if results, ok := preloadMap[toString(objectRealValue)]; ok {
				f.Set(reflect.Append(f, results...))
			} else {
				f.Set(reflect.MakeSlice(f.Type(), 0, 0))
			}
		}
	} else {
		scope.Err(field.Set(resultsValue))
	}
}

// handleBelongsToPreload used to preload belongs to associations
func (scope *Scope) handleBelongsToPreload(field *Field, conditions []interface{}) {
	relation := field.Relationship

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.ForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// find relations
	results := makeSlice(field.Struct.Type)
	scope.Err(preloadDB.Where(fmt.Sprintf("%v IN (%v)", toQueryCondition(scope, relation.AssociationForeignDBNames), toQueryMarks(primaryKeys)), toQueryValues(primaryKeys)...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		resultsValue       = indirect(reflect.ValueOf(results))
		indirectScopeValue = scope.IndirectValue()
	)

	foreignFieldToObjects := make(map[string][]*reflect.Value)
	if indirectScopeValue.Kind() == reflect.Slice {
		for j := 0; j < indirectScopeValue.Len(); j++ {
			object := indirect(indirectScopeValue.Index(j))
			valueString := toString(getValueFromFields(object, relation.ForeignFieldNames))
			foreignFieldToObjects[valueString] = append(foreignFieldToObjects[valueString], &object)
		}
	}

	for i := 0; i < resultsValue.Len(); i++ {
		result := resultsValue.Index(i)
		if indirectScopeValue.Kind() == reflect.Slice {
			valueString := toString(getValueFromFields(result, relation.AssociationForeignFieldNames))
			if objects, found := foreignFieldToObjects[valueString]; found {
				for _, object := range objects {
					object.FieldByName(field.Name).Set(result)
				}
			}
		} else {
			scope.Err(field.Set(result))
		}
	}
}

// handleManyToManyPreload used to preload many to many associations
func (scope *Scope) handleManyToManyPreload(field *Field, conditions []interface{}) {
	var (
		relation         = field.Relationship
		joinTableHandler = relation.JoinTableHandler
		fieldType        = field.Struct.Type.Elem()
		foreignKeyValue  interface{}
		foreignKeyType   = reflect.ValueOf(&foreignKeyValue).Type()
		linkHash         = map[string][]reflect.Value{}
		isPtr            bool
	)

	if fieldType.Kind() == reflect.Ptr {
		isPtr = true
		fieldType = fieldType.Elem()
	}

	var sourceKeys = []string{}
	for _, key := range joinTableHandler.SourceForeignKeys() {
		sourceKeys = append(sourceKeys, key.DBName)
	}

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// generate query with join table
	newScope := scope.New(reflect.New(fieldType).Interface())
	preloadDB = preloadDB.Table(newScope.TableName()).Model(newScope.Value)

	if len(preloadDB.search.selects) == 0 {
		preloadDB = preloadDB.Select("*")
	}

	preloadDB = joinTableHandler.JoinWith(joinTableHandler, preloadDB, scope.Value)

	// preload inline conditions
	if len(preloadConditions) > 0 {
		preloadDB = preloadDB.Where(preloadConditions[0], preloadConditions[1:]...)
	}

	rows, err := preloadDB.Rows()

	if scope.Err(err) != nil {
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	for rows.Next() {
		var (
			elem   = reflect.New(fieldType).Elem()
			fields = scope.New(elem.Addr().Interface()).Fields()
		)

		// register foreign keys in join tables
		var joinTableFields []*Field
		for _, sourceKey := range sourceKeys {
			joinTableFields = append(joinTableFields, &Field{StructField: &StructField{DBName: sourceKey, IsNormal: true}, Field: reflect.New(foreignKeyType).Elem()})
		}

		scope.scan(rows, columns, append(fields, joinTableFields...))

		scope.New(elem.Addr().Interface()).
			InstanceSet("gorm:skip_query_callback", true).
			callCallbacks(scope.db.parent.callbacks.queries)

		var foreignKeys = make([]interface{}, len(sourceKeys))
		// generate hashed forkey keys in join table
		for idx, joinTableField := range joinTableFields {
			if !joinTableField.Field.IsNil() {
				foreignKeys[idx] = joinTableField.Field.Elem().Interface()
			}
		}
		hashedSourceKeys := toString(foreignKeys)

		if isPtr {
			linkHash[hashedSourceKeys] = append(linkHash[hashedSourceKeys], elem.Addr())
		} else {
			linkHash[hashedSourceKeys] = append(linkHash[hashedSourceKeys], elem)
		}
	}

	if err := rows.Err(); err != nil {
		scope.Err(err)
	}

	// assign find results
	var (
		indirectScopeValue = scope.IndirectValue()
		fieldsSourceMap    = map[string][]reflect.Value{}
		foreignFieldNames  = []string{}
	)

	for _, dbName := range relation.ForeignFieldNames {
		if field, ok := scope.FieldByName(dbName); ok {
			foreignFieldNames = append(foreignFieldNames, field.Name)
		}
	}

	if indirectScopeValue.Kind() == reflect.Slice {
		for j := 0; j < indirectScopeValue.Len(); j++ {
			object := indirect(indirectScopeValue.Index(j))
			key := toString(getValueFromFields(object, foreignFieldNames))
			fieldsSourceMap[key] = append(fieldsSourceMap[key], object.FieldByName(field.Name))
		}
	} else if indirectScopeValue.IsValid() {
		key := toString(getValueFromFields(indirectScopeValue, foreignFieldNames))
		fieldsSourceMap[key] = append(fieldsSourceMap[key], indirectScopeValue.FieldByName(field.Name))
	}

	for source, fields := range fieldsSourceMap {
		for _, f := range fields {
			//If not 0 this means Value is a pointer and we already added preloaded models to it
			if f.Len() != 0 {
				continue
			}

			v := reflect.MakeSlice(f.Type(), 0, 0)
			if len(linkHash[source]) > 0 {
				v = reflect.Append(f, linkHash[source]...)
			}

			f.Set(v)
		}
	}
}
