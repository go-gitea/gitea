// +build !go1.11

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

package scan

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/tools/go/loader"

	"github.com/go-openapi/spec"
)

func addExtension(ve *spec.VendorExtensible, key string, value interface{}) {
	if os.Getenv("SWAGGER_GENERATE_EXTENSION") == "false" {
		return
	}

	ve.AddExtension(key, value)
}

type schemaTypable struct {
	schema *spec.Schema
	level  int
}

func (st schemaTypable) Typed(tpe, format string) {
	st.schema.Typed(tpe, format)
}

func (st schemaTypable) SetRef(ref spec.Ref) {
	st.schema.Ref = ref
}

func (st schemaTypable) Schema() *spec.Schema {
	return st.schema
}

func (st schemaTypable) Items() swaggerTypable {
	if st.schema.Items == nil {
		st.schema.Items = new(spec.SchemaOrArray)
	}
	if st.schema.Items.Schema == nil {
		st.schema.Items.Schema = new(spec.Schema)
	}

	st.schema.Typed("array", "")
	return schemaTypable{st.schema.Items.Schema, st.level + 1}
}

func (st schemaTypable) AdditionalProperties() swaggerTypable {
	if st.schema.AdditionalProperties == nil {
		st.schema.AdditionalProperties = new(spec.SchemaOrBool)
	}
	if st.schema.AdditionalProperties.Schema == nil {
		st.schema.AdditionalProperties.Schema = new(spec.Schema)
	}

	st.schema.Typed("object", "")
	return schemaTypable{st.schema.AdditionalProperties.Schema, st.level + 1}
}
func (st schemaTypable) Level() int { return st.level }

type schemaValidations struct {
	current *spec.Schema
}

func (sv schemaValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}
func (sv schemaValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv schemaValidations) SetMultipleOf(val float64)  { sv.current.MultipleOf = &val }
func (sv schemaValidations) SetMinItems(val int64)      { sv.current.MinItems = &val }
func (sv schemaValidations) SetMaxItems(val int64)      { sv.current.MaxItems = &val }
func (sv schemaValidations) SetMinLength(val int64)     { sv.current.MinLength = &val }
func (sv schemaValidations) SetMaxLength(val int64)     { sv.current.MaxLength = &val }
func (sv schemaValidations) SetPattern(val string)      { sv.current.Pattern = val }
func (sv schemaValidations) SetUnique(val bool)         { sv.current.UniqueItems = val }
func (sv schemaValidations) SetDefault(val interface{}) { sv.current.Default = val }
func (sv schemaValidations) SetExample(val interface{}) { sv.current.Example = val }
func (sv schemaValidations) SetEnum(val string) {
	sv.current.Enum = parseEnum(val, &spec.SimpleSchema{Format: sv.current.Format, Type: sv.current.Type[0]})
}

type schemaDecl struct {
	File      *ast.File
	Decl      *ast.GenDecl
	TypeSpec  *ast.TypeSpec
	GoName    string
	Name      string
	annotated bool
}

func newSchemaDecl(file *ast.File, decl *ast.GenDecl, ts *ast.TypeSpec) *schemaDecl {
	sd := &schemaDecl{
		File:     file,
		Decl:     decl,
		TypeSpec: ts,
	}
	sd.inferNames()
	return sd
}

func (sd *schemaDecl) hasAnnotation() bool {
	sd.inferNames()
	return sd.annotated
}

func (sd *schemaDecl) inferNames() (goName string, name string) {
	if sd.GoName != "" {
		goName, name = sd.GoName, sd.Name
		return
	}
	goName = sd.TypeSpec.Name.Name
	name = goName
	if sd.Decl.Doc != nil {
	DECLS:
		for _, cmt := range sd.Decl.Doc.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxModelOverride.FindStringSubmatch(ln)
				if len(matches) > 0 {
					sd.annotated = true
				}
				if len(matches) > 1 && len(matches[1]) > 0 {
					name = matches[1]
					break DECLS
				}
			}
		}
	}
	sd.GoName = goName
	sd.Name = name
	return
}

type schemaParser struct {
	program    *loader.Program
	postDecls  []schemaDecl
	known      map[string]spec.Schema
	discovered *schemaDecl
}

func newSchemaParser(prog *loader.Program) *schemaParser {
	scp := new(schemaParser)
	scp.program = prog
	scp.known = make(map[string]spec.Schema)
	return scp
}

