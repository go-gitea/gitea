package graphql

import (
	"fmt"
	"reflect"
	"strings"
)

const TAG = "json"

// can't take recursive slice type
// e.g
// type Person struct{
//	Friends []Person
// }
// it will throw panic stack-overflow
func BindFields(obj interface{}) Fields {
	t := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)
	fields := make(map[string]*Field)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		tag := extractTag(field.Tag)
		if tag == "-" {
			continue
		}

		fieldType := field.Type

		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		var graphType Output
		if fieldType.Kind() == reflect.Struct {
			structFields := BindFields(v.Field(i).Interface())

			if tag == "" {
				fields = appendFields(fields, structFields)
				continue
			} else {
				graphType = NewObject(ObjectConfig{
					Name:   tag,
					Fields: structFields,
				})
			}
		}

		if tag == "" {
			continue
		}

		if graphType == nil {
			graphType = getGraphType(fieldType)
		}
		fields[tag] = &Field{
			Type: graphType,
			Resolve: func(p ResolveParams) (interface{}, error) {
				return extractValue(tag, p.Source), nil
			},
		}
	}
	return fields
}

func getGraphType(tipe reflect.Type) Output {
	kind := tipe.Kind()
	switch kind {
	case reflect.String:
		return String
	case reflect.Int, reflect.Int8, reflect.Int32, reflect.Int64:
		return Int
	case reflect.Float32, reflect.Float64:
		return Float
	case reflect.Bool:
		return Boolean
	case reflect.Slice:
		return getGraphList(tipe)
	}
	return String
}

func getGraphList(tipe reflect.Type) *List {
	if tipe.Kind() == reflect.Slice {
		switch tipe.Elem().Kind() {
		case reflect.Int, reflect.Int8, reflect.Int32, reflect.Int64:
			return NewList(Int)
		case reflect.Bool:
			return NewList(Boolean)
		case reflect.Float32, reflect.Float64:
			return NewList(Float)
		case reflect.String:
			return NewList(String)
		}
	}
	// finally bind object
	t := reflect.New(tipe.Elem())
	name := strings.Replace(fmt.Sprint(tipe.Elem()), ".", "_", -1)
	obj := NewObject(ObjectConfig{
		Name:   name,
		Fields: BindFields(t.Elem().Interface()),
	})
	return NewList(obj)
}

func appendFields(dest, origin Fields) Fields {
	for key, value := range origin {
		dest[key] = value
	}
	return dest
}

func extractValue(originTag string, obj interface{}) interface{} {
	val := reflect.Indirect(reflect.ValueOf(obj))

	for j := 0; j < val.NumField(); j++ {
		field := val.Type().Field(j)
		if field.Type.Kind() == reflect.Struct {
			res := extractValue(originTag, val.Field(j).Interface())
			if res != nil {
				return res
			}
		}

		if originTag == extractTag(field.Tag) {
			return reflect.Indirect(val.Field(j)).Interface()
		}
	}
	return nil
}

func extractTag(tag reflect.StructTag) string {
	t := tag.Get(TAG)
	if t != "" {
		t = strings.Split(t, ",")[0]
	}
	return t
}

// lazy way of binding args
func BindArg(obj interface{}, tags ...string) FieldConfigArgument {
	v := reflect.Indirect(reflect.ValueOf(obj))
	var config = make(FieldConfigArgument)
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)

		mytag := extractTag(field.Tag)
		if inArray(tags, mytag) {
			config[mytag] = &ArgumentConfig{
				Type: getGraphType(field.Type),
			}
		}
	}
	return config
}

func inArray(slice interface{}, item interface{}) bool {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic("inArray() given a non-slice type")
	}

	for i := 0; i < s.Len(); i++ {
		if reflect.DeepEqual(item, s.Index(i).Interface()) {
			return true
		}
	}
	return false
}
