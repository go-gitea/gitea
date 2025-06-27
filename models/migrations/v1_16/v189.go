// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"encoding/binary"
	"fmt"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/json"

	"xorm.io/xorm"
)

func UnwrapLDAPSourceCfg(x *xorm.Engine) error {
	jsonUnmarshalHandleDoubleEncode := func(bs []byte, v any) error {
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
				if rs[0] == 0xff && rs[1] == 0xfe {
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

	// LoginSource represents an external way for authorizing users.
	type LoginSource struct {
		ID        int64 `xorm:"pk autoincr"`
		Type      int
		IsActived bool   `xorm:"INDEX NOT NULL DEFAULT false"`
		IsActive  bool   `xorm:"INDEX NOT NULL DEFAULT false"`
		Cfg       string `xorm:"TEXT"`
	}

	const ldapType = 2
	const dldapType = 5

	type WrappedSource struct {
		Source map[string]any
	}

	// change lower_email as unique
	if err := x.Sync(new(LoginSource)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()

	const batchSize = 100
	for start := 0; ; start += batchSize {
		sources := make([]*LoginSource, 0, batchSize)
		if err := sess.Limit(batchSize, start).Where("`type` = ? OR `type` = ?", ldapType, dldapType).Find(&sources); err != nil {
			return err
		}
		if len(sources) == 0 {
			break
		}

		for _, source := range sources {
			wrapped := &WrappedSource{
				Source: map[string]any{},
			}
			err := jsonUnmarshalHandleDoubleEncode([]byte(source.Cfg), &wrapped)
			if err != nil {
				return fmt.Errorf("failed to unmarshal %s: %w", source.Cfg, err)
			}
			if len(wrapped.Source) > 0 {
				bs, err := json.Marshal(wrapped.Source)
				if err != nil {
					return err
				}
				source.Cfg = string(bs)
				if _, err := sess.ID(source.ID).Cols("cfg").Update(source); err != nil {
					return err
				}
			}
		}
	}

	if _, err := x.SetExpr("is_active", "is_actived").Update(&LoginSource{}); err != nil {
		return fmt.Errorf("SetExpr Update failed:  %w", err)
	}

	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "login_source", "is_actived"); err != nil {
		return err
	}

	return sess.Commit()
}
