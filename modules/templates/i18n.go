// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"maps"
	"os"
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
	case *parse.IdentifierNode:
		return n.Ident
	default:
		return ""
	}
}

func isI18nTrN(name string) bool {
	return strings.HasSuffix(name, ".TrN")
}

func isCompositingTemplateFunc(name string) bool {
	return name == "print" || name == "printf" || name == "println" ||
		strings.HasSuffix(name, ".print") || strings.HasSuffix(name, ".printf")
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
	case *parse.ListNode:
		for _, sub := range x.Nodes {
			collectStringLiterals(sub, keys)
		}
	case *parse.ActionNode:
		if x.Pipe != nil {
			collectStringLiterals(x.Pipe, keys)
		}
	case *parse.IfNode:
		collectStringLiterals(x.List, keys)
		if x.ElseList != nil {
			collectStringLiterals(x.ElseList, keys)
		}
	case *parse.RangeNode:
		collectStringLiterals(x.List, keys)
		if x.ElseList != nil {
			collectStringLiterals(x.ElseList, keys)
		}
		if x.Pipe != nil {
			collectStringLiterals(x.Pipe, keys)
		}
	case *parse.WithNode:
		collectStringLiterals(x.List, keys)
		if x.ElseList != nil {
			collectStringLiterals(x.ElseList, keys)
		}
		if x.Pipe != nil {
			collectStringLiterals(x.Pipe, keys)
		}
	case *parse.TemplateNode:
		if x.Pipe != nil {
			collectStringLiterals(x.Pipe, keys)
		}
	}
}

func pipeHasCompositing(p *parse.PipeNode) bool {
	for _, cmd := range p.Cmds {
		if len(cmd.Args) == 0 {
			continue
		}
		if isCompositingTemplateFunc(i18nFuncName(cmd.Args[0])) {
			return true
		}
		for _, arg := range cmd.Args[1:] {
			if nested, ok := arg.(*parse.PipeNode); ok && pipeHasCompositing(nested) {
				return true
			}
		}
	}
	return false
}

func collectTrArgKeys(arg parse.Node, keys container.Set[string]) (dynamic string, ok bool) {
	switch n := arg.(type) {
	case *parse.StringNode:
		keys.Add(n.Text)
		return "", true
	case *parse.PipeNode:
		if pipeHasCompositing(n) {
			return "composited translation key (print/printf is forbidden)", false
		}
		collectStringLiterals(n, keys)
		return "", true
	case *parse.VariableNode, *parse.FieldNode, *parse.ChainNode, *parse.DotNode, *parse.IdentifierNode, *parse.NilNode, *parse.NumberNode, *parse.BoolNode:
		// Pass-through: the verbatim key must be declared at the fusion point.
		return "", true
	default:
		return "non-static translation key (keys must be verbatim string literals at the fusion point)", false
	}
}

func collectI18nKeys(node parse.Node, keys container.Set[string], dynamic *[]string) {
	switch n := node.(type) {
	case *parse.ListNode:
		for _, sub := range n.Nodes {
			collectI18nKeys(sub, keys, dynamic)
		}
	case *parse.TemplateNode:
		if n.Pipe != nil {
			collectI18nKeys(n.Pipe, keys, dynamic)
		}
	case *parse.IfNode:
		collectI18nKeys(n.List, keys, dynamic)
		if n.ElseList != nil {
			collectI18nKeys(n.ElseList, keys, dynamic)
		}
		if n.Pipe != nil {
			collectI18nKeys(n.Pipe, keys, dynamic)
		}
	case *parse.RangeNode:
		collectI18nKeys(n.List, keys, dynamic)
		if n.ElseList != nil {
			collectI18nKeys(n.ElseList, keys, dynamic)
		}
		if n.Pipe != nil {
			collectI18nKeys(n.Pipe, keys, dynamic)
		}
	case *parse.WithNode:
		collectI18nKeys(n.List, keys, dynamic)
		if n.ElseList != nil {
			collectI18nKeys(n.ElseList, keys, dynamic)
		}
		if n.Pipe != nil {
			collectI18nKeys(n.Pipe, keys, dynamic)
		}
	case *parse.ActionNode:
		if n.Pipe != nil {
			collectI18nKeys(n.Pipe, keys, dynamic)
		}
	case *parse.PipeNode:
		for _, cmd := range n.Cmds {
			collectI18nKeys(cmd, keys, dynamic)
		}
	case *parse.CommandNode:
		if len(n.Args) >= 2 {
			funcName := i18nFuncName(n.Args[0])
			if isI18nFunc(funcName) {
				start, needed := 1, 1
				if isI18nTrN(funcName) {
					start, needed = 2, 2 // TrN count, key1, keyN, ...
				}
				got := 0
				for _, arg := range n.Args[start:] {
					if got >= needed {
						break
					}
					got++
					if msg, ok := collectTrArgKeys(arg, keys); !ok {
						*dynamic = append(*dynamic, msg+": "+arg.String())
					}
				}
			}
		}
		for _, arg := range n.Args {
			if p, ok := arg.(*parse.PipeNode); ok {
				collectI18nKeys(p, keys, dynamic)
			}
		}
	}
}

// TemplateI18nScan is the result of scanning a template for translation keys.
type TemplateI18nScan struct {
	Keys     container.Set[string]
	Declared container.Set[string]
	Dynamic  []string
}

func extractI18nKeys(node parse.Node) container.Set[string] {
	keys := container.Set[string]{}
	var dynamic []string
	collectI18nKeys(node, keys, &dynamic)
	return keys
}

func parseTemplateFile(p string) (*template.Template, error) {
	bs, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	// The template parser requires the function map otherwise it will return failure
	funcMap := template.FuncMap{}
	maps.Copy(funcMap, newFuncMapWebPage())
	for name, fn := range mailBodyFuncMap() {
		if _, exists := funcMap[name]; !exists {
			funcMap[name] = fn
		}
	}
	funcMap["ctx"] = func() any { return nil }
	return template.New("test").Funcs(funcMap).Parse(string(bs))
}

func FindTemplateKeys(p string) (container.Set[string], error) {
	t, err := parseTemplateFile(p)
	if err != nil {
		return nil, err
	}
	return extractI18nKeys(t.Root), nil
}

func ScanTemplateI18n(p string) (TemplateI18nScan, error) {
	t, err := parseTemplateFile(p)
	if err != nil {
		return TemplateI18nScan{}, err
	}
	keys := container.Set[string]{}
	var dynamic []string
	collectI18nKeys(t.Root, keys, &dynamic)
	declared := container.Set[string]{}
	collectStringLiterals(t.Root, declared)
	return TemplateI18nScan{Keys: keys, Declared: declared, Dynamic: dynamic}, nil
}
