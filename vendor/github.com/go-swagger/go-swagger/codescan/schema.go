package codescan

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/go-openapi/spec"
	"github.com/pkg/errors"
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
func (st schemaTypable) AddExtension(key string, value interface{}) {
	addExtension(&st.schema.VendorExtensible, key, value)
}

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

type schemaBuilder struct {
	ctx        *scanCtx
	decl       *entityDecl
	GoName     string
	Name       string
	annotated  bool
	discovered []*entityDecl
	postDecls  []*entityDecl
}

func (s *schemaBuilder) inferNames() (goName string, name string) {
	if s.GoName != "" {
		goName, name = s.GoName, s.Name
		return
	}

	goName = s.decl.Ident.Name
	name = goName
	defer func() {
		s.GoName = goName
		s.Name = name
	}()
	if s.decl.Comments == nil {
		return
	}

DECLS:
	for _, cmt := range s.decl.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxModelOverride.FindStringSubmatch(ln)
			if len(matches) > 0 {
				s.annotated = true
			}
			if len(matches) > 1 && len(matches[1]) > 0 {
				name = matches[1]
				break DECLS
			}
		}
	}
	return
}

func (s *schemaBuilder) Build(definitions map[string]spec.Schema) error {
	s.inferNames()

	schema := definitions[s.Name]
	err := s.buildFromDecl(s.decl, &schema)
	if err != nil {
		return err
	}
	definitions[s.Name] = schema
	return nil
}

func (s *schemaBuilder) buildFromDecl(decl *entityDecl, schema *spec.Schema) error {
	// analyze doc comment for the model
	sp := new(sectionedParser)
	sp.setTitle = func(lines []string) { schema.Title = joinDropLast(lines) }
	sp.setDescription = func(lines []string) { schema.Description = joinDropLast(lines) }
	if err := sp.Parse(s.decl.Comments); err != nil {
		return err
	}

	// if the type is marked to ignore, just return
	if sp.ignored {
		return nil
	}

	switch tpe := s.decl.Type.Obj().Type().(type) {
	case *types.Basic:
		debugLog("basic: %v", tpe.Name())
	case *types.Struct:
		if err := s.buildFromStruct(s.decl, tpe, schema, make(map[string]string)); err != nil {
			return err
		}
	case *types.Interface:
		if err := s.buildFromInterface(s.decl, tpe, schema, make(map[string]string)); err != nil {
			return err
		}
	case *types.Array:
		debugLog("array: %v -> %v", s.decl.Ident.Name, tpe.Elem().String())
	case *types.Slice:
		debugLog("slice: %v -> %v", s.decl.Ident.Name, tpe.Elem().String())
	case *types.Map:
		debugLog("map: %v -> [%v]%v", s.decl.Ident.Name, tpe.Key().String(), tpe.Elem().String())
	case *types.Named:
		o := tpe.Obj()
		debugLog("got the named type object: %s.%s | isAlias: %t | exported: %t", o.Pkg().Path(), o.Name(), o.IsAlias(), o.Exported())
		if o != nil {
			if o.Pkg().Name() == "time" && o.Name() == "Time" {
				schema.Typed("string", "date-time")
				return nil
			}

			ps := schemaTypable{schema, 0}
			for {
				ti := s.decl.Pkg.TypesInfo.Types[s.decl.Spec.Type]
				if ti.IsBuiltin() {
					break
				}
				if ti.IsType() {
					if err := s.buildFromType(ti.Type, ps); err != nil {
						return err
					}
					break
				}
			}
		}
	default:
		log.Printf("WARNING: Missing parser for a %T, skipping model: %s\n", tpe, s.Name)
		return nil
	}

	if schema.Ref.String() == "" {
		if s.Name != s.GoName {
			addExtension(&schema.VendorExtensible, "x-go-name", s.GoName)
		}
		addExtension(&schema.VendorExtensible, "x-go-package", s.decl.Type.Obj().Pkg().Path())
	}
	return nil
}

