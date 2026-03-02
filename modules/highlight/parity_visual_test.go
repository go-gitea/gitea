//go:build highlight_visual_parity
// +build highlight_visual_parity

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"fmt"
	stdhtml "html"
	"io"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/net/html"
)

type visualParitySample struct {
	name     string
	fileName string
	code     string
}

type visualParityResult struct {
	name          string
	mismatchRate  float64
	mismatchCount int
	comparedCount int
}

var visualParityTop50 = []visualParitySample{
	{name: "go", fileName: "sample.go", code: "package main\n\nfunc main() { println(1) }\n"},
	{name: "python", fileName: "sample.py", code: "def f(x):\n    return x + 1\n"},
	{name: "javascript", fileName: "sample.js", code: "export function f(x) { return x + 1; }\n"},
	{name: "typescript", fileName: "sample.ts", code: "export function f(x: number): number { return x + 1 }\n"},
	{name: "tsx", fileName: "sample.tsx", code: "export function App() { return <div>Hello</div> }\n"},
	{name: "java", fileName: "Sample.java", code: "class A { int f() { return 1; } }\n"},
	{name: "c", fileName: "sample.c", code: "int main(void) { int a = 1; return a; }\n"},
	{name: "cpp", fileName: "sample.cpp", code: "int main() { auto x = 1; return x; }\n"},
	{name: "csharp", fileName: "sample.cs", code: "class A { int F() => 1; }\n"},
	{name: "rust", fileName: "sample.rs", code: "fn main() { let x: i32 = 1; println!(\"{}\", x); }\n"},
	{name: "php", fileName: "sample.php", code: "<?php function f($x){ return $x + 1; }\n"},
	{name: "ruby", fileName: "sample.rb", code: "def f(x)\n  x + 1\nend\n"},
	{name: "sql", fileName: "sample.sql", code: "SELECT id, name FROM users WHERE id > 10;\n"},
	{name: "html", fileName: "sample.html", code: "<!doctype html><html><body><h1>Hello</h1></body></html>\n"},
	{name: "css", fileName: "sample.css", code: "body { color: #333; margin: 0; }\n"},
	{name: "json", fileName: "sample.json", code: "{\"a\":1,\"b\":[2,3]}\n"},
	{name: "yaml", fileName: "sample.yaml", code: "a: 1\nb:\n  - 2\n"},
	{name: "toml", fileName: "sample.toml", code: "name = \"x\"\ncount = 2\n"},
	{name: "bash", fileName: "sample.sh", code: "#!/usr/bin/env bash\necho \"$HOME\"\n"},
	{name: "markdown", fileName: "sample.md", code: "# Title\n\n`code`\n"},
	{name: "lua", fileName: "sample.lua", code: "local x = 1\nprint(x)\n"},
	{name: "kotlin", fileName: "sample.kt", code: "fun f(x: Int): Int = x + 1\n"},
	{name: "swift", fileName: "sample.swift", code: "func f(_ x: Int) -> Int { x + 1 }\n"},
	{name: "scala", fileName: "sample.scala", code: "object A { def f(x:Int) = x + 1 }\n"},
	{name: "dart", fileName: "sample.dart", code: "int f(int x) => x + 1;\n"},
	{name: "hcl", fileName: "sample.hcl", code: "resource \"x\" \"y\" { enabled = true }\n"},
	{name: "dockerfile", fileName: "Dockerfile", code: "FROM alpine:3.20\nRUN echo hi\n"},
	{name: "graphql", fileName: "sample.graphql", code: "type User { id: ID! name: String! }\n"},
	{name: "xml", fileName: "sample.xml", code: "<root><item k=\"v\"/></root>\n"},
	{name: "ini", fileName: "sample.ini", code: "[x]\na=1\n"},
	{name: "make", fileName: "Makefile", code: "all:\n\t@echo hi\n"},
	{name: "cmake", fileName: "CMakeLists.txt", code: "cmake_minimum_required(VERSION 3.10)\nproject(x)\n"},
	{name: "nix", fileName: "sample.nix", code: "{ pkgs ? import <nixpkgs> {} }:\n\"ok\"\n"},
	{name: "elixir", fileName: "sample.ex", code: "defmodule A do\n  def f(x), do: x + 1\nend\n"},
	{name: "erlang", fileName: "sample.erl", code: "-module(a).\n-export([f/1]).\nf(X) -> X + 1.\n"},
	{name: "ocaml", fileName: "sample.ml", code: "let f x = x + 1\n"},
	{name: "haskell", fileName: "sample.hs", code: "f x = x + 1\n"},
	{name: "r", fileName: "sample.r", code: "f <- function(x) x + 1\n"},
	{name: "julia", fileName: "sample.jl", code: "f(x) = x + 1\n"},
	{name: "perl", fileName: "sample.pl", code: "sub f { my ($x) = @_; return $x + 1; }\n"},
	{name: "powershell", fileName: "sample.ps1", code: "function f($x) { return $x + 1 }\n"},
	{name: "vue", fileName: "sample.vue", code: "<template><div>{{ msg }}</div></template>\n<script setup>const msg = 'hi'</script>\n"},
	{name: "svelte", fileName: "sample.svelte", code: "<script>let x = 1;</script><h1>{x}</h1>\n"},
	{name: "proto", fileName: "sample.proto", code: "syntax = \"proto3\";\nmessage X { int32 id = 1; }\n"},
	{name: "rego", fileName: "sample.rego", code: "package p\nallow if input.user == \"admin\"\n"},
	{name: "json5", fileName: "sample.json5", code: "{a: 1, b: 'x'}\n"},
	{name: "zig", fileName: "sample.zig", code: "pub fn main() void { const x: i32 = 1; _ = x; }\n"},
	{name: "solidity", fileName: "sample.sol", code: "contract X { function f() public pure returns(uint){ return 1; } }\n"},
	{name: "nginx", fileName: "nginx.conf", code: "server { listen 80; location / { return 200; } }\n"},
	{name: "diff", fileName: "sample.diff", code: "--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n"},
}

