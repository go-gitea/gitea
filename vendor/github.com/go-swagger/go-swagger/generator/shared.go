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
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"unicode"

	swaggererrors "github.com/go-openapi/errors"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
	"golang.org/x/tools/imports"
)

//go:generate go-bindata -mode 420 -modtime 1482416923 -pkg=generator -ignore=.*\.sw? -ignore=.*\.md ./templates/...

// LanguageOpts to describe a language to the code generator
type LanguageOpts struct {
	ReservedWords    []string
	BaseImportFunc   func(string) string `json:"-"`
	reservedWordsSet map[string]struct{}
	initialized      bool
	formatFunc       func(string, []byte) ([]byte, error)
	fileNameFunc     func(string) string
}

// Init the language option
func (l *LanguageOpts) Init() {
	if !l.initialized {
		l.initialized = true
		l.reservedWordsSet = make(map[string]struct{})
		for _, rw := range l.ReservedWords {
			l.reservedWordsSet[rw] = struct{}{}
		}
	}
}

// MangleName makes sure a reserved word gets a safe name
func (l *LanguageOpts) MangleName(name, suffix string) string {
	if _, ok := l.reservedWordsSet[swag.ToFileName(name)]; !ok {
		return name
	}
	return strings.Join([]string{name, suffix}, "_")
}

// MangleVarName makes sure a reserved word gets a safe name
func (l *LanguageOpts) MangleVarName(name string) string {
	nm := swag.ToVarName(name)
	if _, ok := l.reservedWordsSet[nm]; !ok {
		return nm
	}
	return nm + "Var"
}

// MangleFileName makes sure a file name gets a safe name
func (l *LanguageOpts) MangleFileName(name string) string {
	if l.fileNameFunc != nil {
		return l.fileNameFunc(name)
	}
	return swag.ToFileName(name)
}

// ManglePackageName makes sure a package gets a safe name.
// In case of a file system path (e.g. name contains "/" or "\" on Windows), this return only the last element.
func (l *LanguageOpts) ManglePackageName(name, suffix string) string {
	if name == "" {
		return suffix
	}
	pth := filepath.ToSlash(filepath.Clean(name)) // preserve path
	_, pkg := path.Split(pth)                     // drop path
	return l.MangleName(swag.ToFileName(pkg), suffix)
}

// ManglePackagePath makes sure a full package path gets a safe name.
// Only the last part of the path is altered.
func (l *LanguageOpts) ManglePackagePath(name string, suffix string) string {
	if name == "" {
		return suffix
	}
	target := filepath.ToSlash(filepath.Clean(name)) // preserve path
	parts := strings.Split(target, "/")
	parts[len(parts)-1] = l.ManglePackageName(parts[len(parts)-1], suffix)
	return strings.Join(parts, "/")
}

// FormatContent formats a file with a language specific formatter
func (l *LanguageOpts) FormatContent(name string, content []byte) ([]byte, error) {
	if l.formatFunc != nil {
		return l.formatFunc(name, content)
	}
	return content, nil
}

func (l *LanguageOpts) baseImport(tgt string) string {
	if l.BaseImportFunc != nil {
		return l.BaseImportFunc(tgt)
	}
	return ""
}

var golang = GoLangOpts()

