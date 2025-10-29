// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"
)

func TestValue_parse(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		key    string
		valStr string
		want   bool
	}{
		{
			name:   "Parse Invert Retrieval",
			key:    "picture.disable_gravatar",
			valStr: "false",
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := ValueJSON[bool]("picture.disable_gravatar").Invert()
			got := value.parse(tt.key, tt.valStr)

			if got != tt.want {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValue_getKey(t *testing.T) {
	tests := []struct {
		name       string // description of this test case
		valueClass *Value[bool]
		want       string
	}{
		{
			name:       "Custom dynKey name",
			valueClass: ValueJSON[bool]("picture.enable_gravatar").SelectFrom("picture.disable_gravatar"),
			want:       "picture.disable_gravatar",
		},
		{
			name:       "Normal dynKey name",
			valueClass: ValueJSON[bool]("picture.disable_gravatar"),
			want:       "picture.disable_gravatar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.valueClass.getKey()

			if got != tt.want {
				t.Errorf("getKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValue_invert(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		valueClass *Value[bool]
		want       bool
	}{
		{
			name:       "Invert typed true",
			valueClass: ValueJSON[bool]("picture.enable_gravatar").WithDefault(true).Invert(),
			want:       false,
		},
		{
			name:       "Invert typed false",
			valueClass: ValueJSON[bool]("picture.enable_gravatar").WithDefault(false).Invert(),
			want:       true,
		},
		{
			name:       "Invert typed Does not invert",
			valueClass: ValueJSON[bool]("picture.enable_gravatar").WithDefault(false),
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.valueClass.invert(tt.valueClass.def)

			if got != tt.want {
				t.Errorf("invert() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValue_invertBoolStr(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		valueClass *Value[bool]
		val        string
		want       string
	}{
		{
			name:       "Invert boolean string true",
			valueClass: ValueJSON[bool]("picture.enable_gravatar"),
			val:        "true",
			want:       "false",
		},
		{
			name:       "Invert boolean string false",
			valueClass: ValueJSON[bool]("picture.enable_gravatar"),
			val:        "false",
			want:       "true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.valueClass.invertBoolStr(tt.val)
			if got != tt.want {
				t.Errorf("invertBoolStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValue_SelectFromKey(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		valueClass *Value[bool]
		want       string
	}{
		{
			name:       "SelectFrom set and get",
			valueClass: ValueJSON[bool]("picture.enable_gravatar").SelectFrom("picture.disable_gravatar"),
			want:       "picture.disable_gravatar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.valueClass.SelectFromKey()

			if got != tt.want {
				t.Errorf("SelectFromKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
