// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"strings"
	"unicode/utf8"

	git_model "gitea.dev/models/git"
	"gitea.dev/modules/analyze"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/attribute"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/typesniffer"
	"gitea.dev/modules/util"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

// DiffStreamFile is a file of a streamed diff, its hunks are decoded to UTF-8 and parsed by the client
type DiffStreamFile struct {
	Name        string               `json:"name"`
	OldName     string               `json:"oldName"`
	NameHash    string               `json:"nameHash"`
	Type        string               `json:"type"` // like the statuses of the diff tree: added, modified, deleted, renamed, copied
	OldMode     string               `json:"oldMode,omitempty"`
	Mode        string               `json:"mode,omitempty"`
	IsBinary    bool                 `json:"isBinary,omitempty"`
	IsVendored  bool                 `json:"isVendored,omitempty"`
	IsGenerated bool                 `json:"isGenerated,omitempty"`
	IsProtected bool                 `json:"isProtected,omitempty"`
	Language    string               `json:"language,omitempty"`
	TabSize     int                  `json:"tabSize"`
	Submodule   *DiffStreamSubmodule `json:"submodule,omitempty"`
	LFS         *DiffStreamLFS       `json:"lfs,omitempty"`
	OldSize     int64                `json:"oldSize,omitempty"` // only for binary and rendered (image, CSV) files
	NewSize     int64                `json:"newSize,omitempty"`
	OldImage    string               `json:"oldImage,omitempty"` // the mime type if it's an image
	NewImage    string               `json:"newImage,omitempty"`
	TooLarge    bool                 `json:"tooLarge,omitempty"`
	Hunks       string               `json:"hunks,omitempty"`

	hunksSize            int
	oldBlobID, newBlobID string
}

type DiffStreamSubmodule struct {
	RepoLink    string `json:"repoLink,omitempty"`
	OldCommit   string `json:"oldCommit,omitempty"`
	NewCommit   string `json:"newCommit,omitempty"`
	OldLink     string `json:"oldLink,omitempty"`
	NewLink     string `json:"newLink,omitempty"`
	CompareLink string `json:"compareLink,omitempty"`
}

type DiffStreamLFSObject struct {
	Size   int64 `json:"size"`
	Exists bool  `json:"exists"`
}

// DiffStreamLFS describes the LFS pointers of both sides, a side is nil if the file doesn't exist there
type DiffStreamLFS struct {
	Old *DiffStreamLFSObject `json:"old,omitempty"`
	New *DiffStreamLFSObject `json:"new,omitempty"`
}

type DiffStreamOptions struct {
	DiffCommonOptions
	Files       []string
	MaxSize     int64 // stops streaming before exceeding it, -1 means no limit
	MaxFileSize int64 // omits the hunks of larger files, -1 means no limit
	RepoID      int64 // for LFS lookups, 0 to skip them
	RepoLink    string
}

