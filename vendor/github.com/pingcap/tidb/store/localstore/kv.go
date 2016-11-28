// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package localstore

import (
	"net/url"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/store/localstore/engine"
	"github.com/pingcap/tidb/util/segmentmap"
	"github.com/twinj/uuid"
)

var (
	_ kv.Storage = (*dbStore)(nil)
)

type op int

const (
	opSeek = iota + 1
	opCommit
)

const (
	maxSeekWorkers = 3

	lowerWaterMark = 10 // second
)

type command struct {
	op    op
	txn   *dbTxn
	args  interface{}
	reply interface{}
	done  chan error
}

type seekReply struct {
	key   []byte
	value []byte
}

type commitReply struct {
	err error
}

type seekArgs struct {
	key []byte
}

type commitArgs struct {
}

// Seek searches for the first key in the engine which is >= key in byte order, returns (nil, nil, ErrNotFound)
// if such key is not found.
func (s *dbStore) Seek(key []byte) ([]byte, []byte, error) {
	c := &command{
		op:   opSeek,
		args: &seekArgs{key: key},
		done: make(chan error, 1),
	}

	s.commandCh <- c
	err := <-c.done
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	reply := c.reply.(*seekReply)
	return reply.key, reply.value, nil
}

// Commit writes the changed data in Batch.
func (s *dbStore) CommitTxn(txn *dbTxn) error {
	if len(txn.lockedKeys) == 0 {
		return nil
	}
	c := &command{
		op:   opCommit,
		txn:  txn,
		args: &commitArgs{},
		done: make(chan error, 1),
	}

	s.commandCh <- c
	err := <-c.done
	return errors.Trace(err)
}

func (s *dbStore) seekWorker(wg *sync.WaitGroup, seekCh chan *command) {
	defer wg.Done()
	for {
		var pending []*command
		select {
		case cmd, ok := <-seekCh:
			if !ok {
				return
			}
			pending = append(pending, cmd)
		L:
			for {
				select {
				case cmd, ok := <-seekCh:
					if !ok {
						break L
					}
					pending = append(pending, cmd)
				default:
					break L
				}
			}
		}

		s.doSeek(pending)
	}
}

func (s *dbStore) scheduler() {
	closed := false
	seekCh := make(chan *command, 1000)
	wgSeekWorkers := &sync.WaitGroup{}
	wgSeekWorkers.Add(maxSeekWorkers)
	for i := 0; i < maxSeekWorkers; i++ {
		go s.seekWorker(wgSeekWorkers, seekCh)
	}

	segmentIndex := int64(0)

	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case cmd := <-s.commandCh:
			if closed {
				cmd.done <- ErrDBClosed
				continue
			}
			switch cmd.op {
			case opSeek:
				seekCh <- cmd
			case opCommit:
				s.doCommit(cmd)
			}
		case <-s.closeCh:
			closed = true
			// notify seek worker to exit
			close(seekCh)
			wgSeekWorkers.Wait()
			s.wg.Done()
		case <-tick.C:
			segmentIndex = segmentIndex % s.recentUpdates.SegmentCount()
			s.cleanRecentUpdates(segmentIndex)
			segmentIndex++
		}
	}
}

func (s *dbStore) cleanRecentUpdates(segmentIndex int64) {
	m, err := s.recentUpdates.GetSegment(segmentIndex)
	if err != nil {
		log.Error(err)
		return
	}

	now := time.Now().Unix()
	for k, v := range m {
		dis := now - version2Second(v.(kv.Version))
		if dis > lowerWaterMark {
			delete(m, k)
		}
	}
}

func (s *dbStore) tryLock(txn *dbTxn) (err error) {
	// check conflict
	for k := range txn.lockedKeys {
		if _, ok := s.keysLocked[k]; ok {
			return errors.Trace(kv.ErrLockConflict)
		}

		lastVer, ok := s.recentUpdates.Get([]byte(k))
		if !ok {
			continue
		}
		// If there's newer version of this key, returns error.
		if lastVer.(kv.Version).Cmp(kv.Version{Ver: txn.tid}) > 0 {
			return errors.Trace(kv.ErrConditionNotMatch)
		}
	}

	// record
	for k := range txn.lockedKeys {
		s.keysLocked[k] = txn.tid
	}

	return nil
}

func (s *dbStore) doCommit(cmd *command) {
	txn := cmd.txn
	curVer, err := globalVersionProvider.CurrentVersion()
	if err != nil {
		log.Fatal(err)
	}
	err = s.tryLock(txn)
	if err != nil {
		cmd.done <- errors.Trace(err)
		return
	}
	// Update commit version.
	txn.version = curVer
	b := s.db.NewBatch()
	txn.us.WalkBuffer(func(k kv.Key, value []byte) error {
		mvccKey := MvccEncodeVersionKey(kv.Key(k), curVer)
		if len(value) == 0 { // Deleted marker
			b.Put(mvccKey, nil)
			s.compactor.OnDelete(k)
		} else {
			b.Put(mvccKey, value)
			s.compactor.OnSet(k)
		}
		return nil
	})
	err = s.writeBatch(b)
	s.unLockKeys(txn)
	cmd.done <- errors.Trace(err)
}

