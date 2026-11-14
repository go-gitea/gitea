// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"
)

const MetadataFileName = "maven-metadata.xml"

// SnapshotVersion represents a file advertised by the snapshot metadata
type SnapshotVersion struct {
	Classifier string `xml:"classifier"`
	Extension  string `xml:"extension"`
	Value      string `xml:"value"`
}

// FileName returns the name of the file the snapshot version refers to
func (v SnapshotVersion) FileName(artifactID string) string {
	name := artifactID + "-" + v.Value
	if v.Classifier != "" {
		name += "-" + v.Classifier
	}
	return name + "." + v.Extension
}

// ParseSnapshotMetadata parses the files advertised by the maven-metadata.xml of a snapshot version
func ParseSnapshotMetadata(r io.Reader) ([]SnapshotVersion, error) {
	var m struct {
		Versions []SnapshotVersion `xml:"versioning>snapshotVersions>snapshotVersion"`
	}
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charset.NewReaderLabel
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m.Versions, nil
}

var snapshotBuildPattern = regexp.MustCompile(`^\d{8}\.\d{6}-(\d+)[-.]`)

// ParseSnapshotBuildNumber extracts the build number from a timestamped snapshot file name
func ParseSnapshotBuildNumber(artifactID, version, filename string) (int, bool) {
	rest, ok := strings.CutPrefix(filename, artifactID+"-"+strings.TrimSuffix(version, "-SNAPSHOT")+"-")
	if !ok {
		return 0, false
	}
	m := snapshotBuildPattern.FindStringSubmatch(rest)
	if m == nil {
		return 0, false
	}
	build, err := strconv.Atoi(m[1])
	return build, err == nil
}
