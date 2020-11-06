package rupture

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

const metaFilename = "rupture_meta.json"

func indexMetadataPath(dir string) string {
	return filepath.Join(dir, metaFilename)
}

// IndexMetadata contains metadata about a bleve index.
type IndexMetadata struct {
	// The version of the data in the index. This can be useful for tracking
	// schema changes or data migrations.
	Version int `json:"version"`
}

// in addition to the user-exposed metadata, we keep additional, internal-only
// metadata for sharded indices.
const shardedMetadataFilename = "rupture_sharded_meta.json"

func shardedIndexMetadataPath(dir string) string {
	return filepath.Join(dir, shardedMetadataFilename)
}

type shardedIndexMetadata struct {
	NumShards int `json:"num_shards"`
}

func readJSON(path string, meta interface{}) error {
	metaBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(metaBytes, meta)
}

func writeJSON(path string, meta interface{}) error {
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, metaBytes, 0666)
}

// ReadIndexMetadata returns the metadata for the index at the specified path.
// If no such index metadata exists, an empty metadata and a nil error are
// returned.
func ReadIndexMetadata(path string) (*IndexMetadata, error) {
	meta := &IndexMetadata{}
	metaPath := indexMetadataPath(path)
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return meta, nil
	} else if err != nil {
		return nil, err
	}
	return meta, readJSON(metaPath, meta)
}

// WriteIndexMetadata writes metadata for the index at the specified path.
func WriteIndexMetadata(path string, meta *IndexMetadata) error {
	return writeJSON(indexMetadataPath(path), meta)
}
