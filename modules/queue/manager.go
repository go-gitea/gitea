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
	Queues  map[int64]*Description
}

// Description represents a working queue inheriting from Gitea.
type Description struct {
	mutex           sync.Mutex
	QID             int64
	Queue           Queue
	Type            Type
	Name            string
	Configuration   interface{}
	ExemplarType    string
	addWorkers      func(number int, timeout time.Duration) context.CancelFunc
	numberOfWorkers func() int
	counter         int64
	PoolWorkers     map[int64]*PoolWorkers
}

// DescriptionList implements the sort.Interface
type DescriptionList []*Description

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
			Queues: make(map[int64]*Description),
		}
	}
	return manager
}

// Add adds a queue to this manager
func (m *Manager) Add(queue Queue,
	t Type,
	configuration,
	exemplar interface{},
	addWorkers func(number int, timeout time.Duration) context.CancelFunc,
	numberOfWorkers func() int) int64 {

	cfg, _ := json.Marshal(configuration)
	desc := &Description{
		Queue:           queue,
		Type:            t,
		Configuration:   string(cfg),
		ExemplarType:    reflect.TypeOf(exemplar).String(),
		PoolWorkers:     make(map[int64]*PoolWorkers),
		addWorkers:      addWorkers,
		numberOfWorkers: numberOfWorkers,
	}
	m.mutex.Lock()
	m.counter++
	desc.QID = m.counter
	desc.Name = fmt.Sprintf("queue-%d", desc.QID)
	if named, ok := queue.(Named); ok {
		desc.Name = named.Name()
	}
	m.Queues[desc.QID] = desc
	m.mutex.Unlock()
	log.Trace("Queue Manager registered: %s (QID: %d)", desc.Name, desc.QID)
	return desc.QID
}

// Remove a queue from the Manager
func (m *Manager) Remove(qid int64) {
	m.mutex.Lock()
	delete(m.Queues, qid)
	m.mutex.Unlock()
	log.Trace("Queue Manager removed: QID: %d", qid)

}

// GetDescription by qid
func (m *Manager) GetDescription(qid int64) *Description {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.Queues[qid]
}

// Descriptions returns the queue descriptions
func (m *Manager) Descriptions() []*Description {
	m.mutex.Lock()
	descs := make([]*Description, 0, len(m.Queues))
	for _, desc := range m.Queues {
		descs = append(descs, desc)
	}
	m.mutex.Unlock()
	sort.Sort(DescriptionList(descs))
	return descs
}

// Workers returns the poolworkers
func (q *Description) Workers() []*PoolWorkers {
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
func (q *Description) RegisterWorkers(number int, start time.Time, hasTimeout bool, timeout time.Time, cancel context.CancelFunc) int64 {
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
func (q *Description) CancelWorkers(pid int64) {
	q.mutex.Lock()
	pw, ok := q.PoolWorkers[pid]
	q.mutex.Unlock()
	if !ok {
		return
	}
	pw.Cancel()
}

// RemoveWorkers deletes pooled workers with pid
func (q *Description) RemoveWorkers(pid int64) {
	q.mutex.Lock()
	delete(q.PoolWorkers, pid)
	q.mutex.Unlock()
}

// AddWorkers adds workers to the queue if it has registered an add worker function
func (q *Description) AddWorkers(number int, timeout time.Duration) {
	if q.addWorkers != nil {
		_ = q.addWorkers(number, timeout)
	}
}

// NumberOfWorkers returns the number of workers in the queue
func (q *Description) NumberOfWorkers() int {
	if q.numberOfWorkers != nil {
		return q.numberOfWorkers()
	}
	return -1
}

func (l DescriptionList) Len() int {
	return len(l)
}

func (l DescriptionList) Less(i, j int) bool {
	return l[i].Name < l[j].Name
}

func (l DescriptionList) Swap(i, j int) {
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
