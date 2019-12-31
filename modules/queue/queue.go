// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// ErrInvalidConfiguration is called when there is invalid configuration for a queue
type ErrInvalidConfiguration struct {
	cfg interface{}
	err error
}

func (err ErrInvalidConfiguration) Error() string {
	if err.err != nil {
		return fmt.Sprintf("Invalid Configuration Argument: %v: Error: %v", err.cfg, err.err)
	}
	return fmt.Sprintf("Invalid Configuration Argument: %v", err.cfg)
}

// IsErrInvalidConfiguration checks if an error is an ErrInvalidConfiguration
func IsErrInvalidConfiguration(err error) bool {
	_, ok := err.(ErrInvalidConfiguration)
	return ok
}

// Type is a type of Queue
type Type string

// Data defines an type of queuable data
type Data interface{}

// HandlerFunc is a function that takes a variable amount of data and processes it
type HandlerFunc func(...Data)

// NewQueueFunc is a function that creates a queue
type NewQueueFunc func(handler HandlerFunc, config interface{}, exemplar interface{}) (Queue, error)

// Shutdownable represents a queue that can be shutdown
type Shutdownable interface {
	Shutdown()
	Terminate()
}

// Named represents a queue with a name
type Named interface {
	Name() string
}

// Queue defines an interface to save an issue indexer queue
type Queue interface {
	Run(atShutdown, atTerminate func(context.Context, func()))
	Push(Data) error
}

// DummyQueueType is the type for the dummy queue
const DummyQueueType Type = "dummy"

// NewDummyQueue creates a new DummyQueue
func NewDummyQueue(handler HandlerFunc, opts, exemplar interface{}) (Queue, error) {
	return &DummyQueue{}, nil
}

// DummyQueue represents an empty queue
type DummyQueue struct {
}

// Run starts to run the queue
func (b *DummyQueue) Run(_, _ func(context.Context, func())) {}

// Push pushes data to the queue
func (b *DummyQueue) Push(Data) error {
	return nil
}

func toConfig(exemplar, cfg interface{}) (interface{}, error) {
	if reflect.TypeOf(cfg).AssignableTo(reflect.TypeOf(exemplar)) {
		return cfg, nil
	}

	configBytes, ok := cfg.([]byte)
	if !ok {
		configStr, ok := cfg.(string)
		if !ok {
			return nil, ErrInvalidConfiguration{cfg: cfg}
		}
		configBytes = []byte(configStr)
	}
	newVal := reflect.New(reflect.TypeOf(exemplar))
	if err := json.Unmarshal(configBytes, newVal.Interface()); err != nil {
		return nil, ErrInvalidConfiguration{cfg: cfg, err: err}
	}
	return newVal.Elem().Interface(), nil
}

var queuesMap = map[Type]NewQueueFunc{DummyQueueType: NewDummyQueue}

// RegisteredTypes provides the list of requested types of queues
func RegisteredTypes() []Type {
	types := make([]Type, len(queuesMap))
	i := 0
	for key := range queuesMap {
		types[i] = key
		i++
	}
	return types
}

// RegisteredTypesAsString provides the list of requested types of queues
func RegisteredTypesAsString() []string {
	types := make([]string, len(queuesMap))
	i := 0
	for key := range queuesMap {
		types[i] = string(key)
		i++
	}
	return types
}

// NewQueue takes a queue Type and HandlerFunc some options and possibly an exemplar and returns a Queue or an error
func NewQueue(queueType Type, handlerFunc HandlerFunc, opts, exemplar interface{}) (Queue, error) {
	newFn, ok := queuesMap[queueType]
	if !ok {
		return nil, fmt.Errorf("Unsupported queue type: %v", queueType)
	}
	return newFn(handlerFunc, opts, exemplar)
}
