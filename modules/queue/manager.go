// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"errors"
	"maps"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Manager is a manager for the queues created by "CreateXxxQueue" functions, these queues are called "managed queues".
type Manager struct {
	mu sync.Mutex

	qidCounter int64
	Queues     map[int64]ManagedWorkerPoolQueue
}

type ManagedWorkerPoolQueue interface {
	GetName() string
	GetType() string
	GetItemTypeName() string
	GetWorkerNumber() int
	GetWorkerActiveNumber() int
	GetWorkerMaxNumber() int
	SetWorkerMaxNumber(num int)
	GetQueueItemNumber() int

	// FlushWithContext tries to make the handler process all items in the queue synchronously.
	// It is for testing purpose only. It's not designed to be used in a cluster.
	// Negative timeout means discarding all items in the queue.
	FlushWithContext(ctx context.Context, timeout time.Duration) error

	// RemoveAllItems removes all items in the base queue (on-the-fly items are not affected)
	RemoveAllItems(ctx context.Context) error
}

var manager *Manager

func init() {
	manager = &Manager{
		Queues: make(map[int64]ManagedWorkerPoolQueue),
	}
}

func GetManager() *Manager {
	return manager
}

func (m *Manager) AddManagedQueue(managed ManagedWorkerPoolQueue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.qidCounter++
	m.Queues[m.qidCounter] = managed
}

func (m *Manager) GetManagedQueue(qid int64) ManagedWorkerPoolQueue {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Queues[qid]
}

func (m *Manager) ManagedQueues() map[int64]ManagedWorkerPoolQueue {
	m.mu.Lock()
	defer m.mu.Unlock()

	queues := make(map[int64]ManagedWorkerPoolQueue, len(m.Queues))
	maps.Copy(queues, m.Queues)
	return queues
}

// FlushAll tries to make all managed queues process all items synchronously, until timeout or the queue is empty.
// It is for testing purpose only. It's not designed to be used in a cluster.
// Negative timeout means discarding all items in the queue.
func (m *Manager) FlushAll(ctx context.Context, timeout time.Duration) error {
	var finalErrors []error
	qs := m.ManagedQueues()
	for _, q := range qs {
		if err := q.FlushWithContext(ctx, timeout); err != nil {
			finalErrors = append(finalErrors, err)
		}
	}
	return errors.Join(finalErrors...)
}

// CreateSimpleQueue creates a simple queue from global setting config provider by name
func CreateSimpleQueue[T any](ctx context.Context, name string, handler HandlerFuncT[T]) *WorkerPoolQueue[T] {
	return createWorkerPoolQueue(ctx, name, setting.CfgProvider, handler, false)
}

// CreateUniqueQueue creates a unique queue from global setting config provider by name
func CreateUniqueQueue[T any](ctx context.Context, name string, handler HandlerFuncT[T]) *WorkerPoolQueue[T] {
	return createWorkerPoolQueue(ctx, name, setting.CfgProvider, handler, true)
}

func createWorkerPoolQueue[T any](ctx context.Context, name string, cfgProvider setting.ConfigProvider, handler HandlerFuncT[T], unique bool) *WorkerPoolQueue[T] {
	queueSetting, err := setting.GetQueueSettings(cfgProvider, name)
	if err != nil {
		log.Error("Failed to get queue settings for %q: %v", name, err)
		return nil
	}
	w, err := NewWorkerPoolQueueWithContext(ctx, name, queueSetting, handler, unique)
	if err != nil {
		log.Error("Failed to create queue %q: %v", name, err)
		return nil
	}
	GetManager().AddManagedQueue(w)
	return w
}
