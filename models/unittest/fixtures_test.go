// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type testSub2 int64

func (t testSub2) String() string {
	return "unknow"
}

type testSubConversion struct{}

func (c *testSubConversion) FromDB([]byte) error {
	return nil
}

func (c *testSubConversion) ToDB() ([]byte, error) {
	return []byte("testSubConversion"), nil
}

func TestDefaultFixtureDumper(t *testing.T) {
	type TestModelSub struct {
		A int64
		B int64
	}

	type TestExtern struct {
		AA string
		BB int
	}

	type TestModel struct {
		ID          int64    `xorm:"pk autoincr"`
		A           int64    `xorm:"BIGINT DEFAULT 133 NOT NULL"`
		D           []string `xorm:"JSON"`
		Description string   `xorm:"TEXT"`
		IDString    string
		G           *TestModelSub `xorm:"-"`
		W           string        `xorm:"-"`
		TestBool    bool
		privateTest int64
		EmptyStr    string
		EmptyStr2   string `xorm:"TEXT"`
		Line2       string
		F           testSub2
		MM          int64      `xorm:"dd_ww"`
		ExternVerb  TestExtern `xorm:"extends"`
		NumStr      string
		JJ          *testSubConversion
		FF          []byte
	}

	buffer := bytes.NewBuffer(nil)

	err := DefaultFixtureDumper(&TestModel{
		ID:          12,
		A:           10,
		Description: "hello \" ' gitea",
		D:           []string{"test1", "test2"},
		W:           "hello world",
		IDString:    "AAAAAA",
		privateTest: 10,
		Line2:       "hello ' gitea",
		F:           12,
		MM:          10,
		ExternVerb: TestExtern{
			AA: "hello world",
			BB: 15,
		},
		NumStr: "1234123412341234123412341234123412341234",
		FF:     []byte("hello world"),
	}, buffer)

	assert.NoError(t, err)
	assert.EqualValues(t, `-
  id: 12
  a: 10
  d: '["test1","test2"]'
  description: 'hello " '' gitea'
  id_string: AAAAAA
  test_bool: false
  empty_str2: ''
  line2: 'hello '' gitea'
  f: 12
  dd_ww: 10
  aa: hello world
  bb: 15
  num_str: '1234123412341234123412341234123412341234'
  jj: testSubConversion
  ff: 0x68656c6c6f20776f726c64

`, buffer.String())

	result := make([]map[string]any, 0, 10)
	err = yaml.Unmarshal(buffer.Bytes(), &result)
	assert.EqualValues(t, "hello \" ' gitea", result[0]["description"])
	assert.NoError(t, err)
}
