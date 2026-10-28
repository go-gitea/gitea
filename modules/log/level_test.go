// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
	"testing"

	"gitea.dev/modules/json"

	"github.com/stretchr/testify/assert"
)

type testLevel struct {
	Level Level `json:"level"`
}

func TestLevelMarshalUnmarshalJSON(t *testing.T) {
	levelBytes, err := json.Marshal(testLevel{
		Level: INFO,
	})
	assert.NoError(t, err)
	assert.Equal(t, string(makeTestLevelBytes(INFO.String())), string(levelBytes))

	var testLevel testLevel
	for _, levelJSON := range [][]byte{
		levelBytes,
		makeTestLevelBytes(`FOFOO`),
		fmt.Appendf(nil, `{"level":%d}`, 2),
		fmt.Appendf(nil, `{"level":%d}`, 10012),
		[]byte(`{"level":{}}`),
	} {
		err = json.Unmarshal(levelJSON, &testLevel)
		assert.NoError(t, err)
		assert.Equal(t, INFO, testLevel.Level)
	}

	assert.Equal(t, INFO.String(), Level(1001).String())

	err = json.Unmarshal([]byte(`{"level":{}`), &testLevel.Level)
	assert.Error(t, err)
}

func makeTestLevelBytes(level string) []byte {
	return fmt.Appendf(nil, `{"level":"%s"}`, level)
}
