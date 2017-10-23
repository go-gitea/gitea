// TODO:
// With model objects here, we can write tests for them and for TreeListing? Or maybe
// they should go elsewhere?

// Then we can write fuller integration tests, with model objects that we're expecting and asserting
// against
import (
	"path/filepath"

	"code.gitea.io/git"
)

// RepoFile represents a file blob contained in the repository
type RepoFile struct {
	Path     string            `json:"path"`
	// Mode     git.EntryMode     `json:"mode"`  // TODO: Do we include this? It'll require exporting the mode as public in the `git` module...
	Type     git.ObjectType    `json:"type"`
	// Size     int64             `json:"size"` // TODO: Do we include this? It's expensive...
	SHA      string            `json:"sha"`
	URL      string            `json:"url"`
}

// RepoTreeListing represents a tree (or subtree) listing in the repository
type RepoTreeListing struct {
	SHA      string     `json:"sha"`
	Path     string     `json:"path"`
	Tree     []*RepoFile `json:"tree"`
}

// NewRepoFile creates a new RepoFile from a Git tree entry and some metadata.
func NewRepoFile(e *git.TreeEntry, parentPath string, rawLink string) *RepoFile {
	var filePath string
	if parentPath != "" {
		filePath = filepath.Join(parentPath, e.Name())
	} else {
		filePath = e.Name()
	}
	return &RepoFile{
		Path: filePath,
		// Mode: e.mode,  // TODO: Not exported by `git.TreeEntry`
		Type: e.Type,
		// Size: e.Size(), // TODO: Expensive!
		SHA: e.ID.String(),
		URL: filepath.Join(rawLink, filePath),
	}
}

// NewRepoTreeListing creates a new RepoTreeListing from a Git tree and some metadata
func NewRepoTreeListing(t *git.Tree, treePath, rawLink string, recursive bool) (*RepoTreeListing, error) {
	tree, err := t.SubTree(treePath)
	if err != nil {
		return nil, err
	}

	var entries []*RepoFile
	treeEntries, err := tree.ListEntries()
	if err != nil {
		return nil, err
	}
	treeEntries.CustomSort(base.NaturalSortLess)
	for i := range treeEntries {
		entry := treeEntries[i]
		if entry.IsDir() && recursive {
			subListing, err := treeListing(t, filepath.Join(treePath, entry.Name()), rawLink, recursive)
			if err != nil {
				return nil, err
			}
			entries = append(entries, subListing.Tree...)
		} else {
			entries = append(entries, models.NewRepoFile(treeEntries[i], treePath, rawLink))
		}
	}

	return &RepoTreeListing{
		SHA: tree.ID.String(),
	        Path: treePath,
		Tree: entries,
	}, nil
}
