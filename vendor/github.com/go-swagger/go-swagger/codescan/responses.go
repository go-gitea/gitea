package codescan

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/pkg/errors"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/go-openapi/spec"
)

type responseTypable struct {
	in       string
	header   *spec.Header
	response *spec.Response
}

func (ht responseTypable) Level() int { return 0 }

func (ht responseTypable) Typed(tpe, format string) {
	ht.header.Typed(tpe, format)
}

func bodyTypable(in string, schema *spec.Schema) (swaggerTypable, *spec.Schema) {
	if in == "body" {
		// get the schema for items on the schema property
		if schema == nil {
			schema = new(spec.Schema)
		}
		if schema.Items == nil {
			schema.Items = new(spec.SchemaOrArray)
		}
		if schema.Items.Schema == nil {
			schema.Items.Schema = new(spec.Schema)
		}
		schema.Typed("array", "")
		return schemaTypable{schema.Items.Schema, 1}, schema
	}
	return nil, nil
}

func (ht responseTypable) Items() swaggerTypable {
	bdt, schema := bodyTypable(ht.in, ht.response.Schema)
	if bdt != nil {
		ht.response.Schema = schema
		return bdt
	}

	if ht.header.Items == nil {
		ht.header.Items = new(spec.Items)
	}
	ht.header.Type = "array"
	return itemsTypable{ht.header.Items, 1}
}

func (ht responseTypable) SetRef(ref spec.Ref) {
	// having trouble seeing the usefulness of this one here
	ht.Schema().Ref = ref
}

func (ht responseTypable) Schema() *spec.Schema {
	if ht.response.Schema == nil {
		ht.response.Schema = new(spec.Schema)
	}
	return ht.response.Schema
}

func (ht responseTypable) SetSchema(schema *spec.Schema) {
	ht.response.Schema = schema
}
func (ht responseTypable) CollectionOf(items *spec.Items, format string) {
	ht.header.CollectionOf(items, format)
}
func (ht responseTypable) AddExtension(key string, value interface{}) {
	ht.response.AddExtension(key, value)
}

type headerValidations struct {
	current *spec.Header
}

