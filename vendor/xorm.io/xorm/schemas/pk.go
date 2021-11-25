// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schemas

import (
	"bytes"
	"encoding/gob"

	"xorm.io/xorm/internal/utils"
)

// PK represents primary key values
type PK []interface{}

// NewPK creates primay keys
func NewPK(pks ...interface{}) *PK {
	p := PK(pks)
	return &p
}

// IsZero return true if primay keys are zero
func (p *PK) IsZero() bool {
	for _, k := range *p {
		if utils.IsZero(k) {
			return true
		}
	}
	return false
}

// ToString convert to SQL string
func (p *PK) ToString() (string, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(*p)
	return buf.String(), err
}

// FromString reads content to load primary keys
func (p *PK) FromString(content string) error {
	dec := gob.NewDecoder(bytes.NewBufferString(content))
	err := dec.Decode(p)
	return err
}
