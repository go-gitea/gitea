// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"
	"os"
	"text/template"
	"text/template/parse"

	"code.gitea.io/gitea/modules/container"
)

func isI18nFunc(name string) bool {
	return name == "ctx.Locale.Tr" || name == "ctx.Locale.TrN" || name == "ctx.Locale.TrString"
}

func extractI18nKeys(node parse.Node) container.Set[string] {
	switch n := node.(type) {
	case *parse.WithNode:
		keys := extractI18nKeys(n.List)
		if n.Pipe != nil {
			keys = keys.Union(extractI18nKeys(n.Pipe))
		}
		return keys
	case *parse.ListNode:
		var keys = container.Set[string]{}
		for _, sub := range n.Nodes {
			keys = keys.Union(extractI18nKeys(sub))
		}
		return keys
	case *parse.TemplateNode: // ignore the file inclusion
		fmt.Printf("Visiting node: %#v\n---\n%s\n---\n", node, node.String())
		if n.Pipe != nil {
			return extractI18nKeys(n.Pipe)
		}
		return container.Set[string]{}
	case *parse.TextNode: // ignore text nodes
		return container.Set[string]{}
	case *parse.IfNode:
		keys := extractI18nKeys(n.List)
		if n.ElseList != nil {
			keys = keys.Union(extractI18nKeys(n.ElseList))
		}
		return keys
	case *parse.RangeNode:
		keys := extractI18nKeys(n.List)
		if n.Pipe != nil {
			keys = keys.Union(extractI18nKeys(n.Pipe))
		}
		return keys
	case *parse.ActionNode:
		return extractI18nKeys(n.Pipe)
	case *parse.PipeNode:
		var keys = container.Set[string]{}
		for _, cmd := range n.Cmds {
			keys = keys.Union(extractI18nKeys(cmd))
		}
		return keys
	case *parse.CommandNode:
		if len(n.Args) >= 2 {
			if ident, ok := n.Args[0].(*parse.ChainNode); ok && isI18nFunc(ident.String()) {
				var keys = container.Set[string]{}
				for _, arg := range n.Args[1:] { // sometimes it will be `ctx.Locale.Tr (print "key")`
					if str, ok := arg.(*parse.StringNode); ok {
						keys.Add(str.Text)
						if (ident.String() == "ctx.Locale.TrN" && len(keys) == 2) || (ident.String() != "ctx.Locale.TrN" && len(keys) == 1) {
							return keys
						}
					} else if p, ok := arg.(*parse.PipeNode); ok {
						for _, cmd := range p.Cmds {
							if cmd.Args != nil && len(cmd.Args) > 0 {
								for _, cmdArg := range cmd.Args {
									if str, ok := cmdArg.(*parse.StringNode); ok {
										keys.Add(str.Text)
										if (ident.String() == "ctx.Locale.TrN" && len(keys) == 2) || (ident.String() != "ctx.Locale.TrN" && len(keys) == 1) {
											return keys
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return container.Set[string]{}
}

func FindTemplateKeys(p string) (container.Set[string], error) {
	bs, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	// The template parser requires the function map otherwise it will return failure
	t, err := template.New("test").Funcs(NewFuncMap()).Parse(string(bs))
	if err != nil {
		return nil, err
	}

	return extractI18nKeys(t.Root), nil
}
