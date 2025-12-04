// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
//go:generate go run main.go ../../

package main

import (
	"bytes"
	encjson "encoding/json" //nolint:depguard // this package wraps it
	"errors"
	"fmt"
	"iter"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"code.gitea.io/gitea/modules/json"
)

type Pair struct {
	Key   string
	Value any
}

type OrderedMap struct {
	Pairs   []Pair
	indices map[string]int
}

func (o OrderedMap) Get(key string) (any, bool) {
	if _, ok := o.indices[key]; ok {
		return o.Pairs[o.indices[key]].Value, true
	}
	return nil, false
}

func (o *OrderedMap) Set(key string, value any) {
	if _, ok := o.indices[key]; ok {
		o.Pairs[o.indices[key]] = Pair{key, value}
	} else {
		o.Pairs = append(o.Pairs, Pair{key, value})
		o.indices[key] = len(o.Pairs) - 1
	}
}

func (o OrderedMap) Iter() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for _, it := range o.Pairs {
			yield(it.Key, it.Value)
		}
	}
}

func (o *OrderedMap) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		o.Pairs = nil
		o.indices = nil
		return nil
	}

	dec := encjson.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(encjson.Delim)
	if !ok || delim != '{' {
		return errors.New("OrderedMap: expected '{' at start of object")
	}

	// Reset storage
	if o.indices == nil {
		o.indices = make(map[string]int)
	} else {
		for k := range o.indices {
			delete(o.indices, k)
		}
	}
	o.Pairs = o.Pairs[:0]

	for dec.More() {
		tk, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := tk.(string)
		if !ok {
			return fmt.Errorf(
				"OrderedMap: expected string key, got %T (%v)",
				tk,
				tk,
			)
		}

		var raw encjson.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return fmt.Errorf("OrderedMap: decode value for %q: %w", key, err)
		}

		val, err := decodeJSONValue(raw)
		if err != nil {
			return fmt.Errorf("OrderedMap: unmarshal value for %q: %w", key, err)
		}

		if idx, exists := o.indices[key]; exists {
			o.Pairs[idx].Value = val
		} else {
			o.indices[key] = len(o.Pairs)
			o.Pairs = append(o.Pairs, Pair{Key: key, Value: val})
		}
	}

	end, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := end.(encjson.Delim); !ok || d != '}' {
		return errors.New("OrderedMap: expected '}' at end of object")
	}

	return nil
}

func decodeJSONValue(raw encjson.RawMessage) (any, error) {
	t := bytes.TrimSpace(raw)
	if bytes.Equal(t, []byte("null")) {
		return nil, nil
	}

	d := encjson.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return nil, err
	}

	switch tt := tok.(type) {
	case encjson.Delim:
		switch tt {
		case '{':
			var inner OrderedMap
			if err := inner.UnmarshalJSON(raw); err != nil {
				return nil, err
			}
			return inner, nil
		case '[':
			var arr []any
			for d.More() {
				var elemRaw encjson.RawMessage
				if err := d.Decode(&elemRaw); err != nil {
					return nil, err
				}
				v, err := decodeJSONValue(elemRaw)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if end, err := d.Token(); err != nil {
				return nil, err
			} else if end != encjson.Delim(']') {
				return nil, errors.New("expected ']'")
			}
			return arr, nil
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", tt)
		}
	default:
		var v any
		d = encjson.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil
	}
}

func (o OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("{")
	for i, kv := range o.Pairs {
		if i != 0 {
			buf.WriteString(",")
		}
		key, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(":")
		val, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}

	buf.WriteString("}")
	return buf.Bytes(), nil
}

var rxPath = regexp.MustCompile(`(?m)^(/repos/\{owner})/(\{repo})`)

func generatePaths(root string) *OrderedMap {
	pathData := &OrderedMap{
		indices: make(map[string]int),
	}
	endpoints := &OrderedMap{
		indices: make(map[string]int),
	}
	fileToRead, err := filepath.Rel(root, "./templates/swagger/v1_json.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	swaggerBytes, err := os.ReadFile(fileToRead)
	if err != nil {
		log.Fatal(err)
	}
	raw := OrderedMap{
		indices: make(map[string]int),
	}
	err = json.Unmarshal(swaggerBytes, &raw)
	if err != nil {
		log.Fatal(err)
	}
	rpaths, has := raw.Get("paths")
	if !has {
		log.Fatal("paths not found")
	}
	paths := rpaths.(OrderedMap)
	for k, v := range paths.Iter() {
		if !rxPath.MatchString(k) {
			// skip if this endpoint does not start with `/repos/{owner}/{repo}`
			continue
		}
		// generate new endpoint path with `/group/{group_id}` in between the `owner` and `repo` params
		nk := rxPath.ReplaceAllString(k, "$1/group/{group_id}/$2")
		methodMap := v.(OrderedMap)

		for method, methodSpec := range methodMap.Iter() {
			specMap := methodSpec.(OrderedMap)
			var params []OrderedMap
			aparams, has := specMap.Get("parameters")
			if !has {
				continue
			}
			rparams := aparams.([]any)
			for _, rparam := range rparams {
				params = append(params, rparam.(OrderedMap))
			}
			param := OrderedMap{
				indices: make(map[string]int),
			}
			param.Set("description", "group ID of the repo")
			param.Set("name", "group_id")
			param.Set("type", "integer")
			param.Set("format", "int64")
			param.Set("required", true)
			param.Set("in", "path")
			params = append(params, param)
			// i believe for...range loops create copies of each item that's iterated over,
			// so we need to take extra care to ensure we're mutating the original map entry
			specMap.Set("parameters", params)
			methodMap.Set(method, specMap)
			//(methodMap[method].(map[string]any))["parameters"] = params
		}
		endpoints.Set(nk, methodMap)
	}
	pathData.Set("paths", endpoints)
	return pathData
}

func writeMapToFile(filename string, data *OrderedMap) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	bytes = append(bytes, '\n')
	err = os.WriteFile(filename, bytes, 0o666)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	var err error
	root := "../../"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	err = os.Chdir(root)
	if err != nil {
		log.Fatal(err)
	}

	pathData := generatePaths(".")
	out := "./templates/swagger/v1_groups.json"
	writeMapToFile(out, pathData)
}
