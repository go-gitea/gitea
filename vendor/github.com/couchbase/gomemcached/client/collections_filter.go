package memcached

import (
	"encoding/json"
	"fmt"
)

// Collection based filter
type CollectionsFilter struct {
	ManifestUid    uint64
	UseManifestUid bool
	StreamId       uint16
	UseStreamId    bool

	// Use either ScopeId OR CollectionsList, not both
	CollectionsList []uint32
	ScopeId         uint32
}

type nonStreamIdNonResumeScopeMeta struct {
	ScopeId string `json:"scope"`
}

type nonStreamIdResumeScopeMeta struct {
	ManifestId string `json:"uid"`
}

type nonStreamIdNonResumeCollectionsMeta struct {
	CollectionsList []string `json:"collections"`
}

type nonStreamIdResumeCollectionsMeta struct {
	ManifestId      string   `json:"uid"`
	CollectionsList []string `json:"collections"`
}

type streamIdNonResumeCollectionsMeta struct {
	CollectionsList []string `json:"collections"`
	StreamId        uint16   `json:"sid"`
}

type streamIdNonResumeScopeMeta struct {
	ScopeId  string `json:"scope"`
	StreamId uint16 `json:"sid"`
}

func (c *CollectionsFilter) IsValid() error {
	if c.UseManifestUid {
		return fmt.Errorf("Not implemented yet")
	}

	if len(c.CollectionsList) > 0 && c.ScopeId > 0 {
		return fmt.Errorf("Collection list is specified but scope ID is also specified")
	}

	return nil
}

func (c *CollectionsFilter) outputCollectionsFilterColList() (outputList []string) {
	for _, collectionUint := range c.CollectionsList {
		outputList = append(outputList, fmt.Sprintf("%x", collectionUint))
	}
	return
}

func (c *CollectionsFilter) outputScopeId() string {
	return fmt.Sprintf("%x", c.ScopeId)
}

func (c *CollectionsFilter) ToStreamReqBody() ([]byte, error) {
	if err := c.IsValid(); err != nil {
		return nil, err
	}

	var output interface{}

	switch c.UseStreamId {
	case true:
		switch c.UseManifestUid {
		case true:
			// TODO
			return nil, fmt.Errorf("NotImplemented0")
		case false:
			switch len(c.CollectionsList) > 0 {
			case true:
				filter := &streamIdNonResumeCollectionsMeta{
					StreamId:        c.StreamId,
					CollectionsList: c.outputCollectionsFilterColList(),
				}
				output = *filter
			case false:
				filter := &streamIdNonResumeScopeMeta{
					StreamId: c.StreamId,
					ScopeId:  c.outputScopeId(),
				}
				output = *filter
			}
		}
	case false:
		switch c.UseManifestUid {
		case true:
			// TODO
			return nil, fmt.Errorf("NotImplemented1")
		case false:
			switch len(c.CollectionsList) > 0 {
			case true:
				filter := &nonStreamIdNonResumeCollectionsMeta{
					CollectionsList: c.outputCollectionsFilterColList(),
				}
				output = *filter
			case false:
				output = nonStreamIdNonResumeScopeMeta{ScopeId: c.outputScopeId()}
			}
		}
	}

	data, err := json.Marshal(output)
	if err != nil {
		return nil, err
	} else {
		return data, nil
	}
}
