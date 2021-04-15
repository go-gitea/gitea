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
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
	"github.com/kr/pretty"
	"github.com/mitchellh/mapstructure"
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
	body    = "body"
	b64     = "byte"
)

// Extensions supported by go-swagger
const (
	xClass        = "x-class"         // class name used by discriminator
	xGoCustomTag  = "x-go-custom-tag" // additional tag for serializers on struct fields
	xGoName       = "x-go-name"       // name of the generated go variable
	xGoType       = "x-go-type"       // reuse existing type (do not generate)
	xIsNullable   = "x-isnullable"
	xNullable     = "x-nullable" // turns the schema into a pointer
	xOmitEmpty    = "x-omitempty"
	xSchemes      = "x-schemes" // additional schemes supported for operations (server generation)
	xOrder        = "x-order"   // sort order for properties (or any schema)
	xGoJSONString = "x-go-json-string"
	xGoEnumCI     = "x-go-enum-ci" // make string enumeration case-insensitive

	xGoOperationTag = "x-go-operation-tag" // additional tag to override generation in operation groups
)

// swaggerTypeName contains a mapping from go type to swagger type or format
var swaggerTypeName map[string]string

func initTypes() {
	swaggerTypeName = make(map[string]string)
	for k, v := range typeMapping {
		swaggerTypeName[v] = k
	}
}

