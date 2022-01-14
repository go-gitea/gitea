[![CBOR Library - Slideshow and Latest Docs.](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_slides.gif)](https://github.com/fxamacker/cbor/blob/master/README.md)

# CBOR library in Go
[__`fxamacker/cbor`__](https://github.com/fxamacker/cbor) is a CBOR encoder & decoder in [Go](https://golang.org).  It has a standard API, CBOR tags, options for duplicate map keys, float64→32→16, `toarray`, `keyasint`, etc.  Each release passes 375+ tests and 250+ million execs fuzzing.

[![](https://github.com/fxamacker/cbor/workflows/ci/badge.svg)](https://github.com/fxamacker/cbor/actions?query=workflow%3Aci)
[![](https://github.com/fxamacker/cbor/workflows/cover%20%E2%89%A598%25/badge.svg)](https://github.com/fxamacker/cbor/actions?query=workflow%3A%22cover+%E2%89%A598%25%22)
[![](https://github.com/fxamacker/cbor/workflows/linters/badge.svg)](https://github.com/fxamacker/cbor/actions?query=workflow%3Alinters)
[![Go Report Card](https://goreportcard.com/badge/github.com/fxamacker/cbor)](https://goreportcard.com/report/github.com/fxamacker/cbor)
[![Release](https://img.shields.io/github/release/fxamacker/cbor.svg?style=flat-square)](https://github.com/fxamacker/cbor/releases)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/fxamacker/cbor/master/LICENSE)

__What is CBOR__?  [CBOR](CBOR_GOLANG.md) ([RFC 7049](https://tools.ietf.org/html/rfc7049)) is a binary data format inspired by JSON and MessagePack.  CBOR is used in [IETF](https://www.ietf.org) Internet Standards such as COSE ([RFC 8152](https://tools.ietf.org/html/rfc8152)) and CWT ([RFC 8392 CBOR Web Token](https://tools.ietf.org/html/rfc8392)). WebAuthn also uses CBOR.

__`fxamacker/cbor`__ is safe and fast.  It safely handles malformed CBOR data:

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_security_table.svg?sanitize=1 "CBOR Security Comparison")

__`fxamacker/cbor`__ is fast when using CBOR data with Go structs:

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_speed_table.svg?sanitize=1 "CBOR Speed Comparison")

Benchmarks used data from [RFC 8392 Appendix A.1](https://tools.ietf.org/html/rfc8392#appendix-A.1) and default options for each CBOR library.

__`fxamacker/cbor`__ produces smaller binaries. All builds of cisco/senml had MessagePack feature removed:

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_size_comparison.png "CBOR library and program size comparison chart")

<hr>

__Standard API__: functions with signatures identical to [`encoding/json`](https://golang.org/pkg/encoding/json/) include:  
`Marshal`, `Unmarshal`, `NewEncoder`, `NewDecoder`, `encoder.Encode`, and `decoder.Decode`.

__Standard interfaces__ allow custom encoding or decoding:  
`BinaryMarshaler`, `BinaryUnmarshaler`, `Marshaler`, and `Unmarshaler`.

__Struct tags__ like __`toarray`__ & __`keyasint`__ translate Go struct fields to CBOR array elements, etc.

<br>

[![CBOR API](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_api_struct_tags.png)](#usage) 

<hr>

__`fxamacker/cbor`__ is a full-featured CBOR encoder and decoder.  Support for CBOR includes:

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_features.svg?sanitize=1 "CBOR Features")

<hr>

⚓  [__Installation__](#installation) • [__System Requirements__](#system-requirements) • [__Quick Start Guide__](#quick-start)

<hr>

__Why this CBOR library?__ It doesn't crash and it has well-balanced qualities: small, fast, safe and easy. It also has a standard API, CBOR tags (built-in and user-defined), float64→32→16, and duplicate map key options.

* __Standard API__. Codec functions with signatures identical to [`encoding/json`](https://golang.org/pkg/encoding/json/) include:  
`Marshal`, `Unmarshal`, `NewEncoder`, `NewDecoder`, `encoder.Encode`, and `decoder.Decode`.

* __Customizable__. Standard interfaces are provided to allow user-implemented encoding or decoding:  
`BinaryMarshaler`, `BinaryUnmarshaler`, `Marshaler`, and `Unmarshaler`.

* __Small apps__.  Same programs are 4-9 MB smaller by switching to this library.  No code gen and the only imported pkg is [x448/float16](https://github.com/x448/float16) which is maintained by the same team as this library.

* __Small data__.  The `toarray`, `keyasint`, and `omitempty` struct tags shrink size of Go structs encoded to CBOR.  Integers encode to smallest form that fits.  Floats can shrink from float64 -> float32 -> float16 if values fit.

* __Fast__. v1.3 became faster than a well-known library that uses `unsafe` optimizations and code gen.  Faster libraries will always exist, but speed is only one factor.  This library doesn't use `unsafe` optimizations or code gen.  

* __Safe__ and reliable. It prevents crashes on malicious CBOR data by using extensive tests, coverage-guided fuzzing, data validation, and avoiding Go's [`unsafe`](https://golang.org/pkg/unsafe/) pkg.  Decoder settings include: `MaxNestedLevels`, `MaxArrayElements`, `MaxMapPairs`, and `IndefLength`.

* __Easy__ and saves time. Simple (no param) functions return preset `EncOptions` so you don't have to know the differences between Canonical CBOR and CTAP2 Canonical CBOR to use those standards.

💡 Struct tags are a Go language feature.  CBOR tags relate to a CBOR data type (major type 6).

Struct tags for CBOR and JSON like `` `cbor:"name,omitempty"` `` and `` `json:"name,omitempty"` `` are supported so you can leverage your existing code.  If both `cbor:` and `json:` tags exist then it will use `cbor:`.

New struct tags like __`keyasint`__ and __`toarray`__ make compact CBOR data such as COSE, CWT, and SenML easier to use. 

⚓  [Quick Start](#quick-start) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Installation

👉 If Go modules aren't used, delete or modify example_test.go  
from `"github.com/fxamacker/cbor/v2"` to `"github.com/fxamacker/cbor"`

Using Go modules is recommended.
```
$ GO111MODULE=on go get github.com/fxamacker/cbor/v2
```

```go
import (
	"github.com/fxamacker/cbor/v2" // imports as package "cbor"
)
```

[Released versions](https://github.com/fxamacker/cbor/releases) benefit from longer fuzz tests.

## System Requirements

Using Go modules is recommended but not required. 

* Go 1.12 (or newer).
* amd64, arm64, ppc64le and s390x. Other architectures may also work but they are not tested as frequently. 

If Go modules feature isn't used, please see [Installation](#installation) about deleting or modifying example_test.go.

## Quick Start
🛡️ Use Go's `io.LimitReader` to limit size when decoding very large or indefinite size data.

Functions with identical signatures to encoding/json include:  
`Marshal`, `Unmarshal`, `NewEncoder`, `NewDecoder`, `encoder.Encode`, `decoder.Decode`.

__Default Mode__  

If default options are acceptable, package level functions can be used for encoding and decoding.

```go
b, err := cbor.Marshal(v)        // encode v to []byte b

err := cbor.Unmarshal(b, &v)     // decode []byte b to v

encoder := cbor.NewEncoder(w)    // create encoder with io.Writer w

decoder := cbor.NewDecoder(r)    // create decoder with io.Reader r
```

__Modes__

If you need to use options or CBOR tags, then you'll want to create a mode.

"Mode" means defined way of encoding or decoding -- it links the standard API to your CBOR options and CBOR tags.  This way, you don't pass around options and the API remains identical to `encoding/json`.

EncMode and DecMode are interfaces created from EncOptions or DecOptions structs.  
For example, `em, err := cbor.EncOptions{...}.EncMode()` or `em, err := cbor.CanonicalEncOptions().EncMode()`.

EncMode and DecMode use immutable options so their behavior won't accidentally change at runtime.  Modes are reusable, safe for concurrent use, and allow fast parallelism.

__Creating and Using Encoding Modes__

💡 Avoid using init().  For best performance, reuse EncMode and DecMode after creating them.

Most apps will probably create one EncMode and DecMode before init().  However, there's no limit and each can use different options.

```go
// Create EncOptions using either struct literal or a function.
opts := cbor.CanonicalEncOptions()

// If needed, modify opts. For example: opts.Time = cbor.TimeUnix

// Create reusable EncMode interface with immutable options, safe for concurrent use.
em, err := opts.EncMode()   

// Use EncMode like encoding/json, with same function signatures.
b, err := em.Marshal(v)      // encode v to []byte b

encoder := em.NewEncoder(w)  // create encoder with io.Writer w
err := encoder.Encode(v)     // encode v to io.Writer w
```

__Creating Modes With CBOR Tags__

A TagSet is used to specify CBOR tags.
 
```go
em, err := opts.EncMode()                  // no tags
em, err := opts.EncModeWithTags(ts)        // immutable tags
em, err := opts.EncModeWithSharedTags(ts)  // mutable shared tags
```

TagSet and all modes using it are safe for concurrent use.  Equivalent API is available for DecMode.

__Predefined Encoding Options__

```go
func CanonicalEncOptions() EncOptions {}            // settings for RFC 7049 Canonical CBOR
func CTAP2EncOptions() EncOptions {}                // settings for FIDO2 CTAP2 Canonical CBOR
func CoreDetEncOptions() EncOptions {}              // settings from a draft RFC (subject to change)
func PreferredUnsortedEncOptions() EncOptions {}    // settings from a draft RFC (subject to change)
```

The empty curly braces prevent a syntax highlighting bug on GitHub, please ignore them.

__Struct Tags (keyasint, toarray, omitempty)__

The `keyasint`, `toarray`, and `omitempty` struct tags make it easy to use compact CBOR message formats.  Internet standards often use CBOR arrays and CBOR maps with int keys to save space.

__More Info About API, Options, and Usage__

Options are listed in the Features section: [Encoding Options](#encoding-options) and [Decoding Options](#decoding-options)

For more details about each setting, see [Options](#options) section.

For additional API and usage examples, see [API](#api) and [Usage](#usage) sections.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Current Status
Latest version is v2.x, which has:

* __Stable API__ –  Six codec function signatures will never change.  No breaking API changes for other funcs in same major version.  And these two functions are subject to change until the draft RFC is approved by IETF (est. in 2020):
  * CoreDetEncOptions() is subject to change because it uses draft standard.
  * PreferredUnsortedEncOptions() is subject to change because it uses draft standard.
* __Passed all tests__ – v2.x passed all 375+ tests on amd64, arm64, ppc64le and s390x with linux.
* __Passed fuzzing__ – v2.2 passed 459+ million execs in coverage-guided fuzzing on Feb 24, 2020 (still fuzzing.)

__Why v2.x?__:

v1 required breaking API changes to support new features like CBOR tags, detection of duplicate map keys, and having more functions with identical signatures to `encoding/json`.

v2.1 is roughly 26% faster and uses 57% fewer allocs than v1.x when decoding COSE and CWT using default options.

__Recent Activity__:

* Release v2.1 (Feb. 17, 2020) 
   - [x] CBOR tags (major type 6) for encoding and decoding.
   - [x] Decoding options for duplicate map key detection: `DupMapKeyQuiet` (default) and `DupMapKeyEnforcedAPF`
   - [x] Decoding optimizations. Structs using keyasint tag (like COSE and CWT) is  
   24-28% faster and 53-61% fewer allocs than both v1.5 and v2.0.1.

* Release v2.2 (Feb. 24, 2020)
   - [x] CBOR BSTR <--> Go byte array (byte slices were already supported)
   - [x] Add more encoding and decoding options (MaxNestedLevels, MaxArrayElements, MaxMapKeyPairs, TagsMd, etc.)
   - [x] Fix potential error when decoding shorter CBOR indef length array to Go array (slice wasn't affected). This bug affects all prior versions of 1.x and 2.x.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Design Goals 
This library is designed to be a generic CBOR encoder and decoder.  It was initially created for a [WebAuthn (FIDO2) server library](https://github.com/fxamacker/webauthn), because existing CBOR libraries (in Go) didn't meet certain criteria in 2019.

This library is designed to be:

* __Easy__ – API is like `encoding/json` plus `keyasint` and `toarray` struct tags.
* __Small__ – Programs in cisco/senml are 4 MB smaller by switching to this library. In extreme cases programs can be smaller by 9+ MB. No code gen and the only imported pkg is x448/float16 which is maintained by the same team.
* __Safe and reliable__ – No `unsafe` pkg, coverage >95%, coverage-guided fuzzing, and data validation to avoid crashes on malformed or malicious data. Decoder settings include: `MaxNestedLevels`, `MaxArrayElements`, `MaxMapPairs`, and `IndefLength`.

Avoiding `unsafe` package has benefits.  The `unsafe` package [warns](https://golang.org/pkg/unsafe/):

> Packages that import unsafe may be non-portable and are not protected by the Go 1 compatibility guidelines.

All releases prioritize reliability to avoid crashes on decoding malformed CBOR data. See [Fuzzing and Coverage](#fuzzing-and-code-coverage).

Competing factors are balanced:

* __Speed__ vs __safety__ vs __size__ – to keep size small, avoid code generation. For safety, validate data and avoid Go's `unsafe` pkg.  For speed, use safe optimizations such as caching struct metadata. This library is faster than a well-known library that uses `unsafe` and code gen.
* __Standards compliance__ vs __size__ – Supports CBOR RFC 7049 with minor [limitations](#limitations). To limit bloat, CBOR tags are supported but not all tags are built-in. The API allows users to add tags that aren't built-in.  The API also allows custom encoding and decoding of user-defined Go types.

__Click to expand topic:__

<details>
 <summary>Supported CBOR Features (Highlights)</summary><p>

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_features.svg?sanitize=1 "CBOR Features")

</details>

<details>
 <summary>v2.0 API Design</summary><p>

v2.0 decoupled options from CBOR encoding & decoding functions:

* More encoding/decoding function signatures are identical to encoding/json.
* More function signatures can remain stable forever.
* More flexibility for evolving internal data types, optimizations, and concurrency.
* Features like CBOR tags can be added without more breaking API changes.
* Options to handle duplicate map keys can be added without more breaking API changes.

</details>

Features not in Go's standard library are usually not added.  However, the __`toarray`__ struct tag in __ugorji/go__ was too useful to ignore. It was added in v1.3 when a project mentioned they were using it with CBOR to save disk space.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Features

### Standard API

Many function signatures are identical to encoding/json, including:  
`Marshal`, `Unmarshal`, `NewEncoder`, `NewDecoder`, `encoder.Encode`, `decoder.Decode`.

`RawMessage` can be used to delay CBOR decoding or precompute CBOR encoding, like `encoding/json`.

Standard interfaces allow user-defined types to have custom CBOR encoding and decoding.  They include:  
`BinaryMarshaler`, `BinaryUnmarshaler`, `Marshaler`, and `Unmarshaler`.

`Marshaler` and `Unmarshaler` interfaces are satisfied by `MarshalCBOR` and `UnmarshalCBOR` functions using same params and return types as Go's MarshalJSON and UnmarshalJSON.

### Struct Tags

Support "cbor" and "json" keys in Go's struct tags. If both are specified, then "cbor" is used.

* `toarray` struct tag allows named struct fields for elements of CBOR arrays.
* `keyasint` struct tag allows named struct fields for elements of CBOR maps with int keys.
* `omitempty` struct tag excludes empty field values from being encoded.

See [Usage](#usage).

### CBOR Tags (New in v2.1)

There are three broad categories of CBOR tags:

* __Default built-in CBOR tags__ currently include tag numbers 0 and 1 (Time).  Additional default built-in tags in future releases may include tag numbers 2 and 3 (Bignum).  

* __Optional built-in CBOR tags__ may be provided in the future via build flags or optional package(s) to help reduce bloat.

* __User-defined CBOR tags__ are easy by using TagSet to associate tag numbers to user-defined Go types.

### Preferred Serialization

Preferred serialization encodes integers and floating-point values using the fewest bytes possible.

* Integers are always encoded using the fewest bytes possible.
* Floating-point values can optionally encode from float64->float32->float16 when values fit.

### Compact Data Size

The combination of preferred serialization and struct tags (toarray, keyasint, omitempty) allows very compact data size.

### Predefined Encoding Options

Easy-to-use functions (no params) return preset EncOptions struct:  
`CanonicalEncOptions`, `CTAP2EncOptions`, `CoreDetEncOptions`, `PreferredUnsortedEncOptions`

### Encoding Options

Integers always encode to the shortest form that preserves value.  By default, time values are encoded without tags.

Encoding of other data types and map key sort order are determined by encoder options.

| Encoding Option | Available Settings (defaults in bold, aliases in italics) |
| --------------- | --------------------------------------------------------- |
| EncOptions.Sort | __`SortNone`__, `SortLengthFirst`, `SortBytewiseLexical`, _`SortCanonical`_, _`SortCTAP2`_, _`SortCoreDeterministic`_ |
| EncOptions.Time | __`TimeUnix`__, `TimeUnixMicro`, `TimeUnixDynamic`, `TimeRFC3339`, `TimeRFC3339Nano` |
| EncOptions.TimeTag | __`EncTagNone`__, `EncTagRequired` |
| EncOptions.ShortestFloat | __`ShortestFloatNone`__, `ShortestFloat16` |
| EncOptions.InfConvert | __`InfConvertFloat16`__, `InfConvertNone` |
| EncOptions.NaNConvert | __`NaNConvert7e00`__, `NaNConvertNone`, `NaNConvertQuiet`, `NaNConvertPreserveSignal` |
| EncOptions.IndefLength | __`IndefLengthAllowed`__, `IndefLengthForbidden` |
| EncOptions.TagsMd | __`TagsAllowed`__, `TagsForbidden` |

See [Options](#options) section for details about each setting.

### Decoding Options

| Decoding Option | Available Settings (defaults in bold, aliases in italics) |
| --------------- | --------------------------------------------------------- |
| DecOptions.TimeTag | __`DecTagIgnored`__, `DecTagOptional`, `DecTagRequired` |
| DecOptions.DupMapKey | __`DupMapKeyQuiet`__, `DupMapKeyEnforcedAPF` |
| DecOptions.IndefLength | __`IndefLengthAllowed`__, `IndefLengthForbidden` |
| DecOptions.TagsMd | __`TagsAllowed`__, `TagsForbidden` |
| DecOptions.MaxNestedLevels | __32__, can be set to [4, 256] |
| DecOptions.MaxArrayElements | __131072__, can be set to [16, 134217728] |
| DecOptions.MaxMapPairs | __131072__, can be set to [16, 134217728] |

See [Options](#options) section for details about each setting.

### Additional Features

* Decoder always checks for invalid UTF-8 string errors.
* Decoder always decodes in-place to slices, maps, and structs.
* Decoder tries case-sensitive first and falls back to case-insensitive field name match when decoding to structs. 
* Both encoder and decoder support indefinite length CBOR data (["streaming"](https://tools.ietf.org/html/rfc7049#section-2.2)).
* Both encoder and decoder correctly handles nil slice, map, pointer, and interface values.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Standards
This library is a full-featured generic CBOR [(RFC 7049)](https://tools.ietf.org/html/rfc7049) encoder and decoder.  Notable CBOR features include:

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_features.svg?sanitize=1 "CBOR Features")

See the Features section for list of [Encoding Options](#encoding-options) and [Decoding Options](#decoding-options).

Known limitations are noted in the [Limitations section](#limitations). 

Go nil values for slices, maps, pointers, etc. are encoded as CBOR null.  Empty slices, maps, etc. are encoded as empty CBOR arrays and maps.

Decoder checks for all required well-formedness errors, including all "subkinds" of syntax errors and too little data.

After well-formedness is verified, basic validity errors are handled as follows:

* Invalid UTF-8 string: Decoder always checks and returns invalid UTF-8 string error.
* Duplicate keys in a map: Decoder has options to ignore or enforce rejection of duplicate map keys.

When decoding well-formed CBOR arrays and maps, decoder saves the first error it encounters and continues with the next item.  Options to handle this differently may be added in the future.

See [Options](#options) section for detailed settings or [Features](#features) section for a summary of options.

__Click to expand topic:__

<details>
 <summary>Duplicate Map Keys</summary><p>

This library provides options for fast detection and rejection of duplicate map keys based on applying a Go-specific data model to CBOR's extended generic data model in order to determine duplicate vs distinct map keys. Detection relies on whether the CBOR map key would be a duplicate "key" when decoded and applied to the user-provided Go map or struct. 

`DupMapKeyQuiet` turns off detection of duplicate map keys. It tries to use a "keep fastest" method by choosing either "keep first" or "keep last" depending on the Go data type.

`DupMapKeyEnforcedAPF` enforces detection and rejection of duplidate map keys. Decoding stops immediately and returns `DupMapKeyError` when the first duplicate key is detected. The error includes the duplicate map key and the index number. 

APF suffix means "Allow Partial Fill" so the destination map or struct can contain some decoded values at the time of error. It is the caller's responsibility to respond to the `DupMapKeyError` by discarding the partially filled result if that's required by their protocol.

</details>

## Limitations

If any of these limitations prevent you from using this library, please open an issue along with a link to your project.

* CBOR negative int (type 1) that cannot fit into Go's int64 are not supported, such as RFC 7049 example -18446744073709551616.  Decoding these values returns `cbor.UnmarshalTypeError` like Go's `encoding/json`. However, this may be resolved in a future release by adding support for `big.Int`. Until then, users can use the API for custom encoding and decoding.
* CBOR `Undefined` (0xf7) value decodes to Go's `nil` value.  CBOR `Null` (0xf6) more closely matches Go's `nil`.
* CBOR map keys with data types not supported by Go for map keys are ignored and an error is returned after continuing to decode remaining items.  
* When using io.Reader interface to read very large or indefinite length CBOR data, Go's `io.LimitReader` should be used to limit size.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## API
Many function signatures are identical to Go's encoding/json, such as:  
`Marshal`, `Unmarshal`, `NewEncoder`, `NewDecoder`, `encoder.Encode`, and `decoder.Decode`.

Interfaces identical or comparable to Go's encoding, encoding/json, or encoding/gob include:  
`Marshaler`, `Unmarshaler`, `BinaryMarshaler`, and `BinaryUnmarshaler`.

Like `encoding/json`, `RawMessage` can be used to delay CBOR decoding or precompute CBOR encoding.

"Mode" in this API means defined way of encoding or decoding -- it links the standard API to CBOR options and CBOR tags.

EncMode and DecMode are interfaces created from EncOptions or DecOptions structs.  
For example, `em, err := cbor.EncOptions{...}.EncMode()` or `em, err := cbor.CanonicalEncOptions().EncMode()`.

EncMode and DecMode use immutable options so their behavior won't accidentally change at runtime.  Modes are intended to be reused and are safe for concurrent use.

__API for Default Mode__

If default options are acceptable, then you don't need to create EncMode or DecMode.

```go
Marshal(v interface{}) ([]byte, error)
NewEncoder(w io.Writer) *Encoder

Unmarshal(data []byte, v interface{}) error
NewDecoder(r io.Reader) *Decoder
```

__API for Creating & Using Encoding Modes__

```go
// EncMode interface uses immutable options and is safe for concurrent use.
type EncMode interface {
	Marshal(v interface{}) ([]byte, error)
	NewEncoder(w io.Writer) *Encoder
	EncOptions() EncOptions  // returns copy of options
}

// EncOptions specifies encoding options.
type EncOptions struct {
...
}

// EncMode returns an EncMode interface created from EncOptions.
func (opts EncOptions) EncMode() (EncMode, error) {}

// EncModeWithTags returns EncMode with options and tags that are both immutable. 
func (opts EncOptions) EncModeWithTags(tags TagSet) (EncMode, error) {}

// EncModeWithSharedTags returns EncMode with immutable options and mutable shared tags. 
func (opts EncOptions) EncModeWithSharedTags(tags TagSet) (EncMode, error) {}
```

The empty curly braces prevent a syntax highlighting bug, please ignore them.

__API for Predefined Encoding Options__

```go
func CanonicalEncOptions() EncOptions {}            // settings for RFC 7049 Canonical CBOR
func CTAP2EncOptions() EncOptions {}                // settings for FIDO2 CTAP2 Canonical CBOR
func CoreDetEncOptions() EncOptions {}              // settings from a draft RFC (subject to change)
func PreferredUnsortedEncOptions() EncOptions {}    // settings from a draft RFC (subject to change)
```

__API for Creating & Using Decoding Modes__

```go
// DecMode interface uses immutable options and is safe for concurrent use.
type DecMode interface {
	Unmarshal(data []byte, v interface{}) error
	NewDecoder(r io.Reader) *Decoder
	DecOptions() DecOptions  // returns copy of options
}

// DecOptions specifies decoding options.
type DecOptions struct {
...
}

// DecMode returns a DecMode interface created from DecOptions.
func (opts DecOptions) DecMode() (DecMode, error) {}

// DecModeWithTags returns DecMode with options and tags that are both immutable. 
func (opts DecOptions) DecModeWithTags(tags TagSet) (DecMode, error) {}

// DecModeWithSharedTags returns DecMode with immutable options and mutable shared tags. 
func (opts DecOptions) DecModeWithSharedTags(tags TagSet) (DecMode, error) {}
```

The empty curly braces prevent a syntax highlighting bug, please ignore them.

__API for Using CBOR Tags__

`TagSet` can be used to associate user-defined Go type(s) to tag number(s).  It's also used to create EncMode or DecMode. For example, `em := EncOptions{...}.EncModeWithTags(ts)` or `em := EncOptions{...}.EncModeWithSharedTags(ts)`. This allows every standard API exported by em (like `Marshal` and `NewEncoder`) to use the specified tags automatically.

`Tag` and `RawTag` can be used to encode/decode a tag number with a Go value, but `TagSet` is generally recommended.

```go
type TagSet interface {
    // Add adds given tag number(s), content type, and tag options to TagSet.
    Add(opts TagOptions, contentType reflect.Type, num uint64, nestedNum ...uint64) error

    // Remove removes given tag content type from TagSet.
    Remove(contentType reflect.Type)    
}
```

`Tag` and `RawTag` types can also be used to encode/decode tag number with Go value.

```go
type Tag struct {
    Number  uint64
    Content interface{}
}

type RawTag struct {
    Number  uint64
    Content RawMessage
}
```

See [API docs (godoc.org)](https://godoc.org/github.com/fxamacker/cbor) for more details and more functions.  See [Usage section](#usage) for usage and code examples.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Options

Options for the decoding and encoding are listed here.

### Decoding Options

| DecOptions.TimeTag | Description |
| ------------------ | ----------- |
| DecTagIgnored (default) | Tag numbers are ignored (if present) for time values. |
| DecTagOptional | Tag numbers are only checked for validity if present for time values. |
| DecTagRequired | Tag numbers must be provided for time values except for CBOR Null and CBOR Undefined. |

CBOR Null and CBOR Undefined are silently treated as Go's zero time instant.  Go's `time` package provides `IsZero` function, which reports whether t represents the zero time instant, January 1, year 1, 00:00:00 UTC. 

| DecOptions.DupMapKey | Description |
| -------------------- | ----------- |
| DupMapKeyQuiet (default) | turns off detection of duplicate map keys. It uses a "keep fastest" method by choosing either "keep first" or "keep last" depending on the Go data type. |
| DupMapKeyEnforcedAPF | enforces detection and rejection of duplidate map keys. Decoding stops immediately and returns `DupMapKeyError` when the first duplicate key is detected. The error includes the duplicate map key and the index number. |

`DupMapKeyEnforcedAPF` uses "Allow Partial Fill" so the destination map or struct can contain some decoded values at the time of error.  Users can respond to the `DupMapKeyError` by discarding the partially filled result if that's required by their protocol.

| DecOptions.IndefLength | Description |
| ---------------------- | ----------- |
|IndefLengthAllowed (default) | allow indefinite length data |
|IndefLengthForbidden | forbid indefinite length data |

| DecOptions.TagsMd | Description |
| ----------------- | ----------- |
|TagsAllowed (default) | allow CBOR tags (major type 6) |
|TagsForbidden | forbid CBOR tags (major type 6) |

| DecOptions.MaxNestedLevels | Description |
| -------------------------- | ----------- |
| 32 (default) | allowed setting is [4, 256] |

| DecOptions.MaxArrayElements | Description |
| --------------------------- | ----------- |
| 131072 (default) | allowed setting is [16, 134217728] |

| DecOptions.MaxMapPairs | Description |
| ---------------------- | ----------- |
| 131072 (default) | allowed setting is [16, 134217728] |

### Encoding Options

__Integers always encode to the shortest form that preserves value__.  Encoding of other data types and map key sort order are determined by encoding options.

These functions are provided to create and return a modifiable EncOptions struct with predefined settings.

| Predefined EncOptions | Description |
| --------------------- | ----------- |
| CanonicalEncOptions() |[Canonical CBOR (RFC 7049 Section 3.9)](https://tools.ietf.org/html/rfc7049#section-3.9). |
| CTAP2EncOptions() |[CTAP2 Canonical CBOR (FIDO2 CTAP2)](https://fidoalliance.org/specs/fido-v2.0-id-20180227/fido-client-to-authenticator-protocol-v2.0-id-20180227.html#ctap2-canonical-cbor-encoding-form). |
| PreferredUnsortedEncOptions() |Unsorted, encode float64->float32->float16 when values fit, NaN values encoded as float16 0x7e00. |
| CoreDetEncOptions() |PreferredUnsortedEncOptions() + map keys are sorted bytewise lexicographic. |

🌱 CoreDetEncOptions() and PreferredUnsortedEncOptions() are subject to change until the draft RFC they used is approved by IETF.

| EncOptions.Sort | Description |
| --------------- | ----------- |
| SortNone (default) |No sorting for map keys. |
| SortLengthFirst |Length-first map key ordering. |
| SortBytewiseLexical |Bytewise lexicographic map key ordering |
| SortCanonical |(alias) Same as SortLengthFirst [(RFC 7049 Section 3.9)](https://tools.ietf.org/html/rfc7049#section-3.9) |
| SortCTAP2 |(alias) Same as SortBytewiseLexical [(CTAP2 Canonical CBOR)](https://fidoalliance.org/specs/fido-v2.0-id-20180227/fido-client-to-authenticator-protocol-v2.0-id-20180227.html#ctap2-canonical-cbor-encoding-form). |
| SortCoreDeterministic |(alias) Same as SortBytewiseLexical. |

| EncOptions.Time | Description |
| --------------- | ----------- |
| TimeUnix (default) | (seconds) Encode as integer. |
| TimeUnixMicro | (microseconds) Encode as floating-point.  ShortestFloat option determines size. |
| TimeUnixDynamic | (seconds or microseconds) Encode as integer if time doesn't have fractional seconds, otherwise encode as floating-point rounded to microseconds. |
| TimeRFC3339 | (seconds) Encode as RFC 3339 formatted string. |
| TimeRFC3339Nano | (nanoseconds) Encode as RFC3339 formatted string. |

| EncOptions.TimeTag | Description |
| ------------------ | ----------- |
| EncTagNone (default) | Tag number will not be encoded for time values. |
| EncTagRequired | Tag number (0 or 1) will be encoded unless time value is undefined/zero-instant. |

__Undefined Time Values__

By default, undefined (zero instant) time values will encode as CBOR Null without tag number for both EncTagNone and EncTagRequired.  Although CBOR Undefined might be technically more correct for EncTagRequired, CBOR Undefined might not be supported by other generic decoders and it isn't supported by JSON.

Go's `time` package provides `IsZero` function, which reports whether t represents the zero time instant, January 1, year 1, 00:00:00 UTC. 

__Floating-Point Options__

Encoder has 3 types of options for floating-point data: ShortestFloatMode, InfConvertMode, and NaNConvertMode.

| EncOptions.ShortestFloat | Description |
| ------------------------ | ----------- |
| ShortestFloatNone (default) | No size conversion. Encode float32 and float64 to CBOR floating-point of same bit-size. |
| ShortestFloat16 | Encode float64 -> float32 -> float16 ([IEEE 754 binary16](https://en.wikipedia.org/wiki/Half-precision_floating-point_format)) when values fit. |

Conversions for infinity and NaN use InfConvert and NaNConvert settings.

| EncOptions.InfConvert | Description |
| --------------------- | ----------- |
| InfConvertFloat16 (default) | Convert +- infinity to float16 since they always preserve value (recommended) |
| InfConvertNone |Don't convert +- infinity to other representations -- used by CTAP2 Canonical CBOR |

| EncOptions.NaNConvert | Description |
| --------------------- | ----------- |
| NaNConvert7e00 (default) | Encode to 0xf97e00 (CBOR float16 = 0x7e00) -- used by RFC 7049 Canonical CBOR. |
| NaNConvertNone | Don't convert NaN to other representations -- used by CTAP2 Canonical CBOR. |
| NaNConvertQuiet | Force quiet bit = 1 and use shortest form that preserves NaN payload. |
| NaNConvertPreserveSignal | Convert to smallest form that preserves value (quit bit unmodified and NaN payload preserved). |

| EncOptions.IndefLength | Description |
| ---------------------- | ----------- |
|IndefLengthAllowed (default) | allow indefinite length data |
|IndefLengthForbidden | forbid indefinite length data |

| EncOptions.TagsMd | Description |
| ----------------- | ----------- |
|TagsAllowed (default) | allow CBOR tags (major type 6) |
|TagsForbidden | forbid CBOR tags (major type 6) |

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Usage
🛡️ Use Go's `io.LimitReader` to limit size when decoding very large or indefinite size data.

Functions with identical signatures to encoding/json include:  
`Marshal`, `Unmarshal`, `NewEncoder`, `NewDecoder`, `encoder.Encode`, `decoder.Decode`.

__Default Mode__  

If default options are acceptable, package level functions can be used for encoding and decoding.

```go
b, err := cbor.Marshal(v)        // encode v to []byte b

err := cbor.Unmarshal(b, &v)     // decode []byte b to v

encoder := cbor.NewEncoder(w)    // create encoder with io.Writer w

decoder := cbor.NewDecoder(r)    // create decoder with io.Reader r
```

__Modes__

If you need to use options or CBOR tags, then you'll want to create a mode.

"Mode" means defined way of encoding or decoding -- it links the standard API to your CBOR options and CBOR tags.  This way, you don't pass around options and the API remains identical to `encoding/json`.

EncMode and DecMode are interfaces created from EncOptions or DecOptions structs.  
For example, `em, err := cbor.EncOptions{...}.EncMode()` or `em, err := cbor.CanonicalEncOptions().EncMode()`.

EncMode and DecMode use immutable options so their behavior won't accidentally change at runtime.  Modes are reusable, safe for concurrent use, and allow fast parallelism.

__Creating and Using Encoding Modes__

EncMode is an interface ([API](#api)) created from EncOptions struct.  EncMode uses immutable options after being created and is safe for concurrent use.  For best performance, EncMode should be reused.

```go
// Create EncOptions using either struct literal or a function.
opts := cbor.CanonicalEncOptions()

// If needed, modify opts. For example: opts.Time = cbor.TimeUnix

// Create reusable EncMode interface with immutable options, safe for concurrent use.
em, err := opts.EncMode()   

// Use EncMode like encoding/json, with same function signatures.
b, err := em.Marshal(v)      // encode v to []byte b

encoder := em.NewEncoder(w)  // create encoder with io.Writer w
err := encoder.Encode(v)     // encode v to io.Writer w
```

__Struct Tags (keyasint, toarray, omitempty)__

The `keyasint`, `toarray`, and `omitempty` struct tags make it easy to use compact CBOR message formats.  Internet standards often use CBOR arrays and CBOR maps with int keys to save space.

<hr>

[![CBOR API](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_api_struct_tags.png)](#usage)

<hr>

__Decoding CWT (CBOR Web Token)__ using `keyasint` and `toarray` struct tags:

```go
// Signed CWT is defined in RFC 8392
type signedCWT struct {
	_           struct{} `cbor:",toarray"`
	Protected   []byte
	Unprotected coseHeader
	Payload     []byte
	Signature   []byte
}

// Part of COSE header definition
type coseHeader struct {
	Alg int    `cbor:"1,keyasint,omitempty"`
	Kid []byte `cbor:"4,keyasint,omitempty"`
	IV  []byte `cbor:"5,keyasint,omitempty"`
}

// data is []byte containing signed CWT

var v signedCWT
if err := cbor.Unmarshal(data, &v); err != nil {
	return err
}
```

__Encoding CWT (CBOR Web Token)__ using `keyasint` and `toarray` struct tags:

```go
// Use signedCWT struct defined in "Decoding CWT" example.

var v signedCWT
...
if data, err := cbor.Marshal(v); err != nil {
	return err
}
```

__Encoding and Decoding CWT (CBOR Web Token) with CBOR Tags__

```go
// Use signedCWT struct defined in "Decoding CWT" example.

// Create TagSet (safe for concurrency).
tags := cbor.NewTagSet()
// Register tag COSE_Sign1 18 with signedCWT type.
tags.Add(	
	cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired}, 
	reflect.TypeOf(signedCWT{}), 
	18)

// Create DecMode with immutable tags.
dm, _ := cbor.DecOptions{}.DecModeWithTags(tags)

// Unmarshal to signedCWT with tag support.
var v signedCWT
if err := dm.Unmarshal(data, &v); err != nil {
	return err
}

// Create EncMode with immutable tags.
em, _ := cbor.EncOptions{}.EncModeWithTags(tags)

// Marshal signedCWT with tag number.
if data, err := cbor.Marshal(v); err != nil {
	return err
}
```

For more examples, see [examples_test.go](example_test.go).

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Comparisons

Comparisons are between this newer library and a well-known library that had 1,000+ stars before this library was created.  Default build settings for each library were used for all comparisons.

__This library is safer__.  Small malicious CBOR messages are rejected quickly before they exhaust system resources.

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_security_table.svg?sanitize=1 "CBOR Security Comparison")

__This library is smaller__. Programs like senmlCat can be 4 MB smaller by switching to this library.  Programs using more complex CBOR data types can be 9.2 MB smaller.

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_size_comparison.png "CBOR library and program size comparison chart")

__This library is faster__ for encoding and decoding CBOR Web Token (CWT).  However, speed is only one factor and it can vary depending on data types and sizes.  Unlike the other library, this one doesn't use Go's ```unsafe``` package or code gen.

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_speed_comparison.png "CBOR library speed comparison chart")

The resource intensive `codec.CborHandle` initialization (in the other library) was placed outside the benchmark loop to make sure their library wasn't penalized.

__This library uses less memory__ for encoding and decoding CBOR Web Token (CWT) using test data from RFC 8392 A.1.

![alt text](https://github.com/fxamacker/images/raw/master/cbor/v2.2.0/cbor_memory_table.svg?sanitize=1 "CBOR Speed Comparison")

Doing your own comparisons is highly recommended.  Use your most common message sizes and data types.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Benchmarks

Go structs are faster than maps with string keys:

* decoding into struct is >28% faster than decoding into map.
* encoding struct is >35% faster than encoding map.

Go structs with `keyasint` struct tag are faster than maps with integer keys:

* decoding into struct is >28% faster than decoding into map.
* encoding struct is >34% faster than encoding map.

Go structs with `toarray` struct tag are faster than slice:

* decoding into struct is >15% faster than decoding into slice.
* encoding struct is >12% faster than encoding slice.

Doing your own benchmarks is highly recommended.  Use your most common message sizes and data types.

See [Benchmarks for fxamacker/cbor](CBOR_BENCHMARKS.md).

## Fuzzing and Code Coverage

__Over 375 tests__ must pass on 4 architectures before tagging a release.  They include all RFC 7049 examples, bugs found by fuzzing, maliciously crafted CBOR data, and over 87 tests with malformed data.

__Code coverage__ must not fall below 95% when tagging a release.  Code coverage is 98.6% (`go test -cover`) for cbor v2.2 which is among the highest for libraries (in Go) of this type.

__Coverage-guided fuzzing__ must pass 250+ million execs before tagging a release.  Fuzzing uses [fxamacker/cbor-fuzz](https://github.com/fxamacker/cbor-fuzz).  Default corpus has:

* 2 files related to WebAuthn (FIDO U2F key).
* 3 files with custom struct.
* 9 files with [CWT examples (RFC 8392 Appendix A)](https://tools.ietf.org/html/rfc8392#appendix-A).
* 17 files with [COSE examples (RFC 8152 Appendix B & C)](https://github.com/cose-wg/Examples/tree/master/RFC8152).
* 81 files with [CBOR examples (RFC 7049 Appendix A) ](https://tools.ietf.org/html/rfc7049#appendix-A). It excludes 1 errata first reported in [issue #46](https://github.com/fxamacker/cbor/issues/46).

Over 1,100 files (corpus) are used for fuzzing because it includes fuzz-generated corpus.

To prevent excessive delays, fuzzing is not restarted for a release if changes are limited to docs and comments.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)

## Versions and API Changes
This project uses [Semantic Versioning](https://semver.org), so the API is always backwards compatible unless the major version number changes.  

These functions have signatures identical to encoding/json and they will likely never change even after major new releases:  `Marshal`, `Unmarshal`, `NewEncoder`, `NewDecoder`, `encoder.Encode`, and `decoder.Decode`.

Newly added API documented as "subject to change" are excluded from SemVer.

Newly added API in the master branch that has never been release tagged are excluded from SemVer.

## Code of Conduct 
This project has adopted the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).  Contact [faye.github@gmail.com](mailto:faye.github@gmail.com) with any questions or comments.

## Contributing
Please refer to [How to Contribute](CONTRIBUTING.md).

## Security Policy
Security fixes are provided for the latest released version.

To report security vulnerabilities, please email [faye.github@gmail.com](mailto:faye.github@gmail.com) and allow time for the problem to be resolved before reporting it to the public.

## Disclaimers
Phrases like "no crashes" or "doesn't crash" mean there are no known crash bugs in the latest version based on results of unit tests and coverage-guided fuzzing.  It doesn't imply the software is 100% bug-free or 100% invulnerable to all known and unknown attacks.

Please read the license for additional disclaimers and terms.

## Special Thanks

__Making this library better__  

* Montgomery Edwards⁴⁴⁸ for [x448/float16](https://github.com/x448/float16), updating the docs, creating charts & slideshow, filing issues, nudging me to ask for feedback from users, helping with design of v2.0-v2.1 API, and general idea for DupMapKeyEnforcedAPF.
* Stefan Tatschner for using this library in [sep](https://git.sr.ht/~rumpelsepp/sep), being the 1st to discover my CBOR library, requesting time.Time in issue #1, and submitting this library in a [PR to cbor.io](https://github.com/cbor/cbor.github.io/pull/56) on Aug 12, 2019.
* Yawning Angel for using this library to [oasis-core](https://github.com/oasislabs/oasis-core), and requesting BinaryMarshaler in issue #5.
* Jernej Kos for requesting RawMessage in issue #11 and offering feedback on v2.1 API for CBOR tags.
* ZenGround0 for using this library in [go-filecoin](https://github.com/filecoin-project/go-filecoin), filing "toarray" bug in issue #129, and requesting  
CBOR BSTR <--> Go array in #133.
* Keith Randall for [fixing Go bugs and providing workarounds](https://github.com/golang/go/issues/36400) so we don't have to wait for new versions of Go.

__Help clarifying CBOR RFC 7049 or 7049bis__

* Carsten Bormann for RFC 7049 (CBOR), his fast confirmation to my RFC 7049 errata, approving my pull request to 7049bis, and his patience when I misread a line in 7049bis.
* Laurence Lundblade for his help on the IETF mailing list for 7049bis and for pointing out on a CBORbis issue that CBOR Undefined might be problematic translating to JSON.
* Jeffrey Yasskin for his help on the IETF mailing list for 7049bis.

__Words of encouragement and support__

* Jakob Borg for his words of encouragement about this library at Go Forum.  This is especially appreciated in the early stages when there's a lot of rough edges.


## License 
Copyright © 2019-present [Faye Amacker](https://github.com/fxamacker).

fxamacker/cbor is licensed under the MIT License.  See [LICENSE](LICENSE) for the full license text.

<hr>

⚓  [Install](#installation) • [Status](#current-status) • [Design Goals](#design-goals) • [Features](#features) • [Standards](#standards) • [API](#api) • [Usage](#usage) • [Fuzzing](#fuzzing-and-code-coverage) • [Security Policy](#security-policy) • [License](#license)
