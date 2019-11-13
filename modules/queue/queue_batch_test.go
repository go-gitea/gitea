// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import "testing"

import "github.com/stretchr/testify/assert"

import "context"

func TestBatchedChannelQueue(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) {
		assert.True(t, len(data) == 2)
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
	}

	nilFn := func(_ context.Context, _ func()) {}

	queue, err := NewBatchedChannelQueue(handle, BatchedChannelQueueConfiguration{QueueLength: 20, BatchLength: 2}, &testData{})
	assert.NoError(t, err)

	go queue.Run(nilFn, nilFn)

	test1 := testData{"A", 1}
	test2 := testData{"B", 2}

	queue.Push(&test1)
	go queue.Push(&test2)

	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	result2 := <-handleChan
	assert.Equal(t, test2.TestString, result2.TestString)
	assert.Equal(t, test2.TestInt, result2.TestInt)

	err = queue.Push(test1)
	assert.Error(t, err)
}
