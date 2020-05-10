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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

const asMethod = "()"

/*
Rewrite specification document first:

* anonymous objects
* tuples
* extensible objects (properties + additionalProperties)
* AllOfs when they match the rewrite criteria (not a nullable allOf)

Find string enums and generate specialized idiomatic enum with them

Every action that happens tracks the path which is a linked list of refs


*/

// GenerateDefinition generates a model file for a schema definition.
func GenerateDefinition(modelNames []string, opts *GenOpts) error {
	if opts == nil {
		return errors.New("gen opts are required")
	}

	templates.SetAllowOverride(opts.AllowTemplateOverride)

	if opts.TemplateDir != "" {
		if err := templates.LoadDir(opts.TemplateDir); err != nil {
			return err
		}
	}

	if err := opts.CheckOpts(); err != nil {
		return err
	}

	// Load the spec
	specPath, specDoc, err := loadSpec(opts.Spec)
	if err != nil {
		return err
	}

	if len(modelNames) == 0 {
		for k := range specDoc.Spec().Definitions {
			modelNames = append(modelNames, k)
		}
	}

	for _, modelName := range modelNames {
		// lookup schema
		model, ok := specDoc.Spec().Definitions[modelName]
		if !ok {
			return fmt.Errorf("model %q not found in definitions given by %q", modelName, specPath)
		}

		// generate files
		generator := definitionGenerator{
			Name:    modelName,
			Model:   model,
			SpecDoc: specDoc,
			Target: filepath.Join(
				opts.Target,
				filepath.FromSlash(opts.LanguageOpts.ManglePackagePath(opts.ModelPackage, ""))),
			opts: opts,
		}

		if err := generator.Generate(); err != nil {
			return err
		}
	}

	return nil
}

type definitionGenerator struct {
	Name    string
	Model   spec.Schema
	SpecDoc *loads.Document
	Target  string
	opts    *GenOpts
}

func (m *definitionGenerator) Generate() error {

	mod, err := makeGenDefinition(m.Name, m.Target, m.Model, m.SpecDoc, m.opts)
	if err != nil {
		return fmt.Errorf("could not generate definitions for model %s on target %s: %v", m.Name, m.Target, err)
	}

	if m.opts.DumpData {
		bb, _ := json.MarshalIndent(swag.ToDynamicJSON(mod), "", " ")
		fmt.Fprintln(os.Stdout, string(bb))
		return nil
	}

	if m.opts.IncludeModel {
		log.Println("including additional model")
		if err := m.generateModel(mod); err != nil {
			return fmt.Errorf("could not generate model: %v", err)
		}
	}
	log.Println("generated model", m.Name)

	return nil
}

func (m *definitionGenerator) generateModel(g *GenDefinition) error {
	debugLog("rendering definitions for %+v", *g)
	return m.opts.renderDefinition(g)
}

func makeGenDefinition(name, pkg string, schema spec.Schema, specDoc *loads.Document, opts *GenOpts) (*GenDefinition, error) {
	gd, err := makeGenDefinitionHierarchy(name, pkg, "", schema, specDoc, opts)

	if err == nil && gd != nil {
		// before yielding the schema to the renderer, we check if the top-level Validate method gets some content
		// this means that the immediate content of the top level definitions has at least one validation.
		//
		// If none is found at this level and that no special case where no Validate() method is exposed at all
		// (e.g. io.ReadCloser and interface{} types and their aliases), then there is an empty Validate() method which
		// just return nil (the object abides by the runtime.Validatable interface, but knows it has nothing to validate).
		//
		// We do this at the top level because of the possibility of aliased types which always bubble up validation to types which
		// are referring to them. This results in correct but inelegant code with empty validations.
		gd.GenSchema.HasValidations = shallowValidationLookup(gd.GenSchema)
	}
	return gd, err
}

func shallowValidationLookup(sch GenSchema) bool {
	// scan top level need for validations
	//
	// NOTE: this supersedes the previous NeedsValidation flag
	// With the introduction of this shallow lookup, it is no more necessary
	// to establish a distinction between HasValidations (e.g. carries on validations)
	// and NeedsValidation (e.g. should have a Validate method with something in it).
	// The latter was almost not used anyhow.

	if sch.IsArray && sch.HasValidations {
		return true
	}
	if sch.IsStream || sch.IsInterface { // these types have no validation - aliased types on those do not implement the Validatable interface
		return false
	}
	if sch.Required || sch.IsCustomFormatter && !sch.IsStream {
		return true
	}
	if sch.MaxLength != nil || sch.MinLength != nil || sch.Pattern != "" || sch.MultipleOf != nil || sch.Minimum != nil || sch.Maximum != nil || len(sch.Enum) > 0 || len(sch.ItemsEnum) > 0 {
		return true
	}
	for _, a := range sch.AllOf {
		if a.HasValidations {
			return true
		}
	}
	for _, p := range sch.Properties {
		// Using a base type within another structure triggers validation of the base type.
		// The discriminator property in the base type definition itself does not.
		if (p.HasValidations || p.Required) && !(sch.IsBaseType && p.Name == sch.DiscriminatorField) || (p.IsAliased || p.IsComplexObject) && !(p.IsInterface || p.IsStream) {
			return true
		}
	}
	if sch.IsTuple && (sch.AdditionalItems != nil && (sch.AdditionalItems.HasValidations || sch.AdditionalItems.Required)) {
		return true
	}
	if sch.HasAdditionalProperties && (sch.AdditionalProperties.IsInterface || sch.AdditionalProperties.IsStream) {
		return false
	}

	if sch.HasAdditionalProperties && (sch.AdditionalProperties.HasValidations || sch.AdditionalProperties.Required || sch.AdditionalProperties.IsAliased && !(sch.AdditionalProperties.IsInterface || sch.AdditionalProperties.IsStream)) {
		return true
	}

	if sch.IsAliased && (sch.IsPrimitive && sch.HasValidations) { // non primitive aliased have either other attributes with validation (above) or shall not validate
		return true
	}
	if sch.HasBaseType || sch.IsSubType {
		return true
	}
	return false
}

func makeGenDefinitionHierarchy(name, pkg, container string, schema spec.Schema, specDoc *loads.Document, opts *GenOpts) (*GenDefinition, error) {
	// Check if model is imported from external package using x-go-type
	_, external := schema.Extensions[xGoType]

	receiver := "m"
	// models are resolved in the current package
	resolver := newTypeResolver("", specDoc)
	resolver.ModelName = name
	analyzed := analysis.New(specDoc.Spec())

	di := discriminatorInfo(analyzed)

	pg := schemaGenContext{
		Path:                       "",
		Name:                       name,
		Receiver:                   receiver,
		IndexVar:                   "i",
		ValueExpr:                  receiver,
		Schema:                     schema,
		Required:                   false,
		TypeResolver:               resolver,
		Named:                      true,
		ExtraSchemas:               make(map[string]GenSchema),
		Discrimination:             di,
		Container:                  container,
		IncludeValidator:           opts.IncludeValidator,
		IncludeModel:               opts.IncludeModel,
		StrictAdditionalProperties: opts.StrictAdditionalProperties,
	}
	if err := pg.makeGenSchema(); err != nil {
		return nil, fmt.Errorf("could not generate schema for %s: %v", name, err)
	}
	dsi, ok := di.Discriminators["#/definitions/"+name]
	if ok {
		// when these 2 are true then the schema will render as an interface
		pg.GenSchema.IsBaseType = true
		pg.GenSchema.IsExported = true
		pg.GenSchema.DiscriminatorField = dsi.FieldName

		if pg.GenSchema.Discriminates == nil {
			pg.GenSchema.Discriminates = make(map[string]string)
		}
		pg.GenSchema.Discriminates[name] = dsi.GoType
		pg.GenSchema.DiscriminatorValue = name

		for _, v := range dsi.Children {
			pg.GenSchema.Discriminates[v.FieldValue] = v.GoType
		}

		for j := range pg.GenSchema.Properties {
			if !strings.HasSuffix(pg.GenSchema.Properties[j].ValueExpression, asMethod) {
				pg.GenSchema.Properties[j].ValueExpression += asMethod
			}
		}
	}

	dse, ok := di.Discriminated["#/definitions/"+name]
	if ok {
		pg.GenSchema.DiscriminatorField = dse.FieldName
		pg.GenSchema.DiscriminatorValue = dse.FieldValue
		pg.GenSchema.IsSubType = true
		knownProperties := make(map[string]struct{})

		// find the referenced definitions
		// check if it has a discriminator defined
		// when it has a discriminator get the schema and run makeGenSchema for it.
		// replace the ref with this new genschema
		swsp := specDoc.Spec()
		for i, ss := range schema.AllOf {
			ref := ss.Ref
			for ref.String() != "" {
				var rsch *spec.Schema
				var err error
				rsch, err = spec.ResolveRef(swsp, &ref)
				if err != nil {
					return nil, err
				}
				ref = rsch.Ref
				if rsch != nil && rsch.Ref.String() != "" {
					ref = rsch.Ref
					continue
				}
				ref = spec.Ref{}
				if rsch != nil && rsch.Discriminator != "" {
					gs, err := makeGenDefinitionHierarchy(strings.TrimPrefix(ss.Ref.String(), "#/definitions/"), pkg, pg.GenSchema.Name, *rsch, specDoc, opts)
					if err != nil {
						return nil, err
					}
					gs.GenSchema.IsBaseType = true
					gs.GenSchema.IsExported = true
					pg.GenSchema.AllOf[i] = gs.GenSchema
					schPtr := &(pg.GenSchema.AllOf[i])
					if schPtr.AdditionalItems != nil {
						schPtr.AdditionalItems.IsBaseType = true
					}
					if schPtr.AdditionalProperties != nil {
						schPtr.AdditionalProperties.IsBaseType = true
					}
					for j := range schPtr.Properties {
						schPtr.Properties[j].IsBaseType = true
						knownProperties[schPtr.Properties[j].Name] = struct{}{}
					}
				}
			}
		}

		// dedupe the fields
		alreadySeen := make(map[string]struct{})
		for i, ss := range pg.GenSchema.AllOf {
			var remainingProperties GenSchemaList
			for _, p := range ss.Properties {
				if _, ok := knownProperties[p.Name]; !ok || ss.IsBaseType {
					if _, seen := alreadySeen[p.Name]; !seen {
						remainingProperties = append(remainingProperties, p)
						alreadySeen[p.Name] = struct{}{}
					}
				}
			}
			pg.GenSchema.AllOf[i].Properties = remainingProperties
		}

	}

	defaultImports := []string{
		"github.com/go-openapi/errors",
		"github.com/go-openapi/runtime",
		"github.com/go-openapi/swag",
		"github.com/go-openapi/validate",
	}

	return &GenDefinition{
		GenCommon: GenCommon{
			Copyright:        opts.Copyright,
			TargetImportPath: filepath.ToSlash(opts.LanguageOpts.baseImport(opts.Target)),
		},
		Package:        opts.LanguageOpts.ManglePackageName(path.Base(filepath.ToSlash(pkg)), "definitions"),
		GenSchema:      pg.GenSchema,
		DependsOn:      pg.Dependencies,
		DefaultImports: defaultImports,
		ExtraSchemas:   gatherExtraSchemas(pg.ExtraSchemas),
		Imports:        findImports(&pg.GenSchema),
		External:       external,
	}, nil
}

