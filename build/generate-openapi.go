// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

const (
	swaggerSpecPath = "templates/swagger/v1_json.tmpl"
	openapi3OutPath = "templates/swagger/v1_openapi3_json.tmpl"

	// Go template variables in the Swagger spec that must be replaced
	// with placeholders before parsing as JSON.
	appSubUrlVar = "{{.SwaggerAppSubUrl}}"
	appVerVar    = "{{.SwaggerAppVer}}"

	appSubUrlPlaceholder = "GITEA_APP_SUB_URL_PLACEHOLDER"
	appVerPlaceholder    = "0.0.0-gitea-placeholder"
)

var (
	appSubUrlRe = regexp.MustCompile(regexp.QuoteMeta(appSubUrlVar))
	appVerRe    = regexp.MustCompile(regexp.QuoteMeta(appVerVar))
)

func main() {
	data, err := os.ReadFile(swaggerSpecPath)
	if err != nil {
		log.Fatalf("reading swagger spec: %v", err)
	}

	// Replace Go template variables with safe placeholders for JSON parsing
	cleaned := appSubUrlRe.ReplaceAll(data, []byte(appSubUrlPlaceholder))
	cleaned = appVerRe.ReplaceAll(cleaned, []byte(appVerPlaceholder))

	var swagger2 openapi2.T
	if err := json.Unmarshal(cleaned, &swagger2); err != nil {
		log.Fatalf("parsing swagger 2.0: %v", err)
	}

	oas3, err := openapi2conv.ToV3(&swagger2)
	if err != nil {
		log.Fatalf("converting to openapi 3.0: %v", err)
	}

	// Set server URL with the placeholder for AppSubUrl
	oas3.Servers = openapi3.Servers{
		{URL: appSubUrlPlaceholder + "/api/v1"},
	}

	// Fix "type: file" schemas left over from Swagger 2.0 conversion.
	fixFileSchemas(oas3)

	// OAS3 post-processing: enrich the spec with details that Swagger 2.0
	// and go-swagger cannot express.
	addURIFormats(oas3)
	addDeprecatedFlags(oas3)
	extractSharedEnums(oas3)

	out, err := json.MarshalIndent(oas3, "", "  ")
	if err != nil {
		log.Fatalf("marshaling openapi 3.0: %v", err)
	}

	// Re-inject Go template variables
	result := strings.ReplaceAll(string(out), appSubUrlPlaceholder, appSubUrlVar)
	result = strings.ReplaceAll(result, appVerPlaceholder, appVerVar)

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	if err := os.WriteFile(openapi3OutPath, []byte(result), 0o644); err != nil {
		log.Fatalf("writing openapi 3.0 spec: %v", err)
	}

	fmt.Printf("Generated %s\n", openapi3OutPath)
}

func fixFileSchemas(doc *openapi3.T) {
	for _, pathItem := range doc.Paths.Map() {
		for _, op := range []*openapi3.Operation{
			pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Patch,
			pathItem.Delete, pathItem.Head, pathItem.Options, pathItem.Trace,
		} {
			if op == nil {
				continue
			}
			for _, resp := range op.Responses.Map() {
				if resp.Value == nil {
					continue
				}
				for _, mediaType := range resp.Value.Content {
					fixSchema(mediaType.Schema)
				}
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for _, mediaType := range op.RequestBody.Value.Content {
					fixSchema(mediaType.Schema)
				}
			}
		}
	}
}

func fixSchema(ref *openapi3.SchemaRef) {
	if ref == nil || ref.Value == nil {
		return
	}
	if ref.Value.Type.Is("file") {
		ref.Value.Type = &openapi3.Types{"string"}
		ref.Value.Format = "binary"
	}
}

// addURIFormats sets format: uri on string properties whose names indicate
// they hold URLs. This information is lost in Swagger 2.0 but is valuable
// for code generators.
func addURIFormats(doc *openapi3.T) {
	if doc.Components == nil {
		return
	}
	for _, schemaRef := range doc.Components.Schemas {
		if schemaRef.Value == nil {
			continue
		}
		for propName, propRef := range schemaRef.Value.Properties {
			if propRef == nil || propRef.Value == nil || propRef.Ref != "" {
				continue
			}
			prop := propRef.Value
			if !prop.Type.Is("string") || prop.Format != "" {
				continue
			}
			if isURLProperty(propName) {
				prop.Format = "uri"
			}
		}
	}
}

func isURLProperty(name string) bool {
	if strings.HasSuffix(name, "_url") {
		return true
	}
	switch name {
	case "url", "html_url", "clone_url":
		return true
	}
	return false
}

// addDeprecatedFlags sets deprecated: true on schema properties whose
// description contains "deprecated".
func addDeprecatedFlags(doc *openapi3.T) {
	if doc.Components == nil {
		return
	}
	for _, schemaRef := range doc.Components.Schemas {
		if schemaRef.Value == nil {
			continue
		}
		for _, propRef := range schemaRef.Value.Properties {
			if propRef == nil || propRef.Value == nil || propRef.Ref != "" {
				continue
			}
			desc := strings.ToLower(propRef.Value.Description)
			if strings.Contains(desc, "deprecated") {
				propRef.Value.Deprecated = true
			}
		}
	}
}

