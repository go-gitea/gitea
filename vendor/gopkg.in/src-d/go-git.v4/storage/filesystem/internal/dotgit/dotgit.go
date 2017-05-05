// https://github.com/git/git/blob/master/Documentation/gitrepository-layout.txt
package dotgit

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing"

	"srcd.works/go-billy.v1"
)

const (
	suffix         = ".git"
	packedRefsPath = "packed-refs"
	configPath     = "config"
	shallowPath    = "shallow"

	objectsPath = "objects"
	packPath    = "pack"
	refsPath    = "refs"

	packExt = ".pack"
	idxExt  = ".idx"
)

var (
	// ErrNotFound is returned by New when the path is not found.
	ErrNotFound = errors.New("path not found")
	// ErrIdxNotFound is returned by Idxfile when the idx file is not found
	ErrIdxNotFound = errors.New("idx file not found")
	// ErrPackfileNotFound is returned by Packfile when the packfile is not found
	ErrPackfileNotFound = errors.New("packfile not found")
	// ErrConfigNotFound is returned by Config when the config is not found
	ErrConfigNotFound = errors.New("config file not found")
	// ErrPackedRefsDuplicatedRef is returned when a duplicated reference is
	// found in the packed-ref file. This is usually the case for corrupted git
	// repositories.
	ErrPackedRefsDuplicatedRef = errors.New("duplicated ref found in packed-ref file")
	// ErrPackedRefsBadFormat is returned when the packed-ref file corrupt.
	ErrPackedRefsBadFormat = errors.New("malformed packed-ref")
	// ErrSymRefTargetNotFound is returned when a symbolic reference is
	// targeting a non-existing object. This usually means the repository
	// is corrupt.
	ErrSymRefTargetNotFound = errors.New("symbolic reference target not found")
)

// The DotGit type represents a local git repository on disk. This
// type is not zero-value-safe, use the New function to initialize it.
type DotGit struct {
	fs billy.Filesystem
}

// New returns a DotGit value ready to be used. The path argument must
// be the absolute path of a git repository directory (e.g.
// "/foo/bar/.git").
func New(fs billy.Filesystem) *DotGit {
	return &DotGit{fs: fs}
}

// ConfigWriter returns a file pointer for write to the config file
func (d *DotGit) ConfigWriter() (billy.File, error) {
	return d.fs.Create(configPath)
}

// Config returns a file pointer for read to the config file
func (d *DotGit) Config() (billy.File, error) {
	return d.fs.Open(configPath)
}

// ShallowWriter returns a file pointer for write to the shallow file
func (d *DotGit) ShallowWriter() (billy.File, error) {
	return d.fs.Create(shallowPath)
}

// Shallow returns a file pointer for read to the shallow file
func (d *DotGit) Shallow() (billy.File, error) {
	f, err := d.fs.Open(shallowPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	return f, nil
}

// NewObjectPack return a writer for a new packfile, it saves the packfile to
// disk and also generates and save the index for the given packfile.
func (d *DotGit) NewObjectPack() (*PackWriter, error) {
	return newPackWrite(d.fs)
}

// ObjectPacks returns the list of availables packfiles
func (d *DotGit) ObjectPacks() ([]plumbing.Hash, error) {
	packDir := d.fs.Join(objectsPath, packPath)
	files, err := d.fs.ReadDir(packDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var packs []plumbing.Hash
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), packExt) {
			continue
		}

		n := f.Name()
		h := plumbing.NewHash(n[5 : len(n)-5]) //pack-(hash).pack
		packs = append(packs, h)

	}

	return packs, nil
}

// ObjectPack returns a fs.File of the given packfile
func (d *DotGit) ObjectPack(hash plumbing.Hash) (billy.File, error) {
	file := d.fs.Join(objectsPath, packPath, fmt.Sprintf("pack-%s.pack", hash.String()))

	pack, err := d.fs.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPackfileNotFound
		}

		return nil, err
	}

	return pack, nil
}

// ObjectPackIdx returns a fs.File of the index file for a given packfile
func (d *DotGit) ObjectPackIdx(hash plumbing.Hash) (billy.File, error) {
	file := d.fs.Join(objectsPath, packPath, fmt.Sprintf("pack-%s.idx", hash.String()))
	idx, err := d.fs.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPackfileNotFound
		}

		return nil, err
	}

	return idx, nil
}

// NewObject return a writer for a new object file.
func (d *DotGit) NewObject() (*ObjectWriter, error) {
	return newObjectWriter(d.fs)
}

