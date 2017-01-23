/*
Copyright 2015 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package repository contains helper methods for working with the Git repo.
package repository

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const branchRefPrefix = "refs/heads/"

// GitRepo represents an instance of a (local) git repository.
type GitRepo struct {
	Path string
}

// Run the given git command with the given I/O reader/writers, returning an error if it fails.
func (repo *GitRepo) runGitCommandWithIO(stdin io.Reader, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.Command("git", args...)
	log.Print("runGitCommandWithIO", repo, args)
	cmd.Dir = repo.Path
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// Run the given git command and return its stdout, or an error if the command fails.
func (repo *GitRepo) runGitCommandRaw(args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := repo.runGitCommandWithIO(nil, &stdout, &stderr, args...)
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// Run the given git command and return its stdout, or an error if the command fails.
func (repo *GitRepo) runGitCommand(args ...string) (string, error) {
	stdout, stderr, err := repo.runGitCommandRaw(args...)
	if err != nil {
		if stderr == "" {
			stderr = "Error running git command: " + strings.Join(args, " ")
		}
		err = fmt.Errorf(stderr)
	}
	return stdout, err
}

// Run the given git command using the same stdin, stdout, and stderr as the review tool.
func (repo *GitRepo) runGitCommandInline(args ...string) error {
	return repo.runGitCommandWithIO(os.Stdin, os.Stdout, os.Stderr, args...)
}

// NewGitRepo determines if the given working directory is inside of a git repository,
// and returns the corresponding GitRepo instance if it is.
func NewGitRepo(path string) (*GitRepo, error) {
	repo := &GitRepo{Path: path}
	log.Print("NewGitRepo", repo)
	_, _, err := repo.runGitCommandRaw("rev-parse")
	log.Print("NewGitRepo after init", repo)
	log.Print("NewGitRepo after init", err)
	if err == nil {
		log.Print("NewGitRepo return", repo)
		return repo, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return nil, err
	}
	return nil, err
}

// GetPath returns the path to the repo.
func (repo *GitRepo) GetPath() string {
	return repo.Path
}

// GetRepoStateHash returns a hash which embodies the entire current state of a repository.
func (repo *GitRepo) GetRepoStateHash() (string, error) {
	stateSummary, error := repo.runGitCommand("show-ref")
	return fmt.Sprintf("%x", sha1.Sum([]byte(stateSummary))), error
}

// GetUserEmail returns the email address that the user has used to configure git.
func (repo *GitRepo) GetUserEmail() (string, error) {
	return repo.runGitCommand("config", "user.email")
}

// GetCoreEditor returns the name of the editor that the user has used to configure git.
func (repo *GitRepo) GetCoreEditor() (string, error) {
	return repo.runGitCommand("var", "GIT_EDITOR")
}

// GetSubmitStrategy returns the way in which a review is submitted
func (repo *GitRepo) GetSubmitStrategy() (string, error) {
	submitStrategy, _ := repo.runGitCommand("config", "appraise.submit")
	return submitStrategy, nil
}

// HasUncommittedChanges returns true if there are local, uncommitted changes.
func (repo *GitRepo) HasUncommittedChanges() (bool, error) {
	out, err := repo.runGitCommand("status", "--porcelain")
	if err != nil {
		return false, err
	}
	if len(out) > 0 {
		return true, nil
	}
	return false, nil
}

// VerifyCommit verifies that the supplied hash points to a known commit.
func (repo *GitRepo) VerifyCommit(hash string) error {
	out, err := repo.runGitCommand("cat-file", "-t", hash)
	if err != nil {
		return err
	}
	objectType := strings.TrimSpace(string(out))
	if objectType != "commit" {
		return fmt.Errorf("Hash %q points to a non-commit object of type %q", hash, objectType)
	}
	return nil
}

// VerifyGitRef verifies that the supplied ref points to a known commit.
func (repo *GitRepo) VerifyGitRef(ref string) error {
	_, err := repo.runGitCommand("show-ref", "--verify", ref)
	return err
}

// GetHeadRef returns the ref that is the current HEAD.
func (repo *GitRepo) GetHeadRef() (string, error) {
	return repo.runGitCommand("symbolic-ref", "HEAD")
}

// GetCommitHash returns the hash of the commit pointed to by the given ref.
func (repo *GitRepo) GetCommitHash(ref string) (string, error) {
	return repo.runGitCommand("show", "-s", "--format=%H", ref)
}

// ResolveRefCommit returns the commit pointed to by the given ref, which may be a remote ref.
//
// This differs from GetCommitHash which only works on exact matches, in that it will try to
// intelligently handle the scenario of a ref not existing locally, but being known to exist
// in a remote repo.
//
// This method should be used when a command may be performed by either the reviewer or the
// reviewee, while GetCommitHash should be used when the encompassing command should only be
// performed by the reviewee.
func (repo *GitRepo) ResolveRefCommit(ref string) (string, error) {
	if err := repo.VerifyGitRef(ref); err == nil {
		return repo.GetCommitHash(ref)
	}
	if strings.HasPrefix(ref, "refs/heads/") {
		// The ref is a branch. Check if it exists in exactly one remote
		pattern := strings.Replace(ref, "refs/heads", "**", 1)
		matchingOutput, err := repo.runGitCommand("for-each-ref", "--format=%(refname)", pattern)
		if err != nil {
			return "", err
		}
		matchingRefs := strings.Split(matchingOutput, "\n")
		if len(matchingRefs) == 1 && matchingRefs[0] != "" {
			// There is exactly one match
			return repo.GetCommitHash(matchingRefs[0])
		}
		return "", fmt.Errorf("Unable to find a git ref matching the pattern %q", pattern)
	}
	return "", fmt.Errorf("Unknown git ref %q", ref)
}

// GetCommitMessage returns the message stored in the commit pointed to by the given ref.
func (repo *GitRepo) GetCommitMessage(ref string) (string, error) {
	return repo.runGitCommand("show", "-s", "--format=%B", ref)
}

// GetCommitTime returns the commit time of the commit pointed to by the given ref.
func (repo *GitRepo) GetCommitTime(ref string) (string, error) {
	return repo.runGitCommand("show", "-s", "--format=%ct", ref)
}

// GetLastParent returns the last parent of the given commit (as ordered by git).
func (repo *GitRepo) GetLastParent(ref string) (string, error) {
	return repo.runGitCommand("rev-list", "--skip", "1", "-n", "1", ref)
}

// GetCommitDetails returns the details of a commit's metadata.
func (repo GitRepo) GetCommitDetails(ref string) (*CommitDetails, error) {
	var err error
	show := func(formatString string) (result string) {
		if err != nil {
			return ""
		}
		result, err = repo.runGitCommand("show", "-s", ref, fmt.Sprintf("--format=tformat:%s", formatString))
		return result
	}

	jsonFormatString := "{\"tree\":\"%T\", \"time\": \"%at\"}"
	detailsJSON := show(jsonFormatString)
	if err != nil {
		return nil, err
	}
	var details CommitDetails
	err = json.Unmarshal([]byte(detailsJSON), &details)
	if err != nil {
		return nil, err
	}
	details.Author = show("%an")
	details.AuthorEmail = show("%ae")
	details.Summary = show("%s")
	parentsString := show("%P")
	details.Parents = strings.Split(parentsString, " ")
	if err != nil {
		return nil, err
	}
	return &details, nil
}

// MergeBase determines if the first commit that is an ancestor of the two arguments.
func (repo *GitRepo) MergeBase(a, b string) (string, error) {
	return repo.runGitCommand("merge-base", a, b)
}

// IsAncestor determines if the first argument points to a commit that is an ancestor of the second.
func (repo *GitRepo) IsAncestor(ancestor, descendant string) (bool, error) {
	_, _, err := repo.runGitCommandRaw("merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, fmt.Errorf("Error while trying to determine commit ancestry: %v", err)
}

// Diff computes the diff between two given commits.
func (repo *GitRepo) Diff(left, right string, diffArgs ...string) (string, error) {
	args := []string{"diff"}
	args = append(args, diffArgs...)
	args = append(args, fmt.Sprintf("%s..%s", left, right))
	return repo.runGitCommand(args...)
}

// Show returns the contents of the given file at the given commit.
func (repo *GitRepo) Show(commit, path string) (string, error) {
	return repo.runGitCommand("show", fmt.Sprintf("%s:%s", commit, path))
}

// SwitchToRef changes the currently-checked-out ref.
func (repo *GitRepo) SwitchToRef(ref string) error {
	// If the ref starts with "refs/heads/", then we have to trim that prefix,
	// or else we will wind up in a detached HEAD state.
	if strings.HasPrefix(ref, branchRefPrefix) {
		ref = ref[len(branchRefPrefix):]
	}
	_, err := repo.runGitCommand("checkout", ref)
	return err
}

// mergeArchives merges two archive refs.
func (repo *GitRepo) mergeArchives(archive, remoteArchive string) error {
	remoteHash, err := repo.GetCommitHash(remoteArchive)
	if err != nil {
		return err
	}
	if remoteHash == "" {
		// The remote archive does not exist, so we have nothing to do
		return nil
	}

	archiveHash, err := repo.GetCommitHash(archive)
	if err != nil {
		return err
	}
	if archiveHash == "" {
		// The local archive does not exist, so we merely need to set it
		_, err := repo.runGitCommand("update-ref", archive, remoteHash)
		return err
	}

	isAncestor, err := repo.IsAncestor(archiveHash, remoteHash)
	if err != nil {
		return err
	}
	if isAncestor {
		// The archive can simply be fast-forwarded
		_, err := repo.runGitCommand("update-ref", archive, remoteHash, archiveHash)
		return err
	}

	// Create a merge commit of the two archives
	refDetails, err := repo.GetCommitDetails(remoteArchive)
	if err != nil {
		return err
	}
	newArchiveHash, err := repo.runGitCommand("commit-tree", "-p", remoteHash, "-p", archiveHash, "-m", "Merge local and remote archives", refDetails.Tree)
	if err != nil {
		return err
	}
	newArchiveHash = strings.TrimSpace(newArchiveHash)
	_, err = repo.runGitCommand("update-ref", archive, newArchiveHash, archiveHash)
	return err
}

// ArchiveRef adds the current commit pointed to by the 'ref' argument
// under the ref specified in the 'archive' argument.
//
// Both the 'ref' and 'archive' arguments are expected to be the fully
// qualified names of git refs (e.g. 'refs/heads/my-change' or
// 'refs/devtools/archives/reviews').
//
// If the ref pointed to by the 'archive' argument does not exist
// yet, then it will be created.
func (repo *GitRepo) ArchiveRef(ref, archive string) error {
	refHash, err := repo.GetCommitHash(ref)
	if err != nil {
		return err
	}
	refDetails, err := repo.GetCommitDetails(ref)
	if err != nil {
		return err
	}

	commitTreeArgs := []string{"commit-tree"}
	archiveHash, err := repo.GetCommitHash(archive)
	if err != nil {
		archiveHash = ""
	} else {
		commitTreeArgs = append(commitTreeArgs, "-p", archiveHash)
	}
	commitTreeArgs = append(commitTreeArgs, "-p", refHash, "-m", fmt.Sprintf("Archive %s", refHash), refDetails.Tree)
	newArchiveHash, err := repo.runGitCommand(commitTreeArgs...)
	if err != nil {
		return err
	}
	newArchiveHash = strings.TrimSpace(newArchiveHash)
	updateRefArgs := []string{"update-ref", archive, newArchiveHash}
	if archiveHash != "" {
		updateRefArgs = append(updateRefArgs, archiveHash)
	}
	_, err = repo.runGitCommand(updateRefArgs...)
	return err
}

// MergeRef merges the given ref into the current one.
//
// The ref argument is the ref to merge, and fastForward indicates that the
// current ref should only move forward, as opposed to creating a bubble merge.
// The messages argument(s) provide text that should be included in the default
// merge commit message (separated by blank lines).
func (repo *GitRepo) MergeRef(ref string, fastForward bool, messages ...string) error {
	args := []string{"merge"}
	if fastForward {
		args = append(args, "--ff", "--ff-only")
	} else {
		args = append(args, "--no-ff")
	}
	if len(messages) > 0 {
		commitMessage := strings.Join(messages, "\n\n")
		args = append(args, "-e", "-m", commitMessage)
	}
	args = append(args, ref)
	return repo.runGitCommandInline(args...)
}

// RebaseRef rebases the current ref onto the given one.
func (repo *GitRepo) RebaseRef(ref string) error {
	return repo.runGitCommandInline("rebase", "-i", ref)
}

// ListCommits returns the list of commits reachable from the given ref.
//
// The generated list is in chronological order (with the oldest commit first).
//
// If the specified ref does not exist, then this method returns an empty result.
func (repo *GitRepo) ListCommits(ref string) []string {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := repo.runGitCommandWithIO(nil, &stdout, &stderr, "rev-list", "--reverse", ref); err != nil {
		return nil
	}

	byteLines := bytes.Split(stdout.Bytes(), []byte("\n"))
	var commits []string
	for _, byteLine := range byteLines {
		commits = append(commits, string(byteLine))
	}
	return commits
}

// ListCommitsBetween returns the list of commits between the two given revisions.
//
// The "from" parameter is the starting point (exclusive), and the "to"
// parameter is the ending point (inclusive).
//
// The "from" commit does not need to be an ancestor of the "to" commit. If it
// is not, then the merge base of the two is used as the starting point.
// Admittedly, this makes calling these the "between" commits is a bit of a
// misnomer, but it also makes the method easier to use when you want to
// generate the list of changes in a feature branch, as it eliminates the need
// to explicitly calculate the merge base. This also makes the semantics of the
// method compatible with git's built-in "rev-list" command.
//
// The generated list is in chronological order (with the oldest commit first).
func (repo *GitRepo) ListCommitsBetween(from, to string) ([]string, error) {
	out, err := repo.runGitCommand("rev-list", "--reverse", from+".."+to)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// GetNotes uses the "git" command-line tool to read the notes from the given ref for a given revision.
func (repo *GitRepo) GetNotes(notesRef, revision string) []Note {
	var notes []Note
	rawNotes, err := repo.runGitCommand("notes", "--ref", notesRef, "show", revision)
	if err != nil {
		// We just assume that this means there are no notes
		return nil
	}
	for _, line := range strings.Split(rawNotes, "\n") {
		notes = append(notes, Note([]byte(line)))
	}
	return notes
}

func stringsReader(s []*string) io.Reader {
	var subReaders []io.Reader
	for _, strPtr := range s {
		subReader := strings.NewReader(*strPtr)
		subReaders = append(subReaders, subReader, strings.NewReader("\n"))
	}
	return io.MultiReader(subReaders...)
}

// splitBatchCheckOutput parses the output of a 'git cat-file --batch-check=...' command.
//
// The output is expected to be formatted as a series of entries, with each
// entry consisting of:
// 1. The SHA1 hash of the git object being output, followed by a space.
// 2. The git "type" of the object (commit, blob, tree, missing, etc), followed by a newline.
//
// To generate this format, make sure that the 'git cat-file' command includes
// the argument '--batch-check=%(objectname) %(objecttype)'.
//
// The return value is a map from object hash to a boolean indicating if that object is a commit.
func splitBatchCheckOutput(out *bytes.Buffer) (map[string]bool, error) {
	isCommit := make(map[string]bool)
	reader := bufio.NewReader(out)
	for {
		nameLine, err := reader.ReadString(byte(' '))
		if err == io.EOF {
			return isCommit, nil
		}
		if err != nil {
			return nil, fmt.Errorf("Failure while reading the next object name: %v", err)
		}
		nameLine = strings.TrimSuffix(nameLine, " ")
		typeLine, err := reader.ReadString(byte('\n'))
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("Failure while reading the next object type: %q - %v", nameLine, err)
		}
		typeLine = strings.TrimSuffix(typeLine, "\n")
		if typeLine == "commit" {
			isCommit[nameLine] = true
		}
	}
}

// splitBatchCatFileOutput parses the output of a 'git cat-file --batch=...' command.
//
// The output is expected to be formatted as a series of entries, with each
// entry consisting of:
// 1. The SHA1 hash of the git object being output, followed by a newline.
// 2. The size of the object's contents in bytes, followed by a newline.
// 3. The objects contents.
//
// To generate this format, make sure that the 'git cat-file' command includes
// the argument '--batch=%(objectname)\n%(objectsize)'.
func splitBatchCatFileOutput(out *bytes.Buffer) (map[string][]byte, error) {
	contentsMap := make(map[string][]byte)
	reader := bufio.NewReader(out)
	for {
		nameLine, err := reader.ReadString(byte('\n'))
		if strings.HasSuffix(nameLine, "\n") {
			nameLine = strings.TrimSuffix(nameLine, "\n")
		}
		if err == io.EOF {
			return contentsMap, nil
		}
		if err != nil {
			return nil, fmt.Errorf("Failure while reading the next object name: %v", err)
		}
		sizeLine, err := reader.ReadString(byte('\n'))
		if strings.HasSuffix(sizeLine, "\n") {
			sizeLine = strings.TrimSuffix(sizeLine, "\n")
		}
		if err != nil {
			return nil, fmt.Errorf("Failure while reading the next object size: %q - %v", nameLine, err)
		}
		size, err := strconv.Atoi(sizeLine)
		if err != nil {
			return nil, fmt.Errorf("Failure while parsing the next object size: %q - %v", nameLine, err)
		}
		contentBytes := make([]byte, size, size)
		readDest := contentBytes
		len := 0
		err = nil
		for err == nil && len < size {
			nextLen := 0
			nextLen, err = reader.Read(readDest)
			len += nextLen
			readDest = contentBytes[len:]
		}
		contentsMap[nameLine] = contentBytes
		if err == io.EOF {
			return contentsMap, nil
		}
		if err != nil {
			return nil, err
		}
		for bs, err := reader.Peek(1); err == nil && bs[0] == byte('\n'); bs, err = reader.Peek(1) {
			reader.ReadByte()
		}
	}
}

// notesMapping represents the association between a git object and the notes for that object.
type notesMapping struct {
	ObjectHash *string
	NotesHash  *string
}

// notesOverview represents a high-level overview of all the notes under a single notes ref.
type notesOverview struct {
	NotesMappings      []*notesMapping
	ObjectHashesReader io.Reader
	NotesHashesReader  io.Reader
}

// notesOverview returns an overview of the git notes stored under the given ref.
func (repo *GitRepo) notesOverview(notesRef string) (*notesOverview, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := repo.runGitCommandWithIO(nil, &stdout, &stderr, "notes", "--ref", notesRef, "list"); err != nil {
		return nil, err
	}

	var notesMappings []*notesMapping
	var objHashes []*string
	var notesHashes []*string
	outScanner := bufio.NewScanner(&stdout)
	for outScanner.Scan() {
		line := outScanner.Text()
		lineParts := strings.Split(line, " ")
		if len(lineParts) != 2 {
			return nil, fmt.Errorf("Malformed output line from 'git-notes list': %q", line)
		}
		objHash := &lineParts[1]
		notesHash := &lineParts[0]
		notesMappings = append(notesMappings, &notesMapping{
			ObjectHash: objHash,
			NotesHash:  notesHash,
		})
		objHashes = append(objHashes, objHash)
		notesHashes = append(notesHashes, notesHash)
	}
	err := outScanner.Err()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("Failure parsing the output of 'git-notes list': %v", err)
	}
	return &notesOverview{
		NotesMappings:      notesMappings,
		ObjectHashesReader: stringsReader(objHashes),
		NotesHashesReader:  stringsReader(notesHashes),
	}, nil
}

// getIsCommitMap returns a mapping of all the annotated objects that are commits.
func (overview *notesOverview) getIsCommitMap(repo *GitRepo) (map[string]bool, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := repo.runGitCommandWithIO(overview.ObjectHashesReader, &stdout, &stderr, "cat-file", "--batch-check=%(objectname) %(objecttype)"); err != nil {
		return nil, fmt.Errorf("Failure performing a batch file check: %v", err)
	}
	isCommit, err := splitBatchCheckOutput(&stdout)
	if err != nil {
		return nil, fmt.Errorf("Failure parsing the output of a batch file check: %v", err)
	}
	return isCommit, nil
}

// getNoteContentsMap returns a mapping from all the notes hashes to their contents.
func (overview *notesOverview) getNoteContentsMap(repo *GitRepo) (map[string][]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := repo.runGitCommandWithIO(overview.NotesHashesReader, &stdout, &stderr, "cat-file", "--batch=%(objectname)\n%(objectsize)"); err != nil {
		return nil, fmt.Errorf("Failure performing a batch file read: %v", err)
	}
	noteContentsMap, err := splitBatchCatFileOutput(&stdout)
	if err != nil {
		return nil, fmt.Errorf("Failure parsing the output of a batch file read: %v", err)
	}
	return noteContentsMap, nil
}

// GetAllNotes reads the contents of the notes under the given ref for every commit.
//
// The returned value is a mapping from commit hash to the list of notes for that commit.
//
// This is the batch version of the corresponding GetNotes(...) method.
func (repo *GitRepo) GetAllNotes(notesRef string) (map[string][]Note, error) {
	// This code is unfortunately quite complicated, but it needs to be so.
	//
	// Conceptually, this is equivalent to:
	//   result := make(map[string][]Note)
	//   for _, commit := range repo.ListNotedRevisions(notesRef) {
	//     result[commit] = repo.GetNotes(notesRef, commit)
	//   }
	//   return result, nil
	//
	// However, that logic would require separate executions of the 'git'
	// command for every annotated commit. For a repo with 10s of thousands
	// of reviews, that would mean calling Cmd.Run(...) 10s of thousands of
	// times. That, in turn, would take so long that the tool would be unusable.
	//
	// This method avoids that by taking advantage of the 'git cat-file --batch="..."'
	// command. That allows us to use a single invocation of Cmd.Run(...) to
	// inspect multiple git objects at once.
	//
	// As such, regardless of the number of reviews in a repo, we can get all
	// of the notes using a total of three invocations of Cmd.Run(...):
	//  1. One to list all the annotated objects (and their notes hash)
	//  2. A second one to filter out all of the annotated objects that are not commits.
	//  3. A final one to get the contents of all of the notes blobs.
	overview, err := repo.notesOverview(notesRef)
	if err != nil {
		return nil, err
	}
	isCommit, err := overview.getIsCommitMap(repo)
	if err != nil {
		return nil, fmt.Errorf("Failure building the set of commit objects: %v", err)
	}
	noteContentsMap, err := overview.getNoteContentsMap(repo)
	if err != nil {
		return nil, fmt.Errorf("Failure building the mapping from notes hash to contents: %v", err)
	}
	commitNotesMap := make(map[string][]Note)
	for _, notesMapping := range overview.NotesMappings {
		if !isCommit[*notesMapping.ObjectHash] {
			continue
		}
		noteBytes := noteContentsMap[*notesMapping.NotesHash]
		byteSlices := bytes.Split(noteBytes, []byte("\n"))
		var notes []Note
		for _, slice := range byteSlices {
			notes = append(notes, Note(slice))
		}
		commitNotesMap[*notesMapping.ObjectHash] = notes
	}

	return commitNotesMap, nil
}

// AppendNote appends a note to a revision under the given ref.
func (repo *GitRepo) AppendNote(notesRef, revision string, note Note) error {
	_, err := repo.runGitCommand("notes", "--ref", notesRef, "append", "-m", string(note), revision)
	return err
}

// ListNotedRevisions returns the collection of revisions that are annotated by notes in the given ref.
func (repo *GitRepo) ListNotedRevisions(notesRef string) []string {
	var revisions []string
	notesListOut, err := repo.runGitCommand("notes", "--ref", notesRef, "list")
	if err != nil {
		return nil
	}
	notesList := strings.Split(notesListOut, "\n")
	for _, notePair := range notesList {
		noteParts := strings.SplitN(notePair, " ", 2)
		if len(noteParts) == 2 {
			objHash := noteParts[1]
			objType, err := repo.runGitCommand("cat-file", "-t", objHash)
			// If a note points to an object that we do not know about (yet), then err will not
			// be nil. We can safely just ignore those notes.
			if err == nil && objType == "commit" {
				revisions = append(revisions, objHash)
			}
		}
	}
	return revisions
}

// PushNotes pushes git notes to a remote repo.
func (repo *GitRepo) PushNotes(remote, notesRefPattern string) error {
	refspec := fmt.Sprintf("%s:%s", notesRefPattern, notesRefPattern)

	// The push is liable to fail if the user forgot to do a pull first, so
	// we treat errors as user errors rather than fatal errors.
	err := repo.runGitCommandInline("push", remote, refspec)
	if err != nil {
		return fmt.Errorf("Failed to push to the remote '%s': %v", remote, err)
	}
	return nil
}

// PushNotesAndArchive pushes the given notes and archive refs to a remote repo.
func (repo *GitRepo) PushNotesAndArchive(remote, notesRefPattern, archiveRefPattern string) error {
	notesRefspec := fmt.Sprintf("%s:%s", notesRefPattern, notesRefPattern)
	archiveRefspec := fmt.Sprintf("%s:%s", archiveRefPattern, archiveRefPattern)
	err := repo.runGitCommandInline("push", remote, notesRefspec, archiveRefspec)
	if err != nil {
		return fmt.Errorf("Failed to push the local archive to the remote '%s': %v", remote, err)
	}
	return nil
}

func getRemoteNotesRef(remote, localNotesRef string) string {
	relativeNotesRef := strings.TrimPrefix(localNotesRef, "refs/notes/")
	return "refs/notes/" + remote + "/" + relativeNotesRef
}

func (repo *GitRepo) mergeRemoteNotes(remote, notesRefPattern string) error {
	remoteRefs, err := repo.runGitCommand("ls-remote", remote, notesRefPattern)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(remoteRefs, "\n") {
		lineParts := strings.Split(line, "\t")
		if len(lineParts) == 2 {
			ref := lineParts[1]
			remoteRef := getRemoteNotesRef(remote, ref)
			_, err := repo.runGitCommand("notes", "--ref", ref, "merge", remoteRef, "-s", "cat_sort_uniq")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// PullNotes fetches the contents of the given notes ref from a remote repo,
// and then merges them with the corresponding local notes using the
// "cat_sort_uniq" strategy.
func (repo *GitRepo) PullNotes(remote, notesRefPattern string) error {
	remoteNotesRefPattern := getRemoteNotesRef(remote, notesRefPattern)
	fetchRefSpec := fmt.Sprintf("+%s:%s", notesRefPattern, remoteNotesRefPattern)
	err := repo.runGitCommandInline("fetch", remote, fetchRefSpec)
	if err != nil {
		return err
	}

	return repo.mergeRemoteNotes(remote, notesRefPattern)
}

func getRemoteArchiveRef(remote, archiveRefPattern string) string {
	relativeArchiveRef := strings.TrimPrefix(archiveRefPattern, "refs/devtools/archives/")
	return "refs/devtools/remoteArchives/" + remote + "/" + relativeArchiveRef
}

func (repo *GitRepo) mergeRemoteArchives(remote, archiveRefPattern string) error {
	remoteRefs, err := repo.runGitCommand("ls-remote", remote, archiveRefPattern)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(remoteRefs, "\n") {
		lineParts := strings.Split(line, "\t")
		if len(lineParts) == 2 {
			ref := lineParts[1]
			remoteRef := getRemoteArchiveRef(remote, ref)
			if err := repo.mergeArchives(ref, remoteRef); err != nil {
				return err
			}
		}
	}
	return nil
}

// PullNotesAndArchive fetches the contents of the notes and archives refs from
// a remote repo, and merges them with the corresponding local refs.
//
// For notes refs, we assume that every note can be automatically merged using
// the 'cat_sort_uniq' strategy (the git-appraise schemas fit that requirement),
// so we automatically merge the remote notes into the local notes.
//
// For "archive" refs, they are expected to be used solely for maintaining
// reachability of commits that are part of the history of any reviews,
// so we do not maintain any consistency with their tree objects. Instead,
// we merely ensure that their history graph includes every commit that we
// intend to keep.
func (repo *GitRepo) PullNotesAndArchive(remote, notesRefPattern, archiveRefPattern string) error {
	remoteArchiveRef := getRemoteArchiveRef(remote, archiveRefPattern)
	archiveFetchRefSpec := fmt.Sprintf("+%s:%s", archiveRefPattern, remoteArchiveRef)

	remoteNotesRefPattern := getRemoteNotesRef(remote, notesRefPattern)
	notesFetchRefSpec := fmt.Sprintf("+%s:%s", notesRefPattern, remoteNotesRefPattern)

	err := repo.runGitCommandInline("fetch", remote, notesFetchRefSpec, archiveFetchRefSpec)
	if err != nil {
		return err
	}

	if err := repo.mergeRemoteNotes(remote, notesRefPattern); err != nil {
		return err
	}
	if err := repo.mergeRemoteArchives(remote, archiveRefPattern); err != nil {
		return err
	}
	return nil
}
