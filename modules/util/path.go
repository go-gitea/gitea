// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// PathJoinRel joins the path elements into a single path, each element is cleaned by path.Clean separately.
// It only returns the following values (like path.Join), any redundant part (empty, relative dots, slashes) is removed.
// It's caller's duty to make every element not bypass its own directly level, to avoid security issues.
//
//	empty => ``
//	`` => ``
//	`..` => `.`
//	`dir` => `dir`
//	`/dir/` => `dir`
//	`foo\..\bar` => `foo\..\bar`
//	{`foo`, ``, `bar`} => `foo/bar`
//	{`foo`, `..`, `bar`} => `foo/bar`
func PathJoinRel(elem ...string) string {
	elems := make([]string, len(elem))
	for i, e := range elem {
		if e == "" {
			continue
		}
		elems[i] = path.Clean("/" + e)
	}
	p := path.Join(elems...)
	if p == "" {
		return ""
	} else if p == "/" {
		return "."
	} else {
		return p[1:]
	}
}

// PathJoinRelX joins the path elements into a single path like PathJoinRel,
// and covert all backslashes to slashes. (X means "extended", also means the combination of `\` and `/`).
// It's caller's duty to make every element not bypass its own directly level, to avoid security issues.
// It returns similar results as PathJoinRel except:
//
//	`foo\..\bar` => `bar`  (because it's processed as `foo/../bar`)
//
// All backslashes are handled as slashes, the result only contains slashes.
func PathJoinRelX(elem ...string) string {
	elems := make([]string, len(elem))
	for i, e := range elem {
		if e == "" {
			continue
		}
		elems[i] = path.Clean("/" + strings.ReplaceAll(e, "\\", "/"))
	}
	return PathJoinRel(elems...)
}

const pathSeparator = string(os.PathSeparator)

// FilePathJoinAbs joins the path elements into a single file path, each element is cleaned by filepath.Clean separately.
// All slashes/backslashes are converted to path separators before cleaning, the result only contains path separators.
// The first element must be an absolute path, caller should prepare the base path.
// It's caller's duty to make every element not bypass its own directly level, to avoid security issues.
// Like PathJoinRel, any redundant part (empty, relative dots, slashes) is removed.
//
//	{`/foo`, ``, `bar`} => `/foo/bar`
//	{`/foo`, `..`, `bar`} => `/foo/bar`
func FilePathJoinAbs(base string, sub ...string) string {
	elems := make([]string, 1, len(sub)+1)

	// POSIX filesystem can have `\` in file names. Windows: `\` and `/` are both used for path separators
	// to keep the behavior consistent, we do not allow `\` in file names, replace all `\` with `/`
	if isOSWindows() {
		elems[0] = filepath.Clean(base)
	} else {
		elems[0] = filepath.Clean(strings.ReplaceAll(base, "\\", pathSeparator))
	}
	if !filepath.IsAbs(elems[0]) {
		// This shouldn't happen. If there is really necessary to pass in relative path, return the full path with filepath.Abs() instead
		panic(fmt.Sprintf("FilePathJoinAbs: %q (for path %v) is not absolute, do not guess a relative path based on current working directory", elems[0], elems))
	}
	for _, s := range sub {
		if s == "" {
			continue
		}
		if isOSWindows() {
			elems = append(elems, filepath.Clean(pathSeparator+s))
		} else {
			elems = append(elems, filepath.Clean(pathSeparator+strings.ReplaceAll(s, "\\", pathSeparator)))
		}
	}
	// the elems[0] must be an absolute path, just join them together
	return filepath.Join(elems...)
}

