package diff

import (
	"fmt"
	"strings"

	"github.com/go-openapi/spec"
)

func getTypeFromSchema(schema *spec.Schema) (typeName string, isArray bool) {
	refStr := definitionFromRef(schema.Ref)
	if len(refStr) > 0 {
		return refStr, false
	}
	typeName = schema.Type[0]
	if typeName == ArrayType {
		typeName, _ = getSchemaType(&schema.Items.Schema.SchemaProps)
		return typeName, true
	}
	return typeName, false

}

func getTypeFromSimpleSchema(schema *spec.SimpleSchema) (typeName string, isArray bool) {
	typeName = schema.Type
	format := schema.Format
	if len(format) > 0 {
		typeName = fmt.Sprintf("%s.%s", typeName, format)
	}
	if typeName == ArrayType {
		typeName, _ = getSchemaType(&schema.Items.SimpleSchema)
		return typeName, true
	}
	return typeName, false

}

func getTypeFromSchemaProps(schema *spec.SchemaProps) (typeName string, isArray bool) {
	refStr := definitionFromRef(schema.Ref)
	if len(refStr) > 0 {
		return refStr, false
	}
	if len(schema.Type) > 0 {
		typeName = schema.Type[0]
		format := schema.Format
		if len(format) > 0 {
			typeName = fmt.Sprintf("%s.%s", typeName, format)
		}
		if typeName == ArrayType {
			typeName, _ = getSchemaType(&schema.Items.Schema.SchemaProps)
			return typeName, true
		}
	}
	return typeName, false

}

func getSchemaTypeStr(item interface{}) string {
	typeStr, isArray := getSchemaType(item)
	return formatTypeString(typeStr, isArray)
}

func getSchemaType(item interface{}) (typeName string, isArray bool) {

	switch s := item.(type) {
	case *spec.Schema:
		typeName, isArray = getTypeFromSchema(s)
	case *spec.SchemaProps:
		typeName, isArray = getTypeFromSchemaProps(s)
	case spec.SchemaProps:
		typeName, isArray = getTypeFromSchemaProps(&s)
	case spec.SimpleSchema:
		typeName, isArray = getTypeFromSimpleSchema(&s)
	case *spec.SimpleSchema:
		typeName, isArray = getTypeFromSimpleSchema(s)
	default:
		typeName = "unknown"
	}

	return

}

func formatTypeString(typ string, isarray bool) string {
	if isarray {
		return fmt.Sprintf("<array[%s]>", typ)
	}
	return fmt.Sprintf("<%s>", typ)
}

func definitionFromRef(ref spec.Ref) string {
	url := ref.GetURL()
	if url == nil {
		return ""
	}
	fragmentParts := strings.Split(url.Fragment, "/")
	numParts := len(fragmentParts)

	return fragmentParts[numParts-1]
}

func isArray(item interface{}) bool {
	switch s := item.(type) {
	case *spec.Schema:
		return isArrayType(s.Type)
	case *spec.SchemaProps:
		return isArrayType(s.Type)
	case *spec.SimpleSchema:
		return isArrayType(spec.StringOrArray{s.Type})
	default:
		return false
	}
}

func isPrimitive(item interface{}) bool {
	switch s := item.(type) {
	case *spec.Schema:
		return isPrimitiveType(s.Type)
	case *spec.SchemaProps:
		return isPrimitiveType(s.Type)
	case spec.StringOrArray:
		return isPrimitiveType(s)
	default:
		return false
	}
}