// GoLangOpts for rendering items as golang code
func GoLangOpts() *LanguageOpts {
	var goOtherReservedSuffixes = map[string]bool{
		// see:
		// https://golang.org/src/go/build/syslist.go
		// https://golang.org/doc/install/source#environment

		// goos
		"android":   true,
		"darwin":    true,
		"dragonfly": true,
		"freebsd":   true,
		"js":        true,
		"linux":     true,
		"nacl":      true,
		"netbsd":    true,
		"openbsd":   true,
		"plan9":     true,
		"solaris":   true,
		"windows":   true,
		"zos":       true,

		// arch
		"386":         true,
		"amd64":       true,
		"amd64p32":    true,
		"arm":         true,
		"armbe":       true,
		"arm64":       true,
		"arm64be":     true,
		"mips":        true,
		"mipsle":      true,
		"mips64":      true,
		"mips64le":    true,
		"mips64p32":   true,
		"mips64p32le": true,
		"ppc":         true,
		"ppc64":       true,
		"ppc64le":     true,
		"riscv":       true,
		"riscv64":     true,
		"s390":        true,
		"s390x":       true,
		"sparc":       true,
		"sparc64":     true,
		"wasm":        true,

		// other reserved suffixes
		"test": true,
	}

	opts := new(LanguageOpts)
	opts.ReservedWords = []string{
		"break", "default", "func", "interface", "select",
		"case", "defer", "go", "map", "struct",
		"chan", "else", "goto", "package", "switch",
		"const", "fallthrough", "if", "range", "type",
		"continue", "for", "import", "return", "var",
	}
	opts.formatFunc = func(ffn string, content []byte) ([]byte, error) {
		opts := new(imports.Options)
		opts.TabIndent = true
		opts.TabWidth = 2
		opts.Fragment = true
		opts.Comments = true
		return imports.Process(ffn, content, opts)
	}
	opts.fileNameFunc = func(name string) string {
		// whenever a generated file name ends with a suffix
		// that is meaningful to go build, adds a "swagger"
		// suffix
		parts := strings.Split(swag.ToFileName(name), "_")
		if goOtherReservedSuffixes[parts[len(parts)-1]] {
			// file name ending with a reserved arch or os name
			// are appended an innocuous suffix "swagger"
			parts = append(parts, "swagger")
		}
		return strings.Join(parts, "_")
	}

	opts.BaseImportFunc = func(tgt string) string {
		tgt = filepath.Clean(tgt)
		// On Windows, filepath.Abs("") behaves differently than on Unix.
		// Windows: yields an error, since Abs() does not know the volume.
		// UNIX: returns current working directory
		if tgt == "" {
			tgt = "."
		}
		tgtAbsPath, err := filepath.Abs(tgt)
		if err != nil {
			log.Fatalf("could not evaluate base import path with target \"%s\": %v", tgt, err)
		}

		var tgtAbsPathExtended string
		tgtAbsPathExtended, err = filepath.EvalSymlinks(tgtAbsPath)
		if err != nil {
			log.Fatalf("could not evaluate base import path with target \"%s\" (with symlink resolution): %v", tgtAbsPath, err)
		}

		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			gopath = filepath.Join(os.Getenv("HOME"), "go")
		}

		var pth string
		for _, gp := range filepath.SplitList(gopath) {
			// EvalSymLinks also calls the Clean
			gopathExtended, er := filepath.EvalSymlinks(gp)
			if er != nil {
				log.Fatalln(er)
			}
			gopathExtended = filepath.Join(gopathExtended, "src")
			gp = filepath.Join(gp, "src")

			// At this stage we have expanded and unexpanded target path. GOPATH is fully expanded.
			// Expanded means symlink free.
			// We compare both types of targetpath<s> with gopath.
			// If any one of them coincides with gopath , it is imperative that
			// target path lies inside gopath. How?
			// 		- Case 1: Irrespective of symlinks paths coincide. Both non-expanded paths.
			// 		- Case 2: Symlink in target path points to location inside GOPATH. (Expanded Target Path)
			//    - Case 3: Symlink in target path points to directory outside GOPATH (Unexpanded target path)

			// Case 1: - Do nothing case. If non-expanded paths match just generate base import path as if
			//				   there are no symlinks.

			// Case 2: - Symlink in target path points to location inside GOPATH. (Expanded Target Path)
			//					 First if will fail. Second if will succeed.

			// Case 3: - Symlink in target path points to directory outside GOPATH (Unexpanded target path)
			// 					 First if will succeed and break.

			//compares non expanded path for both
			if ok, relativepath := checkPrefixAndFetchRelativePath(tgtAbsPath, gp); ok {
				pth = relativepath
				break
			}

			// Compares non-expanded target path
			if ok, relativepath := checkPrefixAndFetchRelativePath(tgtAbsPath, gopathExtended); ok {
				pth = relativepath
				break
			}

			// Compares expanded target path.
			if ok, relativepath := checkPrefixAndFetchRelativePath(tgtAbsPathExtended, gopathExtended); ok {
				pth = relativepath
				break
			}

		}

		mod, goModuleAbsPath, err := tryResolveModule(tgtAbsPath)
		switch {
		case err != nil:
			log.Fatalf("Failed to resolve module using go.mod file: %s", err)
		case mod != "":
			relTgt := relPathToRelGoPath(goModuleAbsPath, tgtAbsPath)
			if !strings.HasSuffix(mod, relTgt) {
				return mod + relTgt
			}
			return mod
		}

		if pth == "" {
			log.Fatalln("target must reside inside a location in the $GOPATH/src or be a module")
		}
		return pth
	}
	opts.Init()
	return opts
}

var moduleRe = regexp.MustCompile(`module[ \t]+([^\s]+)`)

// resolveGoModFile walks up the directory tree starting from 'dir' until it
// finds a go.mod file. If go.mod is found it will return the related file
// object. If no go.mod file is found it will return an error.
func resolveGoModFile(dir string) (*os.File, string, error) {
	goModPath := filepath.Join(dir, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		if os.IsNotExist(err) && dir != filepath.Dir(dir) {
			return resolveGoModFile(filepath.Dir(dir))
		}
		return nil, "", err
	}
	return f, dir, nil
}

// relPathToRelGoPath takes a relative os path and returns the relative go
// package path. For unix nothing will change but for windows \ will be
// converted to /.
func relPathToRelGoPath(modAbsPath, absPath string) string {
	if absPath == "." {
		return ""
	}

	path := strings.TrimPrefix(absPath, modAbsPath)
	pathItems := strings.Split(path, string(filepath.Separator))
	return strings.Join(pathItems, "/")
}

