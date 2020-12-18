package gorm

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Define callbacks for updating
func init() {
	DefaultCallback.Update().Register("gorm:assign_updating_attributes", assignUpdatingAttributesCallback)
	DefaultCallback.Update().Register("gorm:begin_transaction", beginTransactionCallback)
	DefaultCallback.Update().Register("gorm:before_update", beforeUpdateCallback)
	DefaultCallback.Update().Register("gorm:save_before_associations", saveBeforeAssociationsCallback)
	DefaultCallback.Update().Register("gorm:update_time_stamp", updateTimeStampForUpdateCallback)
	DefaultCallback.Update().Register("gorm:update", updateCallback)
	DefaultCallback.Update().Register("gorm:save_after_associations", saveAfterAssociationsCallback)
	DefaultCallback.Update().Register("gorm:after_update", afterUpdateCallback)
	DefaultCallback.Update().Register("gorm:commit_or_rollback_transaction", commitOrRollbackTransactionCallback)
}

// assignUpdatingAttributesCallback assign updating attributes to model
func assignUpdatingAttributesCallback(scope *Scope) {
	if attrs, ok := scope.InstanceGet("gorm:update_interface"); ok {
		if updateMaps, hasUpdate := scope.updatedAttrsWithValues(attrs); hasUpdate {
			scope.InstanceSet("gorm:update_attrs", updateMaps)
		} else {
			scope.SkipLeft()
		}
	}
}

// beforeUpdateCallback will invoke `BeforeSave`, `BeforeUpdate` method before updating
func beforeUpdateCallback(scope *Scope) {
	if scope.DB().HasBlockGlobalUpdate() && !scope.hasConditions() {
		scope.Err(errors.New("missing WHERE clause while updating"))
		return
	}
	if _, ok := scope.Get("gorm:update_column"); !ok {
		if !scope.HasError() {
			scope.CallMethod("BeforeSave")
		}
		if !scope.HasError() {
			scope.CallMethod("BeforeUpdate")
		}
	}
}

// updateTimeStampForUpdateCallback will set `UpdatedAt` when updating
func updateTimeStampForUpdateCallback(scope *Scope) {
	if _, ok := scope.Get("gorm:update_column"); !ok {
		scope.SetColumn("UpdatedAt", scope.db.nowFunc())
	}
}

// updateCallback the callback used to update data to database
func updateCallback(scope *Scope) {
	if !scope.HasError() {
		var sqls []string

		if updateAttrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
			// Sort the column names so that the generated SQL is the same every time.
			updateMap := updateAttrs.(map[string]interface{})
			var columns []string
			for c := range updateMap {
				columns = append(columns, c)
			}
			sort.Strings(columns)

			for _, column := range columns {
				value := updateMap[column]
				sqls = append(sqls, fmt.Sprintf("%v = %v", scope.Quote(column), scope.AddToVars(value)))
			}
		} else {
			for _, field := range scope.Fields() {
				if scope.changeableField(field) {
					if !field.IsPrimaryKey && field.IsNormal && (field.Name != "CreatedAt" || !field.IsBlank) {
						if !field.IsForeignKey || !field.IsBlank || !field.HasDefaultValue {
							sqls = append(sqls, fmt.Sprintf("%v = %v", scope.Quote(field.DBName), scope.AddToVars(field.Field.Interface())))
						}
					} else if relationship := field.Relationship; relationship != nil && relationship.Kind == "belongs_to" {
						for _, foreignKey := range relationship.ForeignDBNames {
							if foreignField, ok := scope.FieldByName(foreignKey); ok && !scope.changeableField(foreignField) {
								sqls = append(sqls,
									fmt.Sprintf("%v = %v", scope.Quote(foreignField.DBName), scope.AddToVars(foreignField.Field.Interface())))
							}
						}
					}
				}
			}
		}

		var extraOption string
		if str, ok := scope.Get("gorm:update_option"); ok {
			extraOption = fmt.Sprint(str)
		}

		if len(sqls) > 0 {
			scope.Raw(fmt.Sprintf(
				"UPDATE %v SET %v%v%v",
				scope.QuotedTableName(),
				strings.Join(sqls, ", "),
				addExtraSpaceIfExist(scope.CombinedConditionSql()),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
}

// afterUpdateCallback will invoke `AfterUpdate`, `AfterSave` method after updating
func afterUpdateCallback(scope *Scope) {
	if _, ok := scope.Get("gorm:update_column"); !ok {
		if !scope.HasError() {
			scope.CallMethod("AfterUpdate")
		}
		if !scope.HasError() {
			scope.CallMethod("AfterSave")
		}
	}
}