func findImports(sch *GenSchema) map[string]string {
	imp := map[string]string{}
	t := sch.resolvedType
	if t.Pkg != "" && t.PkgAlias != "" {
		imp[t.PkgAlias] = t.Pkg
	}
	if sch.Items != nil {
		sub := findImports(sch.Items)
		for k, v := range sub {
			imp[k] = v
		}
	}
	if sch.AdditionalItems != nil {
		sub := findImports(sch.AdditionalItems)
		for k, v := range sub {
			imp[k] = v
		}
	}
	if sch.Object != nil {
		sub := findImports(sch.Object)
		for k, v := range sub {
			imp[k] = v
		}
	}
	if sch.Properties != nil {
		for _, p := range sch.Properties {
			sub := findImports(&p)
			for k, v := range sub {
				imp[k] = v
			}
		}
	}
	if sch.AdditionalProperties != nil {
		sub := findImports(sch.AdditionalProperties)
		for k, v := range sub {
			imp[k] = v
		}
	}
	if sch.AllOf != nil {
		for _, p := range sch.AllOf {
			sub := findImports(&p)
			for k, v := range sub {
				imp[k] = v
			}
		}
	}
	return imp
}

type schemaGenContext struct {
	Required                   bool
	AdditionalProperty         bool
	Untyped                    bool
	Named                      bool
	RefHandled                 bool
	IsVirtual                  bool
	IsTuple                    bool
	IncludeValidator           bool
	IncludeModel               bool
	StrictAdditionalProperties bool
	Index                      int

	Path         string
	Name         string
	ParamName    string
	Accessor     string
	Receiver     string
	IndexVar     string
	KeyVar       string
	ValueExpr    string
	Container    string
	Schema       spec.Schema
	TypeResolver *typeResolver

	GenSchema      GenSchema
	Dependencies   []string // NOTE: Dependencies is actually set nowhere
	ExtraSchemas   map[string]GenSchema
	Discriminator  *discor
	Discriminated  *discee
	Discrimination *discInfo
}

func (sg *schemaGenContext) NewSliceBranch(schema *spec.Schema) *schemaGenContext {
	debugLog("new slice branch %s (model: %s)", sg.Name, sg.TypeResolver.ModelName)
	pg := sg.shallowClone()
	indexVar := pg.IndexVar
	if pg.Path == "" {
		pg.Path = "strconv.Itoa(" + indexVar + ")"
	} else {
		pg.Path = pg.Path + "+ \".\" + strconv.Itoa(" + indexVar + ")"
	}
	// check who is parent, if it's a base type then rewrite the value expression
	if sg.Discrimination != nil && sg.Discrimination.Discriminators != nil {
		_, rewriteValueExpr := sg.Discrimination.Discriminators["#/definitions/"+sg.TypeResolver.ModelName]
		if (pg.IndexVar == "i" && rewriteValueExpr) || sg.GenSchema.ElemType.IsBaseType {
			if !sg.GenSchema.IsAliased {
				pg.ValueExpr = sg.Receiver + "." + swag.ToJSONName(sg.GenSchema.Name) + "Field"
			} else {
				pg.ValueExpr = sg.Receiver
			}
		}
	}
	sg.GenSchema.IsBaseType = sg.GenSchema.ElemType.HasDiscriminator
	pg.IndexVar = indexVar + "i"
	pg.ValueExpr = pg.ValueExpr + "[" + indexVar + "]"
	pg.Schema = *schema
	pg.Required = false
	if sg.IsVirtual {
		pg.TypeResolver = sg.TypeResolver.NewWithModelName(sg.TypeResolver.ModelName)
	}

	// when this is an anonymous complex object, this needs to become a ref
	return pg
}

func (sg *schemaGenContext) NewAdditionalItems(schema *spec.Schema) *schemaGenContext {
	debugLog("new additional items\n")

	pg := sg.shallowClone()
	indexVar := pg.IndexVar
	pg.Name = sg.Name + " items"
	itemsLen := 0
	if sg.Schema.Items != nil {
		itemsLen = sg.Schema.Items.Len()
	}
	var mod string
	if itemsLen > 0 {
		mod = "+" + strconv.Itoa(itemsLen)
	}
	if pg.Path == "" {
		pg.Path = "strconv.Itoa(" + indexVar + mod + ")"
	} else {
		pg.Path = pg.Path + "+ \".\" + strconv.Itoa(" + indexVar + mod + ")"
	}
	pg.IndexVar = indexVar
	pg.ValueExpr = sg.ValueExpr + "." + pascalize(sg.GoName()) + "Items[" + indexVar + "]"
	pg.Schema = spec.Schema{}
	if schema != nil {
		pg.Schema = *schema
	}
	pg.Required = false
	return pg
}

func (sg *schemaGenContext) NewTupleElement(schema *spec.Schema, index int) *schemaGenContext {
	debugLog("New tuple element\n")

	pg := sg.shallowClone()
	if pg.Path == "" {
		pg.Path = "\"" + strconv.Itoa(index) + "\""
	} else {
		pg.Path = pg.Path + "+ \".\"+\"" + strconv.Itoa(index) + "\""
	}
	pg.ValueExpr = pg.ValueExpr + ".P" + strconv.Itoa(index)

	pg.Required = true
	pg.IsTuple = true
	pg.Schema = *schema

	return pg
}

func (sg *schemaGenContext) NewStructBranch(name string, schema spec.Schema) *schemaGenContext {
	debugLog("new struct branch %s (parent %s)", sg.Name, sg.Container)
	pg := sg.shallowClone()
	if sg.Path == "" {
		pg.Path = fmt.Sprintf("%q", name)
	} else {
		pg.Path = pg.Path + "+\".\"+" + fmt.Sprintf("%q", name)
	}
	pg.Name = name
	pg.ValueExpr = pg.ValueExpr + "." + pascalize(goName(&schema, name))
	pg.Schema = schema
	for _, fn := range sg.Schema.Required {
		if name == fn {
			pg.Required = true
			break
		}
	}
	debugLog("made new struct branch %s (parent %s)", pg.Name, pg.Container)
	return pg
}

func (sg *schemaGenContext) shallowClone() *schemaGenContext {
	debugLog("cloning context %s\n", sg.Name)
	pg := new(schemaGenContext)
	*pg = *sg
	if pg.Container == "" {
		pg.Container = sg.Name
	}
	pg.GenSchema = GenSchema{}
	pg.Dependencies = nil
	pg.Named = false
	pg.Index = 0
	pg.IsTuple = false
	pg.IncludeValidator = sg.IncludeValidator
	pg.IncludeModel = sg.IncludeModel
	pg.StrictAdditionalProperties = sg.StrictAdditionalProperties
	return pg
}

func (sg *schemaGenContext) NewCompositionBranch(schema spec.Schema, index int) *schemaGenContext {
	debugLog("new composition branch %s (parent: %s, index: %d)", sg.Name, sg.Container, index)
	pg := sg.shallowClone()
	pg.Schema = schema
	pg.Name = "AO" + strconv.Itoa(index)
	if sg.Name != sg.TypeResolver.ModelName {
		pg.Name = sg.Name + pg.Name
	}
	pg.Index = index
	debugLog("made new composition branch %s (parent: %s)", pg.Name, pg.Container)
	return pg
}

