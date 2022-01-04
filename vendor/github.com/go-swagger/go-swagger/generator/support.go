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
	"log"
	"path"
	"path/filepath"
	"sort"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
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

// GenerateMarkdown documentation for a swagger specification
func GenerateMarkdown(output string, modelNames, operationIDs []string, opts *GenOpts) error {
	if output == "." || output == "" {
		output = "markdown.md"
	}

	if err := opts.EnsureDefaults(); err != nil {
		return err
	}
	MarkdownSectionOpts(opts, output)

	generator, err := newAppGenerator("", modelNames, operationIDs, opts)
	if err != nil {
		return err
	}

	return generator.GenerateMarkdown()
}

func newAppGenerator(name string, modelNames, operationIDs []string, opts *GenOpts) (*appGenerator, error) {
	if err := opts.CheckOpts(); err != nil {
		return nil, err
	}

	if err := opts.setTemplates(); err != nil {
		return nil, err
	}

	specDoc, analyzed, err := opts.analyzeSpec()
	if err != nil {
		return nil, err
	}

	models, err := gatherModels(specDoc, modelNames)
	if err != nil {
		return nil, err
	}

	operations := gatherOperations(analyzed, operationIDs)

	if len(operations) == 0 && !opts.IgnoreOperations {
		return nil, errors.New("no operations were selected")
	}

	opts.Name = appNameOrDefault(specDoc, name, defaultServerName)
	if opts.IncludeMain && opts.MainPackage == "" {
		// default target for the generated main
		opts.MainPackage = swag.ToCommandName(mainNameOrDefault(specDoc, name, defaultServerName) + "-server")
	}

	apiPackage := opts.LanguageOpts.ManglePackagePath(opts.APIPackage, defaultOperationsTarget)
	return &appGenerator{
		Name:              opts.Name,
		Receiver:          "o",
		SpecDoc:           specDoc,
		Analyzed:          analyzed,
		Models:            models,
		Operations:        operations,
		Target:            opts.Target,
		DumpData:          opts.DumpData,
		Package:           opts.LanguageOpts.ManglePackageName(apiPackage, defaultOperationsTarget),
		APIPackage:        apiPackage,
		ModelsPackage:     opts.LanguageOpts.ManglePackagePath(opts.ModelPackage, defaultModelsTarget),
		ServerPackage:     opts.LanguageOpts.ManglePackagePath(opts.ServerPackage, defaultServerTarget),
		ClientPackage:     opts.LanguageOpts.ManglePackagePath(opts.ClientPackage, defaultClientTarget),
		OperationsPackage: filepath.Join(opts.LanguageOpts.ManglePackagePath(opts.ServerPackage, defaultServerTarget), apiPackage),
		Principal:         opts.PrincipalAlias(),
		DefaultScheme:     opts.DefaultScheme,
		DefaultProduces:   opts.DefaultProduces,
		DefaultConsumes:   opts.DefaultConsumes,
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
	MainPackage       string
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

func (a *appGenerator) Generate() error {
	app, err := a.makeCodegenApp()
	if err != nil {
		return err
	}

	if a.DumpData {
		return dumpData(app)
	}

	// NOTE: relative to previous implem with chan.
	// IPC removed concurrent execution because of the FuncMap that is being shared
	// templates are now lazy loaded so there is concurrent map access I can't guard
	if a.GenOpts.IncludeModel {
		log.Printf("rendering %d models", len(app.Models))
		for _, md := range app.Models {
			mod := md
			mod.IncludeModel = true
			mod.IncludeValidator = a.GenOpts.IncludeValidator
			if err := a.GenOpts.renderDefinition(&mod); err != nil {
				return err
			}
		}
	}

	if a.GenOpts.IncludeHandler {
		log.Printf("rendering %d operation groups (tags)", app.OperationGroups.Len())
		for _, g := range app.OperationGroups {
			opg := g
			log.Printf("rendering %d operations for %s", opg.Operations.Len(), opg.Name)
			for _, p := range opg.Operations {
				op := p
				if err := a.GenOpts.renderOperation(&op); err != nil {
					return err
				}
			}
			// optional OperationGroups templates generation
			if err := a.GenOpts.renderOperationGroup(&opg); err != nil {
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
		// allows for calling GenerateSupport standalone
		ca, err := a.makeCodegenApp()
		if err != nil {
			return err
		}
		app = &ca
	}

	baseImport := a.GenOpts.LanguageOpts.baseImport(a.Target)
	serverPath := path.Join(baseImport,
		a.GenOpts.LanguageOpts.ManglePackagePath(a.ServerPackage, defaultServerTarget))

	pkgAlias := deconflictPkg(importAlias(serverPath), renameServerPackage)
	app.DefaultImports[pkgAlias] = serverPath
	app.ServerPackageAlias = pkgAlias

	// add client import for cli generation
	clientPath := path.Join(baseImport,
		a.GenOpts.LanguageOpts.ManglePackagePath(a.ClientPackage, defaultClientTarget))
	clientPkgAlias := importAlias(clientPath)
	app.DefaultImports[clientPkgAlias] = clientPath

	return a.GenOpts.renderApplication(app)
}

func (a *appGenerator) GenerateMarkdown() error {
	app, err := a.makeCodegenApp()
	if err != nil {
		return err
	}

	return a.GenOpts.renderApplication(&app)
}

func (a *appGenerator) makeSecuritySchemes() GenSecuritySchemes {
	requiredSecuritySchemes := make(map[string]spec.SecurityScheme, len(a.Analyzed.RequiredSecuritySchemes()))
	for _, scheme := range a.Analyzed.RequiredSecuritySchemes() {
		if req, ok := a.SpecDoc.Spec().SecurityDefinitions[scheme]; ok && req != nil {
			requiredSecuritySchemes[scheme] = *req
		}
	}
	return gatherSecuritySchemes(requiredSecuritySchemes, a.Name, a.Principal, a.Receiver, a.GenOpts.PrincipalIsNullable())
}

func (a *appGenerator) makeCodegenApp() (GenApp, error) {
	log.Println("building a plan for generation")

	sw := a.SpecDoc.Spec()
	receiver := a.Receiver

	consumes, _ := a.makeConsumes()
	produces, _ := a.makeProduces()
	security := a.makeSecuritySchemes()

	log.Println("generation target", a.Target)

	baseImport := a.GenOpts.LanguageOpts.baseImport(a.Target)
	defaultImports := a.GenOpts.defaultImports()

	imports := make(map[string]string, 50)
	alias := deconflictPkg(a.GenOpts.LanguageOpts.ManglePackageName(a.OperationsPackage, defaultOperationsTarget), renameAPIPackage)
	imports[alias] = path.Join(
		baseImport,
		a.GenOpts.LanguageOpts.ManglePackagePath(a.OperationsPackage, defaultOperationsTarget))

	implAlias := ""
	if a.GenOpts.ImplementationPackage != "" {
		implAlias = deconflictPkg(a.GenOpts.LanguageOpts.ManglePackageName(a.GenOpts.ImplementationPackage, defaultImplementationTarget), renameImplementationPackage)
		imports[implAlias] = a.GenOpts.ImplementationPackage
	}

	log.Printf("planning definitions (found: %d)", len(a.Models))

	genModels := make(GenDefinitions, 0, len(a.Models))
	for mn, m := range a.Models {
		model, err := makeGenDefinition(
			mn,
			a.ModelsPackage,
			m,
			a.SpecDoc,
			a.GenOpts,
		)
		if err != nil {
			return GenApp{}, fmt.Errorf("error in model %s while planning definitions: %v", mn, err)
		}
		if model != nil {
			if !model.External {
				genModels = append(genModels, *model)
			}

			// Copy model imports to operation imports
			// TODO(fredbi): mangle model pkg aliases
			for alias, pkg := range model.Imports {
				target := a.GenOpts.LanguageOpts.ManglePackageName(alias, "")
				imports[target] = pkg
			}
		}
	}
	sort.Sort(genModels)

	log.Printf("planning operations (found: %d)", len(a.Operations))

	genOps := make(GenOperations, 0, len(a.Operations))
	for operationName, opp := range a.Operations {
		o := opp.Op
		o.ID = operationName

		bldr := codeGenOpBuilder{
			ModelsPackage:    a.ModelsPackage,
			Principal:        a.GenOpts.PrincipalAlias(),
			Target:           a.Target,
			DefaultImports:   defaultImports,
			Imports:          imports,
			DefaultScheme:    a.DefaultScheme,
			Doc:              a.SpecDoc,
			Analyzed:         a.Analyzed,
			BasePath:         a.SpecDoc.BasePath(),
			GenOpts:          a.GenOpts,
			Name:             operationName,
			Operation:        *o,
			Method:           opp.Method,
			Path:             opp.Path,
			IncludeValidator: a.GenOpts.IncludeValidator,
			APIPackage:       a.APIPackage, // defaults to main operations package
			DefaultProduces:  a.DefaultProduces,
			DefaultConsumes:  a.DefaultConsumes,
		}

		tag, tags, ok := bldr.analyzeTags()
		if !ok {
			continue // operation filtered according to CLI params
		}

		bldr.Authed = len(a.Analyzed.SecurityRequirementsFor(o)) > 0
		bldr.Security = a.Analyzed.SecurityRequirementsFor(o)
		bldr.SecurityDefinitions = a.Analyzed.SecurityDefinitionsFor(o)
		bldr.RootAPIPackage = a.GenOpts.LanguageOpts.ManglePackageName(a.ServerPackage, defaultServerTarget)

		st := o.Tags
		if a.GenOpts != nil {
			st = a.GenOpts.Tags
		}
		intersected := intersectTags(o.Tags, st)
		if len(st) > 0 && len(intersected) == 0 {
			continue
		}

		op, err := bldr.MakeOperation()
		if err != nil {
			return GenApp{}, err
		}

		op.ReceiverName = receiver
		op.Tags = tags // ordered tags for this operation, possibly filtered by CLI params
		genOps = append(genOps, op)

		if !a.GenOpts.SkipTagPackages && tag != "" {
			importPath := filepath.ToSlash(
				path.Join(
					baseImport,
					a.GenOpts.LanguageOpts.ManglePackagePath(a.OperationsPackage, defaultOperationsTarget),
					a.GenOpts.LanguageOpts.ManglePackageName(bldr.APIPackage, defaultOperationsTarget),
				))
			defaultImports[bldr.APIPackageAlias] = importPath
		}
	}
	sort.Sort(genOps)

	opsGroupedByPackage := make(map[string]GenOperations, len(genOps))
	for _, operation := range genOps {
		opsGroupedByPackage[operation.PackageAlias] = append(opsGroupedByPackage[operation.PackageAlias], operation)
	}

	log.Printf("grouping operations into packages (packages: %d)", len(opsGroupedByPackage))

	opGroups := make(GenOperationGroups, 0, len(opsGroupedByPackage))
	for k, v := range opsGroupedByPackage {
		log.Printf("operations for package packages %q (found: %d)", k, len(v))
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
		var pkg string
		if len(vv) > 0 {
			pkg = vv[0].Package
		} else {
			pkg = k
		}

		opGroup := GenOperationGroup{
			GenCommon: GenCommon{
				Copyright:        a.GenOpts.Copyright,
				TargetImportPath: baseImport,
			},
			Name:           pkg,
			PackageAlias:   k,
			Operations:     vv,
			DefaultImports: defaultImports,
			Imports:        imports,
			RootPackage:    a.APIPackage,
			GenOpts:        a.GenOpts,
		}
		opGroups = append(opGroups, opGroup)
	}
	sort.Sort(opGroups)

	log.Println("planning meta data and facades")

	var collectedSchemes, extraSchemes []string
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

	jsonb, _ := json.MarshalIndent(a.SpecDoc.OrigSpec(), "", "  ")
	flatjsonb, _ := json.MarshalIndent(a.SpecDoc.Spec(), "", "  ")

	return GenApp{
		GenCommon: GenCommon{
			Copyright:        a.GenOpts.Copyright,
			TargetImportPath: baseImport,
		},
		APIPackage:                 a.GenOpts.LanguageOpts.ManglePackageName(a.ServerPackage, defaultServerTarget),
		APIPackageAlias:            alias,
		ImplementationPackageAlias: implAlias,
		Package:                    a.Package,
		ReceiverName:               receiver,
		Name:                       a.Name,
		Host:                       host,
		BasePath:                   basePath,
		Schemes:                    schemeOrDefault(collectedSchemes, a.DefaultScheme),
		ExtraSchemes:               extraSchemes,
		ExternalDocs:               trimExternalDoc(sw.ExternalDocs),
		Tags:                       trimTags(sw.Tags),
		Info:                       trimInfo(sw.Info),
		Consumes:                   consumes,
		Produces:                   produces,
		DefaultConsumes:            a.DefaultConsumes,
		DefaultProduces:            a.DefaultProduces,
		DefaultImports:             defaultImports,
		Imports:                    imports,
		SecurityDefinitions:        security,
		SecurityRequirements:       securityRequirements(a.SpecDoc.Spec().Security), // top level securityRequirements
		Models:                     genModels,
		Operations:                 genOps,
		OperationGroups:            opGroups,
		Principal:                  a.GenOpts.PrincipalAlias(),
		SwaggerJSON:                generateReadableSpec(jsonb),
		FlatSwaggerJSON:            generateReadableSpec(flatjsonb),
		ExcludeSpec:                a.GenOpts.ExcludeSpec,
		GenOpts:                    a.GenOpts,

		PrincipalIsNullable: a.GenOpts.PrincipalIsNullable(),
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

func trimExternalDoc(in *spec.ExternalDocumentation) *spec.ExternalDocumentation {
	if in == nil {
		return nil
	}

	return &spec.ExternalDocumentation{
		URL:         in.URL,
		Description: trimBOM(in.Description),
	}
}

func trimInfo(in *spec.Info) *spec.Info {
	if in == nil {
		return nil
	}

	return &spec.Info{
		InfoProps: spec.InfoProps{
			Contact:        in.Contact,
			Title:          trimBOM(in.Title),
			Description:    trimBOM(in.Description),
			TermsOfService: trimBOM(in.TermsOfService),
			License:        in.License,
			Version:        in.Version,
		},
		VendorExtensible: in.VendorExtensible,
	}
}

func trimTags(in []spec.Tag) []spec.Tag {
	if in == nil {
		return nil
	}

	tags := make([]spec.Tag, 0, len(in))

	for _, tag := range in {
		tags = append(tags, spec.Tag{
			TagProps: spec.TagProps{
				Name:         tag.Name,
				Description:  trimBOM(tag.Description),
				ExternalDocs: trimExternalDoc(tag.ExternalDocs),
			},
		})
	}

	return tags
}
