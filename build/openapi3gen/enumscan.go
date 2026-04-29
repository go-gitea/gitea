// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package openapi3gen converts Gitea's Swagger 2.0 spec to an OpenAPI 3.0
// spec. It discovers Go enum type names by scanning swagger:enum annotations
// in the source tree, then names extracted shared-enum schemas accordingly.
package openapi3gen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// EnumKey returns a canonical key for a set of enum values: values are
// stringified, sorted, and joined with "|". Used to match enum value sets
// across spec properties and scanned Go type declarations.
func EnumKey(values []any) string {
	strs := make([]string, len(values))
	for i, v := range values {
		strs[i] = fmt.Sprintf("%v", v)
	}
	sort.Strings(strs)
	return strings.Join(strs, "|")
}

var rxSwaggerEnum = regexp.MustCompile(`swagger:enum\s+(\w+)`)

// ScanSwaggerEnumTypes walks .go files under each dir and returns a map from
// a canonical value-set key (see EnumKey) to the Go type name declared with
// // swagger:enum TypeName.
//
// Returns an error on parse failure, on an annotation for a type whose
// constants can't be extracted, or on value-set collisions between two
// different enum types.
func ScanSwaggerEnumTypes(dirs []string) (map[string]string, error) {
	fset := token.NewFileSet()
	parsed := []*ast.File{}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			parsed = append(parsed, file)
		}
	}

	enumTypes := map[string]string{} // typeName → "" (presence marker)
	enumValues := map[string][]any{} // typeName → values

	// Pass 1: collect every // swagger:enum TypeName declaration.
	for _, file := range parsed {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			if err := collectEnumType(gd, enumTypes); err != nil {
				return nil, fmt.Errorf("%s: %w", fset.Position(gd.Pos()).Filename, err)
			}
		}
	}

	// Pass 2: collect const values; now every annotated type is visible.
	for _, file := range parsed {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			collectEnumValues(gd, enumTypes, enumValues)
		}
	}

	result := map[string]string{}
	for typeName := range enumTypes {
		values, ok := enumValues[typeName]
		if !ok || len(values) == 0 {
			return nil, fmt.Errorf("swagger:enum %s has no const block with typed string values", typeName)
		}
		key := EnumKey(values)
		if existing, ok := result[key]; ok && existing != typeName {
			return nil, fmt.Errorf("swagger:enum value-set collision: %s and %s both use %q", existing, typeName, key)
		}
		result[key] = typeName
	}
	return result, nil
}

// collectEnumType scans a `type` GenDecl for // swagger:enum annotations,
// handling both the lone form (`// swagger:enum Foo\n type Foo string`)
// where the comment group is attached to the GenDecl, and the grouped form:
//
//	type (
//	    // swagger:enum Foo
//	    Foo string
//	)
//
// where the comment group is attached to each TypeSpec. Caveat: Go's parser
// only attaches a CommentGroup when it is immediately adjacent to the decl.
// A blank line (not a `//` continuation line) between the comment and the
// declaration drops the Doc, so annotations MUST sit directly above their
// type. All current annotated files obey this — the rule is noted here so
// a future edit that inserts a blank line fails fast rather than silently.
func collectEnumType(gd *ast.GenDecl, enumTypes map[string]string) error {
	if err := registerEnumAnnotation(gd.Doc, gd.Specs, enumTypes); err != nil {
		return err
	}
	for _, spec := range gd.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || ts.Doc == nil {
			continue
		}
		if err := registerEnumAnnotation(ts.Doc, []ast.Spec{ts}, enumTypes); err != nil {
			return err
		}
	}
	return nil
}

func registerEnumAnnotation(doc *ast.CommentGroup, specs []ast.Spec, enumTypes map[string]string) error {
	if doc == nil {
		return nil
	}
	matches := rxSwaggerEnum.FindStringSubmatch(doc.Text())
	if len(matches) < 2 {
		return nil
	}
	annotated := matches[1]
	for _, spec := range specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if ts.Name.Name == annotated {
			enumTypes[annotated] = ""
			return nil
		}
	}
	return fmt.Errorf("swagger:enum %s: no type declaration with that name in the same decl group; check for a typo", annotated)
}

func collectEnumValues(gd *ast.GenDecl, enumTypes map[string]string, enumValues map[string][]any) {
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok || vs.Type == nil {
			continue
		}
		ident, ok := vs.Type.(*ast.Ident)
		if !ok {
			continue
		}
		if _, isEnum := enumTypes[ident.Name]; !isEnum {
			continue
		}
		for _, val := range vs.Values {
			lit, ok := val.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			unquoted, err := strconv.Unquote(lit.Value)
			if err != nil {
				continue
			}
			enumValues[ident.Name] = append(enumValues[ident.Name], unquoted)
		}
	}
}
