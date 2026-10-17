// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"gitea.dev/modules/json"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// The client-side diff viewer highlights the languages without a CodeMirror grammar with chroma's regex lexers,
// see web_src/js/modules/diffviewer/chroma.ts for the format.
const (
	outputDir              = "assets/chroma-lexers"
	codemirrorLanguageFile = "assets/codemirror-languages.json"
)

type lexerIndexEntry struct {
	Name      string   `json:"name"`
	File      string   `json:"file"`
	Aliases   []string `json:"aliases,omitempty"`
	Filenames []string `json:"filenames,omitempty"`
}

type lexerFile struct {
	Name     string              `json:"name"`
	Flags    string              `json:"flags"`
	EnsureNL bool                `json:"ensureNL,omitempty"`
	Rules    map[string][][3]any `json:"rules"` // pattern, emitter, mutator
	Using    []string            `json:"using,omitempty"`
	config   *chroma.Config
}

func field(v any, name string) reflect.Value {
	return reflect.Indirect(reflect.ValueOf(v)).FieldByName(name)
}

func tokenClass(t chroma.TokenType) string {
	if t == chroma.Error {
		return "" // partial code like a diff hunk is often not valid, don't mark it
	}
	for ; t != 0; t = t.Parent() {
		if class, ok := chroma.StandardTypes[t]; ok {
			return class
		}
	}
	return ""
}

func convertEmitter(e chroma.Emitter, using *[]string) (any, error) {
	if e == nil {
		return nil, nil
	}
	if t, ok := e.(chroma.TokenType); ok {
		return tokenClass(t), nil
	}
	serialisable, ok := e.(chroma.SerialisableEmitter)
	if !ok {
		return nil, fmt.Errorf("emitter %T is not serialisable", e)
	}
	convertList := func(list reflect.Value) ([]any, error) {
		var out []any
		for i := range list.Len() {
			emitter, _ := list.Index(i).Interface().(chroma.Emitter)
			converted, err := convertEmitter(emitter, using)
			if err != nil {
				return nil, err
			}
			out = append(out, converted)
		}
		return out, nil
	}
	switch kind := serialisable.EmitterKind(); kind {
	case "bygroups":
		groups, err := convertList(field(e, "Emitters"))
		return map[string]any{"g": groups}, err
	case "using":
		name := field(e, "Lexer").String()
		*using = append(*using, name)
		return map[string]any{"u": name}, nil
	case "usingself":
		return map[string]any{"s": field(e, "State").String()}, nil
	case "usingbygroup":
		groups, err := convertList(field(e, "Emitters"))
		return map[string]any{"ug": []any{field(e, "SublexerNameGroup").Int(), field(e, "CodeGroup").Int(), groups}}, err
	default:
		return nil, fmt.Errorf("unknown emitter kind %q", kind)
	}
}