// StreamDiff runs "git diff" and calls onFile for every file, it returns whether the diff was truncated by MaxSize
func StreamDiff(ctx context.Context, gitRepo *git.Repository, opts *DiffStreamOptions, onFile func(*DiffStreamFile) error) (truncated bool, _ error) {
	beforeCommit, beforeCommitID, afterCommit, err := prepareDiffCommits(ctx, gitRepo, &opts.DiffCommonOptions)
	if err != nil {
		return false, err
	}

	checker, err := attribute.NewBatchChecker(ctx, gitRepo, opts.AfterCommitID, []string{attribute.LinguistVendored, attribute.LinguistGenerated, attribute.LinguistLanguage, attribute.GitlabLanguage})
	if err != nil {
		return false, err
	}
	defer checker.Close()

	cmd := gitcmd.NewCommand().
		AddArguments("diff", "--full-index", "--src-prefix=\\a/", "--dst-prefix=\\b/").
		AddArguments(opts.WhitespaceBehavior...).
		AddOptionFormat("--find-renames=%s", setting.Git.DiffRenameSimilarityThreshold).
		AddDynamicArguments(beforeCommitID.String(), opts.AfterCommitID).
		AddDashesAndList(opts.Files...)

	cmdCtx, cmdCancel := context.WithCancel(ctx)
	defer cmdCancel()
	reader, readerClose := cmd.MakeStdoutPipe()
	defer readerClose()
	gitErr := make(chan error, 1)
	go func() {
		gitErr <- cmd.WithRepo(gitRepo).RunWithStderr(cmdCtx)
	}()

	emit := func(file *DiffStreamFile, hunks *bytes.Buffer) error {
		if !file.TooLarge {
			file.Hunks = decodeHunksToUTF8(hunks.Bytes())
			file.LFS = detectLFSPointers(ctx, opts.RepoID, file.Hunks)
		}
		if attrs, err := checker.CheckPath(file.Name); err == nil {
			// the attributes are preferred, detecting by name is slower
			if vendored := attrs.GetVendored(); vendored.Has() {
				file.IsVendored = vendored.Value()
			} else {
				file.IsVendored = analyze.IsVendor(file.Name)
			}
			if generated := attrs.GetGenerated(); generated.Has() {
				file.IsGenerated = generated.Value()
			} else {
				file.IsGenerated = analyze.IsGenerated(file.Name)
			}
			file.Language = attrs.GetLanguage().Value()
		}
		if file.Submodule != nil {
			fillSubmoduleLinks(ctx, opts.RepoLink, gitRepo, file, beforeCommit, afterCommit)
		}
		imageMime := mime.TypeByExtension(path.Ext(file.Name))
		if !strings.HasPrefix(imageMime, "image/") || imageMime == "image/svg+xml" && !setting.UI.SVG.Enabled {
			imageMime = ""
		}
		switch {
		case file.LFS != nil: // the objects aren't read, their extension tells whether they are images
			file.OldImage = util.Iif(file.LFS.Old != nil && file.LFS.Old.Exists, imageMime, "")
			file.NewImage = util.Iif(file.LFS.New != nil && file.LFS.New.Exists, imageMime, "")
		case file.IsBinary || isRenderedDiffFile(file.Name):
			// reading a blob streams it completely, so only text files named like images (SVG) are sniffed
			sniff := imageMime != "" && !file.IsBinary
			file.OldSize, file.OldImage = blobInfo(ctx, gitRepo, file.oldBlobID, imageMime, sniff)
			file.NewSize, file.NewImage = blobInfo(ctx, gitRepo, file.newBlobID, imageMime, sniff)
		}
		return onFile(file)
	}

	truncated, err = splitDiffStream(bufio.NewReaderSize(reader, defaultDiffLineBufferSize), opts.MaxFileSize, opts.MaxSize, emit)
	cmdCancel()
	// a failing git command looks like the end of the diff to the parser
	if runErr := <-gitErr; err == nil && runErr != nil && !gitcmd.IsErrorCanceledOrKilled(runErr) {
		err = fmt.Errorf("StreamDiff: git diff in %s: %w", gitRepo.LogString(), runErr)
	}
	return truncated, err
}

// splitDiffStream parses the extended headers of each file and collects its hunks, the hunks of files beyond maxFileSize are only counted.
// It stops before the collected hunks exceed maxSize and returns whether it did.
func splitDiffStream(input *bufio.Reader, maxFileSize, maxSize int64, emit func(*DiffStreamFile, *bytes.Buffer) error) (truncated bool, _ error) {
	if maxSize >= 0 && (maxFileSize < 0 || maxFileSize > maxSize) {
		maxFileSize = maxSize // a larger file can't be sent anyway
	}
	var file *DiffStreamFile
	var hunks bytes.Buffer
	var line []byte // the current line, hunk lines are only kept for submodules
	var totalSize int64
	inHunks, isHunkLine, lineStart := false, false, true
	flush := func() error {
		if file == nil {
			return nil
		}
		if !file.TooLarge {
			if maxSize >= 0 && totalSize+int64(file.hunksSize) > maxSize {
				truncated = true
				return nil
			}
			totalSize += int64(file.hunksSize)
		}
		return emit(file, &hunks)
	}
	for {
		// lines are read in chunks, so the long lines of too large files aren't held in memory
		chunk, err := input.ReadSlice('\n')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) && !errors.Is(err, io.EOF) {
			return truncated, err
		}
		if lineStart {
			line = line[:0]
			isHunkLine = file != nil && !bytes.HasPrefix(chunk, []byte(cmdDiffHead)) && (inHunks || bytes.HasPrefix(chunk, []byte("@@ ")))
			inHunks = inHunks || isHunkLine
		}
		if isHunkLine {
			file.hunksSize += len(chunk)
			if !file.TooLarge && maxFileSize >= 0 && int64(file.hunksSize) > maxFileSize {
				file.TooLarge = true
				hunks.Reset()
			}
			if !file.TooLarge {
				hunks.Write(chunk)
			}
		}
		if !isHunkLine || file.Submodule != nil {
			line = append(line, chunk...)
		}
		lineStart = !errors.Is(err, bufio.ErrBufferFull)
		if lineStart && len(line) > 0 {
			text := strings.TrimSuffix(string(line), "\n")
			switch {
			case isHunkLine:
				parseSubmoduleCommitLine(file, text)
			case strings.HasPrefix(text, cmdDiffHead):
				if err := flush(); err != nil || truncated {
					return truncated, err
				}
				file, inHunks = newDiffStreamFile(text), false
				hunks.Reset()
			case file == nil:
				return false, fmt.Errorf("splitDiffStream: invalid first line: %s", text)
			default:
				parseDiffStreamHeader(file, text)
			}
		}
		if errors.Is(err, io.EOF) {
			err = flush() // not in the return statement, which could read "truncated" before the call
			return truncated, err
		}
	}
}

