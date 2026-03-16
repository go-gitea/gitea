// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"fmt"
	"strings"
	"testing"
)

type benchLang struct {
	name       string
	fileName   string
	lang       string
	makeCode   func(n int) string
	commentFmt string // format string for cache-busting comment, must contain %d
}

var benchLangs = []benchLang{
	{
		name: "Go", fileName: "bench.go", lang: "Go", commentFmt: "// %d\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			sb.WriteString("package main\n\nimport \"fmt\"\n\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "func f%d() int { v := %d; fmt.Println(v); return v }\n", i, i)
			}
			return sb.String()
		},
	},
	{
		name: "Python", fileName: "bench.py", lang: "Python", commentFmt: "# %d\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			sb.WriteString("import os\nimport sys\n\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "def func_%d(x: int, y: str = \"hello\") -> int:\n    \"\"\"Docstring for func_%d.\"\"\"\n    result = x + %d\n    if result > 100:\n        return result * 2\n    return result\n\n", i, i, i)
			}
			return sb.String()
		},
	},
	{
		name: "JavaScript", fileName: "bench.js", lang: "JavaScript", commentFmt: "// %d\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			sb.WriteString("'use strict';\n\nconst utils = require('./utils');\n\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "function func_%d(x, y = 'default') {\n  const result = x + %d;\n  if (result > 100) { return result * 2; }\n  return result;\n}\n\n", i, i)
			}
			return sb.String()
		},
	},
	{
		name: "TypeScript", fileName: "bench.ts", lang: "TypeScript", commentFmt: "// %d\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			sb.WriteString("import { Component } from '@angular/core';\n\ninterface Result { value: number; label: string; }\n\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "export function func_%d(x: number, y: string = 'hi'): Result {\n  const value = x + %d;\n  return { value, label: y };\n}\n\n", i, i)
			}
			return sb.String()
		},
	},
	{
		name: "C", fileName: "bench.c", lang: "C", commentFmt: "// %d\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			sb.WriteString("#include <stdio.h>\n#include <stdlib.h>\n\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "int func_%d(int x, int y) {\n    int result = x + %d;\n    if (result > 100) { return result * 2; }\n    printf(\"%%d\\n\", result);\n    return result;\n}\n\n", i, i)
			}
			return sb.String()
		},
	},
	{
		name: "Rust", fileName: "bench.rs", lang: "Rust", commentFmt: "// %d\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			sb.WriteString("use std::collections::HashMap;\n\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "pub fn func_%d(x: i32, y: &str) -> i32 {\n    let result = x + %d;\n    if result > 100 { return result * 2; }\n    println!(\"{}: {}\", y, result);\n    result\n}\n\n", i, i)
			}
			return sb.String()
		},
	},
	{
		name: "Java", fileName: "Bench.java", lang: "Java", commentFmt: "// %d\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			sb.WriteString("package com.example;\n\nimport java.util.List;\nimport java.util.ArrayList;\n\npublic class Bench {\n\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "    public static int func_%d(int x, String y) {\n        int result = x + %d;\n        if (result > 100) { return result * 2; }\n        System.out.println(y + \": \" + result);\n        return result;\n    }\n\n", i, i)
			}
			sb.WriteString("}\n")
			return sb.String()
		},
	},
	{
		name: "Ruby", fileName: "bench.rb", lang: "Ruby", commentFmt: "# %d\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			sb.WriteString("require 'json'\nrequire 'net/http'\n\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "def func_%d(x, y = 'default')\n  result = x + %d\n  return result * 2 if result > 100\n  puts \"#{y}: #{result}\"\n  result\nend\n\n", i, i)
			}
			return sb.String()
		},
	},
	{
		name: "CSS", fileName: "bench.css", lang: "CSS", commentFmt: "/* %d */\n",
		makeCode: func(n int) string {
			var sb strings.Builder
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, ".component-%d {\n  color: #%02x%02x%02x;\n  margin: %dpx %dpx;\n  display: flex;\n  align-items: center;\n  font-size: %drem;\n}\n\n", i, i%256, (i*7)%256, (i*13)%256, i%20, (i*3)%20, i%4+1)
			}
			return sb.String()
		},
	},
}

func benchmarkRenderCodeChroma(b *testing.B, bl benchLang) {
	code := bl.makeCode(200)
	lexer := DetectChromaLexerByFileName(bl.fileName, "")
	if lexer == nil {
		b.Fatalf("chroma lexer not found for %s", bl.name)
	}

	b.ResetTimer()
	for b.Loop() {
		renderCodeByChromaLexer(lexer, code)
	}
}

func benchmarkRenderCodeTreeSitter(b *testing.B, bl benchLang) {
	entry := resolveTreeSitterEntry(bl.fileName, bl.lang)
	if entry == nil {
		b.Skipf("tree-sitter entry unavailable for %s", bl.name)
	}
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		b.Skipf("tree-sitter renderer unavailable for %s", bl.name)
	}

	base := bl.makeCode(200)

	b.ResetTimer()
	i := 0
	for b.Loop() {
		code := []byte(base + fmt.Sprintf(bl.commentFmt, i))
		result, ok := renderer.render(code, true)
		if !ok || result == "" {
			b.Fatalf("tree-sitter render failed for %s", bl.name)
		}
		i++
	}
}

func benchmarkRenderCodeTreeSitterCached(b *testing.B, bl benchLang) {
	entry := resolveTreeSitterEntry(bl.fileName, bl.lang)
	if entry == nil {
		b.Skipf("tree-sitter entry unavailable for %s", bl.name)
	}
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		b.Skipf("tree-sitter renderer unavailable for %s", bl.name)
	}

	code := []byte(bl.makeCode(200))
	if _, ok := renderer.render(code, true); !ok {
		b.Fatalf("tree-sitter warmup failed for %s", bl.name)
	}

	b.ResetTimer()
	for b.Loop() {
		result, ok := renderer.render(code, true)
		if !ok || result == "" {
			b.Fatalf("tree-sitter cached render failed for %s", bl.name)
		}
	}
}

func BenchmarkRenderCode_Chroma(b *testing.B) {
	for _, bl := range benchLangs {
		b.Run(bl.name, func(b *testing.B) { benchmarkRenderCodeChroma(b, bl) })
	}
}

func BenchmarkRenderCode_TreeSitter(b *testing.B) {
	for _, bl := range benchLangs {
		b.Run(bl.name, func(b *testing.B) { benchmarkRenderCodeTreeSitter(b, bl) })
	}
}

func BenchmarkRenderCode_TreeSitter_Cached(b *testing.B) {
	for _, bl := range benchLangs {
		b.Run(bl.name, func(b *testing.B) { benchmarkRenderCodeTreeSitterCached(b, bl) })
	}
}
