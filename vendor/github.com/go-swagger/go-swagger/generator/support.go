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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

// GenerateServer generates a server application
func GenerateServer(name string, modelNames, operationIDs []string, opts *GenOpts) error {
	generator, err := newAppGenerator(name, modelNames, operationIDs, opts)
	if err != nil {
		return err
	}
	return generator.Generate()
}

// GenerateSupport generates the supporting files for an API
func GenerateSupport(name string, modelNames, operationIDs []string, opts *GenOpts) error {
	generator, err := newAppGenerator(name, modelNames, operationIDs, opts)
	if err != nil {
		return err
	}
	return generator.GenerateSupport(nil)
}

func newAppGenerator(name string, modelNames, operationIDs []string, opts *GenOpts) (*appGenerator, error) {
	if opts == nil {
		return nil, errors.New("gen opts are required")
	}
	if err := opts.CheckOpts(); err != nil {
		return nil, err
	}

	templates.LoadDefaults()
	if opts.Template != "" {
		if err := templates.LoadContrib(opts.Template); err != nil {
			return nil, err
		}
	}

	templates.SetAllowOverride(opts.AllowTemplateOverride)

	if opts.TemplateDir != "" {
		if err := templates.LoadDir(opts.TemplateDir); err != nil {
			return nil, err
		}
	}

	// Load the spec
	var err error
	var specDoc *loads.Document

	opts.Spec, err = findSwaggerSpec(opts.Spec)
	if err != nil {
		return nil, err
	}

	if !filepath.IsAbs(opts.Spec) {
		cwd, _ := os.Getwd()
		opts.Spec = filepath.Join(cwd, opts.Spec)
	}

	if opts.PropertiesSpecOrder {
		opts.Spec = WithAutoXOrder(opts.Spec)
	}

	opts.Spec, specDoc, err = loadSpec(opts.Spec)
	if err != nil {
		return nil, err
	}

	specDoc, err = validateAndFlattenSpec(opts, specDoc)
	if err != nil {
		return nil, err
	}

	analyzed := analysis.New(specDoc.Spec())

	models, err := gatherModels(specDoc, modelNames)
	if err != nil {
		return nil, err
	}

	operations := gatherOperations(analyzed, operationIDs)
	if len(operations) == 0 {
		return nil, errors.New("no operations were selected")
	}

	defaultScheme := opts.DefaultScheme
	if defaultScheme == "" {
		defaultScheme = "http"
	}

	defaultProduces := opts.DefaultProduces
	if defaultProduces == "" {
		defaultProduces = runtime.JSONMime
	}

	defaultConsumes := opts.DefaultConsumes
	if defaultConsumes == "" {
		defaultConsumes = runtime.JSONMime
	}

	opts.Name = appNameOrDefault(specDoc, name, "swagger")
	apiPackage := opts.LanguageOpts.ManglePackagePath(opts.APIPackage, "api")
	return &appGenerator{
		Name:              opts.Name,
		Receiver:          "o",
		SpecDoc:           specDoc,
		Analyzed:          analyzed,
		Models:            models,
		Operations:        operations,
		Target:            opts.Target,
		DumpData:          opts.DumpData,
		Package:           opts.LanguageOpts.ManglePackageName(apiPackage, "api"),
		APIPackage:        apiPackage,
		ModelsPackage:     opts.LanguageOpts.ManglePackagePath(opts.ModelPackage, "definitions"),
		ServerPackage:     opts.LanguageOpts.ManglePackagePath(opts.ServerPackage, "server"),
		ClientPackage:     opts.LanguageOpts.ManglePackagePath(opts.ClientPackage, "client"),
		OperationsPackage: filepath.Join(opts.LanguageOpts.ManglePackagePath(opts.ServerPackage, "server"), apiPackage),
		Principal:         opts.Principal,
		DefaultScheme:     defaultScheme,
		DefaultProduces:   defaultProduces,
		DefaultConsumes:   defaultConsumes,
		GenOpts:           opts,
	}, nil
}

type appGenerator struct {
	Name              string
	Receiver          string
	SpecDoc           *loads.Document
	Analyzed          *analysis.Spec
	Package           string
	APIPackage        string
	ModelsPackage     string
	ServerPackage     string
	ClientPackage     string
	OperationsPackage string
	Principal         string
	Models            map[string]spec.Schema
	Operations        map[string]opRef
	Target            string
	DumpData          bool
	DefaultScheme     string
	DefaultProduces   string
	DefaultConsumes   string
	GenOpts           *GenOpts
}