type enumUsage struct {
	schemaName string
	propName   string
	propRef    *openapi3.SchemaRef
	inItems    bool
}

// extractSharedEnums finds identical enum arrays used by multiple schema
// properties, creates a standalone named schema for each, and replaces
// the inline enums with $ref pointers.
func extractSharedEnums(doc *openapi3.T) {
	if doc.Components == nil {
		return
	}

	enumGroups := map[string][]enumUsage{}

	for schemaName, schemaRef := range doc.Components.Schemas {
		if schemaRef.Value == nil {
			continue
		}
		for propName, propRef := range schemaRef.Value.Properties {
			if propRef == nil || propRef.Value == nil || propRef.Ref != "" {
				continue
			}
			if len(propRef.Value.Enum) > 1 && propRef.Value.Type.Is("string") {
				key := enumKey(propRef.Value.Enum)
				enumGroups[key] = append(enumGroups[key], enumUsage{schemaName, propName, propRef, false})
			}
			if propRef.Value.Type.Is("array") && propRef.Value.Items != nil &&
				propRef.Value.Items.Value != nil && propRef.Value.Items.Ref == "" &&
				len(propRef.Value.Items.Value.Enum) > 1 && propRef.Value.Items.Value.Type.Is("string") {
				key := enumKey(propRef.Value.Items.Value.Enum)
				enumGroups[key] = append(enumGroups[key], enumUsage{schemaName, propName, propRef, true})
			}
		}
	}

	for key, usages := range enumGroups {
		if len(usages) < 2 {
			continue
		}

		enumName := deriveEnumName(usages)
		if _, exists := doc.Components.Schemas[enumName]; exists {
			enumName += "Type"
		}
		if _, exists := doc.Components.Schemas[enumName]; exists {
			continue
		}

		var enumValues []any
		if usages[0].inItems {
			enumValues = usages[0].propRef.Value.Items.Value.Enum
		} else {
			enumValues = usages[0].propRef.Value.Enum
		}

		doc.Components.Schemas[enumName] = &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
				Enum: enumValues,
			},
		}

		ref := "#/components/schemas/" + enumName

		for _, usage := range usages {
			if usage.inItems {
				usage.propRef.Value.Items = &openapi3.SchemaRef{Ref: ref}
			} else {
				old := usage.propRef.Value
				if old.Description == "" && !old.Deprecated && old.Format == "" {
					usage.propRef.Ref = ref
					usage.propRef.Value = nil
				} else {
					usage.propRef.Value = &openapi3.Schema{
						AllOf: openapi3.SchemaRefs{
							{Ref: ref},
						},
						Description: old.Description,
						Deprecated:  old.Deprecated,
					}
				}
			}
		}

		_ = key
	}
}

func enumKey(values []any) string {
	strs := make([]string, len(values))
	for i, v := range values {
		strs[i] = fmt.Sprintf("%v", v)
	}
	sort.Strings(strs)
	return strings.Join(strs, "|")
}

var knownEnumTypes = map[string]string{
	"CommitStatus":     "CommitStatusState",
	"State":            "StateType",
	"ReviewState":      "ReviewStateType",
	"NotifySubject":    "NotifySubjectType",
	"IssueFormField":   "IssueFormFieldType",
	"ObjectFormatName": "ObjectFormatName",
}

func deriveEnumName(usages []enumUsage) string {
	for _, u := range usages {
		if u.propRef.Value == nil {
			continue
		}
		desc, ok := u.propRef.Value.Extensions["x-go-enum-desc"]
		if !ok {
			continue
		}
		s, ok := desc.(string)
		if !ok {
			continue
		}
		parts := strings.Fields(s)
		if len(parts) < 2 {
			continue
		}
		constName := parts[1]

		var vals []any
		if u.inItems {
			vals = u.propRef.Value.Items.Value.Enum
		} else {
			vals = u.propRef.Value.Enum
		}
		for _, v := range vals {
			vs := fmt.Sprintf("%v", v)
			lowerConst := strings.ToLower(constName)
			lowerVal := strings.ToLower(vs)
			if strings.HasSuffix(lowerConst, lowerVal) && len(lowerVal) < len(lowerConst) {
				prefix := constName[:len(constName)-len(vs)]
				if goType, ok := knownEnumTypes[prefix]; ok {
					return goType
				}
				return prefix
			}
		}
	}

	nameCounts := map[string]int{}
	for _, u := range usages {
		nameCounts[u.propName]++
	}
	bestName := ""
	bestCount := 0
	for name, count := range nameCounts {
		if count > bestCount || (count == bestCount && name < bestName) {
			bestName = name
			bestCount = count
		}
	}
	result := ""
	for _, p := range strings.Split(bestName, "_") {
		if len(p) > 0 {
			result += strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return result + "Enum"
}
