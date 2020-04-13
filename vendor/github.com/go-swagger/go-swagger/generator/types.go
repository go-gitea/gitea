// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	"fmt"
	"log"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
	"github.com/kr/pretty"
)

const (
	iface   = "interface{}"
	array   = "array"
	file    = "file"
	number  = "number"
	integer = "integer"
	boolean = "boolean"
	str     = "string"
	object  = "object"
	binary  = "binary"
	sHTTP   = "http"
	body    = "body"
)

// Extensions supported by go-swagger
const (
	xClass       = "x-class"         // class name used by discriminator
	xGoCustomTag = "x-go-custom-tag" // additional tag for serializers on struct fields
	xGoName      = "x-go-name"       // name of the generated go variable
	xGoType      = "x-go-type"       // reuse existing type (do not generate)
	xIsNullable  = "x-isnullable"
	xNullable    = "x-nullable" // turns the schema into a pointer
	xOmitEmpty   = "x-omitempty"
	xSchemes     = "x-schemes" // additional schemes supported for operations (server generation)
	xOrder       = "x-order"   // sort order for properties (or any schema)
)

// swaggerTypeMapping contains a mapping from go type to swagger type or format
var swaggerTypeName map[string]string

func init() {
	swaggerTypeName = make(map[string]string)
	for k, v := range typeMapping {
		swaggerTypeName[v] = k
	}
}

func simpleResolvedType(tn, fmt string, items *spec.Items) (result resolvedType) {
	result.SwaggerType = tn
	result.SwaggerFormat = fmt

	if tn == file {
		// special case of swagger type "file", rendered as io.ReadCloser interface
		result.IsPrimitive = true
		result.GoType = formatMapping[str][binary]
		result.IsStream = true
		return
	}

	if fmt != "" {
		fmtn := strings.Replace(fmt, "-", "", -1)
		if fmm, ok := formatMapping[tn]; ok {
			if tpe, ok := fmm[fmtn]; ok {
				result.GoType = tpe
				result.IsPrimitive = true
				_, result.IsCustomFormatter = customFormatters[tpe]
				// special case of swagger format "binary", rendered as io.ReadCloser interface
				// TODO(fredbi): should set IsCustomFormatter=false when binary
				result.IsStream = fmt == binary
				return
			}
		}
	}

	if tpe, ok := typeMapping[tn]; ok {
		result.GoType = tpe
		_, result.IsPrimitive = primitives[tpe]
		result.IsPrimitive = ok
		return
	}

	if tn == array {
		result.IsArray = true
		result.IsPrimitive = false
		result.IsCustomFormatter = false
		result.IsNullable = false
		if items == nil {
			result.GoType = "[]" + iface
			return
		}
		res := simpleResolvedType(items.Type, items.Format, items.Items)
		result.GoType = "[]" + res.GoType
		return
	}
	result.GoType = tn
	_, result.IsPrimitive = primitives[tn]
	return
}

func typeForHeader(header spec.Header) resolvedType {
	return simpleResolvedType(header.Type, header.Format, header.Items)
}

func newTypeResolver(pkg string, doc *loads.Document) *typeResolver {
	resolver := typeResolver{ModelsPackage: pkg, Doc: doc}
	resolver.KnownDefs = make(map[string]struct{}, len(doc.Spec().Definitions))
	for k, sch := range doc.Spec().Definitions {
		tpe, _, _ := knownDefGoType(k, sch, nil)
		resolver.KnownDefs[tpe] = struct{}{}
	}
	return &resolver
}

// knownDefGoType returns go type, package and package alias for definition
func knownDefGoType(def string, schema spec.Schema, clear func(string) string) (string, string, string) {
	debugLog("known def type: %q", def)
	ext := schema.Extensions
	if nm, ok := ext.GetString(xGoName); ok {
		if clear == nil {
			debugLog("known def type %s no clear: %q", xGoName, nm)
			return nm, "", ""
		}
		debugLog("known def type %s clear: %q -> %q", xGoName, nm, clear(nm))
		return clear(nm), "", ""
	}
	v, ok := ext[xGoType]
	if !ok {
		if clear == nil {
			debugLog("known def type no clear: %q", def)
			return def, "", ""
		}
		debugLog("known def type clear: %q -> %q", def, clear(def))
		return clear(def), "", ""
	}
	xt := v.(map[string]interface{})
	t := xt["type"].(string)
	impIface, ok := xt["import"]

	if !ok {
		return t, "", ""
	}

	imp := impIface.(map[string]interface{})
	pkg := imp["package"].(string)
	al, ok := imp["alias"]
	var alias string
	if ok {
		alias = al.(string)
	} else {
		alias = path.Base(pkg)
	}
	debugLog("known def type %s no clear: %q: pkg=%s, alias=%s", xGoType, alias+"."+t, pkg, alias)
	return alias + "." + t, pkg, alias
}

