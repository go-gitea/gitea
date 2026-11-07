// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package optional_test

import (
	std_json "encoding/json" //nolint:depguard // for testing purpose
	"testing"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/optional"

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

type testSerializationStruct struct {
	NormalString string                `json:"normal_string" yaml:"normal_string"`
	NormalBool   bool                  `json:"normal_bool" yaml:"normal_bool"`
	OptBool      optional.Option[bool] `json:"optional_bool,omitempty" yaml:"optional_bool,omitempty"`

	// It causes an undefined behavior: should the "omitempty" tag only omit "null", or also the empty string?
	// The behavior is inconsistent between json and v2 packages, and there is no such use case in Gitea.
	// If anyone really needs it, they can use json.MarshalKeepOptionalEmpty to revert the v1 behavior
	OptString optional.Option[string] `json:"optional_string,omitempty" yaml:"optional_string,omitempty"`

	OptTwoBool   optional.Option[bool]   `json:"optional_two_bool" yaml:"optional_two_bool"`
	OptTwoString optional.Option[string] `json:"optional_two_string" yaml:"optional_two_string"`
}

type testSerializationNullValue struct {
	OptString optional.Option[string] `json:"opt_null,omitempty" yaml:"opt_null,omitempty"`
}

type testSerializationNullPointer struct {
	OptNull optional.Option[*time.Time] `json:"opt_null,omitempty" yaml:"opt_null,omitempty"`
}

func TestOptionalToJsonAndYaml(t *testing.T) {
	tests := []struct {
		name     string
		obj      *testSerializationStruct
		want     string
		wantYaml string
	}{
		{
			name: "empty",
			obj:  new(testSerializationStruct),
			want: `{"normal_string":"","normal_bool":false,"optional_two_bool":null,"optional_two_string":null}`,
			wantYaml: `normal_string: ""
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
				OptBool:      optional.Some(false),
				OptString:    optional.Some(""),
				OptTwoBool:   optional.None[bool](),
				OptTwoString: optional.None[string](),
			},
			want: `{"normal_string":"a string","normal_bool":true,"optional_bool":false,"optional_string":"","optional_two_bool":null,"optional_two_string":null}`,
			wantYaml: `normal_string: a string
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
			b, err := json.MarshalKeepOptionalEmpty(tc.obj)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, string(b), "gitea json module returned unexpected")

			b, err = std_json.Marshal(tc.obj)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, string(b), "std json module returned unexpected")

			b, err = yaml.Marshal(tc.obj)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantYaml, string(b), "yaml module returned unexpected")
		})
	}

	t.Run("NullValue", func(t *testing.T) {
		var v1 testSerializationNullValue
		err := json.Unmarshal([]byte(`{"opt_null":null}`), &v1)
		assert.NoError(t, err)
		assert.False(t, v1.OptString.Has())
	})

	t.Run("NullPointer", func(t *testing.T) {
		var v1 testSerializationNullPointer
		err := json.Unmarshal([]byte(`{"opt_null":null}`), &v1)
		assert.NoError(t, err)
		assert.True(t, v1.OptNull.Has())
		assert.Nil(t, v1.OptNull.Value())

		var v2 testSerializationNullPointer
		err = std_json.Unmarshal([]byte(`{}`), &v2)
		assert.NoError(t, err)
		assert.False(t, v2.OptNull.Has())

		var v3 testSerializationNullPointer
		err = std_json.Unmarshal([]byte(`{"opt_null":"2006-01-02T15:04:05Z"}`), &v3)
		assert.NoError(t, err)
		assert.True(t, v3.OptNull.Has())
		assert.NotNil(t, v3.OptNull.Value())
	})
}

func TestOptionalFromJsonAndYaml(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		dataYaml string
		want     testSerializationStruct
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
			data: `{"normal_string":"a string","normal_bool":true,"optional_bool":false,"optional_string":"","optional_two_bool":null,"optional_two_string":null}`,
			dataYaml: `
normal_string: a string
normal_bool: true
optional_bool: false
optional_string: ""
optional_two_bool: null
optional_two_string: null
`,
			want: testSerializationStruct{
				NormalString: "a string",
				NormalBool:   true,
				OptBool:      optional.Some(false),
				OptString:    optional.Some(""),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, format := range []struct {
				data      string
				unmarshal func([]byte, any) error
				message   string
			}{
				{tc.data, json.Unmarshal, "gitea json module returned unexpected"},
				{tc.data, std_json.Unmarshal, "std json module returned unexpected"},
				{tc.dataYaml, yaml.Unmarshal, "yaml module returned unexpected"},
			} {
				var obj testSerializationStruct
				err := format.unmarshal([]byte(format.data), &obj)
				assert.NoError(t, err)
				assert.Equal(t, tc.want, obj, format.message)
			}
		})
	}
	t.Run("empty but init", func(t *testing.T) {
		var obj testSerializationStruct
		err := yaml.Unmarshal([]byte(`normal_string: ""
normal_bool: false
optional_bool:
optional_two_bool:
optional_two_string:
`), &obj)
		assert.NoError(t, err)
		assert.Equal(t, testSerializationStruct{}, obj, "yaml module returned unexpected")
	})
}
