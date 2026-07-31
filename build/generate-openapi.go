// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// generate-openapi converts Gitea's Swagger 2.0 spec into an OpenAPI 3.0 spec.
//
// Gitea generates a Swagger 2.0 spec from code annotations (make generate-swagger).
// This tool converts it to OAS3 so that SDK generators and tools that require
// OAS3 (e.g. progenitor for Rust) can consume it directly. The conversion also
// deduplicates inline enum definitions into named schema components, producing
// cleaner SDK output with proper enum types instead of anonymous strings.
//
// Run: go run build/generate-openapi.go
// Output: templates/swagger/v1_openapi3_json.tmpl

//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"gitea.dev/build/openapi3gen"
)

const (
	swaggerSpecPath = "templates/swagger/v1-swagger.generated.json"
	openapi3OutPath = "templates/swagger/v1-openapi3.generated.json"
)

var enumScanDirs = []string{
	"modules/structs",
	"modules/commitstatus",
}

func main() {
	astEnumMap, err := openapi3gen.ScanSwaggerEnumTypes(enumScanDirs)
	if err != nil {
		log.Fatalf("scanning swagger:enum annotations: %v", err)
	}
	names := make([]string, 0, len(astEnumMap))
	for _, ns := range astEnumMap {
		names = append(names, ns...)
	}
	sort.Strings(names)
	fmt.Fprintf(os.Stderr, "discovered %d swagger:enum types: %s\n", len(names), strings.Join(names, ", "))

	data, err := os.ReadFile(swaggerSpecPath)
	if err != nil {
		log.Fatalf("reading swagger spec: %v", err)
	}

	oas3, err := openapi3gen.Convert(data, astEnumMap)
	if err != nil {
		log.Fatalf("converting to openapi 3.0: %v", err)
	}

	out, err := json.MarshalIndent(oas3, "", "  ")
	if err != nil {
		log.Fatalf("marshaling openapi 3.0: %v", err)
	}

	if err := os.WriteFile(openapi3OutPath, out, 0o644); err != nil {
		log.Fatalf("writing openapi 3.0 spec: %v", err)
	}

	fmt.Printf("Generated %s\n", openapi3OutPath)
}