func WithAutoXOrder(specPath string) string {
	lookFor := func(ele interface{}, key string) (yaml.MapSlice, bool) {
		if slice, ok := ele.(yaml.MapSlice); ok {
			for _, v := range slice {
				if v.Key == key {
					if slice, ok := v.Value.(yaml.MapSlice); ok {
						return slice, ok
					}
				}
			}
		}
		return nil, false
	}

	var addXOrder func(interface{})
	addXOrder = func(element interface{}) {
		if props, ok := lookFor(element, "properties"); ok {
			for i, prop := range props {
				if pSlice, ok := prop.Value.(yaml.MapSlice); ok {
					isObject := false
					xOrderIndex := -1 //Find if x-order already exists

					for i, v := range pSlice {
						if v.Key == "type" && v.Value == object {
							isObject = true
						}
						if v.Key == xOrder {
							xOrderIndex = i
							break
						}
					}

					if xOrderIndex > -1 { //Override existing x-order
						pSlice[xOrderIndex] = yaml.MapItem{Key: xOrder, Value: i}
					} else { // append new x-order
						pSlice = append(pSlice, yaml.MapItem{Key: xOrder, Value: i})
					}
					prop.Value = pSlice
					props[i] = prop

					if isObject {
						addXOrder(pSlice)
					}
				}
			}
		}
	}

	yamlDoc, err := swag.YAMLData(specPath)
	if err != nil {
		panic(err)
	}

	if defs, ok := lookFor(yamlDoc, "definitions"); ok {
		for _, def := range defs {
			addXOrder(def.Value)
		}
	}

	addXOrder(yamlDoc)

	out, err := yaml.Marshal(yamlDoc)
	if err != nil {
		panic(err)
	}

	tmpFile, err := ioutil.TempFile("", filepath.Base(specPath))
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(tmpFile.Name(), out, 0); err != nil {
		panic(err)
	}
	return tmpFile.Name()
}

// 1. Checks if the child path and parent path coincide.
// 2. If they do return child path  relative to parent path.
// 3. Everything else return false
func checkPrefixAndFetchRelativePath(childpath string, parentpath string) (bool, string) {
	// Windows (local) file systems - NTFS, as well as FAT and variants
	// are case insensitive.
	cp, pp := childpath, parentpath
	if goruntime.GOOS == "windows" {
		cp = strings.ToLower(cp)
		pp = strings.ToLower(pp)
	}

	if strings.HasPrefix(cp, pp) {
		pth, err := filepath.Rel(parentpath, childpath)
		if err != nil {
			log.Fatalln(err)
		}
		return true, pth
	}

	return false, ""

}

func (a *appGenerator) Generate() error {

	app, err := a.makeCodegenApp()
	if err != nil {
		return err
	}

	if a.DumpData {
		bb, err := json.MarshalIndent(app, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(bb))
		return nil
	}

	// NOTE: relative to previous implem with chan.
	// IPC removed concurrent execution because of the FuncMap that is being shared
	// templates are now lazy loaded so there is concurrent map access I can't guard
	if a.GenOpts.IncludeModel {
		log.Printf("rendering %d models", len(app.Models))
		for _, mod := range app.Models {
			modCopy := mod
			modCopy.IncludeValidator = true // a.GenOpts.IncludeValidator
			modCopy.IncludeModel = true
			if err := a.GenOpts.renderDefinition(&modCopy); err != nil {
				return err
			}
		}
	}

	if a.GenOpts.IncludeHandler {
		log.Printf("rendering %d operation groups (tags)", app.OperationGroups.Len())
		for _, opg := range app.OperationGroups {
			opgCopy := opg
			log.Printf("rendering %d operations for %s", opg.Operations.Len(), opg.Name)
			for _, op := range opgCopy.Operations {
				opCopy := op

				if err := a.GenOpts.renderOperation(&opCopy); err != nil {
					return err
				}
			}
			// Optional OperationGroups templates generation
			opGroup := opg
			opGroup.DefaultImports = app.DefaultImports
			if err := a.GenOpts.renderOperationGroup(&opGroup); err != nil {
				return fmt.Errorf("error while rendering operation group: %v", err)
			}
		}
	}

	if a.GenOpts.IncludeSupport {
		log.Printf("rendering support")
		if err := a.GenerateSupport(&app); err != nil {
			return err
		}
	}
	return nil
}

