// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package optional

import (
	std_json "encoding/json"
	"testing"

	"code.gitea.io/gitea/modules/json"
	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"
)

type testSerializationStruct struct {
	NormalString string         `json:"normal_string" yaml:"normal_string"`
	NormalBool   bool           `json:"normal_bool" yaml:"normal_bool"`
	OptBool      Option[bool]   `json:"optional_bool,omitempty" yaml:"optional_bool,omitempty"`
	OptString    Option[string] `json:"optional_string,omitempty" yaml:"optional_string,omitempty"`
	OptTwoBool   Option[bool]   `json:"optional_two_bool" yaml:"optional_two_bool"`
	OptTwoString Option[string] `json:"optional_twostring" yaml:"optional_two_string"`
}

func TestOptionalToJson(t *testing.T) {
	tests := []struct {
		name string
		obj  *testSerializationStruct
		want string
		// TODO: report this bug to upstream, as `jsoniter.ConfigCompatibleWithStandardLibrary` claims to be std compatible
		wantGitea string
	}{
		{
			name:      "empty",
			obj:       new(testSerializationStruct),
			want:      `{"normal_string":"","normal_bool":false,"optional_two_bool":null,"optional_twostring":null}`,
			wantGitea: `{"normal_string":"","normal_bool":false,"optional_bool":null,"optional_string":null,"optional_two_bool":null,"optional_twostring":null}`,
		},
		{
			name: "some",
			obj: &testSerializationStruct{
				NormalString: "a string",
				NormalBool:   true,
				OptBool:      Some(false),
				OptString:    Some(""),
			},
			want: `{"normal_string":"a string","normal_bool":true,"optional_bool":false,"optional_string":"","optional_two_bool":null,"optional_twostring":null}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.obj)
			assert.NoError(t, err)
			if tc.wantGitea != "" {
				assert.EqualValues(t, tc.wantGitea, string(b), "gitea json module returned unexpected")
			} else {
				assert.EqualValues(t, tc.want, string(b), "gitea json module returned unexpected")
			}

			b, err = std_json.Marshal(tc.obj)
			assert.NoError(t, err)
			assert.EqualValues(t, tc.want, string(b), "std json module returned unexpected")
		})
	}
}

func TestOptionalFromJson(t *testing.T) {
	tests := []struct {
		name string
		data string
		want testSerializationStruct
	}{
		{
			name: "empty",
			data: `{}`,
			want: testSerializationStruct{
				NormalString: "",
			},
		},
		{
			name: "some",
			data: `{"normal_string":"a string","normal_bool":true,"optional_bool":false,"optional_string":"","optional_two_bool":null,"optional_twostring":null}`,
			want: testSerializationStruct{
				NormalString: "a string",
				NormalBool:   true,
				OptBool:      Some(false),
				OptString:    Some(""),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var obj1 testSerializationStruct
			err := json.Unmarshal([]byte(tc.data), &obj1)
			assert.NoError(t, err)
			assert.EqualValues(t, tc.want, obj1, "gitea json module returned unexpected")

			var obj2 testSerializationStruct
			err = std_json.Unmarshal([]byte(tc.data), &obj2)
			assert.NoError(t, err)
			assert.EqualValues(t, tc.want, obj2, "std json module returned unexpected")
		})
	}
}

func TestOptionalToYaml(t *testing.T) {
	tests := []struct {
		name string
		obj  *testSerializationStruct
		want string
	}{
		{
			name: "empty",
			obj:  new(testSerializationStruct),
			want: `normal_string: ""
normal_bool: false
optional_two_bool: null
optional_two_string: null
`,
		},
		{
			name: "some",
			obj: &testSerializationStruct{
				NormalString: "a string",
				NormalBool:   true,
				OptBool:      Some(false),
				OptString:    Some(""),
			},
			want: `normal_string: a string
normal_bool: true
optional_bool: false
optional_string: ""
optional_two_bool: null
optional_two_string: null
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := yaml.Marshal(tc.obj)
			assert.NoError(t, err)
			assert.EqualValues(t, tc.want, string(b), "yaml module returned unexpected")
		})
	}
}

func TestOptionalFromYaml(t *testing.T) {
	tests := []struct {
		name string
		data string
		want testSerializationStruct
	}{
		{
			name: "empty",
			data: ``,
			want: testSerializationStruct{},
		},
		{
			name: "empty but init",
			data: `normal_string: ""
normal_bool: false
optional_bool:
optional_two_bool:
optional_two_string:
`,
			want: testSerializationStruct{},
		},
		{
			name: "some",
			data: `
normal_string: a string
normal_bool: true
optional_bool: false
optional_string: ""
optional_two_bool: null
optional_twostring: null
`,
			want: testSerializationStruct{
				NormalString: "a string",
				NormalBool:   true,
				OptBool:      Some(false),
				OptString:    Some(""),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var obj testSerializationStruct
			err := yaml.Unmarshal([]byte(tc.data), &obj)
			assert.NoError(t, err)
			assert.EqualValues(t, tc.want, obj, "yaml module returned unexpected")
		})
	}
}
