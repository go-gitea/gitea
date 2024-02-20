// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package optional

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testSerializationStruct struct {
	NormalString string         `json:"normal_string" yaml:"normal_string"`
	NormalBool   bool           `json:"normal_bool" yaml:"normal_bool"`
	OptBool      Option[bool]   `json:"optional_bool,omitempty" yaml:"optional_bool,omitempty"`
	OptString    Option[string] `json:"optional_string,omitempty" yaml:"optional_string,omitempty"`
	// TODO: fix case
	// OptTwoBool   Option[bool]   `json:"optional_two_bool" yaml:"optional_twobool"`
	// OptTwoString Option[string] `json:"optional_twostring" yaml:"optional_twostring"`
}

func TestOptionalToJson(t *testing.T) {
	tests := []struct {
		name string
		obj  *testSerializationStruct
		want string
	}{
		{
			name: "empty",
			obj:  new(testSerializationStruct),
			want: `{"normal_string":"","normal_bool":false}`,
		},
		{
			name: "some",
			obj: &testSerializationStruct{
				NormalString: "a string",
				NormalBool:   true,
				OptBool:      Some(false),
				OptString:    Some(""),
			},
			want: `{"normal_string":"a string","normal_bool":true,"optional_bool":false,"optional_string":""}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.obj)
			assert.NoError(t, err)
			assert.EqualValues(t, tc.want, string(b))
		})
	}
}

func TestOptionalFromJson(t *testing.T) {
}

func TestOptionalToYaml(t *testing.T) {
}

func TestOptionalFromYaml(t *testing.T) {
}
