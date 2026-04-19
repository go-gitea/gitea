// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package openapi3gen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

// Convert parses a Swagger 2.0 spec and returns an OAS3 spec, applying
// Gitea-specific post-processing: file-schema fixups, URI formats,
// deprecated flags, and shared-enum extraction.
//
// astEnumMap is a value-set-key → Go-type-name map (built by
// ScanSwaggerEnumTypes). If a shared enum in the spec has no entry in the
// map, Convert returns an error — no fallback naming.
func Convert(swaggerJSON []byte, astEnumMap map[string]string) (*openapi3.T, error) {
	var swagger2 openapi2.T
	if err := json.Unmarshal(swaggerJSON, &swagger2); err != nil {
		return nil, fmt.Errorf("parsing swagger 2.0: %w", err)
	}

	oas3, err := openapi2conv.ToV3(&swagger2)
	if err != nil {
		return nil, fmt.Errorf("converting to openapi 3.0: %w", err)
	}

	fixFileSchemas(oas3)
	addURIFormats(oas3)
	addDeprecatedFlags(oas3)
	if err := extractSharedEnums(oas3, astEnumMap); err != nil {
		return nil, err
	}
	return oas3, nil
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
func extractSharedEnums(doc *openapi3.T, astEnumMap map[string]string) error {
	if doc.Components == nil {
		return nil
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
				key := EnumKey(propRef.Value.Enum)
				enumGroups[key] = append(enumGroups[key], enumUsage{schemaName, propName, propRef, false})
			}
			if propRef.Value.Type.Is("array") && propRef.Value.Items != nil &&
				propRef.Value.Items.Value != nil && propRef.Value.Items.Ref == "" &&
				len(propRef.Value.Items.Value.Enum) > 1 && propRef.Value.Items.Value.Type.Is("string") {
				key := EnumKey(propRef.Value.Items.Value.Enum)
				enumGroups[key] = append(enumGroups[key], enumUsage{schemaName, propName, propRef, true})
			}
		}
	}

	for key, usages := range enumGroups {
		if len(usages) < 2 {
			continue
		}

		enumName, err := deriveEnumName(key, usages, astEnumMap)
		if err != nil {
			return err
		}
		if _, exists := doc.Components.Schemas[enumName]; exists {
			return fmt.Errorf("enum name collision: %s already exists as a component schema", enumName)
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
	}
	return nil
}

// deriveEnumName looks up a shared enum's Go type name from astEnumMap by
// value-set key. If no annotation matches, returns an error identifying the
// offending properties and the fix.
func deriveEnumName(key string, usages []enumUsage, astEnumMap map[string]string) (string, error) {
	if name, ok := astEnumMap[key]; ok {
		return name, nil
	}

	props := map[string]bool{}
	for _, u := range usages {
		props[fmt.Sprintf("%s.%s", u.schemaName, u.propName)] = true
	}
	propList := make([]string, 0, len(props))
	for p := range props {
		propList = append(propList, p)
	}
	return "", fmt.Errorf(
		"no swagger:enum annotation matches value-set %q used by %d properties: %v; "+
			"fix by adding a named string type with // swagger:enum to modules/structs or modules/commitstatus",
		key, len(usages), propList,
	)
}