func (s *schemaBuilder) buildFromType(tpe types.Type, tgt swaggerTypable) error {
	switch titpe := tpe.(type) {
	case *types.Basic:
		return swaggerSchemaForType(titpe.String(), tgt)
	case *types.Pointer:
		return s.buildFromType(titpe.Elem(), tgt)
	case *types.Struct:
		return s.buildFromStruct(s.decl, titpe, tgt.Schema(), make(map[string]string))
	case *types.Interface:
		return s.buildFromInterface(s.decl, titpe, tgt.Schema(), make(map[string]string))
	case *types.Slice:
		return s.buildFromType(titpe.Elem(), tgt.Items())
	case *types.Array:
		return s.buildFromType(titpe.Elem(), tgt.Items())
	case *types.Map:
		//debugLog("map: %v -> [%v]%v", fld.Name(), ftpe.Key().String(), ftpe.Elem().String())
		// check if key is a string type, if not print a message
		// and skip the map property. Only maps with string keys can go into additional properties
		sch := tgt.Schema()
		if sch == nil {
			return errors.New("items doesn't support maps")
		}
		eleProp := schemaTypable{sch, tgt.Level()}
		key := titpe.Key()
		if key.Underlying().String() == "string" {
			if err := s.buildFromType(titpe.Elem(), eleProp.AdditionalProperties()); err != nil {
				return err
			}
			return nil
		}
	case *types.Named:
		tio := titpe.Obj()
		if tio.Pkg() == nil && tio.Name() == "error" {
			return swaggerSchemaForType(tio.Name(), tgt)
		}
		debugLog("named refined type %s.%s", tio.Pkg().Path(), tio.Name())
		pkg, found := s.ctx.PkgForType(tpe)
		if !found {
			// this must be a builtin
			debugLog("skipping because package is nil: %s", tpe.String())
			return nil
		}
		if pkg.Name == "time" && tio.Name() == "Time" {
			tgt.Typed("string", "date-time")
			return nil
		}
		cmt, hasComments := s.ctx.FindComments(pkg, tio.Name())
		if !hasComments {
			cmt = new(ast.CommentGroup)
		}

		switch utitpe := tpe.Underlying().(type) {
		case *types.Struct:
			if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
				if decl.Type.Obj().Pkg().Path() == "time" && decl.Type.Obj().Name() == "Time" {
					tgt.Typed("string", "date-time")
					return nil
				}
				if sfnm, isf := strfmtName(cmt); isf {
					tgt.Typed("string", sfnm)
					return nil
				}
				if err := s.makeRef(decl, tgt); err != nil {
					return err
				}
				return nil
			}
		case *types.Interface:
			if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
				if err := s.makeRef(decl, tgt); err != nil {
					return err
				}
				return nil
			}
		case *types.Basic:
			if sfnm, isf := strfmtName(cmt); isf {
				tgt.Typed("string", sfnm)
				return nil
			}

			if enumName, ok := enumName(cmt); ok {
				debugLog(enumName)
				return nil
			}

			if defaultName, ok := defaultName(cmt); ok {
				debugLog(defaultName)
				return nil
			}

			if typeName, ok := typeName(cmt); ok {
				_ = swaggerSchemaForType(typeName, tgt)
				return nil
			}

			if isAliasParam(tgt) || aliasParam(cmt) {
				err := swaggerSchemaForType(utitpe.Name(), tgt)
				if err == nil {
					return nil
				}
			}
			if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
				if err := s.makeRef(decl, tgt); err != nil {
					return err
				}
				return nil
			}
			return swaggerSchemaForType(utitpe.String(), tgt)
		case *types.Array:
			if sfnm, isf := strfmtName(cmt); isf {
				if sfnm == "byte" {
					tgt.Typed("string", sfnm)
					return nil
				}
				tgt.Items().Typed("string", sfnm)
				return nil
			}
			if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
				if err := s.makeRef(decl, tgt); err != nil {
					return err
				}
				return nil
			}
			return s.buildFromType(utitpe.Elem(), tgt.Items())
		case *types.Slice:
			if sfnm, isf := strfmtName(cmt); isf {
				if sfnm == "byte" {
					tgt.Typed("string", sfnm)
					return nil
				}
				tgt.Items().Typed("string", sfnm)
				return nil
			}
			if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
				if err := s.makeRef(decl, tgt); err != nil {
					return err
				}
				return nil
			}
			return s.buildFromType(utitpe.Elem(), tgt.Items())
		case *types.Map:
			if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
				if err := s.makeRef(decl, tgt); err != nil {
					return err
				}
				return nil
			}
			return nil

		default:
			log.Printf("WARNING: can't figure out object type for named type (%T): %v [alias: %t]", tpe.Underlying(), tpe.Underlying(), titpe.Obj().IsAlias())

			return nil
		}
	default:
		//log.Printf("WARNING: can't determine refined type %s (%T)", titpe.String(), titpe)
		panic(fmt.Sprintf("WARNING: can't determine refined type %s (%T)", titpe.String(), titpe))
	}

	return nil
}

