// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"encoding/json"
	"reflect"
)

// toConfig will attempt to convert a given configuration cfg into the provided exemplar type.
//
// It will tolerate the cfg being passed as a []byte or string of a json representation of the
// exemplar or the correct type of the exemplar itself
func toConfig(exemplar, cfg interface{}) (interface{}, error) {
	if reflect.TypeOf(cfg).AssignableTo(reflect.TypeOf(exemplar)) {
		return cfg, nil
	}

	configBytes, ok := cfg.([]byte)
	if !ok {
		configStr, ok := cfg.(string)
		if !ok {
			return nil, ErrInvalidConfiguration{cfg: cfg}
		}
		configBytes = []byte(configStr)
	}
	newVal := reflect.New(reflect.TypeOf(exemplar))
	if err := json.Unmarshal(configBytes, newVal.Interface()); err != nil {
		return nil, ErrInvalidConfiguration{cfg: cfg, err: err}
	}
	return newVal.Elem().Interface(), nil
}

// unmarshalAs will attempt to unmarshal provided bytes as the provided exemplar
func unmarshalAs(bs []byte, exemplar interface{}) (data Data, err error) {
	if exemplar != nil {
		t := reflect.TypeOf(exemplar)
		n := reflect.New(t)
		ne := n.Elem()
		err = json.Unmarshal(bs, ne.Addr().Interface())
		data = ne.Interface().(Data)
	} else {
		err = json.Unmarshal(bs, &data)
	}

	return
}

// assignableTo will check if provided data is assignable to the same type as the exemplar
// if the provided exemplar is nil then it will always return true
func assignableTo(data Data, exemplar interface{}) bool {
	if exemplar == nil {
		return true
	}

	// Assert data is of same type as exemplar
	t := reflect.TypeOf(data)
	exemplarType := reflect.TypeOf(exemplar)

	return t.AssignableTo(exemplarType) && data != nil
}
