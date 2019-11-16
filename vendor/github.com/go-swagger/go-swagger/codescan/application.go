package codescan

import (
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"os"
	"strings"

	"github.com/go-openapi/swag"

	"golang.org/x/tools/go/packages"

	"github.com/go-openapi/spec"
)

const pkgLoadMode = packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo

func safeConvert(str string) bool {
	b, err := swag.ConvertBool(str)
	if err != nil {
		return false
	}
	return b
}

// Debug is true when process is run with DEBUG=1 env var
var Debug = safeConvert(os.Getenv("DEBUG"))

type node uint32

const (
	metaNode node = 1 << iota
	routeNode
	operationNode
	modelNode
	parametersNode
	responseNode
)

// Options for the scanner
type Options struct {
	Packages    []string
	InputSpec   *spec.Swagger
	ScanModels  bool
	WorkDir     string
	BuildTags   string
	ExcludeDeps bool
	Include     []string
	Exclude     []string
	IncludeTags []string
	ExcludeTags []string
}

type scanCtx struct {
	pkgs []*packages.Package
	app  *typeIndex
}

func sliceToSet(names []string) map[string]bool {
	result := make(map[string]bool)
	for _, v := range names {
		result[v] = true
	}
	return result
}

// Run the scanner to produce a spec with the options provided
func Run(opts *Options) (*spec.Swagger, error) {
	sc, err := newScanCtx(opts)
	if err != nil {
		return nil, err
	}
	sb := newSpecBuilder(opts.InputSpec, sc, opts.ScanModels)
	return sb.Build()
}

func newScanCtx(opts *Options) (*scanCtx, error) {
	cfg := &packages.Config{
		Dir:   opts.WorkDir,
		Mode:  pkgLoadMode,
		Tests: false,
	}
	if opts.BuildTags != "" {
		cfg.BuildFlags = []string{"-tags", opts.BuildTags}
	}

	pkgs, err := packages.Load(cfg, opts.Packages...)
	if err != nil {
		return nil, err
	}

	app, err := newTypeIndex(pkgs, opts.ExcludeDeps,
		sliceToSet(opts.IncludeTags), sliceToSet(opts.ExcludeTags),
		opts.Include, opts.Exclude)
	if err != nil {
		return nil, err
	}

	return &scanCtx{
		pkgs: pkgs,
		app:  app,
	}, nil
}

type entityDecl struct {
	Comments               *ast.CommentGroup
	Type                   *types.Named
	Ident                  *ast.Ident
	Spec                   *ast.TypeSpec
	File                   *ast.File
	Pkg                    *packages.Package
	hasModelAnnotation     bool
	hasResponseAnnotation  bool
	hasParameterAnnotation bool
}

func (d *entityDecl) Names() (name, goName string) {
	goName = d.Ident.Name
	name = goName
	if d.Comments == nil {
		return
	}

DECLS:
	for _, cmt := range d.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxModelOverride.FindStringSubmatch(ln)
			if len(matches) > 0 {
				d.hasModelAnnotation = true
			}
			if len(matches) > 1 && len(matches[1]) > 0 {
				name = matches[1]
				break DECLS
			}
		}
	}
	return
}

func (d *entityDecl) ResponseNames() (name, goName string) {
	goName = d.Ident.Name
	name = goName
	if d.Comments == nil {
		return
	}

DECLS:
	for _, cmt := range d.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxResponseOverride.FindStringSubmatch(ln)
			if len(matches) > 0 {
				d.hasResponseAnnotation = true
			}
			if len(matches) > 1 && len(matches[1]) > 0 {
				name = matches[1]
				break DECLS
			}
		}
	}
	return
}

