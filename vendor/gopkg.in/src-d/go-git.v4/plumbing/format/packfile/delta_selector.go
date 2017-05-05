package packfile

import (
	"sort"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

const (
	// deltas based on deltas, how many steps we can do.
	// 50 is the default value used in JGit
	maxDepth = int64(50)
)

// applyDelta is the set of object types that we should apply deltas
var applyDelta = map[plumbing.ObjectType]bool{
	plumbing.BlobObject: true,
	plumbing.TreeObject: true,
}

type deltaSelector struct {
	storer storer.EncodedObjectStorer
}

func newDeltaSelector(s storer.EncodedObjectStorer) *deltaSelector {
	return &deltaSelector{s}
}

// ObjectsToPack creates a list of ObjectToPack from the hashes provided,
// creating deltas if it's suitable, using an specific internal logic
func (dw *deltaSelector) ObjectsToPack(hashes []plumbing.Hash) ([]*ObjectToPack, error) {
	otp, err := dw.objectsToPack(hashes)
	if err != nil {
		return nil, err
	}

	dw.sort(otp)

	if err := dw.walk(otp); err != nil {
		return nil, err
	}

	return otp, nil
}

func (dw *deltaSelector) objectsToPack(hashes []plumbing.Hash) ([]*ObjectToPack, error) {
	var objectsToPack []*ObjectToPack
	for _, h := range hashes {
		o, err := dw.storer.EncodedObject(plumbing.AnyObject, h)
		if err != nil {
			return nil, err
		}

		objectsToPack = append(objectsToPack, newObjectToPack(o))
	}

	return objectsToPack, nil
}

func (dw *deltaSelector) sort(objectsToPack []*ObjectToPack) {
	sort.Sort(byTypeAndSize(objectsToPack))
}

func (dw *deltaSelector) walk(objectsToPack []*ObjectToPack) error {
	for i := 0; i < len(objectsToPack); i++ {
		target := objectsToPack[i]

		// We only want to create deltas from specific types
		if !applyDelta[target.Original.Type()] {
			continue
		}

		for j := i - 1; j >= 0; j-- {
			base := objectsToPack[j]
			// Objects must use only the same type as their delta base.
			if base.Original.Type() != target.Original.Type() {
				break
			}

			if err := dw.tryToDeltify(base, target); err != nil {
				return err
			}
		}
	}

	return nil
}

func (dw *deltaSelector) tryToDeltify(base, target *ObjectToPack) error {
	// If the sizes are radically different, this is a bad pairing.
	if target.Original.Size() < base.Original.Size()>>4 {
		return nil
	}

	msz := dw.deltaSizeLimit(
		target.Object.Size(),
		base.Depth,
		target.Depth,
		target.IsDelta(),
	)

	// Nearly impossible to fit useful delta.
	if msz <= 8 {
		return nil
	}

	// If we have to insert a lot to make this work, find another.
	if base.Original.Size()-target.Object.Size() > msz {
		return nil
	}

	// Now we can generate the delta using originals
	delta, err := GetDelta(base.Original, target.Original)
	if err != nil {
		return err
	}

	// if delta better than target
	if delta.Size() < msz {
		target.SetDelta(base, delta)
	}

	return nil
}

func (dw *deltaSelector) deltaSizeLimit(targetSize int64, baseDepth int,
	targetDepth int, targetDelta bool) int64 {
	if !targetDelta {
		// Any delta should be no more than 50% of the original size
		// (for text files deflate of whole form should shrink 50%).
		n := targetSize >> 1

		// Evenly distribute delta size limits over allowed depth.
		// If src is non-delta (depth = 0), delta <= 50% of original.
		// If src is almost at limit (9/10), delta <= 10% of original.
		return n * (maxDepth - int64(baseDepth)) / maxDepth
	}

	// With a delta base chosen any new delta must be "better".
	// Retain the distribution described above.
	d := int64(targetDepth)
	n := targetSize

	// If src is whole (depth=0) and base is near limit (depth=9/10)
	// any delta using src can be 10x larger and still be better.
	//
	// If src is near limit (depth=9/10) and base is whole (depth=0)
	// a new delta dependent on src must be 1/10th the size.
	return n * (maxDepth - int64(baseDepth)) / (maxDepth - d)
}

type byTypeAndSize []*ObjectToPack

func (a byTypeAndSize) Len() int { return len(a) }

func (a byTypeAndSize) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func (a byTypeAndSize) Less(i, j int) bool {
	if a[i].Object.Type() < a[j].Object.Type() {
		return false
	}

	if a[i].Object.Type() > a[j].Object.Type() {
		return true
	}

	return a[i].Object.Size() > a[j].Object.Size()
}
