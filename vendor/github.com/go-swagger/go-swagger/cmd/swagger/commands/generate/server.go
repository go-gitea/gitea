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
	"log"
	"strings"

	"github.com/go-swagger/go-swagger/generator"
)

// Server the command to generate an entire server application
type Server struct {
	shared
	Name                       string   `long:"name" short:"A" description:"the name of the application, defaults to a mangled value of info.title"`
	Operations                 []string `long:"operation" short:"O" description:"specify an operation to include, repeat for multiple"`
	Tags                       []string `long:"tags" description:"the tags to include, if not specified defaults to all"`
	Principal                  string   `long:"principal" short:"P" description:"the model to use for the security principal"`
	DefaultScheme              string   `long:"default-scheme" description:"the default scheme for this API" default:"http"`
	Models                     []string `long:"model" short:"M" description:"specify a model to include, repeat for multiple"`
	SkipModels                 bool     `long:"skip-models" description:"no models will be generated when this flag is specified"`
	SkipOperations             bool     `long:"skip-operations" description:"no operations will be generated when this flag is specified"`
	SkipSupport                bool     `long:"skip-support" description:"no supporting files will be generated when this flag is specified"`
	ExcludeMain                bool     `long:"exclude-main" description:"exclude main function, so just generate the library"`
	ExcludeSpec                bool     `long:"exclude-spec" description:"don't embed the swagger specification"`
	WithContext                bool     `long:"with-context" description:"handlers get a context as first arg (deprecated)"`
	DumpData                   bool     `long:"dump-data" description:"when present dumps the json for the template generator instead of generating files"`
	FlagStrategy               string   `long:"flag-strategy" description:"the strategy to provide flags for the server" default:"go-flags" choice:"go-flags" choice:"pflag" choice:"flag"`
	CompatibilityMode          string   `long:"compatibility-mode" description:"the compatibility mode for the tls server" default:"modern" choice:"modern" choice:"intermediate"`
	SkipValidation             bool     `long:"skip-validation" description:"skips validation of spec prior to generation"`
	RegenerateConfigureAPI     bool     `long:"regenerate-configureapi" description:"Force regeneration of configureapi.go"`
	KeepSpecOrder              bool     `long:"keep-spec-order" description:"Keep schema properties order identical to spec file"`
	StrictAdditionalProperties bool     `long:"strict-additional-properties" description:"disallow extra properties when additionalProperties is set to false"`
}

func (s *Server) getOpts() (*generator.GenOpts, error) {
	// warning: deprecation
	if s.WithContext {
		log.Printf("warning: deprecated option --with-context is ignored")
	}

	return &generator.GenOpts{
		Spec:                       string(s.Spec),
		Target:                     string(s.Target),
		APIPackage:                 s.APIPackage,
		ModelPackage:               s.ModelPackage,
		ServerPackage:              s.ServerPackage,
		ClientPackage:              s.ClientPackage,
		Principal:                  s.Principal,
		DefaultScheme:              s.DefaultScheme,
		IncludeModel:               !s.SkipModels,
		IncludeValidator:           !s.SkipModels,
		IncludeHandler:             !s.SkipOperations,
		IncludeParameters:          !s.SkipOperations,
		IncludeResponses:           !s.SkipOperations,
		IncludeURLBuilder:          !s.SkipOperations,
		IncludeMain:                !s.ExcludeMain,
		IncludeSupport:             !s.SkipSupport,
		PropertiesSpecOrder:        s.KeepSpecOrder,
		ValidateSpec:               !s.SkipValidation,
		ExcludeSpec:                s.ExcludeSpec,
		StrictAdditionalProperties: s.StrictAdditionalProperties,
		Template:                   s.Template,
		RegenerateConfigureAPI:     s.RegenerateConfigureAPI,
		TemplateDir:                string(s.TemplateDir),
		DumpData:                   s.DumpData,
		Models:                     s.Models,
		Operations:                 s.Operations,
		Tags:                       s.Tags,
		Name:                       s.Name,
		FlagStrategy:               s.FlagStrategy,
		CompatibilityMode:          s.CompatibilityMode,
		ExistingModels:             s.ExistingModels,
		AllowTemplateOverride:      s.AllowTemplateOverride,
	}, nil
}

func (s *Server) getShared() *shared {
	return &s.shared
}

func (s *Server) generate(opts *generator.GenOpts) error {
	return generator.GenerateServer(s.Name, s.Models, s.Operations, opts)
}

func (s *Server) log(rp string) {
	var flagsPackage string
	if strings.HasPrefix(s.FlagStrategy, "pflag") {
		flagsPackage = "github.com/spf13/pflag"
	} else if strings.HasPrefix(s.FlagStrategy, "flag") {
		flagsPackage = "flag"
	} else {
		flagsPackage = "github.com/jessevdk/go-flags"
	}

	log.Printf(`Generation completed!

For this generation to compile you need to have some packages in your GOPATH:

	* github.com/go-openapi/runtime
	* `+flagsPackage+`

You can get these now with: go get -u -f %s/...
`, rp)
}

// Execute runs this command
func (s *Server) Execute(args []string) error {
	return createSwagger(s)
}
