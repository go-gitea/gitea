// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package openapi3gen

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestDeriveEnumName_hit(t *testing.T) {
	key := EnumKey([]any{"red", "green", "blue"})
	astMap := map[string]string{key: "Color"}
	usages := []enumUsage{{schemaName: "Paint", propName: "color"}}
	got, err := deriveEnumName(key, usages, astMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Color" {
		t.Fatalf("got %q, want %q", got, "Color")
	}
}

func TestDeriveEnumName_miss(t *testing.T) {
	key := EnumKey([]any{"x", "y"})
	usages := []enumUsage{{schemaName: "Thing", propName: "kind"}}
	_, err := deriveEnumName(key, usages, map[string]string{})
	if err == nil {
		t.Fatal("expected miss error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Thing.kind") {
		t.Fatalf("error %q should list the missing usage", msg)
	}
	if !strings.Contains(msg, "swagger:enum") {
		t.Fatalf("error %q should hint at the fix", msg)
	}
}

func TestExtractSharedEnums_usesASTMap(t *testing.T) {
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"A": {Value: &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: openapi3.Schemas{
						"color": {Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
							Enum: []any{"red", "green", "blue"},
						}},
					},
				}},
				"B": {Value: &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: openapi3.Schemas{
						"color": {Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
							Enum: []any{"red", "green", "blue"},
						}},
					},
				}},
			},
		},
	}
	astMap := map[string]string{EnumKey([]any{"red", "green", "blue"}): "Color"}
	if err := extractSharedEnums(doc, astMap); err != nil {
		t.Fatalf("extractSharedEnums: %v", err)
	}
	if _, ok := doc.Components.Schemas["Color"]; !ok {
		t.Fatalf("expected Color schema to be extracted")
	}
}

func TestFixFileSchemas_recursesIntoNested(t *testing.T) {
	fileType := func() *openapi3.SchemaRef {
		return &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"file"}}}
	}
	doc := &openapi3.T{
		Paths: openapi3.NewPaths(),
	}
	doc.Paths.Set("/upload", &openapi3.PathItem{
		Post: &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{
						"multipart/form-data": {
							Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
								Type: &openapi3.Types{"object"},
								Properties: openapi3.Schemas{
									"attachment": fileType(),
									"items": {Value: &openapi3.Schema{
										Type:  &openapi3.Types{"array"},
										Items: fileType(),
									}},
									"alt": {Value: &openapi3.Schema{
										AllOf: openapi3.SchemaRefs{fileType()},
									}},
									"one": {Value: &openapi3.Schema{
										OneOf: openapi3.SchemaRefs{fileType()},
									}},
									"any": {Value: &openapi3.Schema{
										AnyOf: openapi3.SchemaRefs{fileType()},
									}},
									"not": {Value: &openapi3.Schema{
										Not: fileType(),
									}},
								},
							}},
						},
					},
				},
			},
			Responses: openapi3.NewResponses(),
		},
	})

	fixFileSchemas(doc)

	props := doc.Paths.Value("/upload").Post.RequestBody.Value.Content["multipart/form-data"].Schema.Value.Properties
	if !props["attachment"].Value.Type.Is("string") || props["attachment"].Value.Format != "binary" {
		t.Errorf("nested property not fixed: %+v", props["attachment"].Value)
	}
	if !props["items"].Value.Items.Value.Type.Is("string") || props["items"].Value.Items.Value.Format != "binary" {
		t.Errorf("array items not fixed: %+v", props["items"].Value.Items.Value)
	}
	if !props["alt"].Value.AllOf[0].Value.Type.Is("string") || props["alt"].Value.AllOf[0].Value.Format != "binary" {
		t.Errorf("allOf branch not fixed: %+v", props["alt"].Value.AllOf[0].Value)
	}
	if !props["one"].Value.OneOf[0].Value.Type.Is("string") || props["one"].Value.OneOf[0].Value.Format != "binary" {
		t.Errorf("oneOf branch not fixed: %+v", props["one"].Value.OneOf[0].Value)
	}
	if !props["any"].Value.AnyOf[0].Value.Type.Is("string") || props["any"].Value.AnyOf[0].Value.Format != "binary" {
		t.Errorf("anyOf branch not fixed: %+v", props["any"].Value.AnyOf[0].Value)
	}
	if !props["not"].Value.Not.Value.Type.Is("string") || props["not"].Value.Not.Value.Format != "binary" {
		t.Errorf("not branch not fixed: %+v", props["not"].Value.Not.Value)
	}
}

func TestExtractSharedEnums_missReturnsError(t *testing.T) {
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"A": {Value: &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: openapi3.Schemas{
						"color": {Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
							Enum: []any{"red", "green"},
						}},
					},
				}},
				"B": {Value: &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: openapi3.Schemas{
						"color": {Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
							Enum: []any{"red", "green"},
						}},
					},
				}},
			},
		},
	}
	if err := extractSharedEnums(doc, map[string]string{}); err == nil {
		t.Fatal("expected miss error")
	}
}