type typeResolver struct {
	Doc           *loads.Document
	ModelsPackage string
	ModelName     string
	KnownDefs     map[string]struct{}
	// unexported fields
	keepDefinitionsPkg string
	knownDefsKept      map[string]struct{}
}

// NewWithModelName clones a type resolver and specifies a new model name
func (t *typeResolver) NewWithModelName(name string) *typeResolver {
	tt := newTypeResolver(t.ModelsPackage, t.Doc)
	tt.ModelName = name

	// propagates kept definitions
	tt.keepDefinitionsPkg = t.keepDefinitionsPkg
	tt.knownDefsKept = t.knownDefsKept
	return tt
}

// withKeepDefinitionsPackage instructs the type resolver to keep previously resolved package name for
// definitions known at the moment it is first called.
func (t *typeResolver) withKeepDefinitionsPackage(definitionsPackage string) *typeResolver {
	t.keepDefinitionsPkg = definitionsPackage
	t.knownDefsKept = make(map[string]struct{}, len(t.KnownDefs))
	for k := range t.KnownDefs {
		t.knownDefsKept[k] = struct{}{}
	}
	return t
}

// IsNullable hints the generator as to render the type with a pointer or not.
//
// A schema is deemed nullable (i.e. rendered by a pointer) when:
// - a custom extension says it has to be so
// - it is an object with properties
// - it is a composed object (allOf)
//
// The interpretation of Required as a mean to make a type nullable is carried on elsewhere.
func (t *typeResolver) IsNullable(schema *spec.Schema) bool {
	nullable := t.isNullable(schema)
	return nullable || len(schema.AllOf) > 0
}

func (t *typeResolver) resolveSchemaRef(schema *spec.Schema, isRequired bool) (returns bool, result resolvedType, err error) {
	if schema.Ref.String() != "" {
		debugLog("resolving ref (anon: %t, req: %t) %s", false, isRequired, schema.Ref.String())
		returns = true
		var ref *spec.Schema
		var er error

		ref, er = spec.ResolveRef(t.Doc.Spec(), &schema.Ref)
		if er != nil {
			debugLog("error resolving ref %s: %v", schema.Ref.String(), er)
			err = er
			return
		}
		res, er := t.ResolveSchema(ref, false, isRequired)
		if er != nil {
			err = er
			return
		}
		result = res

		tn := filepath.Base(schema.Ref.GetURL().Fragment)
		tpe, pkg, alias := knownDefGoType(tn, *ref, t.goTypeName)
		debugLog("type name %s, package %s, alias %s", tpe, pkg, alias)
		if tpe != "" {
			result.GoType = tpe
			result.Pkg = pkg
			result.PkgAlias = alias
		}
		result.HasDiscriminator = res.HasDiscriminator
		result.IsBaseType = result.HasDiscriminator
		result.IsNullable = t.IsNullable(ref)
		//result.IsAliased = true
		return

	}
	return
}

func (t *typeResolver) inferAliasing(result *resolvedType, schema *spec.Schema, isAnonymous bool, isRequired bool) {
	if !isAnonymous && t.ModelName != "" {
		result.AliasedType = result.GoType
		result.IsAliased = true
		result.GoType = t.goTypeName(t.ModelName)
	}
}