func (scp *schemaParser) Parse(gofile *ast.File, target interface{}) error {
	tgt := target.(map[string]spec.Schema)
	for _, decl := range gofile.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spc := range gd.Specs {
			if ts, ok := spc.(*ast.TypeSpec); ok {
				sd := newSchemaDecl(gofile, gd, ts)
				if err := scp.parseDecl(tgt, sd); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (scp *schemaParser) parseDecl(definitions map[string]spec.Schema, decl *schemaDecl) error {
	// check if there is a swagger:model tag that is followed by a word,
	// this word is the type name for swagger
	// the package and type are recorded in the extensions
	// once type name is found convert it to a schema, by looking up the schema in the
	// definitions dictionary that got passed into this parse method

	// if our schemaParser is parsing a discovered schemaDecl and it does not match
	// the current schemaDecl we can skip parsing.
	if scp.discovered != nil && scp.discovered.Name != decl.Name {
		return nil
	}

	decl.inferNames()
	schema := definitions[decl.Name]
	schPtr := &schema

	// analyze doc comment for the model
	sp := new(sectionedParser)
	sp.setTitle = func(lines []string) { schema.Title = joinDropLast(lines) }
	sp.setDescription = func(lines []string) { schema.Description = joinDropLast(lines) }
	if err := sp.Parse(decl.Decl.Doc); err != nil {
		return err
	}

	// if the type is marked to ignore, just return
	if sp.ignored {
		return nil
	}

	// analyze struct body for fields etc
	// each exported struct field:
	// * gets a type mapped to a go primitive
	// * perhaps gets a format
	// * has to document the validations that apply for the type and the field
	// * when the struct field points to a model it becomes a ref: #/definitions/ModelName
	// * the first line of the comment is the title
	// * the following lines are the description
	switch tpe := decl.TypeSpec.Type.(type) {
	case *ast.StructType:
		if err := scp.parseStructType(decl.File, schPtr, tpe, make(map[string]string)); err != nil {
			return err
		}
	case *ast.InterfaceType:
		if err := scp.parseInterfaceType(decl.File, schPtr, tpe, make(map[string]string)); err != nil {
			return err
		}
	case *ast.Ident:
		prop := &schemaTypable{schPtr, 0}
		if strfmtName, ok := strfmtName(decl.Decl.Doc); ok {
			prop.Typed("string", strfmtName)
		} else {
			if err := scp.parseNamedType(decl.File, tpe, prop); err != nil {
				return err
			}
		}
	case *ast.SelectorExpr:
		prop := &schemaTypable{schPtr, 0}
		if strfmtName, ok := strfmtName(decl.Decl.Doc); ok {
			prop.Typed("string", strfmtName)
		} else {
			if err := scp.parseNamedType(decl.File, tpe, prop); err != nil {
				return err
			}
		}

	case *ast.ArrayType:
		prop := &schemaTypable{schPtr, 0}
		if strfmtName, ok := strfmtName(decl.Decl.Doc); ok {
			prop.Items().Typed("string", strfmtName)
		} else {
			if err := scp.parseNamedType(decl.File, tpe, &schemaTypable{schPtr, 0}); err != nil {
				return err
			}
		}

	case *ast.MapType:
		prop := &schemaTypable{schPtr, 0}
		if strfmtName, ok := strfmtName(decl.Decl.Doc); ok {
			prop.AdditionalProperties().Typed("string", strfmtName)
		} else {
			if err := scp.parseNamedType(decl.File, tpe, &schemaTypable{schPtr, 0}); err != nil {
				return err
			}
		}
	default:
		log.Printf("WARNING: Missing parser for a %T, skipping model: %s\n", tpe, decl.Name)
		return nil
	}

	if schPtr.Ref.String() == "" {
		if decl.Name != decl.GoName {
			addExtension(&schPtr.VendorExtensible, "x-go-name", decl.GoName)
		}
		for _, pkgInfo := range scp.program.AllPackages {
			if pkgInfo.Importable {
				for _, fil := range pkgInfo.Files {
					if fil.Pos() == decl.File.Pos() {
						addExtension(&schPtr.VendorExtensible, "x-go-package", pkgInfo.Pkg.Path())
					}
				}
			}
		}
	}
	definitions[decl.Name] = schema
	return nil
}

func (scp *schemaParser) parseNamedType(gofile *ast.File, expr ast.Expr, prop swaggerTypable) error {
	switch ftpe := expr.(type) {
	case *ast.Ident: // simple value
		pkg, err := scp.packageForFile(gofile, ftpe)
		if err != nil {
			return err
		}
		return scp.parseIdentProperty(pkg, ftpe, prop)

	case *ast.StarExpr: // pointer to something, optional by default
		if err := scp.parseNamedType(gofile, ftpe.X, prop); err != nil {
			return err
		}

	case *ast.ArrayType: // slice type
		if err := scp.parseNamedType(gofile, ftpe.Elt, prop.Items()); err != nil {
			return err
		}

	case *ast.StructType:
		schema := prop.Schema()
		if schema == nil {
			return fmt.Errorf("items doesn't support embedded structs")
		}
		return scp.parseStructType(gofile, prop.Schema(), ftpe, make(map[string]string))

	case *ast.SelectorExpr:
		err := scp.typeForSelector(gofile, ftpe, prop)
		return err

	case *ast.MapType:
		// check if key is a string type, if not print a message
		// and skip the map property. Only maps with string keys can go into additional properties
		sch := prop.Schema()
		if sch == nil {
			return fmt.Errorf("items doesn't support maps")
		}
		if keyIdent, ok := ftpe.Key.(*ast.Ident); sch != nil && ok {
			if keyIdent.Name == "string" {
				if sch.AdditionalProperties == nil {
					sch.AdditionalProperties = new(spec.SchemaOrBool)
				}
				sch.AdditionalProperties.Allows = false
				if sch.AdditionalProperties.Schema == nil {
					sch.AdditionalProperties.Schema = new(spec.Schema)
				}
				if err := scp.parseNamedType(gofile, ftpe.Value, schemaTypable{sch.AdditionalProperties.Schema, 0}); err != nil {
					return err
				}
				sch.Typed("object", "")
			}
		}

	case *ast.InterfaceType:
		prop.Schema().Typed("object", "")
	default:
		pos := "unknown file:unknown position"
		if scp != nil {
			if scp.program != nil {
				if scp.program.Fset != nil {
					pos = scp.program.Fset.Position(expr.Pos()).String()
				}
			}
		}
		return fmt.Errorf("expr (%s) is unsupported for a schema", pos)
	}
	return nil
}

func (scp *schemaParser) parseEmbeddedType(gofile *ast.File, schema *spec.Schema, expr ast.Expr, seenPreviously map[string]string) error {
	switch tpe := expr.(type) {
	case *ast.Ident:
		// do lookup of type
		// take primitives into account, they should result in an error for swagger
		pkg, err := scp.packageForFile(gofile, tpe)
		if err != nil {
			return err
		}
		file, _, ts, err := findSourceFile(pkg, tpe.Name)
		if err != nil {
			return err
		}

		switch st := ts.Type.(type) {
		case *ast.StructType:
			return scp.parseStructType(file, schema, st, seenPreviously)
		case *ast.InterfaceType:
			return scp.parseInterfaceType(file, schema, st, seenPreviously)
		default:
			prop := &schemaTypable{schema, 0}
			return scp.parseNamedType(gofile, st, prop)
		}

	case *ast.SelectorExpr:
		// look up package, file and then type
		pkg, err := scp.packageForSelector(gofile, tpe.X)
		if err != nil {
			return fmt.Errorf("embedded struct: %v", err)
		}
		file, _, ts, err := findSourceFile(pkg, tpe.Sel.Name)
		if err != nil {
			return fmt.Errorf("embedded struct: %v", err)
		}
		if st, ok := ts.Type.(*ast.StructType); ok {
			return scp.parseStructType(file, schema, st, seenPreviously)
		}
		if st, ok := ts.Type.(*ast.InterfaceType); ok {
			return scp.parseInterfaceType(file, schema, st, seenPreviously)
		}
	case *ast.StarExpr:
		return scp.parseEmbeddedType(gofile, schema, tpe.X, seenPreviously)
	default:
		return fmt.Errorf(
			"parseEmbeddedType: unsupported type %v at position %#v",
			expr,
			scp.program.Fset.Position(tpe.Pos()),
		)
	}
	return fmt.Errorf("unable to resolve embedded struct for: %v", expr)
}

func (scp *schemaParser) parseAllOfMember(gofile *ast.File, schema *spec.Schema, expr ast.Expr, seenPreviously map[string]string) error {
	// TODO: check if struct is annotated with swagger:model or known in the definitions otherwise
	var pkg *loader.PackageInfo
	var file *ast.File
	var gd *ast.GenDecl
	var ts *ast.TypeSpec
	var err error

	switch tpe := expr.(type) {
	case *ast.Ident:
		// do lookup of type
		// take primitives into account, they should result in an error for swagger
		pkg, err = scp.packageForFile(gofile, tpe)
		if err != nil {
			return err
		}
		file, gd, ts, err = findSourceFile(pkg, tpe.Name)
		if err != nil {
			return err
		}

	case *ast.SelectorExpr:
		// look up package, file and then type
		pkg, err = scp.packageForSelector(gofile, tpe.X)
		if err != nil {
			return fmt.Errorf("embedded struct: %v", err)
		}
		file, gd, ts, err = findSourceFile(pkg, tpe.Sel.Name)
		if err != nil {
			return fmt.Errorf("embedded struct: %v", err)
		}
	default:
		return fmt.Errorf("unable to resolve allOf member for: %v", expr)
	}

	sd := newSchemaDecl(file, gd, ts)
	if sd.hasAnnotation() && pkg.String() != "time" && ts.Name.Name != "Time" {
		ref, err := spec.NewRef("#/definitions/" + sd.Name)
		if err != nil {
			return err
		}
		schema.Ref = ref
		scp.postDecls = append(scp.postDecls, *sd)
	} else {
		switch st := ts.Type.(type) {
		case *ast.StructType:
			return scp.parseStructType(file, schema, st, seenPreviously)
		case *ast.InterfaceType:
			return scp.parseInterfaceType(file, schema, st, seenPreviously)
		}
	}

	return nil
}
func (scp *schemaParser) parseInterfaceType(gofile *ast.File, bschema *spec.Schema, tpe *ast.InterfaceType, seenPreviously map[string]string) error {
	if tpe.Methods == nil {
		return nil
	}

	// first check if this has embedded interfaces, if so make sure to refer to those by ref
	// when they are decorated with an allOf annotation
	// go over the method list again and this time collect the nullary methods and parse the comments
	// as if they are properties on a struct
	var schema *spec.Schema
	seenProperties := seenPreviously
	hasAllOf := false

	for _, fld := range tpe.Methods.List {
		if len(fld.Names) == 0 {
			// if this created an allOf property then we have to rejig the schema var
			// because all the fields collected that aren't from embedded structs should go in
			// their own proper schema
			// first process embedded structs in order of embedding
			if allOfMember(fld.Doc) {
				hasAllOf = true
				if schema == nil {
					schema = new(spec.Schema)
				}
				var newSch spec.Schema
				// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
				// otherwise the fields will just be included as normal properties
				if err := scp.parseAllOfMember(gofile, &newSch, fld.Type, seenProperties); err != nil {
					return err
				}

				if fld.Doc != nil {
					for _, cmt := range fld.Doc.List {
						for _, ln := range strings.Split(cmt.Text, "\n") {
							matches := rxAllOf.FindStringSubmatch(ln)
							ml := len(matches)
							if ml > 1 {
								mv := matches[ml-1]
								if mv != "" {
									addExtension(&bschema.VendorExtensible, "x-class", mv)
								}
							}
						}
					}
				}

				bschema.AllOf = append(bschema.AllOf, newSch)
				continue
			}

			var newSch spec.Schema
			// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
			// otherwise the fields will just be included as normal properties
			if err := scp.parseEmbeddedType(gofile, &newSch, fld.Type, seenProperties); err != nil {
				return err
			}
			bschema.AllOf = append(bschema.AllOf, newSch)
			hasAllOf = true
		}
	}

	if schema == nil {
		schema = bschema
	}
	// then add and possibly override values
	if schema.Properties == nil {
		schema.Properties = make(map[string]spec.Schema)
	}
	schema.Typed("object", "")
	for _, fld := range tpe.Methods.List {
		if mtpe, ok := fld.Type.(*ast.FuncType); ok && mtpe.Params.NumFields() == 0 && mtpe.Results.NumFields() == 1 {
			gnm := fld.Names[0].Name
			nm := gnm
			if fld.Doc != nil {
				for _, cmt := range fld.Doc.List {
					for _, ln := range strings.Split(cmt.Text, "\n") {
						matches := rxName.FindStringSubmatch(ln)
						ml := len(matches)
						if ml > 1 {
							nm = matches[ml-1]
						}
					}
				}
			}

			ps := schema.Properties[nm]
			if err := parseProperty(scp, gofile, mtpe.Results.List[0].Type, schemaTypable{&ps, 0}); err != nil {
				return err
			}

			if err := scp.createParser(nm, schema, &ps, fld).Parse(fld.Doc); err != nil {
				return err
			}

			if ps.Ref.String() == "" && nm != gnm {
				addExtension(&ps.VendorExtensible, "x-go-name", gnm)
			}
			seenProperties[nm] = gnm
			schema.Properties[nm] = ps
		}

	}
	if schema != nil && hasAllOf && len(schema.Properties) > 0 {
		bschema.AllOf = append(bschema.AllOf, *schema)
	}
	for k := range schema.Properties {
		if _, ok := seenProperties[k]; !ok {
			delete(schema.Properties, k)
		}
	}
	return nil
}

func (scp *schemaParser) parseStructType(gofile *ast.File, bschema *spec.Schema, tpe *ast.StructType, seenPreviously map[string]string) error {
	if tpe.Fields == nil {
		return nil
	}
	var schema *spec.Schema
	seenProperties := seenPreviously
	hasAllOf := false

	for _, fld := range tpe.Fields.List {
		if len(fld.Names) == 0 {
			// if the field is annotated with swagger:ignore, ignore it
			if ignored(fld.Doc) {
				continue
			}

			_, ignore, _, err := parseJSONTag(fld)
			if err != nil {
				return err
			}
			if ignore {
				continue
			}

			// if this created an allOf property then we have to rejig the schema var
			// because all the fields collected that aren't from embedded structs should go in
			// their own proper schema
			// first process embedded structs in order of embedding
			if allOfMember(fld.Doc) {
				hasAllOf = true
				if schema == nil {
					schema = new(spec.Schema)
				}
				var newSch spec.Schema
				// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
				// otherwise the fields will just be included as normal properties
				if err := scp.parseAllOfMember(gofile, &newSch, fld.Type, seenProperties); err != nil {
					return err
				}

				if fld.Doc != nil {
					for _, cmt := range fld.Doc.List {
						for _, ln := range strings.Split(cmt.Text, "\n") {
							matches := rxAllOf.FindStringSubmatch(ln)
							ml := len(matches)
							if ml > 1 {
								mv := matches[ml-1]
								if mv != "" {
									addExtension(&bschema.VendorExtensible, "x-class", mv)
								}
							}
						}
					}
				}

				bschema.AllOf = append(bschema.AllOf, newSch)
				continue
			}
			if schema == nil {
				schema = bschema
			}

			// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
			// otherwise the fields will just be included as normal properties
			if err := scp.parseEmbeddedType(gofile, schema, fld.Type, seenProperties); err != nil {
				return err
			}
		}
	}
	if schema == nil {
		schema = bschema
	}

	// then add and possibly override values
	if schema.Properties == nil {
		schema.Properties = make(map[string]spec.Schema)
	}
	schema.Typed("object", "")
	for _, fld := range tpe.Fields.List {
		if len(fld.Names) > 0 && fld.Names[0] != nil && fld.Names[0].IsExported() {
			// if the field is annotated with swagger:ignore, ignore it
			if ignored(fld.Doc) {
				continue
			}

			gnm := fld.Names[0].Name
			nm, ignore, isString, err := parseJSONTag(fld)
			if err != nil {
				return err
			}
			if ignore {
				for seenTagName, seenFieldName := range seenPreviously {
					if seenFieldName == gnm {
						delete(schema.Properties, seenTagName)
						break
					}
				}
				continue
			}

			ps := schema.Properties[nm]
			if err := parseProperty(scp, gofile, fld.Type, schemaTypable{&ps, 0}); err != nil {
				return err
			}
			if isString {
				ps.Typed("string", ps.Format)
				ps.Ref = spec.Ref{}
			}
			if strfmtName, ok := strfmtName(fld.Doc); ok {
				ps.Typed("string", strfmtName)
				ps.Ref = spec.Ref{}
			}

			if err := scp.createParser(nm, schema, &ps, fld).Parse(fld.Doc); err != nil {
				return err
			}

			if ps.Ref.String() == "" && nm != gnm {
				addExtension(&ps.VendorExtensible, "x-go-name", gnm)
			}
			// we have 2 cases:
			// 1. field with different name override tag
			// 2. field with different name removes tag
			// so we need to save both tag&name
			seenProperties[nm] = gnm
			schema.Properties[nm] = ps
		}
	}
	if schema != nil && hasAllOf && len(schema.Properties) > 0 {
		bschema.AllOf = append(bschema.AllOf, *schema)
	}
	for k := range schema.Properties {
		if _, ok := seenProperties[k]; !ok {
			delete(schema.Properties, k)
		}
	}
	return nil
}

func schemaVendorExtensibleSetter(meta *spec.Schema) func(json.RawMessage) error {
	return func(jsonValue json.RawMessage) error {
		var jsonData spec.Extensions
		err := json.Unmarshal(jsonValue, &jsonData)
		if err != nil {
			return err
		}
		for k := range jsonData {
			if !rxAllowedExtensions.MatchString(k) {
				return fmt.Errorf("invalid schema extension name, should start from `x-`: %s", k)
			}
		}
		meta.Extensions = jsonData
		return nil
	}
}

func (scp *schemaParser) createParser(nm string, schema, ps *spec.Schema, fld *ast.Field) *sectionedParser {
	sp := new(sectionedParser)

	schemeType, err := ps.Type.MarshalJSON()
	if err != nil {
		return nil
	}

	if ps.Ref.String() == "" {
		sp.setDescription = func(lines []string) { ps.Description = joinDropLast(lines) }
		sp.taggers = []tagParser{
			newSingleLineTagParser("maximum", &setMaximum{schemaValidations{ps}, rxf(rxMaximumFmt, "")}),
			newSingleLineTagParser("minimum", &setMinimum{schemaValidations{ps}, rxf(rxMinimumFmt, "")}),
			newSingleLineTagParser("multipleOf", &setMultipleOf{schemaValidations{ps}, rxf(rxMultipleOfFmt, "")}),
			newSingleLineTagParser("minLength", &setMinLength{schemaValidations{ps}, rxf(rxMinLengthFmt, "")}),
			newSingleLineTagParser("maxLength", &setMaxLength{schemaValidations{ps}, rxf(rxMaxLengthFmt, "")}),
			newSingleLineTagParser("pattern", &setPattern{schemaValidations{ps}, rxf(rxPatternFmt, "")}),
			newSingleLineTagParser("minItems", &setMinItems{schemaValidations{ps}, rxf(rxMinItemsFmt, "")}),
			newSingleLineTagParser("maxItems", &setMaxItems{schemaValidations{ps}, rxf(rxMaxItemsFmt, "")}),
			newSingleLineTagParser("unique", &setUnique{schemaValidations{ps}, rxf(rxUniqueFmt, "")}),
			newSingleLineTagParser("enum", &setEnum{schemaValidations{ps}, rxf(rxEnumFmt, "")}),
			newSingleLineTagParser("default", &setDefault{&spec.SimpleSchema{Type: string(schemeType)}, schemaValidations{ps}, rxf(rxDefaultFmt, "")}),
			newSingleLineTagParser("type", &setDefault{&spec.SimpleSchema{Type: string(schemeType)}, schemaValidations{ps}, rxf(rxDefaultFmt, "")}),
			newSingleLineTagParser("example", &setExample{&spec.SimpleSchema{Type: string(schemeType)}, schemaValidations{ps}, rxf(rxExampleFmt, "")}),
			newSingleLineTagParser("required", &setRequiredSchema{schema, nm}),
			newSingleLineTagParser("readOnly", &setReadOnlySchema{ps}),
			newSingleLineTagParser("discriminator", &setDiscriminator{schema, nm}),
			newMultiLineTagParser("YAMLExtensionsBlock", newYamlParser(rxExtensions, schemaVendorExtensibleSetter(ps)), true),
		}

		itemsTaggers := func(items *spec.Schema, level int) []tagParser {
			schemeType, err := items.Type.MarshalJSON()
			if err != nil {
				return nil
			}
			// the expression is 1-index based not 0-index
			itemsPrefix := fmt.Sprintf(rxItemsPrefixFmt, level+1)
			return []tagParser{
				newSingleLineTagParser(fmt.Sprintf("items%dMaximum", level), &setMaximum{schemaValidations{items}, rxf(rxMaximumFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dMinimum", level), &setMinimum{schemaValidations{items}, rxf(rxMinimumFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dMultipleOf", level), &setMultipleOf{schemaValidations{items}, rxf(rxMultipleOfFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dMinLength", level), &setMinLength{schemaValidations{items}, rxf(rxMinLengthFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dMaxLength", level), &setMaxLength{schemaValidations{items}, rxf(rxMaxLengthFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dPattern", level), &setPattern{schemaValidations{items}, rxf(rxPatternFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dMinItems", level), &setMinItems{schemaValidations{items}, rxf(rxMinItemsFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dMaxItems", level), &setMaxItems{schemaValidations{items}, rxf(rxMaxItemsFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dUnique", level), &setUnique{schemaValidations{items}, rxf(rxUniqueFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dEnum", level), &setEnum{schemaValidations{items}, rxf(rxEnumFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dDefault", level), &setDefault{&spec.SimpleSchema{Type: string(schemeType)}, schemaValidations{items}, rxf(rxDefaultFmt, itemsPrefix)}),
				newSingleLineTagParser(fmt.Sprintf("items%dExample", level), &setExample{&spec.SimpleSchema{Type: string(schemeType)}, schemaValidations{items}, rxf(rxExampleFmt, itemsPrefix)}),
			}
		}

		var parseArrayTypes func(expr ast.Expr, items *spec.SchemaOrArray, level int) ([]tagParser, error)
		parseArrayTypes = func(expr ast.Expr, items *spec.SchemaOrArray, level int) ([]tagParser, error) {
			if items == nil || items.Schema == nil {
				return []tagParser{}, nil
			}
			switch iftpe := expr.(type) {
			case *ast.ArrayType:
				eleTaggers := itemsTaggers(items.Schema, level)
				sp.taggers = append(eleTaggers, sp.taggers...)
				otherTaggers, err := parseArrayTypes(iftpe.Elt, items.Schema.Items, level+1)
				if err != nil {
					return nil, err
				}
				return otherTaggers, nil
			case *ast.Ident:
				taggers := []tagParser{}
				if iftpe.Obj == nil {
					taggers = itemsTaggers(items.Schema, level)
				}
				otherTaggers, err := parseArrayTypes(expr, items.Schema.Items, level+1)
				if err != nil {
					return nil, err
				}
				return append(taggers, otherTaggers...), nil
			case *ast.StarExpr:
				otherTaggers, err := parseArrayTypes(iftpe.X, items, level)
				if err != nil {
					return nil, err
				}
				return otherTaggers, nil
			default:
				return nil, fmt.Errorf("unknown field type ele for %q", nm)
			}
		}
		// check if this is a primitive, if so parse the validations from the
		// doc comments of the slice declaration.
		if ftped, ok := fld.Type.(*ast.ArrayType); ok {
			taggers, err := parseArrayTypes(ftped.Elt, ps.Items, 0)
			if err != nil {
				return sp
			}
			sp.taggers = append(taggers, sp.taggers...)
		}

	} else {
		sp.taggers = []tagParser{
			newSingleLineTagParser("required", &setRequiredSchema{schema, nm}),
		}
	}
	return sp
}

// hasFilePathPrefix reports whether the filesystem path s begins with the
// elements in prefix.
//
// taken from: https://github.com/golang/go/blob/c87520c5981ecdeaa99e7ba636a6088f900c0c75/src/cmd/go/internal/load/path.go#L60-L80
func hasFilePathPrefix(s, prefix string) bool {
	sv := strings.ToUpper(filepath.VolumeName(s))
	pv := strings.ToUpper(filepath.VolumeName(prefix))
	s = s[len(sv):]
	prefix = prefix[len(pv):]
	switch {
	default:
		return false
	case sv != pv:
		return false
	case len(s) == len(prefix):
		return s == prefix
	case len(s) > len(prefix):
		if prefix != "" && prefix[len(prefix)-1] == filepath.Separator {
			return strings.HasPrefix(s, prefix)
		}
		return s[len(prefix)] == filepath.Separator && s[:len(prefix)] == prefix
	}
}

func (scp *schemaParser) packageForFile(gofile *ast.File, tpe *ast.Ident) (*loader.PackageInfo, error) {
	fn := scp.program.Fset.File(gofile.Pos()).Name()
	if Debug {
		log.Println("trying for", fn, tpe.Name, tpe.String())
	}
	fa, err := filepath.Abs(fn)
	if err != nil {
		return nil, err
	}
	if Debug {
		log.Println("absolute path", fa)
	}
	var fgp string
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(os.Getenv("HOME"), "go")
	}
	for _, p := range append(filepath.SplitList(gopath), runtime.GOROOT()) {
		pref := filepath.Join(p, "src")
		if hasFilePathPrefix(fa, pref) {
			fgp = filepath.Dir(strings.TrimPrefix(fa, pref))[1:]
			break
		}
	}
	if Debug {
		log.Println("package in gopath", fgp)
	}
	for pkg, pkgInfo := range scp.program.AllPackages {
		if Debug {
			log.Println("inferring for", tpe.Name, "with", gofile.Name.Name, "at", pkg.Path(), "against", filepath.ToSlash(fgp))
		}
		if pkg.Name() == gofile.Name.Name && filepath.ToSlash(fgp) == pkg.Path() {
			return pkgInfo, nil
		}
	}

	return nil, fmt.Errorf("unable to determine package for %s", fn)
}

func (scp *schemaParser) packageForSelector(gofile *ast.File, expr ast.Expr) (*loader.PackageInfo, error) {

	if pth, ok := expr.(*ast.Ident); ok {
		// lookup import
		var selPath string
		for _, imp := range gofile.Imports {
			pv, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				pv = imp.Path.Value
			}
			if imp.Name != nil {
				if imp.Name.Name == pth.Name {
					selPath = pv
					break
				}
			} else {
				pkg := scp.program.Package(pv)
				if pkg != nil && pth.Name == pkg.Pkg.Name() {
					selPath = pv
					break
				} else {
					parts := strings.Split(pv, "/")
					if len(parts) > 0 && parts[len(parts)-1] == pth.Name {
						selPath = pv
						break
					}
				}
			}
		}
		// find actual struct
		if selPath == "" {
			return nil, fmt.Errorf("no import found for %s", pth.Name)
		}

		pkg := scp.program.Package(selPath)
		if pkg != nil {
			return pkg, nil
		}
		// TODO: I must admit this made me cry, it's not even a great solution.
		pkg = scp.program.Package("github.com/go-swagger/go-swagger/vendor/" + selPath)
		if pkg != nil {
			return pkg, nil
		}
		for _, info := range scp.program.AllPackages {
			n := info.String()
			path := "/vendor/" + selPath
			if strings.HasSuffix(n, path) {
				pkg = scp.program.Package(n)
				return pkg, nil
			}
		}
	}
	return nil, fmt.Errorf("can't determine selector path from %v", expr)
}

func (scp *schemaParser) makeRef(file *ast.File, pkg *loader.PackageInfo, gd *ast.GenDecl, ts *ast.TypeSpec, prop swaggerTypable) error {
	sd := newSchemaDecl(file, gd, ts)
	sd.inferNames()
	// make an exception for time.Time because this is a well-known string format
	if sd.Name == "Time" && pkg.String() == "time" {
		return nil
	}
	ref, err := spec.NewRef("#/definitions/" + sd.Name)
	if err != nil {
		return err
	}
	prop.SetRef(ref)
	scp.postDecls = append(scp.postDecls, *sd)
	return nil
}

func (scp *schemaParser) parseIdentProperty(pkg *loader.PackageInfo, expr *ast.Ident, prop swaggerTypable) error {
	// before proceeding make an exception to time.Time because it is a well known string format
	if pkg.String() == "time" && expr.String() == "Time" {
		prop.Typed("string", "date-time")
		return nil
	}

	// find the file this selector points to
	file, gd, ts, err := findSourceFile(pkg, expr.Name)

	if err != nil {
		err := swaggerSchemaForType(expr.Name, prop)
		if err != nil {
			return fmt.Errorf("package %s, error is: %v", pkg.String(), err)
		}
		return nil
	}

	if at, ok := ts.Type.(*ast.ArrayType); ok {
		// the swagger spec defines strfmt base64 as []byte.
		// in that case we don't actually want to turn it into an array
		// but we want to turn it into a string
		if _, ok := at.Elt.(*ast.Ident); ok {
			if strfmtName, ok := strfmtName(gd.Doc); ok {
				prop.Typed("string", strfmtName)
				return nil
			}
		}
		// this is a selector, so most likely not base64
		if strfmtName, ok := strfmtName(gd.Doc); ok {
			prop.Items().Typed("string", strfmtName)
			return nil
		}
	}

	// look at doc comments for swagger:strfmt [name]
	// when found this is the format name, create a schema with that name
	if strfmtName, ok := strfmtName(gd.Doc); ok {
		prop.Typed("string", strfmtName)
		return nil
	}

	if enumName, ok := enumName(gd.Doc); ok {
		log.Println(enumName)
		return nil
	}

	if defaultName, ok := defaultName(gd.Doc); ok {
		log.Println(defaultName)
		return nil
	}

	if typeName, ok := typeName(gd.Doc); ok {
		_ = swaggerSchemaForType(typeName, prop)
		return nil
	}

	if isAliasParam(prop) || aliasParam(gd.Doc) {
		itype, ok := ts.Type.(*ast.Ident)
		if ok {
			err := swaggerSchemaForType(itype.Name, prop)
			if err == nil {
				return nil
			}
		}
	}
	switch tpe := ts.Type.(type) {
	case *ast.ArrayType:
		return scp.makeRef(file, pkg, gd, ts, prop)
	case *ast.StructType:
		return scp.makeRef(file, pkg, gd, ts, prop)

	case *ast.Ident:
		return scp.makeRef(file, pkg, gd, ts, prop)

	case *ast.StarExpr:
		return parseProperty(scp, file, tpe.X, prop)

	case *ast.SelectorExpr:
		// return scp.refForSelector(file, gd, tpe, ts, prop)
		return scp.makeRef(file, pkg, gd, ts, prop)

	case *ast.InterfaceType:
		return scp.makeRef(file, pkg, gd, ts, prop)

	case *ast.MapType:
		return scp.makeRef(file, pkg, gd, ts, prop)

	default:
		err := swaggerSchemaForType(expr.Name, prop)
		if err != nil {
			return fmt.Errorf("package %s, error is: %v", pkg.String(), err)
		}
		return nil
	}

}

func (scp *schemaParser) typeForSelector(gofile *ast.File, expr *ast.SelectorExpr, prop swaggerTypable) error {
	pkg, err := scp.packageForSelector(gofile, expr.X)
	if err != nil {
		return err
	}

	return scp.parseIdentProperty(pkg, expr.Sel, prop)
}

func findSourceFile(pkg *loader.PackageInfo, typeName string) (*ast.File, *ast.GenDecl, *ast.TypeSpec, error) {
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			if gd, ok := decl.(*ast.GenDecl); ok {
				for _, gs := range gd.Specs {
					if ts, ok := gs.(*ast.TypeSpec); ok {
						strfmtNme, isStrfmt := strfmtName(gd.Doc)
						if (isStrfmt && strfmtNme == typeName) || ts.Name != nil && ts.Name.Name == typeName {
							return file, gd, ts, nil
						}
					}
				}
			}
		}
	}
	return nil, nil, nil, fmt.Errorf("unable to find %s in %s", typeName, pkg.String())
}

func allOfMember(comments *ast.CommentGroup) bool {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				if rxAllOf.MatchString(ln) {
					return true
				}
			}
		}
	}
	return false
}

func fileParam(comments *ast.CommentGroup) bool {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				if rxFileUpload.MatchString(ln) {
					return true
				}
			}
		}
	}
	return false
}

