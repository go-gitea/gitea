package gorm

import (
	"database/sql"
	"errors"
	"go/ast"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/jinzhu/inflection"
)

// DefaultTableNameHandler default table name handler
var DefaultTableNameHandler = func(db *DB, defaultTableName string) string {
	return defaultTableName
}

var modelStructsMap sync.Map

// ModelStruct model definition
type ModelStruct struct {
	PrimaryFields []*StructField
	StructFields  []*StructField
	ModelType     reflect.Type

	defaultTableName string
	l                sync.Mutex
}

// TableName returns model's table name
func (s *ModelStruct) TableName(db *DB) string {
	s.l.Lock()
	defer s.l.Unlock()

	if s.defaultTableName == "" && db != nil && s.ModelType != nil {
		// Set default table name
		if tabler, ok := reflect.New(s.ModelType).Interface().(tabler); ok {
			s.defaultTableName = tabler.TableName()
		} else {
			tableName := ToTableName(s.ModelType.Name())
			db.parent.RLock()
			if db == nil || (db.parent != nil && !db.parent.singularTable) {
				tableName = inflection.Plural(tableName)
			}
			db.parent.RUnlock()
			s.defaultTableName = tableName
		}
	}

	return DefaultTableNameHandler(db, s.defaultTableName)
}

// StructField model field's struct definition
type StructField struct {
	DBName          string
	Name            string
	Names           []string
	IsPrimaryKey    bool
	IsNormal        bool
	IsIgnored       bool
	IsScanner       bool
	HasDefaultValue bool
	Tag             reflect.StructTag
	TagSettings     map[string]string
	Struct          reflect.StructField
	IsForeignKey    bool
	Relationship    *Relationship

	tagSettingsLock sync.RWMutex
}

// TagSettingsSet Sets a tag in the tag settings map
func (sf *StructField) TagSettingsSet(key, val string) {
	sf.tagSettingsLock.Lock()
	defer sf.tagSettingsLock.Unlock()
	sf.TagSettings[key] = val
}

// TagSettingsGet returns a tag from the tag settings
func (sf *StructField) TagSettingsGet(key string) (string, bool) {
	sf.tagSettingsLock.RLock()
	defer sf.tagSettingsLock.RUnlock()
	val, ok := sf.TagSettings[key]
	return val, ok
}

// TagSettingsDelete deletes a tag
func (sf *StructField) TagSettingsDelete(key string) {
	sf.tagSettingsLock.Lock()
	defer sf.tagSettingsLock.Unlock()
	delete(sf.TagSettings, key)
}

func (sf *StructField) clone() *StructField {
	clone := &StructField{
		DBName:          sf.DBName,
		Name:            sf.Name,
		Names:           sf.Names,
		IsPrimaryKey:    sf.IsPrimaryKey,
		IsNormal:        sf.IsNormal,
		IsIgnored:       sf.IsIgnored,
		IsScanner:       sf.IsScanner,
		HasDefaultValue: sf.HasDefaultValue,
		Tag:             sf.Tag,
		TagSettings:     map[string]string{},
		Struct:          sf.Struct,
		IsForeignKey:    sf.IsForeignKey,
	}

	if sf.Relationship != nil {
		relationship := *sf.Relationship
		clone.Relationship = &relationship
	}

	// copy the struct field tagSettings, they should be read-locked while they are copied
	sf.tagSettingsLock.Lock()
	defer sf.tagSettingsLock.Unlock()
	for key, value := range sf.TagSettings {
		clone.TagSettings[key] = value
	}

	return clone
}

// Relationship described the relationship between models
type Relationship struct {
	Kind                         string
	PolymorphicType              string
	PolymorphicDBName            string
	PolymorphicValue             string
	ForeignFieldNames            []string
	ForeignDBNames               []string
	AssociationForeignFieldNames []string
	AssociationForeignDBNames    []string
	JoinTableHandler             JoinTableHandlerInterface
}

func getForeignField(column string, fields []*StructField) *StructField {
	for _, field := range fields {
		if field.Name == column || field.DBName == column || field.DBName == ToColumnName(column) {
			return field
		}
	}
	return nil
}

