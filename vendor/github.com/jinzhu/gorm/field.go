package gorm

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
)

// Field model field definition
type Field struct {
	*StructField
	IsBlank bool
	Field   reflect.Value
}

// Set set a value to the field
func (field *Field) Set(value interface{}) (err error) {
	if !field.Field.IsValid() {
		return errors.New("field value not valid")
	}

	if !field.Field.CanAddr() {
		return ErrUnaddressable
	}

	reflectValue, ok := value.(reflect.Value)
	if !ok {
		reflectValue = reflect.ValueOf(value)
	}

	fieldValue := field.Field
	if reflectValue.IsValid() {
		if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
			fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
		} else {
			if fieldValue.Kind() == reflect.Ptr {
				if fieldValue.IsNil() {
					fieldValue.Set(reflect.New(field.Struct.Type.Elem()))
				}
				fieldValue = fieldValue.Elem()
			}

			if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
				fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
			} else if scanner, ok := fieldValue.Addr().Interface().(sql.Scanner); ok {
				v := reflectValue.Interface()
				if valuer, ok := v.(driver.Valuer); ok {
					if v, err = valuer.Value(); err == nil {
						err = scanner.Scan(v)
					}
				} else {
					err = scanner.Scan(v)
				}
			} else {
				err = fmt.Errorf("could not convert argument of field %s from %s to %s", field.Name, reflectValue.Type(), fieldValue.Type())
			}
		}
	} else {
		field.Field.Set(reflect.Zero(field.Field.Type()))
	}

	field.IsBlank = isBlank(field.Field)
	return err
}
