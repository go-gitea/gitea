# zstd 

[Zstandard](https://facebook.github.io/zstd/) is a real-time compression algorithm, providing high compression ratios. 
It offers a very wide range of compression / speed trade-off, while being backed by a very fast decoder.
A high performance compression algorithm is implemented. For now focused on speed. 

This package provides [compression](#Compressor) to and [decompression](#Decompressor) of Zstandard content. 
Note that custom dictionaries are not supported yet, so if your code relies on that, 
you cannot use the package as-is.

This package is pure Go and without use of "unsafe". 
If a significant speedup can be achieved using "unsafe", it may be added as an option later.

The `zstd` package is provided as open source software using a Go standard license.

Currently the package is heavily optimized for 64 bit processors and will be significantly slower on 32 bit processors.

## Installation

Install using `go get -u github.com/klauspost/compress`. The package is located in `github.com/klauspost/compress/zstd`.

Godoc Documentation: https://godoc.org/github.com/klauspost/compress/zstd


## Compressor

### Status: 

STABLE - there may always be subtle bugs, a wide variety of content has been tested and the library is actively 
used by several projects. This library is being continuously [fuzz-tested](https://github.com/klauspost/compress-fuzz),
kindly supplied by [fuzzit.dev](https://fuzzit.dev/).

There may still be specific combinations of data types/size/settings that could lead to edge cases, 
so as always, testing is recommended.  

For now, a high speed (fastest) and medium-fast (default) compressor has been implemented. 

The "Fastest" compression ratio is roughly equivalent to zstd level 1. 
The "Default" compression ratio is roughly equivalent to zstd level 3 (default).

In terms of speed, it is typically 2x as fast as the stdlib deflate/gzip in its fastest mode. 
The compression ratio compared to stdlib is around level 3, but usually 3x as fast.

Compared to cgo zstd, the speed is around level 3 (default), but compression slightly worse, between level 1&2.

 
### Usage

An Encoder can be used for either compressing a stream via the
`io.WriteCloser` interface supported by the Encoder or as multiple independent
tasks via the `EncodeAll` function.
Smaller encodes are encouraged to use the EncodeAll function.
Use `NewWriter` to create a new instance that can be used for both.

To create a writer with default options, do like this:

```Go
// Compress input to output.
func Compress(in io.Reader, out io.Writer) error {
    w, err := NewWriter(output)
    if err != nil {
        return err
    }
    _, err := io.Copy(w, input)
    if err != nil {
        enc.Close()
        return err
    }
    return enc.Close()
}
```

Now you can encode by writing data to `enc`. The output will be finished writing when `Close()` is called.
Even if your encode fails, you should still call `Close()` to release any resources that may be held up.  

The above is fine for big encodes. However, whenever possible try to *reuse* the writer.

To reuse the encoder, you can use the `Reset(io.Writer)` function to change to another output. 
This will allow the encoder to reuse all resources and avoid wasteful allocations. 

Currently stream encoding has 'light' concurrency, meaning up to 2 goroutines can be working on part 
of a stream. This is independent of the `WithEncoderConcurrency(n)`, but that is likely to change 
in the future. So if you want to limit concurrency for future updates, specify the concurrency
you would like.

You can specify your desired compression level using `WithEncoderLevel()` option. Currently only pre-defined 
compression settings can be specified.

#### Future Compatibility Guarantees

This will be an evolving project. When using this package it is important to note that both the compression efficiency and speed may change.

The goal will be to keep the default efficiency at the default zstd (level 3). 
However the encoding should never be assumed to remain the same, 
and you should not use hashes of compressed output for similarity checks.

The Encoder can be assumed to produce the same output from the exact same code version.
However, the may be modes in the future that break this, 
although they will not be enabled without an explicit option.   

This encoder is not designed to (and will probably never) output the exact same bitstream as the reference encoder.

Also note, that the cgo decompressor currently does not [report all errors on invalid input](https://github.com/DataDog/zstd/issues/59),
[omits error checks](https://github.com/DataDog/zstd/issues/61), [ignores checksums](https://github.com/DataDog/zstd/issues/43) 
and seems to ignore concatenated streams, even though [it is part of the spec](https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#frames).

#### Blocks

For compressing small blocks, the returned encoder has a function called `EncodeAll(src, dst []byte) []byte`.

`EncodeAll` will encode all input in src and append it to dst.
This function can be called concurrently, but each call will only run on a single goroutine.

Encoded blocks can be concatenated and the result will be the combined input stream.
Data compressed with EncodeAll can be decoded with the Decoder, using either a stream or `DecodeAll`.

Especially when encoding blocks you should take special care to reuse the encoder. 
This will effectively make it run without allocations after a warmup period. 
To make it run completely without allocations, supply a destination buffer with space for all content.   

```Go
import "github.com/klauspost/compress/zstd"

// Create a writer that caches compressors.
// For this operation type we supply a nil Reader.
var encoder, _ = zstd.NewWriter(nil)

// Compress a buffer. 
// If you have a destination buffer, the allocation in the call can also be eliminated.
func Compress(src []byte) []byte {
    return encoder.EncodeAll(src, make([]byte, 0, len(src)))
} 
```

You can control the maximum number of concurrent encodes using the `WithEncoderConcurrency(n)` 
option when creating the writer.

Using the Encoder for both a stream and individual blocks concurrently is safe. 

### Performance

I have collected some speed examples to compare speed and compression against other compressors.

* `file` is the input file.
* `out` is the compressor used. `zskp` is this package. `gzstd` is gzip standard library. `zstd` is the Datadog cgo library.
* `level` is the compression level used. For `zskp` level 1 is "fastest", level 2 is "default".
* `insize`/`outsize` is the input/output size.
* `millis` is the number of milliseconds used for compression.
* `mb/s` is megabytes (2^20 bytes) per second.

```
The test data for the Large Text Compression Benchmark is the first
10^9 bytes of the English Wikipedia dump on Mar. 3, 2006.
http://mattmahoney.net/dc/textdata.html

file    out     level   insize  outsize     millis  mb/s
enwik9  zskp    1   1000000000  343833033   5840    163.30
enwik9  zskp    2   1000000000  317822183   8449    112.87
enwik9  gzstd   1   1000000000  382578136   13627   69.98
enwik9  gzstd   3   1000000000  349139651   22344   42.68
enwik9  zstd    1   1000000000  357416379   4838    197.12
enwik9  zstd    3   1000000000  313734522   7556    126.21

GOB stream of binary data. Highly compressible.
https://files.klauspost.com/compress/gob-stream.7z

file        out level   insize      outsize     millis  mb/s
gob-stream  zskp    1   1911399616  234981983   5100    357.42
gob-stream  zskp    2   1911399616  208674003   6698    272.15
gob-stream  gzstd   1   1911399616  357382641   14727   123.78
gob-stream  gzstd   3   1911399616  327835097   17005   107.19
gob-stream  zstd    1   1911399616  250787165   4075    447.22
gob-stream  zstd    3   1911399616  208191888   5511    330.77

Highly compressible JSON file. Similar to logs in a lot of ways.
https://files.klauspost.com/compress/adresser.001.gz

file            out level   insize      outsize     millis  mb/s
adresser.001    zskp    1   1073741824  18510122    1477    692.83
adresser.001    zskp    2   1073741824  19831697    1705    600.59
adresser.001    gzstd   1   1073741824  47755503    3079    332.47
adresser.001    gzstd   3   1073741824  40052381    3051    335.63
adresser.001    zstd    1   1073741824  16135896    994     1030.18
adresser.001    zstd    3   1073741824  17794465    905     1131.49

VM Image, Linux mint with a few installed applications:
https://files.klauspost.com/compress/rawstudio-mint14.7z

file    out level   insize  outsize millis  mb/s
rawstudio-mint14.tar    zskp    1   8558382592  3648168838  33398   244.38
rawstudio-mint14.tar    zskp    2   8558382592  3376721436  50962   160.16
rawstudio-mint14.tar    gzstd   1   8558382592  3926257486  84712   96.35
rawstudio-mint14.tar    gzstd   3   8558382592  3740711978  176344  46.28
rawstudio-mint14.tar    zstd    1   8558382592  3607859742  27903   292.51
rawstudio-mint14.tar    zstd    3   8558382592  3341710879  46700   174.77


The test data is designed to test archivers in realistic backup scenarios.
http://mattmahoney.net/dc/10gb.html

file    out level   insize  outsize millis  mb/s
10gb.tar    zskp    1   10065157632 4883149814  45715   209.97
10gb.tar    zskp    2   10065157632 4638110010  60970   157.44
10gb.tar    gzstd   1   10065157632 5198296126  97769   98.18
10gb.tar    gzstd   3   10065157632 4932665487  313427  30.63
10gb.tar    zstd    1   10065157632 4940796535  40391   237.65
10gb.tar    zstd    3   10065157632 4638618579  52911   181.42

Silesia Corpus:
http://sun.aei.polsl.pl/~sdeor/corpus/silesia.zip

file    out level   insize  outsize millis  mb/s
silesia.tar zskp    1   211947520   73025800    1108    182.26
silesia.tar zskp    2   211947520   67674684    1599    126.41
silesia.tar gzstd   1   211947520   80007735    2515    80.37
silesia.tar gzstd   3   211947520   73133380    4259    47.45
silesia.tar zstd    1   211947520   73513991    933     216.64
silesia.tar zstd    3   211947520   66793301    1377    146.79
```

### Converters

As part of the development process a *Snappy* -> *Zstandard* converter was also built.

This can convert a *framed* [Snappy Stream](https://godoc.org/github.com/golang/snappy#Writer) to a zstd stream. 
Note that a single block is not framed.

Conversion is done by converting the stream directly from Snappy without intermediate full decoding.
Therefore the compression ratio is much less than what can be done by a full decompression
and compression, and a faulty Snappy stream may lead to a faulty Zstandard stream without
any errors being generated.
No CRC value is being generated and not all CRC values of the Snappy stream are checked.
However, it provides really fast re-compression of Snappy streams.


```
BenchmarkSnappy_ConvertSilesia-8           1  1156001600 ns/op   183.35 MB/s
Snappy len 103008711 -> zstd len 82687318

BenchmarkSnappy_Enwik9-8           1  6472998400 ns/op   154.49 MB/s
Snappy len 508028601 -> zstd len 390921079
```


```Go
    s := zstd.SnappyConverter{}
    n, err = s.Convert(input, output)
    if err != nil {
        fmt.Println("Re-compressed stream to", n, "bytes")
    }
```

The converter `s` can be reused to avoid allocations, even after errors.


## Decompressor

Staus: STABLE - there may still be subtle bugs, but a wide variety of content has been tested.

This library is being continuously [fuzz-tested](https://github.com/klauspost/compress-fuzz),
kindly supplied by [fuzzit.dev](https://fuzzit.dev/). 
The main purpose of the fuzz testing is to ensure that it is not possible to crash the decoder, 
or run it past its limits with ANY input provided.  
 
### Usage

The package has been designed for two main usages, big streams of data and smaller in-memory buffers. 
There are two main usages of the package for these. Both of them are accessed by creating a `Decoder`.

For streaming use a simple setup could look like this:

```Go
import "github.com/klauspost/compress/zstd"

func Decompress(in io.Reader, out io.Writer) error {
    d, err := zstd.NewReader(input)
    if err != nil {
        return err
    }
    defer d.Close()
    
    // Copy content...
    _, err := io.Copy(out, d)
    return err
}
```

It is important to use the "Close" function when you no longer need the Reader to stop running goroutines. 
See "Allocation-less operation" below.

For decoding buffers, it could look something like this:

```Go
import "github.com/klauspost/compress/zstd"

// Create a reader that caches decompressors.
// For this operation type we supply a nil Reader.
var decoder, _ = zstd.NewReader(nil)

// Decompress a buffer. We don't supply a destination buffer,
// so it will be allocated by the decoder.
func Decompress(src []byte) ([]byte, error) {
    return decoder.DecodeAll(src, nil)
} 
```

Both of these cases should provide the functionality needed. 
The decoder can be used for *concurrent* decompression of multiple buffers. 
It will only allow a certain number of concurrent operations to run. 
To tweak that yourself use the `WithDecoderConcurrency(n)` option when creating the decoder.   

### Allocation-less operation

The decoder has been designed to operate without allocations after a warmup. 

This means that you should *store* the decoder for best performance. 
To re-use a stream decoder, use the `Reset(r io.Reader) error` to switch to another stream.
A decoder can safely be re-used even if the previous stream failed.

To release the resources, you must call the `Close()` function on a decoder.
After this it can *no longer be reused*, but all running goroutines will be stopped.
So you *must* use this if you will no longer need the Reader.

For decompressing smaller buffers a single decoder can be used.
When decoding buffers, you can supply a destination slice with length 0 and your expected capacity.
In this case no unneeded allocations should be made. 

### Concurrency

The buffer decoder does everything on the same goroutine and does nothing concurrently.
It can however decode several buffers concurrently. Use `WithDecoderConcurrency(n)` to limit that.

The stream decoder operates on

* One goroutine reads input and splits the input to several block decoders.
* A number of decoders will decode blocks.
* A goroutine coordinates these blocks and sends history from one to the next.

So effectively this also means the decoder will "read ahead" and prepare data to always be available for output.

Since "blocks" are quite dependent on the output of the previous block stream decoding will only have limited concurrency.

In practice this means that concurrency is often limited to utilizing about 2 cores effectively.
 
 
### Benchmarks

These are some examples of performance compared to [datadog cgo library](https://github.com/DataDog/zstd).

The first two are streaming decodes and the last are smaller inputs. 
 
```
BenchmarkDecoderSilesia-8             20       642550210 ns/op   329.85 MB/s      3101 B/op        8 allocs/op
BenchmarkDecoderSilesiaCgo-8         100       384930000 ns/op   550.61 MB/s    451878 B/op     9713 allocs/op

BenchmarkDecoderEnwik9-2              10        3146000080 ns/op         317.86 MB/s        2649 B/op          9 allocs/op
BenchmarkDecoderEnwik9Cgo-2           20        1905900000 ns/op         524.69 MB/s     1125120 B/op      45785 allocs/op

BenchmarkDecoder_DecodeAll/z000000.zst-8               200     7049994 ns/op   138.26 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000001.zst-8            100000       19560 ns/op    97.49 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000002.zst-8              5000      297599 ns/op   236.99 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000003.zst-8              2000      725502 ns/op   141.17 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000004.zst-8            200000        9314 ns/op    54.54 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000005.zst-8             10000      137500 ns/op   104.72 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000006.zst-8               500     2316009 ns/op   206.06 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000007.zst-8             20000       64499 ns/op   344.90 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000008.zst-8             50000       24900 ns/op   219.56 MB/s        40 B/op        2 allocs/op
BenchmarkDecoder_DecodeAll/z000009.zst-8              1000     2348999 ns/op   154.01 MB/s        40 B/op        2 allocs/op

BenchmarkDecoder_DecodeAllCgo/z000000.zst-8            500     4268005 ns/op   228.38 MB/s   1228849 B/op        3 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000001.zst-8         100000       15250 ns/op   125.05 MB/s      2096 B/op        3 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000002.zst-8          10000      147399 ns/op   478.49 MB/s     73776 B/op        3 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000003.zst-8           5000      320798 ns/op   319.27 MB/s    139312 B/op        3 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000004.zst-8         200000       10004 ns/op    50.77 MB/s       560 B/op        3 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000005.zst-8          20000       73599 ns/op   195.64 MB/s     19120 B/op        3 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000006.zst-8           1000     1119003 ns/op   426.48 MB/s    557104 B/op        3 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000007.zst-8          20000      103450 ns/op   215.04 MB/s     71296 B/op        9 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000008.zst-8         100000       20130 ns/op   271.58 MB/s      6192 B/op        3 allocs/op
BenchmarkDecoder_DecodeAllCgo/z000009.zst-8           2000     1123500 ns/op   322.00 MB/s    368688 B/op        3 allocs/op
```

This reflects the performance around May 2019, but this may be out of date.

# Contributions

Contributions are always welcome. 
For new features/fixes, remember to add tests and for performance enhancements include benchmarks.

For sending files for reproducing errors use a service like [goobox](https://goobox.io/#/upload) or similar to share your files.

For general feedback and experience reports, feel free to open an issue or write me on [Twitter](https://twitter.com/sh0dan).

This package includes the excellent [`github.com/cespare/xxhash`](https://github.com/cespare/xxhash) package Copyright (c) 2016 Caleb Spare.