func (t *typeResolver) resolveFormat(schema *spec.Schema, isAnonymous bool, isRequired bool) (returns bool, result resolvedType, err error) {

	if schema.Format != "" {
		// defaults to string
		result.SwaggerType = str
		if len(schema.Type) > 0 {
			result.SwaggerType = schema.Type[0]
		}

		debugLog("resolving format (anon: %t, req: %t)", isAnonymous, isRequired)
		schFmt := strings.Replace(schema.Format, "-", "", -1)
		if fmm, ok := formatMapping[result.SwaggerType]; ok {
			if tpe, ok := fmm[schFmt]; ok {
				returns = true
				result.GoType = tpe
				_, result.IsCustomFormatter = customFormatters[tpe]
			}
		}
		if tpe, ok := typeMapping[schFmt]; !returns && ok {
			returns = true
			result.GoType = tpe
			_, result.IsCustomFormatter = customFormatters[tpe]
		}

		result.SwaggerFormat = schema.Format
		t.inferAliasing(&result, schema, isAnonymous, isRequired)
		// special case of swagger format "binary", rendered as io.ReadCloser interface and is therefore not a primitive type
		// TODO: should set IsCustomFormatter=false in this case.
		result.IsPrimitive = schFmt != binary
		result.IsStream = schFmt == binary
		// propagate extensions in resolvedType
		result.Extensions = schema.Extensions

		switch result.SwaggerType {
		case str:
			result.IsNullable = nullableStrfmt(schema, isRequired)
		case number, integer:
			result.IsNullable = nullableNumber(schema, isRequired)
		default:
			result.IsNullable = t.IsNullable(schema)
		}
	}
	return
}

func (t *typeResolver) isNullable(schema *spec.Schema) bool {
	check := func(extension string) (bool, bool) {
		v, found := schema.Extensions[extension]
		nullable, cast := v.(bool)
		return nullable, found && cast
	}

	if nullable, ok := check(xIsNullable); ok {
		return nullable
	}
	if nullable, ok := check(xNullable); ok {
		return nullable
	}
	return len(schema.Properties) > 0
}

func setIsEmptyOmitted(result *resolvedType, schema *spec.Schema, tpe string) {
	defaultValue := true
	if tpe == array {
		defaultValue = false
	}
	v, found := schema.Extensions[xOmitEmpty]
	if !found {
		result.IsEmptyOmitted = defaultValue
		return
	}

	omitted, cast := v.(bool)
	result.IsEmptyOmitted = omitted && cast
}

func (t *typeResolver) firstType(schema *spec.Schema) string {
	if len(schema.Type) == 0 || schema.Type[0] == "" {
		return object
	}
	if len(schema.Type) > 1 {
		// JSON-Schema multiple types, e.g. {"type": [ "object", "array" ]} are not supported.
		// TODO: should keep the first _supported_ type, e.g. skip null
		log.Printf("warning: JSON-Schema type definition as array with several types is not supported in %#v. Taking the first type: %s", schema.Type, schema.Type[0])
	}
	return schema.Type[0]
}

func (t *typeResolver) resolveArray(schema *spec.Schema, isAnonymous, isRequired bool) (result resolvedType, err error) {
	debugLog("resolving array (anon: %t, req: %t)", isAnonymous, isRequired)

	result.IsArray = true
	result.IsNullable = false

	if schema.AdditionalItems != nil {
		result.HasAdditionalItems = (schema.AdditionalItems.Allows || schema.AdditionalItems.Schema != nil)
	}

	if schema.Items == nil {
		result.GoType = "[]" + iface
		result.SwaggerType = array
		result.SwaggerFormat = ""
		t.inferAliasing(&result, schema, isAnonymous, isRequired)

		return
	}

	if len(schema.Items.Schemas) > 0 {
		result.IsArray = false
		result.IsTuple = true
		result.SwaggerType = array
		result.SwaggerFormat = ""
		t.inferAliasing(&result, schema, isAnonymous, isRequired)

		return
	}

	rt, er := t.ResolveSchema(schema.Items.Schema, true, false)
	if er != nil {
		err = er
		return
	}
	// override the general nullability rule from ResolveSchema():
	// only complex items are nullable (when not discriminated, not forced by x-nullable)
	rt.IsNullable = t.IsNullable(schema.Items.Schema) && !rt.HasDiscriminator
	result.GoType = "[]" + rt.GoType
	if rt.IsNullable && !strings.HasPrefix(rt.GoType, "*") {
		result.GoType = "[]*" + rt.GoType
	}

	result.ElemType = &rt
	result.SwaggerType = array
	result.SwaggerFormat = ""
	t.inferAliasing(&result, schema, isAnonymous, isRequired)
	result.Extensions = schema.Extensions

	return
}