func tryResolveModule(baseTargetPath string) (string, string, error) {
	f, goModAbsPath, err := resolveGoModFile(baseTargetPath)
	switch {
	case os.IsNotExist(err):
		return "", "", nil
	case err != nil:
		return "", "", err
	}

	src, err := ioutil.ReadAll(f)
	if err != nil {
		return "", "", err
	}

	match := moduleRe.FindSubmatch(src)
	if len(match) != 2 {
		return "", "", nil
	}

	return string(match[1]), goModAbsPath, nil
}

func findSwaggerSpec(nm string) (string, error) {
	specs := []string{"swagger.json", "swagger.yml", "swagger.yaml"}
	if nm != "" {
		specs = []string{nm}
	}
	var name string
	for _, nn := range specs {
		f, err := os.Stat(nn)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err != nil && os.IsNotExist(err) {
			continue
		}
		if f.IsDir() {
			return "", fmt.Errorf("%s is a directory", nn)
		}
		name = nn
		break
	}
	if name == "" {
		return "", errors.New("couldn't find a swagger spec")
	}
	return name, nil
}

// DefaultSectionOpts for a given opts, this is used when no config file is passed
// and uses the embedded templates when no local override can be found
func DefaultSectionOpts(gen *GenOpts) {
	sec := gen.Sections
	if len(sec.Models) == 0 {
		sec.Models = []TemplateOpts{
			{
				Name:     "definition",
				Source:   "asset:model",
				Target:   "{{ joinFilePath .Target (toPackagePath .ModelPackage) }}",
				FileName: "{{ (snakize (pascalize .Name)) }}.go",
			},
		}
	}

	if len(sec.Operations) == 0 {
		if gen.IsClient {
			sec.Operations = []TemplateOpts{
				{
					Name:     "parameters",
					Source:   "asset:clientParameter",
					Target:   "{{ joinFilePath .Target (toPackagePath .ClientPackage) (toPackagePath .Package) }}",
					FileName: "{{ (snakize (pascalize .Name)) }}_parameters.go",
				},
				{
					Name:     "responses",
					Source:   "asset:clientResponse",
					Target:   "{{ joinFilePath .Target (toPackagePath .ClientPackage) (toPackagePath .Package) }}",
					FileName: "{{ (snakize (pascalize .Name)) }}_responses.go",
				},
			}

		} else {
			ops := []TemplateOpts{}
			if gen.IncludeParameters {
				ops = append(ops, TemplateOpts{
					Name:     "parameters",
					Source:   "asset:serverParameter",
					Target:   "{{ if gt (len .Tags) 0 }}{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .APIPackage) (toPackagePath .Package)  }}{{ else }}{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .Package) }}{{ end }}",
					FileName: "{{ (snakize (pascalize .Name)) }}_parameters.go",
				})
			}
			if gen.IncludeURLBuilder {
				ops = append(ops, TemplateOpts{
					Name:     "urlbuilder",
					Source:   "asset:serverUrlbuilder",
					Target:   "{{ if gt (len .Tags) 0 }}{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .APIPackage) (toPackagePath .Package) }}{{ else }}{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .Package) }}{{ end }}",
					FileName: "{{ (snakize (pascalize .Name)) }}_urlbuilder.go",
				})
			}
			if gen.IncludeResponses {
				ops = append(ops, TemplateOpts{
					Name:     "responses",
					Source:   "asset:serverResponses",
					Target:   "{{ if gt (len .Tags) 0 }}{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .APIPackage) (toPackagePath .Package) }}{{ else }}{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .Package) }}{{ end }}",
					FileName: "{{ (snakize (pascalize .Name)) }}_responses.go",
				})
			}
			if gen.IncludeHandler {
				ops = append(ops, TemplateOpts{
					Name:     "handler",
					Source:   "asset:serverOperation",
					Target:   "{{ if gt (len .Tags) 0 }}{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .APIPackage) (toPackagePath .Package) }}{{ else }}{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .Package) }}{{ end }}",
					FileName: "{{ (snakize (pascalize .Name)) }}.go",
				})
			}
			sec.Operations = ops
		}
	}

	if len(sec.OperationGroups) == 0 {
		if gen.IsClient {
			sec.OperationGroups = []TemplateOpts{
				{
					Name:     "client",
					Source:   "asset:clientClient",
					Target:   "{{ joinFilePath .Target (toPackagePath .ClientPackage) (toPackagePath .Name)}}",
					FileName: "{{ (snakize (pascalize .Name)) }}_client.go",
				},
			}
		} else {
			sec.OperationGroups = []TemplateOpts{}
		}
	}

	if len(sec.Application) == 0 {
		if gen.IsClient {
			sec.Application = []TemplateOpts{
				{
					Name:     "facade",
					Source:   "asset:clientFacade",
					Target:   "{{ joinFilePath .Target (toPackagePath .ClientPackage) }}",
					FileName: "{{ snakize .Name }}Client.go",
				},
			}
		} else {
			sec.Application = []TemplateOpts{
				{
					Name:       "configure",
					Source:     "asset:serverConfigureapi",
					Target:     "{{ joinFilePath .Target (toPackagePath .ServerPackage) }}",
					FileName:   "configure_{{ (snakize (pascalize .Name)) }}.go",
					SkipExists: !gen.RegenerateConfigureAPI,
				},
				{
					Name:     "main",
					Source:   "asset:serverMain",
					Target:   "{{ joinFilePath .Target \"cmd\" (dasherize (pascalize .Name)) }}-server",
					FileName: "main.go",
				},
				{
					Name:     "embedded_spec",
					Source:   "asset:swaggerJsonEmbed",
					Target:   "{{ joinFilePath .Target (toPackagePath .ServerPackage) }}",
					FileName: "embedded_spec.go",
				},
				{
					Name:     "server",
					Source:   "asset:serverServer",
					Target:   "{{ joinFilePath .Target (toPackagePath .ServerPackage) }}",
					FileName: "server.go",
				},
				{
					Name:     "builder",
					Source:   "asset:serverBuilder",
					Target:   "{{ joinFilePath .Target (toPackagePath .ServerPackage) (toPackagePath .APIPackage) }}",
					FileName: "{{ snakize (pascalize .Name) }}_api.go",
				},
				{
					Name:     "doc",
					Source:   "asset:serverDoc",
					Target:   "{{ joinFilePath .Target (toPackagePath .ServerPackage) }}",
					FileName: "doc.go",
				},
			}
		}
	}
	gen.Sections = sec

}

