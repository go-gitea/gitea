package printer

import (
	"fmt"
	"strings"

	"reflect"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/visitor"
)

func getMapValue(m map[string]interface{}, key string) interface{} {
	tokens := strings.Split(key, ".")
	valMap := m
	for _, token := range tokens {
		v, ok := valMap[token]
		if !ok {
			return nil
		}
		switch v := v.(type) {
		case []interface{}:
			return v
		case map[string]interface{}:
			valMap = v
			continue
		default:
			return v
		}
	}
	return valMap
}
func getMapSliceValue(m map[string]interface{}, key string) []interface{} {
	tokens := strings.Split(key, ".")
	valMap := m
	for _, token := range tokens {
		v, ok := valMap[token]
		if !ok {
			return []interface{}{}
		}
		switch v := v.(type) {
		case []interface{}:
			return v
		default:
			return []interface{}{}
		}
	}
	return []interface{}{}
}
func getMapValueString(m map[string]interface{}, key string) string {
	tokens := strings.Split(key, ".")
	valMap := m
	for _, token := range tokens {
		v, ok := valMap[token]
		if !ok {
			return ""
		}
		if v == nil {
			return ""
		}
		switch v := v.(type) {
		case map[string]interface{}:
			valMap = v
			continue
		case string:
			return v
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func toSliceString(slice interface{}) []string {
	if slice == nil {
		return []string{}
	}
	res := []string{}
	switch reflect.TypeOf(slice).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(slice)
		for i := 0; i < s.Len(); i++ {
			elem := s.Index(i)
			elemInterface := elem.Interface()
			if elem, ok := elemInterface.(string); ok {
				res = append(res, elem)
			}
		}
		return res
	default:
		return res
	}
}

func join(str []string, sep string) string {
	ss := []string{}
	// filter out empty strings
	for _, s := range str {
		if s == "" {
			continue
		}
		ss = append(ss, s)
	}
	return strings.Join(ss, sep)
}

func wrap(start, maybeString, end string) string {
	if maybeString == "" {
		return maybeString
	}
	return start + maybeString + end
}

// Given array, print each item on its own line, wrapped in an indented "{ }" block.
func block(maybeArray interface{}) string {
	s := toSliceString(maybeArray)
	if len(s) == 0 {
		return "{}"
	}
	return indent("{\n"+join(s, "\n")) + "\n}"
}

func indent(maybeString interface{}) string {
	if maybeString == nil {
		return ""
	}
	switch str := maybeString.(type) {
	case string:
		return strings.Replace(str, "\n", "\n  ", -1)
	}
	return ""
}

var printDocASTReducer = map[string]visitor.VisitFunc{
	"Name": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.Name:
			return visitor.ActionUpdate, node.Value
		case map[string]interface{}:
			return visitor.ActionUpdate, getMapValue(node, "Value")
		}
		return visitor.ActionNoChange, nil
	},
	"Variable": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.Variable:
			return visitor.ActionUpdate, fmt.Sprintf("$%v", node.Name)
		case map[string]interface{}:
			return visitor.ActionUpdate, "$" + getMapValueString(node, "Name")
		}
		return visitor.ActionNoChange, nil
	},

	// Document
	"Document": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.Document:
			definitions := toSliceString(node.Definitions)
			return visitor.ActionUpdate, join(definitions, "\n\n") + "\n"
		case map[string]interface{}:
			definitions := toSliceString(getMapValue(node, "Definitions"))
			return visitor.ActionUpdate, join(definitions, "\n\n") + "\n"
		}
		return visitor.ActionNoChange, nil
	},
	"OperationDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.OperationDefinition:
			op := string(node.Operation)
			name := fmt.Sprintf("%v", node.Name)

			varDefs := wrap("(", join(toSliceString(node.VariableDefinitions), ", "), ")")
			directives := join(toSliceString(node.Directives), " ")
			selectionSet := fmt.Sprintf("%v", node.SelectionSet)
			// Anonymous queries with no directives or variable definitions can use
			// the query short form.
			str := ""
			if name == "" && directives == "" && varDefs == "" && op == ast.OperationTypeQuery {
				str = selectionSet
			} else {
				str = join([]string{
					op,
					join([]string{name, varDefs}, ""),
					directives,
					selectionSet,
				}, " ")
			}
			return visitor.ActionUpdate, str
		case map[string]interface{}:

			op := getMapValueString(node, "Operation")
			name := getMapValueString(node, "Name")

			varDefs := wrap("(", join(toSliceString(getMapValue(node, "VariableDefinitions")), ", "), ")")
			directives := join(toSliceString(getMapValue(node, "Directives")), " ")
			selectionSet := getMapValueString(node, "SelectionSet")
			str := ""
			if name == "" && directives == "" && varDefs == "" && op == ast.OperationTypeQuery {
				str = selectionSet
			} else {
				str = join([]string{
					op,
					join([]string{name, varDefs}, ""),
					directives,
					selectionSet,
				}, " ")
			}
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"VariableDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.VariableDefinition:
			variable := fmt.Sprintf("%v", node.Variable)
			ttype := fmt.Sprintf("%v", node.Type)
			defaultValue := fmt.Sprintf("%v", node.DefaultValue)

			return visitor.ActionUpdate, variable + ": " + ttype + wrap(" = ", defaultValue, "")
		case map[string]interface{}:

			variable := getMapValueString(node, "Variable")
			ttype := getMapValueString(node, "Type")
			defaultValue := getMapValueString(node, "DefaultValue")

			return visitor.ActionUpdate, variable + ": " + ttype + wrap(" = ", defaultValue, "")

		}
		return visitor.ActionNoChange, nil
	},
	"SelectionSet": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.SelectionSet:
			str := block(node.Selections)
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			selections := getMapValue(node, "Selections")
			str := block(selections)
			return visitor.ActionUpdate, str

		}
		return visitor.ActionNoChange, nil
	},
	"Field": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.Argument:
			name := fmt.Sprintf("%v", node.Name)
			value := fmt.Sprintf("%v", node.Value)
			return visitor.ActionUpdate, name + ": " + value
		case map[string]interface{}:

			alias := getMapValueString(node, "Alias")
			name := getMapValueString(node, "Name")
			args := toSliceString(getMapValue(node, "Arguments"))
			directives := toSliceString(getMapValue(node, "Directives"))
			selectionSet := getMapValueString(node, "SelectionSet")

			str := join(
				[]string{
					wrap("", alias, ": ") + name + wrap("(", join(args, ", "), ")"),
					join(directives, " "),
					selectionSet,
				},
				" ",
			)
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"Argument": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.FragmentSpread:
			name := fmt.Sprintf("%v", node.Name)
			directives := toSliceString(node.Directives)
			return visitor.ActionUpdate, "..." + name + wrap(" ", join(directives, " "), "")
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			value := getMapValueString(node, "Value")
			return visitor.ActionUpdate, name + ": " + value
		}
		return visitor.ActionNoChange, nil
	},

	// Fragments
	"FragmentSpread": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.InlineFragment:
			typeCondition := fmt.Sprintf("%v", node.TypeCondition)
			directives := toSliceString(node.Directives)
			selectionSet := fmt.Sprintf("%v", node.SelectionSet)
			return visitor.ActionUpdate, "... on " + typeCondition + " " + wrap("", join(directives, " "), " ") + selectionSet
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			directives := toSliceString(getMapValue(node, "Directives"))
			return visitor.ActionUpdate, "..." + name + wrap(" ", join(directives, " "), "")
		}
		return visitor.ActionNoChange, nil
	},
	"InlineFragment": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case map[string]interface{}:
			typeCondition := getMapValueString(node, "TypeCondition")
			directives := toSliceString(getMapValue(node, "Directives"))
			selectionSet := getMapValueString(node, "SelectionSet")
			return visitor.ActionUpdate,
				join([]string{
					"...",
					wrap("on ", typeCondition, ""),
					join(directives, " "),
					selectionSet,
				}, " ")
		}
		return visitor.ActionNoChange, nil
	},
	"FragmentDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.FragmentDefinition:
			name := fmt.Sprintf("%v", node.Name)
			typeCondition := fmt.Sprintf("%v", node.TypeCondition)
			directives := toSliceString(node.Directives)
			selectionSet := fmt.Sprintf("%v", node.SelectionSet)
			return visitor.ActionUpdate, "fragment " + name + " on " + typeCondition + " " + wrap("", join(directives, " "), " ") + selectionSet
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			typeCondition := getMapValueString(node, "TypeCondition")
			directives := toSliceString(getMapValue(node, "Directives"))
			selectionSet := getMapValueString(node, "SelectionSet")
			return visitor.ActionUpdate, "fragment " + name + " on " + typeCondition + " " + wrap("", join(directives, " "), " ") + selectionSet
		}
		return visitor.ActionNoChange, nil
	},

	// Value
	"IntValue": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.IntValue:
			return visitor.ActionUpdate, fmt.Sprintf("%v", node.Value)
		case map[string]interface{}:
			return visitor.ActionUpdate, getMapValueString(node, "Value")
		}
		return visitor.ActionNoChange, nil
	},
	"FloatValue": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.FloatValue:
			return visitor.ActionUpdate, fmt.Sprintf("%v", node.Value)
		case map[string]interface{}:
			return visitor.ActionUpdate, getMapValueString(node, "Value")
		}
		return visitor.ActionNoChange, nil
	},
	"StringValue": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.StringValue:
			return visitor.ActionUpdate, `"` + fmt.Sprintf("%v", node.Value) + `"`
		case map[string]interface{}:
			return visitor.ActionUpdate, `"` + getMapValueString(node, "Value") + `"`
		}
		return visitor.ActionNoChange, nil
	},
	"BooleanValue": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.BooleanValue:
			return visitor.ActionUpdate, fmt.Sprintf("%v", node.Value)
		case map[string]interface{}:
			return visitor.ActionUpdate, getMapValueString(node, "Value")
		}
		return visitor.ActionNoChange, nil
	},
	"EnumValue": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.EnumValue:
			return visitor.ActionUpdate, fmt.Sprintf("%v", node.Value)
		case map[string]interface{}:
			return visitor.ActionUpdate, getMapValueString(node, "Value")
		}
		return visitor.ActionNoChange, nil
	},
	"ListValue": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.ListValue:
			return visitor.ActionUpdate, "[" + join(toSliceString(node.Values), ", ") + "]"
		case map[string]interface{}:
			return visitor.ActionUpdate, "[" + join(toSliceString(getMapValue(node, "Values")), ", ") + "]"
		}
		return visitor.ActionNoChange, nil
	},
	"ObjectValue": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.ObjectValue:
			return visitor.ActionUpdate, "{" + join(toSliceString(node.Fields), ", ") + "}"
		case map[string]interface{}:
			return visitor.ActionUpdate, "{" + join(toSliceString(getMapValue(node, "Fields")), ", ") + "}"
		}
		return visitor.ActionNoChange, nil
	},
	"ObjectField": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.ObjectField:
			name := fmt.Sprintf("%v", node.Name)
			value := fmt.Sprintf("%v", node.Value)
			return visitor.ActionUpdate, name + ": " + value
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			value := getMapValueString(node, "Value")
			return visitor.ActionUpdate, name + ": " + value
		}
		return visitor.ActionNoChange, nil
	},

	// Directive
	"Directive": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.Directive:
			name := fmt.Sprintf("%v", node.Name)
			args := toSliceString(node.Arguments)
			return visitor.ActionUpdate, "@" + name + wrap("(", join(args, ", "), ")")
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			args := toSliceString(getMapValue(node, "Arguments"))
			return visitor.ActionUpdate, "@" + name + wrap("(", join(args, ", "), ")")
		}
		return visitor.ActionNoChange, nil
	},

	// Type
	"Named": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.Named:
			return visitor.ActionUpdate, fmt.Sprintf("%v", node.Name)
		case map[string]interface{}:
			return visitor.ActionUpdate, getMapValueString(node, "Name")
		}
		return visitor.ActionNoChange, nil
	},
	"List": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.List:
			return visitor.ActionUpdate, "[" + fmt.Sprintf("%v", node.Type) + "]"
		case map[string]interface{}:
			return visitor.ActionUpdate, "[" + getMapValueString(node, "Type") + "]"
		}
		return visitor.ActionNoChange, nil
	},
	"NonNull": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.NonNull:
			return visitor.ActionUpdate, fmt.Sprintf("%v", node.Type) + "!"
		case map[string]interface{}:
			return visitor.ActionUpdate, getMapValueString(node, "Type") + "!"
		}
		return visitor.ActionNoChange, nil
	},

	// Type System Definitions
	"SchemaDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.SchemaDefinition:
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				"schema",
				join(directives, " "),
				block(node.OperationTypes),
			}, " ")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			operationTypes := toSliceString(getMapValue(node, "OperationTypes"))
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				"schema",
				join(directives, " "),
				block(operationTypes),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"OperationTypeDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.OperationTypeDefinition:
			str := fmt.Sprintf("%v: %v", node.Operation, node.Type)
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			operation := getMapValueString(node, "Operation")
			ttype := getMapValueString(node, "Type")
			str := fmt.Sprintf("%v: %v", operation, ttype)
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"ScalarDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.ScalarDefinition:
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				"scalar",
				fmt.Sprintf("%v", node.Name),
				join(directives, " "),
			}, " ")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				"scalar",
				name,
				join(directives, " "),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"ObjectDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.ObjectDefinition:
			name := fmt.Sprintf("%v", node.Name)
			interfaces := toSliceString(node.Interfaces)
			fields := node.Fields
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				"type",
				name,
				wrap("implements ", join(interfaces, " & "), ""),
				join(directives, " "),
				block(fields),
			}, " ")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			interfaces := toSliceString(getMapValue(node, "Interfaces"))
			fields := getMapValue(node, "Fields")
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				"type",
				name,
				wrap("implements ", join(interfaces, " & "), ""),
				join(directives, " "),
				block(fields),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"FieldDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.FieldDefinition:
			name := fmt.Sprintf("%v", node.Name)
			ttype := fmt.Sprintf("%v", node.Type)
			args := toSliceString(node.Arguments)
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := name + wrap("(", join(args, ", "), ")") + ": " + ttype + wrap(" ", join(directives, " "), "")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			ttype := getMapValueString(node, "Type")
			args := toSliceString(getMapValue(node, "Arguments"))
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := name + wrap("(", join(args, ", "), ")") + ": " + ttype + wrap(" ", join(directives, " "), "")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"InputValueDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.InputValueDefinition:
			name := fmt.Sprintf("%v", node.Name)
			ttype := fmt.Sprintf("%v", node.Type)
			defaultValue := fmt.Sprintf("%v", node.DefaultValue)
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				name + ": " + ttype,
				wrap("= ", defaultValue, ""),
				join(directives, " "),
			}, " ")

			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			ttype := getMapValueString(node, "Type")
			defaultValue := getMapValueString(node, "DefaultValue")
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				name + ": " + ttype,
				wrap("= ", defaultValue, ""),
				join(directives, " "),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"InterfaceDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.InterfaceDefinition:
			name := fmt.Sprintf("%v", node.Name)
			fields := node.Fields
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				"interface",
				name,
				join(directives, " "),
				block(fields),
			}, " ")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			fields := getMapValue(node, "Fields")
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				"interface",
				name,
				join(directives, " "),
				block(fields),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"UnionDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.UnionDefinition:
			name := fmt.Sprintf("%v", node.Name)
			types := toSliceString(node.Types)
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				"union",
				name,
				join(directives, " "),
				"= " + join(types, " | "),
			}, " ")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			types := toSliceString(getMapValue(node, "Types"))
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				"union",
				name,
				join(directives, " "),
				"= " + join(types, " | "),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"EnumDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.EnumDefinition:
			name := fmt.Sprintf("%v", node.Name)
			values := node.Values
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				"enum",
				name,
				join(directives, " "),
				block(values),
			}, " ")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			values := getMapValue(node, "Values")
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				"enum",
				name,
				join(directives, " "),
				block(values),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"EnumValueDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.EnumValueDefinition:
			name := fmt.Sprintf("%v", node.Name)
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				name,
				join(directives, " "),
			}, " ")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				name,
				join(directives, " "),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"InputObjectDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.InputObjectDefinition:
			name := fmt.Sprintf("%v", node.Name)
			fields := node.Fields
			directives := []string{}
			for _, directive := range node.Directives {
				directives = append(directives, fmt.Sprintf("%v", directive.Name))
			}
			str := join([]string{
				"input",
				name,
				join(directives, " "),
				block(fields),
			}, " ")
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			fields := getMapValue(node, "Fields")
			directives := []string{}
			for _, directive := range getMapSliceValue(node, "Directives") {
				directives = append(directives, fmt.Sprintf("%v", directive))
			}
			str := join([]string{
				"input",
				name,
				join(directives, " "),
				block(fields),
			}, " ")
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"TypeExtensionDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.TypeExtensionDefinition:
			definition := fmt.Sprintf("%v", node.Definition)
			str := "extend " + definition
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			definition := getMapValueString(node, "Definition")
			str := "extend " + definition
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
	"DirectiveDefinition": func(p visitor.VisitFuncParams) (string, interface{}) {
		switch node := p.Node.(type) {
		case *ast.DirectiveDefinition:
			args := wrap("(", join(toSliceString(node.Arguments), ", "), ")")
			str := fmt.Sprintf("directive @%v%v on %v", node.Name, args, join(toSliceString(node.Locations), " | "))
			return visitor.ActionUpdate, str
		case map[string]interface{}:
			name := getMapValueString(node, "Name")
			locations := toSliceString(getMapValue(node, "Locations"))
			args := toSliceString(getMapValue(node, "Arguments"))
			argsStr := wrap("(", join(args, ", "), ")")
			str := fmt.Sprintf("directive @%v%v on %v", name, argsStr, join(locations, " | "))
			return visitor.ActionUpdate, str
		}
		return visitor.ActionNoChange, nil
	},
}

func Print(astNode ast.Node) (printed interface{}) {
	defer func() interface{} {
		if r := recover(); r != nil {
			return fmt.Sprintf("%v", astNode)
		}
		return printed
	}()
	printed = visitor.Visit(astNode, &visitor.VisitorOptions{
		LeaveKindMap: printDocASTReducer,
	}, nil)
	return printed
}
