package graphql

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	"github.com/graphql-go/graphql/language/printer"
)

// Prepares an object map of variableValues of the correct type based on the
// provided variable definitions and arbitrary input. If the input cannot be
// parsed to match the variable definitions, a GraphQLError will be returned.
func getVariableValues(
	schema Schema,
	definitionASTs []*ast.VariableDefinition,
	inputs map[string]interface{}) (map[string]interface{}, error) {
	values := map[string]interface{}{}
	for _, defAST := range definitionASTs {
		if defAST == nil || defAST.Variable == nil || defAST.Variable.Name == nil {
			continue
		}
		varName := defAST.Variable.Name.Value
		if varValue, err := getVariableValue(schema, defAST, inputs[varName]); err != nil {
			return values, err
		} else {
			values[varName] = varValue
		}
	}
	return values, nil
}

// Prepares an object map of argument values given a list of argument
// definitions and list of argument AST nodes.
func getArgumentValues(
	argDefs []*Argument, argASTs []*ast.Argument,
	variableValues map[string]interface{}) map[string]interface{} {

	argASTMap := map[string]*ast.Argument{}
	for _, argAST := range argASTs {
		if argAST.Name != nil {
			argASTMap[argAST.Name.Value] = argAST
		}
	}
	results := map[string]interface{}{}
	for _, argDef := range argDefs {
		var (
			tmp   interface{}
			value ast.Value
		)
		if tmpValue, ok := argASTMap[argDef.PrivateName]; ok {
			value = tmpValue.Value
		}
		if tmp = valueFromAST(value, argDef.Type, variableValues); isNullish(tmp) {
			tmp = argDef.DefaultValue
		}
		if !isNullish(tmp) {
			results[argDef.PrivateName] = tmp
		}
	}
	return results
}

// Given a variable definition, and any value of input, return a value which
// adheres to the variable definition, or throw an error.
func getVariableValue(schema Schema, definitionAST *ast.VariableDefinition, input interface{}) (interface{}, error) {
	ttype, err := typeFromAST(schema, definitionAST.Type)
	if err != nil {
		return nil, err
	}
	variable := definitionAST.Variable

	if ttype == nil || !IsInputType(ttype) {
		return "", gqlerrors.NewError(
			fmt.Sprintf(`Variable "$%v" expected value of type `+
				`"%v" which cannot be used as an input type.`, variable.Name.Value, printer.Print(definitionAST.Type)),
			[]ast.Node{definitionAST},
			"",
			nil,
			[]int{},
			nil,
		)
	}

	isValid, messages := isValidInputValue(input, ttype)
	if isValid {
		if isNullish(input) {
			if definitionAST.DefaultValue != nil {
				return valueFromAST(definitionAST.DefaultValue, ttype, nil), nil
			}
		}
		return coerceValue(ttype, input), nil
	}
	if isNullish(input) {
		return "", gqlerrors.NewError(
			fmt.Sprintf(`Variable "$%v" of required type `+
				`"%v" was not provided.`, variable.Name.Value, printer.Print(definitionAST.Type)),
			[]ast.Node{definitionAST},
			"",
			nil,
			[]int{},
			nil,
		)
	}
	// convert input interface into string for error message
	bts, _ := json.Marshal(input)
	var (
		inputStr = string(bts)
		msg      string
	)
	if len(messages) > 0 {
		msg = "\n" + strings.Join(messages, "\n")
	}

	return "", gqlerrors.NewError(
		fmt.Sprintf(`Variable "$%v" got invalid value `+
			`%v.%v`, variable.Name.Value, inputStr, msg),
		[]ast.Node{definitionAST},
		"",
		nil,
		[]int{},
		nil,
	)
}

