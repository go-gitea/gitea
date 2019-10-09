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
	"log"
	"regexp"

	"golang.org/x/tools/go/loader"
)

type packageFilter struct {
	Name string
}

func (pf *packageFilter) Matches(path string) bool {
	matched, err := regexp.MatchString(pf.Name, path)
	if err != nil {
		log.Fatal(err)
	}
	return matched
}

type packageFilters []packageFilter

func (pf packageFilters) HasFilters() bool {
	return len(pf) > 0
}

func (pf packageFilters) Matches(path string) bool {
	for _, mod := range pf {
		if mod.Matches(path) {
			return true
		}
	}
	return false
}

type classifiedProgram struct {
	Meta       []*ast.File
	Models     []*ast.File
	Routes     []*ast.File
	Operations []*ast.File
	Parameters []*ast.File
	Responses  []*ast.File
}

// programClassifier classifies the files of a program into buckets
// for processing by a swagger spec generator. This buckets files in
// 3 groups: Meta, Models and Operations.
//
// Each of these buckets is then processed with an appropriate parsing strategy
//
// When there are Include or Exclude filters provide they are used to limit the
// candidates prior to parsing.
// The include filters take precedence over the excludes. So when something appears
// in both filters it will be included.
type programClassifier struct {
	Includes packageFilters
	Excludes packageFilters
}

func (pc *programClassifier) Classify(prog *loader.Program) (*classifiedProgram, error) {
	var cp classifiedProgram
	for pkg, pkgInfo := range prog.AllPackages {
		if Debug {
			log.Printf("analyzing: %s\n", pkg.Path())
		}
		if pc.Includes.HasFilters() {
			if !pc.Includes.Matches(pkg.Path()) {
				continue
			}
		} else if pc.Excludes.HasFilters() {
			if pc.Excludes.Matches(pkg.Path()) {
				continue
			}
		}

		for _, file := range pkgInfo.Files {
			var ro, op, mt, pm, rs, mm bool // only add a particular file once
			for _, comments := range file.Comments {
				var seenStruct string
				for _, cline := range comments.List {
					if cline != nil {
						matches := rxSwaggerAnnotation.FindStringSubmatch(cline.Text)
						if len(matches) > 1 {
							switch matches[1] {
							case "route":
								if !ro {
									cp.Routes = append(cp.Routes, file)
									ro = true
								}
							case "operation":
								if !op {
									cp.Operations = append(cp.Operations, file)
									op = true
								}
							case "model":
								if !mm {
									cp.Models = append(cp.Models, file)
									mm = true
								}
								if seenStruct == "" || seenStruct == matches[1] {
									seenStruct = matches[1]
								} else {
									return nil, fmt.Errorf("classifier: already annotated as %s, can't also be %q", seenStruct, matches[1])
								}
							case "meta":
								if !mt {
									cp.Meta = append(cp.Meta, file)
									mt = true
								}
							case "parameters":
								if !pm {
									cp.Parameters = append(cp.Parameters, file)
									pm = true
								}
								if seenStruct == "" || seenStruct == matches[1] {
									seenStruct = matches[1]
								} else {
									return nil, fmt.Errorf("classifier: already annotated as %s, can't also be %q", seenStruct, matches[1])
								}
							case "response":
								if !rs {
									cp.Responses = append(cp.Responses, file)
									rs = true
								}
								if seenStruct == "" || seenStruct == matches[1] {
									seenStruct = matches[1]
								} else {
									return nil, fmt.Errorf("classifier: already annotated as %s, can't also be %q", seenStruct, matches[1])
								}
							case "strfmt", "name", "discriminated", "file", "enum", "default", "alias", "type":
								// TODO: perhaps collect these and pass along to avoid lookups later on
							case "allOf":
							case "ignore":
							default:
								return nil, fmt.Errorf("classifier: unknown swagger annotation %q", matches[1])
							}
						}

					}
				}
			}
		}
	}

	return &cp, nil
}
