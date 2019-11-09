package codescan

import (
	"errors"
	"strconv"
	"strings"

	"github.com/go-openapi/spec"
)

const (
	// ParamDescriptionKey indicates the tag used to define a parameter description in swagger:route
	ParamDescriptionKey = "description"
	// ParamNameKey indicates the tag used to define a parameter name in swagger:route
	ParamNameKey = "name"
	// ParamInKey indicates the tag used to define a parameter location in swagger:route
	ParamInKey = "in"
	// ParamRequiredKey indicates the tag used to declare whether a parameter is required in swagger:route
	ParamRequiredKey = "required"
	// ParamTypeKey indicates the tag used to define the parameter type in swagger:route
	ParamTypeKey = "type"
	// ParamAllowEmptyKey indicates the tag used to indicate whether a parameter allows empty values in swagger:route
	ParamAllowEmptyKey = "allowempty"

	// SchemaMinKey indicates the tag used to indicate the minimum value allowed for this type in swagger:route
	SchemaMinKey = "min"
	// SchemaMaxKey indicates the tag used to indicate the maximum value allowed for this type in swagger:route
	SchemaMaxKey = "max"
	// SchemaEnumKey indicates the tag used to specify the allowed values for this type in swagger:route
	SchemaEnumKey = "enum"
	// SchemaFormatKey indicates the expected format for this field in swagger:route
	SchemaFormatKey = "format"
	// SchemaDefaultKey indicates the default value for this field in swagger:route
	SchemaDefaultKey = "default"
	// SchemaMinLenKey indicates the minimum length this field in swagger:route
	SchemaMinLenKey = "minlength"
	// SchemaMaxLenKey indicates the minimum length this field in swagger:route
	SchemaMaxLenKey = "maxlength"

	// TypeArray is the identifier for an array type in swagger:route
	TypeArray = "array"
	// TypeNumber is the identifier for a number type in swagger:route
	TypeNumber = "number"
	// TypeInteger is the identifier for an integer type in swagger:route
	TypeInteger = "integer"
	// TypeBoolean is the identifier for a boolean type in swagger:route
	TypeBoolean = "boolean"
	// TypeBool is the identifier for a boolean type in swagger:route
	TypeBool = "bool"
	// TypeObject is the identifier for an object type in swagger:route
	TypeObject = "object"
	// TypeString is the identifier for a string type in swagger:route
	TypeString = "string"
)

var (
	validIn    = []string{"path", "query", "header", "body", "form"}
	basicTypes = []string{TypeInteger, TypeNumber, TypeString, TypeBoolean, TypeBool, TypeArray}
)

func newSetParams(params []*spec.Parameter, setter func([]*spec.Parameter)) *setOpParams {
	return &setOpParams{
		set:        setter,
		parameters: params,
	}
}

type setOpParams struct {
	set        func([]*spec.Parameter)
	parameters []*spec.Parameter
}

func (s *setOpParams) Matches(line string) bool {
	return rxParameters.MatchString(line)
}

func (s *setOpParams) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	var current *spec.Parameter
	var extraData map[string]string

	for _, line := range lines {
		l := strings.TrimSpace(line)

		if strings.HasPrefix(l, "+") {
			s.finalizeParam(current, extraData)
			current = new(spec.Parameter)
			extraData = make(map[string]string)
			l = strings.TrimPrefix(l, "+")
		}

		kv := strings.SplitN(l, ":", 2)

		if len(kv) <= 1 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])

		if current == nil {
			return errors.New("invalid route/operation schema provided")
		}

		switch key {
		case ParamDescriptionKey:
			current.Description = value
		case ParamNameKey:
			current.Name = value
		case ParamInKey:
			v := strings.ToLower(value)
			if contains(validIn, v) {
				current.In = v
			}
		case ParamRequiredKey:
			if v, err := strconv.ParseBool(value); err == nil {
				current.Required = v
			}
		case ParamTypeKey:
			if current.Schema == nil {
				current.Schema = new(spec.Schema)
			}
			if contains(basicTypes, value) {
				current.Type = strings.ToLower(value)
				if current.Type == TypeBool {
					current.Type = TypeBoolean
				}
			} else if ref, err := spec.NewRef("#/definitions/" + value); err == nil {
				current.Type = TypeObject
				current.Schema.Ref = ref
			}
			current.Schema.Type = spec.StringOrArray{current.Type}
		case ParamAllowEmptyKey:
			if v, err := strconv.ParseBool(value); err == nil {
				current.AllowEmptyValue = v
			}
		default:
			extraData[key] = value
		}
	}

	s.finalizeParam(current, extraData)
	s.set(s.parameters)
	return nil
}

func (s *setOpParams) finalizeParam(param *spec.Parameter, data map[string]string) {
	if param == nil {
		return
	}

	processSchema(data, param)
	s.parameters = append(s.parameters, param)
}

func processSchema(data map[string]string, param *spec.Parameter) {
	if param.Schema == nil {
		return
	}

	var enumValues []string

	for key, value := range data {
		switch key {
		case SchemaMinKey:
			if t := getType(param.Schema); t == TypeNumber || t == TypeInteger {
				v, _ := strconv.ParseFloat(value, 64)
				param.Schema.Minimum = &v
			}
		case SchemaMaxKey:
			if t := getType(param.Schema); t == TypeNumber || t == TypeInteger {
				v, _ := strconv.ParseFloat(value, 64)
				param.Schema.Maximum = &v
			}
		case SchemaMinLenKey:
			if getType(param.Schema) == TypeArray {
				v, _ := strconv.ParseInt(value, 10, 64)
				param.Schema.MinLength = &v
			}
		case SchemaMaxLenKey:
			if getType(param.Schema) == TypeArray {
				v, _ := strconv.ParseInt(value, 10, 64)
				param.Schema.MaxLength = &v
			}
		case SchemaEnumKey:
			enumValues = strings.Split(value, ",")
		case SchemaFormatKey:
			param.Schema.Format = value
		case SchemaDefaultKey:
			param.Schema.Default = convert(param.Type, value)
		}
	}

	if param.Description != "" {
		param.Schema.Description = param.Description
	}

	convertEnum(param.Schema, enumValues)
}

func convertEnum(schema *spec.Schema, enumValues []string) {
	if len(enumValues) == 0 {
		return
	}

	var finalEnum []interface{}
	for _, v := range enumValues {
		finalEnum = append(finalEnum, convert(schema.Type[0], strings.TrimSpace(v)))
	}
	schema.Enum = finalEnum
}

func convert(typeStr, valueStr string) interface{} {
	switch typeStr {
	case TypeInteger:
		fallthrough
	case TypeNumber:
		if num, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return num
		}
	case TypeBoolean:
		fallthrough
	case TypeBool:
		if b, err := strconv.ParseBool(valueStr); err == nil {
			return b
		}
	}
	return valueStr
}

func getType(schema *spec.Schema) string {
	if len(schema.Type) == 0 {
		return ""
	}
	return schema.Type[0]
}

func contains(arr []string, obj string) bool {
	for _, v := range arr {
		if v == obj {
			return true
		}
	}
	return false
}