func (a *appGenerator) GenerateSupport(ap *GenApp) error {
	app := ap
	if ap == nil {
		ca, err := a.makeCodegenApp()
		if err != nil {
			return err
		}
		app = &ca
	}
	baseImport := a.GenOpts.LanguageOpts.baseImport(a.Target)
	importPath := path.Join(filepath.ToSlash(baseImport), a.GenOpts.LanguageOpts.ManglePackagePath(a.OperationsPackage, ""))
	app.DefaultImports = append(
		app.DefaultImports,
		path.Join(filepath.ToSlash(baseImport), a.GenOpts.LanguageOpts.ManglePackagePath(a.ServerPackage, "")),
		importPath,
	)

	return a.GenOpts.renderApplication(app)
}

var mediaTypeNames = map[*regexp.Regexp]string{
	regexp.MustCompile("application/.*json"):                "json",
	regexp.MustCompile("application/.*yaml"):                "yaml",
	regexp.MustCompile("application/.*protobuf"):            "protobuf",
	regexp.MustCompile("application/.*capnproto"):           "capnproto",
	regexp.MustCompile("application/.*thrift"):              "thrift",
	regexp.MustCompile("(?:application|text)/.*xml"):        "xml",
	regexp.MustCompile("text/.*markdown"):                   "markdown",
	regexp.MustCompile("text/.*html"):                       "html",
	regexp.MustCompile("text/.*csv"):                        "csv",
	regexp.MustCompile("text/.*tsv"):                        "tsv",
	regexp.MustCompile("text/.*javascript"):                 "js",
	regexp.MustCompile("text/.*css"):                        "css",
	regexp.MustCompile("text/.*plain"):                      "txt",
	regexp.MustCompile("application/.*octet-stream"):        "bin",
	regexp.MustCompile("application/.*tar"):                 "tar",
	regexp.MustCompile("application/.*gzip"):                "gzip",
	regexp.MustCompile("application/.*gz"):                  "gzip",
	regexp.MustCompile("application/.*raw-stream"):          "bin",
	regexp.MustCompile("application/x-www-form-urlencoded"): "urlform",
	regexp.MustCompile("multipart/form-data"):               "multipartform",
}

var knownProducers = map[string]string{
	"json":          "runtime.JSONProducer()",
	"yaml":          "yamlpc.YAMLProducer()",
	"xml":           "runtime.XMLProducer()",
	"txt":           "runtime.TextProducer()",
	"bin":           "runtime.ByteStreamProducer()",
	"urlform":       "runtime.DiscardProducer",
	"multipartform": "runtime.DiscardProducer",
}

var knownConsumers = map[string]string{
	"json":          "runtime.JSONConsumer()",
	"yaml":          "yamlpc.YAMLConsumer()",
	"xml":           "runtime.XMLConsumer()",
	"txt":           "runtime.TextConsumer()",
	"bin":           "runtime.ByteStreamConsumer()",
	"urlform":       "runtime.DiscardConsumer",
	"multipartform": "runtime.DiscardConsumer",
}

func getSerializer(sers []GenSerGroup, ext string) (*GenSerGroup, bool) {
	for i := range sers {
		s := &sers[i]
		if s.Name == ext {
			return s, true
		}
	}
	return nil, false
}

func mediaTypeName(tn string) (string, bool) {
	for k, v := range mediaTypeNames {
		if k.MatchString(tn) {
			return v, true
		}
	}
	return "", false
}