// Given a type and any value, return a runtime value coerced to match the type.
func coerceValue(ttype Input, value interface{}) interface{} {
	if isNullish(value) {
		return nil
	}
	switch ttype := ttype.(type) {
	case *NonNull:
		return coerceValue(ttype.OfType, value)
	case *List:
		var values = []interface{}{}
		valType := reflect.ValueOf(value)
		if valType.Kind() == reflect.Slice {
			for i := 0; i < valType.Len(); i++ {
				val := valType.Index(i).Interface()
				values = append(values, coerceValue(ttype.OfType, val))
			}
			return values
		}
		return append(values, coerceValue(ttype.OfType, value))
	case *InputObject:
		var obj = map[string]interface{}{}
		valueMap, _ := value.(map[string]interface{})
		if valueMap == nil {
			valueMap = map[string]interface{}{}
		}

		for name, field := range ttype.Fields() {
			fieldValue := coerceValue(field.Type, valueMap[name])
			if isNullish(fieldValue) {
				fieldValue = field.DefaultValue
			}
			if !isNullish(fieldValue) {
				obj[name] = fieldValue
			}
		}
		return obj
	case *Scalar:
		if parsed := ttype.ParseValue(value); !isNullish(parsed) {
			return parsed
		}
	case *Enum:
		if parsed := ttype.ParseValue(value); !isNullish(parsed) {
			return parsed
		}
	}

	return nil
}

// graphql-js/src/utilities.js`
// TODO: figure out where to organize utils
// TODO: change to *Schema
func typeFromAST(schema Schema, inputTypeAST ast.Type) (Type, error) {
	switch inputTypeAST := inputTypeAST.(type) {
	case *ast.List:
		innerType, err := typeFromAST(schema, inputTypeAST.Type)
		if err != nil {
			return nil, err
		}
		return NewList(innerType), nil
	case *ast.NonNull:
		innerType, err := typeFromAST(schema, inputTypeAST.Type)
		if err != nil {
			return nil, err
		}
		return NewNonNull(innerType), nil
	case *ast.Named:
		nameValue := ""
		if inputTypeAST.Name != nil {
			nameValue = inputTypeAST.Name.Value
		}
		ttype := schema.Type(nameValue)
		return ttype, nil
	default:
		return nil, invariant(inputTypeAST.GetKind() == kinds.Named, "Must be a named type.")
	}
}

// isValidInputValue alias isValidJSValue
// Given a value and a GraphQL type, determine if the value will be
// accepted for that type. This is primarily useful for validating the
// runtime values of query variables.
func isValidInputValue(value interface{}, ttype Input) (bool, []string) {
	if isNullish(value) {
		if ttype, ok := ttype.(*NonNull); ok {
			if ttype.OfType.Name() != "" {
				return false, []string{fmt.Sprintf(`Expected "%v!", found null.`, ttype.OfType.Name())}
			}
			return false, []string{"Expected non-null value, found null."}
		}
		return true, nil
	}
	switch ttype := ttype.(type) {
	case *NonNull:
		return isValidInputValue(value, ttype.OfType)
	case *List:
		valType := reflect.ValueOf(value)
		if valType.Kind() == reflect.Ptr {
			valType = valType.Elem()
		}
		if valType.Kind() == reflect.Slice {
			messagesReduce := []string{}
			for i := 0; i < valType.Len(); i++ {
				val := valType.Index(i).Interface()
				_, messages := isValidInputValue(val, ttype.OfType)
				for idx, message := range messages {
					messagesReduce = append(messagesReduce, fmt.Sprintf(`In element #%v: %v`, idx+1, message))
				}
			}
			return (len(messagesReduce) == 0), messagesReduce
		}
		return isValidInputValue(value, ttype.OfType)

	case *InputObject:
		messagesReduce := []string{}

		valueMap, ok := value.(map[string]interface{})
		if !ok {
			return false, []string{fmt.Sprintf(`Expected "%v", found not an object.`, ttype.Name())}
		}
		fields := ttype.Fields()

		// to ensure stable order of field evaluation
		fieldNames := []string{}
		valueMapFieldNames := []string{}

		for fieldName := range fields {
			fieldNames = append(fieldNames, fieldName)
		}
		sort.Strings(fieldNames)

		for fieldName := range valueMap {
			valueMapFieldNames = append(valueMapFieldNames, fieldName)
		}
		sort.Strings(valueMapFieldNames)

		// Ensure every provided field is defined.
		for _, fieldName := range valueMapFieldNames {
			if _, ok := fields[fieldName]; !ok {
				messagesReduce = append(messagesReduce, fmt.Sprintf(`In field "%v": Unknown field.`, fieldName))
			}
		}

		// Ensure every defined field is valid.
		for _, fieldName := range fieldNames {
			_, messages := isValidInputValue(valueMap[fieldName], fields[fieldName].Type)
			if messages != nil {
				for _, message := range messages {
					messagesReduce = append(messagesReduce, fmt.Sprintf(`In field "%v": %v`, fieldName, message))
				}
			}
		}
		return (len(messagesReduce) == 0), messagesReduce
	case *Scalar:
		if parsedVal := ttype.ParseValue(value); isNullish(parsedVal) {
			return false, []string{fmt.Sprintf(`Expected type "%v", found "%v".`, ttype.Name(), value)}
		}
	case *Enum:
		if parsedVal := ttype.ParseValue(value); isNullish(parsedVal) {
			return false, []string{fmt.Sprintf(`Expected type "%v", found "%v".`, ttype.Name(), value)}
		}
	}

	return true, nil
}

