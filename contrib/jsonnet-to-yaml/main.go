// Copyright 2017 Google Inc. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/sergi/go-diff/diffmatchpatch"
	"gopkg.in/yaml.v3"
)

func main() {
	inputFlag := flag.String("input", ".drone.jsonnet", "Input file")
	flag.StringVar(inputFlag, "i", *inputFlag, "--input")
	outputFlag := flag.String("output", ".drone.yml", "Output file")
	flag.StringVar(outputFlag, "o", *outputFlag, "--output")
	compareFlag := flag.Bool("compare", false, "Compare input and output")
	htmlFlag := flag.Bool("html", false, "Output HTML")
	flag.Parse()

	source, err := os.ReadFile(*inputFlag)
	if err != nil {
		panic(err)
	}
	vm := jsonnet.MakeVM()
	streams, err := vm.EvaluateAnonymousSnippetStream(*inputFlag, string(source))
	if err != nil {
		panic(err)
	}

	var jsonnetDoc strings.Builder
	jsonnetDoc.WriteString("---\n")
	enc := yaml.NewEncoder(&jsonnetDoc)
	enc.SetIndent(2)
	for _, stream := range streams {
		var j any
		if err := yaml.Unmarshal([]byte(stream), &j); err != nil {
			panic(err)
		}

		if err := enc.Encode(j); err != nil {
			panic(err)
		}
	}

	if *compareFlag {
		diff, err := compare(jsonnetDoc.String(), *outputFlag, *htmlFlag)
		if err != nil {
			panic(err)
		}
		exit := 0
		if len(diff) > 0 {
			fmt.Println(diff)
			exit = 1
		}
		os.Exit(exit)
	}

	fi, err := os.Create(*outputFlag)
	if err != nil {
		panic(err)
	}
	defer fi.Close()
	if _, err := fi.WriteString(jsonnetDoc.String()); err != nil {
		panic(err)
	}
}

func compare(eval, outputFile string, diffHTML bool) (string, error) {
	curSource, err := os.ReadFile(outputFile)
	if err != nil {
		return "", err
	}

	var currentDoc strings.Builder
	currentDoc.WriteString("---\n")
	dec := yaml.NewDecoder(bytes.NewReader(curSource))
	enc := yaml.NewEncoder(&currentDoc)
	enc.SetIndent(2)
	for {
		var y any
		err := dec.Decode(&y)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return "", nil
			}
			break
		}
		if err := enc.Encode(y); err != nil {
			return "", err
		}
	}

	if eval == currentDoc.String() {
		return "", nil
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(currentDoc.String(), eval, false)
	if diffHTML {
		return dmp.DiffPrettyHtml(diffs), nil
	}
	return dmp.DiffPrettyText(diffs), nil
}
