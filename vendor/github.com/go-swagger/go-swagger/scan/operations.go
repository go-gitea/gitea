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

func newOperationsParser(prog *loader.Program) *operationsParser {
	return &operationsParser{
		program: prog,
	}
}

type operationsParser struct {
	program     *loader.Program
	definitions map[string]spec.Schema
	operations  map[string]*spec.Operation
	responses   map[string]spec.Response
}

func (op *operationsParser) Parse(gofile *ast.File, target interface{}, includeTags map[string]bool, excludeTags map[string]bool) error {
	tgt := target.(*spec.Paths)
	for _, comsec := range gofile.Comments {
		content := parsePathAnnotation(rxOperation, comsec.List)

		if content.Method == "" {
			continue // it's not, next!
		}

		if !shouldAcceptTag(content.Tags, includeTags, excludeTags) {
			if Debug {
				fmt.Printf("operation %s %s is ignored due to tag rules\n", content.Method, content.Path)
			}
			continue
		}

		pthObj := tgt.Paths[content.Path]

		op := setPathOperation(
			content.Method, content.ID,
			&pthObj, op.operations[content.ID])

		op.Tags = content.Tags

		sp := new(yamlSpecScanner)
		sp.setTitle = func(lines []string) { op.Summary = joinDropLast(lines) }
		sp.setDescription = func(lines []string) { op.Description = joinDropLast(lines) }

		if err := sp.Parse(content.Remaining); err != nil {
			return fmt.Errorf("operation (%s): %v", op.ID, err)
		}
		if err := sp.UnmarshalSpec(op.UnmarshalJSON); err != nil {
			return fmt.Errorf("operation (%s): %v", op.ID, err)
		}

		if tgt.Paths == nil {
			tgt.Paths = make(map[string]spec.PathItem)
		}

		tgt.Paths[content.Path] = pthObj
	}

	return nil
}
