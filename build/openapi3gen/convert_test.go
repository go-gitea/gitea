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