func convertMutator(m chroma.Mutator) (any, error) {
	if m == nil {
		return nil, nil
	}
	serialisable, ok := m.(chroma.SerialisableMutator)
	if !ok {
		return nil, fmt.Errorf("mutator %T is not serialisable", m)
	}
	switch kind := serialisable.MutatorKind(); kind {
	case "push":
		return map[string]any{"push": field(m, "States").Interface()}, nil
	case "pop":
		return map[string]any{"pop": field(m, "Depth").Int()}, nil
	case "include":
		return map[string]any{"include": field(m, "State").String()}, nil
	case "combined":
		return map[string]any{"combined": field(m, "States").Interface()}, nil
	case "mutators":
		var out []any
		list := field(m, "Mutators")
		for i := range list.Len() {
			converted, err := convertMutator(list.Index(i).Interface().(chroma.Mutator))
			if err != nil {
				return nil, err
			}
			out = append(out, converted)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown mutator kind %q", kind)
	}
}

func convertLexer(lexer chroma.Lexer) (*lexerFile, error) {
	regexLexer, ok := lexer.(*chroma.RegexLexer)
	if !ok {
		return nil, fmt.Errorf("%T is not a regex lexer", lexer)
	}
	rules, err := regexLexer.Rules()
	if err != nil {
		return nil, err
	}
	config := lexer.Config()
	file := &lexerFile{Name: config.Name, EnsureNL: config.EnsureNL, Rules: map[string][][3]any{}, config: config}
	if !config.NotMultiline {
		file.Flags += "m"
	}
	if config.CaseInsensitive {
		file.Flags += "i"
	}
	if config.DotAll {
		file.Flags += "s"
	}
	for state, stateRules := range rules {
		for _, rule := range stateRules {
			emitter, err := convertEmitter(rule.Type, &file.Using)
			if err != nil {
				return nil, err
			}
			mutator, err := convertMutator(rule.Mutator)
			if err != nil {
				return nil, err
			}
			file.Rules[state] = append(file.Rules[state], [3]any{rule.Pattern, emitter, mutator})
		}
	}
	slices.Sort(file.Using)
	file.Using = slices.Compact(file.Using)
	return file, nil
}

// codemirrorExtensions returns the file extensions the code editor has a grammar for
func codemirrorExtensions() map[string]bool {
	data, err := os.ReadFile(codemirrorLanguageFile)
	if err != nil {
		log.Fatalf("Unable to read %s: %v", codemirrorLanguageFile, err)
	}
	var languages []struct {
		Extensions []string `json:"extensions"`
	}
	if err := json.Unmarshal(data, &languages); err != nil {
		log.Fatalf("Unable to parse %s: %v", codemirrorLanguageFile, err)
	}
	extensions := map[string]bool{}
	for _, language := range languages {
		for _, ext := range language.Extensions {
			extensions[strings.ToLower(ext)] = true
		}
	}
	return extensions
}

func isCoveredByCodemirror(config *chroma.Config, extensions map[string]bool) bool {
	if len(config.Filenames) == 0 {
		return false
	}
	for _, glob := range config.Filenames {
		ext, ok := strings.CutPrefix(glob, "*.")
		if !ok || strings.ContainsAny(ext, "*?[") || !extensions[strings.ToLower(ext)] {
			return false
		}
	}
	return true
}

func fileName(name string) string {
	return strings.Map(func(r rune) rune {
		if 'a' <= r && r <= 'z' || '0' <= r && r <= '9' {
			return r
		}
		if 'A' <= r && r <= 'Z' {
			return r - 'A' + 'a'
		}
		return '-'
	}, name)
}

func main() {
	extensions := codemirrorExtensions()
	converted := map[string]*lexerFile{}
	for _, lexer := range lexers.GlobalLexerRegistry.Lexers {
		file, err := convertLexer(lexer)
		if err != nil {
			log.Printf("Skipping %s: %v", lexer.Config().Name, err)
			continue
		}
		converted[file.Name] = file
	}

	// the lexers without a CodeMirror grammar, and the ones they delegate to
	wanted := map[string]bool{}
	var want func(name string)
	want = func(name string) {
		file := converted[name]
		if file == nil || wanted[name] {
			return
		}
		wanted[name] = true
		for _, using := range file.Using {
			if lexer := lexers.Get(using); lexer != nil {
				want(lexer.Config().Name)
			}
		}
	}
	for name, file := range converted {
		if !isCoveredByCodemirror(file.config, extensions) {
			want(name)
		}
	}

	if err := os.RemoveAll(outputDir); err != nil {
		log.Fatalf("Unable to clean %s: %v", outputDir, err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("Unable to create %s: %v", outputDir, err)
	}
	var index []lexerIndexEntry
	for name := range wanted {
		file := converted[name]
		entry := lexerIndexEntry{Name: name, File: fileName(name), Aliases: file.config.Aliases, Filenames: append(slices.Clone(file.config.Filenames), file.config.AliasFilenames...)}
		data, err := json.Marshal(file)
		if err != nil {
			log.Fatalf("Unable to marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(outputDir, entry.File+".json"), append(data, '\n'), 0o644); err != nil {
			log.Fatalf("Unable to write %s: %v", name, err)
		}
		index = append(index, entry)
	}
	slices.SortFunc(index, func(a, b lexerIndexEntry) int { return strings.Compare(a.Name, b.Name) })
	data, err := json.Marshal(index)
	if err != nil {
		log.Fatalf("Unable to marshal the index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "index.json"), append(data, '\n'), 0o644); err != nil {
		log.Fatalf("Unable to write the index: %v", err)
	}
	log.Printf("Wrote %d of %d lexers", len(index), len(converted))
}