func (sv headerValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}
func (sv headerValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv headerValidations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv headerValidations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv headerValidations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv headerValidations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv headerValidations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv headerValidations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv headerValidations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv headerValidations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv headerValidations) SetEnum(val string) {
	sv.current.Enum = parseEnum(val, &spec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}
func (sv headerValidations) SetDefault(val interface{}) { sv.current.Default = val }
func (sv headerValidations) SetExample(val interface{}) { sv.current.Example = val }

type responseBuilder struct {
	ctx       *scanCtx
	decl      *entityDecl
	postDecls []*entityDecl
}

func (r *responseBuilder) Build(responses map[string]spec.Response) error {
	// check if there is a swagger:response tag that is followed by one or more words,
	// these words are the ids of the operations this parameter struct applies to
	// once type name is found convert it to a schema, by looking up the schema in the
	// parameters dictionary that got passed into this parse method

	name, _ := r.decl.ResponseNames()
	response := responses[name]
	debugLog("building response: %s", name)

	// analyze doc comment for the model
	sp := new(sectionedParser)
	sp.setDescription = func(lines []string) { response.Description = joinDropLast(lines) }
	if err := sp.Parse(r.decl.Comments); err != nil {
		return err
	}

	// analyze struct body for fields etc
	// each exported struct field:
	// * gets a type mapped to a go primitive
	// * perhaps gets a format
	// * has to document the validations that apply for the type and the field
	// * when the struct field points to a model it becomes a ref: #/definitions/ModelName
	// * comments that aren't tags is used as the description
	if err := r.buildFromType(r.decl.Type, &response, make(map[string]bool)); err != nil {
		return err
	}
	responses[name] = response
	return nil
}

func (r *responseBuilder) buildFromField(fld *types.Var, tpe types.Type, typable swaggerTypable, seen map[string]bool) error {
	debugLog("build from field %s: %T", fld.Name(), tpe)
	switch ftpe := tpe.(type) {
	case *types.Basic:
		return swaggerSchemaForType(ftpe.Name(), typable)
	case *types.Struct:
		sb := schemaBuilder{
			decl: r.decl,
			ctx:  r.ctx,
		}
		if err := sb.buildFromType(tpe, typable); err != nil {
			return err
		}
		r.postDecls = append(r.postDecls, sb.postDecls...)
		return nil
	case *types.Pointer:
		return r.buildFromField(fld, ftpe.Elem(), typable, seen)
	case *types.Interface:
		sb := schemaBuilder{
			decl: r.decl,
			ctx:  r.ctx,
		}
		if err := sb.buildFromType(tpe, typable); err != nil {
			return err
		}
		r.postDecls = append(r.postDecls, sb.postDecls...)
		return nil
	case *types.Array:
		return r.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Slice:
		return r.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Map:
		schema := new(spec.Schema)
		typable.Schema().Typed("object", "").AdditionalProperties = &spec.SchemaOrBool{
			Schema: schema,
		}
		sb := schemaBuilder{
			decl: r.decl,
			ctx:  r.ctx,
		}
		if err := sb.buildFromType(ftpe.Elem(), schemaTypable{schema, typable.Level() + 1}); err != nil {
			return err
		}
		return nil
	case *types.Named:
		if decl, found := r.ctx.DeclForType(ftpe.Obj().Type()); found {
			if decl.Type.Obj().Pkg().Path() == "time" && decl.Type.Obj().Name() == "Time" {
				typable.Typed("string", "date-time")
				return nil
			}
			if sfnm, isf := strfmtName(decl.Comments); isf {
				typable.Typed("string", sfnm)
				return nil
			}
			sb := &schemaBuilder{ctx: r.ctx, decl: decl}
			sb.inferNames()
			if err := sb.buildFromType(decl.Type, typable); err != nil {
				return err
			}
			r.postDecls = append(r.postDecls, sb.postDecls...)
			return nil
		}
		return errors.Errorf("unable to find package and source file for: %s", ftpe.String())
	default:
		return errors.Errorf("unknown type for %s: %T", fld.String(), fld.Type())
	}
}

func (r *responseBuilder) buildFromType(otpe types.Type, resp *spec.Response, seen map[string]bool) error {
	switch tpe := otpe.(type) {
	case *types.Pointer:
		return r.buildFromType(tpe.Elem(), resp, seen)
	case *types.Named:
		o := tpe.Obj()
		switch stpe := o.Type().Underlying().(type) {
		case *types.Struct:
			debugLog("build from type %s: %T", tpe.Obj().Name(), otpe)
			if decl, found := r.ctx.DeclForType(o.Type()); found {
				return r.buildFromStruct(decl, stpe, resp, seen)
			}
			return r.buildFromStruct(r.decl, stpe, resp, seen)
		default:
			if decl, found := r.ctx.DeclForType(o.Type()); found {
				var schema spec.Schema
				typable := schemaTypable{schema: &schema, level: 0}

				if decl.Type.Obj().Pkg().Path() == "time" && decl.Type.Obj().Name() == "Time" {
					typable.Typed("string", "date-time")
					return nil
				}
				if sfnm, isf := strfmtName(decl.Comments); isf {
					typable.Typed("string", sfnm)
					return nil
				}
				sb := &schemaBuilder{ctx: r.ctx, decl: decl}
				sb.inferNames()
				if err := sb.buildFromType(tpe.Underlying(), typable); err != nil {
					return err
				}
				resp.WithSchema(&schema)
				r.postDecls = append(r.postDecls, sb.postDecls...)
				return nil
			}
			return errors.Errorf("responses can only be structs, did you mean for %s to be the response body?", otpe.String())
		}
	default:
		return errors.New("anonymous types are currently not supported for responses")
	}
}

func (r *responseBuilder) buildFromStruct(decl *entityDecl, tpe *types.Struct, resp *spec.Response, seen map[string]bool) error {
	if tpe.NumFields() == 0 {
		return nil
	}

	for i := 0; i < tpe.NumFields(); i++ {
		fld := tpe.Field(i)
		if fld.Embedded() {

			if err := r.buildFromType(fld.Type(), resp, seen); err != nil {
				return err
			}
			continue
		}
		if fld.Anonymous() {
			debugLog("skipping anonymous field")
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

		var in string
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

		ps := resp.Headers[name]

		// support swagger:file for response
		// An API operation can return a file, such as an image or PDF. In this case,
		// define the response schema with type: file and specify the appropriate MIME types in the produces section.
		if afld.Doc != nil && fileParam(afld.Doc) {
			resp.Schema = &spec.Schema{}
			resp.Schema.Typed("file", "")
		} else if err := r.buildFromField(fld, fld.Type(), responseTypable{in, &ps, resp}, seen); err != nil {
			return err
		}

		if strfmtName, ok := strfmtName(afld.Doc); ok {
			ps.Typed("string", strfmtName)
		}

		sp := new(sectionedParser)
		sp.setDescription = func(lines []string) { ps.Description = joinDropLast(lines) }
		sp.taggers = []tagParser{
			newSingleLineTagParser("maximum", &setMaximum{headerValidations{&ps}, rxf(rxMaximumFmt, "")}),
			newSingleLineTagParser("minimum", &setMinimum{headerValidations{&ps}, rxf(rxMinimumFmt, "")}),
			newSingleLineTagParser("multipleOf", &setMultipleOf{headerValidations{&ps}, rxf(rxMultipleOfFmt, "")}),
			newSingleLineTagParser("minLength", &setMinLength{headerValidations{&ps}, rxf(rxMinLengthFmt, "")}),
			newSingleLineTagParser("maxLength", &setMaxLength{headerValidations{&ps}, rxf(rxMaxLengthFmt, "")}),
			newSingleLineTagParser("pattern", &setPattern{headerValidations{&ps}, rxf(rxPatternFmt, "")}),
			newSingleLineTagParser("collectionFormat", &setCollectionFormat{headerValidations{&ps}, rxf(rxCollectionFormatFmt, "")}),
			newSingleLineTagParser("minItems", &setMinItems{headerValidations{&ps}, rxf(rxMinItemsFmt, "")}),
			newSingleLineTagParser("maxItems", &setMaxItems{headerValidations{&ps}, rxf(rxMaxItemsFmt, "")}),
			newSingleLineTagParser("unique", &setUnique{headerValidations{&ps}, rxf(rxUniqueFmt, "")}),
			newSingleLineTagParser("enum", &setEnum{headerValidations{&ps}, rxf(rxEnumFmt, "")}),
			newSingleLineTagParser("default", &setDefault{&ps.SimpleSchema, headerValidations{&ps}, rxf(rxDefaultFmt, "")}),
			newSingleLineTagParser("example", &setExample{&ps.SimpleSchema, headerValidations{&ps}, rxf(rxExampleFmt, "")}),
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

		if err := sp.Parse(afld.Doc); err != nil {
			return err
		}

		if in != "body" {
			seen[name] = true
			if resp.Headers == nil {
				resp.Headers = make(map[string]spec.Header)
			}
			resp.Headers[name] = ps
		}
	}

	for k := range resp.Headers {
		if !seen[k] {
			delete(resp.Headers, k)
		}
	}
	return nil
}