func (sg *schemaGenContext) NewAdditionalProperty(schema spec.Schema) *schemaGenContext {
	debugLog("new additional property %s (expr: %s)", sg.Name, sg.ValueExpr)
	pg := sg.shallowClone()
	pg.Schema = schema
	if pg.KeyVar == "" {
		pg.ValueExpr = sg.ValueExpr
	}
	pg.KeyVar += "k"
	pg.ValueExpr += "[" + pg.KeyVar + "]"
	pg.Path = pg.KeyVar
	pg.GenSchema.Suffix = "Value"
	if sg.Path != "" {
		pg.Path = sg.Path + "+\".\"+" + pg.KeyVar
	}
	// propagates the special IsNullable override for maps of slices and
	// maps of aliased types.
	pg.GenSchema.IsMapNullOverride = sg.GenSchema.IsMapNullOverride
	return pg
}

func hasSliceValidations(model *spec.Schema) (hasSliceValidations bool) {
	hasSliceValidations = model.MaxItems != nil || model.MinItems != nil || model.UniqueItems || len(model.Enum) > 0
	return
}

func hasValidations(model *spec.Schema, isRequired bool) (hasValidation bool) {
	// NOTE: needsValidation has gone deprecated and is replaced by top-level's shallowValidationLookup()
	hasNumberValidation := model.Maximum != nil || model.Minimum != nil || model.MultipleOf != nil
	hasStringValidation := model.MaxLength != nil || model.MinLength != nil || model.Pattern != ""
	hasEnum := len(model.Enum) > 0

	// since this was added to deal with discriminator, we'll fix this when testing discriminated types
	simpleObject := len(model.Properties) > 0 && model.Discriminator == ""

	// lift validations from allOf branches
	hasAllOfValidation := false
	for _, s := range model.AllOf {
		hasAllOfValidation = hasValidations(&s, false)
		hasAllOfValidation = s.Ref.String() != "" || hasAllOfValidation
		if hasAllOfValidation {
			break
		}
	}

	hasValidation = hasNumberValidation || hasStringValidation || hasSliceValidations(model) || hasEnum || simpleObject || hasAllOfValidation || isRequired

	return
}

// handleFormatConflicts handles all conflicting model properties when a format is set
func handleFormatConflicts(model *spec.Schema) {
	switch model.Format {
	case "date", "datetime", "uuid", "bsonobjectid", "base64", "duration":
		model.MinLength = nil
		model.MaxLength = nil
		model.Pattern = ""
		// more cases should be inserted here if they arise
	}
}

func (sg *schemaGenContext) schemaValidations() sharedValidations {
	model := sg.Schema
	// resolve any conflicting properties if the model has a format
	handleFormatConflicts(&model)

	isRequired := sg.Required
	if model.Default != nil || model.ReadOnly {
		// when readOnly or default is specified, this disables Required validation (Swagger-specific)
		isRequired = false
	}
	hasSliceValidations := model.MaxItems != nil || model.MinItems != nil || model.UniqueItems || len(model.Enum) > 0
	hasValidations := hasValidations(&model, isRequired)

	s := sharedValidationsFromSchema(model, sg.Required)
	s.HasValidations = hasValidations
	s.HasSliceValidations = hasSliceValidations
	return s
}

func mergeValidation(other *schemaGenContext) bool {
	// NOTE: NeesRequired and NeedsValidation are deprecated
	if other.GenSchema.AdditionalProperties != nil && other.GenSchema.AdditionalProperties.HasValidations {
		return true
	}
	if other.GenSchema.AdditionalItems != nil && other.GenSchema.AdditionalItems.HasValidations {
		return true
	}
	for _, sch := range other.GenSchema.AllOf {
		if sch.HasValidations {
			return true
		}
	}
	return other.GenSchema.HasValidations
}

func (sg *schemaGenContext) MergeResult(other *schemaGenContext, liftsRequired bool) {
	sg.GenSchema.HasValidations = sg.GenSchema.HasValidations || mergeValidation(other)

	if liftsRequired && other.GenSchema.AdditionalProperties != nil && other.GenSchema.AdditionalProperties.Required {
		sg.GenSchema.Required = true
	}
	if liftsRequired && other.GenSchema.Required {
		sg.GenSchema.Required = other.GenSchema.Required
	}

	if other.GenSchema.HasBaseType {
		sg.GenSchema.HasBaseType = other.GenSchema.HasBaseType
	}

	sg.Dependencies = append(sg.Dependencies, other.Dependencies...)

	// lift extra schemas
	for k, v := range other.ExtraSchemas {
		sg.ExtraSchemas[k] = v
	}
	if other.GenSchema.IsMapNullOverride {
		sg.GenSchema.IsMapNullOverride = true
	}
}

func (sg *schemaGenContext) buildProperties() error {
	debugLog("building properties %s (parent: %s)", sg.Name, sg.Container)

	for k, v := range sg.Schema.Properties {
		debugLogAsJSON("building property %s[%q] (tup: %t) (BaseType: %t)",
			sg.Name, k, sg.IsTuple, sg.GenSchema.IsBaseType, sg.Schema)
		debugLog("property %s[%q] (tup: %t) HasValidations: %t)",
			sg.Name, k, sg.IsTuple, sg.GenSchema.HasValidations)

		// check if this requires de-anonymizing, if so lift this as a new struct and extra schema
		tpe, err := sg.TypeResolver.ResolveSchema(&v, true, sg.IsTuple || containsString(sg.Schema.Required, k))
		if sg.Schema.Discriminator == k {
			tpe.IsNullable = false
		}
		if err != nil {
			return err
		}

		vv := v
		var hasValidation bool
		if tpe.IsComplexObject && tpe.IsAnonymous && len(v.Properties) > 0 {
			// this is an anonymous complex construct: build a new new type for it
			pg := sg.makeNewStruct(sg.Name+swag.ToGoName(k), v)
			pg.IsTuple = sg.IsTuple
			if sg.Path != "" {
				pg.Path = sg.Path + "+ \".\"+" + fmt.Sprintf("%q", k)
			} else {
				pg.Path = fmt.Sprintf("%q", k)
			}
			if err := pg.makeGenSchema(); err != nil {
				return err
			}
			if v.Discriminator != "" {
				pg.GenSchema.IsBaseType = true
				pg.GenSchema.IsExported = true
				pg.GenSchema.HasBaseType = true
			}

			vv = *spec.RefProperty("#/definitions/" + pg.Name)
			hasValidation = pg.GenSchema.HasValidations
			sg.ExtraSchemas[pg.Name] = pg.GenSchema
			// NOTE: MergeResult lifts validation status and extra schemas
			sg.MergeResult(pg, false)
		}

		emprop := sg.NewStructBranch(k, vv)
		emprop.IsTuple = sg.IsTuple

		if err := emprop.makeGenSchema(); err != nil {
			return err
		}

		// whatever the validations says, if we have an interface{}, do not validate
		// NOTE: this may be the case when the type is left empty and we get a Enum validation.
		if emprop.GenSchema.IsInterface || emprop.GenSchema.IsStream {
			emprop.GenSchema.HasValidations = false
		} else if hasValidation || emprop.GenSchema.HasValidations || emprop.GenSchema.Required || emprop.GenSchema.IsAliased || len(emprop.GenSchema.AllOf) > 0 {
			emprop.GenSchema.HasValidations = true
			sg.GenSchema.HasValidations = true
		}

		// generates format validation on property
		emprop.GenSchema.HasValidations = emprop.GenSchema.HasValidations || (tpe.IsCustomFormatter && !tpe.IsStream) || (tpe.IsArray && tpe.ElemType.IsCustomFormatter && !tpe.ElemType.IsStream)

		if emprop.Schema.Ref.String() != "" {
			// expand the schema of this property, so we take informed decisions about its type
			ref := emprop.Schema.Ref
			var sch *spec.Schema
			for ref.String() != "" {
				var rsch *spec.Schema
				var err error
				specDoc := sg.TypeResolver.Doc
				rsch, err = spec.ResolveRef(specDoc.Spec(), &ref)
				if err != nil {
					return err
				}
				ref = rsch.Ref
				if rsch != nil && rsch.Ref.String() != "" {
					ref = rsch.Ref
					continue
				}
				ref = spec.Ref{}
				sch = rsch
			}

			if emprop.Discrimination != nil {
				if _, ok := emprop.Discrimination.Discriminators[emprop.Schema.Ref.String()]; ok {
					emprop.GenSchema.IsBaseType = true
					emprop.GenSchema.IsNullable = false
					emprop.GenSchema.HasBaseType = true
				}
				if _, ok := emprop.Discrimination.Discriminated[emprop.Schema.Ref.String()]; ok {
					emprop.GenSchema.IsSubType = true
				}
			}

			// set property name
			var nm = filepath.Base(emprop.Schema.Ref.GetURL().Fragment)

			tr := sg.TypeResolver.NewWithModelName(goName(&emprop.Schema, swag.ToGoName(nm)))
			ttpe, err := tr.ResolveSchema(sch, false, true)
			if err != nil {
				return err
			}
			if ttpe.IsAliased {
				emprop.GenSchema.IsAliased = true
			}

			// lift validations
			hv := hasValidations(sch, false)

			// include format validation, excluding binary
			hv = hv || (ttpe.IsCustomFormatter && !ttpe.IsStream) || (ttpe.IsArray && ttpe.ElemType.IsCustomFormatter && !ttpe.ElemType.IsStream)

			// a base type property is always validated against the base type
			// exception: for the base type definition itself (see shallowValidationLookup())
			if (hv || emprop.GenSchema.IsBaseType) && !(emprop.GenSchema.IsInterface || emprop.GenSchema.IsStream) {
				emprop.GenSchema.HasValidations = true
			}
			if ttpe.HasAdditionalItems && sch.AdditionalItems.Schema != nil {
				// when AdditionalItems specifies a Schema, there is a validation
				// check if we stepped upon an exception
				child, err := tr.ResolveSchema(sch.AdditionalItems.Schema, false, true)
				if err != nil {
					return err
				}
				if !child.IsInterface && !child.IsStream {
					emprop.GenSchema.HasValidations = true
				}
			}
			if ttpe.IsMap && sch.AdditionalProperties != nil && sch.AdditionalProperties.Schema != nil {
				// when AdditionalProperties specifies a Schema, there is a validation
				// check if we stepped upon an exception
				child, err := tr.ResolveSchema(sch.AdditionalProperties.Schema, false, true)
				if err != nil {
					return err
				}
				if !child.IsInterface && !child.IsStream {
					emprop.GenSchema.HasValidations = true
				}
			}
		}

		if sg.Schema.Discriminator == k {
			// this is the discriminator property:
			// it is required, but forced as non-nullable,
			// since we never fill it with a zero-value
			// TODO: when no other property than discriminator, there is no validation
			emprop.GenSchema.IsNullable = false
		}
		if emprop.GenSchema.IsBaseType {
			sg.GenSchema.HasBaseType = true
		}
		sg.MergeResult(emprop, false)

		// when discriminated, data is accessed via a getter func
		if emprop.GenSchema.HasDiscriminator {
			emprop.GenSchema.ValueExpression += asMethod
		}

		emprop.GenSchema.Extensions = emprop.Schema.Extensions

		// set custom serializer tag
		if customTag, found := emprop.Schema.Extensions[xGoCustomTag]; found {
			emprop.GenSchema.CustomTag = customTag.(string)
		}
		sg.GenSchema.Properties = append(sg.GenSchema.Properties, emprop.GenSchema)
	}
	sort.Sort(sg.GenSchema.Properties)

	return nil
}

