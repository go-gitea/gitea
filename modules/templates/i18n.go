// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
	"text/template/parse"

	"code.gitea.io/gitea/modules/container"
)

func isI18nFunc(name string) bool {
	return name == "ctx.Locale.Tr" || name == "ctx.Locale.TrN" || name == "ctx.Locale.TrString"
}

var i18nCheckPattern = regexp.MustCompile(`<!--\s*i18n-check:\s*([^>]+?)\s*-->`)

func extractI18nKeys(node parse.Node) container.Set[string] {
	keys, _ := collectI18nKeys(node, "")
	return keys
}

func collectI18nKeys(node parse.Node, override string) (container.Set[string], string) {
	switch n := node.(type) {
	case *parse.WithNode:
		keys, nextOverride := collectI18nKeys(n.List, override)
		if n.Pipe != nil {
			var pipeKeys container.Set[string]
			pipeKeys, nextOverride = collectI18nKeys(n.Pipe, nextOverride)
			keys = keys.Union(pipeKeys)
		}
		return keys, nextOverride
	case *parse.ListNode:
		var keys = container.Set[string]{}
		pending := override
		for _, sub := range n.Nodes {
			var subKeys container.Set[string]
			subKeys, pending = collectI18nKeys(sub, pending)
			keys = keys.Union(subKeys)
		}
		return keys, pending
	case *parse.TemplateNode: // ignore the file inclusion
		fmt.Printf("Visiting node: %#v\n---\n%s\n---\n", node, node.String())
		if n.Pipe != nil {
			return collectI18nKeys(n.Pipe, override)
		}
		return container.Set[string]{}, override
	case *parse.TextNode: // detect optional override hints
		if hint, ok := extractI18nCheckOverride(string(n.Text)); ok {
			return container.Set[string]{}, hint
		}
		return container.Set[string]{}, override
	case *parse.IfNode:
		keys, nextOverride := collectI18nKeys(n.List, override)
		if n.ElseList != nil {
			var elseKeys container.Set[string]
			elseKeys, nextOverride = collectI18nKeys(n.ElseList, nextOverride)
			keys = keys.Union(elseKeys)
		}
		return keys, nextOverride
	case *parse.RangeNode:
		keys, nextOverride := collectI18nKeys(n.List, override)
		if n.Pipe != nil {
			var pipeKeys container.Set[string]
			pipeKeys, nextOverride = collectI18nKeys(n.Pipe, nextOverride)
			keys = keys.Union(pipeKeys)
		}
		return keys, nextOverride
	case *parse.ActionNode:
		return collectI18nKeys(n.Pipe, override)
	case *parse.PipeNode:
		var keys = container.Set[string]{}
		pending := override
		for _, cmd := range n.Cmds {
			var subKeys container.Set[string]
			subKeys, pending = collectI18nKeys(cmd, pending)
			keys = keys.Union(subKeys)
		}
		return keys, pending
	case *parse.CommandNode:
		if len(n.Args) >= 2 {
			if ident, ok := n.Args[0].(*parse.ChainNode); ok && isI18nFunc(ident.String()) {
				var keys = container.Set[string]{}
				if override != "" {
					keys.Add(strings.TrimSpace(override))
					return keys, ""
				}
				for _, arg := range n.Args[1:] { // sometimes it will be `ctx.Locale.Tr (print "key")`
					if str, ok := arg.(*parse.StringNode); ok {
						keys.Add(str.Text)
						if (ident.String() == "ctx.Locale.TrN" && len(keys) == 2) || (ident.String() != "ctx.Locale.TrN" && len(keys) == 1) {
							return keys, override
						}
					} else if p, ok := arg.(*parse.PipeNode); ok {
						for _, cmd := range p.Cmds {
							if cmd.Args != nil && len(cmd.Args) > 0 {
								for _, cmdArg := range cmd.Args {
									if str, ok := cmdArg.(*parse.StringNode); ok {
										keys.Add(str.Text)
										if (ident.String() == "ctx.Locale.TrN" && len(keys) == 2) || (ident.String() != "ctx.Locale.TrN" && len(keys) == 1) {
											return keys, override
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
	return container.Set[string]{}, override
}

func extractI18nCheckOverride(text string) (string, bool) {
	matches := i18nCheckPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return "", false
	}
	return strings.TrimSpace(matches[len(matches)-1][1]), true
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