func (t *typeResolver) goTypeName(nm string) string {
	if len(t.knownDefsKept) > 0 {
		// if a definitions package has been defined, already resolved definitions are
		// always resolved against their original package (e.g. "models"), and not the
		// current package.
		// This allows complex anonymous extra schemas to reuse known definitions generated in another package.
		if _, ok := t.knownDefsKept[nm]; ok {
			return strings.Join([]string{t.keepDefinitionsPkg, swag.ToGoName(nm)}, ".")
		}
	}

	if t.ModelsPackage == "" {
		return swag.ToGoName(nm)
	}
	if _, ok := t.KnownDefs[nm]; ok {
		return strings.Join([]string{t.ModelsPackage, swag.ToGoName(nm)}, ".")
	}
	return swag.ToGoName(nm)
}

func (t *typeResolver) resolveObject(schema *spec.Schema, isAnonymous bool) (result resolvedType, err error) {
	debugLog("resolving object %s (anon: %t, req: %t)", t.ModelName, isAnonymous, false)

	result.IsAnonymous = isAnonymous

	result.IsBaseType = schema.Discriminator != ""
	if !isAnonymous {
		result.SwaggerType = object
		tpe, pkg, alias := knownDefGoType(t.ModelName, *schema, t.goTypeName)
		result.GoType = tpe
		result.Pkg = pkg
		result.PkgAlias = alias
	}
	if len(schema.AllOf) > 0 {
		result.GoType = t.goTypeName(t.ModelName)
		result.IsComplexObject = true
		var isNullable bool
		for _, p := range schema.AllOf {
			if t.IsNullable(&p) {
				isNullable = true
			}
		}
		result.IsNullable = isNullable
		result.SwaggerType = object
		return
	}

	// if this schema has properties, build a map of property name to
	// resolved type, this should also flag the object as anonymous,
	// when a ref is found, the anonymous flag will be reset
	if len(schema.Properties) > 0 {
		result.IsNullable = t.IsNullable(schema)
		result.IsComplexObject = true
		// no return here, still need to check for additional properties
	}

	// account for additional properties
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		sch := schema.AdditionalProperties.Schema
		et, er := t.ResolveSchema(sch, sch.Ref.String() == "", false)
		if er != nil {
			err = er
			return
		}

		result.IsMap = !result.IsComplexObject

		result.SwaggerType = object

		// only complex map elements are nullable (when not forced by x-nullable)
		// TODO: figure out if required to check when not discriminated like arrays?
		et.IsNullable = t.isNullable(schema.AdditionalProperties.Schema)
		if et.IsNullable {
			result.GoType = "map[string]*" + et.GoType
		} else {
			result.GoType = "map[string]" + et.GoType
		}

		// Resolving nullability conflicts for:
		// - map[][]...[]{items}
		// - map[]{aliased type}
		//
		// when IsMap is true and the type is a distinct definition,
		// aliased type or anonymous construct generated independently.
		//
		// IsMapNullOverride is to be handled by the generator for special cases
		// where the map element is considered non nullable and the element itself is.
		//
		// This allows to appreciate nullability according to the context
		needsOverride := result.IsMap && (et.IsArray || (sch.Ref.String() != "" || et.IsAliased || et.IsAnonymous))

		if needsOverride {
			var er error
			if et.IsArray {
				var it resolvedType
				s := sch
				// resolve the last items after nested arrays
				for s.Items != nil && s.Items.Schema != nil {
					it, er = t.ResolveSchema(s.Items.Schema, sch.Ref.String() == "", false)
					if er != nil {
						return
					}
					s = s.Items.Schema
				}
				// mark an override when nullable status conflicts, i.e. when the original type is not already nullable
				if !it.IsAnonymous || it.IsAnonymous && it.IsNullable {
					result.IsMapNullOverride = true
				}
			} else {
				// this locks the generator on the local nullability status
				result.IsMapNullOverride = true
			}
		}

		t.inferAliasing(&result, schema, isAnonymous, false)
		result.ElemType = &et
		return
	}

	if len(schema.Properties) > 0 {
		return
	}

	// an object without property and without AdditionalProperties schema is rendered as interface{}
	result.GoType = iface
	result.IsMap = true
	result.SwaggerType = object
	result.IsNullable = false
	result.IsInterface = len(schema.Properties) == 0
	return
}