func (a *appGenerator) makeConsumes() (consumes GenSerGroups, consumesJSON bool) {
	reqCons := a.Analyzed.RequiredConsumes()
	sort.Strings(reqCons)
	for _, cons := range reqCons {
		cn, ok := mediaTypeName(cons)
		if !ok {
			nm := swag.ToJSONName(cons)
			ser := GenSerializer{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           nm,
				MediaType:      cons,
				Implementation: "",
			}

			consumes = append(consumes, GenSerGroup{
				AppName:        ser.AppName,
				ReceiverName:   ser.ReceiverName,
				Name:           ser.Name,
				MediaType:      cons,
				AllSerializers: []GenSerializer{ser},
				Implementation: ser.Implementation,
			})
			continue
		}
		nm := swag.ToJSONName(cn)
		if nm == "json" {
			consumesJSON = true
		}

		if ser, ok := getSerializer(consumes, cn); ok {
			ser.AllSerializers = append(ser.AllSerializers, GenSerializer{
				AppName:        ser.AppName,
				ReceiverName:   ser.ReceiverName,
				Name:           ser.Name,
				MediaType:      cons,
				Implementation: knownConsumers[nm],
			})
			sort.Sort(ser.AllSerializers)
			continue
		}

		ser := GenSerializer{
			AppName:        a.Name,
			ReceiverName:   a.Receiver,
			Name:           nm,
			MediaType:      cons,
			Implementation: knownConsumers[nm],
		}

		consumes = append(consumes, GenSerGroup{
			AppName:        ser.AppName,
			ReceiverName:   ser.ReceiverName,
			Name:           ser.Name,
			MediaType:      cons,
			AllSerializers: []GenSerializer{ser},
			Implementation: ser.Implementation,
		})
	}
	if len(consumes) == 0 {
		consumes = append(consumes, GenSerGroup{
			AppName:      a.Name,
			ReceiverName: a.Receiver,
			Name:         "json",
			MediaType:    runtime.JSONMime,
			AllSerializers: []GenSerializer{{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           "json",
				MediaType:      runtime.JSONMime,
				Implementation: knownConsumers["json"],
			}},
			Implementation: knownConsumers["json"],
		})
		consumesJSON = true
	}
	sort.Sort(consumes)
	return
}

func (a *appGenerator) makeProduces() (produces GenSerGroups, producesJSON bool) {
	reqProds := a.Analyzed.RequiredProduces()
	sort.Strings(reqProds)
	for _, prod := range reqProds {
		pn, ok := mediaTypeName(prod)
		if !ok {
			nm := swag.ToJSONName(prod)
			ser := GenSerializer{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           nm,
				MediaType:      prod,
				Implementation: "",
			}
			produces = append(produces, GenSerGroup{
				AppName:        ser.AppName,
				ReceiverName:   ser.ReceiverName,
				Name:           ser.Name,
				MediaType:      prod,
				Implementation: ser.Implementation,
				AllSerializers: []GenSerializer{ser},
			})
			continue
		}
		nm := swag.ToJSONName(pn)
		if nm == "json" {
			producesJSON = true
		}

		if ser, ok := getSerializer(produces, pn); ok {
			ser.AllSerializers = append(ser.AllSerializers, GenSerializer{
				AppName:        ser.AppName,
				ReceiverName:   ser.ReceiverName,
				Name:           ser.Name,
				MediaType:      prod,
				Implementation: knownProducers[nm],
			})
			sort.Sort(ser.AllSerializers)
			continue
		}

		ser := GenSerializer{
			AppName:        a.Name,
			ReceiverName:   a.Receiver,
			Name:           nm,
			MediaType:      prod,
			Implementation: knownProducers[nm],
		}
		produces = append(produces, GenSerGroup{
			AppName:        ser.AppName,
			ReceiverName:   ser.ReceiverName,
			Name:           ser.Name,
			MediaType:      prod,
			Implementation: ser.Implementation,
			AllSerializers: []GenSerializer{ser},
		})
	}
	if len(produces) == 0 {
		produces = append(produces, GenSerGroup{
			AppName:      a.Name,
			ReceiverName: a.Receiver,
			Name:         "json",
			MediaType:    runtime.JSONMime,
			AllSerializers: []GenSerializer{{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           "json",
				MediaType:      runtime.JSONMime,
				Implementation: knownProducers["json"],
			}},
			Implementation: knownProducers["json"],
		})
		producesJSON = true
	}
	sort.Sort(produces)
	return
}

func (a *appGenerator) makeSecuritySchemes() GenSecuritySchemes {
	if a.Principal == "" {
		a.Principal = "interface{}"
	}
	requiredSecuritySchemes := make(map[string]spec.SecurityScheme, len(a.Analyzed.RequiredSecuritySchemes()))
	for _, scheme := range a.Analyzed.RequiredSecuritySchemes() {
		if req, ok := a.SpecDoc.Spec().SecurityDefinitions[scheme]; ok && req != nil {
			requiredSecuritySchemes[scheme] = *req
		}
	}
	return gatherSecuritySchemes(requiredSecuritySchemes, a.Name, a.Principal, a.Receiver)
}