func canonicalClass(class string) string {
	class = strings.TrimSpace(class)
	switch {
	case class == "", class == "added-code", class == "removed-code":
		return ""
	case strings.HasPrefix(class, "k"):
		return "keyword"
	case strings.HasPrefix(class, "c"):
		return "comment"
	case strings.HasPrefix(class, "s"):
		return "string"
	case strings.HasPrefix(class, "m"):
		return "number"
	case strings.HasPrefix(class, "o"):
		return "operator"
	case strings.HasPrefix(class, "p"):
		return "punct"
	case class == "err":
		return "error"
	default:
		return "name"
	}
}

func htmlClassTrack(rendered string) ([]rune, []string, error) {
	var text []rune
	var classes []string
	var classStack []string

	z := html.NewTokenizer(strings.NewReader(rendered))
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); err != nil && err != io.EOF {
				return nil, nil, err
			}
			return text, classes, nil
		case html.StartTagToken:
			tag, hasAttr := z.TagName()
			if string(tag) != "span" {
				continue
			}
			class := ""
			for hasAttr {
				k, v, more := z.TagAttr()
				if string(k) == "class" {
					class = string(v)
				}
				hasAttr = more
			}
			classStack = append(classStack, canonicalClass(class))
		case html.EndTagToken:
			tag, _ := z.TagName()
			if string(tag) == "span" && len(classStack) > 0 {
				classStack = classStack[:len(classStack)-1]
			}
		case html.TextToken:
			raw := stdhtml.UnescapeString(string(z.Text()))
			class := ""
			if len(classStack) > 0 {
				class = classStack[len(classStack)-1]
			}
			for _, r := range raw {
				text = append(text, r)
				classes = append(classes, class)
			}
		}
	}
}

