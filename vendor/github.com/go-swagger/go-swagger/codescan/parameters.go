package codescan

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/pkg/errors"

	"github.com/go-openapi/spec"
)

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

func (pt paramTypable) AddExtension(key string, value interface{}) {
	if pt.param.In == "body" {
		pt.Schema().AddExtension(key, value)
	} else {
		pt.param.AddExtension(key, value)
	}
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

func (pt itemsTypable) AddExtension(key string, value interface{}) {
	pt.items.AddExtension(key, value)
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

type parameterBuilder struct {
	ctx       *scanCtx
	decl      *entityDecl
	postDecls []*entityDecl
}

func (p *parameterBuilder) Build(operations map[string]*spec.Operation) error {

	// check if there is a swagger:parameters tag that is followed by one or more words,
	// these words are the ids of the operations this parameter struct applies to
	// once type name is found convert it to a schema, by looking up the schema in the
	// parameters dictionary that got passed into this parse method
	for _, opid := range p.decl.OperationIDS() {
		operation, ok := operations[opid]
		if !ok {
			operation = new(spec.Operation)
			operations[opid] = operation
			operation.ID = opid
		}
		debugLog("building parameters for: %s", opid)

		// analyze struct body for fields etc
		// each exported struct field:
		// * gets a type mapped to a go primitive
		// * perhaps gets a format
		// * has to document the validations that apply for the type and the field
		// * when the struct field points to a model it becomes a ref: #/definitions/ModelName
		// * comments that aren't tags is used as the description
		if err := p.buildFromType(p.decl.Type, operation, make(map[string]spec.Parameter)); err != nil {
			return err
		}
	}
	return nil
}

func (p *parameterBuilder) buildFromType(otpe types.Type, op *spec.Operation, seen map[string]spec.Parameter) error {
	switch tpe := otpe.(type) {
	case *types.Pointer:
		return p.buildFromType(tpe.Elem(), op, seen)
	case *types.Named:
		o := tpe.Obj()
		switch stpe := o.Type().Underlying().(type) {
		case *types.Struct:
			debugLog("build from type %s: %T", tpe.Obj().Name(), otpe)
			if decl, found := p.ctx.DeclForType(o.Type()); found {
				return p.buildFromStruct(decl, stpe, op, seen)
			}
			return p.buildFromStruct(p.decl, stpe, op, seen)
		default:
			return errors.Errorf("unhandled type (%T): %s", stpe, o.Type().Underlying().String())
		}
	default:
		return errors.Errorf("unhandled type (%T): %s", otpe, tpe.String())
	}
}

func (p *parameterBuilder) buildFromField(fld *types.Var, tpe types.Type, typable swaggerTypable, seen map[string]spec.Parameter) error {
	debugLog("build from field %s: %T", fld.Name(), tpe)
	switch ftpe := tpe.(type) {
	case *types.Basic:
		return swaggerSchemaForType(ftpe.Name(), typable)
	case *types.Struct:
		sb := schemaBuilder{
			decl: p.decl,
			ctx:  p.ctx,
		}
		if err := sb.buildFromType(tpe, typable); err != nil {
			return err
		}
		p.postDecls = append(p.postDecls, sb.postDecls...)
		return nil
	case *types.Pointer:
		return p.buildFromField(fld, ftpe.Elem(), typable, seen)
	case *types.Interface:
		sb := schemaBuilder{
			decl: p.decl,
			ctx:  p.ctx,
		}
		if err := sb.buildFromType(tpe, typable); err != nil {
			return err
		}
		p.postDecls = append(p.postDecls, sb.postDecls...)
		return nil
	case *types.Array:
		return p.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Slice:
		return p.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Map:
		schema := new(spec.Schema)
		typable.Schema().Typed("object", "").AdditionalProperties = &spec.SchemaOrBool{
			Schema: schema,
		}
		sb := schemaBuilder{
			decl: p.decl,
			ctx:  p.ctx,
		}
		if err := sb.buildFromType(ftpe.Elem(), schemaTypable{schema, typable.Level() + 1}); err != nil {
			return err
		}
		return nil
	case *types.Named:
		if decl, found := p.ctx.DeclForType(ftpe.Obj().Type()); found {
			if decl.Type.Obj().Pkg().Path() == "time" && decl.Type.Obj().Name() == "Time" {
				typable.Typed("string", "date-time")
				return nil
			}
			if sfnm, isf := strfmtName(decl.Comments); isf {
				typable.Typed("string", sfnm)
				return nil
			}
			//if err := r.makeRef(decl, typable); err != nil {
			//	return err
			//}
			sb := &schemaBuilder{ctx: p.ctx, decl: decl}
			sb.inferNames()
			if err := sb.buildFromType(decl.Type, typable); err != nil {
				return err
			}
			p.postDecls = append(p.postDecls, sb.postDecls...)
			return nil
		}
		return errors.Errorf("unable to find package and source file for: %s", ftpe.String())
	default:
		return errors.Errorf("unknown type for %s: %T", fld.String(), fld.Type())
	}
}

func (p *parameterBuilder) buildFromStruct(decl *entityDecl, tpe *types.Struct, op *spec.Operation, seen map[string]spec.Parameter) error {
	if tpe.NumFields() == 0 {
		return nil
	}

	var sequence []string

	for i := 0; i < tpe.NumFields(); i++ {
		fld := tpe.Field(i)

		if fld.Embedded() {
			if err := p.buildFromType(fld.Type(), op, seen); err != nil {
				return err
			}
			continue
		}

		tg := tpe.Tag(i)

		var afld *ast.Field
		ans, _ := astutil.PathEnclosingInterval(decl.File, fld.Pos(), fld.Pos())
		for _, an := range ans {
			at, valid := an.(*ast.Field)
			if !valid {
				continue
			}

			debugLog("field %s: %s(%T) [%q] ==> %s", fld.Name(), fld.Type().String(), fld.Type(), tg, at.Doc.Text())
			afld = at
			break
		}

		if afld == nil {
			debugLog("can't find source associated with %s for %s", fld.String(), tpe.String())
			continue
		}

		// if the field is annotated with swagger:ignore, ignore it
		if ignored(afld.Doc) {
			continue
		}

		name, ignore, _, err := parseJSONTag(afld)
		if err != nil {
			return err
		}
		if ignore {
			continue
		}

		in := "query"
		// scan for param location first, this changes some behavior down the line
		if afld.Doc != nil {
			for _, cmt := range afld.Doc.List {
				for _, line := range strings.Split(cmt.Text, "\n") {
					matches := rxIn.FindStringSubmatch(line)
					if len(matches) > 0 && len(strings.TrimSpace(matches[1])) > 0 {
						in = strings.TrimSpace(matches[1])
					}
				}
			}
		}

		ps := seen[name]
		ps.In = in
		var pty swaggerTypable = paramTypable{&ps}
		if in == "body" {
			pty = schemaTypable{pty.Schema(), 0}
		}
		if in == "formData" && afld.Doc != nil && fileParam(afld.Doc) {
			pty.Typed("file", "")
		} else if err := p.buildFromField(fld, fld.Type(), pty, seen); err != nil {
			return err
		}

		if strfmtName, ok := strfmtName(afld.Doc); ok {
			ps.Typed("string", strfmtName)
			ps.Ref = spec.Ref{}
			ps.Items = nil
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
					return nil, fmt.Errorf("unknown field type ele for %q", name)
				}
			}

			// check if this is a primitive, if so parse the validations from the
			// doc comments of the slice declaration.
			if ftped, ok := afld.Type.(*ast.ArrayType); ok {
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
		if err := sp.Parse(afld.Doc); err != nil {
			return err
		}
		if ps.In == "path" {
			ps.Required = true
		}

		if ps.Name == "" {
			ps.Name = name
		}

		if name != fld.Name() {
			addExtension(&ps.VendorExtensible, "x-go-name", fld.Name())
		}
		seen[name] = ps
		sequence = append(sequence, name)
	}

	for _, k := range sequence {
		p := seen[k]
		for i, v := range op.Parameters {
			if v.Name == k {
				op.Parameters = append(op.Parameters[:i], op.Parameters[i+1:]...)
				break
			}
		}
		op.Parameters = append(op.Parameters, p)
	}
	return nil
}