func (d *entityDecl) OperationIDS() (result []string) {
	if d == nil || d.Comments == nil {
		return nil
	}

	for _, cmt := range d.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxParametersOverride.FindStringSubmatch(ln)
			if len(matches) > 0 {
				d.hasParameterAnnotation = true
			}
			if len(matches) > 1 && len(matches[1]) > 0 {
				for _, pt := range strings.Split(matches[1], " ") {
					tr := strings.TrimSpace(pt)
					if len(tr) > 0 {
						result = append(result, tr)
					}
				}
			}
		}
	}
	return
}

func (d *entityDecl) HasModelAnnotation() bool {
	if d.hasModelAnnotation {
		return true
	}
	if d.Comments == nil {
		return false
	}
	for _, cmt := range d.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxModelOverride.FindStringSubmatch(ln)
			if len(matches) > 0 {
				d.hasModelAnnotation = true
				return true
			}
		}
	}
	return false
}

func (d *entityDecl) HasResponseAnnotation() bool {
	if d.hasResponseAnnotation {
		return true
	}
	if d.Comments == nil {
		return false
	}
	for _, cmt := range d.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxResponseOverride.FindStringSubmatch(ln)
			if len(matches) > 0 {
				d.hasResponseAnnotation = true
				return true
			}
		}
	}
	return false
}

func (d *entityDecl) HasParameterAnnotation() bool {
	if d.hasParameterAnnotation {
		return true
	}
	if d.Comments == nil {
		return false
	}
	for _, cmt := range d.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxParametersOverride.FindStringSubmatch(ln)
			if len(matches) > 0 {
				d.hasParameterAnnotation = true
				return true
			}
		}
	}
	return false
}

func (s *scanCtx) FindDecl(pkgPath, name string) (*entityDecl, bool) {
	if pkg, ok := s.app.AllPackages[pkgPath]; ok {
		for _, file := range pkg.Syntax {
			for _, d := range file.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok {
					continue
				}

				for _, sp := range gd.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok && ts.Name.Name == name {
						def, ok := pkg.TypesInfo.Defs[ts.Name]
						if !ok {
							debugLog("couldn't find type info for %s", ts.Name)
							continue
						}
						nt, isNamed := def.Type().(*types.Named)
						if !isNamed {
							debugLog("%s is not a named type but a %T", ts.Name, def.Type())
							continue
						}
						decl := &entityDecl{
							Comments: gd.Doc,
							Type:     nt,
							Ident:    ts.Name,
							Spec:     ts,
							File:     file,
							Pkg:      pkg,
						}
						return decl, true
					}

				}
			}
		}
	}
	return nil, false
}

func (s *scanCtx) FindModel(pkgPath, name string) (*entityDecl, bool) {
	for _, cand := range s.app.Models {
		ct := cand.Type.Obj()
		if ct.Name() == name && ct.Pkg().Path() == pkgPath {
			return cand, true
		}
	}
	if decl, found := s.FindDecl(pkgPath, name); found {
		s.app.Models[decl.Ident] = decl
		return decl, true
	}
	return nil, false
}

func (s *scanCtx) PkgForPath(pkgPath string) (*packages.Package, bool) {
	v, ok := s.app.AllPackages[pkgPath]
	return v, ok
}

func (s *scanCtx) DeclForType(t types.Type) (*entityDecl, bool) {
	switch tpe := t.(type) {
	case *types.Pointer:
		return s.DeclForType(tpe.Elem())
	case *types.Named:
		return s.FindDecl(tpe.Obj().Pkg().Path(), tpe.Obj().Name())

	default:
		log.Printf("unknown type to find the package for [%T]: %s", t, t.String())
		return nil, false
	}
}

func (s *scanCtx) PkgForType(t types.Type) (*packages.Package, bool) {
	switch tpe := t.(type) {
	// case *types.Basic:
	// case *types.Struct:
	// case *types.Pointer:
	// case *types.Interface:
	// case *types.Array:
	// case *types.Slice:
	// case *types.Map:
	case *types.Named:
		v, ok := s.app.AllPackages[tpe.Obj().Pkg().Path()]
		return v, ok
	default:
		log.Printf("unknown type to find the package for [%T]: %s", t, t.String())
		return nil, false
	}
}