func (sg *schemaGenContext) buildAllOf() error {
	if len(sg.Schema.AllOf) == 0 {
		return nil
	}

	var hasArray, hasNonArray int

	sort.Sort(sg.GenSchema.AllOf)
	if sg.Container == "" {
		sg.Container = sg.Name
	}
	debugLogAsJSON("building all of for %d entries", len(sg.Schema.AllOf), sg.Schema)
	for i, sch := range sg.Schema.AllOf {
		tpe, ert := sg.TypeResolver.ResolveSchema(&sch, sch.Ref.String() == "", false)
		if ert != nil {
			return ert
		}

		// check for multiple arrays in allOf branches.
		// Although a valid JSON-Schema construct, it is not suited for serialization.
		// This is the same if we attempt to serialize an array with another object.
		// We issue a generation warning on this.
		if tpe.IsArray {
			hasArray++
		} else {
			hasNonArray++
		}
		debugLogAsJSON("trying", sch)
		if (tpe.IsAnonymous && len(sch.AllOf) > 0) || (sch.Ref.String() == "" && !tpe.IsComplexObject && (tpe.IsArray || tpe.IsInterface || tpe.IsPrimitive)) {
			// cases where anonymous structures cause the creation of a new type:
			// - nested allOf: this one is itself a AllOf: build a new type for it
			// - anonymous simple types for edge cases: array, primitive, interface{}
			// NOTE: when branches are aliased or anonymous, the nullable property in the branch type is lost.
			name := swag.ToVarName(goName(&sch, sg.Name+"AllOf"+strconv.Itoa(i)))
			debugLog("building anonymous nested allOf in %s: %s", sg.Name, name)
			ng := sg.makeNewStruct(name, sch)
			if err := ng.makeGenSchema(); err != nil {
				return err
			}

			newsch := spec.RefProperty("#/definitions/" + ng.Name)
			sg.Schema.AllOf[i] = *newsch

			pg := sg.NewCompositionBranch(*newsch, i)
			if err := pg.makeGenSchema(); err != nil {
				return err
			}

			// lift extra schemas & validations from new type
			pg.MergeResult(ng, true)

			// lift validations when complex or ref'ed:
			// - parent always calls its Validatable child
			// - child may or may not have validations
			//
			// Exception: child is not Validatable when interface or stream
			if !pg.GenSchema.IsInterface && !pg.GenSchema.IsStream {
				sg.GenSchema.HasValidations = true
			}

			// add the newly created type to the list of schemas to be rendered inline
			pg.ExtraSchemas[ng.Name] = ng.GenSchema

			sg.MergeResult(pg, true)

			sg.GenSchema.AllOf = append(sg.GenSchema.AllOf, pg.GenSchema)

			continue
		}

		comprop := sg.NewCompositionBranch(sch, i)
		if err := comprop.makeGenSchema(); err != nil {
			return err
		}
		if comprop.GenSchema.IsMap && comprop.GenSchema.HasAdditionalProperties && comprop.GenSchema.AdditionalProperties != nil && !comprop.GenSchema.IsInterface {
			// the anonymous branch is a map for AdditionalProperties: rewrite value expression
			comprop.GenSchema.ValueExpression = comprop.GenSchema.ValueExpression + "." + comprop.Name
			comprop.GenSchema.AdditionalProperties.ValueExpression = comprop.GenSchema.ValueExpression + "[" + comprop.GenSchema.AdditionalProperties.KeyVar + "]"
		}

		// lift validations when complex or ref'ed
		if (comprop.GenSchema.IsComplexObject || comprop.Schema.Ref.String() != "") && !(comprop.GenSchema.IsInterface || comprop.GenSchema.IsStream) {
			comprop.GenSchema.HasValidations = true
		}
		sg.MergeResult(comprop, true)
		sg.GenSchema.AllOf = append(sg.GenSchema.AllOf, comprop.GenSchema)
	}

	if hasArray > 1 || (hasArray > 0 && hasNonArray > 0) {
		log.Printf("warning: cannot generate serializable allOf with conflicting array definitions in %s", sg.Container)
	}

	sg.GenSchema.IsNullable = true

	// prevent IsAliased to bubble up (e.g. when a single branch is itself aliased)
	sg.GenSchema.IsAliased = sg.GenSchema.IsAliased && len(sg.GenSchema.AllOf) < 2

	return nil
}

type mapStack struct {
	Type     *spec.Schema
	Next     *mapStack
	Previous *mapStack
	ValueRef *schemaGenContext
	Context  *schemaGenContext
	NewObj   *schemaGenContext
}

func newMapStack(context *schemaGenContext) (first, last *mapStack, err error) {
	ms := &mapStack{
		Type:    &context.Schema,
		Context: context,
	}

	l := ms
	for l.HasMore() {
		tpe, err := l.Context.TypeResolver.ResolveSchema(l.Type.AdditionalProperties.Schema, true, true)
		if err != nil {
			return nil, nil, err
		}

		if !tpe.IsMap {
			//reached the end of the rabbit hole
			if tpe.IsComplexObject && tpe.IsAnonymous {
				// found an anonymous object: create the struct from a newly created definition
				nw := l.Context.makeNewStruct(l.Context.Name+" Anon", *l.Type.AdditionalProperties.Schema)
				sch := spec.RefProperty("#/definitions/" + nw.Name)
				l.NewObj = nw

				l.Type.AdditionalProperties.Schema = sch
				l.ValueRef = l.Context.NewAdditionalProperty(*sch)
			}
			// other cases where to stop are: a $ref or a simple object
			break
		}

		// continue digging for maps
		l.Next = &mapStack{
			Previous: l,
			Type:     l.Type.AdditionalProperties.Schema,
			Context:  l.Context.NewAdditionalProperty(*l.Type.AdditionalProperties.Schema),
		}
		l = l.Next
	}

	//return top and bottom entries of this stack of AdditionalProperties
	return ms, l, nil
}