// nullableBool makes a boolean a pointer when we want to distinguish the zero value from no value set.
// This is the case when:
// - a x-nullable extension says so in the spec
// - it is **not** a read-only property
// - it is a required property
// - it has a default value
func nullableBool(schema *spec.Schema, isRequired bool) bool {
	if nullable := nullableExtension(schema.Extensions); nullable != nil {
		return *nullable
	}
	required := isRequired && schema.Default == nil && !schema.ReadOnly
	optional := !isRequired && (schema.Default != nil || schema.ReadOnly)

	return required || optional
}

// nullableNumber makes a number a pointer when we want to distinguish the zero value from no value set.
// This is the case when:
// - a x-nullable extension says so in the spec
// - it is **not** a read-only property
// - it is a required property
// - boundaries defines the zero value as a valid value:
//   - there is a non-exclusive boundary set at the zero value of the type
//   - the [min,max] range crosses the zero value of the type
func nullableNumber(schema *spec.Schema, isRequired bool) bool {
	if nullable := nullableExtension(schema.Extensions); nullable != nil {
		return *nullable
	}
	hasDefault := schema.Default != nil && !swag.IsZero(schema.Default)

	isMin := schema.Minimum != nil && (*schema.Minimum != 0 || schema.ExclusiveMinimum)
	bcMin := schema.Minimum != nil && *schema.Minimum == 0 && !schema.ExclusiveMinimum
	isMax := schema.Minimum == nil && (schema.Maximum != nil && (*schema.Maximum != 0 || schema.ExclusiveMaximum))
	bcMax := schema.Maximum != nil && *schema.Maximum == 0 && !schema.ExclusiveMaximum
	isMinMax := (schema.Minimum != nil && schema.Maximum != nil && *schema.Minimum < *schema.Maximum)
	bcMinMax := (schema.Minimum != nil && schema.Maximum != nil && (*schema.Minimum < 0 && 0 < *schema.Maximum))

	nullable := !schema.ReadOnly && (isRequired || (hasDefault && !(isMin || isMax || isMinMax)) || bcMin || bcMax || bcMinMax)
	return nullable
}

// nullableString makes a string nullable when we want to distinguish the zero value from no value set.
// This is the case when:
// - a x-nullable extension says so in the spec
// - it is **not** a read-only property
// - it is a required property
// - it has a MinLength property set to 0
// - it has a default other than "" (the zero for strings) and no MinLength or zero MinLength
func nullableString(schema *spec.Schema, isRequired bool) bool {
	if nullable := nullableExtension(schema.Extensions); nullable != nil {
		return *nullable
	}
	hasDefault := schema.Default != nil && !swag.IsZero(schema.Default)

	isMin := schema.MinLength != nil && *schema.MinLength != 0
	bcMin := schema.MinLength != nil && *schema.MinLength == 0

	nullable := !schema.ReadOnly && (isRequired || (hasDefault && !isMin) || bcMin)
	return nullable
}

func nullableStrfmt(schema *spec.Schema, isRequired bool) bool {
	notBinary := schema.Format != binary
	if nullable := nullableExtension(schema.Extensions); nullable != nil && notBinary {
		return *nullable
	}
	hasDefault := schema.Default != nil && !swag.IsZero(schema.Default)

	nullable := !schema.ReadOnly && (isRequired || hasDefault)
	return notBinary && nullable
}

func nullableExtension(ext spec.Extensions) *bool {
	if ext == nil {
		return nil
	}

	if boolPtr := boolExtension(ext, xNullable); boolPtr != nil {
		return boolPtr
	}

	return boolExtension(ext, xIsNullable)
}

func boolExtension(ext spec.Extensions, key string) *bool {
	if v, ok := ext[key]; ok {
		if bb, ok := v.(bool); ok {
			return &bb
		}
	}
	return nil
}

