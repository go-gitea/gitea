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
			value := ValueJSON[bool]("picture.disable_gravatar").WithFileConfig(CfgSecKey{Sec: "picture", Key: "DISABLE_GRAVATAR"}).Invert()
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
			valueClass: ValueJSON[bool]("picture.enable_gravatar").SelectFrom("picture.disable_gravatar").WithFileConfig(CfgSecKey{Sec: "", Key: ""}),
			want:       "picture.enable_gravatar",
		},
		{
			name:       "Normal dynKey name",
			valueClass: ValueJSON[bool]("picture.disable_gravatar").WithFileConfig(CfgSecKey{Sec: "", Key: ""}),
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