func (s *scanCtx) FindComments(pkg *packages.Package, name string) (*ast.CommentGroup, bool) {
	for _, f := range pkg.Syntax {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, s := range gd.Specs {
				if ts, ok := s.(*ast.TypeSpec); ok {
					if ts.Name.Name == name {
						return gd.Doc, true
					}
				}
			}
		}
	}
	return nil, false
}

func newTypeIndex(pkgs []*packages.Package,
	excludeDeps bool, includeTags, excludeTags map[string]bool,
	includePkgs, excludePkgs []string) (*typeIndex, error) {

	ac := &typeIndex{
		AllPackages: make(map[string]*packages.Package),
		Models:      make(map[*ast.Ident]*entityDecl),
		excludeDeps: excludeDeps,
		includeTags: includeTags,
		excludeTags: excludeTags,
		includePkgs: includePkgs,
		excludePkgs: excludePkgs,
	}
	if err := ac.build(pkgs); err != nil {
		return nil, err
	}
	return ac, nil
}

type typeIndex struct {
	AllPackages map[string]*packages.Package
	Models      map[*ast.Ident]*entityDecl
	Meta        []metaSection
	Routes      []parsedPathContent
	Operations  []parsedPathContent
	Parameters  []*entityDecl
	Responses   []*entityDecl
	excludeDeps bool
	includeTags map[string]bool
	excludeTags map[string]bool
	includePkgs []string
	excludePkgs []string
}

func (a *typeIndex) build(pkgs []*packages.Package) error {
	for _, pkg := range pkgs {
		if _, known := a.AllPackages[pkg.PkgPath]; known {
			continue
		}
		a.AllPackages[pkg.PkgPath] = pkg
		if err := a.processPackage(pkg); err != nil {
			return err
		}
		if err := a.walkImports(pkg); err != nil {
			return err
		}
	}

	return nil
}

func (a *typeIndex) processPackage(pkg *packages.Package) error {
	if !shouldAcceptPkg(pkg.PkgPath, a.includePkgs, a.excludePkgs) {
		debugLog("package %s is ignored due to rules", pkg.Name)
		return nil
	}

	for _, file := range pkg.Syntax {
		n, err := a.detectNodes(file)
		if err != nil {
			return err
		}

		if n&metaNode != 0 {
			a.Meta = append(a.Meta, metaSection{Comments: file.Doc})
		}

		if n&operationNode != 0 {
			for _, cmts := range file.Comments {
				pp := parsePathAnnotation(rxOperation, cmts.List)
				if pp.Method == "" {
					continue // not a valid operation
				}
				if !shouldAcceptTag(pp.Tags, a.includeTags, a.excludeTags) {
					debugLog("operation %s %s is ignored due to tag rules", pp.Method, pp.Path)
					continue
				}
				a.Operations = append(a.Operations, pp)
			}
		}

		if n&routeNode != 0 {
			for _, cmts := range file.Comments {
				pp := parsePathAnnotation(rxRoute, cmts.List)
				if pp.Method == "" {
					continue // not a valid operation
				}
				if !shouldAcceptTag(pp.Tags, a.includeTags, a.excludeTags) {
					debugLog("operation %s %s is ignored due to tag rules", pp.Method, pp.Path)
					continue
				}
				a.Routes = append(a.Routes, pp)
			}
		}

		for _, dt := range file.Decls {
			switch fd := dt.(type) {
			case *ast.BadDecl:
				continue
			case *ast.FuncDecl:
				if fd.Body == nil {
					continue
				}
				for _, stmt := range fd.Body.List {
					if dstm, ok := stmt.(*ast.DeclStmt); ok {
						if gd, isGD := dstm.Decl.(*ast.GenDecl); isGD {
							a.processDecl(pkg, file, n, gd)
						}
					}
				}
			case *ast.GenDecl:
				a.processDecl(pkg, file, n, fd)
			}
		}
	}
	return nil
}

