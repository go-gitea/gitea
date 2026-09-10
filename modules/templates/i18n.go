// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"os"
	"regexp"
	"strings"
	"text/template"
	"text/template/parse"

	"gitea.dev/modules/container"
)

func isI18nFunc(name string) bool {
	return strings.HasSuffix(name, ".Tr") || strings.HasSuffix(name, ".TrN") || strings.HasSuffix(name, ".TrString")
}

func i18nFuncName(arg parse.Node) string {
	switch n := arg.(type) {
	case *parse.ChainNode:
		return n.String()
	case *parse.FieldNode:
		return n.String()
	case *parse.VariableNode:
		return n.String()
	default:
		return ""
	}
}

func collectStringLiterals(n parse.Node, keys container.Set[string]) {
	switch x := n.(type) {
	case *parse.StringNode:
		keys.Add(x.Text)
	case *parse.PipeNode:
		for _, cmd := range x.Cmds {
			collectStringLiterals(cmd, keys)
		}
	case *parse.CommandNode:
		for _, arg := range x.Args {
			collectStringLiterals(arg, keys)
		}
	}
}

func isI18nTrN(name string) bool {
	return strings.HasSuffix(name, ".TrN")
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
		var keys = container.Set[string]{}
		pending := override
		if len(n.Args) >= 2 {
			funcName := i18nFuncName(n.Args[0])
			if isI18nFunc(funcName) {
				if override != "" {
					keys.Add(strings.TrimSpace(override))
					pending = ""
				} else {
					needed := 1
					if isI18nTrN(funcName) {
						needed = 2
					}
					for _, arg := range n.Args[1:] { // sometimes it will be `ctx.Locale.Tr (print "key")` or `Iif`
						if str, ok := arg.(*parse.StringNode); ok {
							keys.Add(str.Text)
							if len(keys) == needed {
								break
							}
						} else if p, ok := arg.(*parse.PipeNode); ok {
							collectStringLiterals(p, keys)
							if len(keys) >= needed {
								break
							}
						}
					}
				}
			}
		}
		for _, arg := range n.Args {
			if p, ok := arg.(*parse.PipeNode); ok {
				var subKeys container.Set[string]
				subKeys, pending = collectI18nKeys(p, pending)
				keys = keys.Union(subKeys)
			}
		}
		if len(keys) > 0 {
			return keys, pending
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
	funcMap := newFuncMapWebPage()
	for name, fn := range mailBodyFuncMap() {
		if _, exists := funcMap[name]; !exists {
			funcMap[name] = fn
		}
	}
	funcMap["ctx"] = func() any { return nil }
	t, err := template.New("test").Funcs(template.FuncMap(funcMap)).Parse(string(bs))
	if err != nil {
		return nil, err
	}

	return extractI18nKeys(t.Root), nil
}
