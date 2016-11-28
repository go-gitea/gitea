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

package structure

import (
	"bytes"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/util/codec"
)

// TypeFlag is for data structure meta/data flag.
type TypeFlag byte

const (
	// StringMeta is the flag for string meta.
	StringMeta TypeFlag = 'S'
	// StringData is the flag for string data.
	StringData TypeFlag = 's'
	// HashMeta is the flag for hash meta.
	HashMeta TypeFlag = 'H'
	// HashData is the flag for hash data.
	HashData TypeFlag = 'h'
	// ListMeta is the flag for list meta.
	ListMeta TypeFlag = 'L'
	// ListData is the flag for list data.
	ListData TypeFlag = 'l'
)

func (t *TxStructure) encodeStringDataKey(key []byte) kv.Key {
	// for codec Encode, we may add extra bytes data, so here and following encode
	// we will use extra length like 4 for a little optimization.
	ek := make([]byte, 0, len(t.prefix)+len(key)+24)
	ek = append(ek, t.prefix...)
	ek = codec.EncodeBytes(ek, key)
	return codec.EncodeUint(ek, uint64(StringData))
}

func (t *TxStructure) encodeHashMetaKey(key []byte) kv.Key {
	ek := make([]byte, 0, len(t.prefix)+len(key)+24)
	ek = append(ek, t.prefix...)
	ek = codec.EncodeBytes(ek, key)
	return codec.EncodeUint(ek, uint64(HashMeta))
}

func (t *TxStructure) encodeHashDataKey(key []byte, field []byte) kv.Key {
	ek := make([]byte, 0, len(t.prefix)+len(key)+len(field)+30)
	ek = append(ek, t.prefix...)
	ek = codec.EncodeBytes(ek, key)
	ek = codec.EncodeUint(ek, uint64(HashData))
	return codec.EncodeBytes(ek, field)
}

func (t *TxStructure) decodeHashDataKey(ek kv.Key) ([]byte, []byte, error) {
	var (
		key   []byte
		field []byte
		err   error
		tp    uint64
	)

	if !bytes.HasPrefix(ek, t.prefix) {
		return nil, nil, errors.New("invalid encoded hash data key prefix")
	}

	ek = ek[len(t.prefix):]

	ek, key, err = codec.DecodeBytes(ek)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	ek, tp, err = codec.DecodeUint(ek)
	if err != nil {
		return nil, nil, errors.Trace(err)
	} else if TypeFlag(tp) != HashData {
		return nil, nil, errors.Errorf("invalid encoded hash data key flag %c", byte(tp))
	}

	_, field, err = codec.DecodeBytes(ek)
	return key, field, errors.Trace(err)
}

func (t *TxStructure) hashDataKeyPrefix(key []byte) kv.Key {
	ek := make([]byte, 0, len(t.prefix)+len(key)+24)
	ek = append(ek, t.prefix...)
	ek = codec.EncodeBytes(ek, key)
	return codec.EncodeUint(ek, uint64(HashData))
}

func (t *TxStructure) encodeListMetaKey(key []byte) kv.Key {
	ek := make([]byte, 0, len(t.prefix)+len(key)+24)
	ek = append(ek, t.prefix...)
	ek = codec.EncodeBytes(ek, key)
	return codec.EncodeUint(ek, uint64(ListMeta))
}

func (t *TxStructure) encodeListDataKey(key []byte, index int64) kv.Key {
	ek := make([]byte, 0, len(t.prefix)+len(key)+36)
	ek = append(ek, t.prefix...)
	ek = codec.EncodeBytes(ek, key)
	ek = codec.EncodeUint(ek, uint64(ListData))
	return codec.EncodeInt(ek, index)
}
