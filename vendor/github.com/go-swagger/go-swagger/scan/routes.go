// +build !go1.11

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

package scan

import (
	"fmt"
	"go/ast"

	"github.com/go-openapi/spec"

	"golang.org/x/tools/go/loader"
)

func opConsumesSetter(op *spec.Operation) func([]string) {
	return func(consumes []string) { op.Consumes = consumes }
}

func opProducesSetter(op *spec.Operation) func([]string) {
	return func(produces []string) { op.Produces = produces }
}

func opSchemeSetter(op *spec.Operation) func([]string) {
	return func(schemes []string) { op.Schemes = schemes }
}

func opSecurityDefsSetter(op *spec.Operation) func([]map[string][]string) {
	return func(securityDefs []map[string][]string) { op.Security = securityDefs }
}

func opResponsesSetter(op *spec.Operation) func(*spec.Response, map[int]spec.Response) {
	return func(def *spec.Response, scr map[int]spec.Response) {
		if op.Responses == nil {
			op.Responses = new(spec.Responses)
		}
		op.Responses.Default = def
		op.Responses.StatusCodeResponses = scr
	}
}

func opParamSetter(op *spec.Operation) func([]*spec.Parameter) {
	return func(params []*spec.Parameter) {
		for _, v := range params {
			op.AddParam(v)
		}
	}
}

func newRoutesParser(prog *loader.Program) *routesParser {
	return &routesParser{
		program: prog,
	}
}

type routesParser struct {
	program     *loader.Program
	definitions map[string]spec.Schema
	operations  map[string]*spec.Operation
	responses   map[string]spec.Response
	parameters  []*spec.Parameter
}

func (rp *routesParser) Parse(gofile *ast.File, target interface{}, includeTags map[string]bool, excludeTags map[string]bool) error {
	tgt := target.(*spec.Paths)
	for _, comsec := range gofile.Comments {
		content := parsePathAnnotation(rxRoute, comsec.List)

		if content.Method == "" {
			continue // it's not, next!
		}

		if !shouldAcceptTag(content.Tags, includeTags, excludeTags) {
			if Debug {
				fmt.Printf("route %s %s is ignored due to tag rules\n", content.Method, content.Path)
			}
			continue
		}

		pthObj := tgt.Paths[content.Path]
		op := setPathOperation(
			content.Method, content.ID,
			&pthObj, rp.operations[content.ID])

		op.Tags = content.Tags

		sp := new(sectionedParser)
		sp.setTitle = func(lines []string) { op.Summary = joinDropLast(lines) }
		sp.setDescription = func(lines []string) { op.Description = joinDropLast(lines) }
		sr := newSetResponses(rp.definitions, rp.responses, opResponsesSetter(op))
		spa := newSetParams(rp.parameters, opParamSetter(op))
		sp.taggers = []tagParser{
			newMultiLineTagParser("Consumes", newMultilineDropEmptyParser(rxConsumes, opConsumesSetter(op)), false),
			newMultiLineTagParser("Produces", newMultilineDropEmptyParser(rxProduces, opProducesSetter(op)), false),
			newSingleLineTagParser("Schemes", newSetSchemes(opSchemeSetter(op))),
			newMultiLineTagParser("Security", newSetSecurity(rxSecuritySchemes, opSecurityDefsSetter(op)), false),
			newMultiLineTagParser("Parameters", spa, false),
			newMultiLineTagParser("Responses", sr, false),
		}
		if err := sp.Parse(content.Remaining); err != nil {
			return fmt.Errorf("operation (%s): %v", op.ID, err)
		}

		if tgt.Paths == nil {
			tgt.Paths = make(map[string]spec.PathItem)
		}
		tgt.Paths[content.Path] = pthObj
	}

	return nil
}

func shouldAcceptTag(tags []string, includeTags map[string]bool, excludeTags map[string]bool) bool {
	for _, tag := range tags {
		if len(includeTags) > 0 {
			if includeTags[tag] {
				return true
			}
		} else if len(excludeTags) > 0 {
			if excludeTags[tag] {
				return false
			}
		}
	}
	return len(includeTags) <= 0
}