// Objects returns a slice with the hashes of objects found under the
// .git/objects/ directory.
func (d *DotGit) Objects() ([]plumbing.Hash, error) {
	files, err := d.fs.ReadDir(objectsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var objects []plumbing.Hash
	for _, f := range files {
		if f.IsDir() && len(f.Name()) == 2 && isHex(f.Name()) {
			base := f.Name()
			d, err := d.fs.ReadDir(d.fs.Join(objectsPath, base))
			if err != nil {
				return nil, err
			}

			for _, o := range d {
				objects = append(objects, plumbing.NewHash(base+o.Name()))
			}
		}
	}

	return objects, nil
}

// Object return a fs.File poiting the object file, if exists
func (d *DotGit) Object(h plumbing.Hash) (billy.File, error) {
	hash := h.String()
	file := d.fs.Join(objectsPath, hash[0:2], hash[2:40])

	return d.fs.Open(file)
}

func (d *DotGit) SetRef(r *plumbing.Reference) error {
	var content string
	switch r.Type() {
	case plumbing.SymbolicReference:
		content = fmt.Sprintf("ref: %s\n", r.Target())
	case plumbing.HashReference:
		content = fmt.Sprintln(r.Hash().String())
	}

	f, err := d.fs.Create(r.Name().String())
	if err != nil {
		return err
	}

	if _, err := f.Write([]byte(content)); err != nil {
		return err
	}
	return f.Close()
}

// Refs scans the git directory collecting references, which it returns.
// Symbolic references are resolved and included in the output.
func (d *DotGit) Refs() ([]*plumbing.Reference, error) {
	var refs []*plumbing.Reference
	if err := d.addRefsFromPackedRefs(&refs); err != nil {
		return nil, err
	}

	if err := d.addRefsFromRefDir(&refs); err != nil {
		return nil, err
	}

	if err := d.addRefFromHEAD(&refs); err != nil {
		return nil, err
	}

	return refs, nil
}

// Ref returns the reference for a given reference name.
func (d *DotGit) Ref(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	ref, err := d.readReferenceFile(".", name.String())
	if err == nil {
		return ref, nil
	}

	refs, err := d.Refs()
	if err != nil {
		return nil, err
	}

	for _, ref := range refs {
		if ref.Name() == name {
			return ref, nil
		}
	}

	return nil, plumbing.ErrReferenceNotFound
}

func (d *DotGit) addRefsFromPackedRefs(refs *[]*plumbing.Reference) (err error) {
	f, err := d.fs.Open(packedRefsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	defer func() {
		if errClose := f.Close(); err == nil {
			err = errClose
		}
	}()

	s := bufio.NewScanner(f)
	for s.Scan() {
		ref, err := d.processLine(s.Text())
		if err != nil {
			return err
		}

		if ref != nil {
			*refs = append(*refs, ref)
		}
	}

	return s.Err()
}

// process lines from a packed-refs file
func (d *DotGit) processLine(line string) (*plumbing.Reference, error) {
	switch line[0] {
	case '#': // comment - ignore
		return nil, nil
	case '^': // annotated tag commit of the previous line - ignore
		return nil, nil
	default:
		ws := strings.Split(line, " ") // hash then ref
		if len(ws) != 2 {
			return nil, ErrPackedRefsBadFormat
		}

		return plumbing.NewReferenceFromStrings(ws[1], ws[0]), nil
	}
}

func (d *DotGit) addRefsFromRefDir(refs *[]*plumbing.Reference) error {
	return d.walkReferencesTree(refs, refsPath)
}

func (d *DotGit) walkReferencesTree(refs *[]*plumbing.Reference, relPath string) error {
	files, err := d.fs.ReadDir(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, f := range files {
		newRelPath := d.fs.Join(relPath, f.Name())
		if f.IsDir() {
			if err = d.walkReferencesTree(refs, newRelPath); err != nil {
				return err
			}

			continue
		}

		ref, err := d.readReferenceFile(".", newRelPath)
		if err != nil {
			return err
		}

		if ref != nil {
			*refs = append(*refs, ref)
		}
	}

	return nil
}

func (d *DotGit) addRefFromHEAD(refs *[]*plumbing.Reference) error {
	ref, err := d.readReferenceFile(".", "HEAD")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	*refs = append(*refs, ref)
	return nil
}

func (d *DotGit) readReferenceFile(refsPath, refFile string) (ref *plumbing.Reference, err error) {
	path := d.fs.Join(refsPath, refFile)

	f, err := d.fs.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() {
		if errClose := f.Close(); err == nil {
			err = errClose
		}
	}()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	line := strings.TrimSpace(string(b))
	return plumbing.NewReferenceFromStrings(refFile, line), nil
}

func isHex(s string) bool {
	for _, b := range []byte(s) {
		if isNum(b) {
			continue
		}
		if isHexAlpha(b) {
			continue
		}

		return false
	}

	return true
}

func isNum(b byte) bool {
	return b >= '0' && b <= '9'
}

func isHexAlpha(b byte) bool {
	return b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