// TemplateOpts allows
type TemplateOpts struct {
	Name       string `mapstructure:"name"`
	Source     string `mapstructure:"source"`
	Target     string `mapstructure:"target"`
	FileName   string `mapstructure:"file_name"`
	SkipExists bool   `mapstructure:"skip_exists"`
	SkipFormat bool   `mapstructure:"skip_format"`
}

// SectionOpts allows for specifying options to customize the templates used for generation
type SectionOpts struct {
	Application     []TemplateOpts `mapstructure:"application"`
	Operations      []TemplateOpts `mapstructure:"operations"`
	OperationGroups []TemplateOpts `mapstructure:"operation_groups"`
	Models          []TemplateOpts `mapstructure:"models"`
}

// GenOpts the options for the generator
type GenOpts struct {
	IncludeModel               bool
	IncludeValidator           bool
	IncludeHandler             bool
	IncludeParameters          bool
	IncludeResponses           bool
	IncludeURLBuilder          bool
	IncludeMain                bool
	IncludeSupport             bool
	ExcludeSpec                bool
	DumpData                   bool
	ValidateSpec               bool
	FlattenOpts                *analysis.FlattenOpts
	IsClient                   bool
	defaultsEnsured            bool
	PropertiesSpecOrder        bool
	StrictAdditionalProperties bool
	AllowTemplateOverride      bool

	Spec                   string
	APIPackage             string
	ModelPackage           string
	ServerPackage          string
	ClientPackage          string
	Principal              string
	Target                 string
	Sections               SectionOpts
	LanguageOpts           *LanguageOpts
	TypeMapping            map[string]string
	Imports                map[string]string
	DefaultScheme          string
	DefaultProduces        string
	DefaultConsumes        string
	TemplateDir            string
	Template               string
	RegenerateConfigureAPI bool
	Operations             []string
	Models                 []string
	Tags                   []string
	Name                   string
	FlagStrategy           string
	CompatibilityMode      string
	ExistingModels         string
	Copyright              string
}

// CheckOpts carries out some global consistency checks on options.
//
// At the moment, these checks simply protect TargetPath() and SpecPath()
// functions. More checks may be added here.
func (g *GenOpts) CheckOpts() error {
	if !filepath.IsAbs(g.Target) {
		if _, err := filepath.Abs(g.Target); err != nil {
			return fmt.Errorf("could not locate target %s: %v", g.Target, err)
		}
	}
	if filepath.IsAbs(g.ServerPackage) {
		return fmt.Errorf("you shouldn't specify an absolute path in --server-package: %s", g.ServerPackage)
	}
	if !filepath.IsAbs(g.Spec) && !strings.HasPrefix(g.Spec, "http://") && !strings.HasPrefix(g.Spec, "https://") {
		if _, err := filepath.Abs(g.Spec); err != nil {
			return fmt.Errorf("could not locate spec: %s", g.Spec)
		}
	}
	return nil
}

// TargetPath returns the target generation path relative to the server package.
// This method is used by templates, e.g. with {{ .TargetPath }}
//
// Errors cases are prevented by calling CheckOpts beforehand.
//
// Example:
// Target: ${PWD}/tmp
// ServerPackage: abc/efg
//
// Server is generated in ${PWD}/tmp/abc/efg
// relative TargetPath returned: ../../../tmp
//
func (g *GenOpts) TargetPath() string {
	var tgt string
	if g.Target == "" {
		tgt = "." // That's for windows
	} else {
		tgt = g.Target
	}
	tgtAbs, _ := filepath.Abs(tgt)
	srvPkg := filepath.FromSlash(g.LanguageOpts.ManglePackagePath(g.ServerPackage, "server"))
	srvrAbs := filepath.Join(tgtAbs, srvPkg)
	tgtRel, _ := filepath.Rel(srvrAbs, filepath.Dir(tgtAbs))
	tgtRel = filepath.Join(tgtRel, filepath.Base(tgtAbs))
	return tgtRel
}