// Build rewinds the stack of additional properties, building schemas from bottom to top
func (mt *mapStack) Build() error {
	if mt.NewObj == nil && mt.ValueRef == nil && mt.Next == nil && mt.Previous == nil {
		csch := mt.Type.AdditionalProperties.Schema
		cp := mt.Context.NewAdditionalProperty(*csch)
		d := mt.Context.TypeResolver.Doc

		asch, err := analysis.Schema(analysis.SchemaOpts{
			Root:     d.Spec(),
			BasePath: d.SpecFilePath(),
			Schema:   csch,
		})
		if err != nil {
			return err
		}
		cp.Required = !asch.IsSimpleSchema && !asch.IsMap

		// when the schema is an array or an alias, this may result in inconsistent
		// nullable status between the map element and the array element (resp. the aliased type).
		//
		// Example: when an object has no property and only additionalProperties,
		// which turn out to be arrays of some other object.

		// save the initial override
		hadOverride := cp.GenSchema.IsMapNullOverride
		if err := cp.makeGenSchema(); err != nil {
			return err
		}

		// if we have an override at the top of stack, propagates it down nested arrays
		if hadOverride && cp.GenSchema.IsArray {
			// do it for nested arrays: override is also about map[string][][]... constructs
			it := &cp.GenSchema
			for it.Items != nil && it.IsArray {
				it.Items.IsMapNullOverride = hadOverride
				it = it.Items
			}
		}
		// cover other cases than arrays (aliased types)
		cp.GenSchema.IsMapNullOverride = hadOverride

		mt.Context.MergeResult(cp, false)
		mt.Context.GenSchema.AdditionalProperties = &cp.GenSchema

		// lift validations
		if (csch.Ref.String() != "" || cp.GenSchema.IsAliased) && !(cp.GenSchema.IsInterface || cp.GenSchema.IsStream) {
			// - we stopped on a ref, or anything else that require we call its Validate() method
			// - if the alias / ref is on an interface (or stream) type: no validation
			mt.Context.GenSchema.HasValidations = true
			mt.Context.GenSchema.AdditionalProperties.HasValidations = true
		}

		debugLog("early mapstack exit, nullable: %t for %s", cp.GenSchema.IsNullable, cp.GenSchema.Name)
		return nil
	}
	cur := mt
	for cur != nil {
		if cur.NewObj != nil {
			// a new model has been created during the stack construction (new ref on anonymous object)
			if err := cur.NewObj.makeGenSchema(); err != nil {
				return err
			}
		}

		if cur.ValueRef != nil {
			if err := cur.ValueRef.makeGenSchema(); err != nil {
				return nil
			}
		}

		if cur.NewObj != nil {
			// newly created model from anonymous object is declared as extra schema
			cur.Context.MergeResult(cur.NewObj, false)

			// propagates extra schemas
			cur.Context.ExtraSchemas[cur.NewObj.Name] = cur.NewObj.GenSchema
		}

		if cur.ValueRef != nil {
			// this is the genSchema for this new anonymous AdditionalProperty
			if err := cur.Context.makeGenSchema(); err != nil {
				return err
			}

			// if there is a ValueRef, we must have a NewObj (from newMapStack() construction)
			cur.ValueRef.GenSchema.HasValidations = cur.NewObj.GenSchema.HasValidations
			cur.Context.MergeResult(cur.ValueRef, false)
			cur.Context.GenSchema.AdditionalProperties = &cur.ValueRef.GenSchema
		}

		if cur.Previous != nil {
			// we have a parent schema: build a schema for current AdditionalProperties
			if err := cur.Context.makeGenSchema(); err != nil {
				return err
			}
		}
		if cur.Next != nil {
			// we previously made a child schema: lifts things from that one
			// - Required is not lifted (in a cascade of maps, only the last element is actually checked for Required)
			cur.Context.MergeResult(cur.Next.Context, false)
			cur.Context.GenSchema.AdditionalProperties = &cur.Next.Context.GenSchema

			// lift validations
			c := &cur.Next.Context.GenSchema
			if (cur.Next.Context.Schema.Ref.String() != "" || c.IsAliased) && !(c.IsInterface || c.IsStream) {
				// - we stopped on a ref, or anything else that require we call its Validate()
				// - if the alias / ref is on an interface (or stream) type: no validation
				cur.Context.GenSchema.HasValidations = true
				cur.Context.GenSchema.AdditionalProperties.HasValidations = true
			}
		}
		if cur.ValueRef != nil {
			cur.Context.MergeResult(cur.ValueRef, false)
			cur.Context.GenSchema.AdditionalProperties = &cur.ValueRef.GenSchema
		}

		if cur.Context.GenSchema.AdditionalProperties != nil {
			// propagate overrides up the resolved schemas, but leaves any ExtraSchema untouched
			cur.Context.GenSchema.AdditionalProperties.IsMapNullOverride = cur.Context.GenSchema.IsMapNullOverride
		}
		cur = cur.Previous
	}

	return nil
}

func (mt *mapStack) HasMore() bool {
	return mt.Type.AdditionalProperties != nil && (mt.Type.AdditionalProperties.Schema != nil || mt.Type.AdditionalProperties.Allows)
}

/* currently unused:
func (mt *mapStack) Dict() map[string]interface{} {
	res := make(map[string]interface{})
	res["context"] = mt.Context.Schema
	if mt.Next != nil {
		res["next"] = mt.Next.Dict()
	}
	if mt.NewObj != nil {
		res["obj"] = mt.NewObj.Schema
	}
	if mt.ValueRef != nil {
		res["value"] = mt.ValueRef.Schema
	}
	return res
}
*/

func (sg *schemaGenContext) buildAdditionalProperties() error {
	if sg.Schema.AdditionalProperties == nil {
		return nil
	}
	addp := *sg.Schema.AdditionalProperties

	wantsAdditional := addp.Schema != nil || addp.Allows
	sg.GenSchema.HasAdditionalProperties = wantsAdditional
	if !wantsAdditional {
		return nil
	}

	// flag swap
	if sg.GenSchema.IsComplexObject {
		sg.GenSchema.IsAdditionalProperties = true
		sg.GenSchema.IsComplexObject = false
		sg.GenSchema.IsMap = false
	}

	if addp.Schema == nil {
		// this is for AdditionalProperties:true|false
		if addp.Allows {
			// additionalProperties: true is rendered as: map[string]interface{}
			addp.Schema = &spec.Schema{}

			addp.Schema.Typed("object", "")
			sg.GenSchema.HasAdditionalProperties = true
			sg.GenSchema.IsComplexObject = false
			sg.GenSchema.IsMap = true

			sg.GenSchema.ValueExpression += "." + swag.ToGoName(sg.Name+" additionalProperties")
			cp := sg.NewAdditionalProperty(*addp.Schema)
			cp.Name += "AdditionalProperties"
			cp.Required = false
			if err := cp.makeGenSchema(); err != nil {
				return err
			}
			sg.MergeResult(cp, false)
			sg.GenSchema.AdditionalProperties = &cp.GenSchema
			debugLog("added interface{} schema for additionalProperties[allows == true], IsInterface=%t", cp.GenSchema.IsInterface)
		}
		return nil
	}

	if !sg.GenSchema.IsMap && (sg.GenSchema.IsAdditionalProperties && sg.Named) {
		// we have a complex object with an AdditionalProperties schema

		tpe, ert := sg.TypeResolver.ResolveSchema(addp.Schema, addp.Schema.Ref.String() == "", false)
		if ert != nil {
			return ert
		}

		if tpe.IsComplexObject && tpe.IsAnonymous {
			// if the AdditionalProperties is an anonymous complex object, generate a new type for it
			pg := sg.makeNewStruct(sg.Name+" Anon", *addp.Schema)
			if err := pg.makeGenSchema(); err != nil {
				return err
			}
			sg.MergeResult(pg, false)
			sg.ExtraSchemas[pg.Name] = pg.GenSchema

			sg.Schema.AdditionalProperties.Schema = spec.RefProperty("#/definitions/" + pg.Name)
			sg.IsVirtual = true

			comprop := sg.NewAdditionalProperty(*sg.Schema.AdditionalProperties.Schema)
			if err := comprop.makeGenSchema(); err != nil {
				return err
			}

			comprop.GenSchema.Required = true
			comprop.GenSchema.HasValidations = true

			comprop.GenSchema.ValueExpression = sg.GenSchema.ValueExpression + "." + swag.ToGoName(sg.GenSchema.Name) + "[" + comprop.KeyVar + "]"

			sg.GenSchema.AdditionalProperties = &comprop.GenSchema
			sg.GenSchema.HasAdditionalProperties = true
			sg.GenSchema.ValueExpression += "." + swag.ToGoName(sg.GenSchema.Name)

			sg.MergeResult(comprop, false)

			return nil
		}

		// this is a regular named schema for AdditionalProperties
		sg.GenSchema.ValueExpression += "." + swag.ToGoName(sg.GenSchema.Name)
		comprop := sg.NewAdditionalProperty(*addp.Schema)
		d := sg.TypeResolver.Doc
		asch, err := analysis.Schema(analysis.SchemaOpts{
			Root:     d.Spec(),
			BasePath: d.SpecFilePath(),
			Schema:   addp.Schema,
		})
		if err != nil {
			return err
		}
		comprop.Required = !asch.IsSimpleSchema && !asch.IsMap
		if err := comprop.makeGenSchema(); err != nil {
			return err
		}

		sg.MergeResult(comprop, false)
		sg.GenSchema.AdditionalProperties = &comprop.GenSchema
		sg.GenSchema.AdditionalProperties.ValueExpression = sg.GenSchema.ValueExpression + "[" + comprop.KeyVar + "]"

		// rewrite value expression for arrays and arrays of arrays in maps (rendered as map[string][][]...)
		if sg.GenSchema.AdditionalProperties.IsArray {
			// maps of slices are where an override may take effect
			sg.GenSchema.AdditionalProperties.Items.IsMapNullOverride = sg.GenSchema.AdditionalProperties.IsMapNullOverride
			sg.GenSchema.AdditionalProperties.Items.ValueExpression = sg.GenSchema.ValueExpression + "[" + comprop.KeyVar + "]" + "[" + sg.GenSchema.AdditionalProperties.IndexVar + "]"
			ap := sg.GenSchema.AdditionalProperties.Items
			for ap != nil && ap.IsArray {
				ap.Items.IsMapNullOverride = ap.IsMapNullOverride
				ap.Items.ValueExpression = ap.ValueExpression + "[" + ap.IndexVar + "]"
				ap = ap.Items
			}
		}

		// lift validation
		if (sg.GenSchema.AdditionalProperties.IsComplexObject || sg.GenSchema.AdditionalProperties.IsAliased || sg.GenSchema.AdditionalProperties.Required) && !(sg.GenSchema.AdditionalProperties.IsInterface || sg.GenSchema.IsStream) {
			sg.GenSchema.HasValidations = true
		}
		return nil
	}

	if sg.GenSchema.IsMap && wantsAdditional {
		// this is itself an AdditionalProperties schema with some AdditionalProperties.
		// this also runs for aliased map types (with zero properties save additionalProperties)
		//
		// find out how deep this rabbit hole goes
		// descend, unwind and rewrite
		// This needs to be depth first, so it first goes as deep as it can and then
		// builds the result in reverse order.
		_, ls, err := newMapStack(sg)
		if err != nil {
			return err
		}
		return ls.Build()
	}

	if sg.GenSchema.IsAdditionalProperties && !sg.Named {
		// for an anonymous object, first build the new object
		// and then replace the current one with a $ref to the
		// new object
		newObj := sg.makeNewStruct(sg.GenSchema.Name+" P"+strconv.Itoa(sg.Index), sg.Schema)
		if err := newObj.makeGenSchema(); err != nil {
			return err
		}

		hasMapNullOverride := sg.GenSchema.IsMapNullOverride
		sg.GenSchema = GenSchema{}
		sg.Schema = *spec.RefProperty("#/definitions/" + newObj.Name)
		if err := sg.makeGenSchema(); err != nil {
			return err
		}
		sg.MergeResult(newObj, false)

		sg.GenSchema.IsMapNullOverride = hasMapNullOverride
		if sg.GenSchema.IsArray {
			sg.GenSchema.Items.IsMapNullOverride = hasMapNullOverride
		}

		sg.GenSchema.HasValidations = newObj.GenSchema.HasValidations
		sg.ExtraSchemas[newObj.Name] = newObj.GenSchema
		return nil
	}
	return nil
}

