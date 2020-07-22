package graphql

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/graphql-go/graphql/language/ast"
)

// As per the GraphQL Spec, Integers are only treated as valid when a valid
// 32-bit signed integer, providing the broadest support across platforms.
//
// n.b. JavaScript's integers are safe between -(2^53 - 1) and 2^53 - 1 because
// they are internally represented as IEEE 754 doubles.
func coerceInt(value interface{}) interface{} {
	switch value := value.(type) {
	case bool:
		if value == true {
			return 1
		}
		return 0
	case *bool:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	case int:
		if value < int(math.MinInt32) || value > int(math.MaxInt32) {
			return nil
		}
		return value
	case *int:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	case int8:
		return int(value)
	case *int8:
		if value == nil {
			return nil
		}
		return int(*value)
	case int16:
		return int(value)
	case *int16:
		if value == nil {
			return nil
		}
		return int(*value)
	case int32:
		return int(value)
	case *int32:
		if value == nil {
			return nil
		}
		return int(*value)
	case int64:
		if value < int64(math.MinInt32) || value > int64(math.MaxInt32) {
			return nil
		}
		return int(value)
	case *int64:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	case uint:
		if value > math.MaxInt32 {
			return nil
		}
		return int(value)
	case *uint:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	case uint8:
		return int(value)
	case *uint8:
		if value == nil {
			return nil
		}
		return int(*value)
	case uint16:
		return int(value)
	case *uint16:
		if value == nil {
			return nil
		}
		return int(*value)
	case uint32:
		if value > uint32(math.MaxInt32) {
			return nil
		}
		return int(value)
	case *uint32:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	case uint64:
		if value > uint64(math.MaxInt32) {
			return nil
		}
		return int(value)
	case *uint64:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	case float32:
		if value < float32(math.MinInt32) || value > float32(math.MaxInt32) {
			return nil
		}
		return int(value)
	case *float32:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	case float64:
		if value < float64(math.MinInt32) || value > float64(math.MaxInt32) {
			return nil
		}
		return int(value)
	case *float64:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	case string:
		val, err := strconv.ParseFloat(value, 0)
		if err != nil {
			return nil
		}
		return coerceInt(val)
	case *string:
		if value == nil {
			return nil
		}
		return coerceInt(*value)
	}

	// If the value cannot be transformed into an int, return nil instead of '0'
	// to denote 'no integer found'
	return nil
}

// Int is the GraphQL Integer type definition.
var Int = NewScalar(ScalarConfig{
	Name: "Int",
	Description: "The `Int` scalar type represents non-fractional signed whole numeric " +
		"values. Int can represent values between -(2^31) and 2^31 - 1. ",
	Serialize:  coerceInt,
	ParseValue: coerceInt,
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.IntValue:
			if intValue, err := strconv.Atoi(valueAST.Value); err == nil {
				return intValue
			}
		}
		return nil
	},
})

func coerceFloat(value interface{}) interface{} {
	switch value := value.(type) {
	case bool:
		if value == true {
			return 1.0
		}
		return 0.0
	case *bool:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case int:
		return float64(value)
	case *int:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case int8:
		return float64(value)
	case *int8:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case int16:
		return float64(value)
	case *int16:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case int32:
		return float64(value)
	case *int32:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case int64:
		return float64(value)
	case *int64:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case uint:
		return float64(value)
	case *uint:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case uint8:
		return float64(value)
	case *uint8:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case uint16:
		return float64(value)
	case *uint16:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case uint32:
		return float64(value)
	case *uint32:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case uint64:
		return float64(value)
	case *uint64:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case float32:
		return value
	case *float32:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case float64:
		return value
	case *float64:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	case string:
		val, err := strconv.ParseFloat(value, 0)
		if err != nil {
			return nil
		}
		return val
	case *string:
		if value == nil {
			return nil
		}
		return coerceFloat(*value)
	}

	// If the value cannot be transformed into an float, return nil instead of '0.0'
	// to denote 'no float found'
	return nil
}

// Float is the GraphQL float type definition.
var Float = NewScalar(ScalarConfig{
	Name: "Float",
	Description: "The `Float` scalar type represents signed double-precision fractional " +
		"values as specified by " +
		"[IEEE 754](http://en.wikipedia.org/wiki/IEEE_floating_point). ",
	Serialize:  coerceFloat,
	ParseValue: coerceFloat,
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.FloatValue:
			if floatValue, err := strconv.ParseFloat(valueAST.Value, 64); err == nil {
				return floatValue
			}
		case *ast.IntValue:
			if floatValue, err := strconv.ParseFloat(valueAST.Value, 64); err == nil {
				return floatValue
			}
		}
		return nil
	},
})