// SpecPath returns the path to the spec relative to the server package.
// If the spec is remote keep this absolute location.
//
// If spec is not relative to server (e.g. lives on a different drive on windows),
// then the resolved path is absolute.
//
// This method is used by templates, e.g. with {{ .SpecPath }}
//
// Errors cases are prevented by calling CheckOpts beforehand.
func (g *GenOpts) SpecPath() string {
	if strings.HasPrefix(g.Spec, "http://") || strings.HasPrefix(g.Spec, "https://") {
		return g.Spec
	}
	// Local specifications
	specAbs, _ := filepath.Abs(g.Spec)
	var tgt string
	if g.Target == "" {
		tgt = "." // That's for windows
	} else {
		tgt = g.Target
	}
	tgtAbs, _ := filepath.Abs(tgt)
	srvPkg := filepath.FromSlash(g.LanguageOpts.ManglePackagePath(g.ServerPackage, "server"))
	srvAbs := filepath.Join(tgtAbs, srvPkg)
	specRel, err := filepath.Rel(srvAbs, specAbs)
	if err != nil {
		return specAbs
	}
	return specRel
}

// EnsureDefaults for these gen opts
func (g *GenOpts) EnsureDefaults() error {
	if g.defaultsEnsured {
		return nil
	}
	DefaultSectionOpts(g)
	if g.LanguageOpts == nil {
		g.LanguageOpts = GoLangOpts()
	}
	// set defaults for flattening options
	g.FlattenOpts = &analysis.FlattenOpts{
		Minimal:      true,
		Verbose:      true,
		RemoveUnused: false,
		Expand:       false,
	}
	g.defaultsEnsured = true
	return nil
}

func (g *GenOpts) location(t *TemplateOpts, data interface{}) (string, string, error) {
	v := reflect.Indirect(reflect.ValueOf(data))
	fld := v.FieldByName("Name")
	var name string
	if fld.IsValid() {
		log.Println("name field", fld.String())
		name = fld.String()
	}

	fldpack := v.FieldByName("Package")
	pkg := g.APIPackage
	if fldpack.IsValid() {
		log.Println("package field", fldpack.String())
		pkg = fldpack.String()
	}

	var tags []string
	tagsF := v.FieldByName("Tags")
	if tagsF.IsValid() {
		tags = tagsF.Interface().([]string)
	}

	pthTpl, err := template.New(t.Name + "-target").Funcs(FuncMap).Parse(t.Target)
	if err != nil {
		return "", "", err
	}

	fNameTpl, err := template.New(t.Name + "-filename").Funcs(FuncMap).Parse(t.FileName)
	if err != nil {
		return "", "", err
	}

	d := struct {
		Name, Package, APIPackage, ServerPackage, ClientPackage, ModelPackage, Target string
		Tags                                                                          []string
	}{
		Name:          name,
		Package:       pkg,
		APIPackage:    g.APIPackage,
		ServerPackage: g.ServerPackage,
		ClientPackage: g.ClientPackage,
		ModelPackage:  g.ModelPackage,
		Target:        g.Target,
		Tags:          tags,
	}

	// pretty.Println(data)
	var pthBuf bytes.Buffer
	if e := pthTpl.Execute(&pthBuf, d); e != nil {
		return "", "", e
	}

	var fNameBuf bytes.Buffer
	if e := fNameTpl.Execute(&fNameBuf, d); e != nil {
		return "", "", e
	}
	return pthBuf.String(), fileName(fNameBuf.String()), nil
}

func (g *GenOpts) render(t *TemplateOpts, data interface{}) ([]byte, error) {
	var templ *template.Template

	if strings.HasPrefix(strings.ToLower(t.Source), "asset:") {
		tt, err := templates.Get(strings.TrimPrefix(t.Source, "asset:"))
		if err != nil {
			return nil, err
		}
		templ = tt
	}

	if templ == nil {
		// try to load from repository (and enable dependencies)
		name := swag.ToJSONName(strings.TrimSuffix(t.Source, ".gotmpl"))
		tt, err := templates.Get(name)
		if err == nil {
			templ = tt
		}
	}

	if templ == nil {
		// try to load template from disk, in TemplateDir if specified
		// (dependencies resolution is limited to preloaded assets)
		var templateFile string
		if g.TemplateDir != "" {
			templateFile = filepath.Join(g.TemplateDir, t.Source)
		} else {
			templateFile = t.Source
		}
		content, err := ioutil.ReadFile(templateFile)
		if err != nil {
			return nil, fmt.Errorf("error while opening %s template file: %v", templateFile, err)
		}
		tt, err := template.New(t.Source).Funcs(FuncMap).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("template parsing failed on template %s: %v", t.Name, err)
		}
		templ = tt
	}

	if templ == nil {
		return nil, fmt.Errorf("template %q not found", t.Source)
	}

	var tBuf bytes.Buffer
	if err := templ.Execute(&tBuf, data); err != nil {
		return nil, fmt.Errorf("template execution failed for template %s: %v", t.Name, err)
	}
	log.Printf("executed template %s", t.Source)

	return tBuf.Bytes(), nil
}