func strfmtName(comments *ast.CommentGroup) (string, bool) {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxStrFmt.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
			}
		}
	}
	return "", false
}

func ignored(comments *ast.CommentGroup) bool {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				if rxIgnoreOverride.MatchString(ln) {
					return true
				}
			}
		}
	}
	return false
}

func enumName(comments *ast.CommentGroup) (string, bool) {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxEnum.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
			}
		}
	}
	return "", false
}

func aliasParam(comments *ast.CommentGroup) bool {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				if rxAlias.MatchString(ln) {
					return true
				}
			}
		}
	}
	return false
}

func defaultName(comments *ast.CommentGroup) (string, bool) {
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxDefault.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
			}
		}
	}
	return "", false
}

func typeName(comments *ast.CommentGroup) (string, bool) {

	var typ string
	if comments != nil {
		for _, cmt := range comments.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxType.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					typ = strings.TrimSpace(matches[1])
					return typ, true
				}
			}
		}
	}
	return "", false
}

func parseProperty(scp *schemaParser, gofile *ast.File, fld ast.Expr, prop swaggerTypable) error {
	switch ftpe := fld.(type) {
	case *ast.Ident: // simple value
		pkg, err := scp.packageForFile(gofile, ftpe)
		if err != nil {
			return err
		}
		return scp.parseIdentProperty(pkg, ftpe, prop)

	case *ast.StarExpr: // pointer to something, optional by default
		if err := parseProperty(scp, gofile, ftpe.X, prop); err != nil {
			return err
		}

	case *ast.ArrayType: // slice type
		if err := parseProperty(scp, gofile, ftpe.Elt, prop.Items()); err != nil {
			return err
		}

	case *ast.StructType:
		schema := prop.Schema()
		if schema == nil {
			return fmt.Errorf("items doesn't support embedded structs")
		}
		return scp.parseStructType(gofile, prop.Schema(), ftpe, make(map[string]string))

	case *ast.SelectorExpr:
		err := scp.typeForSelector(gofile, ftpe, prop)
		return err

	case *ast.MapType:
		// check if key is a string type, if not print a message
		// and skip the map property. Only maps with string keys can go into additional properties
		sch := prop.Schema()
		if sch == nil {
			return fmt.Errorf("items doesn't support maps")
		}
		if keyIdent, ok := ftpe.Key.(*ast.Ident); sch != nil && ok {
			if keyIdent.Name == "string" {
				if sch.AdditionalProperties == nil {
					sch.AdditionalProperties = new(spec.SchemaOrBool)
				}
				sch.AdditionalProperties.Allows = false
				if sch.AdditionalProperties.Schema == nil {
					sch.AdditionalProperties.Schema = new(spec.Schema)
				}
				if err := parseProperty(scp, gofile, ftpe.Value, schemaTypable{sch.AdditionalProperties.Schema, 0}); err != nil {
					return err
				}
				sch.Typed("object", "")
			}
		}

	case *ast.InterfaceType:
		prop.Schema().Typed("object", "")
	default:
		pos := "unknown file:unknown position"
		if scp != nil {
			if scp.program != nil {
				if scp.program.Fset != nil {
					pos = scp.program.Fset.Position(fld.Pos()).String()
				}
			}
		}
		return fmt.Errorf("Expr (%s) is unsupported for a schema", pos)
	}
	return nil
}

