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

package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/loads/fmts"
	"github.com/go-swagger/go-swagger/cmd/swagger/commands"
	flags "github.com/jessevdk/go-flags"
)

func init() {
	loads.AddLoader(fmts.YAMLMatcher, fmts.YAMLDoc)
}

var (
	// Debug is true when the SWAGGER_DEBUG env var is not empty
	Debug = os.Getenv("SWAGGER_DEBUG") != ""
)

var opts struct {
	// General options applicable to all commands
	Quiet   func()       `long:"quiet" short:"q" description:"silence logs"`
	LogFile func(string) `long:"log-output" description:"redirect logs to file" value-name:"LOG-FILE"`
	// Version bool `long:"version" short:"v" description:"print the version of the command"`
}

func main() {
	// TODO: reactivate 'defer catch all' once product is stable
	// Recovering from internal panics
	// Stack may be printed in Debug mode
	// Need import "runtime/debug".
	//defer func() {
	//	r := recover()
	//	if r != nil {
	//		log.Printf("Fatal error:", r)
	//		if Debug {
	//			debug.PrintStack()
	//		}
	//		os.Exit(1)
	//	}
	//}()

	parser := flags.NewParser(&opts, flags.Default)
	parser.ShortDescription = "helps you keep your API well described"
	parser.LongDescription = `
Swagger tries to support you as best as possible when building APIs.

It aims to represent the contract of your API with a language agnostic description of your application in json or yaml.
`
	_, err := parser.AddCommand("validate", "validate the swagger document", "validate the provided swagger document against a swagger spec", &commands.ValidateSpec{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("init", "initialize a spec document", "initialize a swagger spec document", &commands.InitCmd{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("version", "print the version", "print the version of the swagger command", &commands.PrintVersion{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("serve", "serve spec and docs", "serve a spec and swagger or redoc documentation ui", &commands.ServeCmd{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("expand", "expand $ref fields in a swagger spec", "expands the $refs in a swagger document to inline schemas", &commands.ExpandSpec{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("flatten", "flattens a swagger document", "expand the remote references in a spec and move inline schemas to definitions, after flattening there are no complex inlined anymore", &commands.FlattenSpec{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("mixin", "merge swagger documents", "merge additional specs into first/primary spec by copying their paths and definitions", &commands.MixinSpec{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("diff", "diff swagger documents", "diff specs showing which changes will break existing clients", &commands.DiffCommand{})
	if err != nil {
		log.Fatal(err)
	}

	genpar, err := parser.AddCommand("generate", "generate go code", "generate go code for the swagger spec file", &commands.Generate{})
	if err != nil {
		log.Fatalln(err)
	}
	for _, cmd := range genpar.Commands() {
		switch cmd.Name {
		case "spec":
			cmd.ShortDescription = "generate a swagger spec document from a go application"
			cmd.LongDescription = cmd.ShortDescription
		case "client":
			cmd.ShortDescription = "generate all the files for a client library"
			cmd.LongDescription = cmd.ShortDescription
		case "server":
			cmd.ShortDescription = "generate all the files for a server application"
			cmd.LongDescription = cmd.ShortDescription
		case "model":
			cmd.ShortDescription = "generate one or more models from the swagger spec"
			cmd.LongDescription = cmd.ShortDescription
		case "support":
			cmd.ShortDescription = "generate supporting files like the main function and the api builder"
			cmd.LongDescription = cmd.ShortDescription
		case "operation":
			cmd.ShortDescription = "generate one or more server operations from the swagger spec"
			cmd.LongDescription = cmd.ShortDescription
		}
	}

	opts.Quiet = func() {
		log.SetOutput(ioutil.Discard)
	}
	opts.LogFile = func(logfile string) {
		f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			log.Fatalf("cannot write to file %s: %v", logfile, err)
		}
		log.SetOutput(f)
	}

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}