// Render template and write generated source code
// generated code is reformatted ("linted"), which gives an
// additional level of checking. If this step fails, the generated
// code is still dumped, for template debugging purposes.
func (g *GenOpts) write(t *TemplateOpts, data interface{}) error {
	dir, fname, err := g.location(t, data)
	if err != nil {
		return fmt.Errorf("failed to resolve template location for template %s: %v", t.Name, err)
	}

	if t.SkipExists && fileExists(dir, fname) {
		debugLog("skipping generation of %s because it already exists and skip_exist directive is set for %s",
			filepath.Join(dir, fname), t.Name)
		return nil
	}

	log.Printf("creating generated file %q in %q as %s", fname, dir, t.Name)
	content, err := g.render(t, data)
	if err != nil {
		return fmt.Errorf("failed rendering template data for %s: %v", t.Name, err)
	}

	if dir != "" {
		_, exists := os.Stat(dir)
		if os.IsNotExist(exists) {
			debugLog("creating directory %q for \"%s\"", dir, t.Name)
			// Directory settings consistent with file privileges.
			// Environment's umask may alter this setup
			if e := os.MkdirAll(dir, 0755); e != nil {
				return e
			}
		}
	}

	// Conditionally format the code, unless the user wants to skip
	formatted := content
	var writeerr error

	if !t.SkipFormat {
		formatted, err = g.LanguageOpts.FormatContent(fname, content)
		if err != nil {
			log.Printf("source formatting failed on template-generated source (%q for %s). Check that your template produces valid code", filepath.Join(dir, fname), t.Name)
			writeerr = ioutil.WriteFile(filepath.Join(dir, fname), content, 0644)
			if writeerr != nil {
				return fmt.Errorf("failed to write (unformatted) file %q in %q: %v", fname, dir, writeerr)
			}
			log.Printf("unformatted generated source %q has been dumped for template debugging purposes. DO NOT build on this source!", fname)
			return fmt.Errorf("source formatting on generated source %q failed: %v", t.Name, err)
		}
	}

	writeerr = ioutil.WriteFile(filepath.Join(dir, fname), formatted, 0644)
	if writeerr != nil {
		return fmt.Errorf("failed to write file %q in %q: %v", fname, dir, writeerr)
	}
	return err
}

func fileName(in string) string {
	ext := filepath.Ext(in)
	return swag.ToFileName(strings.TrimSuffix(in, ext)) + ext
}

func (g *GenOpts) shouldRenderApp(t *TemplateOpts, app *GenApp) bool {
	switch swag.ToFileName(swag.ToGoName(t.Name)) {
	case "main":
		return g.IncludeMain
	case "embedded_spec":
		return !g.ExcludeSpec
	default:
		return true
	}
}

func (g *GenOpts) shouldRenderOperations() bool {
	return g.IncludeHandler || g.IncludeParameters || g.IncludeResponses
}

func (g *GenOpts) renderApplication(app *GenApp) error {
	log.Printf("rendering %d templates for application %s", len(g.Sections.Application), app.Name)
	for _, templ := range g.Sections.Application {
		if !g.shouldRenderApp(&templ, app) {
			continue
		}
		if err := g.write(&templ, app); err != nil {
			return err
		}
	}
	return nil
}

func (g *GenOpts) renderOperationGroup(gg *GenOperationGroup) error {
	log.Printf("rendering %d templates for operation group %s", len(g.Sections.OperationGroups), g.Name)
	for _, templ := range g.Sections.OperationGroups {
		if !g.shouldRenderOperations() {
			continue
		}

		if err := g.write(&templ, gg); err != nil {
			return err
		}
	}
	return nil
}

func (g *GenOpts) renderOperation(gg *GenOperation) error {
	log.Printf("rendering %d templates for operation %s", len(g.Sections.Operations), g.Name)
	for _, templ := range g.Sections.Operations {
		if !g.shouldRenderOperations() {
			continue
		}

		if err := g.write(&templ, gg); err != nil {
			return err
		}
	}
	return nil
}

func (g *GenOpts) renderDefinition(gg *GenDefinition) error {
	log.Printf("rendering %d templates for model %s", len(g.Sections.Models), gg.Name)
	for _, templ := range g.Sections.Models {
		if !g.IncludeModel {
			continue
		}

		if err := g.write(&templ, gg); err != nil {
			return err
		}
	}
	return nil
}

func validateSpec(path string, doc *loads.Document) (err error) {
	if doc == nil {
		if path, doc, err = loadSpec(path); err != nil {
			return err
		}
	}

	result := validate.Spec(doc, strfmt.Default)
	if result == nil {
		return nil
	}

	str := fmt.Sprintf("The swagger spec at %q is invalid against swagger specification %s. see errors :\n", path, doc.Version())
	for _, desc := range result.(*swaggererrors.CompositeError).Errors {
		str += fmt.Sprintf("- %s\n", desc)
	}
	return errors.New(str)
}

