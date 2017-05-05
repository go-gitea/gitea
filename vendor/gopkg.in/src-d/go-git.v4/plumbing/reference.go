package plumbing

import (
	"errors"
	"fmt"
	"strings"
)

const (
	refPrefix       = "refs/"
	refHeadPrefix   = refPrefix + "heads/"
	refTagPrefix    = refPrefix + "tags/"
	refRemotePrefix = refPrefix + "remotes/"
	refNotePrefix   = refPrefix + "notes/"
	symrefPrefix    = "ref: "
)

var (
	ErrReferenceNotFound = errors.New("reference not found")
)

// ReferenceType reference type's
type ReferenceType int8

const (
	InvalidReference  ReferenceType = 0
	HashReference     ReferenceType = 1
	SymbolicReference ReferenceType = 2
)

// ReferenceName reference name's
type ReferenceName string

func (r ReferenceName) String() string {
	return string(r)
}

// Short returns the short name of a ReferenceName
func (r ReferenceName) Short() string {
	parts := strings.Split(string(r), "/")
	return parts[len(parts)-1]
}

const (
	HEAD ReferenceName = "HEAD"
)

// Reference is a representation of git reference
type Reference struct {
	t      ReferenceType
	n      ReferenceName
	h      Hash
	target ReferenceName
}

// NewReferenceFromStrings creates a reference from name and target as string,
// the resulting reference can be a SymbolicReference or a HashReference base
// on the target provided
func NewReferenceFromStrings(name, target string) *Reference {
	n := ReferenceName(name)

	if strings.HasPrefix(target, symrefPrefix) {
		target := ReferenceName(target[len(symrefPrefix):])
		return NewSymbolicReference(n, target)
	}

	return NewHashReference(n, NewHash(target))
}

// NewSymbolicReference creates a new SymbolicReference reference
func NewSymbolicReference(n, target ReferenceName) *Reference {
	return &Reference{
		t:      SymbolicReference,
		n:      n,
		target: target,
	}
}

// NewHashReference creates a new HashReference reference
func NewHashReference(n ReferenceName, h Hash) *Reference {
	return &Reference{
		t: HashReference,
		n: n,
		h: h,
	}
}

// Type return the type of a reference
func (r *Reference) Type() ReferenceType {
	return r.t
}

// Name return the name of a reference
func (r *Reference) Name() ReferenceName {
	return r.n
}

// Hash return the hash of a hash reference
func (r *Reference) Hash() Hash {
	return r.h
}

// Target return the target of a symbolic reference
func (r *Reference) Target() ReferenceName {
	return r.target
}

// IsBranch check if a reference is a branch
func (r *Reference) IsBranch() bool {
	return strings.HasPrefix(string(r.n), refHeadPrefix)
}

// IsNote check if a reference is a note
func (r *Reference) IsNote() bool {
	return strings.HasPrefix(string(r.n), refNotePrefix)
}

// IsRemote check if a reference is a remote
func (r *Reference) IsRemote() bool {
	return strings.HasPrefix(string(r.n), refRemotePrefix)
}

// IsTag check if a reference is a tag
func (r *Reference) IsTag() bool {
	return strings.HasPrefix(string(r.n), refTagPrefix)
}

// Strings dump a reference as a [2]string
func (r *Reference) Strings() [2]string {
	var o [2]string
	o[0] = r.Name().String()

	switch r.Type() {
	case HashReference:
		o[1] = r.Hash().String()
	case SymbolicReference:
		o[1] = symrefPrefix + r.Target().String()
	}

	return o
}

func (r *Reference) String() string {
	s := r.Strings()
	return fmt.Sprintf("%s %s", s[1], s[0])
}
