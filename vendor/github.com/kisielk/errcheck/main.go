package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/kisielk/errcheck/internal/errcheck"
	"github.com/kisielk/gotool"
)

const (
	exitCodeOk int = iota
	exitUncheckedError
	exitFatalError
)

var abspath bool

type ignoreFlag map[string]*regexp.Regexp

func (f ignoreFlag) String() string {
	pairs := make([]string, 0, len(f))
	for pkg, re := range f {
		prefix := ""
		if pkg != "" {
			prefix = pkg + ":"
		}
		pairs = append(pairs, prefix+re.String())
	}
	return fmt.Sprintf("%q", strings.Join(pairs, ","))
}

func (f ignoreFlag) Set(s string) error {
	if s == "" {
		return nil
	}
	for _, pair := range strings.Split(s, ",") {
		colonIndex := strings.Index(pair, ":")
		var pkg, re string
		if colonIndex == -1 {
			pkg = ""
			re = pair
		} else {
			pkg = pair[:colonIndex]
			re = pair[colonIndex+1:]
		}
		regex, err := regexp.Compile(re)
		if err != nil {
			return err
		}
		f[pkg] = regex
	}
	return nil
}

type tagsFlag []string

func (f *tagsFlag) String() string {
	return fmt.Sprintf("%q", strings.Join(*f, " "))
}

func (f *tagsFlag) Set(s string) error {
	if s == "" {
		return nil
	}
	tags := strings.Split(s, " ")
	if tags == nil {
		return nil
	}
	for _, tag := range tags {
		if tag != "" {
			*f = append(*f, tag)
		}
	}
	return nil
}

var dotStar = regexp.MustCompile(".*")

func reportUncheckedErrors(e *errcheck.UncheckedErrors, verbose bool) {
	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}
	for _, uncheckedError := range e.Errors {
		pos := uncheckedError.Pos.String()
		if !abspath {
			newPos, err := filepath.Rel(wd, pos)
			if err == nil {
				pos = newPos
			}
		}

		if verbose && uncheckedError.FuncName != "" {
			fmt.Printf("%s:\t%s\t%s\n", pos, uncheckedError.FuncName, uncheckedError.Line)
		} else {
			fmt.Printf("%s:\t%s\n", pos, uncheckedError.Line)
		}
	}
}

func mainCmd(args []string) int {
	runtime.GOMAXPROCS(runtime.NumCPU())

	checker := errcheck.NewChecker()
	paths, err := parseFlags(checker, args)
	if err != exitCodeOk {
		return err
	}

	if err := checker.CheckPackages(paths...); err != nil {
		if e, ok := err.(*errcheck.UncheckedErrors); ok {
			reportUncheckedErrors(e, checker.Verbose)
			return exitUncheckedError
		} else if err == errcheck.ErrNoGoFiles {
			fmt.Fprintln(os.Stderr, err)
			return exitCodeOk
		}
		fmt.Fprintf(os.Stderr, "error: failed to check packages: %s\n", err)
		return exitFatalError
	}
	return exitCodeOk
}

func parseFlags(checker *errcheck.Checker, args []string) ([]string, int) {
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.BoolVar(&checker.Blank, "blank", false, "if true, check for errors assigned to blank identifier")
	flags.BoolVar(&checker.Asserts, "asserts", false, "if true, check for ignored type assertion results")
	flags.BoolVar(&checker.WithoutTests, "ignoretests", false, "if true, checking of _test.go files is disabled")
	flags.BoolVar(&checker.Verbose, "verbose", false, "produce more verbose logging")

	flags.BoolVar(&abspath, "abspath", false, "print absolute paths to files")

	tags := tagsFlag{}
	flags.Var(&tags, "tags", "space-separated list of build tags to include")
	ignorePkg := flags.String("ignorepkg", "", "comma-separated list of package paths to ignore")
	ignore := ignoreFlag(map[string]*regexp.Regexp{})
	flags.Var(ignore, "ignore", "[deprecated] comma-separated list of pairs of the form pkg:regex\n"+
		"            the regex is used to ignore names within pkg.")

	var excludeFile string
	flags.StringVar(&excludeFile, "exclude", "", "Path to a file containing a list of functions to exclude from checking")

	if err := flags.Parse(args[1:]); err != nil {
		return nil, exitFatalError
	}

	if excludeFile != "" {
		exclude := make(map[string]bool)
		fh, err := os.Open(excludeFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not read exclude file: %s\n", err)
			return nil, exitFatalError
		}
		scanner := bufio.NewScanner(fh)
		for scanner.Scan() {
			name := scanner.Text()
			exclude[name] = true

			if checker.Verbose {
				fmt.Printf("Excluding %s\n", name)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Could not read exclude file: %s\n", err)
			return nil, exitFatalError
		}
		checker.SetExclude(exclude)
	}

	checker.Tags = tags
	for _, pkg := range strings.Split(*ignorePkg, ",") {
		if pkg != "" {
			ignore[pkg] = dotStar
		}
	}
	checker.Ignore = ignore

	// ImportPaths normalizes paths and expands '...'
	return gotool.ImportPaths(flags.Args()), exitCodeOk
}

func main() {
	os.Exit(mainCmd(os.Args))
}
