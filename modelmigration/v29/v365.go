// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"encoding/binary"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/json"
	"gitea.dev/modules/util"
)

// unmarshalHandleDoubleEncode - due to a bug in xorm (see https://gitea.com/xorm/xorm/pulls/1957) - it's
// possible that a Blob may be double encoded or gain an unwanted prefix of 0xff 0xfe.
func unmarshalHandleDoubleEncode(bs []byte, v any) error {
	if len(bs) == 0 {
		// json.Unmarshal will report errors if input is empty (nil or zero-length)
		// It seems that XORM ignores the nil but still passes zero-length string into this function
		// To be consistent, we should treat all empty inputs as success
		return nil
	}
	err := json.Unmarshal(bs, v)
	if err != nil {
		ok := true
		rs := []byte{}
		temp := make([]byte, 2)
		for _, rn := range string(bs) {
			if rn > 0xffff {
				ok = false
				break
			}
			binary.LittleEndian.PutUint16(temp, uint16(rn))
			rs = append(rs, temp...)
		}
		if ok {
			if len(rs) > 1 && rs[0] == 0xff && rs[1] == 0xfe {
				rs = rs[2:]
			}
			err = json.Unmarshal(rs, v)
		}
	}
	if err != nil && len(bs) > 2 && bs[0] == 0xff && bs[1] == 0xfe {
		err = json.Unmarshal(bs[2:], v)
	}
	return err
}

func normalizeLegacyConfigJSON(config string) string {
	if config == "" || json.Valid(util.UnsafeStringToBytes(config)) {
		return config
	}

	m := map[any]any{}
	err := unmarshalHandleDoubleEncode(util.UnsafeStringToBytes(config), &m)
	if err != nil {
		return config
	}
	if bs, err := json.Marshal(m); err == nil {
		return string(bs)
	}
	return config
}

func NormalizeLegacyConfigJSON(ctx context.Context, x base.EngineMigration) error {
	type RepoUnit struct {
		ID     int64
		Config string
	}
	type LoginSource struct {
		ID  int64
		Cfg string
	}
	return base.EngineTransaction(x, func(sess base.Session) error {
		if err := base.IterateTableByColumn(ctx, x, sess, "id", nil, func(_ context.Context, unit *RepoUnit) error {
			config := normalizeLegacyConfigJSON(unit.Config)
			if config == unit.Config {
				return nil
			}
			_, err := sess.ID(unit.ID).Cols("config").Update(&RepoUnit{Config: config})
			return err
		}); err != nil {
			return err
		}
		return base.IterateTableByColumn(ctx, x, sess, "id", nil, func(_ context.Context, source *LoginSource) error {
			config := normalizeLegacyConfigJSON(source.Cfg)
			if config == source.Cfg {
				return nil
			}
			_, err := sess.ID(source.ID).Cols("cfg").Update(&LoginSource{Cfg: config})
			return err
		})
	})
}