func simpleResolvedType(tn, fmt string, items *spec.Items, v *spec.CommonValidations) (result resolvedType) {
	result.SwaggerType = tn
	result.SwaggerFormat = fmt

	defer func() {
		guardValidations(result.SwaggerType, v)
	}()

	if tn == file {
		// special case of swagger type "file", rendered as io.ReadCloser interface
		result.IsPrimitive = true
		result.GoType = formatMapping[str][binary]
		result.IsStream = true
		return
	}

	if fmt != "" {
		defer func() {
			guardFormatConflicts(result.SwaggerFormat, v)
		}()

		fmtn := strings.ReplaceAll(fmt, "-", "")
		if fmm, ok := formatMapping[tn]; ok {
			if tpe, ok := fmm[fmtn]; ok {
				result.GoType = tpe
				result.IsPrimitive = true
				_, result.IsCustomFormatter = customFormatters[tpe]
				// special case of swagger format "binary", rendered as io.ReadCloser interface
				// TODO(fredbi): should set IsCustomFormatter=false when binary
				result.IsStream = fmt == binary
				// special case of swagger format "byte", rendered as a strfmt.Base64 type: no validation
				result.IsBase64 = fmt == b64
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
		res := simpleResolvedType(items.Type, items.Format, items.Items, &items.CommonValidations)
		result.GoType = "[]" + res.GoType
		return
	}
	result.GoType = tn
	_, result.IsPrimitive = primitives[tn]
	return
}

func newTypeResolver(pkg, fullPkg string, doc *loads.Document) *typeResolver {
	resolver := typeResolver{ModelsPackage: pkg, Doc: doc}
	resolver.KnownDefs = make(map[string]struct{}, len(doc.Spec().Definitions))
	for k, sch := range doc.Spec().Definitions {
		tpe, _, _ := resolver.knownDefGoType(k, sch, nil)
		resolver.KnownDefs[tpe] = struct{}{}
	}
	return &resolver
}

// knownDefGoType returns go type, package and package alias for definition
func (t typeResolver) knownDefGoType(def string, schema spec.Schema, clear func(string) string) (string, string, string) {
	debugLog("known def type: %q", def)
	ext := schema.Extensions
	nm, hasGoName := ext.GetString(xGoName)

	if hasGoName {
		debugLog("known def type %s named from %s as %q", def, xGoName, nm)
		def = nm
	}
	extType, isExternalType := t.resolveExternalType(ext)
	if !isExternalType || extType.Embedded {
		if clear == nil {
			debugLog("known def type no clear: %q", def)
			return def, "", ""
		}
		debugLog("known def type clear: %q -> %q", def, clear(def))
		return clear(def), "", ""
	}

	// external type definition trumps regular type resolution
	if extType.Import.Alias == "" {
		debugLog("type %s imported as external type %s, assumed in current package", def, extType.Type)
		return extType.Type, extType.Import.Package, extType.Import.Alias
	}
	debugLog("type %s imported as external type from %s as %s.%s", def, extType.Import.Package, extType.Import.Alias, extType.Type)
	return extType.Import.Alias + "." + extType.Type, extType.Import.Package, extType.Import.Alias
}

// x-go-type:
//   type: mytype
//   import:
//     package:
//     alias:
//   hints:
//     kind: map|object|array|interface|primitive|stream|tuple
//     nullable: true|false
//  embedded: true
type externalTypeDefinition struct {
	Type   string
	Import struct {
		Package string
		Alias   string
	}
	Hints struct {
		Kind         string
		Nullable     *bool
		NoValidation *bool
	}
	Embedded bool
}

func hasExternalType(ext spec.Extensions) (*externalTypeDefinition, bool) {
	v, ok := ext[xGoType]
	if !ok {
		return nil, false
	}

	var extType externalTypeDefinition
	err := mapstructure.Decode(v, &extType)
	if err != nil {
		log.Printf("warning: x-go-type extension could not be decoded (%v). Skipped", v)
		return nil, false
	}

	return &extType, true
}

func (t typeResolver) resolveExternalType(ext spec.Extensions) (*externalTypeDefinition, bool) {
	extType, hasExt := hasExternalType(ext)
	if !hasExt {
		return nil, false
	}

	// NOTE:
	// * basic deconfliction of the default alias
	// * if no package is specified, defaults to models (as provided from CLI or defaut generation location for models)
	toAlias := func(pkg string) string {
		mangled := GoLangOpts().ManglePackageName(pkg, "")
		return deconflictPkg(mangled, func(in string) string {
			return in + "ext"
		})
	}

	switch {
	case extType.Import.Package != "" && extType.Import.Alias == "":
		extType.Import.Alias = toAlias(extType.Import.Package)
	case extType.Import.Package == "" && extType.Import.Alias != "":
		extType.Import.Package = t.ModelsFullPkg
	case extType.Import.Package == "" && extType.Import.Alias == "":
		// in this case, the external type is assumed to be present in the current package.
		// For completion, whenever this type is used in anonymous types declared by operations,
		// we assume this is the package where models are expected to be found.
		extType.Import.Package = t.ModelsFullPkg
		if extType.Import.Package != "" {
			extType.Import.Alias = toAlias(extType.Import.Package)
		}
	}

	debugLogAsJSON("known def external %s type", xGoType, extType)

	return extType, true
}

type typeResolver struct {
	Doc           *loads.Document
	ModelsPackage string // package alias (e.g. "models")
	ModelsFullPkg string // fully qualified package (e.g. "github.com/example/models")
	ModelName     string
	KnownDefs     map[string]struct{}
	// unexported fields
	keepDefinitionsPkg string
	knownDefsKept      map[string]struct{}
}

// NewWithModelName clones a type resolver and specifies a new model name
func (t *typeResolver) NewWithModelName(name string) *typeResolver {
	tt := newTypeResolver(t.ModelsPackage, t.ModelsFullPkg, t.Doc)
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

func (t *typeResolver) resolveSchemaRef(schema *spec.Schema, isRequired bool) (returns bool, result resolvedType, err error) {
	if schema.Ref.String() == "" {
		return
	}
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

	extType, isExternalType := t.resolveExternalType(schema.Extensions)
	if isExternalType {
		// deal with validations for an aliased external type
		result.SkipExternalValidation = swag.BoolValue(extType.Hints.NoValidation)
	}

	res, er := t.ResolveSchema(ref, false, isRequired)
	if er != nil {
		err = er
		return
	}
	result = res

	tn := filepath.Base(schema.Ref.GetURL().Fragment)
	tpe, pkg, alias := t.knownDefGoType(tn, *ref, t.goTypeName)
	debugLog("type name %s, package %s, alias %s", tpe, pkg, alias)
	if tpe != "" {
		result.GoType = tpe
		result.Pkg = pkg
		result.PkgAlias = alias
	}
	result.HasDiscriminator = res.HasDiscriminator
	result.IsBaseType = result.HasDiscriminator
	result.IsNullable = result.IsNullable || t.isNullable(ref) // this has to be overriden for slices and maps
	result.IsEnumCI = false
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
		schFmt := strings.ReplaceAll(schema.Format, "-", "")
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
		result.IsBase64 = schFmt == b64
		// propagate extensions in resolvedType
		result.Extensions = schema.Extensions

		switch result.SwaggerType {
		case str:
			result.IsNullable = nullableStrfmt(schema, isRequired)
		case number, integer:
			result.IsNullable = nullableNumber(schema, isRequired)
		default:
			result.IsNullable = t.isNullable(schema)
		}
	}

	guardFormatConflicts(schema.Format, schema)
	return
}

// isNullable hints the generator as to render the type with a pointer or not.
//
// A schema is deemed nullable (i.e. rendered by a pointer) when:
// - a custom extension says it has to be so
// - it is an object with properties
// - it is a composed object (allOf)
//
// The interpretation of Required as a mean to make a type nullable is carried out elsewhere.
func (t *typeResolver) isNullable(schema *spec.Schema) bool {

	if nullable, ok := t.isNullableOverride(schema); ok {
		return nullable
	}

	return len(schema.Properties) > 0 || len(schema.AllOf) > 0
}

// isNullableOverride determines a nullable flag forced by an extension
func (t *typeResolver) isNullableOverride(schema *spec.Schema) (bool, bool) {
	check := func(extension string) (bool, bool) {
		v, found := schema.Extensions[extension]
		nullable, cast := v.(bool)
		return nullable, found && cast
	}

	if nullable, ok := check(xIsNullable); ok {
		return nullable, ok
	}

	if nullable, ok := check(xNullable); ok {
		return nullable, ok
	}

	return false, false
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

	// Override the general nullability rule from ResolveSchema() in array elements:
	// - only complex items are nullable (when not discriminated, not forced by x-nullable)
	// - arrays of allOf have non nullable elements when not forced by x-nullable
	elem := schema.Items.Schema
	if elem.Ref.String() != "" {
		// drill into $ref to figure out whether we want the element type to nullable or not
		resolved, erf := spec.ResolveRef(t.Doc.Spec(), &elem.Ref)
		if erf != nil {
			debugLog("error resolving ref %s: %v", schema.Ref.String(), erf)
		}
		elem = resolved
	}

	debugLogAsJSON("resolved item for %s", rt.GoType, elem)
	if nullable, ok := t.isNullableOverride(elem); ok {
		debugLog("found nullable override in element %s: %t", rt.GoType, nullable)
		rt.IsNullable = nullable
	} else {
		// this differs from isNullable for elements with AllOf
		debugLog("no nullable override in element %s: Properties: %t, HasDiscriminator: %t", rt.GoType, len(elem.Properties) > 0, rt.HasDiscriminator)
		rt.IsNullable = len(elem.Properties) > 0 && !rt.HasDiscriminator
	}

	result.GoType = "[]" + rt.GoType
	if rt.IsNullable && !strings.HasPrefix(rt.GoType, "*") {
		result.GoType = "[]*" + rt.GoType
	}

	result.ElemType = &rt
	result.SwaggerType = array
	result.SwaggerFormat = ""
	result.IsEnumCI = hasEnumCI(schema.Extensions)
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
		tpe, pkg, alias := t.knownDefGoType(t.ModelName, *schema, t.goTypeName)
		result.GoType = tpe
		result.Pkg = pkg
		result.PkgAlias = alias
	}
	if len(schema.AllOf) > 0 {
		result.GoType = t.goTypeName(t.ModelName)
		result.IsComplexObject = true
		var isNullable bool
		for _, sch := range schema.AllOf {
			p := sch
			if t.isNullable(&p) {
				isNullable = true
			}
		}
		if override, ok := t.isNullableOverride(schema); ok {
			// prioritize x-nullable extensions
			result.IsNullable = override
		} else {
			result.IsNullable = isNullable
		}
		result.SwaggerType = object
		return
	}

	// if this schema has properties, build a map of property name to
	// resolved type, this should also flag the object as anonymous,
	// when a ref is found, the anonymous flag will be reset
	if len(schema.Properties) > 0 {
		result.IsNullable = t.isNullable(schema)
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

		if et.IsExternal {
			// external AdditionalProperties are a special case because we look ahead into schemas
			extType, _, _ := t.knownDefGoType(t.ModelName, *sch, t.goTypeName)
			et.GoType = extType
		}

		// only complex map elements are nullable (when not forced by x-nullable)
		// TODO: figure out if required to check when not discriminated like arrays?
		et.IsNullable = !et.IsArray && t.isNullable(schema.AdditionalProperties.Schema)
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
	result.IsMap = true
	result.SwaggerType = object
	result.IsNullable = false
	// an object without properties but with MinProperties or MaxProperties is rendered as map[string]interface{}
	result.IsInterface = len(schema.Properties) == 0 && !schema.Validations().HasObjectValidations()
	if result.IsInterface {
		result.GoType = iface
	} else {
		result.GoType = "map[string]interface{}"
	}
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

func hasEnumCI(ve spec.Extensions) bool {
	v, ok := ve[xGoEnumCI]
	if !ok {
		return false
	}

	isEnumCI, ok := v.(bool)
	// All enumeration types are case-sensitive by default
	return ok && isEnumCI
}

func (t *typeResolver) shortCircuitResolveExternal(tpe, pkg, alias string, extType *externalTypeDefinition, schema *spec.Schema, isRequired bool) resolvedType {
	// short circuit type resolution for external types
	debugLogAsJSON("shortCircuitResolveExternal", extType)

	var result resolvedType
	result.Extensions = schema.Extensions
	result.GoType = tpe
	result.Pkg = pkg
	result.PkgAlias = alias
	result.IsInterface = false
	// by default consider that we have a type with validations. Use hint "interface" or "noValidation" to disable validations
	result.SkipExternalValidation = swag.BoolValue(extType.Hints.NoValidation)
	result.IsNullable = isRequired

	result.setKind(extType.Hints.Kind)
	if result.IsInterface || result.IsStream {
		result.IsNullable = false
	}
	if extType.Hints.Nullable != nil {
		result.IsNullable = swag.BoolValue(extType.Hints.Nullable)
	}

	if nullable, ok := t.isNullableOverride(schema); ok {
		result.IsNullable = nullable // x-nullable directive rules them all
	}

	// other extensions
	if result.IsArray {
		result.IsEmptyOmitted = false
		tpe = "array"
	}

	result.setExtensions(schema, tpe)
	return result
}

func (t *typeResolver) ResolveSchema(schema *spec.Schema, isAnonymous, isRequired bool) (result resolvedType, err error) {
	debugLog("resolving schema (anon: %t, req: %t) %s", isAnonymous, isRequired, t.ModelName)
	defer func() {
		debugLog("returning after resolve schema: %s", pretty.Sprint(result))
	}()

	if schema == nil {
		result.IsInterface = true
		result.GoType = iface
		return
	}

	extType, isExternalType := t.resolveExternalType(schema.Extensions)
	if isExternalType {
		tpe, pkg, alias := t.knownDefGoType(t.ModelName, *schema, t.goTypeName)
		debugLog("found type %s declared as external, imported from %s as %s. Has type hints? %t, rendered has embedded? %t",
			t.ModelName, pkg, tpe, extType.Hints.Kind != "", extType.Embedded)

		if extType.Hints.Kind != "" && !extType.Embedded {
			// use hint to qualify type
			debugLog("short circuits external type resolution with hint for %s", tpe)
			result = t.shortCircuitResolveExternal(tpe, pkg, alias, extType, schema, isRequired)
			result.IsExternal = isAnonymous // mark anonymous external types only, not definitions
			return
		}

		// use spec to qualify type
		debugLog("marking type %s as external embedded: %t", tpe, extType.Embedded)
		defer func() { // enforce bubbling up decisions taken about being an external type
			// mark this type as an embedded external definition if requested
			result.IsEmbedded = extType.Embedded
			result.IsExternal = isAnonymous // for non-embedded, mark anonymous external types only, not definitions

			result.IsAnonymous = false
			result.IsAliased = true
			result.IsNullable = isRequired
			if extType.Hints.Nullable != nil {
				result.IsNullable = swag.BoolValue(extType.Hints.Nullable)
			}

			result.IsMap = false
			result.AliasedType = result.GoType
			result.IsInterface = false

			if result.IsEmbedded {
				result.ElemType = &resolvedType{
					IsExternal:             isAnonymous, // mark anonymous external types only, not definitions
					IsInterface:            false,
					Pkg:                    extType.Import.Package,
					PkgAlias:               extType.Import.Alias,
					SkipExternalValidation: swag.BoolValue(extType.Hints.NoValidation),
				}
				if extType.Import.Alias != "" {
					result.ElemType.GoType = extType.Import.Alias + "." + extType.Type
				} else {
					result.ElemType.GoType = extType.Type
				}
				result.ElemType.setKind(extType.Hints.Kind)
				if result.IsInterface || result.IsStream {
					result.ElemType.IsNullable = false
				}
				if extType.Hints.Nullable != nil {
					result.ElemType.IsNullable = swag.BoolValue(extType.Hints.Nullable)
				}
				// embedded external: by default consider validation is skipped for the external type
				//
				// NOTE: at this moment the template generates a type assertion, so this setting does not really matter
				// for embedded types.
				if extType.Hints.NoValidation != nil {
					result.ElemType.SkipExternalValidation = swag.BoolValue(extType.Hints.NoValidation)
				} else {
					result.ElemType.SkipExternalValidation = true
				}
			} else {
				// non-embedded external type: by default consider that validation is enabled (SkipExternalValidation: false)
				result.SkipExternalValidation = swag.BoolValue(extType.Hints.NoValidation)
			}

			if nullable, ok := t.isNullableOverride(schema); ok {
				result.IsNullable = nullable
			}
		}()
	}

	tpe := t.firstType(schema)
	var returns bool

	guardValidations(tpe, schema, schema.Type...)

	returns, result, err = t.resolveSchemaRef(schema, isRequired)

	if returns {
		if !isAnonymous {
			result.IsMap = false
			result.IsComplexObject = true
		}

		return
	}

	defer func() {
		result.setExtensions(schema, tpe)
	}()

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
		return
	}

	result.IsNullable = t.isNullable(schema) || isRequired

	switch tpe {
	case array:
		result, err = t.resolveArray(schema, isAnonymous, false)

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
			result = resolvedType{}
			break
		}
		result.HasDiscriminator = schema.Discriminator != ""

	case "null":
		if schema.Validations().HasObjectValidations() {
			// no explicit object type, but inferred from object validations:
			// this makes the type a map[string]interface{} instead of interface{}
			result, err = t.resolveObject(schema, isAnonymous)
			if err != nil {
				result = resolvedType{}
				break
			}
			result.HasDiscriminator = schema.Discriminator != ""
			break
		}

		result.GoType = iface
		result.SwaggerType = object
		result.IsNullable = false
		result.IsInterface = true

	default:
		err = fmt.Errorf("unresolvable: %v (format %q)", schema.Type, schema.Format)
	}

	return
}

func warnSkipValidation(types interface{}) func(string, interface{}) {
	return func(validation string, value interface{}) {
		value = reflect.Indirect(reflect.ValueOf(value)).Interface()
		log.Printf("warning: validation %s (value: %v) not compatible with type %v. Skipped", validation, value, types)
	}
}

// guardValidations removes (with a warning) validations that don't fit with the schema type.
//
// Notice that the "enum" validation is allowed on any type but file.
func guardValidations(tpe string, schema interface {
	Validations() spec.SchemaValidations
	SetValidations(spec.SchemaValidations)
}, types ...string) {

	v := schema.Validations()
	if len(types) == 0 {
		types = []string{tpe}
	}
	defer func() {
		schema.SetValidations(v)
	}()

	if tpe != array {
		v.ClearArrayValidations(warnSkipValidation(types))
	}

	if tpe != str && tpe != file {
		v.ClearStringValidations(warnSkipValidation(types))
	}

	if tpe != object {
		v.ClearObjectValidations(warnSkipValidation(types))
	}

	if tpe != number && tpe != integer {
		v.ClearNumberValidations(warnSkipValidation(types))
	}

	if tpe == file {
		// keep MinLength/MaxLength on file
		if v.Pattern != "" {
			warnSkipValidation(types)("pattern", v.Pattern)
			v.Pattern = ""
		}
		if v.HasEnum() {
			warnSkipValidation(types)("enum", v.Enum)
			v.Enum = nil
		}
	}

	// other cases:  mapped as interface{}: no validations allowed but Enum
}

// guardFormatConflicts handles all conflicting properties
// (for schema model or simple schema) when a format is set.
//
// At this moment, validation guards already handle all known conflicts, but for the
// special case of binary (i.e. io.Reader).
func guardFormatConflicts(format string, schema interface {
	Validations() spec.SchemaValidations
	SetValidations(spec.SchemaValidations)
}) {
	v := schema.Validations()
	msg := fmt.Sprintf("for format %q", format)

	// for this format, no additional validations are supported
	if format == "binary" {
		// no validations supported on binary fields at this moment (io.Reader)
		v.ClearStringValidations(warnSkipValidation(msg))
		if v.HasEnum() {
			warnSkipValidation(msg)
			v.Enum = nil
		}
		schema.SetValidations(v)
	}
	// more cases should be inserted here if they arise
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
	IsJSONString      bool
	IsEnumCI          bool
	IsBase64          bool
	IsExternal        bool

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

	// IsEmbedded applies to externally defined types. When embedded, a type
	// is generated in models that embeds the external type, with the Validate
	// method.
	IsEmbedded bool

	SkipExternalValidation bool
}

// Zero returns an initializer for the type
func (rt resolvedType) Zero() string {
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

// ToString returns a string conversion for a type akin to a string
func (rt resolvedType) ToString(value string) string {
	if !rt.IsPrimitive || rt.SwaggerType != "string" || rt.IsStream {
		return ""
	}
	if rt.IsCustomFormatter {
		if rt.IsAliased {
			return fmt.Sprintf("%s(%s).String()", rt.AliasedType, value)
		}
		return fmt.Sprintf("%s.String()", value)
	}
	var deref string
	if rt.IsNullable {
		deref = "*"
	}
	if rt.GoType == "string" || rt.GoType == "*string" {
		return fmt.Sprintf("%s%s", deref, value)
	}

	return fmt.Sprintf("string(%s%s)", deref, value)
}

func (rt *resolvedType) setExtensions(schema *spec.Schema, origType string) {
	rt.IsEnumCI = hasEnumCI(schema.Extensions)
	rt.setIsEmptyOmitted(schema, origType)
	rt.setIsJSONString(schema, origType)

	if customTag, found := schema.Extensions[xGoCustomTag]; found {
		if rt.Extensions == nil {
			rt.Extensions = make(spec.Extensions)
		}
		rt.Extensions[xGoCustomTag] = customTag
	}
}

func (rt *resolvedType) setIsEmptyOmitted(schema *spec.Schema, tpe string) {
	if v, found := schema.Extensions[xOmitEmpty]; found {
		omitted, cast := v.(bool)
		rt.IsEmptyOmitted = omitted && cast
		return
	}
	// array of primitives are by default not empty-omitted, but arrays of aliased type are
	rt.IsEmptyOmitted = (tpe != array) || (tpe == array && rt.IsAliased)
}

func (rt *resolvedType) setIsJSONString(schema *spec.Schema, tpe string) {
	_, found := schema.Extensions[xGoJSONString]
	if !found {
		rt.IsJSONString = false
		return
	}
	rt.IsJSONString = true
}

func (rt *resolvedType) setKind(kind string) {
	if kind != "" {
		debugLog("overriding kind for %s as %s", rt.GoType, kind)
	}
	switch kind {
	case "map":
		rt.IsMap = true
		rt.IsArray = false
		rt.IsComplexObject = false
		rt.IsInterface = false
		rt.IsStream = false
		rt.IsTuple = false
		rt.IsPrimitive = false
		rt.SwaggerType = object
	case "array":
		rt.IsMap = false
		rt.IsArray = true
		rt.IsComplexObject = false
		rt.IsInterface = false
		rt.IsStream = false
		rt.IsTuple = false
		rt.IsPrimitive = false
		rt.SwaggerType = array
	case "object":
		rt.IsMap = false
		rt.IsArray = false
		rt.IsComplexObject = true
		rt.IsInterface = false
		rt.IsStream = false
		rt.IsTuple = false
		rt.IsPrimitive = false
		rt.SwaggerType = object
	case "interface", "null":
		rt.IsMap = false
		rt.IsArray = false
		rt.IsComplexObject = false
		rt.IsInterface = true
		rt.IsStream = false
		rt.IsTuple = false
		rt.IsPrimitive = false
		rt.SwaggerType = iface
	case "stream":
		rt.IsMap = false
		rt.IsArray = false
		rt.IsComplexObject = false
		rt.IsInterface = false
		rt.IsStream = true
		rt.IsTuple = false
		rt.IsPrimitive = false
		rt.SwaggerType = file
	case "tuple":
		rt.IsMap = false
		rt.IsArray = false
		rt.IsComplexObject = false
		rt.IsInterface = false
		rt.IsStream = false
		rt.IsTuple = true
		rt.IsPrimitive = false
		rt.SwaggerType = array
	case "primitive":
		rt.IsMap = false
		rt.IsArray = false
		rt.IsComplexObject = false
		rt.IsInterface = false
		rt.IsStream = false
		rt.IsTuple = false
		rt.IsPrimitive = true
	case "":
		break
	default:
		log.Printf("warning: unsupported hint value for external type: %q. Skipped", kind)
	}
}
