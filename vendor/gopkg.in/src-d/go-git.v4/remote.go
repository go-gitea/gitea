package git

import (
	"errors"
	"fmt"
	"io"

	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/sideband"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

var NoErrAlreadyUpToDate = errors.New("already up-to-date")

// Remote represents a connection to a remote repository
type Remote struct {
	c *config.RemoteConfig
	s Storer
	p sideband.Progress
}

func newRemote(s Storer, p sideband.Progress, c *config.RemoteConfig) *Remote {
	return &Remote{s: s, p: p, c: c}
}

// Config return the config
func (r *Remote) Config() *config.RemoteConfig {
	return r.c
}

func (r *Remote) String() string {
	fetch := r.c.URL
	push := r.c.URL

	return fmt.Sprintf("%s\t%s (fetch)\n%[1]s\t%s (push)", r.c.Name, fetch, push)
}

// Fetch fetches references from the remote to the local repository.
func (r *Remote) Fetch(o *FetchOptions) error {
	_, err := r.fetch(o)
	return err
}

func (r *Remote) fetch(o *FetchOptions) (refs storer.ReferenceStorer, err error) {
	if o.RemoteName == "" {
		o.RemoteName = r.c.Name
	}

	if err := o.Validate(); err != nil {
		return nil, err
	}

	if len(o.RefSpecs) == 0 {
		o.RefSpecs = r.c.Fetch
	}

	s, err := r.newFetchPackSession()
	if err != nil {
		return nil, err
	}

	defer ioutil.CheckClose(s, &err)

	ar, err := s.AdvertisedReferences()
	if err != nil {
		return nil, err
	}

	req, err := r.newUploadPackRequest(o, ar)
	if err != nil {
		return nil, err
	}

	remoteRefs, err := ar.AllReferences()
	if err != nil {
		return nil, err
	}

	req.Wants, err = getWants(o.RefSpecs, r.s, remoteRefs)
	if len(req.Wants) == 0 {
		return remoteRefs, NoErrAlreadyUpToDate
	}

	req.Haves, err = getHaves(r.s)
	if err != nil {
		return nil, err
	}

	if err := r.fetchPack(o, s, req); err != nil {
		return nil, err
	}

	if err := r.updateLocalReferenceStorage(o.RefSpecs, remoteRefs); err != nil {
		return nil, err
	}

	return remoteRefs, err
}

func (r *Remote) newFetchPackSession() (transport.FetchPackSession, error) {
	ep, err := transport.NewEndpoint(r.c.URL)
	if err != nil {
		return nil, err
	}

	c, err := client.NewClient(ep)
	if err != nil {
		return nil, err
	}

	return c.NewFetchPackSession(ep)
}

func (r *Remote) fetchPack(o *FetchOptions, s transport.FetchPackSession,
	req *packp.UploadPackRequest) (err error) {

	reader, err := s.FetchPack(req)
	if err != nil {
		return err
	}

	defer ioutil.CheckClose(reader, &err)

	if err := r.updateShallow(o, reader); err != nil {
		return err
	}

	if err = r.updateObjectStorage(
		buildSidebandIfSupported(req.Capabilities, reader, r.p),
	); err != nil {
		return err
	}

	return err
}

func getHaves(localRefs storer.ReferenceStorer) ([]plumbing.Hash, error) {
	iter, err := localRefs.IterReferences()
	if err != nil {
		return nil, err
	}

	var haves []plumbing.Hash
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}

		haves = append(haves, ref.Hash())
		return nil
	})
	if err != nil {
		return nil, err
	}

	return haves, nil
}

