// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// BlamePart represents block of blame - continuous lines with one sha
type BlamePart struct {
	Sha          string
	Lines        []string
	PreviousSha  string
	PreviousPath string
}

// BlameReader returns part of file blame one by one
type BlameReader struct {
	output         io.WriteCloser
	reader         io.ReadCloser
	bufferedReader *bufio.Reader
	done           chan error
	lastSha        *string
	ignoreRevsFile string
	objectFormat   ObjectFormat
	cleanupFuncs   []func()
}

func (r *BlameReader) UsesIgnoreRevs() bool {
	return r.ignoreRevsFile != ""
}

// NextPart returns next part of blame (sequential code lines with the same commit)
func (r *BlameReader) NextPart() (*BlamePart, error) {
	var blamePart *BlamePart

	if r.lastSha != nil {
		blamePart = &BlamePart{
			Sha:   *r.lastSha,
			Lines: make([]string, 0),
		}
	}

	const previousHeader = "previous "
	var lineBytes []byte
	var isPrefix bool
	var err error

	for err != io.EOF {
		lineBytes, isPrefix, err = r.bufferedReader.ReadLine()
		if err != nil && err != io.EOF {
			return blamePart, err
		}

		if len(lineBytes) == 0 {
			// isPrefix will be false
			continue
		}

		var objectID string
		objectFormatLength := r.objectFormat.FullLength()

		if len(lineBytes) > objectFormatLength && lineBytes[objectFormatLength] == ' ' && r.objectFormat.IsValid(string(lineBytes[0:objectFormatLength])) {
			objectID = string(lineBytes[0:objectFormatLength])
		}
		if len(objectID) > 0 {
			if blamePart == nil {
				blamePart = &BlamePart{
					Sha:   objectID,
					Lines: make([]string, 0),
				}
			}

			if blamePart.Sha != objectID {
				r.lastSha = &objectID
				// need to munch to end of line...
				for isPrefix {
					_, isPrefix, err = r.bufferedReader.ReadLine()
					if err != nil && err != io.EOF {
						return blamePart, err
					}
				}
				return blamePart, nil
			}
		} else if lineBytes[0] == '\t' {
			blamePart.Lines = append(blamePart.Lines, string(lineBytes[1:]))
		} else if bytes.HasPrefix(lineBytes, []byte(previousHeader)) {
			offset := len(previousHeader) // already includes a space
			blamePart.PreviousSha = string(lineBytes[offset : offset+objectFormatLength])
			offset += objectFormatLength + 1 // +1 for space
			blamePart.PreviousPath = string(lineBytes[offset:])
		}

		// need to munch to end of line...
		for isPrefix {
			_, isPrefix, err = r.bufferedReader.ReadLine()
			if err != nil && err != io.EOF {
				return blamePart, err
			}
		}
	}

	r.lastSha = nil

	return blamePart, nil
}

// Close BlameReader - don't run NextPart after invoking that
func (r *BlameReader) Close() error {
	if r.bufferedReader == nil {
		return nil
	}

	err := <-r.done
	r.bufferedReader = nil
	_ = r.reader.Close()
	_ = r.output.Close()
	for _, cleanup := range r.cleanupFuncs {
		if cleanup != nil {
			cleanup()
		}
	}
	return err
}

// CreateBlameReader creates reader for given repository, commit and file
func CreateBlameReader(ctx context.Context, objectFormat ObjectFormat, repoPath string, commit *Commit, file string, bypassBlameIgnore bool) (rd *BlameReader, err error) {
	var ignoreRevsFileName string
	var ignoreRevsFileCleanup func()
	defer func() {
		if err != nil && ignoreRevsFileCleanup != nil {
			ignoreRevsFileCleanup()
		}
	}()

	cmd := NewCommand("blame", "--porcelain")

	if DefaultFeatures().CheckVersionAtLeast("2.23") && !bypassBlameIgnore {
		ignoreRevsFileName, ignoreRevsFileCleanup, err = tryCreateBlameIgnoreRevsFile(commit)
		if err != nil && !IsErrNotExist(err) {
			return nil, err
		}
		if ignoreRevsFileName != "" {
			// Possible improvement: use --ignore-revs-file /dev/stdin on unix
			// There is no equivalent on Windows. May be implemented if Gitea uses an external git backend.
			cmd.AddOptionValues("--ignore-revs-file", ignoreRevsFileName)
		}
	}

	cmd.AddDynamicArguments(commit.ID.String()).AddDashesAndList(file)

	done := make(chan error, 1)
	reader, stdout, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	go func() {
		stderr := bytes.Buffer{}
		// TODO: it doesn't work for directories (the directories shouldn't be "blamed"), and the "err" should be returned by "Read" but not by "Close"
		err := cmd.Run(ctx, &RunOpts{
			UseContextTimeout: true,
			Dir:               repoPath,
			Stdout:            stdout,
			Stderr:            &stderr,
		})
		done <- err
		_ = stdout.Close()
		if err != nil {
			log.Error("Error running git blame (dir: %v): %v, stderr: %v", repoPath, err, stderr.String())
		}
	}()

	bufferedReader := bufio.NewReader(reader)
	return &BlameReader{
		output:         stdout,
		reader:         reader,
		bufferedReader: bufferedReader,
		done:           done,
		ignoreRevsFile: ignoreRevsFileName,
		objectFormat:   objectFormat,
		cleanupFuncs:   []func(){ignoreRevsFileCleanup},
	}, nil
}

func tryCreateBlameIgnoreRevsFile(commit *Commit) (string, func(), error) {
	entry, err := commit.GetTreeEntryByPath(".git-blame-ignore-revs")
	if err != nil {
		return "", nil, err
	}

	r, err := entry.Blob().DataAsync()
	if err != nil {
		return "", nil, err
	}
	defer r.Close()

	f, cleanup, err := setting.AppDataTempDir("git-repo-content").CreateTempFileRandom("git-blame-ignore-revs")
	if err != nil {
		return "", nil, err
	}
	filename := f.Name()
	_, err = io.Copy(f, r)
	_ = f.Close()
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return filename, cleanup, nil
}