func (a *typeIndex) processDecl(pkg *packages.Package, file *ast.File, n node, gd *ast.GenDecl) {
	for _, sp := range gd.Specs {
		switch ts := sp.(type) {
		case *ast.ValueSpec:
			debugLog("saw value spec: %v", ts.Names)
			return
		case *ast.ImportSpec:
			debugLog("saw import spec: %v", ts.Name)
			return
		case *ast.TypeSpec:
			def, ok := pkg.TypesInfo.Defs[ts.Name]
			if !ok {
				debugLog("couldn't find type info for %s", ts.Name)
				//continue
			}
			nt, isNamed := def.Type().(*types.Named)
			if !isNamed {
				debugLog("%s is not a named type but a %T", ts.Name, def.Type())
				//continue
			}
			decl := &entityDecl{
				Comments: gd.Doc,
				Type:     nt,
				Ident:    ts.Name,
				Spec:     ts,
				File:     file,
				Pkg:      pkg,
			}
			key := ts.Name
			if n&modelNode != 0 && decl.HasModelAnnotation() {
				a.Models[key] = decl
			}
			if n&parametersNode != 0 && decl.HasParameterAnnotation() {
				a.Parameters = append(a.Parameters, decl)
			}
			if n&responseNode != 0 && decl.HasResponseAnnotation() {
				a.Responses = append(a.Responses, decl)
			}
		}
	}
}

func (a *typeIndex) walkImports(pkg *packages.Package) error {
	if a.excludeDeps {
		return nil
	}
	for k := range pkg.Imports {
		if _, known := a.AllPackages[k]; known {
			continue
		}
		pk := pkg.Imports[k]
		a.AllPackages[pk.PkgPath] = pk
		if err := a.processPackage(pk); err != nil {
			return err
		}
		if err := a.walkImports(pk); err != nil {
			return err
		}
	}
	return nil
}

func (a *typeIndex) detectNodes(file *ast.File) (node, error) {
	var n node
	for _, comments := range file.Comments {
		var seenStruct string
		for _, cline := range comments.List {
			if cline == nil {
				continue
			}
		}

		for _, cline := range comments.List {
			if cline == nil {
				continue
			}

			matches := rxSwaggerAnnotation.FindStringSubmatch(cline.Text)
			if len(matches) < 2 {
				continue
			}

			switch matches[1] {
			case "route":
				n |= routeNode
			case "operation":
				n |= operationNode
			case "model":
				n |= modelNode
				if seenStruct == "" || seenStruct == matches[1] {
					seenStruct = matches[1]
				} else {
					return 0, fmt.Errorf("classifier: already annotated as %s, can't also be %q", seenStruct, matches[1])
				}
			case "meta":
				n |= metaNode
			case "parameters":
				n |= parametersNode
				if seenStruct == "" || seenStruct == matches[1] {
					seenStruct = matches[1]
				} else {
					return 0, fmt.Errorf("classifier: already annotated as %s, can't also be %q", seenStruct, matches[1])
				}
			case "response":
				n |= responseNode
				if seenStruct == "" || seenStruct == matches[1] {
					seenStruct = matches[1]
				} else {
					return 0, fmt.Errorf("classifier: already annotated as %s, can't also be %q", seenStruct, matches[1])
				}
			case "strfmt", "name", "discriminated", "file", "enum", "default", "alias", "type":
				// TODO: perhaps collect these and pass along to avoid lookups later on
			case "allOf":
			case "ignore":
			default:
				return 0, fmt.Errorf("classifier: unknown swagger annotation %q", matches[1])
			}
		}
	}
	return n, nil
}

func debugLog(format string, args ...interface{}) {
	if Debug {
		log.Printf(format, args...)
	}
}