// IsDir returns true if given path is a directory,
// or returns false when it's a file or does not exist.
func IsDir(dir string) (bool, error) {
	f, err := os.Stat(dir)
	if err == nil {
		return f.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsFile returns true if given path is a file,
// or returns false when it's a directory or does not exist.
func IsFile(filePath string) (bool, error) {
	f, err := os.Stat(filePath)
	if err == nil {
		return !f.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsExist checks whether a file or directory exists.
// It returns false when the file or directory does not exist.
func IsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil || os.IsExist(err) {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func statDir(dirPath, recPath string, includeDir, isDirOnly, followSymlinks bool) ([]string, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	fis, err := dir.Readdir(0)
	if err != nil {
		return nil, err
	}

	statList := make([]string, 0)
	for _, fi := range fis {
		if CommonSkip(fi.Name()) {
			continue
		}

		relPath := path.Join(recPath, fi.Name())
		curPath := path.Join(dirPath, fi.Name())
		if fi.IsDir() {
			if includeDir {
				statList = append(statList, relPath+"/")
			}
			s, err := statDir(curPath, relPath, includeDir, isDirOnly, followSymlinks)
			if err != nil {
				return nil, err
			}
			statList = append(statList, s...)
		} else if !isDirOnly {
			statList = append(statList, relPath)
		} else if followSymlinks && fi.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(curPath)
			if err != nil {
				return nil, err
			}

			isDir, err := IsDir(link)
			if err != nil {
				return nil, err
			}
			if isDir {
				if includeDir {
					statList = append(statList, relPath+"/")
				}
				s, err := statDir(curPath, relPath, includeDir, isDirOnly, followSymlinks)
				if err != nil {
					return nil, err
				}
				statList = append(statList, s...)
			}
		}
	}
	return statList, nil
}

// StatDir gathers information of given directory by depth-first.
// It returns slice of file list and includes subdirectories if enabled;
// it returns error and nil slice when error occurs in underlying functions,
// or given path is not a directory or does not exist.
//
// Slice does not include given path itself.
// If subdirectories is enabled, they will have suffix '/'.
func StatDir(rootPath string, includeDir ...bool) ([]string, error) {
	if isDir, err := IsDir(rootPath); err != nil {
		return nil, err
	} else if !isDir {
		return nil, errors.New("not a directory or does not exist: " + rootPath)
	}

	isIncludeDir := false
	if len(includeDir) != 0 {
		isIncludeDir = includeDir[0]
	}
	return statDir(rootPath, "", isIncludeDir, false, false)
}

func isOSWindows() bool {
	return runtime.GOOS == "windows"
}

var driveLetterRegexp = regexp.MustCompile("/[A-Za-z]:/")

// FileURLToPath extracts the path information from a file://... url.
func FileURLToPath(u *url.URL) (string, error) {
	if u.Scheme != "file" {
		return "", errors.New("URL scheme is not 'file': " + u.String())
	}

	path := u.Path

	if !isOSWindows() {
		return path, nil
	}

	// If it looks like there's a Windows drive letter at the beginning, strip off the leading slash.
	if driveLetterRegexp.MatchString(path) {
		return path[1:], nil
	}
	return path, nil
}

// HomeDir returns path of '~'(in Linux) on Windows,
// it returns error when the variable does not exist.
func HomeDir() (home string, err error) {
	// TODO: some users run Gitea with mismatched uid  and "HOME=xxx" (they set HOME=xxx by environment manually)
	// TODO: when running gitea as a sub command inside git, the HOME directory is not the user's home directory
	// so at the moment we can not use `user.Current().HomeDir`
	if isOSWindows() {
		home = os.Getenv("USERPROFILE")
		if home == "" {
			home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		}
	} else {
		home = os.Getenv("HOME")
	}

	if home == "" {
		return "", errors.New("cannot get home directory")
	}

	return home, nil
}

// CommonSkip will check a provided name to see if it represents file or directory that should not be watched
func CommonSkip(name string) bool {
	if name == "" {
		return true
	}

	switch name[0] {
	case '.':
		return true
	case 't', 'T':
		return name[1:] == "humbs.db"
	case 'd', 'D':
		return name[1:] == "esktop.ini"
	}

	return false
}

// IsReadmeFileName reports whether name looks like a README file
// based on its name.
func IsReadmeFileName(name string) bool {
	name = strings.ToLower(name)
	if len(name) < 6 {
		return false
	} else if len(name) == 6 {
		return name == "readme"
	}
	return name[:7] == "readme."
}

// IsReadmeFileExtension reports whether name looks like a README file
// based on its name. It will look through the provided extensions and check if the file matches
// one of the extensions and provide the index in the extension list.
// If the filename is `readme.` with an unmatched extension it will match with the index equaling
// the length of the provided extension list.
// Note that the '.' should be provided in ext, e.g ".md"
func IsReadmeFileExtension(name string, ext ...string) (int, bool) {
	name = strings.ToLower(name)
	if len(name) < 6 || name[:6] != "readme" {
		return 0, false
	}

	for i, extension := range ext {
		extension = strings.ToLower(extension)
		if name[6:] == extension {
			return i, true
		}
	}

	if name[6] == '.' {
		return len(ext), true
	}

	return 0, false
}
