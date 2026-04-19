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
	enumTypes := map[string]string{} // typeName → canonical key
	enumValues := map[string][]any{} // typeName → values

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
			if err := scanFile(fset, path, enumTypes, enumValues); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
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

func scanFile(fset *token.FileSet, path string, enumTypes map[string]string, enumValues map[string][]any) error {
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch gd.Tok {
		case token.TYPE:
			collectEnumType(gd, enumTypes)
		case token.CONST:
			collectEnumValues(gd, enumTypes, enumValues)
		}
	}
	return nil
}

func collectEnumType(gd *ast.GenDecl, enumTypes map[string]string) {
	if gd.Doc == nil {
		return
	}
	matches := rxSwaggerEnum.FindStringSubmatch(gd.Doc.Text())
	if len(matches) < 2 {
		return
	}
	annotated := matches[1]
	for _, spec := range gd.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if ts.Name.Name == annotated {
			enumTypes[annotated] = ""
		}
	}
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
			unquoted := strings.Trim(lit.Value, "\"")
			enumValues[ident.Name] = append(enumValues[ident.Name], unquoted)
		}
	}
}
