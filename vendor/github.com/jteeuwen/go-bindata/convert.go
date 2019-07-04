// This work is subject to the CC0 1.0 Universal (CC0 1.0) Public Domain Dedication
// license. Its contents can be found at:
// http://creativecommons.org/publicdomain/zero/1.0/

package bindata

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Translate reads assets from an input directory, converts them
// to Go code and writes new files to the output specified
// in the given configuration.
func Translate(c *Config) error {
	var toc []Asset

	// Ensure our configuration has sane values.
	err := c.validate()
	if err != nil {
		return err
	}

	// Locate all the assets.
	for _, input := range c.Input {
		err = findFiles(input.Path, c.Prefix, input.Recursive, &toc, c.Ignore)
		if err != nil {
			return err
		}
	}

	// Sort to make output stable between invocations
	sort.Sort(ByPath(toc))

	// Create output file.
	fd, err := os.Create(c.Output)
	if err != nil {
		return err
	}

	defer fd.Close()

	// Create a buffered writer for better performance.
	bfd := bufio.NewWriter(fd)
	defer bfd.Flush()

	// Write build tags, if applicable.
	if len(c.Tags) > 0 {
		_, err = fmt.Fprintf(bfd, "// +build %s\n\n", c.Tags)
		if err != nil {
			return err
		}
	}

	// Write package declaration.
	_, err = fmt.Fprintf(bfd, "package %s\n\n", c.Package)
	if err != nil {
		return err
	}

	// Write assets.
	if c.Debug {
		err = writeDebug(bfd, toc)
	} else {
		err = writeRelease(bfd, c, toc)
	}

	if err != nil {
		return err
	}

	// Write table of contents
	if err := writeTOC(bfd, toc); err != nil {
		return err
	}
	// Write hierarchical tree of assets
	return writeTOCTree(bfd, toc)
}

// findFiles recursively finds all the file paths in the given directory tree.
// They are added to the given map as keys. Values will be safe function names
// for each file, which will be used when generating the output code.
func findFiles(dir, prefix string, recursive bool, toc *[]Asset, ignore []*regexp.Regexp) error {
	if len(prefix) > 0 {
		dir, _ = filepath.Abs(dir)
		prefix, _ = filepath.Abs(prefix)
		prefix = filepath.ToSlash(prefix)
	}

	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}

	var list []os.FileInfo

	if !fi.IsDir() {
		dir = ""
		list = []os.FileInfo{fi}
	} else {
		fd, err := os.Open(dir)
		if err != nil {
			return err
		}

		defer fd.Close()

		list, err = fd.Readdir(0)
		if err != nil {
			return err
		}
	}

	knownFuncs := make(map[string]int)

	for _, file := range list {
		var asset Asset
		asset.Path = filepath.Join(dir, file.Name())
		asset.Name = filepath.ToSlash(asset.Path)

		ignoring := false
		for _, re := range ignore {
			if re.MatchString(asset.Path) {
				ignoring = true
				break
			}
		}
		if ignoring {
			continue
		}

		if file.IsDir() {
			if recursive {
				findFiles(asset.Path, prefix, recursive, toc, ignore)
			}
			continue
		}

		if strings.HasPrefix(asset.Name, prefix) {
			asset.Name = asset.Name[len(prefix):]
		}

		// If we have a leading slash, get rid of it.
		if len(asset.Name) > 0 && asset.Name[0] == '/' {
			asset.Name = asset.Name[1:]
		}

		// This shouldn't happen.
		if len(asset.Name) == 0 {
			return fmt.Errorf("Invalid file: %v", asset.Path)
		}

		asset.Func = safeFunctionName(asset.Name, knownFuncs)
		asset.Path, _ = filepath.Abs(asset.Path)
		*toc = append(*toc, asset)
	}

	return nil
}

var regFuncName = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// safeFunctionName converts the given name into a name
// which qualifies as a valid function identifier. It
// also compares against a known list of functions to
// prevent conflict based on name translation.
func safeFunctionName(name string, knownFuncs map[string]int) string {
	name = strings.ToLower(name)
	name = regFuncName.ReplaceAllString(name, "_")

	// Get rid of "__" instances for niceness.
	for strings.Index(name, "__") > -1 {
		name = strings.Replace(name, "__", "_", -1)
	}

	// Leading underscores are silly (unless they prefix a digit (see below)).
	for len(name) > 1 && name[0] == '_' {
		name = name[1:]
	}

	// Identifier can't start with a digit.
	if unicode.IsDigit(rune(name[0])) {
		name = "_" + name
	}

	if num, ok := knownFuncs[name]; ok {
		knownFuncs[name] = num + 1
		name = fmt.Sprintf("%s%d", name, num)
	} else {
		knownFuncs[name] = 2
	}

	return name
}
