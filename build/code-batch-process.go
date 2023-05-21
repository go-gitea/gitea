// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/build/codeformat"
)

// Windows has a limitation for command line arguments, the size can not exceed 32KB.
// So we have to feed the files to some tools (like gofmt) batch by batch

// We also introduce a `gitea-fmt` command, it does better import formatting than gofmt/goimports. `gitea-fmt` calls `gofmt` internally.

var optionLogVerbose bool

func logVerbose(msg string, args ...interface{}) {
	if optionLogVerbose {
		log.Printf(msg, args...)
	}
}

func passThroughCmd(cmd string, args []string) error {
	foundCmd, err := exec.LookPath(cmd)
	if err != nil {
		log.Fatalf("can not find cmd: %s", cmd)
	}
	c := exec.Cmd{
		Path:   foundCmd,
		Args:   append([]string{cmd}, args...),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return c.Run()
}

type fileCollector struct {
	dirs            []string
	includePatterns []*regexp.Regexp
	excludePatterns []*regexp.Regexp
	batchSize       int
}

func newFileCollector(fileFilter string, batchSize int) (*fileCollector, error) {
	co := &fileCollector{batchSize: batchSize}
	if fileFilter == "go-own" {
		co.dirs = []string{
			"build",
			"cmd",
			"contrib",
			"tests",
			"models",
			"modules",
			"routers",
			"services",
		}
		co.includePatterns = append(co.includePatterns, regexp.MustCompile(`.*\.go$`))

		co.excludePatterns = append(co.excludePatterns, regexp.MustCompile(`.*\bbindata\.go$`))
		co.excludePatterns = append(co.excludePatterns, regexp.MustCompile(`tests/gitea-repositories-meta`))
		co.excludePatterns = append(co.excludePatterns, regexp.MustCompile(`tests/integration/migration-test`))
		co.excludePatterns = append(co.excludePatterns, regexp.MustCompile(`modules/git/tests`))
		co.excludePatterns = append(co.excludePatterns, regexp.MustCompile(`models/fixtures`))
		co.excludePatterns = append(co.excludePatterns, regexp.MustCompile(`models/migrations/fixtures`))
		co.excludePatterns = append(co.excludePatterns, regexp.MustCompile(`services/gitdiff/testdata`))
	}

	if co.dirs == nil {
		return nil, fmt.Errorf("unknown file-filter: %s", fileFilter)
	}
	return co, nil
}

func (fc *fileCollector) matchPatterns(path string, regexps []*regexp.Regexp) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	for _, re := range regexps {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func (fc *fileCollector) collectFiles() (res [][]string, err error) {
	var batch []string
	for _, dir := range fc.dirs {
		err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			include := len(fc.includePatterns) == 0 || fc.matchPatterns(path, fc.includePatterns)
			exclude := fc.matchPatterns(path, fc.excludePatterns)
			process := include && !exclude
			if !process {
				if d.IsDir() {
					if exclude {
						logVerbose("exclude dir %s", path)
						return filepath.SkipDir
					}
					// for a directory, if it is not excluded explicitly, we should walk into
					return nil
				}
				// for a file, we skip it if it shouldn't be processed
				logVerbose("skip process %s", path)
				return nil
			}
			if d.IsDir() {
				// skip dir, we don't add dirs to the file list now
				return nil
			}
			if len(batch) >= fc.batchSize {
				res = append(res, batch)
				batch = nil
			}
			batch = append(batch, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	res = append(res, batch)
	return res, nil
}

// substArgFiles expands the {file-list} to a real file list for commands
func substArgFiles(args, files []string) []string {
	for i, s := range args {
		if s == "{file-list}" {
			newArgs := append(args[:i], files...)
			newArgs = append(newArgs, args[i+1:]...)
			return newArgs
		}
	}
	return args
}

func exitWithCmdErrors(subCmd string, subArgs []string, cmdErrors []error) {
	for _, err := range cmdErrors {
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode := exitError.ExitCode()
				log.Printf("run command failed (code=%d): %s %v", exitCode, subCmd, subArgs)
				os.Exit(exitCode)
			} else {
				log.Fatalf("run command failed (err=%s) %s %v", err, subCmd, subArgs)
			}
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
	fmt.Printf(`Usage: %[1]s [options] {command} [arguments]

Options:
  --verbose
  --file-filter=go-own
  --batch-size=100

Commands:
  %[1]s gofmt ...

Arguments:
  {file-list}     the file list

Example:
  %[1]s gofmt -s -d {file-list}

`, "file-batch-exec")
}

func getGoVersion() string {
	goModFile, err := os.ReadFile("go.mod")
	if err != nil {
		log.Fatalf(`Faild to read "go.mod": %v`, err)
		os.Exit(1)
	}
	goModVersionRegex := regexp.MustCompile(`go \d+\.\d+`)
	goModVersionLine := goModVersionRegex.Find(goModFile)
	return string(goModVersionLine[3:])
}

func newFileCollectorFromMainOptions(mainOptions map[string]string) (fc *fileCollector, err error) {
	fileFilter := mainOptions["file-filter"]
	if fileFilter == "" {
		fileFilter = "go-own"
	}
	batchSize, _ := strconv.Atoi(mainOptions["batch-size"])
	if batchSize == 0 {
		batchSize = 100
	}

	return newFileCollector(fileFilter, batchSize)
}

func containsString(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

func giteaFormatGoImports(files []string, doWriteFile bool) error {
	for _, file := range files {
		if err := codeformat.FormatGoImports(file, doWriteFile); err != nil {
			log.Printf("failed to format go imports: %s, err=%v", file, err)
			return err
		}
	}
	return nil
}

func main() {
	mainOptions, subCmd, subArgs := parseArgs()
	if subCmd == "" {
		showUsage()
		os.Exit(1)
	}
	optionLogVerbose = mainOptions["verbose"] != ""

	fc, err := newFileCollectorFromMainOptions(mainOptions)
	if err != nil {
		log.Fatalf("can not create file collector: %s", err.Error())
	}

	fileBatches, err := fc.collectFiles()
	if err != nil {
		log.Fatalf("can not collect files: %s", err.Error())
	}

	processed := 0
	var cmdErrors []error
	for _, files := range fileBatches {
		if len(files) == 0 {
			break
		}
		substArgs := substArgFiles(subArgs, files)
		logVerbose("batch cmd: %s %v", subCmd, substArgs)
		switch subCmd {
		case "gitea-fmt":
			if containsString(subArgs, "-d") {
				log.Print("the -d option is not supported by gitea-fmt")
			}
			cmdErrors = append(cmdErrors, giteaFormatGoImports(files, containsString(subArgs, "-w")))
			cmdErrors = append(cmdErrors, passThroughCmd("go", append([]string{"run", os.Getenv("GOFUMPT_PACKAGE"), "-extra", "-lang", getGoVersion()}, substArgs...)))
		default:
			log.Fatalf("unknown cmd: %s %v", subCmd, subArgs)
		}
		processed += len(files)
	}

	logVerbose("processed %d files", processed)
	exitWithCmdErrors(subCmd, subArgs, cmdErrors)
}
