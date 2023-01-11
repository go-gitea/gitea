// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright (c) 2015, Wade Simmons
// SPDX-License-Identifier: MIT

// gocovmerge takes the results from multiple `go test -coverprofile` runs and
// merges them into one profile

//go:build ignore

package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/cover"
)

func mergeProfiles(p, merge *cover.Profile) {
	if p.Mode != merge.Mode {
		log.Fatalf("cannot merge profiles with different modes")
	}
	// Since the blocks are sorted, we can keep track of where the last block
	// was inserted and only look at the blocks after that as targets for merge
	startIndex := 0
	for _, b := range merge.Blocks {
		startIndex = mergeProfileBlock(p, b, startIndex)
	}
}

func mergeProfileBlock(p *cover.Profile, pb cover.ProfileBlock, startIndex int) int {
	sortFunc := func(i int) bool {
		pi := p.Blocks[i+startIndex]
		return pi.StartLine >= pb.StartLine && (pi.StartLine != pb.StartLine || pi.StartCol >= pb.StartCol)
	}

	i := 0
	if sortFunc(i) != true {
		i = sort.Search(len(p.Blocks)-startIndex, sortFunc)
	}
	i += startIndex
	if i < len(p.Blocks) && p.Blocks[i].StartLine == pb.StartLine && p.Blocks[i].StartCol == pb.StartCol {
		if p.Blocks[i].EndLine != pb.EndLine || p.Blocks[i].EndCol != pb.EndCol {
			log.Fatalf("OVERLAP MERGE: %v %v %v", p.FileName, p.Blocks[i], pb)
		}
		switch p.Mode {
		case "set":
			p.Blocks[i].Count |= pb.Count
		case "count", "atomic":
			p.Blocks[i].Count += pb.Count
		default:
			log.Fatalf("unsupported covermode: '%s'", p.Mode)
		}
	} else {
		if i > 0 {
			pa := p.Blocks[i-1]
			if pa.EndLine >= pb.EndLine && (pa.EndLine != pb.EndLine || pa.EndCol > pb.EndCol) {
				log.Fatalf("OVERLAP BEFORE: %v %v %v", p.FileName, pa, pb)
			}
		}
		if i < len(p.Blocks)-1 {
			pa := p.Blocks[i+1]
			if pa.StartLine <= pb.StartLine && (pa.StartLine != pb.StartLine || pa.StartCol < pb.StartCol) {
				log.Fatalf("OVERLAP AFTER: %v %v %v", p.FileName, pa, pb)
			}
		}
		p.Blocks = append(p.Blocks, cover.ProfileBlock{})
		copy(p.Blocks[i+1:], p.Blocks[i:])
		p.Blocks[i] = pb
	}
	return i + 1
}

func addProfile(profiles []*cover.Profile, p *cover.Profile) []*cover.Profile {
	i := sort.Search(len(profiles), func(i int) bool { return profiles[i].FileName >= p.FileName })
	if i < len(profiles) && profiles[i].FileName == p.FileName {
		mergeProfiles(profiles[i], p)
	} else {
		profiles = append(profiles, nil)
		copy(profiles[i+1:], profiles[i:])
		profiles[i] = p
	}
	return profiles
}

func dumpProfiles(profiles []*cover.Profile, out io.Writer) {
	if len(profiles) == 0 {
		return
	}
	fmt.Fprintf(out, "mode: %s\n", profiles[0].Mode)
	for _, p := range profiles {
		for _, b := range p.Blocks {
			fmt.Fprintf(out, "%s:%d.%d,%d.%d %d %d\n", p.FileName, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.NumStmt, b.Count)
		}
	}
}

func parseArgs() (mainOptions map[string]string, subCmd string, subArgs []string) {
	mainOptions = map[string]string{}
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "" {
			break
		}
		if arg[0] == '-' {
			arg = strings.TrimPrefix(arg, "-")
			arg = strings.TrimPrefix(arg, "-")
			fields := strings.SplitN(arg, "=", 2)
			if len(fields) == 1 {
				mainOptions[fields[0]] = "1"
			} else {
				mainOptions[fields[0]] = fields[1]
			}
		} else {
			subCmd = arg
			subArgs = os.Args[i+1:]
			break
		}
	}
	return
}

func showUsage() {
	fmt.Printf(`Usage: %[1]s {command} [arguments]

Commands:
  %[1]s merge ...
  %[1]s check [coverage_file] [package=percent] ...

Arguments:
  {file-list}     the file list

Example:
  %[1]s merge -s -d {file-list}
	%[1]s check coverage.out code.gitea.io/gitea/modules/setting=4%%

`, "gocoverage")
}

func merge(args []string) {
	var merged []*cover.Profile

	for _, file := range args {
		profiles, err := cover.ParseProfiles(file)
		if err != nil {
			log.Fatalf("failed to parse profile '%s': %v", file, err)
		}
		for _, p := range profiles {
			merged = addProfile(merged, p)
		}
	}

	dumpProfiles(merged, os.Stdout)
}

func percentToInt64(percent string) int64 {
	value := strings.ReplaceAll(percent, "%", "")
	i, err := strconv.ParseFloat(value, 10)
	if err != nil {
		log.Fatalf("invalid percent: %s", percent)
	}
	return int64(i * 10)
}

func profileCount(p *cover.Profile) (int64, int64) {
	blocks := p.Blocks
	var active, total int64
	for i := range blocks {
		stmts := int64(blocks[i].NumStmt)
		total += stmts
		if blocks[i].Count > 0 {
			active += stmts
		}
	}
	return total, active
}

func checkPackages(args []string) {
	if len(args) < 2 {
		log.Fatalf("invalid arguments: %v", args)
		return
	}
	coverageFile, packages := args[0], args[1:]
	profiles, err := cover.ParseProfiles(coverageFile)
	if err != nil {
		log.Fatalf("failed to parse profile '%s': %v", coverageFile, err)
	}
	packagesRequirements := make(map[string]int64)
	for _, p := range packages {
		parts := strings.Split(p, "=")
		if len(parts) != 2 {
			continue
		}
		packagesRequirements[parts[0]] = percentToInt64(parts[1])
	}
	packagesTotals := make(map[string]int64)
	packagesActives := make(map[string]int64)
	for _, p := range profiles {
		pkg := path.Dir(p.FileName)
		_, ok := packagesRequirements[pkg]
		if !ok {
			continue
		}
		total, active := profileCount(p)
		packagesTotals[pkg] += total
		packagesActives[pkg] += active
	}
	var failed bool
	for k, v := range packagesRequirements {
		actual := 100 * float64(packagesActives[k]) / float64(packagesTotals[k])
		if v > int64(math.Floor(actual*10+0.5)) {
			log.Printf("package %s coverage is %.1f%%, required %.1f%%\n", k, actual, float64(v)/10.0)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func main() {
	_, subCmd, subArgs := parseArgs()
	if subCmd == "" {
		showUsage()
		os.Exit(1)
	}

	switch subCmd {
	case "merge":
		merge(subArgs)
	case "check":
		checkPackages(subArgs)
	}
}