func (sg *schemaGenContext) makeNewStruct(name string, schema spec.Schema) *schemaGenContext {
	debugLog("making new struct: name: %s, container: %s", name, sg.Container)
	sp := sg.TypeResolver.Doc.Spec()
	name = swag.ToGoName(name)
	if sg.TypeResolver.ModelName != sg.Name {
		name = swag.ToGoName(sg.TypeResolver.ModelName + " " + name)
	}
	if sp.Definitions == nil {
		sp.Definitions = make(spec.Definitions)
	}
	sp.Definitions[name] = schema
	pg := schemaGenContext{
		Path:                       "",
		Name:                       name,
		Receiver:                   sg.Receiver,
		IndexVar:                   "i",
		ValueExpr:                  sg.Receiver,
		Schema:                     schema,
		Required:                   false,
		Named:                      true,
		ExtraSchemas:               make(map[string]GenSchema),
		Discrimination:             sg.Discrimination,
		Container:                  sg.Container,
		IncludeValidator:           sg.IncludeValidator,
		IncludeModel:               sg.IncludeModel,
		StrictAdditionalProperties: sg.StrictAdditionalProperties,
	}
	if schema.Ref.String() == "" {
		pg.TypeResolver = sg.TypeResolver.NewWithModelName(name)
	}
	pg.GenSchema.IsVirtual = true

	sg.ExtraSchemas[name] = pg.GenSchema
	return &pg
}

func (sg *schemaGenContext) buildArray() error {
	tpe, err := sg.TypeResolver.ResolveSchema(sg.Schema.Items.Schema, true, false)
	if err != nil {
		return err
	}

	// check if the element is a complex object, if so generate a new type for it
	if tpe.IsComplexObject && tpe.IsAnonymous {
		pg := sg.makeNewStruct(sg.Name+" items"+strconv.Itoa(sg.Index), *sg.Schema.Items.Schema)
		if err := pg.makeGenSchema(); err != nil {
			return err
		}
		sg.MergeResult(pg, false)
		sg.ExtraSchemas[pg.Name] = pg.GenSchema
		sg.Schema.Items.Schema = spec.RefProperty("#/definitions/" + pg.Name)
		sg.IsVirtual = true
		return sg.makeGenSchema()
	}

	// create the generation schema for items
	elProp := sg.NewSliceBranch(sg.Schema.Items.Schema)

	// when building a slice of maps, the map item is not required
	// items from maps of aliased or nullable type remain required

	// NOTE(fredbi): since this is reset below, this Required = true serves the obscure purpose
	// of indirectly lifting validations from the slice. This is carried on differently now.
	// elProp.Required = true

	if err := elProp.makeGenSchema(); err != nil {
		return err
	}

	sg.MergeResult(elProp, false)

	sg.GenSchema.IsBaseType = elProp.GenSchema.IsBaseType
	sg.GenSchema.ItemsEnum = elProp.GenSchema.Enum
	elProp.GenSchema.Suffix = "Items"

	elProp.GenSchema.IsNullable = tpe.IsNullable && !tpe.HasDiscriminator
	if elProp.GenSchema.IsNullable {
		sg.GenSchema.GoType = "[]*" + elProp.GenSchema.GoType
	} else {
		sg.GenSchema.GoType = "[]" + elProp.GenSchema.GoType
	}

	sg.GenSchema.IsArray = true

	schemaCopy := elProp.GenSchema

	schemaCopy.Required = false

	// validations of items
	hv := hasValidations(sg.Schema.Items.Schema, false)

	// include format validation, excluding binary
	hv = hv || (schemaCopy.IsCustomFormatter && !schemaCopy.IsStream) || (schemaCopy.IsArray && schemaCopy.ElemType.IsCustomFormatter && !schemaCopy.ElemType.IsStream)

	// base types of polymorphic types must be validated
	// NOTE: IsNullable is not useful to figure out a validation: we use Refed and IsAliased below instead
	if hv || elProp.GenSchema.IsBaseType {
		schemaCopy.HasValidations = true
	}

	if (elProp.Schema.Ref.String() != "" || elProp.GenSchema.IsAliased) && !(elProp.GenSchema.IsInterface || elProp.GenSchema.IsStream) {
		schemaCopy.HasValidations = true
	}

	// lift validations
	sg.GenSchema.HasValidations = sg.GenSchema.HasValidations || schemaCopy.HasValidations
	sg.GenSchema.HasSliceValidations = hasSliceValidations(&sg.Schema)

	// prevents bubbling custom formatter flag
	sg.GenSchema.IsCustomFormatter = false

	sg.GenSchema.Items = &schemaCopy
	if sg.Named {
		sg.GenSchema.AliasedType = sg.GenSchema.GoType
	}

	return nil
}

func (sg *schemaGenContext) buildItems() error {
	if sg.Schema.Items == nil {
		// in swagger, arrays MUST have an items schema
		return nil
	}

	// in Items spec, we have either Schema (array) or Schemas (tuple)
	presentsAsSingle := sg.Schema.Items.Schema != nil
	if presentsAsSingle && sg.Schema.AdditionalItems != nil { // unsure if this a valid of invalid schema
		return fmt.Errorf("single schema (%s) can't have additional items", sg.Name)
	}
	if presentsAsSingle {
		return sg.buildArray()
	}

	// This is a tuple, build a new model that represents this
	if sg.Named {
		sg.GenSchema.Name = sg.Name
		sg.GenSchema.GoType = sg.TypeResolver.goTypeName(sg.Name)
		for i, s := range sg.Schema.Items.Schemas {
			elProp := sg.NewTupleElement(&s, i)

			if s.Ref.String() == "" {
				tpe, err := sg.TypeResolver.ResolveSchema(&s, s.Ref.String() == "", true)
				if err != nil {
					return err
				}
				if tpe.IsComplexObject && tpe.IsAnonymous {
					// if the tuple element is an anonymous complex object, build a new type for it
					pg := sg.makeNewStruct(sg.Name+" Items"+strconv.Itoa(i), s)
					if err := pg.makeGenSchema(); err != nil {
						return err
					}
					elProp.Schema = *spec.RefProperty("#/definitions/" + pg.Name)
					elProp.MergeResult(pg, false)
					elProp.ExtraSchemas[pg.Name] = pg.GenSchema
				}
			}

			if err := elProp.makeGenSchema(); err != nil {
				return err
			}
			if elProp.GenSchema.IsInterface || elProp.GenSchema.IsStream {
				elProp.GenSchema.HasValidations = false
			}
			sg.MergeResult(elProp, false)

			elProp.GenSchema.Name = "p" + strconv.Itoa(i)
			sg.GenSchema.Properties = append(sg.GenSchema.Properties, elProp.GenSchema)
			sg.GenSchema.IsTuple = true
		}
		return nil
	}

	// for an anonymous object, first build the new object
	// and then replace the current one with a $ref to the
	// new tuple object
	var sch spec.Schema
	sch.Typed("object", "")
	sch.Properties = make(map[string]spec.Schema, len(sg.Schema.Items.Schemas))
	for i, v := range sg.Schema.Items.Schemas {
		sch.Required = append(sch.Required, "P"+strconv.Itoa(i))
		sch.Properties["P"+strconv.Itoa(i)] = v
	}
	sch.AdditionalItems = sg.Schema.AdditionalItems
	tup := sg.makeNewStruct(sg.GenSchema.Name+"Tuple"+strconv.Itoa(sg.Index), sch)
	tup.IsTuple = true
	if err := tup.makeGenSchema(); err != nil {
		return err
	}
	tup.GenSchema.IsTuple = true
	tup.GenSchema.IsComplexObject = false
	tup.GenSchema.Title = tup.GenSchema.Name + " a representation of an anonymous Tuple type"
	tup.GenSchema.Description = ""
	sg.ExtraSchemas[tup.Name] = tup.GenSchema

	sg.Schema = *spec.RefProperty("#/definitions/" + tup.Name)
	if err := sg.makeGenSchema(); err != nil {
		return err
	}
	sg.MergeResult(tup, false)
	return nil
}

