// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

/*
Package cbor is a fast & safe CBOR encoder & decoder (RFC 7049) with a
standard API + toarray & keyasint struct tags, CBOR tags, float64->32->16,
CTAP2 & Canonical CBOR, duplicate map key options, and is customizable via
simple API.

CBOR encoding options allow "preferred serialization" by encoding integers and floats
to their smallest forms (like float16) when values fit.

Struct tags like "keyasint", "toarray" and "omitempty" makes CBOR data smaller.

For example, "toarray" makes struct fields encode to array elements.  And "keyasint"
makes struct fields encode to elements of CBOR map with int keys.

Basics

Function signatures identical to encoding/json include:

    Marshal, Unmarshal, NewEncoder, NewDecoder, encoder.Encode, decoder.Decode.

Codec functions are available at package-level (using defaults) or by creating modes
from options at runtime.

"Mode" in this API means definite way of encoding or decoding. Specifically, EncMode or DecMode.

EncMode and DecMode interfaces are created from EncOptions or DecOptions structs.  For example,

    em := cbor.EncOptions{...}.EncMode()
    em := cbor.CanonicalEncOptions().EncMode()
    em := cbor.CTAP2EncOptions().EncMode()

Modes use immutable options to avoid side-effects and simplify concurrency. Behavior of modes
won't accidentally change at runtime after they're created.

Modes are intended to be reused and are safe for concurrent use.

EncMode and DecMode Interfaces

    // EncMode interface uses immutable options and is safe for concurrent use.
    type EncMode interface {
	Marshal(v interface{}) ([]byte, error)
	NewEncoder(w io.Writer) *Encoder
	EncOptions() EncOptions  // returns copy of options
    }

    // DecMode interface uses immutable options and is safe for concurrent use.
    type DecMode interface {
	Unmarshal(data []byte, v interface{}) error
	NewDecoder(r io.Reader) *Decoder
	DecOptions() DecOptions  // returns copy of options
    }

Using Default Encoding Mode

    b, err := cbor.Marshal(v)

    encoder := cbor.NewEncoder(w)
    err = encoder.Encode(v)

Using Default Decoding Mode

    err := cbor.Unmarshal(b, &v)

    decoder := cbor.NewDecoder(r)
    err = decoder.Decode(&v)

Creating and Using Encoding Modes

    // Create EncOptions using either struct literal or a function.
    opts := cbor.CanonicalEncOptions()

    // If needed, modify encoding options
    opts.Time = cbor.TimeUnix

    // Create reusable EncMode interface with immutable options, safe for concurrent use.
    em, err := opts.EncMode()

    // Use EncMode like encoding/json, with same function signatures.
    b, err := em.Marshal(v)
    // or
    encoder := em.NewEncoder(w)
    err := encoder.Encode(v)

Default Options

Default encoding options are listed at https://github.com/fxamacker/cbor#api

Struct Tags

Struct tags like `cbor:"name,omitempty"` and `json:"name,omitempty"` work as expected.
If both struct tags are specified then `cbor` is used.

Struct tags like "keyasint", "toarray", and "omitempty" make it easy to use
very compact formats like COSE and CWT (CBOR Web Tokens) with structs.

For example, "toarray" makes struct fields encode to array elements.  And "keyasint"
makes struct fields encode to elements of CBOR map with int keys.

https://raw.githubusercontent.com/fxamacker/images/master/cbor/v2.0.0/cbor_easy_api.png

Tests and Fuzzing

Over 375 tests are included in this package. Cover-guided fuzzing is handled by a separate package:
fxamacker/cbor-fuzz.
*/
package cbor
