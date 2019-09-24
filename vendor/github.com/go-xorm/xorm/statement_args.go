// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"fmt"

	"xorm.io/builder"
)

func writeArg(w *builder.BytesWriter, arg interface{}) error {
	switch argv := arg.(type) {
	case string:
		if _, err := w.WriteString("'" + argv + "'"); err != nil {
			return err
		}
	case *builder.Builder:
		if err := argv.WriteTo(w); err != nil {
			return err
		}
	default:
		if _, err := w.WriteString(fmt.Sprintf("%v", argv)); err != nil {
			return err
		}
	}
	return nil
}

func writeArgs(w *builder.BytesWriter, args []interface{}) error {
	for i, arg := range args {
		if err := writeArg(w, arg); err != nil {
			return err
		}

		if i+1 != len(args) {
			if _, err := w.WriteString(","); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeStrings(w *builder.BytesWriter, cols []string, leftQuote, rightQuote string) error {
	for i, colName := range cols {
		if len(leftQuote) > 0 && colName[0] != '`' {
			if _, err := w.WriteString(leftQuote); err != nil {
				return err
			}
		}
		if _, err := w.WriteString(colName); err != nil {
			return err
		}
		if len(rightQuote) > 0 && colName[len(colName)-1] != '`' {
			if _, err := w.WriteString(rightQuote); err != nil {
				return err
			}
		}
		if i+1 != len(cols) {
			if _, err := w.WriteString(","); err != nil {
				return err
			}
		}
	}
	return nil
}
