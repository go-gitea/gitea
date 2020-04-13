package codescan

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"github.com/go-openapi/spec"
)

type operationsBuilder struct {
	ctx        *scanCtx
	path       parsedPathContent
	operations map[string]*spec.Operation
}

func (o *operationsBuilder) Build(tgt *spec.Paths) error {
	pthObj := tgt.Paths[o.path.Path]

	op := setPathOperation(
		o.path.Method, o.path.ID,
		&pthObj, o.operations[o.path.ID])

	op.Tags = o.path.Tags

	sp := new(yamlSpecScanner)
	sp.setTitle = func(lines []string) { op.Summary = joinDropLast(lines) }
	sp.setDescription = func(lines []string) { op.Description = joinDropLast(lines) }

	if err := sp.Parse(o.path.Remaining); err != nil {
		return fmt.Errorf("operation (%s): %v", op.ID, err)
	}
	if err := sp.UnmarshalSpec(op.UnmarshalJSON); err != nil {
		return fmt.Errorf("operation (%s): %v", op.ID, err)
	}

	if tgt.Paths == nil {
		tgt.Paths = make(map[string]spec.PathItem)
	}

	tgt.Paths[o.path.Path] = pthObj
	return nil
}

type parsedPathContent struct {
	Method, Path, ID string
	Tags             []string
	Remaining        *ast.CommentGroup
}

func parsePathAnnotation(annotation *regexp.Regexp, lines []*ast.Comment) (cnt parsedPathContent) {
	var justMatched bool

	for _, cmt := range lines {
		txt := cmt.Text
		for _, line := range strings.Split(txt, "\n") {
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
