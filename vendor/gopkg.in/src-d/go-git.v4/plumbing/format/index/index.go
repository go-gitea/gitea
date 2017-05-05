package index

import (
	"errors"
	"os"
	"time"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

var (
	// ErrUnsupportedVersion is returned by Decode when the idxindex file
	// version is not supported.
	ErrUnsupportedVersion = errors.New("Unsuported version")

	indexSignature          = []byte{'D', 'I', 'R', 'C'}
	treeExtSignature        = []byte{'T', 'R', 'E', 'E'}
	resolveUndoExtSignature = []byte{'R', 'E', 'U', 'C'}
)

// Stage during merge
type Stage int

const (
	// Merged is the default stage, fully merged
	Merged Stage = 1
	// AncestorMode is the base revision
	AncestorMode Stage = 1
	// OurMode is the first tree revision, ours
	OurMode Stage = 2
	// TheirMode is the second tree revision, theirs
	TheirMode Stage = 3
)

// Index contains the information about which objects are currently checked out
// in the worktree, having information about the working files. Changes in
// worktree are detected using this Index. The Index is also used during merges
type Index struct {
	Version     uint32
	Entries     []Entry
	Cache       *Tree
	ResolveUndo *ResolveUndo
}

// Entry represents a single file (or stage of a file) in the cache. An entry
// represents exactly one stage of a file. If a file path is unmerged then
// multiple Entry instances may appear for the same path name.
type Entry struct {
	// Hash is the SHA1 of the represented file
	Hash plumbing.Hash
	// Name is the  Entry path name relative to top level directory
	Name string
	// CreatedAt time when the tracked path was created
	CreatedAt time.Time
	// ModifiedAt time when the tracked path was changed
	ModifiedAt time.Time
	// Dev and Inode of the tracked path
	Dev, Inode uint32
	// Mode of the path
	Mode os.FileMode
	// UID and GID, userid and group id of the owner
	UID, GID uint32
	// Size is the length in bytes for regular files
	Size uint32
	// Stage on a merge is defines what stage is representing this entry
	// https://git-scm.com/book/en/v2/Git-Tools-Advanced-Merging
	Stage Stage
	// SkipWorktree used in sparse checkouts
	// https://git-scm.com/docs/git-read-tree#_sparse_checkout
	SkipWorktree bool
	// IntentToAdd record only the fact that the path will be added later
	// https://git-scm.com/docs/git-add ("git add -N")
	IntentToAdd bool
}

// Tree contains pre-computed hashes for trees that can be derived from the
// index. It helps speed up tree object generation from index for a new commit.
type Tree struct {
	Entries []TreeEntry
}

// TreeEntry entry of a cached Tree
type TreeEntry struct {
	// Path component (relative to its parent directory)
	Path string
	// Entries is the number of entries in the index that is covered by the tree
	// this entry represents
	Entries int
	// Trees is the number that represents the number of subtrees this tree has
	Trees int
	// Hash object name for the object that would result from writing this span
	// of index as a tree.
	Hash plumbing.Hash
}

// ResolveUndo when a conflict is resolved (e.g. with "git add path"), these
// higher stage entries will be removed and a stage-0 entry with proper
// resolution is added. When these higher stage entries are removed, they are
// saved in the resolve undo extension
type ResolveUndo struct {
	Entries []ResolveUndoEntry
}

// ResolveUndoEntry contains the information about a conflict when is resolved
type ResolveUndoEntry struct {
	Path   string
	Stages map[Stage]plumbing.Hash
}
