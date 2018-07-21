// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package builder

import (
	"errors"
	"fmt"
)

func (b *Builder) selectWriteTo(w Writer) error {
	if len(b.tableName) <= 0 {
		return errors.New("no table indicated")
	}

	if _, err := fmt.Fprint(w, "SELECT "); err != nil {
		return err
	}
	if len(b.selects) > 0 {
		for i, s := range b.selects {
			if _, err := fmt.Fprint(w, s); err != nil {
				return err
			}
			if i != len(b.selects)-1 {
				if _, err := fmt.Fprint(w, ","); err != nil {
					return err
				}
			}
		}
	} else {
		if _, err := fmt.Fprint(w, "*"); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(w, " FROM ", b.tableName); err != nil {
		return err
	}

	for _, v := range b.joins {
		if _, err := fmt.Fprintf(w, " %s JOIN %s ON ", v.joinType, v.joinTable); err != nil {
			return err
		}

		if err := v.joinCond.WriteTo(w); err != nil {
			return err
		}
	}

	if b.cond.IsValid() {
		if _, err := fmt.Fprint(w, " WHERE "); err != nil {
			return err
		}

		if err := b.cond.WriteTo(w); err != nil {
			return err
		}
	}

	if len(b.groupBy) > 0 {
		if _, err := fmt.Fprint(w, " GROUP BY ", b.groupBy); err != nil {
			return err
		}
	}

	if len(b.having) > 0 {
		if _, err := fmt.Fprint(w, " HAVING ", b.having); err != nil {
			return err
		}
	}

	if len(b.orderBy) > 0 {
		if _, err := fmt.Fprint(w, " ORDER BY ", b.orderBy); err != nil {
			return err
		}
	}

	return nil
}

// OrderBy orderBy SQL
func (b *Builder) OrderBy(orderBy string) *Builder {
	b.orderBy = orderBy
	return b
}

// GroupBy groupby SQL
func (b *Builder) GroupBy(groupby string) *Builder {
	b.groupBy = groupby
	return b
}

// Having having SQL
func (b *Builder) Having(having string) *Builder {
	b.having = having
	return b
}
