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

package generate

import (
	"errors"
	"log"

	"github.com/go-swagger/go-swagger/generator"
)

type modelOptions struct {
	ModelPackage               string   `long:"model-package" short:"m" description:"the package to save the models" default:"models"`
	Models                     []string `long:"model" short:"M" description:"specify a model to include in generation, repeat for multiple (defaults to all)"`
	ExistingModels             string   `long:"existing-models" description:"use pre-generated models e.g. github.com/foobar/model"`
	StrictAdditionalProperties bool     `long:"strict-additional-properties" description:"disallow extra properties when additionalProperties is set to false"`
	KeepSpecOrder              bool     `long:"keep-spec-order" description:"keep schema properties order identical to spec file"`
	AllDefinitions             bool     `long:"all-definitions" description:"generate all model definitions regardless of usage in operations" hidden:"deprecated"`
	StructTags                 []string `long:"struct-tags" description:"the struct tags to generate, repeat for multiple (defaults to json)"`
}

func (mo modelOptions) apply(opts *generator.GenOpts) {
	opts.ModelPackage = mo.ModelPackage
	opts.Models = mo.Models
	opts.ExistingModels = mo.ExistingModels
	opts.StrictAdditionalProperties = mo.StrictAdditionalProperties
	opts.PropertiesSpecOrder = mo.KeepSpecOrder
	opts.IgnoreOperations = mo.AllDefinitions
	opts.StructTags = mo.StructTags
}

// WithModels adds the model options group.
//
// This group is available to all commands that need some model generation.
type WithModels struct {
	Models modelOptions `group:"Options for model generation"`
}

// Model the generate model file command.
//
// Define the options that are specific to the "swagger generate model" command.
type Model struct {
	WithShared
	WithModels

	NoStruct              bool     `long:"skip-struct" description:"when present will not generate the model struct" hidden:"deprecated"`
	Name                  []string `long:"name" short:"n" description:"the model to generate, repeat for multiple (defaults to all). Same as --models"`
	AcceptDefinitionsOnly bool     `long:"accept-definitions-only" description:"accepts a partial swagger spec wih only the definitions key"`
}

func (m Model) apply(opts *generator.GenOpts) {
	m.Shared.apply(opts)
	m.Models.apply(opts)

	opts.IncludeModel = !m.NoStruct
	opts.IncludeValidator = !m.NoStruct
	opts.AcceptDefinitionsOnly = m.AcceptDefinitionsOnly
}

func (m Model) log(rp string) {
	log.Printf(`Generation completed!

For this generation to compile you need to have some packages in your GOPATH:

	* github.com/go-openapi/validate
	* github.com/go-openapi/strfmt

You can get these now with: go get -u -f %s/...
`, rp)
}

func (m *Model) generate(opts *generator.GenOpts) error {
	return generator.GenerateModels(append(m.Name, m.Models.Models...), opts)
}

// Execute generates a model file
func (m *Model) Execute(args []string) error {

	if m.Shared.DumpData && len(append(m.Name, m.Models.Models...)) > 1 {
		return errors.New("only 1 model at a time is supported for dumping data")
	}

	if m.Models.ExistingModels != "" {
		log.Println("warning: Ignoring existing-models flag when generating models.")
	}
	return createSwagger(m)
}
