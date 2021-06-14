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

type serverOptions struct {
	ServerPackage         string `long:"server-package" short:"s" description:"the package to save the server specific code" default:"restapi"`
	MainTarget            string `long:"main-package" short:"" description:"the location of the generated main. Defaults to cmd/{name}-server" default:""`
	ImplementationPackage string `long:"implementation-package" short:"" description:"the location of the backend implementation of the server, which will be autowired with api" default:""`
}

func (cs serverOptions) apply(opts *generator.GenOpts) {
	opts.ServerPackage = cs.ServerPackage
}

// Server the command to generate an entire server application
type Server struct {
	WithShared
	WithModels
	WithOperations

	serverOptions
	schemeOptions
	mediaOptions

	SkipModels             bool   `long:"skip-models" description:"no models will be generated when this flag is specified"`
	SkipOperations         bool   `long:"skip-operations" description:"no operations will be generated when this flag is specified"`
	SkipSupport            bool   `long:"skip-support" description:"no supporting files will be generated when this flag is specified"`
	ExcludeMain            bool   `long:"exclude-main" description:"exclude main function, so just generate the library"`
	ExcludeSpec            bool   `long:"exclude-spec" description:"don't embed the swagger specification"`
	FlagStrategy           string `long:"flag-strategy" description:"the strategy to provide flags for the server" default:"go-flags" choice:"go-flags" choice:"pflag" choice:"flag"` // nolint: staticcheck
	CompatibilityMode      string `long:"compatibility-mode" description:"the compatibility mode for the tls server" default:"modern" choice:"modern" choice:"intermediate"`          // nolint: staticcheck
	RegenerateConfigureAPI bool   `long:"regenerate-configureapi" description:"Force regeneration of configureapi.go"`

	Name string `long:"name" short:"A" description:"the name of the application, defaults to a mangled value of info.title"`
	// TODO(fredbi): CmdName string `long:"cmd-name" short:"A" description:"the name of the server command, when main is generated (defaults to {name}-server)"`

	// deprecated flags
	WithContext bool `long:"with-context" description:"handlers get a context as first arg (deprecated)"`
}

func (s Server) apply(opts *generator.GenOpts) {
	if s.WithContext {
		log.Printf("warning: deprecated option --with-context is ignored")
	}

	s.Shared.apply(opts)
	s.Models.apply(opts)
	s.Operations.apply(opts)
	s.serverOptions.apply(opts)
	s.schemeOptions.apply(opts)
	s.mediaOptions.apply(opts)

	opts.IncludeModel = !s.SkipModels
	opts.IncludeValidator = !s.SkipModels
	opts.IncludeHandler = !s.SkipOperations
	opts.IncludeParameters = !s.SkipOperations
	opts.IncludeResponses = !s.SkipOperations
	opts.IncludeURLBuilder = !s.SkipOperations
	opts.IncludeSupport = !s.SkipSupport
	opts.IncludeMain = !s.ExcludeMain
	opts.FlagStrategy = s.FlagStrategy
	opts.CompatibilityMode = s.CompatibilityMode
	opts.RegenerateConfigureAPI = s.RegenerateConfigureAPI

	opts.Name = s.Name
	opts.MainPackage = s.MainTarget

	opts.ImplementationPackage = s.ImplementationPackage
}

func (s *Server) generate(opts *generator.GenOpts) error {
	return generator.GenerateServer(s.Name, s.Models.Models, s.Operations.Operations, opts)
}

func (s Server) log(rp string) {
	var flagsPackage string
	switch {
	case strings.HasPrefix(s.FlagStrategy, "pflag"):
		flagsPackage = "github.com/spf13/pflag"
	case strings.HasPrefix(s.FlagStrategy, "flag"):
		flagsPackage = "flag"
	default:
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