func (s *dbStore) doSeek(seekCmds []*command) {
	keys := make([][]byte, 0, len(seekCmds))
	for _, cmd := range seekCmds {
		keys = append(keys, cmd.args.(*seekArgs).key)
	}

	results := s.db.MultiSeek(keys)

	for i, cmd := range seekCmds {
		reply := &seekReply{}
		var err error
		reply.key, reply.value, err = results[i].Key, results[i].Value, results[i].Err
		cmd.reply = reply
		cmd.done <- errors.Trace(err)
	}
}

func (s *dbStore) NewBatch() engine.Batch {
	return s.db.NewBatch()
}

type dbStore struct {
	db engine.DB

	txns       map[uint64]*dbTxn
	keysLocked map[string]uint64
	// TODO: clean up recentUpdates
	recentUpdates *segmentmap.SegmentMap
	uuid          string
	path          string
	compactor     *localstoreCompactor
	wg            *sync.WaitGroup

	commandCh chan *command
	closeCh   chan struct{}

	mu     sync.Mutex
	closed bool
}

type storeCache struct {
	mu    sync.Mutex
	cache map[string]*dbStore
}

var (
	globalVersionProvider kv.VersionProvider
	mc                    storeCache

	// ErrDBClosed is the error meaning db is closed and we can use it anymore.
	ErrDBClosed = errors.New("db is closed")
)

func init() {
	mc.cache = make(map[string]*dbStore)
	globalVersionProvider = &LocalVersionProvider{}
}

// Driver implements kv.Driver interface.
type Driver struct {
	// engine.Driver is the engine driver for different local db engine.
	engine.Driver
}

// IsLocalStore checks whether a storage is local or not.
func IsLocalStore(s kv.Storage) bool {
	_, ok := s.(*dbStore)
	return ok
}

// Open opens or creates a storage with specific format for a local engine Driver.
// The path should be a URL format which is described in tidb package.
func (d Driver) Open(path string) (kv.Storage, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	u, err := url.Parse(path)
	if err != nil {
		return nil, errors.Trace(err)
	}

	engineSchema := filepath.Join(u.Host, u.Path)
	if store, ok := mc.cache[engineSchema]; ok {
		// TODO: check the cache store has the same engine with this Driver.
		log.Info("[kv] cache store", engineSchema)
		return store, nil
	}

	db, err := d.Driver.Open(engineSchema)
	if err != nil {
		return nil, errors.Trace(err)
	}

	log.Info("[kv] New store", engineSchema)
	s := &dbStore{
		txns:       make(map[uint64]*dbTxn),
		keysLocked: make(map[string]uint64),
		uuid:       uuid.NewV4().String(),
		path:       engineSchema,
		db:         db,
		compactor:  newLocalCompactor(localCompactDefaultPolicy, db),
		commandCh:  make(chan *command, 1000),
		closed:     false,
		closeCh:    make(chan struct{}),
		wg:         &sync.WaitGroup{},
	}
	s.recentUpdates, err = segmentmap.NewSegmentMap(100)
	if err != nil {
		return nil, errors.Trace(err)

	}
	mc.cache[engineSchema] = s
	s.compactor.Start()
	s.wg.Add(1)
	go s.scheduler()
	return s, nil
}

func (s *dbStore) UUID() string {
	return s.uuid
}

func (s *dbStore) GetSnapshot(ver kv.Version) (kv.Snapshot, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrDBClosed
	}
	s.mu.Unlock()

	currentVer, err := globalVersionProvider.CurrentVersion()
	if err != nil {
		return nil, errors.Trace(err)
	}

	if ver.Cmp(currentVer) > 0 {
		ver = currentVer
	}

	return &dbSnapshot{
		store:   s,
		version: ver,
	}, nil
}

func (s *dbStore) CurrentVersion() (kv.Version, error) {
	return globalVersionProvider.CurrentVersion()
}

// Begin transaction
func (s *dbStore) Begin() (kv.Transaction, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrDBClosed
	}
	s.mu.Unlock()

	beginVer, err := globalVersionProvider.CurrentVersion()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return newTxn(s, beginVer), nil
}

func (s *dbStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrDBClosed
	}

	s.closed = true
	s.mu.Unlock()

	mc.mu.Lock()
	defer mc.mu.Unlock()
	s.compactor.Stop()
	s.closeCh <- struct{}{}
	s.wg.Wait()
	delete(mc.cache, s.path)
	return s.db.Close()
}

func (s *dbStore) writeBatch(b engine.Batch) error {
	if b.Len() == 0 {
		return nil
	}

	if s.closed {
		return errors.Trace(ErrDBClosed)
	}

	err := s.db.Commit(b)
	if err != nil {
		log.Error(err)
		return errors.Trace(err)
	}

	return nil
}

func (s *dbStore) newBatch() engine.Batch {
	return s.db.NewBatch()
}
func (s *dbStore) unLockKeys(txn *dbTxn) error {
	for k := range txn.lockedKeys {
		if tid, ok := s.keysLocked[k]; !ok || tid != txn.tid {
			debug.PrintStack()
			log.Fatalf("should never happend:%v, %v", tid, txn.tid)
		}

		delete(s.keysLocked, k)
		s.recentUpdates.Set([]byte(k), txn.version, true)
	}

	return nil
}
