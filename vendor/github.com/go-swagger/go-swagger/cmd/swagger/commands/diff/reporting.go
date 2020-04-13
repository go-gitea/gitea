package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-openapi/spec"
)

// ArrayType const for array
var ArrayType = "array"

// Compare returns the result of analysing breaking and non breaking changes
// between to Swagger specs
func Compare(spec1, spec2 *spec.Swagger) (diffs SpecDifferences, err error) {
	analyser := NewSpecAnalyser()
	err = analyser.Analyse(spec1, spec2)
	if err != nil {
		return nil, err
	}
	diffs = analyser.Diffs
	return
}

// PathItemOp - combines path and operation into a single keyed entity
type PathItemOp struct {
	ParentPathItem *spec.PathItem  `json:"pathitem"`
	Operation      *spec.Operation `json:"operation"`
}

// URLMethod - combines url and method into a single keyed entity
type URLMethod struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

// DataDirection indicates the direction of change Request vs Response
type DataDirection int

const (
	// Request Used for messages/param diffs in a request
	Request DataDirection = iota
	// Response Used for messages/param diffs in a response
	Response
)

func getParams(pathParams, opParams []spec.Parameter, location string) map[string]spec.Parameter {
	params := map[string]spec.Parameter{}
	// add shared path params
	for _, eachParam := range pathParams {
		if eachParam.In == location {
			params[eachParam.Name] = eachParam
		}
	}
	// add any overridden params
	for _, eachParam := range opParams {
		if eachParam.In == location {
			params[eachParam.Name] = eachParam
		}
	}
	return params
}

func getNameOnlyDiffNode(forLocation string) *Node {
	node := Node{
		Field: forLocation,
	}
	return &node
}

func getSimpleSchemaDiffNode(name string, schema *spec.SimpleSchema) *Node {
	node := Node{
		Field: name,
	}
	if schema != nil {
		node.TypeName, node.IsArray = getSimpleSchemaType(schema)
	}
	return &node
}

func getSchemaDiffNode(name string, schema *spec.Schema) *Node {
	node := Node{
		Field: name,
	}
	if schema != nil {
		node.TypeName, node.IsArray = getSchemaType(&schema.SchemaProps)
	}
	return &node
}

func definitonFromURL(url *url.URL) string {
	if url == nil {
		return ""
	}
	fragmentParts := strings.Split(url.Fragment, "/")
	numParts := len(fragmentParts)
	if numParts == 0 {
		return ""
	}
	return fragmentParts[numParts-1]
}

func getSimpleSchemaType(schema *spec.SimpleSchema) (typeName string, isArray bool) {
	typeName = schema.Type
	if typeName == ArrayType {
		typeName, _ = getSimpleSchemaType(&schema.Items.SimpleSchema)
		return typeName, true
	}
	return typeName, false
}

func getSchemaType(schema *spec.SchemaProps) (typeName string, isArray bool) {
	refStr := definitonFromURL(schema.Ref.GetURL())
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

func primitiveTypeString(typeName, typeFormat string) string {
	if typeFormat != "" {
		return fmt.Sprintf("%s.%s", typeName, typeFormat)
	}
	return typeName
}

// TypeDiff - describes a primitive type change
type TypeDiff struct {
	Change      SpecChangeCode `json:"change-type,omitempty"`
	Description string         `json:"description,omitempty"`
	FromType    string         `json:"from-type,omitempty"`
	ToType      string         `json:"to-type,omitempty"`
}

// didn't use 'width' so as not to confuse with bit width
var numberWideness = map[string]int{
	"number":        3,
	"number.double": 3,
	"double":        3,
	"number.float":  2,
	"float":         2,
	"long":          1,
	"integer.int64": 1,
	"integer":       0,
	"integer.int32": 0,
}

func prettyprint(b []byte) ([]byte, error) {
	var out bytes.Buffer
	err := json.Indent(&out, b, "", "  ")
	return out.Bytes(), err
}

// JSONMarshal allows the item to be correctly rendered to json
func JSONMarshal(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}
