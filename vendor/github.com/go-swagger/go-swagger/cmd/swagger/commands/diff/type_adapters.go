package diff

import (
	"github.com/go-openapi/spec"
)

func forItems(items *spec.Items) *spec.Schema {
	if items == nil {
		return nil
	}
	valids := items.CommonValidations
	schema := spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:             []string{items.SimpleSchema.Type},
			Format:           items.SimpleSchema.Format,
			Maximum:          valids.Maximum,
			ExclusiveMaximum: valids.ExclusiveMaximum,
			Minimum:          valids.Minimum,
			ExclusiveMinimum: valids.ExclusiveMinimum,
			MaxLength:        valids.MaxLength,
			MinLength:        valids.MinLength,
			Pattern:          valids.Pattern,
			MaxItems:         valids.MaxItems,
			MinItems:         valids.MinItems,
			UniqueItems:      valids.UniqueItems,
			MultipleOf:       valids.MultipleOf,
			Enum:             valids.Enum,
		},
	}
	return &schema
}

func forHeader(header spec.Header) *spec.SchemaProps {
	return &spec.SchemaProps{
		Type:             []string{header.Type},
		Format:           header.Format,
		Items:            &spec.SchemaOrArray{Schema: forItems(header.Items)},
		Maximum:          header.Maximum,
		ExclusiveMaximum: header.ExclusiveMaximum,
		Minimum:          header.Minimum,
		ExclusiveMinimum: header.ExclusiveMinimum,
		MaxLength:        header.MaxLength,
		MinLength:        header.MinLength,
		Pattern:          header.Pattern,
		MaxItems:         header.MaxItems,
		MinItems:         header.MinItems,
		UniqueItems:      header.UniqueItems,
		MultipleOf:       header.MultipleOf,
		Enum:             header.Enum,
	}
}

func forParam(param spec.Parameter) *spec.SchemaProps {
	return &spec.SchemaProps{
		Type:             []string{param.Type},
		Format:           param.Format,
		Items:            &spec.SchemaOrArray{Schema: forItems(param.Items)},
		Maximum:          param.Maximum,
		ExclusiveMaximum: param.ExclusiveMaximum,
		Minimum:          param.Minimum,
		ExclusiveMinimum: param.ExclusiveMinimum,
		MaxLength:        param.MaxLength,
		MinLength:        param.MinLength,
		Pattern:          param.Pattern,
		MaxItems:         param.MaxItems,
		MinItems:         param.MinItems,
		UniqueItems:      param.UniqueItems,
		MultipleOf:       param.MultipleOf,
		Enum:             param.Enum,
	}
}

// OperationMap saves indexing operations in PathItems individually
type OperationMap map[string]*spec.Operation

func toMap(item *spec.PathItem) OperationMap {
	m := make(OperationMap)

	if item.Post != nil {
		m["post"] = item.Post
	}
	if item.Get != nil {
		m["get"] = item.Get
	}
	if item.Put != nil {
		m["put"] = item.Put
	}
	if item.Patch != nil {
		m["patch"] = item.Patch
	}
	if item.Head != nil {
		m["head"] = item.Head
	}
	if item.Options != nil {
		m["options"] = item.Options
	}
	if item.Delete != nil {
		m["delete"] = item.Delete
	}
	return m
}

func getURLMethodsFor(spec *spec.Swagger) URLMethods {
	returnURLMethods := URLMethods{}

	for url, eachPath := range spec.Paths.Paths {
		eachPath := eachPath
		opsMap := toMap(&eachPath)
		for method, op := range opsMap {
			returnURLMethods[URLMethod{url, method}] = &PathItemOp{&eachPath, op}
		}
	}
	return returnURLMethods
}

func isStringType(typeName string) bool {
	return typeName == "string" || typeName == "password"
}

// SchemaFromRefFn define this to get a schema for a ref
type SchemaFromRefFn func(spec.Ref) (*spec.Schema, string)

func propertiesFor(schema *spec.Schema, getRefFn SchemaFromRefFn) PropertyMap {
	if isRefType(schema) {
		schema, _ = getRefFn(schema.Ref)
	}
	props := PropertyMap{}

	requiredProps := schema.Required
	requiredMap := map[string]bool{}
	for _, prop := range requiredProps {
		requiredMap[prop] = true
	}

	if schema.Properties != nil {
		for name, prop := range schema.Properties {
			prop := prop
			required := requiredMap[name]
			props[name] = PropertyDefn{Schema: &prop, Required: required}
		}
	}
	for _, e := range schema.AllOf {
		eachAllOf := e
		allOfMap := propertiesFor(&eachAllOf, getRefFn)
		for name, prop := range allOfMap {
			props[name] = prop
		}
	}
	return props
}

func getRef(item interface{}) spec.Ref {
	switch s := item.(type) {
	case *spec.Refable:
		return s.Ref
	case *spec.Schema:
		return s.Ref
	case *spec.SchemaProps:
		return s.Ref
	default:
		return spec.Ref{}
	}
}
