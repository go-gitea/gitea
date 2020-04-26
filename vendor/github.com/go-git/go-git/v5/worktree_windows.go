// +build windows

package git

import (
"code.gitea.io/gitea/traceinit"


	"os"
	"syscall"
	"time"

	"github.com/go-git/go-git/v5/plumbing/format/index"
)

func init () {
traceinit.Trace("vendor/github.com/go-git/go-git/v5/worktree_windows.go")

	fillSystemInfo = func(e *index.Entry, sys interface{}) {
		if os, ok := sys.(*syscall.Win32FileAttributeData); ok {
			seconds := os.CreationTime.Nanoseconds() / 1000000000
			nanoseconds := os.CreationTime.Nanoseconds() - seconds*1000000000
			e.CreatedAt = time.Unix(seconds, nanoseconds)
		}
	}
}

func isSymlinkWindowsNonAdmin(err error) bool {
	const ERROR_PRIVILEGE_NOT_HELD syscall.Errno = 1314

	if err != nil {
		if errLink, ok := err.(*os.LinkError); ok {
			if errNo, ok := errLink.Err.(syscall.Errno); ok {
				return errNo == ERROR_PRIVILEGE_NOT_HELD
			}
		}
	}

	return false
}