func loadSpec(specFile string) (string, *loads.Document, error) {
	// find swagger spec document, verify it exists
	specPath := specFile
	var err error
	if !strings.HasPrefix(specPath, "http") {
		specPath, err = findSwaggerSpec(specFile)
		if err != nil {
			return "", nil, err
		}
	}

	// load swagger spec
	specDoc, err := loads.Spec(specPath)
	if err != nil {
		return "", nil, err
	}
	return specPath, specDoc, nil
}

func fileExists(target, name string) bool {
	_, err := os.Stat(filepath.Join(target, name))
	return !os.IsNotExist(err)
}

func gatherModels(specDoc *loads.Document, modelNames []string) (map[string]spec.Schema, error) {
	models, mnc := make(map[string]spec.Schema), len(modelNames)
	defs := specDoc.Spec().Definitions

	if mnc > 0 {
		var unknownModels []string
		for _, m := range modelNames {
			_, ok := defs[m]
			if !ok {
				unknownModels = append(unknownModels, m)
			}
		}
		if len(unknownModels) != 0 {
			return nil, fmt.Errorf("unknown models: %s", strings.Join(unknownModels, ", "))
		}
	}
	for k, v := range defs {
		if mnc == 0 {
			models[k] = v
		}
		for _, nm := range modelNames {
			if k == nm {
				models[k] = v
			}
		}
	}
	return models, nil
}

func appNameOrDefault(specDoc *loads.Document, name, defaultName string) string {
	if strings.TrimSpace(name) == "" {
		if specDoc.Spec().Info != nil && strings.TrimSpace(specDoc.Spec().Info.Title) != "" {
			name = specDoc.Spec().Info.Title
		} else {
			name = defaultName
		}
	}
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(swag.ToGoName(name), "Test"), "API"), "Test")
}

func containsString(names []string, name string) bool {
	for _, nm := range names {
		if nm == name {
			return true
		}
	}
	return false
}

type opRef struct {
	Method string
	Path   string
	Key    string
	ID     string
	Op     *spec.Operation
}

type opRefs []opRef

func (o opRefs) Len() int           { return len(o) }
func (o opRefs) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o opRefs) Less(i, j int) bool { return o[i].Key < o[j].Key }

func gatherOperations(specDoc *analysis.Spec, operationIDs []string) map[string]opRef {
	var oprefs opRefs

	for method, pathItem := range specDoc.Operations() {
		for path, operation := range pathItem {
			// nm := ensureUniqueName(operation.ID, method, path, operations)
			vv := *operation
			oprefs = append(oprefs, opRef{
				Key:    swag.ToGoName(strings.ToLower(method) + " " + path),
				Method: method,
				Path:   path,
				ID:     vv.ID,
				Op:     &vv,
			})
		}
	}

	sort.Sort(oprefs)

	operations := make(map[string]opRef)
	for _, opr := range oprefs {
		nm := opr.ID
		if nm == "" {
			nm = opr.Key
		}

		oo, found := operations[nm]
		if found && oo.Method != opr.Method && oo.Path != opr.Path {
			nm = opr.Key
		}
		if len(operationIDs) == 0 || containsString(operationIDs, opr.ID) || containsString(operationIDs, nm) {
			opr.ID = nm
			opr.Op.ID = nm
			operations[nm] = opr
		}
	}

	return operations
}

func pascalize(arg string) string {
	runes := []rune(arg)
	switch len(runes) {
	case 0:
		return ""
	case 1: // handle special case when we have a single rune that is not handled by swag.ToGoName
		switch runes[0] {
		case '+', '-', '#', '_': // those cases are handled differently than swag utility
			return prefixForName(arg)
		}
	}
	return swag.ToGoName(swag.ToGoName(arg)) // want to remove spaces
}

func prefixForName(arg string) string {
	first := []rune(arg)[0]
	if len(arg) == 0 || unicode.IsLetter(first) {
		return ""
	}
	switch first {
	case '+':
		return "Plus"
	case '-':
		return "Minus"
	case '#':
		return "HashTag"
		// other cases ($,@ etc..) handled by swag.ToGoName
	}
	return "Nr"
}

func init() {
	// this makes the ToGoName func behave with the special
	// prefixing rule above
	swag.GoNamePrefixFunc = prefixForName
}

func pruneEmpty(in []string) (out []string) {
	for _, v := range in {
		if v != "" {
			out = append(out, v)
		}
	}
	return
}

func trimBOM(in string) string {
	return strings.Trim(in, "\xef\xbb\xbf")
}