func (sg *schemaGenContext) buildAdditionalItems() error {
	wantsAdditionalItems :=
		sg.Schema.AdditionalItems != nil &&
			(sg.Schema.AdditionalItems.Allows || sg.Schema.AdditionalItems.Schema != nil)

	sg.GenSchema.HasAdditionalItems = wantsAdditionalItems
	if wantsAdditionalItems {
		// check if the element is a complex object, if so generate a new type for it
		tpe, err := sg.TypeResolver.ResolveSchema(sg.Schema.AdditionalItems.Schema, true, true)
		if err != nil {
			return err
		}
		if tpe.IsComplexObject && tpe.IsAnonymous {
			pg := sg.makeNewStruct(sg.Name+" Items", *sg.Schema.AdditionalItems.Schema)
			if err := pg.makeGenSchema(); err != nil {
				return err
			}
			sg.Schema.AdditionalItems.Schema = spec.RefProperty("#/definitions/" + pg.Name)
			pg.GenSchema.HasValidations = true
			sg.MergeResult(pg, false)
			sg.ExtraSchemas[pg.Name] = pg.GenSchema
		}

		it := sg.NewAdditionalItems(sg.Schema.AdditionalItems.Schema)
		// if AdditionalItems are themselves arrays, bump the index var
		if tpe.IsArray {
			it.IndexVar += "i"
		}

		if tpe.IsInterface {
			it.Untyped = true
		}

		if err := it.makeGenSchema(); err != nil {
			return err
		}

		// lift validations when complex is not anonymous or ref'ed
		if (tpe.IsComplexObject || it.Schema.Ref.String() != "") && !(tpe.IsInterface || tpe.IsStream) {
			it.GenSchema.HasValidations = true
		}

		sg.MergeResult(it, true)
		sg.GenSchema.AdditionalItems = &it.GenSchema
	}
	return nil
}

func (sg *schemaGenContext) buildXMLName() error {
	if sg.Schema.XML == nil {
		return nil
	}
	sg.GenSchema.XMLName = sg.Name

	if sg.Schema.XML.Name != "" {
		sg.GenSchema.XMLName = sg.Schema.XML.Name
		if sg.Schema.XML.Attribute {
			sg.GenSchema.XMLName += ",attr"
		}
	}
	return nil
}

func (sg *schemaGenContext) shortCircuitNamedRef() (bool, error) {
	// This if block ensures that a struct gets
	// rendered with the ref as embedded ref.
	//
	// NOTE: this assumes that all $ref point to a definition,
	// i.e. the spec is canonical, as guaranteed by minimal flattening.
	//
	// TODO: RefHandled is actually set nowhere
	if sg.RefHandled || !sg.Named || sg.Schema.Ref.String() == "" {
		return false, nil
	}
	debugLogAsJSON("short circuit named ref: %q", sg.Schema.Ref.String(), sg.Schema)

	// Simple aliased types (arrays, maps and primitives)
	//
	// Before deciding to make a struct with a composition branch (below),
	// check if the $ref points to a simple type or polymorphic (base) type.
	//
	// If this is the case, just realias this simple type, without creating a struct.
	asch, era := analysis.Schema(analysis.SchemaOpts{
		Root:     sg.TypeResolver.Doc.Spec(),
		BasePath: sg.TypeResolver.Doc.SpecFilePath(),
		Schema:   &sg.Schema,
	})
	if era != nil {
		return false, era
	}

	if asch.IsArray || asch.IsMap || asch.IsKnownType || asch.IsBaseType {
		tpx, ers := sg.TypeResolver.ResolveSchema(&sg.Schema, false, true)
		if ers != nil {
			return false, ers
		}
		tpe := resolvedType{}
		tpe.IsMap = asch.IsMap
		tpe.IsArray = asch.IsArray
		tpe.IsPrimitive = asch.IsKnownType

		tpe.IsAliased = true
		tpe.AliasedType = ""
		tpe.IsComplexObject = false
		tpe.IsAnonymous = false
		tpe.IsCustomFormatter = false
		tpe.IsBaseType = tpx.IsBaseType

		tpe.GoType = sg.TypeResolver.goTypeName(path.Base(sg.Schema.Ref.String()))

		tpe.IsNullable = tpx.IsNullable // TODO
		tpe.IsInterface = tpx.IsInterface
		tpe.IsStream = tpx.IsStream

		tpe.SwaggerType = tpx.SwaggerType
		sch := spec.Schema{}
		pg := sg.makeNewStruct(sg.Name, sch)
		if err := pg.makeGenSchema(); err != nil {
			return true, err
		}
		sg.MergeResult(pg, true)
		sg.GenSchema = pg.GenSchema
		sg.GenSchema.resolvedType = tpe
		sg.GenSchema.resolvedType.IsSuperAlias = true
		sg.GenSchema.IsBaseType = tpe.IsBaseType

		return true, nil
	}

	// Aliased object: use golang struct composition.
	// This is rendered as a struct with type field, i.e. :
	// Alias struct {
	//		AliasedType
	// }
	nullableOverride := sg.GenSchema.IsNullable

	tpe := resolvedType{}
	tpe.GoType = sg.TypeResolver.goTypeName(sg.Name)
	tpe.SwaggerType = "object"
	tpe.IsComplexObject = true
	tpe.IsMap = false
	tpe.IsArray = false
	tpe.IsAnonymous = false
	tpe.IsNullable = sg.TypeResolver.IsNullable(&sg.Schema)

	item := sg.NewCompositionBranch(sg.Schema, 0)
	if err := item.makeGenSchema(); err != nil {
		return true, err
	}
	sg.GenSchema.resolvedType = tpe
	sg.GenSchema.IsNullable = sg.GenSchema.IsNullable || nullableOverride
	// prevent format from bubbling up in composed type
	item.GenSchema.IsCustomFormatter = false

	sg.MergeResult(item, true)
	sg.GenSchema.AllOf = append(sg.GenSchema.AllOf, item.GenSchema)
	return true, nil
}

// liftSpecialAllOf attempts to simplify the rendering of allOf constructs by lifting simple things into the current schema.
func (sg *schemaGenContext) liftSpecialAllOf() error {
	// if there is only a $ref or a primitive and an x-isnullable schema then this is a nullable pointer
	// so this should not compose several objects, just 1
	// if there is a ref with a discriminator then we look for x-class on the current definition to know
	// the value of the discriminator to instantiate the class
	if len(sg.Schema.AllOf) < 2 {
		return nil
	}
	var seenSchema int
	var seenNullable bool
	var schemaToLift spec.Schema

	for _, sch := range sg.Schema.AllOf {

		tpe, err := sg.TypeResolver.ResolveSchema(&sch, true, true)
		if err != nil {
			return err
		}
		if sg.TypeResolver.IsNullable(&sch) {
			seenNullable = true
		}
		if len(sch.Type) > 0 || len(sch.Properties) > 0 || sch.Ref.GetURL() != nil || len(sch.AllOf) > 0 {
			seenSchema++
			if seenSchema > 1 {
				// won't do anything if several candidates for a lift
				break
			}
			if (!tpe.IsAnonymous && tpe.IsComplexObject) || tpe.IsPrimitive {
				// lifting complex objects here results in inlined structs in the model
				schemaToLift = sch
			}
		}
	}

	if seenSchema == 1 {
		// when there only a single schema to lift in allOf, replace the schema by its allOf definition
		debugLog("lifted schema in allOf for %s", sg.Name)
		sg.Schema = schemaToLift
		sg.GenSchema.IsNullable = seenNullable
	}
	return nil
}