func parseJSONTag(field *ast.Field) (name string, ignore bool, isString bool, err error) {
	if len(field.Names) > 0 {
		name = field.Names[0].Name
	}
	if field.Tag != nil && len(strings.TrimSpace(field.Tag.Value)) > 0 {
		tv, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			return name, false, false, err
		}

		if strings.TrimSpace(tv) != "" {
			st := reflect.StructTag(tv)
			jsonParts := strings.Split(st.Get("json"), ",")
			jsonName := jsonParts[0]

			if len(jsonParts) > 1 && jsonParts[1] == "string" {
				isString = isFieldStringable(field.Type)
			}

			if jsonName == "-" {
				return name, true, isString, nil
			} else if jsonName != "" {
				return jsonName, false, isString, nil
			}
		}
	}
	return name, false, false, nil
}

// isFieldStringable check if the field type is a scalar. If the field type is
// *ast.StarExpr and is pointer type, check if it refers to a scalar.
// Otherwise, the ",string" directive doesn't apply.
func isFieldStringable(tpe ast.Expr) bool {
	if ident, ok := tpe.(*ast.Ident); ok {
		switch ident.Name {
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float64", "string", "bool":
			return true
		}
	} else if starExpr, ok := tpe.(*ast.StarExpr); ok {
		return isFieldStringable(starExpr.X)
	} else {
		return false
	}
	return false
}
