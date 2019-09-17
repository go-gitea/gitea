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
	"fmt"
	"go/ast"
	"strings"

	"github.com/go-openapi/spec"
	"golang.org/x/tools/go/loader"
)

type operationValidationBuilder interface {
	validationBuilder
	SetCollectionFormat(string)
}

type paramTypable struct {
	param *spec.Parameter
}

func (pt paramTypable) Level() int { return 0 }

func (pt paramTypable) Typed(tpe, format string) {
	pt.param.Typed(tpe, format)
}

func (pt paramTypable) SetRef(ref spec.Ref) {
	pt.param.Ref = ref
}

func (pt paramTypable) Items() swaggerTypable {
	bdt, schema := bodyTypable(pt.param.In, pt.param.Schema)
	if bdt != nil {
		pt.param.Schema = schema
		return bdt
	}

	if pt.param.Items == nil {
		pt.param.Items = new(spec.Items)
	}
	pt.param.Type = "array"
	return itemsTypable{pt.param.Items, 1}
}

func (pt paramTypable) Schema() *spec.Schema {
	if pt.param.In != "body" {
		return nil
	}
	if pt.param.Schema == nil {
		pt.param.Schema = new(spec.Schema)
	}
	return pt.param.Schema
}

type itemsTypable struct {
	items *spec.Items
	level int
}

func (pt itemsTypable) Level() int { return pt.level }

func (pt itemsTypable) Typed(tpe, format string) {
	pt.items.Typed(tpe, format)
}

func (pt itemsTypable) SetRef(ref spec.Ref) {
	pt.items.Ref = ref
}

func (pt itemsTypable) Schema() *spec.Schema {
	return nil
}

func (pt itemsTypable) Items() swaggerTypable {
	if pt.items.Items == nil {
		pt.items.Items = new(spec.Items)
	}
	pt.items.Type = "array"
	return itemsTypable{pt.items.Items, pt.level + 1}
}

type paramValidations struct {
	current *spec.Parameter
}

