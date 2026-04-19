// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package openapi3gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnumKey_sortsAndJoins(t *testing.T) {
	key := EnumKey([]any{"b", "a", "c"})
	if key != "a|b|c" {
		t.Fatalf("EnumKey = %q, want %q", key, "a|b|c")
	}
}

func TestEnumKey_handlesNonStringValues(t *testing.T) {
	key := EnumKey([]any{2, 1, 3})
	if key != "1|2|3" {
		t.Fatalf("EnumKey = %q, want %q", key, "1|2|3")
	}
}

func TestScanSwaggerEnumTypes_basic(t *testing.T) {
	dir := t.TempDir()
	src := `package fixture

// Color is a primary color.
// swagger:enum Color
type Color string

const (
	ColorRed   Color = "red"
	ColorGreen Color = "green"
	ColorBlue  Color = "blue"
)
`
	if err := os.WriteFile(filepath.Join(dir, "color.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ScanSwaggerEnumTypes([]string{dir})
	if err != nil {
		t.Fatalf("ScanSwaggerEnumTypes: %v", err)
	}
	wantKey := EnumKey([]any{"red", "green", "blue"})
	if got[wantKey] != "Color" {
		t.Fatalf("map[%q] = %q, want %q", wantKey, got[wantKey], "Color")
	}
}

func TestScanSwaggerEnumTypes_orphanAnnotation(t *testing.T) {
	dir := t.TempDir()
	src := `package fixture

// swagger:enum Sttype
type StateType string

const (
	StateOpen StateType = "open"
)
`
	if err := os.WriteFile(filepath.Join(dir, "typo.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ScanSwaggerEnumTypes([]string{dir})
	if err == nil {
		t.Fatal("expected error for annotation referencing a non-matching type name")
	}
	if !strings.Contains(err.Error(), "Sttype") {
		t.Fatalf("error %q should mention the typo'd name Sttype", err.Error())
	}
}

func TestScanSwaggerEnumTypes_collision(t *testing.T) {
	dir := t.TempDir()
	src := `package fixture

// swagger:enum Alpha
type Alpha string
const (
	AlphaX Alpha = "x"
	AlphaY Alpha = "y"
)

// swagger:enum Beta
type Beta string
const (
	BetaX Beta = "x"
	BetaY Beta = "y"
)
`
	if err := os.WriteFile(filepath.Join(dir, "dup.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ScanSwaggerEnumTypes([]string{dir})
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Alpha") || !strings.Contains(msg, "Beta") {
		t.Fatalf("error %q should mention both Alpha and Beta", msg)
	}
}

func TestScanSwaggerEnumTypes_parseFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte("package fixture\nfunc Foo() {"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ScanSwaggerEnumTypes([]string{dir})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestScanSwaggerEnumTypes_annotationWithoutConsts(t *testing.T) {
	dir := t.TempDir()
	src := `package fixture

// swagger:enum Lonely
type Lonely string
`
	if err := os.WriteFile(filepath.Join(dir, "lonely.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ScanSwaggerEnumTypes([]string{dir})
	if err == nil {
		t.Fatal("expected error for annotation without consts")
	}
	if !strings.Contains(err.Error(), "Lonely") {
		t.Fatalf("error %q should mention Lonely", err.Error())
	}
}