func coerceString(value interface{}) interface{} {
	if v, ok := value.(*string); ok {
		if v == nil {
			return nil
		}
		return *v
	}
	return fmt.Sprintf("%v", value)
}

// String is the GraphQL string type definition
var String = NewScalar(ScalarConfig{
	Name: "String",
	Description: "The `String` scalar type represents textual data, represented as UTF-8 " +
		"character sequences. The String type is most often used by GraphQL to " +
		"represent free-form human-readable text.",
	Serialize:  coerceString,
	ParseValue: coerceString,
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			return valueAST.Value
		}
		return nil
	},
})

func coerceBool(value interface{}) interface{} {
	switch value := value.(type) {
	case bool:
		return value
	case *bool:
		if value == nil {
			return nil
		}
		return *value
	case string:
		switch value {
		case "", "false":
			return false
		}
		return true
	case *string:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case float64:
		if value != 0 {
			return true
		}
		return false
	case *float64:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case float32:
		if value != 0 {
			return true
		}
		return false
	case *float32:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case int:
		if value != 0 {
			return true
		}
		return false
	case *int:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case int8:
		if value != 0 {
			return true
		}
		return false
	case *int8:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case int16:
		if value != 0 {
			return true
		}
		return false
	case *int16:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case int32:
		if value != 0 {
			return true
		}
		return false
	case *int32:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case int64:
		if value != 0 {
			return true
		}
		return false
	case *int64:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case uint:
		if value != 0 {
			return true
		}
		return false
	case *uint:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case uint8:
		if value != 0 {
			return true
		}
		return false
	case *uint8:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case uint16:
		if value != 0 {
			return true
		}
		return false
	case *uint16:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case uint32:
		if value != 0 {
			return true
		}
		return false
	case *uint32:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	case uint64:
		if value != 0 {
			return true
		}
		return false
	case *uint64:
		if value == nil {
			return nil
		}
		return coerceBool(*value)
	}
	return false
}

// Boolean is the GraphQL boolean type definition
var Boolean = NewScalar(ScalarConfig{
	Name:        "Boolean",
	Description: "The `Boolean` scalar type represents `true` or `false`.",
	Serialize:   coerceBool,
	ParseValue:  coerceBool,
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.BooleanValue:
			return valueAST.Value
		}
		return nil
	},
})

// ID is the GraphQL id type definition
var ID = NewScalar(ScalarConfig{
	Name: "ID",
	Description: "The `ID` scalar type represents a unique identifier, often used to " +
		"refetch an object or as key for a cache. The ID type appears in a JSON " +
		"response as a String; however, it is not intended to be human-readable. " +
		"When expected as an input type, any string (such as `\"4\"`) or integer " +
		"(such as `4`) input value will be accepted as an ID.",
	Serialize:  coerceString,
	ParseValue: coerceString,
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.IntValue:
			return valueAST.Value
		case *ast.StringValue:
			return valueAST.Value
		}
		return nil
	},
})

func serializeDateTime(value interface{}) interface{} {
	switch value := value.(type) {
	case time.Time:
		buff, err := value.MarshalText()
		if err != nil {
			return nil
		}

		return string(buff)
	case *time.Time:
		if value == nil {
			return nil
		}
		return serializeDateTime(*value)
	default:
		return nil
	}
}

func unserializeDateTime(value interface{}) interface{} {
	switch value := value.(type) {
	case []byte:
		t := time.Time{}
		err := t.UnmarshalText(value)
		if err != nil {
			return nil
		}

		return t
	case string:
		return unserializeDateTime([]byte(value))
	case *string:
		if value == nil {
			return nil
		}
		return unserializeDateTime([]byte(*value))
	case time.Time:
		return value
	default:
		return nil
	}
}

var DateTime = NewScalar(ScalarConfig{
	Name: "DateTime",
	Description: "The `DateTime` scalar type represents a DateTime." +
		" The DateTime is serialized as an RFC 3339 quoted string",
	Serialize:  serializeDateTime,
	ParseValue: unserializeDateTime,
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			return unserializeDateTime(valueAST.Value)
		}
		return nil
	},
})
