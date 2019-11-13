// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChannelQueue(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) {
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
	}

	nilFn := func(_ context.Context, _ func()) {}

	queue, err := NewChannelQueue(handle, ChannelQueueConfiguration{QueueLength: 20}, &testData{})
	assert.NoError(t, err)

	go queue.Run(nilFn, nilFn)

	test1 := testData{"A", 1}
	go queue.Push(&test1)
	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	err = queue.Push(test1)
	assert.Error(t, err)
}