func (s *schemaBuilder) buildFromInterface(decl *entityDecl, it *types.Interface, schema *spec.Schema, seen map[string]string) error {
	if it.Empty() {
		schema.Typed("object", "")
		return nil
	}

	var (
		tgt      *spec.Schema
		hasAllOf bool
	)

	flist := make([]*ast.Field, it.NumEmbeddeds()+it.NumExplicitMethods())
	for i := range decl.Spec.Type.(*ast.InterfaceType).Methods.List {
		flist[i] = decl.Spec.Type.(*ast.InterfaceType).Methods.List[i]
	}

	// First collect the embedded interfaces
	// create refs when the embedded interface is decorated with an allOf annotation
	for i := 0; i < it.NumEmbeddeds(); i++ {
		fld := it.EmbeddedType(i)

		switch ftpe := fld.(type) {
		case *types.Named:
			o := ftpe.Obj()
			var afld *ast.Field
			for _, an := range flist {
				if len(an.Names) != 0 {
					continue
				}

				tpp := decl.Pkg.TypesInfo.Types[an.Type]
				if tpp.Type.String() != o.Type().String() {
					continue
				}

				//decl.
				debugLog("maybe interface field %s: %s(%T)", o.Name(), o.Type().String(), o.Type())
				afld = an
				break
			}

			if afld == nil {
				debugLog("can't find source associated with %s for %s", fld.String(), it.String())
				continue
			}

			// if the field is annotated with swagger:ignore, ignore it
			if ignored(afld.Doc) {
				continue
			}

			if !allOfMember(afld.Doc) {
				var newSch spec.Schema
				if err := s.buildEmbedded(o.Type(), &newSch, seen); err != nil {
					return err
				}
				schema.AllOf = append(schema.AllOf, newSch)
				hasAllOf = true
				continue
			}

			hasAllOf = true
			if tgt == nil {
				tgt = &spec.Schema{}
			}
			var newSch spec.Schema
			// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
			// otherwise the fields will just be included as normal properties
			if err := s.buildAllOf(o.Type(), &newSch); err != nil {
				return err
			}
			if afld.Doc != nil {
				for _, cmt := range afld.Doc.List {
					for _, ln := range strings.Split(cmt.Text, "\n") {
						matches := rxAllOf.FindStringSubmatch(ln)
						ml := len(matches)
						if ml > 1 {
							mv := matches[ml-1]
							if mv != "" {
								schema.AddExtension("x-class", mv)
							}
						}
					}
				}
			}

			schema.AllOf = append(schema.AllOf, newSch)
		default:
			log.Printf("WARNING: can't figure out object type for allOf named type (%T): %v", ftpe, ftpe.Underlying())
		}
		debugLog("got embedded interface: %s {%T}", fld.String(), fld)
	}

	if tgt == nil {
		tgt = schema
	}
	// We can finally build the actual schema for the struct
	if tgt.Properties == nil {
		tgt.Properties = make(map[string]spec.Schema)
	}
	tgt.Typed("object", "")

	for i := 0; i < it.NumExplicitMethods(); i++ {
		fld := it.ExplicitMethod(i)
		if !fld.Exported() {
			continue
		}
		sig, isSignature := fld.Type().(*types.Signature)
		if !isSignature {
			continue
		}
		if sig.Params().Len() > 0 {
			continue
		}
		if sig.Results() == nil || sig.Results().Len() != 1 {
			continue
		}

		var afld *ast.Field
		ans, _ := astutil.PathEnclosingInterval(decl.File, fld.Pos(), fld.Pos())
		//debugLog("got %d nodes (exact: %t)", len(ans), isExact)
		for _, an := range ans {
			at, valid := an.(*ast.Field)
			if !valid {
				continue
			}

			debugLog("maybe interface field %s: %s(%T)", fld.Name(), fld.Type().String(), fld.Type())
			afld = at
			break
		}

		if afld == nil {
			debugLog("can't find source associated with %s for %s", fld.String(), it.String())
			continue
		}

		// if the field is annotated with swagger:ignore, ignore it
		if ignored(afld.Doc) {
			continue
		}

		name := fld.Name()
		if afld.Doc != nil {
			for _, cmt := range afld.Doc.List {
				for _, ln := range strings.Split(cmt.Text, "\n") {
					matches := rxName.FindStringSubmatch(ln)
					ml := len(matches)
					if ml > 1 {
						name = matches[ml-1]
					}
				}
			}
		}
		ps := tgt.Properties[name]
		if err := s.buildFromType(sig.Results().At(0).Type(), schemaTypable{&ps, 0}); err != nil {
			return err
		}
		if sfName, isStrfmt := strfmtName(afld.Doc); isStrfmt {
			ps.Typed("string", sfName)
			ps.Ref = spec.Ref{}
			ps.Items = nil
		}

		if err := s.createParser(name, tgt, &ps, afld).Parse(afld.Doc); err != nil {
			return err
		}

		if ps.Ref.String() == "" && name != fld.Name() {
			ps.AddExtension("x-go-name", fld.Name())
		}

		seen[name] = fld.Name()
		tgt.Properties[name] = ps
	}

	if tgt == nil {
		return nil
	}
	if hasAllOf && len(tgt.Properties) > 0 {
		schema.AllOf = append(schema.AllOf, *tgt)
	}
	for k := range tgt.Properties {
		if _, ok := seen[k]; !ok {
			delete(tgt.Properties, k)
		}
	}
	return nil
}