func (sv paramValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}
func (sv paramValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv paramValidations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv paramValidations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv paramValidations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv paramValidations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv paramValidations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv paramValidations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv paramValidations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv paramValidations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv paramValidations) SetEnum(val string) {
	sv.current.Enum = parseEnum(val, &spec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}
func (sv paramValidations) SetDefault(val interface{}) { sv.current.Default = val }
func (sv paramValidations) SetExample(val interface{}) { sv.current.Example = val }

type itemsValidations struct {
	current *spec.Items
}

func (sv itemsValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}
func (sv itemsValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv itemsValidations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv itemsValidations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv itemsValidations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv itemsValidations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv itemsValidations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv itemsValidations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv itemsValidations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv itemsValidations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv itemsValidations) SetEnum(val string) {
	sv.current.Enum = parseEnum(val, &spec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}
func (sv itemsValidations) SetDefault(val interface{}) { sv.current.Default = val }
func (sv itemsValidations) SetExample(val interface{}) { sv.current.Example = val }

type paramDecl struct {
	File         *ast.File
	Decl         *ast.GenDecl
	TypeSpec     *ast.TypeSpec
	OperationIDs []string
}

func (sd *paramDecl) inferOperationIDs() (opids []string) {
	if len(sd.OperationIDs) > 0 {
		opids = sd.OperationIDs
		return
	}

	if sd.Decl.Doc != nil {
		for _, cmt := range sd.Decl.Doc.List {
			for _, ln := range strings.Split(cmt.Text, "\n") {
				matches := rxParametersOverride.FindStringSubmatch(ln)
				if len(matches) > 1 && len(matches[1]) > 0 {
					for _, pt := range strings.Split(matches[1], " ") {
						tr := strings.TrimSpace(pt)
						if len(tr) > 0 {
							opids = append(opids, tr)
						}
					}
				}
			}
		}
	}
	sd.OperationIDs = append(sd.OperationIDs, opids...)
	return
}

func newParameterParser(prog *loader.Program) *paramStructParser {
	scp := new(paramStructParser)
	scp.program = prog
	scp.scp = newSchemaParser(prog)
	return scp
}

type paramStructParser struct {
	program   *loader.Program
	postDecls []schemaDecl
	scp       *schemaParser
}

// Parse will traverse a file and look for parameters.
func (pp *paramStructParser) Parse(gofile *ast.File, target interface{}) error {
	tgt := target.(map[string]*spec.Operation)
	for _, decl := range gofile.Decls {
		switch x1 := decl.(type) {
		// Check for parameters at the package level.
		case *ast.GenDecl:
			for _, spc := range x1.Specs {
				switch x2 := spc.(type) {
				case *ast.TypeSpec:
					sd := paramDecl{gofile, x1, x2, nil}
					sd.inferOperationIDs()
					if err := pp.parseDecl(tgt, sd); err != nil {
						return err
					}
				}
			}
		// Check for parameters inside functions.
		case *ast.FuncDecl:
			for _, b := range x1.Body.List {
				switch x2 := b.(type) {
				case *ast.DeclStmt:
					switch x3 := x2.Decl.(type) {
					case *ast.GenDecl:
						for _, spc := range x3.Specs {
							switch x4 := spc.(type) {
							case *ast.TypeSpec:
								sd := paramDecl{gofile, x3, x4, nil}
								sd.inferOperationIDs()
								if err := pp.parseDecl(tgt, sd); err != nil {
									return err
								}
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func (pp *paramStructParser) parseDecl(operations map[string]*spec.Operation, decl paramDecl) error {
	// check if there is a swagger:parameters tag that is followed by one or more words,
	// these words are the ids of the operations this parameter struct applies to
	// once type name is found convert it to a schema, by looking up the schema in the
	// parameters dictionary that got passed into this parse method
	for _, opid := range decl.inferOperationIDs() {
		operation, ok := operations[opid]
		if !ok {
			operation = new(spec.Operation)
			operations[opid] = operation
			operation.ID = opid
		}

		// analyze struct body for fields etc
		// each exported struct field:
		// * gets a type mapped to a go primitive
		// * perhaps gets a format
		// * has to document the validations that apply for the type and the field
		// * when the struct field points to a model it becomes a ref: #/definitions/ModelName
		// * comments that aren't tags is used as the description
		if tpe, ok := decl.TypeSpec.Type.(*ast.StructType); ok {
			if err := pp.parseStructType(decl.File, operation, tpe, make(map[string]spec.Parameter)); err != nil {
				return err
			}
		}

		//operations[opid] = operation
	}
	return nil
}

func (pp *paramStructParser) parseEmbeddedStruct(gofile *ast.File, operation *spec.Operation, expr ast.Expr, seenPreviously map[string]spec.Parameter) error {
	switch tpe := expr.(type) {
	case *ast.Ident:
		// do lookup of type
		// take primitives into account, they should result in an error for swagger
		pkg, err := pp.scp.packageForFile(gofile, tpe)
		if err != nil {
			return fmt.Errorf("embedded struct: %v", err)
		}
		file, _, ts, err := findSourceFile(pkg, tpe.Name)
		if err != nil {
			return fmt.Errorf("embedded struct: %v", err)
		}
		if st, ok := ts.Type.(*ast.StructType); ok {
			return pp.parseStructType(file, operation, st, seenPreviously)
		}
	case *ast.SelectorExpr:
		// look up package, file and then type
		pkg, err := pp.scp.packageForSelector(gofile, tpe.X)
		if err != nil {
			return fmt.Errorf("embedded struct: %v", err)
		}
		file, _, ts, err := findSourceFile(pkg, tpe.Sel.Name)
		if err != nil {
			return fmt.Errorf("embedded struct: %v", err)
		}
		if st, ok := ts.Type.(*ast.StructType); ok {
			return pp.parseStructType(file, operation, st, seenPreviously)
		}
	case *ast.StarExpr:
		return pp.parseEmbeddedStruct(gofile, operation, tpe.X, seenPreviously)
	}
	fmt.Printf("3%#v\n", expr)
	return fmt.Errorf("unable to resolve embedded struct for: %v", expr)
}

func (pp *paramStructParser) parseStructType(gofile *ast.File, operation *spec.Operation, tpe *ast.StructType, seenPreviously map[string]spec.Parameter) error {
	if tpe.Fields != nil {
		pt := seenPreviously

		for _, fld := range tpe.Fields.List {
			if len(fld.Names) == 0 {
				// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
				// otherwise the fields will just be included as normal properties
				if err := pp.parseEmbeddedStruct(gofile, operation, fld.Type, pt); err != nil {
					return err
				}
			}
		}

		// a slice used to keep track of the sequence of the map keys, as maps does not keep to any specific sequence (since Go-1.4)
		sequence := []string{}

		for _, fld := range tpe.Fields.List {
			if len(fld.Names) > 0 && fld.Names[0] != nil && fld.Names[0].IsExported() {
				gnm := fld.Names[0].Name
				nm, ignore, _, err := parseJSONTag(fld)
				if err != nil {
					return err
				}
				if ignore {
					continue
				}

				in := "query"
				// scan for param location first, this changes some behavior down the line
				if fld.Doc != nil {
					for _, cmt := range fld.Doc.List {
						for _, line := range strings.Split(cmt.Text, "\n") {
							matches := rxIn.FindStringSubmatch(line)
							if len(matches) > 0 && len(strings.TrimSpace(matches[1])) > 0 {
								in = strings.TrimSpace(matches[1])
							}
						}
					}
				}

				ps := pt[nm]
				ps.In = in
				var pty swaggerTypable = paramTypable{&ps}
				if in == "body" {
					pty = schemaTypable{pty.Schema(), 0}
				}
				if in == "formData" && fld.Doc != nil && fileParam(fld.Doc) {
					pty.Typed("file", "")
				} else {
					if err := pp.scp.parseNamedType(gofile, fld.Type, pty); err != nil {
						return err
					}
				}

				if strfmtName, ok := strfmtName(fld.Doc); ok {
					ps.Typed("string", strfmtName)
					ps.Ref = spec.Ref{}
				}

				sp := new(sectionedParser)
				sp.setDescription = func(lines []string) { ps.Description = joinDropLast(lines) }
				if ps.Ref.String() == "" {
					sp.taggers = []tagParser{
						newSingleLineTagParser("in", &matchOnlyParam{&ps, rxIn}),
						newSingleLineTagParser("maximum", &setMaximum{paramValidations{&ps}, rxf(rxMaximumFmt, "")}),
						newSingleLineTagParser("minimum", &setMinimum{paramValidations{&ps}, rxf(rxMinimumFmt, "")}),
						newSingleLineTagParser("multipleOf", &setMultipleOf{paramValidations{&ps}, rxf(rxMultipleOfFmt, "")}),
						newSingleLineTagParser("minLength", &setMinLength{paramValidations{&ps}, rxf(rxMinLengthFmt, "")}),
						newSingleLineTagParser("maxLength", &setMaxLength{paramValidations{&ps}, rxf(rxMaxLengthFmt, "")}),
						newSingleLineTagParser("pattern", &setPattern{paramValidations{&ps}, rxf(rxPatternFmt, "")}),
						newSingleLineTagParser("collectionFormat", &setCollectionFormat{paramValidations{&ps}, rxf(rxCollectionFormatFmt, "")}),
						newSingleLineTagParser("minItems", &setMinItems{paramValidations{&ps}, rxf(rxMinItemsFmt, "")}),
						newSingleLineTagParser("maxItems", &setMaxItems{paramValidations{&ps}, rxf(rxMaxItemsFmt, "")}),
						newSingleLineTagParser("unique", &setUnique{paramValidations{&ps}, rxf(rxUniqueFmt, "")}),
						newSingleLineTagParser("enum", &setEnum{paramValidations{&ps}, rxf(rxEnumFmt, "")}),
						newSingleLineTagParser("default", &setDefault{&ps.SimpleSchema, paramValidations{&ps}, rxf(rxDefaultFmt, "")}),
						newSingleLineTagParser("example", &setExample{&ps.SimpleSchema, paramValidations{&ps}, rxf(rxExampleFmt, "")}),
						newSingleLineTagParser("required", &setRequiredParam{&ps}),
					}

					itemsTaggers := func(items *spec.Items, level int) []tagParser {
						// the expression is 1-index based not 0-index
						itemsPrefix := fmt.Sprintf(rxItemsPrefixFmt, level+1)

						return []tagParser{
							newSingleLineTagParser(fmt.Sprintf("items%dMaximum", level), &setMaximum{itemsValidations{items}, rxf(rxMaximumFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dMinimum", level), &setMinimum{itemsValidations{items}, rxf(rxMinimumFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dMultipleOf", level), &setMultipleOf{itemsValidations{items}, rxf(rxMultipleOfFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dMinLength", level), &setMinLength{itemsValidations{items}, rxf(rxMinLengthFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dMaxLength", level), &setMaxLength{itemsValidations{items}, rxf(rxMaxLengthFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dPattern", level), &setPattern{itemsValidations{items}, rxf(rxPatternFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dCollectionFormat", level), &setCollectionFormat{itemsValidations{items}, rxf(rxCollectionFormatFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dMinItems", level), &setMinItems{itemsValidations{items}, rxf(rxMinItemsFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dMaxItems", level), &setMaxItems{itemsValidations{items}, rxf(rxMaxItemsFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dUnique", level), &setUnique{itemsValidations{items}, rxf(rxUniqueFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dEnum", level), &setEnum{itemsValidations{items}, rxf(rxEnumFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dDefault", level), &setDefault{&items.SimpleSchema, itemsValidations{items}, rxf(rxDefaultFmt, itemsPrefix)}),
							newSingleLineTagParser(fmt.Sprintf("items%dExample", level), &setExample{&items.SimpleSchema, itemsValidations{items}, rxf(rxExampleFmt, itemsPrefix)}),
						}
					}

					var parseArrayTypes func(expr ast.Expr, items *spec.Items, level int) ([]tagParser, error)
					parseArrayTypes = func(expr ast.Expr, items *spec.Items, level int) ([]tagParser, error) {
						if items == nil {
							return []tagParser{}, nil
						}
						switch iftpe := expr.(type) {
						case *ast.ArrayType:
							eleTaggers := itemsTaggers(items, level)
							sp.taggers = append(eleTaggers, sp.taggers...)
							otherTaggers, err := parseArrayTypes(iftpe.Elt, items.Items, level+1)
							if err != nil {
								return nil, err
							}
							return otherTaggers, nil
						case *ast.SelectorExpr:
							otherTaggers, err := parseArrayTypes(iftpe.Sel, items.Items, level+1)
							if err != nil {
								return nil, err
							}
							return otherTaggers, nil
						case *ast.Ident:
							taggers := []tagParser{}
							if iftpe.Obj == nil {
								taggers = itemsTaggers(items, level)
							}
							otherTaggers, err := parseArrayTypes(expr, items.Items, level+1)
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
							return err
						}
						sp.taggers = append(taggers, sp.taggers...)
					}

				} else {

					sp.taggers = []tagParser{
						newSingleLineTagParser("in", &matchOnlyParam{&ps, rxIn}),
						newSingleLineTagParser("required", &matchOnlyParam{&ps, rxRequired}),
					}
				}
				if err := sp.Parse(fld.Doc); err != nil {
					return err
				}
				if ps.In == "path" {
					ps.Required = true
				}

				if ps.Name == "" {
					ps.Name = nm
				}

				if nm != gnm {
					addExtension(&ps.VendorExtensible, "x-go-name", gnm)
				}
				pt[nm] = ps
				sequence = append(sequence, nm)
			}
		}

		for _, k := range sequence {
			p := pt[k]
			for i, v := range operation.Parameters {
				if v.Name == k {
					operation.Parameters = append(operation.Parameters[:i], operation.Parameters[i+1:]...)
					break
				}
			}
			operation.Parameters = append(operation.Parameters, p)
		}
	}

	return nil
}

func isAliasParam(prop swaggerTypable) bool {
	var isParam bool
	if param, ok := prop.(paramTypable); ok {
		isParam = param.param.In == "query" ||
			param.param.In == "path" ||
			param.param.In == "formData"
	}
	return isParam
}
