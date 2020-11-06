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

	"github.com/go-swagger/go-swagger/generator"
)

type clientOptions struct {
	ClientPackage string `long:"client-package" short:"c" description:"the package to save the client specific code" default:"client"`
}

func (co clientOptions) apply(opts *generator.GenOpts) {
	opts.ClientPackage = co.ClientPackage
}

// Client the command to generate a swagger client
type Client struct {
	WithShared
	WithModels
	WithOperations

	clientOptions
	schemeOptions
	mediaOptions

	SkipModels     bool `long:"skip-models" description:"no models will be generated when this flag is specified"`
	SkipOperations bool `long:"skip-operations" description:"no operations will be generated when this flag is specified"`

	Name string `long:"name" short:"A" description:"the name of the application, defaults to a mangled value of info.title"`
}

func (c Client) apply(opts *generator.GenOpts) {
	c.Shared.apply(opts)
	c.Models.apply(opts)
	c.Operations.apply(opts)
	c.clientOptions.apply(opts)
	c.schemeOptions.apply(opts)
	c.mediaOptions.apply(opts)

	opts.IncludeModel = !c.SkipModels
	opts.IncludeValidator = !c.SkipModels
	opts.IncludeHandler = !c.SkipOperations
	opts.IncludeParameters = !c.SkipOperations
	opts.IncludeResponses = !c.SkipOperations
	opts.Name = c.Name

	opts.IsClient = true
	opts.IncludeSupport = true
}

func (c *Client) generate(opts *generator.GenOpts) error {
	return generator.GenerateClient(c.Name, c.Models.Models, c.Operations.Operations, opts)
}

func (c *Client) log(rp string) {
	log.Printf(`Generation completed!

For this generation to compile you need to have some packages in your GOPATH:

	* github.com/go-openapi/errors
	* github.com/go-openapi/runtime
	* github.com/go-openapi/runtime/client
	* github.com/go-openapi/strfmt

You can get these now with: go get -u -f %s/...
`, rp)
}

// Execute runs this command
func (c *Client) Execute(args []string) error {
	return createSwagger(c)
}
