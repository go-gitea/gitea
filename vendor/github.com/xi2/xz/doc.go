// Package xz implements XZ decompression natively in Go.
//
// Usage
//
// For ease of use, this package is designed to have a similar API to
// compress/gzip. See the examples for further details.
//
// Implementation
//
// This package is a translation from C to Go of XZ Embedded
// (http://tukaani.org/xz/embedded.html) with enhancements made so as
// to implement all mandatory and optional parts of the XZ file format
// specification v1.0.4. It supports all filters and block check
// types, supports multiple streams, and performs index verification
// using SHA-256 as recommended by the specification.
//
// Speed
//
// On the author's Intel Ivybridge i5, decompression speed is about
// half that of the standard XZ Utils (tested with a recent linux
// kernel tarball).
//
// Thanks
//
// Thanks are due to Lasse Collin and Igor Pavlov, the authors of XZ
// Embedded, on whose code package xz is based. It would not exist
// without their decision to allow others to modify and reuse their
// code.
//
// Bug reports
//
// For bug reports relating to this package please contact the author
// through https://github.com/xi2/xz/issues, and not the authors of XZ
// Embedded.
package xz
