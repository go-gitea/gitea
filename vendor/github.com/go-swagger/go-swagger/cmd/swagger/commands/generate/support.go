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

// Support generates the supporting files
type Support struct {
	shared
	Name          string   `long:"name" short:"A" description:"the name of the application, defaults to a mangled value of info.title"`
	Operations    []string `long:"operation" short:"O" description:"specify an operation to include, repeat for multiple"`
	Principal     string   `long:"principal" description:"the model to use for the security principal"`
	Models        []string `long:"model" short:"M" description:"specify a model to include, repeat for multiple"`
	DumpData      bool     `long:"dump-data" description:"when present dumps the json for the template generator instead of generating files"`
	DefaultScheme string   `long:"default-scheme" description:"the default scheme for this API" default:"http"`
}

func (s *Support) getOpts() (*generator.GenOpts, error) {
	return &generator.GenOpts{
		Spec:          string(s.Spec),
		Target:        string(s.Target),
		APIPackage:    s.APIPackage,
		ModelPackage:  s.ModelPackage,
		ServerPackage: s.ServerPackage,
		ClientPackage: s.ClientPackage,
		Principal:     s.Principal,
		DumpData:      s.DumpData,
		DefaultScheme: s.DefaultScheme,
		Template:      s.Template,
		TemplateDir:   string(s.TemplateDir),
	}, nil
}

func (s *Support) getShared() *shared {
	return &s.shared
}

func (s *Support) generate(opts *generator.GenOpts) error {
	return generator.GenerateSupport(s.Name, nil, nil, opts)
}

func (s *Support) log(rp string) {

	log.Printf(`Generation completed!

For this generation to compile you need to have some packages in your vendor or GOPATH:

  * github.com/go-openapi/runtime
  * github.com/asaskevich/govalidator
  * github.com/jessevdk/go-flags
  * golang.org/x/net/context/ctxhttp

You can get these now with: go get -u -f %s/...
`, rp)
}

// Execute generates the supporting files file
func (s *Support) Execute(args []string) error {
	return createSwagger(s)
}
