// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unittest

import (
	"reflect"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/unittestbridge"
)

var (
	consistencyMap = make(map[string]func(ta unittestbridge.Asserter, bean interface{}))
)

// RegisterConsistencyFunc register consistency check function
func RegisterConsistencyFunc(tableName string, f func(ta unittestbridge.Asserter, bean interface{})) {
	consistencyMap[tableName] = f
}

// CheckConsistencyFor test that all matching database entries are consistent
func CheckConsistencyFor(t unittestbridge.Tester, beansToCheck ...interface{}) {
	ta := unittestbridge.NewAsserter(t)
	for _, bean := range beansToCheck {
		sliceType := reflect.SliceOf(reflect.TypeOf(bean))
		sliceValue := reflect.MakeSlice(sliceType, 0, 10)

		ptrToSliceValue := reflect.New(sliceType)
		ptrToSliceValue.Elem().Set(sliceValue)

		ta.NoError(db.GetEngine(db.DefaultContext).Table(bean).Find(ptrToSliceValue.Interface()))
		sliceValue = ptrToSliceValue.Elem()

		for i := 0; i < sliceValue.Len(); i++ {
			entity := sliceValue.Index(i).Interface()
			checkForConsistency(ta, entity)
		}
	}
}

func checkForConsistency(ta unittestbridge.Asserter, bean interface{}) {
	tb, err := db.TableInfo(bean)
	ta.NoError(err)
	f := consistencyMap[tb.Name]
	if f == nil {
		ta.Errorf("unknown bean type: %#v", bean)
		return
	}
	f(ta, bean)
}