func validateAndFlattenSpec(opts *GenOpts, specDoc *loads.Document) (*loads.Document, error) {

	var err error

	// Validate if needed
	if opts.ValidateSpec {
		log.Printf("validating spec %v", opts.Spec)
		if erv := validateSpec(opts.Spec, specDoc); erv != nil {
			return specDoc, erv
		}
	}

	// Restore spec to original
	opts.Spec, specDoc, err = loadSpec(opts.Spec)
	if err != nil {
		return nil, err
	}

	absBasePath := specDoc.SpecFilePath()
	if !filepath.IsAbs(absBasePath) {
		cwd, _ := os.Getwd()
		absBasePath = filepath.Join(cwd, absBasePath)
	}

	// Some preprocessing is required before codegen
	//
	// This ensures at least that $ref's in the spec document are canonical,
	// i.e all $ref are local to this file and point to some uniquely named definition.
	//
	// Default option is to ensure minimal flattening of $ref, bundling remote $refs and relocating arbitrary JSON
	// pointers as definitions.
	// This preprocessing may introduce duplicate names (e.g. remote $ref with same name). In this case, a definition
	// suffixed with "OAIGen" is produced.
	//
	// Full flattening option farther transforms the spec by moving every complex object (e.g. with some properties)
	// as a standalone definition.
	//
	// Eventually, an "expand spec" option is available. It is essentially useful for testing purposes.
	//
	// NOTE(fredbi): spec expansion may produce some unsupported constructs and is not yet protected against the
	// following cases:
	//  - polymorphic types generation may fail with expansion (expand destructs the reuse intent of the $ref in allOf)
	//  - name duplicates may occur and result in compilation failures
	// The right place to fix these shortcomings is go-openapi/analysis.

	opts.FlattenOpts.BasePath = absBasePath // BasePath must be absolute
	opts.FlattenOpts.Spec = analysis.New(specDoc.Spec())

	var preprocessingOption string
	switch {
	case opts.FlattenOpts.Expand:
		preprocessingOption = "expand"
	case opts.FlattenOpts.Minimal:
		preprocessingOption = "minimal flattening"
	default:
		preprocessingOption = "full flattening"
	}
	log.Printf("preprocessing spec with option:  %s", preprocessingOption)

	if err = analysis.Flatten(*opts.FlattenOpts); err != nil {
		return nil, err
	}

	// yields the preprocessed spec document
	return specDoc, nil
}

// gatherSecuritySchemes produces a sorted representation from a map of spec security schemes
func gatherSecuritySchemes(securitySchemes map[string]spec.SecurityScheme, appName, principal, receiver string) (security GenSecuritySchemes) {
	for scheme, req := range securitySchemes {
		isOAuth2 := strings.ToLower(req.Type) == "oauth2"
		var scopes []string
		if isOAuth2 {
			for k := range req.Scopes {
				scopes = append(scopes, k)
			}
		}
		sort.Strings(scopes)

		security = append(security, GenSecurityScheme{
			AppName:      appName,
			ID:           scheme,
			ReceiverName: receiver,
			Name:         req.Name,
			IsBasicAuth:  strings.ToLower(req.Type) == "basic",
			IsAPIKeyAuth: strings.ToLower(req.Type) == "apikey",
			IsOAuth2:     isOAuth2,
			Scopes:       scopes,
			Principal:    principal,
			Source:       req.In,
			// from original spec
			Description:      req.Description,
			Type:             strings.ToLower(req.Type),
			In:               req.In,
			Flow:             req.Flow,
			AuthorizationURL: req.AuthorizationURL,
			TokenURL:         req.TokenURL,
			Extensions:       req.Extensions,
		})
	}
	sort.Sort(security)
	return
}

// gatherExtraSchemas produces a sorted list of extra schemas.
//
// ExtraSchemas are inlined types rendered in the same model file.
func gatherExtraSchemas(extraMap map[string]GenSchema) (extras GenSchemaList) {
	var extraKeys []string
	for k := range extraMap {
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)
	for _, k := range extraKeys {
		// figure out if top level validations are needed
		p := extraMap[k]
		p.HasValidations = shallowValidationLookup(p)
		extras = append(extras, p)
	}
	return
}

func sharedValidationsFromSimple(v spec.CommonValidations, isRequired bool) (sh sharedValidations) {
	sh = sharedValidations{
		Required:         isRequired,
		Maximum:          v.Maximum,
		ExclusiveMaximum: v.ExclusiveMaximum,
		Minimum:          v.Minimum,
		ExclusiveMinimum: v.ExclusiveMinimum,
		MaxLength:        v.MaxLength,
		MinLength:        v.MinLength,
		Pattern:          v.Pattern,
		MaxItems:         v.MaxItems,
		MinItems:         v.MinItems,
		UniqueItems:      v.UniqueItems,
		MultipleOf:       v.MultipleOf,
		Enum:             v.Enum,
	}
	return
}

func sharedValidationsFromSchema(v spec.Schema, isRequired bool) (sh sharedValidations) {
	sh = sharedValidations{
		Required:         isRequired,
		Maximum:          v.Maximum,
		ExclusiveMaximum: v.ExclusiveMaximum,
		Minimum:          v.Minimum,
		ExclusiveMinimum: v.ExclusiveMinimum,
		MaxLength:        v.MaxLength,
		MinLength:        v.MinLength,
		Pattern:          v.Pattern,
		MaxItems:         v.MaxItems,
		MinItems:         v.MinItems,
		UniqueItems:      v.UniqueItems,
		MultipleOf:       v.MultipleOf,
		Enum:             v.Enum,
	}
	return
}
