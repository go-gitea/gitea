// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
//go:generate go run main.go ../../

package main

import (
	"bytes"
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

func (o OrderedMap) Get(key string) (bool, any) {
	if _, ok := o.indices[key]; ok {
		return true, o.Pairs[o.indices[key]].Value
	}
	return false, nil
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

func innerConvert(it any) any {
	switch v := it.(type) {
	case map[string]any:
		return mapToOrderedMap(v)
	case []any:
		for i := range v {
			v[i] = innerConvert(v[i])
		}
		return v
	default:
		return v
	}
}

func mapToOrderedMap(m map[string]any) OrderedMap {
	var om OrderedMap
	om.indices = make(map[string]int)
	i := 0
	for k, v := range m {
		om.Pairs = append(om.Pairs, Pair{k, innerConvert(v)})
		om.indices[k] = i
		i++
	}
	return om
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
	raw := make(map[string]any)
	err = json.Unmarshal(swaggerBytes, &raw)
	if err != nil {
		log.Fatal(err)
	}
	paths := mapToOrderedMap(raw["paths"].(map[string]any))
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
			has, aparams := specMap.Get("parameters")
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
