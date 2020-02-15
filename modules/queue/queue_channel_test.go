// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"testing"
	"time"

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

	queue, err := NewChannelQueue(handle,
		ChannelQueueConfiguration{
			QueueLength:  0,
			MaxWorkers:   10,
			BlockTimeout: 1 * time.Second,
			BoostTimeout: 5 * time.Minute,
			BoostWorkers: 5,
			Workers:      0,
			Name:         "TestChannelQueue",
		}, &testData{})
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

func TestChannelQueue_Batch(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) {
		assert.True(t, len(data) == 2)
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
	}

	nilFn := func(_ context.Context, _ func()) {}

	queue, err := NewChannelQueue(handle,
		ChannelQueueConfiguration{
			QueueLength:  20,
			BatchLength:  2,
			Workers:      1,
			MaxWorkers:   10,
			BlockTimeout: 1 * time.Second,
			BoostTimeout: 5 * time.Minute,
			BoostWorkers: 5,
		}, &testData{})
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