func (s *schemaBuilder) buildFromStruct(decl *entityDecl, st *types.Struct, schema *spec.Schema, seen map[string]string) error {
	// First check for all of schemas
	var tgt *spec.Schema
	hasAllOf := false

	for i := 0; i < st.NumFields(); i++ {
		fld := st.Field(i)
		if !fld.Anonymous() {
			debugLog("skipping field %q for allOf scan because not anonymous", fld.Name())
			continue
		}
		tg := st.Tag(i)

		debugLog("maybe allof field(%t) %s: %s (%T) [%q](anon: %t, embedded: %t)", fld.IsField(), fld.Name(), fld.Type().String(), fld.Type(), tg, fld.Anonymous(), fld.Embedded())
		var afld *ast.Field
		ans, _ := astutil.PathEnclosingInterval(decl.File, fld.Pos(), fld.Pos())
		//debugLog("got %d nodes (exact: %t)", len(ans), isExact)
		for _, an := range ans {
			at, valid := an.(*ast.Field)
			if !valid {
				continue
			}

			debugLog("maybe allof field %s: %s(%T) [%q]", fld.Name(), fld.Type().String(), fld.Type(), tg)
			afld = at
			break
		}

		if afld == nil {
			debugLog("can't find source associated with %s for %s", fld.String(), st.String())
			continue
		}

		// if the field is annotated with swagger:ignore, ignore it
		if ignored(afld.Doc) {
			continue
		}

		_, ignore, _, err := parseJSONTag(afld)
		if err != nil {
			return err
		}
		if ignore {
			continue
		}

		if !allOfMember(afld.Doc) {
			if tgt == nil {
				tgt = schema
			}
			if err := s.buildEmbedded(fld.Type(), tgt, seen); err != nil {
				return err
			}
			continue
		}
		// if this created an allOf property then we have to rejig the schema var
		// because all the fields collected that aren't from embedded structs should go in
		// their own proper schema
		// first process embedded structs in order of embedding
		hasAllOf = true
		if tgt == nil {
			tgt = &spec.Schema{}
		}
		var newSch spec.Schema
		// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
		// otherwise the fields will just be included as normal properties
		if err := s.buildAllOf(fld.Type(), &newSch); err != nil {
			return err
		}

		if afld.Doc != nil {
			for _, cmt := range afld.Doc.List {
				for _, ln := range strings.Split(cmt.Text, "\n") {
					matches := rxAllOf.FindStringSubmatch(ln)
					ml := len(matches)
					if ml > 1 {
						mv := matches[ml-1]
						if mv != "" {
							schema.AddExtension("x-class", mv)
						}
					}
				}
			}
		}

		schema.AllOf = append(schema.AllOf, newSch)
	}

	if tgt == nil {
		tgt = schema
	}
	// We can finally build the actual schema for the struct
	if tgt.Properties == nil {
		tgt.Properties = make(map[string]spec.Schema)
	}
	tgt.Typed("object", "")

	for i := 0; i < st.NumFields(); i++ {
		fld := st.Field(i)
		tg := st.Tag(i)

		if fld.Embedded() {
			continue
		}

		if !fld.Exported() {
			debugLog("skipping field %s because it's not exported", fld.Name())
			continue
		}

		var afld *ast.Field
		ans, _ := astutil.PathEnclosingInterval(decl.File, fld.Pos(), fld.Pos())
		//debugLog("got %d nodes (exact: %t)", len(ans), isExact)
		for _, an := range ans {
			at, valid := an.(*ast.Field)
			if !valid {
				continue
			}

			debugLog("field %s: %s(%T) [%q] ==> %s", fld.Name(), fld.Type().String(), fld.Type(), tg, at.Doc.Text())
			afld = at
			break
		}

		// if the field is annotated with swagger:ignore, ignore it
		if ignored(afld.Doc) {
			continue
		}

		name, ignore, isString, err := parseJSONTag(afld)
		if err != nil {
			return err
		}
		if ignore {
			for seenTagName, seenFieldName := range seen {
				if seenFieldName == fld.Name() {
					delete(tgt.Properties, seenTagName)
					break
				}
			}
			continue
		}

		ps := tgt.Properties[name]
		if err = s.buildFromType(fld.Type(), schemaTypable{&ps, 0}); err != nil {
			return err
		}
		if isString {
			ps.Typed("string", ps.Format)
			ps.Ref = spec.Ref{}
			ps.Items = nil
		}
		if sfName, isStrfmt := strfmtName(afld.Doc); isStrfmt {
			ps.Typed("string", sfName)
			ps.Ref = spec.Ref{}
			ps.Items = nil
		}

		if err = s.createParser(name, tgt, &ps, afld).Parse(afld.Doc); err != nil {
			return err
		}

		if ps.Ref.String() == "" && name != fld.Name() {
			addExtension(&ps.VendorExtensible, "x-go-name", fld.Name())
		}

		// we have 2 cases:
		// 1. field with different name override tag
		// 2. field with different name removes tag
		// so we need to save both tag&name
		seen[name] = fld.Name()
		tgt.Properties[name] = ps
	}

	if tgt == nil {
		return nil
	}
	if hasAllOf && len(tgt.Properties) > 0 {
		schema.AllOf = append(schema.AllOf, *tgt)
	}
	for k := range tgt.Properties {
		if _, ok := seen[k]; !ok {
			delete(tgt.Properties, k)
		}
	}
	return nil
}

