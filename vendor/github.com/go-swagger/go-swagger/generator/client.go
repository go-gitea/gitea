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
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"
)

// GenerateClient generates a client library for a swagger spec document.
func GenerateClient(name string, modelNames, operationIDs []string, opts *GenOpts) error {
	templates.LoadDefaults()
	if opts == nil {
		return errors.New("gen opts are required")
	}

	if opts.Template != "" {
		if err := templates.LoadContrib(opts.Template); err != nil {
			return err
		}
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
	_, specDoc, err := loadSpec(opts.Spec)
	if err != nil {
		return err
	}

	// Validate and Expand. specDoc is in/out param.
	specDoc, err = validateAndFlattenSpec(opts, specDoc)
	if err != nil {
		return err
	}

	analyzed := analysis.New(specDoc.Spec())

	models, err := gatherModels(specDoc, modelNames)
	if err != nil {
		return err
	}

	operations := gatherOperations(analyzed, operationIDs)

	if len(operations) == 0 {
		return errors.New("no operations were selected")
	}

	defaultScheme := opts.DefaultScheme
	if defaultScheme == "" {
		defaultScheme = sHTTP
	}

	defaultConsumes := opts.DefaultConsumes
	if defaultConsumes == "" {
		defaultConsumes = runtime.JSONMime
	}

	defaultProduces := opts.DefaultProduces
	if defaultProduces == "" {
		defaultProduces = runtime.JSONMime
	}

	generator := appGenerator{
		Name:              appNameOrDefault(specDoc, name, "rest"),
		SpecDoc:           specDoc,
		Analyzed:          analyzed,
		Models:            models,
		Operations:        operations,
		Target:            opts.Target,
		DumpData:          opts.DumpData,
		Package:           opts.LanguageOpts.ManglePackageName(opts.ClientPackage, "client"),
		APIPackage:        opts.LanguageOpts.ManglePackagePath(opts.APIPackage, "api"),
		ModelsPackage:     opts.LanguageOpts.ManglePackagePath(opts.ModelPackage, "definitions"),
		ServerPackage:     opts.LanguageOpts.ManglePackagePath(opts.ServerPackage, "server"),
		ClientPackage:     opts.LanguageOpts.ManglePackagePath(opts.ClientPackage, "client"),
		OperationsPackage: opts.LanguageOpts.ManglePackagePath(opts.ClientPackage, "client"),
		Principal:         opts.Principal,
		DefaultScheme:     defaultScheme,
		DefaultProduces:   defaultProduces,
		DefaultConsumes:   defaultConsumes,
		GenOpts:           opts,
	}
	generator.Receiver = "o"
	return (&clientGenerator{generator}).Generate()
}

type clientGenerator struct {
	appGenerator
}

func (c *clientGenerator) Generate() error {
	app, err := c.makeCodegenApp()
	if app.Name == "" {
		app.Name = "APIClient"
	}
	baseImport := c.GenOpts.LanguageOpts.baseImport(c.Target)

	if c.GenOpts.ExistingModels == "" {
		if app.Imports == nil {
			app.Imports = make(map[string]string)
		}
		pkgAlias := c.GenOpts.LanguageOpts.ManglePackageName(c.ModelsPackage, "models")
		app.Imports[pkgAlias] = path.Join(
			filepath.ToSlash(baseImport),
			c.GenOpts.LanguageOpts.ManglePackagePath(c.GenOpts.ModelPackage, "models"))
	} else {
		app.DefaultImports = append(app.DefaultImports, c.GenOpts.LanguageOpts.ManglePackagePath(c.GenOpts.ExistingModels, ""))
	}
	if err != nil {
		return err
	}

	if c.DumpData {
		bb, _ := json.MarshalIndent(swag.ToDynamicJSON(app), "", "  ")
		fmt.Fprintln(os.Stdout, string(bb))
		return nil
	}

	if c.GenOpts.IncludeModel {
		for _, mod := range app.Models {
			modCopy := mod
			modCopy.IncludeValidator = true
			if !mod.IsStream {
				if err := c.GenOpts.renderDefinition(&modCopy); err != nil {
					return err
				}
			}
		}
	}

	if c.GenOpts.IncludeHandler {
		sort.Sort(app.OperationGroups)
		for i := range app.OperationGroups {
			opGroup := app.OperationGroups[i]
			opGroup.DefaultImports = app.DefaultImports
			opGroup.RootPackage = c.ClientPackage
			opGroup.GenOpts = c.GenOpts
			app.OperationGroups[i] = opGroup
			sort.Sort(opGroup.Operations)
			for _, op := range opGroup.Operations {
				opCopy := op
				if opCopy.Package == "" {
					opCopy.Package = c.Package
				}
				if err := c.GenOpts.renderOperation(&opCopy); err != nil {
					return err
				}
			}
			app.DefaultImports = append(app.DefaultImports,
				path.Join(
					filepath.ToSlash(baseImport),
					c.GenOpts.LanguageOpts.ManglePackagePath(c.ClientPackage, "client"),
					opGroup.Name))

			if err := c.GenOpts.renderOperationGroup(&opGroup); err != nil {
				return err
			}
		}
	}

	if c.GenOpts.IncludeSupport {
		if err := c.GenOpts.renderApplication(&app); err != nil {
			return err
		}
	}

	return nil
}
