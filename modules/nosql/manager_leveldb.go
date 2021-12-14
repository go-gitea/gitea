// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nosql

import (
	"path"
	"strconv"
	"strings"

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
		connection = ToLevelDBURI(connection).String()
		db, ok = m.LevelDBConnections[connection]
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
func (m *Manager) GetLevelDB(connection string) (*leveldb.DB, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	db, ok := m.LevelDBConnections[connection]
	if ok {
		db.count++

		return db.db, nil
	}
	uri := ToLevelDBURI(connection)
	db = &levelDBHolder{
		name: []string{connection, uri.String()},
	}

	dataDir := path.Join(uri.Host, uri.Path)
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
			return nil, err
		}
		db.db, err = leveldb.RecoverFile(dataDir, opts)
		if err != nil {
			return nil, err
		}
	}

	for _, name := range db.name {
		m.LevelDBConnections[name] = db
	}
	db.count++
	return db.db, nil
}
