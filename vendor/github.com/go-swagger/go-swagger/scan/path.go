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
	"go/ast"
	"regexp"
	"strings"

	"github.com/go-openapi/spec"
)

type parsedPathContent struct {
	Method, Path, ID string
	Tags             []string
	Remaining        *ast.CommentGroup
}

func parsePathAnnotation(annotation *regexp.Regexp, lines []*ast.Comment) (cnt parsedPathContent) {
	var justMatched bool

	for _, cmt := range lines {
		for _, line := range strings.Split(cmt.Text, "\n") {
			matches := annotation.FindStringSubmatch(line)
			if len(matches) > 3 {
				cnt.Method, cnt.Path, cnt.ID = matches[1], matches[2], matches[len(matches)-1]
				cnt.Tags = rxSpace.Split(matches[3], -1)
				if len(matches[3]) == 0 {
					cnt.Tags = nil
				}
				justMatched = true
			} else if cnt.Method != "" {
				if cnt.Remaining == nil {
					cnt.Remaining = new(ast.CommentGroup)
				}
				if !justMatched || strings.TrimSpace(rxStripComments.ReplaceAllString(line, "")) != "" {
					cc := new(ast.Comment)
					cc.Slash = cmt.Slash
					cc.Text = line
					cnt.Remaining.List = append(cnt.Remaining.List, cc)
					justMatched = false
				}
			}
		}
	}

	return
}

func setPathOperation(method, id string, pthObj *spec.PathItem, op *spec.Operation) *spec.Operation {
	if op == nil {
		op = new(spec.Operation)
		op.ID = id
	}

	switch strings.ToUpper(method) {
	case "GET":
		if pthObj.Get != nil {
			if id == pthObj.Get.ID {
				op = pthObj.Get
			} else {
				pthObj.Get = op
			}
		} else {
			pthObj.Get = op
		}

	case "POST":
		if pthObj.Post != nil {
			if id == pthObj.Post.ID {
				op = pthObj.Post
			} else {
				pthObj.Post = op
			}
		} else {
			pthObj.Post = op
		}

	case "PUT":
		if pthObj.Put != nil {
			if id == pthObj.Put.ID {
				op = pthObj.Put
			} else {
				pthObj.Put = op
			}
		} else {
			pthObj.Put = op
		}

	case "PATCH":
		if pthObj.Patch != nil {
			if id == pthObj.Patch.ID {
				op = pthObj.Patch
			} else {
				pthObj.Patch = op
			}
		} else {
			pthObj.Patch = op
		}

	case "HEAD":
		if pthObj.Head != nil {
			if id == pthObj.Head.ID {
				op = pthObj.Head
			} else {
				pthObj.Head = op
			}
		} else {
			pthObj.Head = op
		}

	case "DELETE":
		if pthObj.Delete != nil {
			if id == pthObj.Delete.ID {
				op = pthObj.Delete
			} else {
				pthObj.Delete = op
			}
		} else {
			pthObj.Delete = op
		}

	case "OPTIONS":
		if pthObj.Options != nil {
			if id == pthObj.Options.ID {
				op = pthObj.Options
			} else {
				pthObj.Options = op
			}
		} else {
			pthObj.Options = op
		}
	}

	return op
}