func (a *appGenerator) makeCodegenApp() (GenApp, error) {
	log.Println("building a plan for generation")
	sw := a.SpecDoc.Spec()
	receiver := a.Receiver

	var defaultImports []string

	jsonb, _ := json.MarshalIndent(a.SpecDoc.OrigSpec(), "", "  ")
	flatjsonb, _ := json.MarshalIndent(a.SpecDoc.Spec(), "", "  ")

	consumes, _ := a.makeConsumes()
	produces, _ := a.makeProduces()
	sort.Sort(consumes)
	sort.Sort(produces)
	security := a.makeSecuritySchemes()
	baseImport := a.GenOpts.LanguageOpts.baseImport(a.Target)
	var imports = make(map[string]string)

	var genMods GenDefinitions
	importPath := a.GenOpts.ExistingModels
	if a.GenOpts.ExistingModels == "" {
		imports[a.GenOpts.LanguageOpts.ManglePackageName(a.ModelsPackage, "models")] = path.Join(
			filepath.ToSlash(baseImport),
			a.GenOpts.LanguageOpts.ManglePackagePath(a.GenOpts.ModelPackage, "models"))
	}
	if importPath != "" {
		defaultImports = append(defaultImports, importPath)
	}

	log.Println("planning definitions")
	for mn, m := range a.Models {
		mod, err := makeGenDefinition(
			mn,
			a.ModelsPackage,
			m,
			a.SpecDoc,
			a.GenOpts,
		)
		if err != nil {
			return GenApp{}, fmt.Errorf("error in model %s while planning definitions: %v", mn, err)
		}
		if mod != nil {
			if !mod.External {
				genMods = append(genMods, *mod)
			}

			// Copy model imports to operation imports
			for alias, pkg := range mod.Imports {
				target := a.GenOpts.LanguageOpts.ManglePackageName(alias, "")
				imports[target] = pkg
			}
		}
	}
	sort.Sort(genMods)

	log.Println("planning operations")
	tns := make(map[string]struct{})
	var genOps GenOperations
	for on, opp := range a.Operations {
		o := opp.Op
		o.Tags = pruneEmpty(o.Tags)
		o.ID = on

		var bldr codeGenOpBuilder
		bldr.ModelsPackage = a.ModelsPackage
		bldr.Principal = a.Principal
		bldr.Target = a.Target
		bldr.DefaultImports = defaultImports
		bldr.Imports = imports
		bldr.DefaultScheme = a.DefaultScheme
		bldr.Doc = a.SpecDoc
		bldr.Analyzed = a.Analyzed
		bldr.BasePath = a.SpecDoc.BasePath()
		bldr.GenOpts = a.GenOpts

		// TODO: change operation name to something safe
		bldr.Name = on
		bldr.Operation = *o
		bldr.Method = opp.Method
		bldr.Path = opp.Path
		bldr.Authed = len(a.Analyzed.SecurityRequirementsFor(o)) > 0
		bldr.Security = a.Analyzed.SecurityRequirementsFor(o)
		bldr.SecurityDefinitions = a.Analyzed.SecurityDefinitionsFor(o)
		bldr.RootAPIPackage = a.GenOpts.LanguageOpts.ManglePackageName(a.ServerPackage, "server")
		bldr.IncludeValidator = true

		bldr.APIPackage = a.APIPackage
		st := o.Tags
		if a.GenOpts != nil {
			st = a.GenOpts.Tags
		}
		intersected := intersectTags(o.Tags, st)
		if len(st) > 0 && len(intersected) == 0 {
			continue
		}

		if len(intersected) > 0 {
			tag := intersected[0]
			bldr.APIPackage = a.GenOpts.LanguageOpts.ManglePackagePath(tag, a.APIPackage)
			for _, t := range intersected {
				tns[t] = struct{}{}
			}
		}
		op, err := bldr.MakeOperation()
		if err != nil {
			return GenApp{}, err
		}
		op.ReceiverName = receiver
		op.Tags = intersected
		genOps = append(genOps, op)

	}
	for k := range tns {
		importPath := filepath.ToSlash(
			path.Join(
				filepath.ToSlash(baseImport),
				a.GenOpts.LanguageOpts.ManglePackagePath(a.OperationsPackage, ""),
				swag.ToFileName(k)))
		defaultImports = append(defaultImports, importPath)
	}
	sort.Sort(genOps)

	log.Println("grouping operations into packages")
	opsGroupedByPackage := make(map[string]GenOperations)
	for _, operation := range genOps {
		if operation.Package == "" {
			operation.Package = a.Package
		}
		opsGroupedByPackage[operation.Package] = append(opsGroupedByPackage[operation.Package], operation)
	}

	var opGroups GenOperationGroups
	for k, v := range opsGroupedByPackage {
		sort.Sort(v)
		// trim duplicate extra schemas within the same package
		vv := make(GenOperations, 0, len(v))
		seenExtraSchema := make(map[string]bool)
		for _, op := range v {
			uniqueExtraSchemas := make(GenSchemaList, 0, len(op.ExtraSchemas))
			for _, xs := range op.ExtraSchemas {
				if _, alreadyThere := seenExtraSchema[xs.Name]; !alreadyThere {
					seenExtraSchema[xs.Name] = true
					uniqueExtraSchemas = append(uniqueExtraSchemas, xs)
				}
			}
			op.ExtraSchemas = uniqueExtraSchemas
			vv = append(vv, op)
		}

		opGroup := GenOperationGroup{
			GenCommon: GenCommon{
				Copyright:        a.GenOpts.Copyright,
				TargetImportPath: filepath.ToSlash(baseImport),
			},
			Name:           k,
			Operations:     vv,
			DefaultImports: defaultImports,
			Imports:        imports,
			RootPackage:    a.APIPackage,
			GenOpts:        a.GenOpts,
		}
		opGroups = append(opGroups, opGroup)
		var importPath string
		if k == a.APIPackage {
			importPath = path.Join(filepath.ToSlash(baseImport), a.GenOpts.LanguageOpts.ManglePackagePath(a.OperationsPackage, ""))
		} else {
			importPath = path.Join(filepath.ToSlash(baseImport), a.GenOpts.LanguageOpts.ManglePackagePath(a.OperationsPackage, ""), k)
		}
		defaultImports = append(defaultImports, importPath)
	}
	sort.Sort(opGroups)

	log.Println("planning meta data and facades")

	var collectedSchemes []string
	var extraSchemes []string
	for _, op := range genOps {
		collectedSchemes = concatUnique(collectedSchemes, op.Schemes)
		extraSchemes = concatUnique(extraSchemes, op.ExtraSchemes)
	}
	sort.Strings(collectedSchemes)
	sort.Strings(extraSchemes)

	host := "localhost"
	if sw.Host != "" {
		host = sw.Host
	}

	basePath := "/"
	if sw.BasePath != "" {
		basePath = sw.BasePath
	}

	return GenApp{
		GenCommon: GenCommon{
			Copyright:        a.GenOpts.Copyright,
			TargetImportPath: filepath.ToSlash(baseImport),
		},
		APIPackage:          a.GenOpts.LanguageOpts.ManglePackageName(a.ServerPackage, "server"),
		Package:             a.Package,
		ReceiverName:        receiver,
		Name:                a.Name,
		Host:                host,
		BasePath:            basePath,
		Schemes:             schemeOrDefault(collectedSchemes, a.DefaultScheme),
		ExtraSchemes:        extraSchemes,
		ExternalDocs:        sw.ExternalDocs,
		Info:                sw.Info,
		Consumes:            consumes,
		Produces:            produces,
		DefaultConsumes:     a.DefaultConsumes,
		DefaultProduces:     a.DefaultProduces,
		DefaultImports:      defaultImports,
		Imports:             imports,
		SecurityDefinitions: security,
		Models:              genMods,
		Operations:          genOps,
		OperationGroups:     opGroups,
		Principal:           a.Principal,
		SwaggerJSON:         generateReadableSpec(jsonb),
		FlatSwaggerJSON:     generateReadableSpec(flatjsonb),
		ExcludeSpec:         a.GenOpts != nil && a.GenOpts.ExcludeSpec,
		GenOpts:             a.GenOpts,
	}, nil
}

// generateReadableSpec makes swagger json spec as a string instead of bytes
// the only character that needs to be escaped is '`' symbol, since it cannot be escaped in the GO string
// that is quoted as `string data`. The function doesn't care about the beginning or the ending of the
// string it escapes since all data that needs to be escaped is always in the middle of the swagger spec.
func generateReadableSpec(spec []byte) string {
	buf := &bytes.Buffer{}
	for _, b := range string(spec) {
		if b == '`' {
			buf.WriteString("`+\"`\"+`")
		} else {
			buf.WriteRune(b)
		}
	}
	return buf.String()
}