// the "\a/" and "\b/" prefixes force git to quote all names in the head line, so they are never ambiguous
func newDiffStreamFile(headLine string) *DiffStreamFile {
	parsed := createDiffFile(headLine + "\n")
	return &DiffStreamFile{Name: parsed.Name, OldName: parsed.OldName, NameHash: git.HashFilePathForWebUI(parsed.Name), Type: "modified"}
}

func parseDiffStreamHeader(file *DiffStreamFile, line string) {
	var submoduleMode string // a type change between a submodule and a file shows the file's content instead
	if mode, ok := strings.CutPrefix(line, "old mode "); ok {
		file.OldMode = strings.TrimSpace(mode)
	} else if mode, ok := strings.CutPrefix(line, "new mode "); ok {
		file.Mode = strings.TrimSpace(mode)
	} else if mode, ok := strings.CutPrefix(line, "new file mode "); ok {
		file.Type, file.Mode = "added", strings.TrimSpace(mode)
		submoduleMode = file.Mode
	} else if mode, ok := strings.CutPrefix(line, "deleted file mode "); ok {
		file.Type, file.OldMode = "deleted", strings.TrimSpace(mode)
		submoduleMode = file.OldMode
	} else if strings.HasPrefix(line, "rename from ") {
		file.Type = "renamed"
	} else if strings.HasPrefix(line, "copy from ") {
		file.Type = "copied"
	} else if strings.HasPrefix(line, "Binary files ") {
		file.IsBinary = true
	} else if index, ok := strings.CutPrefix(line, "index "); ok { // "index <old>..<new> [<mode>]"
		var blobIDs string
		blobIDs, submoduleMode, _ = strings.Cut(index, " ")
		file.oldBlobID, file.newBlobID, _ = strings.Cut(blobIDs, "..")
	}
	if submoduleMode == "160000" {
		file.Submodule = &DiffStreamSubmodule{}
	}
}

// isRenderedDiffFile reports whether the diff viewer renders the file besides its source, for text formats like SVG and CSV
func isRenderedDiffFile(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".svg", ".csv", ".tsv":
		return true
	}
	return false
}

// blobInfo returns the size of a blob and its image mime type, which is confirmed by sniffing the content if requested
func blobInfo(ctx context.Context, gitRepo *git.Repository, blobID, imageMime string, sniff bool) (size int64, _ string) {
	blob, err := gitRepo.GetBlob(blobID)
	if err != nil {
		return 0, "" // the file doesn't exist on this side
	}
	if !sniff {
		return blob.Size(ctx), imageMime
	}
	data, err := blob.GetBlobBytes(ctx, typesniffer.SniffContentSize)
	if err != nil {
		log.Error("blobInfo: GetBlobBytes: %v", err)
		return blob.Size(ctx), ""
	}
	sniffed := typesniffer.DetectContentType(data)
	return blob.Size(ctx), util.Iif(sniffed.IsImage(), sniffed.GetMimeType(), "")
}

