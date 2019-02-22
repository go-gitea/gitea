package uploader

import (
	"path"
	"strings"
)

// FileLink contains the links for a repo's file
type FileLink struct {
	Self    string
	GitURL  string
	HTMLURL string
}

// FileContent contains information about a repo's file stats and content
type FileContent struct {
	Name        string
	Path        string
	SHA         string
	Size        int64
	URL         string
	HTMLURL     string
	GitURL      string
	DownloadURL string
	Type        string
	Links       []*FileLink
}

type CommitMeta struct {
	URL string
	SHA string
}

// CommitUser contains information of a user in the context of a commit.
type CommitUser struct {
	Name  string
	Email string
	Date  string
}

// FileCommit contains information generated from a Git commit for a repo's file.
type FileCommit struct {
	CommitMeta
	HTMLURL   string
	Author    *CommitUser
	Committer *CommitUser
	Parents   []*CommitMeta
	NodeID    string
	Message   string
	Tree      *CommitMeta
}

// PayloadCommitVerification represents the GPG verification of a commit
type PayloadCommitVerification struct {
	Verified  bool
	Reason    string
	Signature string
	Payload   string
}

// File contains information about a repo's file
type File struct {
	Content      *FileContent
	Commit       *FileCommit
	Verification *PayloadCommitVerification
}

func cleanUploadFileName(name string) string {
	// Rebase the filename
	name = strings.Trim(path.Clean("/"+name), " /")
	// Git disallows any filenames to have a .git directory in them.
	for _, part := range strings.Split(name, "/") {
		if strings.ToLower(part) == ".git" {
			return ""
		}
	}
	return name
}
