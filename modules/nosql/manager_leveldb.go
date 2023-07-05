// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nosql

import (
	"fmt"
	"path"
	"runtime/pprof"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// CloseLevelDB closes a levelDB
func (m *Manager) CloseLevelDB(connection string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	db, ok := m.LevelDBConnections[connection]
	if !ok {
		// Try the full URI
		uri := ToLevelDBURI(connection)
		db, ok = m.LevelDBConnections[uri.String()]

		if !ok {
			// Try the datadir directly
			dataDir := path.Join(uri.Host, uri.Path)

			db, ok = m.LevelDBConnections[dataDir]
		}
	}
	if !ok {
		return nil
	}

	db.count--
	if db.count > 0 {
		return nil
	}

	for _, name := range db.name {
		delete(m.LevelDBConnections, name)
	}
	return db.db.Close()
}

// GetLevelDB gets a levelDB for a particular connection
func (m *Manager) GetLevelDB(connection string) (db *leveldb.DB, err error) {
	// Because we want associate any goroutines created by this call to the main nosqldb context we need to
	// wrap this in a goroutine labelled with the nosqldb context
	done := make(chan struct{})
	var recovered any
	go func() {
		defer func() {
			recovered = recover()
			if recovered != nil {
				log.Critical("PANIC during GetLevelDB: %v\nStacktrace: %s", recovered, log.Stack(2))
			}
			close(done)
		}()
		pprof.SetGoroutineLabels(m.ctx)

		db, err = m.getLevelDB(connection)
	}()
	<-done
	if recovered != nil {
		panic(recovered)
	}
	return db, err
}

func (m *Manager) getLevelDB(connection string) (*leveldb.DB, error) {
	// Convert the provided connection description to the common format
	uri := ToLevelDBURI(connection)

	// Get the datadir
	dataDir := path.Join(uri.Host, uri.Path)

	m.mutex.Lock()
	defer m.mutex.Unlock()
	db, ok := m.LevelDBConnections[connection]
	if ok {
		db.count++

		return db.db, nil
	}

	db, ok = m.LevelDBConnections[uri.String()]
	if ok {
		db.count++

		return db.db, nil
	}

	// if there is already a connection to this leveldb reuse that
	// NOTE: if there differing options then only the first leveldb connection will be used
	db, ok = m.LevelDBConnections[dataDir]
	if ok {
		db.count++
		log.Warn("Duplicate connection to level db: %s with different connection strings. Initial connection: %s. This connection: %s", dataDir, db.name[0], connection)
		db.name = append(db.name, connection)
		m.LevelDBConnections[connection] = db
		return db.db, nil
	}
	db = &levelDBHolder{
		name: []string{connection, uri.String(), dataDir},
	}

	opts := &opt.Options{}
	for k, v := range uri.Query() {
		switch replacer.Replace(strings.ToLower(k)) {
		case "blockcachecapacity":
			opts.BlockCacheCapacity, _ = strconv.Atoi(v[0])
		case "blockcacheevictremoved":
			opts.BlockCacheEvictRemoved, _ = strconv.ParseBool(v[0])
		case "blockrestartinterval":
			opts.BlockRestartInterval, _ = strconv.Atoi(v[0])
		case "blocksize":
			opts.BlockSize, _ = strconv.Atoi(v[0])
		case "compactionexpandlimitfactor":
			opts.CompactionExpandLimitFactor, _ = strconv.Atoi(v[0])
		case "compactiongpoverlapsfactor":
			opts.CompactionGPOverlapsFactor, _ = strconv.Atoi(v[0])
		case "compactionl0trigger":
			opts.CompactionL0Trigger, _ = strconv.Atoi(v[0])
		case "compactionsourcelimitfactor":
			opts.CompactionSourceLimitFactor, _ = strconv.Atoi(v[0])
		case "compactiontablesize":
			opts.CompactionTableSize, _ = strconv.Atoi(v[0])
		case "compactiontablesizemultiplier":
			opts.CompactionTableSizeMultiplier, _ = strconv.ParseFloat(v[0], 64)
		case "compactiontablesizemultiplierperlevel":
			for _, val := range v {
				f, _ := strconv.ParseFloat(val, 64)
				opts.CompactionTableSizeMultiplierPerLevel = append(opts.CompactionTableSizeMultiplierPerLevel, f)
			}
		case "compactiontotalsize":
			opts.CompactionTotalSize, _ = strconv.Atoi(v[0])
		case "compactiontotalsizemultiplier":
			opts.CompactionTotalSizeMultiplier, _ = strconv.ParseFloat(v[0], 64)
		case "compactiontotalsizemultiplierperlevel":
			for _, val := range v {
				f, _ := strconv.ParseFloat(val, 64)
				opts.CompactionTotalSizeMultiplierPerLevel = append(opts.CompactionTotalSizeMultiplierPerLevel, f)
			}
		case "compression":
			val, _ := strconv.Atoi(v[0])
			opts.Compression = opt.Compression(val)
		case "disablebufferpool":
			opts.DisableBufferPool, _ = strconv.ParseBool(v[0])
		case "disableblockcache":
			opts.DisableBlockCache, _ = strconv.ParseBool(v[0])
		case "disablecompactionbackoff":
			opts.DisableCompactionBackoff, _ = strconv.ParseBool(v[0])
		case "disablelargebatchtransaction":
			opts.DisableLargeBatchTransaction, _ = strconv.ParseBool(v[0])
		case "errorifexist":
			opts.ErrorIfExist, _ = strconv.ParseBool(v[0])
		case "errorifmissing":
			opts.ErrorIfMissing, _ = strconv.ParseBool(v[0])
		case "iteratorsamplingrate":
			opts.IteratorSamplingRate, _ = strconv.Atoi(v[0])
		case "nosync":
			opts.NoSync, _ = strconv.ParseBool(v[0])
		case "nowritemerge":
			opts.NoWriteMerge, _ = strconv.ParseBool(v[0])
		case "openfilescachecapacity":
			opts.OpenFilesCacheCapacity, _ = strconv.Atoi(v[0])
		case "readonly":
			opts.ReadOnly, _ = strconv.ParseBool(v[0])
		case "strict":
			val, _ := strconv.Atoi(v[0])
			opts.Strict = opt.Strict(val)
		case "writebuffer":
			opts.WriteBuffer, _ = strconv.Atoi(v[0])
		case "writel0pausetrigger":
			opts.WriteL0PauseTrigger, _ = strconv.Atoi(v[0])
		case "writel0slowdowntrigger":
			opts.WriteL0SlowdownTrigger, _ = strconv.Atoi(v[0])
		case "clientname":
			db.name = append(db.name, v[0])
		}
	}

	var err error
	db.db, err = leveldb.OpenFile(dataDir, opts)
	if err != nil {
		if !errors.IsCorrupted(err) {
			if strings.Contains(err.Error(), "resource temporarily unavailable") {
				err = fmt.Errorf("unable to lock level db at %s: %w", dataDir, err)
				return nil, err
			}

			err = fmt.Errorf("unable to open level db at %s: %w", dataDir, err)
			return nil, err
		}
		db.db, err = leveldb.RecoverFile(dataDir, opts)
	}

	if err != nil {
		return nil, err
	}

	for _, name := range db.name {
		m.LevelDBConnections[name] = db
	}
	db.count++
	return db.db, nil
}
