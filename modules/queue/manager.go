// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

var manager *Manager

// Manager is a queue manager
type Manager struct {
	mutex sync.Mutex

	counter int64
	Queues  map[int64]*ManagedQueue
}

// ManagedQueue represents a working queue inheriting from Gitea.
type ManagedQueue struct {
	mutex         sync.Mutex
	QID           int64
	Queue         Queue
	Type          Type
	Name          string
	Configuration interface{}
	ExemplarType  string
	Pool          ManagedPool
	counter       int64
	PoolWorkers   map[int64]*PoolWorkers
}

// ManagedPool is a simple interface to get certain details from a worker pool
type ManagedPool interface {
	AddWorkers(number int, timeout time.Duration) context.CancelFunc
	NumberOfWorkers() int
	MaxNumberOfWorkers() int
	SetMaxNumberOfWorkers(int)
	BoostTimeout() time.Duration
	BlockTimeout() time.Duration
	BoostWorkers() int
	SetSettings(maxNumberOfWorkers, boostWorkers int, timeout time.Duration)
}

// ManagedQueueList implements the sort.Interface
type ManagedQueueList []*ManagedQueue

// PoolWorkers represents a working queue inheriting from Gitea.
type PoolWorkers struct {
	PID        int64
	Workers    int
	Start      time.Time
	Timeout    time.Time
	HasTimeout bool
	Cancel     context.CancelFunc
}

// PoolWorkersList implements the sort.Interface
type PoolWorkersList []*PoolWorkers

func init() {
	_ = GetManager()
}

// GetManager returns a Manager and initializes one as singleton if there's none yet
func GetManager() *Manager {
	if manager == nil {
		manager = &Manager{
			Queues: make(map[int64]*ManagedQueue),
		}
	}
	return manager
}

// Add adds a queue to this manager
func (m *Manager) Add(queue Queue,
	t Type,
	configuration,
	exemplar interface{},
	pool ManagedPool) int64 {

	cfg, _ := json.Marshal(configuration)
	mq := &ManagedQueue{
		Queue:         queue,
		Type:          t,
		Configuration: string(cfg),
		ExemplarType:  reflect.TypeOf(exemplar).String(),
		PoolWorkers:   make(map[int64]*PoolWorkers),
		Pool:          pool,
	}
	m.mutex.Lock()
	m.counter++
	mq.QID = m.counter
	mq.Name = fmt.Sprintf("queue-%d", mq.QID)
	if named, ok := queue.(Named); ok {
		mq.Name = named.Name()
	}
	m.Queues[mq.QID] = mq
	m.mutex.Unlock()
	log.Trace("Queue Manager registered: %s (QID: %d)", mq.Name, mq.QID)
	return mq.QID
}

// Remove a queue from the Manager
func (m *Manager) Remove(qid int64) {
	m.mutex.Lock()
	delete(m.Queues, qid)
	m.mutex.Unlock()
	log.Trace("Queue Manager removed: QID: %d", qid)

}

// GetManagedQueue by qid
func (m *Manager) GetManagedQueue(qid int64) *ManagedQueue {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.Queues[qid]
}

// ManagedQueues returns the managed queues
func (m *Manager) ManagedQueues() []*ManagedQueue {
	m.mutex.Lock()
	mqs := make([]*ManagedQueue, 0, len(m.Queues))
	for _, mq := range m.Queues {
		mqs = append(mqs, mq)
	}
	m.mutex.Unlock()
	sort.Sort(ManagedQueueList(mqs))
	return mqs
}

// Workers returns the poolworkers
func (q *ManagedQueue) Workers() []*PoolWorkers {
	q.mutex.Lock()
	workers := make([]*PoolWorkers, 0, len(q.PoolWorkers))
	for _, worker := range q.PoolWorkers {
		workers = append(workers, worker)
	}
	q.mutex.Unlock()
	sort.Sort(PoolWorkersList(workers))
	return workers
}

// RegisterWorkers registers workers to this queue
func (q *ManagedQueue) RegisterWorkers(number int, start time.Time, hasTimeout bool, timeout time.Time, cancel context.CancelFunc) int64 {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.counter++
	q.PoolWorkers[q.counter] = &PoolWorkers{
		PID:        q.counter,
		Workers:    number,
		Start:      start,
		Timeout:    timeout,
		HasTimeout: hasTimeout,
		Cancel:     cancel,
	}
	return q.counter
}

// CancelWorkers cancels pooled workers with pid
func (q *ManagedQueue) CancelWorkers(pid int64) {
	q.mutex.Lock()
	pw, ok := q.PoolWorkers[pid]
	q.mutex.Unlock()
	if !ok {
		return
	}
	pw.Cancel()
}

// RemoveWorkers deletes pooled workers with pid
func (q *ManagedQueue) RemoveWorkers(pid int64) {
	q.mutex.Lock()
	pw, ok := q.PoolWorkers[pid]
	delete(q.PoolWorkers, pid)
	q.mutex.Unlock()
	if ok && pw.Cancel != nil {
		pw.Cancel()
	}
}

// AddWorkers adds workers to the queue if it has registered an add worker function
func (q *ManagedQueue) AddWorkers(number int, timeout time.Duration) context.CancelFunc {
	if q.Pool != nil {
		// the cancel will be added to the pool workers description above
		return q.Pool.AddWorkers(number, timeout)
	}
	return nil
}

// NumberOfWorkers returns the number of workers in the queue
func (q *ManagedQueue) NumberOfWorkers() int {
	if q.Pool != nil {
		return q.Pool.NumberOfWorkers()
	}
	return -1
}

// MaxNumberOfWorkers returns the maximum number of workers for the pool
func (q *ManagedQueue) MaxNumberOfWorkers() int {
	if q.Pool != nil {
		return q.Pool.MaxNumberOfWorkers()
	}
	return 0
}

// BoostWorkers returns the number of workers for a boost
func (q *ManagedQueue) BoostWorkers() int {
	if q.Pool != nil {
		return q.Pool.BoostWorkers()
	}
	return -1
}

// BoostTimeout returns the timeout of the next boost
func (q *ManagedQueue) BoostTimeout() time.Duration {
	if q.Pool != nil {
		return q.Pool.BoostTimeout()
	}
	return 0
}

// BlockTimeout returns the timeout til the next boost
func (q *ManagedQueue) BlockTimeout() time.Duration {
	if q.Pool != nil {
		return q.Pool.BlockTimeout()
	}
	return 0
}

// SetSettings sets the setable boost values
func (q *ManagedQueue) SetSettings(maxNumberOfWorkers, boostWorkers int, timeout time.Duration) {
	if q.Pool != nil {
		q.Pool.SetSettings(maxNumberOfWorkers, boostWorkers, timeout)
	}
}

func (l ManagedQueueList) Len() int {
	return len(l)
}

func (l ManagedQueueList) Less(i, j int) bool {
	return l[i].Name < l[j].Name
}

func (l ManagedQueueList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l PoolWorkersList) Len() int {
	return len(l)
}

func (l PoolWorkersList) Less(i, j int) bool {
	return l[i].Start.Before(l[j].Start)
}

func (l PoolWorkersList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}