// GetModelStruct get value's model struct, relationships based on struct and tag definition
func (scope *Scope) GetModelStruct() *ModelStruct {
	var modelStruct ModelStruct
	// Scope value can't be nil
	if scope.Value == nil {
		return &modelStruct
	}

	reflectType := reflect.ValueOf(scope.Value).Type()
	for reflectType.Kind() == reflect.Slice || reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	// Scope value need to be a struct
	if reflectType.Kind() != reflect.Struct {
		return &modelStruct
	}

	// Get Cached model struct
	isSingularTable := false
	if scope.db != nil && scope.db.parent != nil {
		scope.db.parent.RLock()
		isSingularTable = scope.db.parent.singularTable
		scope.db.parent.RUnlock()
	}

	hashKey := struct {
		singularTable bool
		reflectType   reflect.Type
	}{isSingularTable, reflectType}
	if value, ok := modelStructsMap.Load(hashKey); ok && value != nil {
		return value.(*ModelStruct)
	}

	modelStruct.ModelType = reflectType

	// Get all fields
	for i := 0; i < reflectType.NumField(); i++ {
		if fieldStruct := reflectType.Field(i); ast.IsExported(fieldStruct.Name) {
			field := &StructField{
				Struct:      fieldStruct,
				Name:        fieldStruct.Name,
				Names:       []string{fieldStruct.Name},
				Tag:         fieldStruct.Tag,
				TagSettings: parseTagSetting(fieldStruct.Tag),
			}

			// is ignored field
			if _, ok := field.TagSettingsGet("-"); ok {
				field.IsIgnored = true
			} else {
				if _, ok := field.TagSettingsGet("PRIMARY_KEY"); ok {
					field.IsPrimaryKey = true
					modelStruct.PrimaryFields = append(modelStruct.PrimaryFields, field)
				}

				if _, ok := field.TagSettingsGet("DEFAULT"); ok && !field.IsPrimaryKey {
					field.HasDefaultValue = true
				}

				if _, ok := field.TagSettingsGet("AUTO_INCREMENT"); ok && !field.IsPrimaryKey {
					field.HasDefaultValue = true
				}

				indirectType := fieldStruct.Type
				for indirectType.Kind() == reflect.Ptr {
					indirectType = indirectType.Elem()
				}

				fieldValue := reflect.New(indirectType).Interface()
				if _, isScanner := fieldValue.(sql.Scanner); isScanner {
					// is scanner
					field.IsScanner, field.IsNormal = true, true
					if indirectType.Kind() == reflect.Struct {
						for i := 0; i < indirectType.NumField(); i++ {
							for key, value := range parseTagSetting(indirectType.Field(i).Tag) {
								if _, ok := field.TagSettingsGet(key); !ok {
									field.TagSettingsSet(key, value)
								}
							}
						}
					}
				} else if _, isTime := fieldValue.(*time.Time); isTime {
					// is time
					field.IsNormal = true
				} else if _, ok := field.TagSettingsGet("EMBEDDED"); ok || fieldStruct.Anonymous {
					// is embedded struct
					for _, subField := range scope.New(fieldValue).GetModelStruct().StructFields {
						subField = subField.clone()
						subField.Names = append([]string{fieldStruct.Name}, subField.Names...)
						if prefix, ok := field.TagSettingsGet("EMBEDDED_PREFIX"); ok {
							subField.DBName = prefix + subField.DBName
						}

						if subField.IsPrimaryKey {
							if _, ok := subField.TagSettingsGet("PRIMARY_KEY"); ok {
								modelStruct.PrimaryFields = append(modelStruct.PrimaryFields, subField)
							} else {
								subField.IsPrimaryKey = false
							}
						}

						if subField.Relationship != nil && subField.Relationship.JoinTableHandler != nil {
							if joinTableHandler, ok := subField.Relationship.JoinTableHandler.(*JoinTableHandler); ok {
								newJoinTableHandler := &JoinTableHandler{}
								newJoinTableHandler.Setup(subField.Relationship, joinTableHandler.TableName, reflectType, joinTableHandler.Destination.ModelType)
								subField.Relationship.JoinTableHandler = newJoinTableHandler
							}
						}

						modelStruct.StructFields = append(modelStruct.StructFields, subField)
					}
					continue
				} else {
					// build relationships
					switch indirectType.Kind() {
					case reflect.Slice:
						defer func(field *StructField) {
							var (
								relationship           = &Relationship{}
								toScope                = scope.New(reflect.New(field.Struct.Type).Interface())
								foreignKeys            []string
								associationForeignKeys []string
								elemType               = field.Struct.Type
							)

							if foreignKey, _ := field.TagSettingsGet("FOREIGNKEY"); foreignKey != "" {
								foreignKeys = strings.Split(foreignKey, ",")
							}

							if foreignKey, _ := field.TagSettingsGet("ASSOCIATION_FOREIGNKEY"); foreignKey != "" {
								associationForeignKeys = strings.Split(foreignKey, ",")
							} else if foreignKey, _ := field.TagSettingsGet("ASSOCIATIONFOREIGNKEY"); foreignKey != "" {
								associationForeignKeys = strings.Split(foreignKey, ",")
							}

							for elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Ptr {
								elemType = elemType.Elem()
							}

							if elemType.Kind() == reflect.Struct {
								if many2many, _ := field.TagSettingsGet("MANY2MANY"); many2many != "" {
									relationship.Kind = "many_to_many"

									{ // Foreign Keys for Source
										joinTableDBNames := []string{}

										if foreignKey, _ := field.TagSettingsGet("JOINTABLE_FOREIGNKEY"); foreignKey != "" {
											joinTableDBNames = strings.Split(foreignKey, ",")
										}

										// if no foreign keys defined with tag
										if len(foreignKeys) == 0 {
											for _, field := range modelStruct.PrimaryFields {
												foreignKeys = append(foreignKeys, field.DBName)
											}
										}

										for idx, foreignKey := range foreignKeys {
											if foreignField := getForeignField(foreignKey, modelStruct.StructFields); foreignField != nil {
												// source foreign keys (db names)
												relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.DBName)

												// setup join table foreign keys for source
												if len(joinTableDBNames) > idx {
													// if defined join table's foreign key
													relationship.ForeignDBNames = append(relationship.ForeignDBNames, joinTableDBNames[idx])
												} else {
													defaultJointableForeignKey := ToColumnName(reflectType.Name()) + "_" + foreignField.DBName
													relationship.ForeignDBNames = append(relationship.ForeignDBNames, defaultJointableForeignKey)
												}
											}
										}
									}

									{ // Foreign Keys for Association (Destination)
										associationJoinTableDBNames := []string{}

										if foreignKey, _ := field.TagSettingsGet("ASSOCIATION_JOINTABLE_FOREIGNKEY"); foreignKey != "" {
											associationJoinTableDBNames = strings.Split(foreignKey, ",")
										}

										// if no association foreign keys defined with tag
										if len(associationForeignKeys) == 0 {
											for _, field := range toScope.PrimaryFields() {
												associationForeignKeys = append(associationForeignKeys, field.DBName)
											}
										}

										for idx, name := range associationForeignKeys {
											if field, ok := toScope.FieldByName(name); ok {
												// association foreign keys (db names)
												relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, field.DBName)

												// setup join table foreign keys for association
												if len(associationJoinTableDBNames) > idx {
													relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, associationJoinTableDBNames[idx])
												} else {
													// join table foreign keys for association
													joinTableDBName := ToColumnName(elemType.Name()) + "_" + field.DBName
													relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, joinTableDBName)
												}
											}
										}
									}

									joinTableHandler := JoinTableHandler{}
									joinTableHandler.Setup(relationship, many2many, reflectType, elemType)
									relationship.JoinTableHandler = &joinTableHandler
									field.Relationship = relationship
								} else {
									// User has many comments, associationType is User, comment use UserID as foreign key
									var associationType = reflectType.Name()
									var toFields = toScope.GetStructFields()
									relationship.Kind = "has_many"

									if polymorphic, _ := field.TagSettingsGet("POLYMORPHIC"); polymorphic != "" {
										// Dog has many toys, tag polymorphic is Owner, then associationType is Owner
										// Toy use OwnerID, OwnerType ('dogs') as foreign key
										if polymorphicType := getForeignField(polymorphic+"Type", toFields); polymorphicType != nil {
											associationType = polymorphic
											relationship.PolymorphicType = polymorphicType.Name
											relationship.PolymorphicDBName = polymorphicType.DBName
											// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
											if value, ok := field.TagSettingsGet("POLYMORPHIC_VALUE"); ok {
												relationship.PolymorphicValue = value
											} else {
												relationship.PolymorphicValue = scope.TableName()
											}
											polymorphicType.IsForeignKey = true
										}
									}

									// if no foreign keys defined with tag
									if len(foreignKeys) == 0 {
										// if no association foreign keys defined with tag
										if len(associationForeignKeys) == 0 {
											for _, field := range modelStruct.PrimaryFields {
												foreignKeys = append(foreignKeys, associationType+field.Name)
												associationForeignKeys = append(associationForeignKeys, field.Name)
											}
										} else {
											// generate foreign keys from defined association foreign keys
											for _, scopeFieldName := range associationForeignKeys {
												if foreignField := getForeignField(scopeFieldName, modelStruct.StructFields); foreignField != nil {
													foreignKeys = append(foreignKeys, associationType+foreignField.Name)
													associationForeignKeys = append(associationForeignKeys, foreignField.Name)
												}
											}
										}
									} else {
										// generate association foreign keys from foreign keys
										if len(associationForeignKeys) == 0 {
											for _, foreignKey := range foreignKeys {
												if strings.HasPrefix(foreignKey, associationType) {
													associationForeignKey := strings.TrimPrefix(foreignKey, associationType)
													if foreignField := getForeignField(associationForeignKey, modelStruct.StructFields); foreignField != nil {
														associationForeignKeys = append(associationForeignKeys, associationForeignKey)
													}
												}
											}
											if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
												associationForeignKeys = []string{scope.PrimaryKey()}
											}
										} else if len(foreignKeys) != len(associationForeignKeys) {
											scope.Err(errors.New("invalid foreign keys, should have same length"))
											return
										}
									}

									for idx, foreignKey := range foreignKeys {
										if foreignField := getForeignField(foreignKey, toFields); foreignField != nil {
											if associationField := getForeignField(associationForeignKeys[idx], modelStruct.StructFields); associationField != nil {
												// source foreign keys
												foreignField.IsForeignKey = true
												relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, associationField.Name)
												relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, associationField.DBName)

												// association foreign keys
												relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.Name)
												relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
											}
										}
									}

									if len(relationship.ForeignFieldNames) != 0 {
										field.Relationship = relationship
									}
								}
							} else {
								field.IsNormal = true
							}
						}(field)
					case reflect.Struct:
						defer func(field *StructField) {
							var (
								// user has one profile, associationType is User, profile use UserID as foreign key
								// user belongs to profile, associationType is Profile, user use ProfileID as foreign key
								associationType           = reflectType.Name()
								relationship              = &Relationship{}
								toScope                   = scope.New(reflect.New(field.Struct.Type).Interface())
								toFields                  = toScope.GetStructFields()
								tagForeignKeys            []string
								tagAssociationForeignKeys []string
							)

							if foreignKey, _ := field.TagSettingsGet("FOREIGNKEY"); foreignKey != "" {
								tagForeignKeys = strings.Split(foreignKey, ",")
							}

							if foreignKey, _ := field.TagSettingsGet("ASSOCIATION_FOREIGNKEY"); foreignKey != "" {
								tagAssociationForeignKeys = strings.Split(foreignKey, ",")
							} else if foreignKey, _ := field.TagSettingsGet("ASSOCIATIONFOREIGNKEY"); foreignKey != "" {
								tagAssociationForeignKeys = strings.Split(foreignKey, ",")
							}

							if polymorphic, _ := field.TagSettingsGet("POLYMORPHIC"); polymorphic != "" {
								// Cat has one toy, tag polymorphic is Owner, then associationType is Owner
								// Toy use OwnerID, OwnerType ('cats') as foreign key
								if polymorphicType := getForeignField(polymorphic+"Type", toFields); polymorphicType != nil {
									associationType = polymorphic
									relationship.PolymorphicType = polymorphicType.Name
									relationship.PolymorphicDBName = polymorphicType.DBName
									// if Cat has several different types of toys set name for each (instead of default 'cats')
									if value, ok := field.TagSettingsGet("POLYMORPHIC_VALUE"); ok {
										relationship.PolymorphicValue = value
									} else {
										relationship.PolymorphicValue = scope.TableName()
									}
									polymorphicType.IsForeignKey = true
								}
							}

							// Has One
							{
								var foreignKeys = tagForeignKeys
								var associationForeignKeys = tagAssociationForeignKeys
								// if no foreign keys defined with tag
								if len(foreignKeys) == 0 {
									// if no association foreign keys defined with tag
									if len(associationForeignKeys) == 0 {
										for _, primaryField := range modelStruct.PrimaryFields {
											foreignKeys = append(foreignKeys, associationType+primaryField.Name)
											associationForeignKeys = append(associationForeignKeys, primaryField.Name)
										}
									} else {
										// generate foreign keys form association foreign keys
										for _, associationForeignKey := range tagAssociationForeignKeys {
											if foreignField := getForeignField(associationForeignKey, modelStruct.StructFields); foreignField != nil {
												foreignKeys = append(foreignKeys, associationType+foreignField.Name)
												associationForeignKeys = append(associationForeignKeys, foreignField.Name)
											}
										}
									}
								} else {
									// generate association foreign keys from foreign keys
									if len(associationForeignKeys) == 0 {
										for _, foreignKey := range foreignKeys {
											if strings.HasPrefix(foreignKey, associationType) {
												associationForeignKey := strings.TrimPrefix(foreignKey, associationType)
												if foreignField := getForeignField(associationForeignKey, modelStruct.StructFields); foreignField != nil {
													associationForeignKeys = append(associationForeignKeys, associationForeignKey)
												}
											}
										}
										if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
											associationForeignKeys = []string{scope.PrimaryKey()}
										}
									} else if len(foreignKeys) != len(associationForeignKeys) {
										scope.Err(errors.New("invalid foreign keys, should have same length"))
										return
									}
								}

								for idx, foreignKey := range foreignKeys {
									if foreignField := getForeignField(foreignKey, toFields); foreignField != nil {
										if scopeField := getForeignField(associationForeignKeys[idx], modelStruct.StructFields); scopeField != nil {
											foreignField.IsForeignKey = true
											// source foreign keys
											relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, scopeField.Name)
											relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, scopeField.DBName)

											// association foreign keys
											relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.Name)
											relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
										}
									}
								}
							}

							if len(relationship.ForeignFieldNames) != 0 {
								relationship.Kind = "has_one"
								field.Relationship = relationship
							} else {
								var foreignKeys = tagForeignKeys
								var associationForeignKeys = tagAssociationForeignKeys

								if len(foreignKeys) == 0 {
									// generate foreign keys & association foreign keys
									if len(associationForeignKeys) == 0 {
										for _, primaryField := range toScope.PrimaryFields() {
											foreignKeys = append(foreignKeys, field.Name+primaryField.Name)
											associationForeignKeys = append(associationForeignKeys, primaryField.Name)
										}
									} else {
										// generate foreign keys with association foreign keys
										for _, associationForeignKey := range associationForeignKeys {
											if foreignField := getForeignField(associationForeignKey, toFields); foreignField != nil {
												foreignKeys = append(foreignKeys, field.Name+foreignField.Name)
												associationForeignKeys = append(associationForeignKeys, foreignField.Name)
											}
										}
									}
								} else {
									// generate foreign keys & association foreign keys
									if len(associationForeignKeys) == 0 {
										for _, foreignKey := range foreignKeys {
											if strings.HasPrefix(foreignKey, field.Name) {
												associationForeignKey := strings.TrimPrefix(foreignKey, field.Name)
												if foreignField := getForeignField(associationForeignKey, toFields); foreignField != nil {
													associationForeignKeys = append(associationForeignKeys, associationForeignKey)
												}
											}
										}
										if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
											associationForeignKeys = []string{toScope.PrimaryKey()}
										}
									} else if len(foreignKeys) != len(associationForeignKeys) {
										scope.Err(errors.New("invalid foreign keys, should have same length"))
										return
									}
								}

								for idx, foreignKey := range foreignKeys {
									if foreignField := getForeignField(foreignKey, modelStruct.StructFields); foreignField != nil {
										if associationField := getForeignField(associationForeignKeys[idx], toFields); associationField != nil {
											foreignField.IsForeignKey = true

											// association foreign keys
											relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, associationField.Name)
											relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, associationField.DBName)

											// source foreign keys
											relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.Name)
											relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
										}
									}
								}

								if len(relationship.ForeignFieldNames) != 0 {
									relationship.Kind = "belongs_to"
									field.Relationship = relationship
								}
							}
						}(field)
					default:
						field.IsNormal = true
					}
				}
			}

			// Even it is ignored, also possible to decode db value into the field
			if value, ok := field.TagSettingsGet("COLUMN"); ok {
				field.DBName = value
			} else {
				field.DBName = ToColumnName(fieldStruct.Name)
			}

			modelStruct.StructFields = append(modelStruct.StructFields, field)
		}
	}

	if len(modelStruct.PrimaryFields) == 0 {
		if field := getForeignField("id", modelStruct.StructFields); field != nil {
			field.IsPrimaryKey = true
			modelStruct.PrimaryFields = append(modelStruct.PrimaryFields, field)
		}
	}

	modelStructsMap.Store(hashKey, &modelStruct)

	return &modelStruct
}

// GetStructFields get model's field structs
func (scope *Scope) GetStructFields() (fields []*StructField) {
	return scope.GetModelStruct().StructFields
}

func parseTagSetting(tags reflect.StructTag) map[string]string {
	setting := map[string]string{}
	for _, str := range []string{tags.Get("sql"), tags.Get("gorm")} {
		if str == "" {
			continue
		}
		tags := strings.Split(str, ";")
		for _, value := range tags {
			v := strings.Split(value, ":")
			k := strings.TrimSpace(strings.ToUpper(v[0]))
			if len(v) >= 2 {
				setting[k] = strings.Join(v[1:], ":")
			} else {
				setting[k] = k
			}
		}
	}
	return setting
}