func parseSubmoduleCommitLine(file *DiffStreamFile, line string) {
	if after, ok := strings.CutPrefix(line, "-Subproject commit "); ok {
		file.Submodule.OldCommit = strings.TrimSuffix(after, "-dirty")
	} else if after, ok := strings.CutPrefix(line, "+Subproject commit "); ok {
		file.Submodule.NewCommit = strings.TrimSuffix(after, "-dirty")
	}
}

func fillSubmoduleLinks(ctx context.Context, repoLink string, gitRepo *git.Repository, file *DiffStreamFile, beforeCommit, afterCommit *git.Commit) {
	commit := util.Iif(file.Type == "deleted", beforeCommit, afterCommit)
	if commit == nil {
		return
	}
	sub, err := git.GetCommitInfoSubmoduleFile(ctx, repoLink, file.Name, gitRepo, commit, commit.ID)
	if err != nil {
		log.Error("Unable to get the submodule %q: %v", file.Name, err) // without links instead of a failed diff
		return
	}
	if link := sub.SubmoduleWebLinkTree(ctx); link != nil {
		file.Submodule.RepoLink = link.RepoWebLink
	}
	if file.Submodule.OldCommit != "" {
		if link := sub.SubmoduleWebLinkTree(ctx, file.Submodule.OldCommit); link != nil {
			file.Submodule.OldLink = link.CommitWebLink
		}
	}
	if file.Submodule.NewCommit != "" {
		if link := sub.SubmoduleWebLinkTree(ctx, file.Submodule.NewCommit); link != nil {
			file.Submodule.NewLink = link.CommitWebLink
		}
	}
	if file.Submodule.OldCommit != "" && file.Submodule.NewCommit != "" {
		if link := sub.SubmoduleWebLinkCompare(ctx, file.Submodule.OldCommit, file.Submodule.NewCommit); link != nil {
			file.Submodule.CompareLink = link.CommitWebLink
		}
	}
}

// decodeHunksToUTF8 converts each line type separately, because the encodings of both sides may differ
func decodeHunksToUTF8(hunks []byte) string {
	if utf8.Valid(hunks) {
		return string(hunks)
	}
	lines := bytes.SplitAfter(hunks, []byte("\n"))
	byType := map[byte]*bytes.Buffer{' ': {}, '+': {}, '-': {}}
	for _, line := range lines {
		if len(line) > 0 && byType[line[0]] != nil {
			byType[line[0]].Write(line[1:])
		}
	}
	decoders := map[byte]*encoding.Decoder{}
	for lineType, buf := range byType {
		if buf.Len() != 0 && !utf8.Valid(buf.Bytes()) {
			decoders[lineType] = detectDecoder(buf.Bytes())
		}
	}
	var out strings.Builder
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if decoder := decoders[line[0]]; decoder != nil {
			if decoded, _, err := transform.Bytes(decoder, line[1:]); err == nil {
				out.WriteByte(line[0])
				out.Write(decoded)
				continue
			}
		}
		out.WriteString(strings.ToValidUTF8(string(line), "\uFFFD"))
	}
	return out.String()
}

// detectLFSPointers returns the LFS pointers of both sides if the hunks are a pointer file diff with a stored object,
// pointers without them are shown as text
func detectLFSPointers(ctx context.Context, repoID int64, hunks string) *DiffStreamLFS {
	if len(hunks) > 1024 || !strings.Contains(hunks, lfs.MetaFileIdentifier) {
		return nil
	}
	var oldContent, newContent strings.Builder
	for line := range strings.Lines(hunks) {
		switch line[0] {
		case ' ':
			oldContent.WriteString(line[1:])
			newContent.WriteString(line[1:])
		case '-':
			oldContent.WriteString(line[1:])
		case '+':
			newContent.WriteString(line[1:])
		}
	}
	lookup := func(content string) *DiffStreamLFSObject {
		pointer, err := lfs.ReadPointerFromBuffer([]byte(content))
		if err != nil || !pointer.IsValid() {
			return nil
		}
		obj := &DiffStreamLFSObject{Size: pointer.Size}
		if repoID != 0 {
			_, err := git_model.GetLFSMetaObjectByOid(ctx, repoID, pointer.Oid)
			obj.Exists = err == nil
		}
		return obj
	}
	result := &DiffStreamLFS{Old: lookup(oldContent.String()), New: lookup(newContent.String())}
	if result.Old != nil && result.Old.Exists || result.New != nil && result.New.Exists {
		return result
	}
	return nil
}