func (s *schemaBuilder) buildAllOf(tpe types.Type, schema *spec.Schema) error {
	debugLog("allOf %s", tpe.Underlying())
	switch ftpe := tpe.(type) {
	case *types.Pointer:
		return s.buildAllOf(ftpe.Elem(), schema)
	case *types.Named:
		switch utpe := ftpe.Underlying().(type) {
		case *types.Struct:
			decl, found := s.ctx.FindModel(ftpe.Obj().Pkg().Path(), ftpe.Obj().Name())
			if found {
				if ftpe.Obj().Pkg().Path() == "time" && ftpe.Obj().Name() == "Time" {
					schema.Typed("string", "date-time")
					return nil
				}
				if sfnm, isf := strfmtName(decl.Comments); isf {
					schema.Typed("string", sfnm)
					return nil
				}
				if decl.HasModelAnnotation() {
					if err := s.makeRef(decl, schemaTypable{schema, 0}); err != nil {
						return err
					}
					return nil
				}
				return s.buildFromStruct(decl, utpe, schema, make(map[string]string))
			}
			return errors.Errorf("can't find source file for struct: %s", ftpe.String())
		case *types.Interface:
			decl, found := s.ctx.FindModel(ftpe.Obj().Pkg().Path(), ftpe.Obj().Name())
			if found {
				if sfnm, isf := strfmtName(decl.Comments); isf {
					schema.Typed("string", sfnm)
					return nil
				}
				if decl.HasModelAnnotation() {
					if err := s.makeRef(decl, schemaTypable{schema, 0}); err != nil {
						return err
					}
					return nil
				}
				return s.buildFromInterface(decl, utpe, schema, make(map[string]string))
			}
			return errors.Errorf("can't find source file for interface: %s", ftpe.String())
		default:
			log.Printf("WARNING: can't figure out object type for allOf named type (%T): %v", ftpe, ftpe.Underlying())
			return fmt.Errorf("unable to locate source file for allOf %s", utpe.String())
		}
	default:
		log.Printf("WARNING: Missing allOf parser for a %T, skipping field", ftpe)
		return fmt.Errorf("unable to resolve allOf member for: %v", ftpe)
	}
}