func getWants(spec []config.RefSpec, localStorer Storer, remoteRefs storer.ReferenceStorer) ([]plumbing.Hash, error) {
	wantTags := true
	for _, s := range spec {
		if !s.IsWildcard() {
			wantTags = false
			break
		}
	}

	iter, err := remoteRefs.IterReferences()
	if err != nil {
		return nil, err
	}

	wants := map[plumbing.Hash]bool{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if !config.MatchAny(spec, ref.Name()) {
			if !ref.IsTag() || !wantTags {
				return nil
			}
		}

		if ref.Type() == plumbing.SymbolicReference {
			ref, err = storer.ResolveReference(remoteRefs, ref.Name())
			if err != nil {
				return err
			}
		}

		if ref.Type() != plumbing.HashReference {
			return nil
		}

		hash := ref.Hash()
		exists, err := commitExists(localStorer, hash)
		if err != nil {
			return err
		}

		if !exists {
			wants[hash] = true
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	var result []plumbing.Hash
	for h := range wants {
		result = append(result, h)
	}

	return result, nil
}

func commitExists(s storer.EncodedObjectStorer, h plumbing.Hash) (bool, error) {
	_, err := s.EncodedObject(plumbing.CommitObject, h)
	if err == plumbing.ErrObjectNotFound {
		return false, nil
	}

	return true, err
}

func (r *Remote) newUploadPackRequest(o *FetchOptions,
	ar *packp.AdvRefs) (*packp.UploadPackRequest, error) {

	req := packp.NewUploadPackRequestFromCapabilities(ar.Capabilities)

	if o.Depth != 0 {
		req.Depth = packp.DepthCommits(o.Depth)
		if err := req.Capabilities.Set(capability.Shallow); err != nil {
			return nil, err
		}
	}

	if r.p == nil && ar.Capabilities.Supports(capability.NoProgress) {
		if err := req.Capabilities.Set(capability.NoProgress); err != nil {
			return nil, err
		}
	}

	return req, nil
}

func (r *Remote) updateObjectStorage(reader io.Reader) error {
	if sw, ok := r.s.(storer.PackfileWriter); ok {
		w, err := sw.PackfileWriter()
		if err != nil {
			return err
		}

		defer w.Close()
		_, err = io.Copy(w, reader)
		return err
	}

	stream := packfile.NewScanner(reader)
	d, err := packfile.NewDecoder(stream, r.s)
	if err != nil {
		return err
	}

	_, err = d.Decode()
	return err
}

func buildSidebandIfSupported(l *capability.List, reader io.Reader, p sideband.Progress) io.Reader {
	var t sideband.Type

	switch {
	case l.Supports(capability.Sideband):
		t = sideband.Sideband
	case l.Supports(capability.Sideband64k):
		t = sideband.Sideband64k
	default:
		return reader
	}

	d := sideband.NewDemuxer(t, reader)
	d.Progress = p

	return d
}

func (r *Remote) updateLocalReferenceStorage(specs []config.RefSpec, refs memory.ReferenceStorage) error {
	for _, spec := range specs {
		for _, ref := range refs {
			if !spec.Match(ref.Name()) {
				continue
			}

			if ref.Type() != plumbing.HashReference {
				continue
			}

			name := spec.Dst(ref.Name())
			n := plumbing.NewHashReference(name, ref.Hash())
			if err := r.s.SetReference(n); err != nil {
				return err
			}
		}
	}

	return r.buildFetchedTags(refs)
}

func (r *Remote) buildFetchedTags(refs storer.ReferenceStorer) error {
	iter, err := refs.IterReferences()
	if err != nil {
		return err
	}

	return iter.ForEach(func(ref *plumbing.Reference) error {
		if !ref.IsTag() {
			return nil
		}

		_, err := r.s.EncodedObject(plumbing.AnyObject, ref.Hash())
		if err == plumbing.ErrObjectNotFound {
			return nil
		}

		if err != nil {
			return err
		}

		return r.s.SetReference(ref)
	})
}

func (r *Remote) updateShallow(o *FetchOptions, resp *packp.UploadPackResponse) error {
	if o.Depth == 0 {
		return nil
	}

	return r.s.SetShallow(resp.Shallows)
}