func compareRenderedParity(treeSitterHTML, chromaHTML string) (visualParityResult, error) {
	tsText, tsClass, err := htmlClassTrack(treeSitterHTML)
	if err != nil {
		return visualParityResult{}, err
	}
	chText, chClass, err := htmlClassTrack(chromaHTML)
	if err != nil {
		return visualParityResult{}, err
	}

	trimTrailingNewline := func(text []rune, classes []string) ([]rune, []string) {
		if len(text) > 0 && text[len(text)-1] == '\n' {
			return text[:len(text)-1], classes[:len(classes)-1]
		}
		return text, classes
	}
	tsText, tsClass = trimTrailingNewline(tsText, tsClass)
	chText, chClass = trimTrailingNewline(chText, chClass)

	if len(tsText) != len(chText) {
		return visualParityResult{}, fmt.Errorf("text length mismatch: ts=%d chroma=%d", len(tsText), len(chText))
	}
	for i := range tsText {
		if tsText[i] != chText[i] {
			return visualParityResult{}, fmt.Errorf("text mismatch at rune %d: ts=%q chroma=%q", i, string(tsText[i]), string(chText[i]))
		}
	}

	compared := 0
	mismatch := 0
	for i := range tsText {
		r := tsText[i]
		if unicode.IsSpace(r) {
			continue
		}
		if tsClass[i] == "" && chClass[i] == "" {
			continue
		}
		compared++
		if tsClass[i] != chClass[i] {
			mismatch++
		}
	}

	rate := 0.0
	if compared > 0 {
		rate = float64(mismatch) / float64(compared)
	}
	return visualParityResult{
		mismatchRate:  rate,
		mismatchCount: mismatch,
		comparedCount: compared,
	}, nil
}

func TestTreeSitterVisualParityTop50(t *testing.T) {
	executed := 0
	skipped := 0
	totalCompared := 0
	totalMismatch := 0
	maxRate := 0.0
	highMismatchCases := 0
	highMismatchThreshold := 0.30

	for _, tc := range visualParityTop50 {
		t.Run(tc.name, func(t *testing.T) {
			tsRendered, _, ok := tryRenderCodeByTreeSitter(tc.fileName, "", []byte(tc.code), true)
			if !ok {
				skipped++
				t.Skip("tree-sitter renderer unavailable")
			}

			lexer := detectChromaLexerWithAnalyze(tc.fileName, "", []byte(tc.code))
			if lexer == nil || lexer.Config().Name == "fallback" {
				skipped++
				t.Skip("chroma lexer unavailable")
			}
			chromaRendered := renderCodeByChromaLexer(lexer, tc.code)

			result, err := compareRenderedParity(string(tsRendered), string(chromaRendered))
			if err != nil {
				t.Fatalf("parity compare failed: %v", err)
			}
			result.name = tc.name
			executed++
			totalCompared += result.comparedCount
			totalMismatch += result.mismatchCount
			if result.mismatchRate > maxRate {
				maxRate = result.mismatchRate
			}
			if result.mismatchRate > highMismatchThreshold {
				highMismatchCases++
			}
			t.Logf("mismatch_rate=%.2f%% mismatches=%d/%d", result.mismatchRate*100, result.mismatchCount, result.comparedCount)
		})
	}

	if executed < 30 {
		t.Fatalf("insufficient parity coverage: executed=%d skipped=%d", executed, skipped)
	}
	avgRate := 0.0
	if totalCompared > 0 {
		avgRate = float64(totalMismatch) / float64(totalCompared)
	}
	t.Logf("visual parity summary: executed=%d skipped=%d avg_mismatch=%.2f%% max_mismatch=%.2f%% high_mismatch_cases=%d",
		executed, skipped, avgRate*100, maxRate*100, highMismatchCases)

	// Guardrails: keep broad parity drift bounded.
	if avgRate > 0.20 {
		t.Fatalf("average mismatch rate too high: %.2f%%", avgRate*100)
	}
	if highMismatchCases > 8 {
		t.Fatalf("too many high-mismatch cases: %d", highMismatchCases)
	}
}