func (s *schemaBuilder) buildEmbedded(tpe types.Type, schema *spec.Schema, seen map[string]string) error {
	debugLog("embedded %s", tpe.Underlying())
	switch ftpe := tpe.(type) {
	case *types.Pointer:
		return s.buildEmbedded(ftpe.Elem(), schema, seen)
	case *types.Named:
		debugLog("embedded named type: %T", ftpe.Underlying())
		switch utpe := ftpe.Underlying().(type) {
		case *types.Struct:
			decl, found := s.ctx.FindModel(ftpe.Obj().Pkg().Path(), ftpe.Obj().Name())
			if found {
				return s.buildFromStruct(decl, utpe, schema, seen)
			}
			return errors.Errorf("can't find source file for struct: %s", ftpe.String())
		case *types.Interface:
			decl, found := s.ctx.FindModel(ftpe.Obj().Pkg().Path(), ftpe.Obj().Name())
			if found {
				return s.buildFromInterface(decl, utpe, schema, seen)
			}
			return errors.Errorf("can't find source file for struct: %s", ftpe.String())
		default:
			log.Printf("WARNING: can't figure out object type for embedded named type (%T): %v", ftpe, ftpe.Underlying())
		}
	default:
		log.Printf("WARNING: Missing embedded parser for a %T, skipping model\n", ftpe)
		return nil
	}
	return nil
}

func (s *schemaBuilder) makeRef(decl *entityDecl, prop swaggerTypable) error {
	nm, _ := decl.Names()
	ref, err := spec.NewRef("#/definitions/" + nm)
	if err != nil {
		return err
	}
	prop.SetRef(ref)
	s.postDecls = append(s.postDecls, decl)
	return nil
}

func (s *schemaBuilder) createParser(nm string, schema, ps *spec.Schema, fld *ast.Field) *sectionedParser {
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

func parseJSONTag(field *ast.Field) (name string, ignore bool, isString bool, err error) {
	if len(field.Names) > 0 {
		name = field.Names[0].Name
	}
	if field.Tag == nil || len(strings.TrimSpace(field.Tag.Value)) == 0 {
		return name, false, false, nil
	}

	tv, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return name, false, false, err
	}

	if strings.TrimSpace(tv) != "" {
		st := reflect.StructTag(tv)
		jsonParts := strings.Split(st.Get("json"), ",")
		jsonName := jsonParts[0]

		if len(jsonParts) > 1 && jsonParts[1] == "string" {
			// Need to check if the field type is a scalar. Otherwise, the
			// ",string" directive doesn't apply.
			isString = isFieldStringable(field.Type)
		}

		if jsonName == "-" {
			return name, true, isString, nil
		} else if jsonName != "" {
			return jsonName, false, isString, nil
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
