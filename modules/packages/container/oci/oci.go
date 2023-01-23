// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oci

import (
	"time"
)

// https://github.com/opencontainers/image-spec/tree/main/specs-go/v1

// ImageConfig defines the execution parameters which should be used as a base when running a container using an image.
type ImageConfig struct {
	// User defines the username or UID which the process in the container should run as.
	User string `json:"User,omitempty"`

	// ExposedPorts a set of ports to expose from a container running this image.
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`

	// Env is a list of environment variables to be used in a container.
	Env []string `json:"Env,omitempty"`

	// Entrypoint defines a list of arguments to use as the command to execute when the container starts.
	Entrypoint []string `json:"Entrypoint,omitempty"`

	// Cmd defines the default arguments to the entrypoint of the container.
	Cmd []string `json:"Cmd,omitempty"`

	// Volumes is a set of directories describing where the process is likely write data specific to a container instance.
	Volumes map[string]struct{} `json:"Volumes,omitempty"`

	// WorkingDir sets the current working directory of the entrypoint process in the container.
	WorkingDir string `json:"WorkingDir,omitempty"`

	// Labels contains arbitrary metadata for the container.
	Labels map[string]string `json:"Labels,omitempty"`

	// StopSignal contains the system call signal that will be sent to the container to exit.
	StopSignal string `json:"StopSignal,omitempty"`
}

// RootFS describes a layer content addresses
type RootFS struct {
	// Type is the type of the rootfs.
	Type string `json:"type"`

	// DiffIDs is an array of layer content hashes, in order from bottom-most to top-most.
	DiffIDs []string `json:"diff_ids"`
}

// History describes the history of a layer.
type History struct {
	// Created is the combined date and time at which the layer was created, formatted as defined by RFC 3339, section 5.6.
	Created *time.Time `json:"created,omitempty"`

	// CreatedBy is the command which created the layer.
	CreatedBy string `json:"created_by,omitempty"`

	// Author is the author of the build point.
	Author string `json:"author,omitempty"`

	// Comment is a custom message set when creating the layer.
	Comment string `json:"comment,omitempty"`

	// EmptyLayer is used to mark if the history item created a filesystem diff.
	EmptyLayer bool `json:"empty_layer,omitempty"`
}

// Image is the JSON structure which describes some basic information about the image.
// This provides the `application/vnd.oci.image.config.v1+json` mediatype when marshalled to JSON.
type Image struct {
	// Created is the combined date and time at which the image was created, formatted as defined by RFC 3339, section 5.6.
	Created *time.Time `json:"created,omitempty"`

	// Author defines the name and/or email address of the person or entity which created and is responsible for maintaining the image.
	Author string `json:"author,omitempty"`

	// Architecture is the CPU architecture which the binaries in this image are built to run on.
	Architecture string `json:"architecture"`

	// Variant is the variant of the specified CPU architecture which image binaries are intended to run on.
	Variant string `json:"variant,omitempty"`

	// OS is the name of the operating system which the image is built to run on.
	OS string `json:"os"`

	// OSVersion is an optional field specifying the operating system
	// version, for example on Windows `10.0.14393.1066`.
	OSVersion string `json:"os.version,omitempty"`

	// OSFeatures is an optional field specifying an array of strings,
	// each listing a required OS feature (for example on Windows `win32k`).
	OSFeatures []string `json:"os.features,omitempty"`

	// Config defines the execution parameters which should be used as a base when running a container using the image.
	Config ImageConfig `json:"config,omitempty"`

	// RootFS references the layer content addresses used by the image.
	RootFS RootFS `json:"rootfs"`

	// History describes the history of each layer.
	History []History `json:"history,omitempty"`
}

// Descriptor describes the disposition of targeted content.
// This structure provides `application/vnd.oci.descriptor.v1+json` mediatype
// when marshalled to JSON.
type Descriptor struct {
	// MediaType is the media type of the object this schema refers to.
	MediaType MediaType `json:"mediaType,omitempty"`

	// Digest is the digest of the targeted content.
	Digest Digest `json:"digest"`

	// Size specifies the size in bytes of the blob.
	Size int64 `json:"size"`

	// URLs specifies a list of URLs from which this object MAY be downloaded
	URLs []string `json:"urls,omitempty"`

	// Annotations contains arbitrary metadata relating to the targeted content.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Data is an embedding of the targeted content. This is encoded as a base64
	// string when marshalled to JSON (automatically, by encoding/json). If
	// present, Data can be used directly to avoid fetching the targeted content.
	Data []byte `json:"data,omitempty"`

	// Platform describes the platform which the image in the manifest runs on.
	//
	// This should only be used when referring to a manifest.
	Platform *Platform `json:"platform,omitempty"`
}

// Platform describes the platform which the image in the manifest runs on.
type Platform struct {
	// Architecture field specifies the CPU architecture, for example
	// `amd64` or `ppc64`.
	Architecture string `json:"architecture"`

	// OS specifies the operating system, for example `linux` or `windows`.
	OS string `json:"os"`

	// OSVersion is an optional field specifying the operating system
	// version, for example on Windows `10.0.14393.1066`.
	OSVersion string `json:"os.version,omitempty"`

	// OSFeatures is an optional field specifying an array of strings,
	// each listing a required OS feature (for example on Windows `win32k`).
	OSFeatures []string `json:"os.features,omitempty"`

	// Variant is an optional field specifying a variant of the CPU, for
	// example `v7` to specify ARMv7 when architecture is `arm`.
	Variant string `json:"variant,omitempty"`
}

type SchemaMediaBase struct {
	// SchemaVersion is the image manifest schema that this image follows
	SchemaVersion int `json:"schemaVersion"`

	// MediaType specifies the type of this document data structure e.g. `application/vnd.oci.image.manifest.v1+json`
	MediaType MediaType `json:"mediaType,omitempty"`
}

// Manifest provides `application/vnd.oci.image.manifest.v1+json` mediatype structure when marshalled to JSON.
type Manifest struct {
	SchemaMediaBase

	// Config references a configuration object for a container, by digest.
	// The referenced configuration object is a JSON blob that the runtime uses to set up the container.
	Config Descriptor `json:"config"`

	// Layers is an indexed list of layers referenced by the manifest.
	Layers []Descriptor `json:"layers"`

	// Annotations contains arbitrary metadata for the image manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Index references manifests for various platforms.
// This structure provides `application/vnd.oci.image.index.v1+json` mediatype when marshalled to JSON.
type Index struct {
	SchemaMediaBase

	// Manifests references platform specific manifests.
	Manifests []Descriptor `json:"manifests"`

	// Annotations contains arbitrary metadata for the image index.
	Annotations map[string]string `json:"annotations,omitempty"`
}
