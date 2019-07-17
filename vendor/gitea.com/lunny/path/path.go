package path

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// IsDirExist returns true if the path is a directory and exist
func IsDirExist(dirName string) (bool, error) {
	f, err := os.Stat(dirName)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !f.IsDir() {
		return false, errors.New("the same name file exist")
	}
	return true, nil
}

// IsDir returns true if given path is a directory,
// or returns false when it's a file or does not exist.
func IsDir(dir string) bool {
	f, e := os.Stat(dir)
	if e != nil {
		return false
	}
	return f.IsDir()
}

// ExePath returns the executable path.
func ExePath() (string, error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	return filepath.Abs(file)
}

// IsFileExist returns ture if it's a file and exist
func IsFileExist(fileName string) (bool, error) {
	f, err := os.Stat(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if f.IsDir() {
		return false, errors.New("the same name directory exist")
	}
	return true, nil
}

// IsExist checks whether a file or directory exists.
// It returns false when the file or directory does not exist.
func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func statDir(dirPath, recPath string, includeDir, isDirOnly bool) ([]string, error) {
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
		if strings.Contains(fi.Name(), ".DS_Store") {
			continue
		}

		relPath := path.Join(recPath, fi.Name())
		curPath := path.Join(dirPath, fi.Name())
		if fi.IsDir() {
			if includeDir {
				statList = append(statList, relPath+"/")
			}
			s, err := statDir(curPath, relPath, includeDir, isDirOnly)
			if err != nil {
				return nil, err
			}
			statList = append(statList, s...)
		} else if !isDirOnly {
			statList = append(statList, relPath)
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
	if !IsDir(rootPath) {
		return nil, errors.New("not a directory or does not exist: " + rootPath)
	}

	isIncludeDir := false
	if len(includeDir) >= 1 {
		isIncludeDir = includeDir[0]
	}
	return statDir(rootPath, "", isIncludeDir, false)
}

// CopyDir copy files recursively from source to target directory.
//
// The filter accepts a function that process the path info.
// and should return true for need to filter.
//
// It returns error when error occurs in underlying functions.
func CopyDir(srcPath, destPath string, filters ...func(filePath string) bool) error {
	// Check if target directory exists.
	if IsExist(destPath) {
		return errors.New("file or directory alreay exists: " + destPath)
	}

	err := os.MkdirAll(destPath, os.ModePerm)
	if err != nil {
		return err
	}

	// Gather directory info.
	infos, err := StatDir(srcPath, true)
	if err != nil {
		return err
	}

	var filter func(filePath string) bool
	if len(filters) > 0 {
		filter = filters[0]
	}

	for _, info := range infos {
		if filter != nil && filter(info) {
			continue
		}

		curPath := path.Join(destPath, info)
		if strings.HasSuffix(info, "/") {
			err = os.MkdirAll(curPath, os.ModePerm)
		} else {
			err = Copy(path.Join(srcPath, info), curPath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Copy copies file from source to target path.
func Copy(src, dest string) error {
	// Gather file information to set back later.
	si, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Handle symbolic link.
	if si.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		// NOTE: os.Chmod and os.Chtimes don't recoganize symbolic link,
		// which will lead "no such file or directory" error.
		return os.Symlink(target, dest)
	}

	sr, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sr.Close()

	dw, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer dw.Close()

	if _, err = io.Copy(dw, sr); err != nil {
		return err
	}

	// Set back file information.
	if err = os.Chtimes(dest, si.ModTime(), si.ModTime()); err != nil {
		return err
	}
	return os.Chmod(dest, si.Mode())
}