func (sg *schemaGenContext) buildAliased() error {
	if !sg.GenSchema.IsPrimitive && !sg.GenSchema.IsMap && !sg.GenSchema.IsArray && !sg.GenSchema.IsInterface {
		return nil
	}

	if sg.GenSchema.IsPrimitive {
		if sg.GenSchema.SwaggerType == "string" && sg.GenSchema.SwaggerFormat == "" {
			sg.GenSchema.IsAliased = sg.GenSchema.GoType != sg.GenSchema.SwaggerType
		}
		if sg.GenSchema.IsNullable && sg.Named {
			sg.GenSchema.IsNullable = false
		}
	}

	if sg.GenSchema.IsInterface {
		sg.GenSchema.IsAliased = sg.GenSchema.GoType != iface
	}

	if sg.GenSchema.IsMap {
		sg.GenSchema.IsAliased = !strings.HasPrefix(sg.GenSchema.GoType, "map[")
	}

	if sg.GenSchema.IsArray {
		sg.GenSchema.IsAliased = !strings.HasPrefix(sg.GenSchema.GoType, "[]")
	}
	return nil
}

func (sg *schemaGenContext) GoName() string {
	return goName(&sg.Schema, sg.Name)
}

func goName(sch *spec.Schema, orig string) string {
	name, _ := sch.Extensions.GetString(xGoName)
	if name != "" {
		return name
	}
	return orig
}

func (sg *schemaGenContext) checkNeedsPointer(outer *GenSchema, sch *GenSchema, elem *GenSchema) {
	derefType := strings.TrimPrefix(elem.GoType, "*")
	switch {
	case outer.IsAliased && !strings.HasSuffix(outer.AliasedType, "*"+derefType):
		// override nullability of map of primitive elements: render element of aliased or anonymous map as a pointer
		outer.AliasedType = strings.TrimSuffix(outer.AliasedType, derefType) + "*" + derefType
	case sch != nil:
		// nullable primitive
		if sch.IsAnonymous && !strings.HasSuffix(outer.GoType, "*"+derefType) {
			sch.GoType = strings.TrimSuffix(sch.GoType, derefType) + "*" + derefType
		}
	case outer.IsAnonymous && !strings.HasSuffix(outer.GoType, "*"+derefType):
		outer.GoType = strings.TrimSuffix(outer.GoType, derefType) + "*" + derefType
	}
}

// buildMapOfNullable equalizes the nullablity status for aliased and anonymous maps of simple things,
// with the nullability of its innermost element.
//
// NOTE: at the moment, we decide to align the type of the outer element (map) to the type of the inner element
// The opposite could be done and result in non nullable primitive elements. If we do so, the validation
// code needs to be adapted by removing IsZero() and Required() calls in codegen.
func (sg *schemaGenContext) buildMapOfNullable(sch *GenSchema) {
	outer := &sg.GenSchema
	if sch == nil {
		sch = outer
	}
	if sch.IsMap && (outer.IsAliased || outer.IsAnonymous) {
		elem := sch.AdditionalProperties
		for elem != nil {
			if elem.IsPrimitive && elem.IsNullable {
				sg.checkNeedsPointer(outer, nil, elem)
			} else if elem.IsArray {
				// override nullability of array of primitive elements:
				// render element of aliased or anonyous map as a pointer
				it := elem.Items
				for it != nil {
					if it.IsPrimitive && it.IsNullable {
						sg.checkNeedsPointer(outer, sch, it)
					} else if it.IsMap {
						sg.buildMapOfNullable(it)
					}
					it = it.Items
				}
			}
			elem = elem.AdditionalProperties
		}
	}
}

func (sg *schemaGenContext) makeGenSchema() error {
	debugLogAsJSON("making gen schema (anon: %t, req: %t, tuple: %t) %s\n",
		!sg.Named, sg.Required, sg.IsTuple, sg.Name, sg.Schema)

	ex := ""
	if sg.Schema.Example != nil {
		ex = fmt.Sprintf("%#v", sg.Schema.Example)
	}
	sg.GenSchema.IsExported = true
	sg.GenSchema.Example = ex
	sg.GenSchema.Path = sg.Path
	sg.GenSchema.IndexVar = sg.IndexVar
	sg.GenSchema.Location = body
	sg.GenSchema.ValueExpression = sg.ValueExpr
	sg.GenSchema.KeyVar = sg.KeyVar
	sg.GenSchema.OriginalName = sg.Name
	sg.GenSchema.Name = sg.GoName()
	sg.GenSchema.Title = sg.Schema.Title
	sg.GenSchema.Description = trimBOM(sg.Schema.Description)
	sg.GenSchema.ReceiverName = sg.Receiver
	sg.GenSchema.sharedValidations = sg.schemaValidations()
	sg.GenSchema.ReadOnly = sg.Schema.ReadOnly
	sg.GenSchema.IncludeValidator = sg.IncludeValidator
	sg.GenSchema.IncludeModel = sg.IncludeModel
	sg.GenSchema.StrictAdditionalProperties = sg.StrictAdditionalProperties
	sg.GenSchema.Default = sg.Schema.Default

	var err error
	returns, err := sg.shortCircuitNamedRef()
	if err != nil {
		return err
	}
	if returns {
		return nil
	}
	debugLogAsJSON("after short circuit named ref", sg.Schema)

	if e := sg.liftSpecialAllOf(); e != nil {
		return e
	}
	nullableOverride := sg.GenSchema.IsNullable
	debugLogAsJSON("after lifting special all of", sg.Schema)

	if sg.Container == "" {
		sg.Container = sg.GenSchema.Name
	}
	if e := sg.buildAllOf(); e != nil {
		return e
	}

	var tpe resolvedType
	if sg.Untyped {
		tpe, err = sg.TypeResolver.ResolveSchema(nil, !sg.Named, sg.IsTuple || sg.Required || sg.GenSchema.Required)
	} else {
		tpe, err = sg.TypeResolver.ResolveSchema(&sg.Schema, !sg.Named, sg.IsTuple || sg.Required || sg.GenSchema.Required)
	}
	if err != nil {
		return err
	}

	debugLog("gschema rrequired: %t, nullable: %t", sg.GenSchema.Required, sg.GenSchema.IsNullable)
	tpe.IsNullable = tpe.IsNullable || nullableOverride
	sg.GenSchema.resolvedType = tpe
	sg.GenSchema.IsBaseType = tpe.IsBaseType
	sg.GenSchema.HasDiscriminator = tpe.HasDiscriminator

	// include format validations, excluding binary
	sg.GenSchema.HasValidations = sg.GenSchema.HasValidations || (tpe.IsCustomFormatter && !tpe.IsStream) || (tpe.IsArray && tpe.ElemType != nil && tpe.ElemType.IsCustomFormatter && !tpe.ElemType.IsStream)

	// usage of a polymorphic base type is rendered with getter funcs on private properties.
	// In the case of aliased types, the value expression remains unchanged to the receiver.
	if tpe.IsArray && tpe.ElemType != nil && tpe.ElemType.IsBaseType && sg.GenSchema.ValueExpression != sg.GenSchema.ReceiverName {
		sg.GenSchema.ValueExpression += asMethod
	}

	debugLog("gschema nullable: %t", sg.GenSchema.IsNullable)
	if e := sg.buildAdditionalProperties(); e != nil {
		return e
	}

	// rewrite value expression from top-down
	cur := &sg.GenSchema
	for cur.AdditionalProperties != nil {
		cur.AdditionalProperties.ValueExpression = cur.ValueExpression + "[" + cur.AdditionalProperties.KeyVar + "]"
		cur = cur.AdditionalProperties
	}

	prev := sg.GenSchema
	if sg.Untyped {
		debugLogAsJSON("untyped resolve:%t", sg.Named || sg.IsTuple || sg.Required || sg.GenSchema.Required, sg.Schema)
		tpe, err = sg.TypeResolver.ResolveSchema(nil, !sg.Named, sg.Named || sg.IsTuple || sg.Required || sg.GenSchema.Required)
	} else {
		debugLogAsJSON("typed resolve, isAnonymous(%t), n: %t, t: %t, sgr: %t, sr: %t, isRequired(%t), BaseType(%t)",
			!sg.Named, sg.Named, sg.IsTuple, sg.Required, sg.GenSchema.Required,
			sg.Named || sg.IsTuple || sg.Required || sg.GenSchema.Required, sg.GenSchema.IsBaseType, sg.Schema)
		tpe, err = sg.TypeResolver.ResolveSchema(&sg.Schema, !sg.Named, sg.Named || sg.IsTuple || sg.Required || sg.GenSchema.Required)
	}
	if err != nil {
		return err
	}
	otn := tpe.IsNullable // for debug only
	tpe.IsNullable = tpe.IsNullable || nullableOverride
	sg.GenSchema.resolvedType = tpe
	sg.GenSchema.IsComplexObject = prev.IsComplexObject
	sg.GenSchema.IsMap = prev.IsMap
	sg.GenSchema.IsAdditionalProperties = prev.IsAdditionalProperties
	sg.GenSchema.IsBaseType = sg.GenSchema.HasDiscriminator

	debugLogAsJSON("gschema nnullable:IsNullable:%t,resolver.IsNullable:%t,nullableOverride:%t",
		sg.GenSchema.IsNullable, otn, nullableOverride, sg.Schema)
	if err := sg.buildProperties(); err != nil {
		return err
	}

	if err := sg.buildXMLName(); err != nil {
		return err
	}

	if err := sg.buildAdditionalItems(); err != nil {
		return err
	}

	if err := sg.buildItems(); err != nil {
		return err
	}

	if err := sg.buildAliased(); err != nil {
		return err
	}

	sg.buildMapOfNullable(nil)

	debugLog("finished gen schema for %q", sg.Name)
	return nil
}