// Returns true if a value is null, undefined, or NaN.
func isNullish(src interface{}) bool {
	if src == nil {
		return true
	}
	value := reflect.ValueOf(src)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return true
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.String:
		// if src is ptr type and len(string)=0, it returns false
		if !value.IsValid() {
			return true
		}
	case reflect.Int:
		return math.IsNaN(float64(value.Int()))
	case reflect.Float32, reflect.Float64:
		return math.IsNaN(float64(value.Float()))
	}
	return false
}

// Returns true if src is a slice or an array
func isIterable(src interface{}) bool {
	if src == nil {
		return false
	}
	t := reflect.TypeOf(src)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() == reflect.Slice || t.Kind() == reflect.Array
}

/**
 * Produces a value given a GraphQL Value AST.
 *
 * A GraphQL type must be provided, which will be used to interpret different
 * GraphQL Value literals.
 *
 * | GraphQL Value        | JSON Value    |
 * | -------------------- | ------------- |
 * | Input Object         | Object        |
 * | List                 | Array         |
 * | Boolean              | Boolean       |
 * | String / Enum Value  | String        |
 * | Int / Float          | Number        |
 *
 */
func valueFromAST(valueAST ast.Value, ttype Input, variables map[string]interface{}) interface{} {
	if valueAST == nil {
		return nil
	}
	// precedence: value > type
	if valueAST, ok := valueAST.(*ast.Variable); ok {
		if valueAST.Name == nil || variables == nil {
			return nil
		}
		// Note: we're not doing any checking that this variable is correct. We're
		// assuming that this query has been validated and the variable usage here
		// is of the correct type.
		return variables[valueAST.Name.Value]
	}
	switch ttype := ttype.(type) {
	case *NonNull:
		return valueFromAST(valueAST, ttype.OfType, variables)
	case *List:
		values := []interface{}{}
		if valueAST, ok := valueAST.(*ast.ListValue); ok {
			for _, itemAST := range valueAST.Values {
				values = append(values, valueFromAST(itemAST, ttype.OfType, variables))
			}
			return values
		}
		return append(values, valueFromAST(valueAST, ttype.OfType, variables))
	case *InputObject:
		var (
			ok bool
			ov *ast.ObjectValue
			of *ast.ObjectField
		)
		if ov, ok = valueAST.(*ast.ObjectValue); !ok {
			return nil
		}
		fieldASTs := map[string]*ast.ObjectField{}
		for _, of = range ov.Fields {
			if of == nil || of.Name == nil {
				continue
			}
			fieldASTs[of.Name.Value] = of
		}
		obj := map[string]interface{}{}
		for name, field := range ttype.Fields() {
			var value interface{}
			if of, ok = fieldASTs[name]; ok {
				value = valueFromAST(of.Value, field.Type, variables)
			} else {
				value = field.DefaultValue
			}
			if !isNullish(value) {
				obj[name] = value
			}
		}
		return obj
	case *Scalar:
		return ttype.ParseLiteral(valueAST)
	case *Enum:
		return ttype.ParseLiteral(valueAST)
	}

	return nil
}

func invariant(condition bool, message string) error {
	if !condition {
		return gqlerrors.NewFormattedError(message)
	}
	return nil
}

func invariantf(condition bool, format string, a ...interface{}) error {
	if !condition {
		return gqlerrors.NewFormattedError(fmt.Sprintf(format, a...))
	}
	return nil
}