func (t *typeResolver) ResolveSchema(schema *spec.Schema, isAnonymous, isRequired bool) (result resolvedType, err error) {
	debugLog("resolving schema (anon: %t, req: %t) %s", isAnonymous, isRequired, t.ModelName)
	if schema == nil {
		result.IsInterface = true
		result.GoType = iface
		return
	}

	tpe := t.firstType(schema)
	defer setIsEmptyOmitted(&result, schema, tpe)

	var returns bool
	returns, result, err = t.resolveSchemaRef(schema, isRequired)
	if returns {
		if !isAnonymous {
			result.IsMap = false
			result.IsComplexObject = true
			debugLog("not anonymous ref")
		}
		debugLog("returning after ref")
		return
	}

	// special case of swagger type "file", rendered as io.ReadCloser interface
	if t.firstType(schema) == file {
		result.SwaggerType = file
		result.IsPrimitive = true
		result.IsNullable = false
		result.GoType = formatMapping[str][binary]
		result.IsStream = true
		return
	}

	returns, result, err = t.resolveFormat(schema, isAnonymous, isRequired)
	if returns {
		debugLog("returning after resolve format: %s", pretty.Sprint(result))
		return
	}

	result.IsNullable = t.isNullable(schema) || isRequired

	switch tpe {
	case array:
		result, err = t.resolveArray(schema, isAnonymous, false)
		return

	case file, number, integer, boolean:
		result.Extensions = schema.Extensions
		result.GoType = typeMapping[tpe]
		result.SwaggerType = tpe
		t.inferAliasing(&result, schema, isAnonymous, isRequired)

		switch tpe {
		case boolean:
			result.IsPrimitive = true
			result.IsCustomFormatter = false
			result.IsNullable = nullableBool(schema, isRequired)
		case number, integer:
			result.IsPrimitive = true
			result.IsCustomFormatter = false
			result.IsNullable = nullableNumber(schema, isRequired)
		case file:
		}
		return

	case str:
		result.GoType = str
		result.SwaggerType = str
		t.inferAliasing(&result, schema, isAnonymous, isRequired)

		result.IsPrimitive = true
		result.IsNullable = nullableString(schema, isRequired)
		result.Extensions = schema.Extensions

	case object:
		result, err = t.resolveObject(schema, isAnonymous)
		if err != nil {
			return resolvedType{}, err
		}
		result.HasDiscriminator = schema.Discriminator != ""
		return

	case "null":
		result.GoType = iface
		result.SwaggerType = object
		result.IsNullable = false
		result.IsInterface = true
		return

	default:
		err = fmt.Errorf("unresolvable: %v (format %q)", schema.Type, schema.Format)
		return
	}
	return result, err
}

// resolvedType is a swagger type that has been resolved and analyzed for usage
// in a template
type resolvedType struct {
	IsAnonymous       bool
	IsArray           bool
	IsMap             bool
	IsInterface       bool
	IsPrimitive       bool
	IsCustomFormatter bool
	IsAliased         bool
	IsNullable        bool
	IsStream          bool
	IsEmptyOmitted    bool

	// A tuple gets rendered as an anonymous struct with P{index} as property name
	IsTuple            bool
	HasAdditionalItems bool

	// A complex object gets rendered as a struct
	IsComplexObject bool

	// A polymorphic type
	IsBaseType       bool
	HasDiscriminator bool

	GoType        string
	Pkg           string
	PkgAlias      string
	AliasedType   string
	SwaggerType   string
	SwaggerFormat string
	Extensions    spec.Extensions

	// The type of the element in a slice or map
	ElemType *resolvedType

	// IsMapNullOverride indicates that a nullable object is used within an
	// aliased map. In this case, the reference is not rendered with a pointer
	IsMapNullOverride bool

	// IsSuperAlias indicates that the aliased type is really the same type,
	// e.g. in golang, this translates to: type A = B
	IsSuperAlias bool
}

func (rt *resolvedType) Zero() string {
	// if type is aliased, provide zero from the aliased type
	if rt.IsAliased {
		if zr, ok := zeroes[rt.AliasedType]; ok {
			return rt.GoType + "(" + zr + ")"
		}
	}
	// zero function provided as native or by strfmt function
	if zr, ok := zeroes[rt.GoType]; ok {
		return zr
	}
	// map and slice initializer
	if rt.IsMap {
		return "make(" + rt.GoType + ", 50)"
	} else if rt.IsArray {
		return "make(" + rt.GoType + ", 0, 50)"
	}
	// object initializer
	if rt.IsTuple || rt.IsComplexObject {
		if rt.IsNullable {
			return "new(" + rt.GoType + ")"
		}
		return rt.GoType + "{}"
	}
	// interface initializer
	if rt.IsInterface {
		return "nil"
	}

	return ""
}
